package customerfulfillment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	app "orderapp/internal/application/customerfulfillment"
	customerportalapp "orderapp/internal/application/customerportal"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool   *pgxpool.Pool
	schema string
}

func NewRepository(pool *pgxpool.Pool, schema string) *Repository {
	return &Repository{pool: pool, schema: schema}
}

func (r *Repository) StoreParsedImport(ctx context.Context, cmd app.StoreParsedImportCommand) (app.ImportBatch, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.ImportBatch{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	summaryJSON, err := json.Marshal(cmd.Parsed.Summary)
	if err != nil {
		return app.ImportBatch{}, err
	}
	sourceFilename := strings.TrimSpace(cmd.SourceFilename)
	sourceSHA := strings.ToLower(strings.TrimSpace(cmd.SourceSHA256))
	var batchID int64
	inserted := true
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_fulfillment_import_batches(
			customer_id, import_type, source_filename, source_sha256, status,
			total_rows, valid_rows, invalid_rows, summary_json, created_by
		)
		VALUES($1,$2,$3,$4,'parsed',$5,$6,$7,$8::jsonb,$9)
		ON CONFLICT (customer_id, import_type, source_sha256) DO NOTHING
		RETURNING id
	`, r.schema),
		cmd.CustomerID,
		string(cmd.ImportType),
		sourceFilename,
		sourceSHA,
		cmd.Parsed.Summary.TotalRows,
		cmd.Parsed.Summary.ValidRows,
		cmd.Parsed.Summary.InvalidRows,
		string(summaryJSON),
		strings.TrimSpace(cmd.CreatedBy),
	).Scan(&batchID)
	if errors.Is(err, pgx.ErrNoRows) {
		inserted = false
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT id FROM %s.customer_fulfillment_import_batches
			WHERE customer_id=$1 AND import_type=$2 AND source_sha256=$3
		`, r.schema), cmd.CustomerID, string(cmd.ImportType), sourceSHA).Scan(&batchID); err != nil {
			return app.ImportBatch{}, err
		}
	} else if err != nil {
		return app.ImportBatch{}, err
	}

	if inserted {
		for _, row := range cmd.Parsed.Rows {
			payloadJSON, err := json.Marshal(row.Payload)
			if err != nil {
				return app.ImportBatch{}, err
			}
			status := "valid"
			if strings.TrimSpace(row.Error) != "" {
				status = "invalid"
			}
			if _, err := tx.Exec(ctx, fmt.Sprintf(`
				INSERT INTO %s.customer_fulfillment_import_rows(
					batch_id, sheet_name, row_no, row_type, external_key, status, payload, error
				)
				VALUES($1,$2,$3,$4,$5,$6,$7::jsonb,$8)
			`, r.schema),
				batchID,
				row.SheetName,
				row.RowNo,
				row.RowType,
				row.ExternalKey,
				status,
				string(payloadJSON),
				row.Error,
			); err != nil {
				return app.ImportBatch{}, err
			}
		}
	}

	batch, err := r.loadImportBatchTx(ctx, tx, batchID)
	if err != nil {
		return app.ImportBatch{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return app.ImportBatch{}, err
	}
	return batch, nil
}

func (r *Repository) ApplyImport(ctx context.Context, cmd app.ApplyImportCommand) (app.ApplyResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.ApplyResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var customerID int64
	var importType string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT customer_id, import_type
		FROM %s.customer_fulfillment_import_batches
		WHERE id=$1
		FOR UPDATE
	`, r.schema), cmd.BatchID).Scan(&customerID, &importType); err != nil {
		return app.ApplyResult{}, err
	}
	rows, err := r.loadValidImportRowsTx(ctx, tx, cmd.BatchID)
	if err != nil {
		return app.ApplyResult{}, err
	}
	result := app.ApplyResult{BatchID: cmd.BatchID}
	directShipState := newDirectShipApplyState()
	for _, row := range rows {
		var target applyTarget
		switch app.ImportType(importType) {
		case app.ImportTypeProcessingWorkbook:
			target, err = r.applyProcessingImportRow(ctx, tx, customerID, cmd.BatchID, row)
		case app.ImportTypeDirectShipWorkbook:
			target, err = r.applyDirectShipImportRow(ctx, tx, customerID, cmd.BatchID, directShipState, row)
		case app.ImportTypeSettlementWorkbook:
			target, err = r.applySettlementImportRow(ctx, tx, customerID, row)
		default:
			target = applyTarget{}
		}
		if err != nil {
			return app.ApplyResult{}, err
		}
		if err := r.markImportRowAppliedTx(ctx, tx, row.ID, target.TargetType, target.TargetID); err != nil {
			return app.ApplyResult{}, err
		}
		result.AppliedRows++
		result.ProcessingOrders += target.ProcessingOrders
		result.DirectShipOrders += target.DirectShipOrders
		result.FeeItems += target.FeeItems
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.customer_fulfillment_import_batches
		SET status='applied', applied_at=COALESCE(applied_at, now())
		WHERE id=$1
	`, r.schema), cmd.BatchID); err != nil {
		return app.ApplyResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return app.ApplyResult{}, err
	}
	return result, nil
}

func (r *Repository) CustomerPortalContext(ctx context.Context, employeeID int64) (app.CustomerERPContext, error) {
	var row app.CustomerERPContext
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT b.employee_id,
		       b.customer_id,
		       COALESCE(NULLIF(p.display_name,''), c.name, ''),
		       b.role,
		       b.status
		FROM %s.customer_erp_user_bindings b
		JOIN %s.customers c ON c.id=b.customer_id
		LEFT JOIN %s.customer_portal_profiles p ON p.customer_id=b.customer_id
		WHERE b.employee_id=$1
		  AND b.status='active'
		  AND c.active=true
		ORDER BY b.id
		LIMIT 1
	`, r.schema, r.schema, r.schema), employeeID).Scan(&row.EmployeeID, &row.CustomerID, &row.CustomerName, &row.BindingRole, &row.BindingStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.CustomerERPContext{}, app.ErrCustomerERPBindingNotFound
	}
	if err != nil {
		return app.CustomerERPContext{}, err
	}
	return row, nil
}

func (r *Repository) CustomerPortalOverview(ctx context.Context, employeeID int64) (app.CustomerPortalOverview, error) {
	current, err := r.CustomerPortalContext(ctx, employeeID)
	if err != nil {
		return app.CustomerPortalOverview{}, err
	}
	overview := app.CustomerPortalOverview{
		CustomerID:   current.CustomerID,
		CustomerName: current.CustomerName,
	}
	if overview.Capabilities, err = r.listCustomerCapabilityCodes(ctx, current.CustomerID); err != nil {
		return app.CustomerPortalOverview{}, err
	}
	if overview.CustodyBalances, err = r.listCustodyBalances(ctx, current.CustomerID); err != nil {
		return app.CustomerPortalOverview{}, err
	}
	if overview.FinishedGoods, err = r.listFinishedGoods(ctx, current.CustomerID); err != nil {
		return app.CustomerPortalOverview{}, err
	}
	if overview.ProcessingOrders, err = r.listProcessingOrders(ctx, current.CustomerID); err != nil {
		return app.CustomerPortalOverview{}, err
	}
	if overview.DirectShipOrders, err = r.listDirectShipOrders(ctx, current.CustomerID); err != nil {
		return app.CustomerPortalOverview{}, err
	}
	if overview.Fees, err = r.listFeeItems(ctx, current.CustomerID); err != nil {
		return app.CustomerPortalOverview{}, err
	}
	if overview.Settlements, err = r.listSettlements(ctx, current.CustomerID); err != nil {
		return app.CustomerPortalOverview{}, err
	}
	return overview, nil
}

func (r *Repository) SubmitCustomerProcessingWorkOrder(ctx context.Context, cmd app.SubmitCustomerProcessingWorkOrderCommand) (app.ProcessingOrderSummary, error) {
	customerID := cmd.CustomerID
	if customerID <= 0 {
		current, err := r.CustomerPortalContext(ctx, cmd.EmployeeID)
		if err != nil {
			return app.ProcessingOrderSummary{}, err
		}
		customerID = current.CustomerID
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.ProcessingOrderSummary{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	productName := strings.TrimSpace(cmd.ProductName)
	productID := cmd.ProductID
	if productID > 0 && productName == "" {
		_ = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(name,'') FROM %s.products WHERE id=$1 AND active=true`, r.schema), productID).Scan(&productName)
	}
	payload := map[string]any{
		"submitted_by_employee_id": cmd.EmployeeID,
		"raw_bean_item_id":         cmd.RawBeanItemID,
		"raw_bean_name":            cmd.RawBeanName,
		"expected_date":            cmd.ExpectedDate,
		"note":                     cmd.Note,
	}
	var id int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_processing_work_orders(
			batch_id, customer_id, external_key, work_order_no, order_date,
			product_id, product_name, status, input_quantity_g, planned_output_units, payload
		)
		VALUES(0,$1,'','',now()::date,$2,$3,'submitted',$4,$5,$6::jsonb)
		RETURNING id
	`, r.schema), customerID, productID, productName, cmd.InputQuantityG, cmd.PlannedOutputUnits, mustPayloadJSON(payload)).Scan(&id); err != nil {
		return app.ProcessingOrderSummary{}, err
	}
	workOrderNo := fmt.Sprintf("CP-%s-%04d", time.Now().Format("20060102"), id)
	externalKey := fmt.Sprintf("erp_customer_processing:%d", id)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.customer_processing_work_orders
		SET external_key=$2, work_order_no=$3, updated_at=now()
		WHERE id=$1
	`, r.schema), id, externalKey, workOrderNo); err != nil {
		return app.ProcessingOrderSummary{}, err
	}
	rawBeanName := strings.TrimSpace(cmd.RawBeanName)
	if rawBeanName != "" || cmd.InputQuantityG > 0 {
		rawItemID := cmd.RawBeanItemID
		if rawItemID <= 0 && rawBeanName != "" {
			rawItemID, err = r.upsertCustodyItemTx(ctx, tx, customerID, "raw_bean", rawBeanName, rawBeanName, "g", map[string]any{"raw_bean_name": rawBeanName})
			if err != nil {
				return app.ProcessingOrderSummary{}, err
			}
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.customer_processing_work_order_inputs(work_order_id, raw_bean_item_id, raw_bean_name, quantity_g, payload)
			VALUES($1,$2,$3,$4,$5::jsonb)
		`, r.schema), id, rawItemID, rawBeanName, cmd.InputQuantityG, mustPayloadJSON(payload)); err != nil {
			return app.ProcessingOrderSummary{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return app.ProcessingOrderSummary{}, err
	}
	return app.ProcessingOrderSummary{
		WorkOrderNo: workOrderNo,
		ProductName: productName,
		Status:      "submitted",
		QuantityG:   cmd.InputQuantityG,
		Units:       cmd.PlannedOutputUnits,
	}, nil
}

func (r *Repository) SubmitCustomerDirectShipOrder(ctx context.Context, cmd app.SubmitCustomerDirectShipOrderCommand) (app.DirectShipOrderSummary, error) {
	customerID := cmd.CustomerID
	if customerID <= 0 {
		current, err := r.CustomerPortalContext(ctx, cmd.EmployeeID)
		if err != nil {
			return app.DirectShipOrderSummary{}, err
		}
		customerID = current.CustomerID
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.DirectShipOrderSummary{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	receiverSnapshot := strings.TrimSpace(strings.Join([]string{cmd.ReceiverName, cmd.ReceiverPhone, cmd.ReceiverAddress}, " "))
	payload := map[string]any{
		"submitted_by_employee_id": cmd.EmployeeID,
		"receiver_name":            cmd.ReceiverName,
		"receiver_phone":           cmd.ReceiverPhone,
		"receiver_company":         cmd.ReceiverCompany,
		"note":                     cmd.Note,
	}
	var importOrderID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_direct_ship_import_orders(
			batch_id, customer_id, external_order_no, external_seq, order_date,
			receiver_address, status, payload
		)
		VALUES(0,$1,'','',now()::date,$2,'submitted',$3::jsonb)
		RETURNING id
	`, r.schema), customerID, receiverSnapshot, mustPayloadJSON(payload)).Scan(&importOrderID); err != nil {
		return app.DirectShipOrderSummary{}, err
	}
	orderNo := fmt.Sprintf("CDS-%s-%04d", time.Now().Format("20060102"), importOrderID)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.customer_direct_ship_import_orders
		SET external_order_no=$2, external_seq='1'
		WHERE id=$1
	`, r.schema), importOrderID, orderNo); err != nil {
		return app.DirectShipOrderSummary{}, err
	}
	itemPayload := map[string]any{
		"submitted_by_employee_id": cmd.EmployeeID,
		"product_id":               cmd.ProductID,
		"product_name":             cmd.ProductName,
		"spec":                     cmd.Spec,
		"quantity_units":           cmd.QuantityUnits,
		"note":                     cmd.Note,
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_direct_ship_import_order_items(
			import_order_id, batch_id, customer_id, line_no, product_title, spec, quantity_units, payload
		)
		VALUES($1,0,$2,1,$3,$4,$5,$6::jsonb)
	`, r.schema), importOrderID, customerID, cmd.ProductName, cmd.Spec, cmd.QuantityUnits, mustPayloadJSON(itemPayload)); err != nil {
		return app.DirectShipOrderSummary{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return app.DirectShipOrderSummary{}, err
	}
	return app.DirectShipOrderSummary{
		OrderNo:         orderNo,
		OrderDate:       time.Now().Format("2006-01-02"),
		ReceiverAddress: receiverSnapshot,
		Status:          "submitted",
		ItemCount:       1,
	}, nil
}

func (r *Repository) AdjustCustodyInventory(ctx context.Context, cmd app.AdjustCustodyInventoryCommand) (app.CustodyBalance, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.CustodyBalance{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	itemID, err := r.upsertCustodyItemTx(ctx, tx, cmd.CustomerID, cmd.ItemType, cmd.ItemName, cmd.ItemName, custodyUnitForItemType(cmd.ItemType), map[string]any{
		"source": "manual_adjustment",
		"note":   cmd.Note,
		"actor":  cmd.Actor,
	})
	if err != nil {
		return app.CustodyBalance{}, err
	}
	sourceExternalKey := fmt.Sprintf("manual_adjustment:%d:%d", cmd.CustomerID, time.Now().UnixNano())
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_custody_ledger_entries(
			customer_id, item_id, item_type, source_type, source_external_key, movement_type,
			qty_g_delta, qty_units_delta, note, occurred_at
		)
		VALUES($1,$2,$3,'manual_adjustment',$4,'manual_adjustment',$5,$6,$7,now())
	`, r.schema), cmd.CustomerID, itemID, cmd.ItemType, sourceExternalKey, cmd.QuantityGDelta, cmd.QuantityUnitsDelta, cmd.Note); err != nil {
		return app.CustodyBalance{}, err
	}
	if _, err := r.addCustodyBalanceTx(ctx, tx, cmd.CustomerID, itemID, cmd.ItemType, cmd.ItemName, cmd.Spec, cmd.QuantityGDelta, cmd.QuantityUnitsDelta); err != nil {
		return app.CustodyBalance{}, err
	}
	var row app.CustodyBalance
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT item_type, item_name, spec, quantity_g, quantity_units
		FROM %s.customer_custody_balances
		WHERE customer_id=$1 AND item_type=$2 AND item_id=$3
	`, r.schema), cmd.CustomerID, cmd.ItemType, itemID).Scan(&row.ItemType, &row.ItemName, &row.Spec, &row.QuantityG, &row.QuantityUnits); err != nil {
		return app.CustodyBalance{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return app.CustodyBalance{}, err
	}
	return row, nil
}

func (r *Repository) UpsertCustomerERPBinding(ctx context.Context, cmd app.UpsertCustomerERPBindingCommand) (app.CustomerERPBinding, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.CustomerERPBinding{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if cmd.Status == "active" {
		var otherCustomerID int64
		err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT customer_id
			FROM %s.customer_erp_user_bindings
			WHERE employee_id=$1 AND status='active' AND customer_id<>$2
			LIMIT 1
		`, r.schema), cmd.EmployeeID, cmd.CustomerID).Scan(&otherCustomerID)
		if err == nil && otherCustomerID > 0 {
			return app.CustomerERPBinding{}, fmt.Errorf("erp account already bound to another customer")
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return app.CustomerERPBinding{}, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.customer_erp_user_bindings
			SET status='inactive', updated_by=$3, updated_at=now()
			WHERE customer_id=$1 AND status='active' AND employee_id<>$2
		`, r.schema), cmd.CustomerID, cmd.EmployeeID, cmd.Actor); err != nil {
			return app.CustomerERPBinding{}, err
		}
	}

	var row app.CustomerERPBinding
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_erp_user_bindings(customer_id, employee_id, role, status, updated_by, updated_at)
		SELECT $1, e.id, $3, $4, $5, now()
		FROM %s.company_employees e
		WHERE e.id=$2 AND e.active=true AND e.account_type='channel_customer'
		ON CONFLICT (employee_id, customer_id) DO UPDATE SET
			role=excluded.role,
			status=excluded.status,
			updated_by=excluded.updated_by,
			updated_at=now()
		RETURNING customer_id, employee_id, role, status, updated_by, to_char(updated_at,'YYYY-MM-DD HH24:MI')
	`, r.schema, r.schema), cmd.CustomerID, cmd.EmployeeID, cmd.Role, cmd.Status, cmd.Actor).Scan(&row.CustomerID, &row.EmployeeID, &row.Role, &row.Status, &row.UpdatedBy, &row.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.CustomerERPBinding{}, fmt.Errorf("channel customer account required")
	}
	if err != nil {
		return app.CustomerERPBinding{}, err
	}
	_ = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(name,'') FROM %s.company_employees WHERE id=$1`, r.schema), row.EmployeeID).Scan(&row.EmployeeName)
	if err := tx.Commit(ctx); err != nil {
		return app.CustomerERPBinding{}, err
	}
	return row, nil
}

func (r *Repository) grantCustomerTemplateRolesForEmployeeTx(ctx context.Context, tx pgx.Tx, customerID, employeeID int64) error {
	var templateKey string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(capability_template_key,'')
		FROM %s.customer_portal_profiles
		WHERE customer_id=$1
	`, r.schema), customerID).Scan(&templateKey); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	template, ok := customerportalapp.CustomerCapabilityTemplateByKey(templateKey)
	if !ok || len(template.ERPRoleCodes) == 0 {
		return nil
	}
	var hasAuthRoles, hasEmployeeRoles bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL, to_regclass($2) IS NOT NULL`,
		fmt.Sprintf("%s.auth_roles", r.schema),
		fmt.Sprintf("%s.employee_roles", r.schema),
	).Scan(&hasAuthRoles, &hasEmployeeRoles); err != nil {
		return err
	}
	if !hasAuthRoles || !hasEmployeeRoles {
		return nil
	}
	for _, roleCode := range template.ERPRoleCodes {
		roleCode = strings.TrimSpace(roleCode)
		if roleCode == "" {
			continue
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.employee_roles(employee_id, role_code)
			SELECT $1, r.code
			FROM %s.auth_roles r
			WHERE r.code=$2
			ON CONFLICT DO NOTHING
		`, r.schema, r.schema), employeeID, roleCode); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) ListCustomerERPBindings(ctx context.Context, customerID int64) ([]app.CustomerERPBinding, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT b.customer_id, b.employee_id, COALESCE(e.name,''), b.role, b.status, b.updated_by, to_char(b.updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.customer_erp_user_bindings b
		JOIN %s.company_employees e ON e.id=b.employee_id
		WHERE b.customer_id=$1
		ORDER BY b.updated_at DESC, b.id DESC
	`, r.schema, r.schema), customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]app.CustomerERPBinding, 0)
	for rows.Next() {
		var row app.CustomerERPBinding
		if err := rows.Scan(&row.CustomerID, &row.EmployeeID, &row.EmployeeName, &row.Role, &row.Status, &row.UpdatedBy, &row.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repository) CustomerFulfillmentOptions(ctx context.Context, customerID int64) (app.CustomerFulfillmentOptions, error) {
	var out app.CustomerFulfillmentOptions
	var err error
	if out.CustomerSKUs, err = r.listCustomerSKUOptions(ctx, customerID); err != nil {
		return app.CustomerFulfillmentOptions{}, err
	}
	if out.CustodyItems, err = r.listCustodyItemOptions(ctx, customerID); err != nil {
		return app.CustomerFulfillmentOptions{}, err
	}
	if out.Employees, err = r.listEmployeeOptions(ctx); err != nil {
		return app.CustomerFulfillmentOptions{}, err
	}
	if out.Recipients, err = r.listRecipientOptions(ctx, customerID); err != nil {
		return app.CustomerFulfillmentOptions{}, err
	}
	return out, nil
}

func (r *Repository) listCustomerSKUOptions(ctx context.Context, customerID int64) ([]app.CustomerSKUOption, error) {
	out := make([]app.CustomerSKUOption, 0)
	seen := map[string]struct{}{}
	add := func(row app.CustomerSKUOption) {
		row.SKUCode = strings.TrimSpace(row.SKUCode)
		row.ProductName = strings.Join(strings.Fields(strings.TrimSpace(row.ProductName)), " ")
		row.Spec = strings.TrimSpace(row.Spec)
		row.RoastDegree = strings.TrimSpace(row.RoastDegree)
		row.Source = strings.TrimSpace(row.Source)
		if row.ProductName == "" {
			return
		}
		key := fmt.Sprintf("%d|%s|%s", row.ProductID, row.ProductName, row.Spec)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, row)
	}

	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT COALESCE(NULLIF(r.target_id,0), p.id, 0),
		       COALESCE(p.base_product_id,0),
		       COALESCE(r.payload->>'sku_code', ''),
		       COALESCE(NULLIF(r.payload->>'sku_name',''), NULLIF(r.payload->>'product_name',''), p.name, ''),
		       COALESCE(NULLIF(r.payload->>'spec',''), ''),
		       COALESCE(NULLIF(r.payload->>'roast_degree',''), p.roast_level, ''),
		       'customer_sku_import'
		FROM %s.customer_fulfillment_import_rows r
		JOIN %s.customer_fulfillment_import_batches b ON b.id=r.batch_id
		LEFT JOIN %s.products p ON p.id=r.target_id
		WHERE b.customer_id=$1
		  AND r.row_type='customer_sku'
		  AND r.status='applied'
		ORDER BY r.applied_at DESC NULLS LAST, r.id DESC
		LIMIT 200
	`, r.schema, r.schema, r.schema), customerID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var row app.CustomerSKUOption
		if err := rows.Scan(&row.ProductID, &row.BaseProductID, &row.SKUCode, &row.ProductName, &row.Spec, &row.RoastDegree, &row.Source); err != nil {
			rows.Close()
			return nil, err
		}
		add(row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	rows, err = r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(base_product_id,0), '', name, '', COALESCE(roast_level,''), COALESCE(NULLIF(custom_type,''),'customer_product')
		FROM %s.products
		WHERE customer_id=$1
		  AND visibility='customer_only'
		  AND active=true
		ORDER BY name, id
		LIMIT 200
	`, r.schema), customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var row app.CustomerSKUOption
		if err := rows.Scan(&row.ProductID, &row.BaseProductID, &row.SKUCode, &row.ProductName, &row.Spec, &row.RoastDegree, &row.Source); err != nil {
			return nil, err
		}
		add(row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if allowed, err := r.customerAllowsPublicSKUOptions(ctx, customerID); err != nil {
		return nil, err
	} else if allowed {
		rows.Close()
		rows, err = r.pool.Query(ctx, fmt.Sprintf(`
			SELECT id, 0, '公共SKU', name, '', COALESCE(roast_level,''), 'public_sku'
			FROM %s.products
			WHERE active=true
			  AND COALESCE(customer_id,0)=0
			  AND COALESCE(NULLIF(visibility,''),'public')='public'
			ORDER BY name, id
			LIMIT 300
		`, r.schema))
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var row app.CustomerSKUOption
			if err := rows.Scan(&row.ProductID, &row.BaseProductID, &row.SKUCode, &row.ProductName, &row.Spec, &row.RoastDegree, &row.Source); err != nil {
				return nil, err
			}
			add(row)
		}
		return out, rows.Err()
	}
	return out, nil
}

func (r *Repository) listCustomerCapabilityCodes(ctx context.Context, customerID int64) ([]string, error) {
	if customerID <= 0 {
		return []string{}, nil
	}
	var hasTable bool
	if err := r.pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, fmt.Sprintf("%s.customer_service_capabilities", r.schema)).Scan(&hasTable); err != nil {
		return nil, err
	}
	if !hasTable {
		return []string{}, nil
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT capability_code
		FROM %s.customer_service_capabilities
		WHERE customer_id=$1 AND enabled=true
		ORDER BY capability_code
	`, r.schema), customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		code = strings.TrimSpace(code)
		if code != "" {
			out = append(out, code)
		}
	}
	return out, rows.Err()
}

func (r *Repository) customerAllowsPublicSKUOptions(ctx context.Context, customerID int64) (bool, error) {
	if customerID <= 0 {
		return false, nil
	}
	var hasTable bool
	if err := r.pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, fmt.Sprintf("%s.customer_service_capabilities", r.schema)).Scan(&hasTable); err != nil {
		return false, err
	}
	if !hasTable {
		return false, nil
	}
	var allowed bool
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT EXISTS (
			SELECT 1
			FROM %s.customer_service_capabilities
			WHERE customer_id=$1
			  AND enabled=true
			  AND capability_code IN ('direct_ship','product_order')
			  AND lower(COALESCE(config_json->>'public_sku_aliases','false'))='true'
		)
	`, r.schema), customerID).Scan(&allowed)
	return allowed, err
}

func (r *Repository) listCustodyItemOptions(ctx context.Context, customerID int64) ([]app.CustodyItemOption, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT item_id, item_type, item_name, spec, quantity_g, quantity_units
		FROM %s.customer_custody_balances
		WHERE customer_id=$1
		ORDER BY item_type, item_name, spec
		LIMIT 300
	`, r.schema), customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]app.CustodyItemOption, 0)
	for rows.Next() {
		var row app.CustodyItemOption
		if err := rows.Scan(&row.ItemID, &row.ItemType, &row.ItemName, &row.Spec, &row.QuantityG, &row.QuantityUnits); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repository) listEmployeeOptions(ctx context.Context) ([]app.EmployeeOption, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT e.id, COALESCE(e.name,''), COALESCE(e.phone,''), COALESCE(d.name,''), e.active
		FROM %s.company_employees e
		LEFT JOIN %s.company_departments d ON d.id=e.department_id
		WHERE e.active=true
		ORDER BY e.id DESC
		LIMIT 500
	`, r.schema, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]app.EmployeeOption, 0)
	for rows.Next() {
		var row app.EmployeeOption
		if err := rows.Scan(&row.ID, &row.Name, &row.Phone, &row.Department, &row.Active); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repository) listRecipientOptions(ctx context.Context, customerID int64) ([]app.RecipientOption, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT COALESCE(payload->>'receiver_name',''),
		       COALESCE(payload->>'receiver_phone',''),
		       COALESCE(payload->>'receiver_company',''),
		       receiver_address,
		       external_order_no,
		       to_char(created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.customer_direct_ship_import_orders
		WHERE customer_id=$1
		  AND receiver_address <> ''
		ORDER BY created_at DESC, id DESC
		LIMIT 200
	`, r.schema), customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]app.RecipientOption, 0)
	seen := map[string]struct{}{}
	for rows.Next() {
		var row app.RecipientOption
		if err := rows.Scan(&row.ReceiverName, &row.ReceiverPhone, &row.ReceiverCompany, &row.ReceiverAddress, &row.LastOrderNo, &row.LastUsedAt); err != nil {
			return nil, err
		}
		row.ReceiverName = strings.TrimSpace(row.ReceiverName)
		row.ReceiverPhone = strings.TrimSpace(row.ReceiverPhone)
		row.ReceiverCompany = strings.TrimSpace(row.ReceiverCompany)
		row.ReceiverAddress = strings.Join(strings.Fields(strings.TrimSpace(row.ReceiverAddress)), " ")
		if row.ReceiverAddress == "" {
			continue
		}
		key := row.ReceiverPhone + "|" + row.ReceiverAddress
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, row)
	}
	return out, rows.Err()
}

type importRowRecord struct {
	ID          int64
	RowType     string
	ExternalKey string
	Payload     map[string]any
}

type applyTarget struct {
	TargetType       string
	TargetID         int64
	ProcessingOrders int
	DirectShipOrders int
	FeeItems         int
}

func (r *Repository) loadValidImportRowsTx(ctx context.Context, tx pgx.Tx, batchID int64) ([]importRowRecord, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id, row_type, external_key, payload
		FROM %s.customer_fulfillment_import_rows
		WHERE batch_id=$1 AND status='valid'
		ORDER BY id
	`, r.schema), batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]importRowRecord, 0)
	for rows.Next() {
		var row importRowRecord
		var payloadJSON []byte
		if err := rows.Scan(&row.ID, &row.RowType, &row.ExternalKey, &payloadJSON); err != nil {
			return nil, err
		}
		if len(payloadJSON) > 0 {
			if err := json.Unmarshal(payloadJSON, &row.Payload); err != nil {
				return nil, err
			}
		}
		if row.Payload == nil {
			row.Payload = map[string]any{}
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Repository) markImportRowAppliedTx(ctx context.Context, tx pgx.Tx, rowID int64, targetType string, targetID int64) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.customer_fulfillment_import_rows
		SET status='applied', target_type=$2, target_id=$3, applied_at=now()
		WHERE id=$1
	`, r.schema), rowID, targetType, targetID)
	return err
}

func (r *Repository) applyProcessingImportRow(ctx context.Context, tx pgx.Tx, customerID, batchID int64, row importRowRecord) (applyTarget, error) {
	switch row.RowType {
	case "customer_sku":
		id, err := r.upsertCustomerProductTx(ctx, tx, customerID, row.Payload)
		return applyTarget{TargetType: "product", TargetID: id}, err
	case "raw_bean_receipt":
		id, err := r.applyRawBeanMovementTx(ctx, tx, customerID, row, "receipt", 1)
		return applyTarget{TargetType: "customer_custody_item", TargetID: id}, err
	case "raw_bean_issue":
		id, err := r.applyRawBeanMovementTx(ctx, tx, customerID, row, "issue", -1)
		return applyTarget{TargetType: "customer_custody_item", TargetID: id}, err
	case "raw_bean_balance":
		id, err := r.applyRawBeanBalanceTx(ctx, tx, customerID, row)
		return applyTarget{TargetType: "customer_custody_balance", TargetID: id}, err
	case "packaging_balance":
		id, err := r.applyPackagingBalanceTx(ctx, tx, customerID, row)
		return applyTarget{TargetType: "customer_custody_balance", TargetID: id}, err
	case "processing_work_order":
		id, err := r.applyProcessingWorkOrderTx(ctx, tx, customerID, batchID, row)
		return applyTarget{TargetType: "customer_processing_work_order", TargetID: id, ProcessingOrders: 1}, err
	case "packaging_job":
		id, err := r.applyPackagingJobTx(ctx, tx, customerID, batchID, row)
		return applyTarget{TargetType: "customer_processing_packaging_job", TargetID: id}, err
	case "conversion_job":
		id, err := r.applyConversionJobTx(ctx, tx, customerID, batchID, row)
		return applyTarget{TargetType: "customer_inventory_conversion_job", TargetID: id}, err
	default:
		return applyTarget{}, nil
	}
}

func (r *Repository) upsertCustomerProductTx(ctx context.Context, tx pgx.Tx, customerID int64, payload map[string]any) (int64, error) {
	name := payloadString(payload, "sku_name", "product_name", "name")
	if name == "" {
		return 0, fmt.Errorf("customer sku name required")
	}
	roastDegree := payloadString(payload, "roast_degree")
	var id int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id FROM %s.products
		WHERE customer_id=$1 AND name=$2 AND visibility='customer_only' AND active=true
		ORDER BY id
		LIMIT 1
	`, r.schema), customerID, name).Scan(&id)
	if err == nil {
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.products
			SET roast_level=$2, custom_type='processing_customer_sku'
			WHERE id=$1
		`, r.schema), id, roastDegree)
		return id, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(name, roast_level, default_price, active, customer_id, visibility, custom_type, created_at)
		VALUES($1,$2,0,true,$3,'customer_only','processing_customer_sku',now())
		RETURNING id
	`, r.schema), name, roastDegree, customerID).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *Repository) applyRawBeanMovementTx(ctx context.Context, tx pgx.Tx, customerID int64, row importRowRecord, movementType string, sign int64) (int64, error) {
	name := payloadString(row.Payload, "raw_bean_name", "item_name")
	if name == "" {
		return 0, fmt.Errorf("raw bean name required")
	}
	qtyG := payloadInt64(row.Payload, "quantity_g")
	itemID, err := r.upsertCustodyItemTx(ctx, tx, customerID, "raw_bean", name, name, "g", row.Payload)
	if err != nil {
		return 0, err
	}
	deltaG := qtyG * sign
	if err := r.insertCustodyLedgerOnceTx(ctx, tx, customerID, itemID, "raw_bean", row.RowType, row.ExternalKey, movementType, deltaG, 0); err != nil {
		return 0, err
	}
	if _, err := r.addCustodyBalanceTx(ctx, tx, customerID, itemID, "raw_bean", name, "", deltaG, 0); err != nil {
		return 0, err
	}
	return itemID, nil
}

func (r *Repository) applyRawBeanBalanceTx(ctx context.Context, tx pgx.Tx, customerID int64, row importRowRecord) (int64, error) {
	name := payloadString(row.Payload, "raw_bean_name", "item_name")
	if name == "" {
		return 0, fmt.Errorf("raw bean name required")
	}
	qtyG := payloadInt64(row.Payload, "quantity_g")
	itemID, err := r.upsertCustodyItemTx(ctx, tx, customerID, "raw_bean", name, name, "g", row.Payload)
	if err != nil {
		return 0, err
	}
	currentG, currentUnits, err := r.currentCustodyBalanceTx(ctx, tx, customerID, itemID, "raw_bean")
	if err != nil {
		return 0, err
	}
	if err := r.insertCustodyLedgerOnceTx(ctx, tx, customerID, itemID, "raw_bean", row.RowType, row.ExternalKey, "balance_adjustment", qtyG-currentG, -currentUnits); err != nil {
		return 0, err
	}
	return r.setCustodyBalanceTx(ctx, tx, customerID, itemID, "raw_bean", name, "", qtyG, 0)
}

func (r *Repository) applyPackagingBalanceTx(ctx context.Context, tx pgx.Tx, customerID int64, row importRowRecord) (int64, error) {
	name := payloadString(row.Payload, "packaging_name", "item_name")
	if name == "" {
		return 0, fmt.Errorf("packaging name required")
	}
	units := payloadInt64(row.Payload, "quantity_units")
	itemID, err := r.upsertCustodyItemTx(ctx, tx, customerID, "packaging", name, name, "unit", row.Payload)
	if err != nil {
		return 0, err
	}
	currentG, currentUnits, err := r.currentCustodyBalanceTx(ctx, tx, customerID, itemID, "packaging")
	if err != nil {
		return 0, err
	}
	if err := r.insertCustodyLedgerOnceTx(ctx, tx, customerID, itemID, "packaging", row.RowType, row.ExternalKey, "balance_adjustment", -currentG, units-currentUnits); err != nil {
		return 0, err
	}
	return r.setCustodyBalanceTx(ctx, tx, customerID, itemID, "packaging", name, "", 0, units)
}

func (r *Repository) applyProcessingWorkOrderTx(ctx context.Context, tx pgx.Tx, customerID, batchID int64, row importRowRecord) (int64, error) {
	workOrderNo := payloadString(row.Payload, "work_order_no")
	productName := payloadString(row.Payload, "product_name")
	rawBeanName := payloadString(row.Payload, "raw_bean_name")
	if workOrderNo == "" || productName == "" {
		return 0, fmt.Errorf("work order no and product name required")
	}
	productID, _ := r.findCustomerProductIDTx(ctx, tx, customerID, productName)
	inputQtyG := payloadInt64(row.Payload, "input_quantity_g")
	plannedUnits := payloadInt64(row.Payload, "planned_output_units")
	var id int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_processing_work_orders(
			batch_id, customer_id, external_key, work_order_no, order_date,
			product_id, product_name, status, input_quantity_g, planned_output_units, payload
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb)
		ON CONFLICT (customer_id, external_key) WHERE external_key <> '' DO UPDATE SET
			batch_id=excluded.batch_id,
			work_order_no=excluded.work_order_no,
			order_date=excluded.order_date,
			product_id=excluded.product_id,
			product_name=excluded.product_name,
			status=excluded.status,
			input_quantity_g=excluded.input_quantity_g,
			planned_output_units=excluded.planned_output_units,
			payload=excluded.payload,
			updated_at=now()
		RETURNING id
	`, r.schema),
		batchID,
		customerID,
		row.ExternalKey,
		workOrderNo,
		parseDateValue(payloadString(row.Payload, "date")),
		productID,
		productName,
		payloadString(row.Payload, "status"),
		inputQtyG,
		plannedUnits,
		mustPayloadJSON(row.Payload),
	).Scan(&id); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.customer_processing_work_order_inputs WHERE work_order_id=$1`, r.schema), id); err != nil {
		return 0, err
	}
	if rawBeanName != "" || inputQtyG != 0 {
		rawItemID, err := r.upsertCustodyItemTx(ctx, tx, customerID, "raw_bean", rawBeanName, rawBeanName, "g", map[string]any{"raw_bean_name": rawBeanName})
		if err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.customer_processing_work_order_inputs(work_order_id, raw_bean_item_id, raw_bean_name, quantity_g, payload)
			VALUES($1,$2,$3,$4,$5::jsonb)
		`, r.schema), id, rawItemID, rawBeanName, inputQtyG, mustPayloadJSON(row.Payload)); err != nil {
			return 0, err
		}
	}
	return id, nil
}

func (r *Repository) applyPackagingJobTx(ctx context.Context, tx pgx.Tx, customerID, batchID int64, row importRowRecord) (int64, error) {
	packagingName := payloadString(row.Payload, "packaging_name")
	if packagingName == "" {
		return 0, fmt.Errorf("packaging name required")
	}
	if _, err := r.upsertCustodyItemTx(ctx, tx, customerID, "packaging", packagingName, packagingName, "unit", row.Payload); err != nil {
		return 0, err
	}
	var id int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_processing_packaging_jobs(
			batch_id, customer_id, external_key, work_order_no, product_name, packaging_name, quantity_units, payload
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8::jsonb)
		ON CONFLICT (customer_id, external_key) WHERE external_key <> '' DO UPDATE SET
			batch_id=excluded.batch_id,
			work_order_no=excluded.work_order_no,
			product_name=excluded.product_name,
			packaging_name=excluded.packaging_name,
			quantity_units=excluded.quantity_units,
			payload=excluded.payload
		RETURNING id
	`, r.schema),
		batchID,
		customerID,
		row.ExternalKey,
		payloadString(row.Payload, "work_order_no"),
		payloadString(row.Payload, "product_name"),
		packagingName,
		payloadInt64(row.Payload, "quantity_units"),
		mustPayloadJSON(row.Payload),
	).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *Repository) applyConversionJobTx(ctx context.Context, tx pgx.Tx, customerID, batchID int64, row importRowRecord) (int64, error) {
	var id int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_inventory_conversion_jobs(
			batch_id, customer_id, external_key, job_no, from_product, to_product, quantity_units, payload
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8::jsonb)
		ON CONFLICT (customer_id, external_key) WHERE external_key <> '' DO UPDATE SET
			batch_id=excluded.batch_id,
			job_no=excluded.job_no,
			from_product=excluded.from_product,
			to_product=excluded.to_product,
			quantity_units=excluded.quantity_units,
			payload=excluded.payload
		RETURNING id
	`, r.schema),
		batchID,
		customerID,
		row.ExternalKey,
		payloadString(row.Payload, "job_no"),
		payloadString(row.Payload, "from_product"),
		payloadString(row.Payload, "to_product"),
		payloadInt64(row.Payload, "quantity_units"),
		mustPayloadJSON(row.Payload),
	).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *Repository) upsertCustodyItemTx(ctx context.Context, tx pgx.Tx, customerID int64, itemType, externalCode, itemName, unit string, payload map[string]any) (int64, error) {
	externalCode = strings.TrimSpace(externalCode)
	itemName = strings.TrimSpace(itemName)
	if itemName == "" {
		return 0, fmt.Errorf("custody item name required")
	}
	if externalCode == "" {
		externalCode = itemName
	}
	var id int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_custody_items(customer_id, item_type, external_code, item_name, unit, payload, updated_at)
		VALUES($1,$2,$3,$4,$5,$6::jsonb,now())
		ON CONFLICT (customer_id, item_type, external_code) WHERE external_code <> '' DO UPDATE SET
			item_name=excluded.item_name,
			unit=excluded.unit,
			payload=excluded.payload,
			updated_at=now()
		RETURNING id
	`, r.schema), customerID, itemType, externalCode, itemName, unit, mustPayloadJSON(payload)).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *Repository) insertCustodyLedgerOnceTx(ctx context.Context, tx pgx.Tx, customerID, itemID int64, itemType, sourceType, sourceExternalKey, movementType string, deltaG, deltaUnits int64) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_custody_ledger_entries(
			customer_id, item_id, item_type, source_type, source_external_key, movement_type, qty_g_delta, qty_units_delta
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (customer_id, source_type, source_external_key, item_id, movement_type)
			WHERE source_external_key <> ''
		DO NOTHING
	`, r.schema), customerID, itemID, itemType, sourceType, sourceExternalKey, movementType, deltaG, deltaUnits)
	return err
}

func (r *Repository) currentCustodyBalanceTx(ctx context.Context, tx pgx.Tx, customerID, itemID int64, itemType string) (int64, int64, error) {
	var qtyG, qtyUnits int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT quantity_g, quantity_units
		FROM %s.customer_custody_balances
		WHERE customer_id=$1 AND item_type=$2 AND item_id=$3
	`, r.schema), customerID, itemType, itemID).Scan(&qtyG, &qtyUnits)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, nil
	}
	return qtyG, qtyUnits, err
}

func (r *Repository) addCustodyBalanceTx(ctx context.Context, tx pgx.Tx, customerID, itemID int64, itemType, itemName, spec string, deltaG, deltaUnits int64) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_custody_balances(customer_id, item_id, item_type, item_name, spec, quantity_g, quantity_units, updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,now())
		ON CONFLICT (customer_id, item_type, item_id) DO UPDATE SET
			item_name=excluded.item_name,
			spec=excluded.spec,
			quantity_g=%s.customer_custody_balances.quantity_g + excluded.quantity_g,
			quantity_units=%s.customer_custody_balances.quantity_units + excluded.quantity_units,
			updated_at=now()
		RETURNING id
	`, r.schema, r.schema, r.schema), customerID, itemID, itemType, itemName, spec, deltaG, deltaUnits).Scan(&id)
	return id, err
}

func (r *Repository) setCustodyBalanceTx(ctx context.Context, tx pgx.Tx, customerID, itemID int64, itemType, itemName, spec string, qtyG, qtyUnits int64) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_custody_balances(customer_id, item_id, item_type, item_name, spec, quantity_g, quantity_units, updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,now())
		ON CONFLICT (customer_id, item_type, item_id) DO UPDATE SET
			item_name=excluded.item_name,
			spec=excluded.spec,
			quantity_g=excluded.quantity_g,
			quantity_units=excluded.quantity_units,
			updated_at=now()
		RETURNING id
	`, r.schema), customerID, itemID, itemType, itemName, spec, qtyG, qtyUnits).Scan(&id)
	return id, err
}

func (r *Repository) findCustomerProductIDTx(ctx context.Context, tx pgx.Tx, customerID int64, name string) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id FROM %s.products
		WHERE customer_id=$1 AND name=$2 AND visibility='customer_only' AND active=true
		ORDER BY id
		LIMIT 1
	`, r.schema), customerID, strings.TrimSpace(name)).Scan(&id)
	return id, err
}

type directShipApplyState struct {
	importOrderIDs map[string]int64
	orderIDs       map[string]int64
	lineNoByOrder  map[string]int
}

func newDirectShipApplyState() *directShipApplyState {
	return &directShipApplyState{
		importOrderIDs: map[string]int64{},
		orderIDs:       map[string]int64{},
		lineNoByOrder:  map[string]int{},
	}
}

func (r *Repository) applyDirectShipImportRow(ctx context.Context, tx pgx.Tx, customerID, batchID int64, state *directShipApplyState, row importRowRecord) (applyTarget, error) {
	switch row.RowType {
	case "direct_ship_order":
		importOrderID, orderID, err := r.applyDirectShipOrderTx(ctx, tx, customerID, batchID, row)
		if err != nil {
			return applyTarget{}, err
		}
		orderNo := payloadString(row.Payload, "order_no")
		if orderNo != "" {
			state.importOrderIDs[orderNo] = importOrderID
			state.orderIDs[orderNo] = orderID
		}
		return applyTarget{TargetType: "customer_direct_ship_import_order", TargetID: importOrderID, DirectShipOrders: 1}, nil
	case "direct_ship_item":
		itemID, err := r.applyDirectShipItemTx(ctx, tx, customerID, batchID, state, row)
		return applyTarget{TargetType: "customer_direct_ship_import_order_item", TargetID: itemID}, err
	default:
		return applyTarget{}, nil
	}
}

func (r *Repository) applyDirectShipOrderTx(ctx context.Context, tx pgx.Tx, customerID, batchID int64, row importRowRecord) (int64, int64, error) {
	orderNo := payloadString(row.Payload, "order_no")
	if orderNo == "" {
		return 0, 0, fmt.Errorf("direct ship order no required")
	}
	seq := payloadString(row.Payload, "sequence_no")
	receiverName, receiverPhone, receiverAddress := parseReceiverSnapshot(payloadString(row.Payload, "receiver_address"))
	warehouse, err := r.customerProcessingWarehouseTx(ctx, tx, customerID)
	if err != nil {
		return 0, 0, err
	}
	waybillNo := trackingSummaryFromCustomerFulfillment(payloadString(row.Payload, "waybill_no"))
	var importOrderID, orderID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_direct_ship_import_orders(
			batch_id, customer_id, external_order_no, external_seq, order_date,
			receiver_address, status, payload
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8::jsonb)
		ON CONFLICT (customer_id, external_order_no, external_seq) WHERE external_order_no <> '' DO UPDATE SET
			batch_id=excluded.batch_id,
			order_date=excluded.order_date,
			receiver_address=excluded.receiver_address,
			status=excluded.status,
			payload=excluded.payload
		RETURNING id, order_id
	`, r.schema),
		batchID,
		customerID,
		orderNo,
		seq,
		parseDateValue(payloadString(row.Payload, "order_date")),
		payloadString(row.Payload, "receiver_address"),
		payloadString(row.Payload, "status"),
		mustPayloadJSON(row.Payload),
	).Scan(&importOrderID, &orderID); err != nil {
		return 0, 0, err
	}
	if orderID <= 0 {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.orders(
				order_no, order_date, customer_id, portal_service_code,
				receiver_name, receiver_phone, receiver_address,
				ship_tracking_no, source_warehouse, notes, created_at
			)
			VALUES($1,$2,$3,'direct_ship',$4,$5,$6,$7,$8,$9,now())
			RETURNING id
		`, r.schema),
			orderNo,
			parseDateValue(payloadString(row.Payload, "order_date")),
			customerID,
			receiverName,
			receiverPhone,
			receiverAddress,
			waybillNo,
			warehouse,
			payloadString(row.Payload, "remark"),
		).Scan(&orderID); err != nil {
			return 0, 0, err
		}
	} else {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.orders
			SET receiver_name=$2,
				receiver_phone=$3,
				receiver_address=$4,
				ship_tracking_no=CASE WHEN $5 <> '' THEN $5 ELSE ship_tracking_no END,
				source_warehouse=$6,
				portal_service_code='direct_ship'
			WHERE id=$1
		`, r.schema), orderID, receiverName, receiverPhone, receiverAddress, waybillNo, warehouse); err != nil {
			return 0, 0, err
		}
	}
	if waybillNo != "" {
		if _, err := upsertCustomerFulfillmentOrderTrackingsTx(ctx, tx, r.schema, orderID, waybillNo, "customer_fulfillment_direct_ship"); err != nil {
			return 0, 0, err
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.customer_direct_ship_import_orders SET order_id=$2 WHERE id=$1
	`, r.schema), importOrderID, orderID); err != nil {
		return 0, 0, err
	}
	return importOrderID, orderID, nil
}

func (r *Repository) applyDirectShipItemTx(ctx context.Context, tx pgx.Tx, customerID, batchID int64, state *directShipApplyState, row importRowRecord) (int64, error) {
	orderNo := payloadString(row.Payload, "order_no")
	if orderNo == "" {
		return 0, fmt.Errorf("direct ship item order no required")
	}
	importOrderID := state.importOrderIDs[orderNo]
	orderID := state.orderIDs[orderNo]
	if importOrderID <= 0 || orderID <= 0 {
		var seq string
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT id, order_id, external_seq
			FROM %s.customer_direct_ship_import_orders
			WHERE customer_id=$1 AND external_order_no=$2
			ORDER BY id DESC
			LIMIT 1
		`, r.schema), customerID, orderNo).Scan(&importOrderID, &orderID, &seq); err != nil {
			return 0, err
		}
		state.importOrderIDs[orderNo] = importOrderID
		state.orderIDs[orderNo] = orderID
	}
	if orderID <= 0 {
		return 0, fmt.Errorf("direct ship order not applied")
	}
	state.lineNoByOrder[orderNo]++
	lineNo := state.lineNoByOrder[orderNo]
	productTitle := payloadString(row.Payload, "product_title", "product_name")
	var productID any
	matchedProductID, productErr := r.findProductForDirectShipTx(ctx, tx, customerID, productTitle)
	if productErr == nil {
		productID = matchedProductID
	} else if !errors.Is(productErr, pgx.ErrNoRows) {
		return 0, productErr
	}
	quantity := payloadInt64(row.Payload, "quantity_units")
	var importItemID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_direct_ship_import_order_items(
			import_order_id, batch_id, customer_id, line_no, product_title, spec, quantity_units, payload
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8::jsonb)
		RETURNING id
	`, r.schema),
		importOrderID,
		batchID,
		customerID,
		lineNo,
		productTitle,
		payloadString(row.Payload, "spec"),
		quantity,
		mustPayloadJSON(row.Payload),
	).Scan(&importItemID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.order_items(order_id, line_no, product_id, item_name, qty, unit, spec, unit_price, line_total)
		VALUES($1,$2,$3,$4,$5,'件',$6,0,0)
	`, r.schema), orderID, lineNo, productID, productTitle, quantity, payloadString(row.Payload, "spec")); err != nil {
		return 0, err
	}
	if waybillNo := payloadString(row.Payload, "waybill_no"); waybillNo != "" {
		if _, err := upsertCustomerFulfillmentOrderTrackingsTx(ctx, tx, r.schema, orderID, waybillNo, "customer_fulfillment_direct_ship_item"); err != nil {
			return 0, err
		}
	}
	return importItemID, nil
}

func upsertCustomerFulfillmentOrderTrackingsTx(ctx context.Context, tx pgx.Tx, schema string, orderID int64, raw, source string) (string, error) {
	numbers := normalizeCustomerFulfillmentTrackings(raw)
	for _, no := range numbers {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.order_shipping_trackings(order_id, tracking_no, source, created_by)
			VALUES($1,$2,$3,'customer_fulfillment')
			ON CONFLICT (order_id, tracking_no) DO NOTHING
		`, schema), orderID, no, source); err != nil {
			return "", err
		}
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT tracking_no
		FROM %s.order_shipping_trackings
		WHERE order_id=$1
		ORDER BY id
	`, schema), orderID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	all := make([]string, 0)
	for rows.Next() {
		var no string
		if err := rows.Scan(&no); err != nil {
			return "", err
		}
		all = append(all, no)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	summary := strings.Join(all, "\n")
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET ship_tracking_no=$2 WHERE id=$1`, schema), orderID, summary); err != nil {
		return "", err
	}
	return summary, nil
}

func trackingSummaryFromCustomerFulfillment(raw string) string {
	return strings.Join(normalizeCustomerFulfillmentTrackings(raw), "\n")
}

func normalizeCustomerFulfillmentTrackings(raw string) []string {
	parts := strings.FieldsFunc(strings.TrimSpace(raw), func(r rune) bool {
		switch r {
		case ',', ';', '，', '；', '、', '\n', '\r', '\t', ' ':
			return true
		default:
			return false
		}
	})
	seen := make(map[string]bool, len(parts))
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		no := strings.TrimSpace(part)
		if no == "" || seen[no] {
			continue
		}
		seen[no] = true
		out = append(out, no)
	}
	return out
}

func (r *Repository) customerProcessingWarehouseTx(ctx context.Context, tx pgx.Tx, customerID int64) (string, error) {
	var warehouse string
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT processing_warehouse_code
		FROM %s.customer_portal_profiles
		WHERE customer_id=$1
	`, r.schema), customerID).Scan(&warehouse)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	warehouse = strings.TrimSpace(warehouse)
	if warehouse == "" {
		warehouse = fmt.Sprintf("cust_%d_processing", customerID)
	}
	return warehouse, nil
}

func (r *Repository) findProductForDirectShipTx(ctx context.Context, tx pgx.Tx, customerID int64, name string) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id
		FROM %s.products
		WHERE active=true AND name=$1 AND (customer_id=0 OR customer_id=$2)
		ORDER BY CASE WHEN customer_id=$2 THEN 0 ELSE 1 END, id
		LIMIT 1
	`, r.schema), strings.TrimSpace(name), customerID).Scan(&id)
	return id, err
}

func parseReceiverSnapshot(raw string) (string, string, string) {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return "", "", ""
	}
	phoneIndex := -1
	for i, field := range fields {
		if isChineseMobileLike(field) {
			phoneIndex = i
			break
		}
	}
	if phoneIndex < 0 {
		return "", "", strings.TrimSpace(raw)
	}
	name := strings.Join(fields[:phoneIndex], " ")
	phone := fields[phoneIndex]
	address := strings.Join(fields[phoneIndex+1:], " ")
	return name, phone, address
}

func isChineseMobileLike(value string) bool {
	if len(value) != 11 || !strings.HasPrefix(value, "1") {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (r *Repository) applySettlementImportRow(ctx context.Context, tx pgx.Tx, customerID int64, row importRowRecord) (applyTarget, error) {
	if row.RowType != "fee_item" {
		return applyTarget{}, nil
	}
	mappedFeeType := mapSettlementFeeType(payloadString(row.Payload, "fee_type"))
	if mappedFeeType == "" {
		return applyTarget{}, fmt.Errorf("fee type invalid")
	}
	var existingID int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id
		FROM %s.customer_fee_items
		WHERE source_type='customer_fulfillment_import' AND source_id=$1
	`, r.schema), row.ID).Scan(&existingID)
	if err == nil {
		return applyTarget{TargetType: "customer_fee_item", TargetID: existingID, FeeItems: 1}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return applyTarget{}, err
	}
	amountCents := payloadInt64(row.Payload, "amount_cents")
	amount := fmt.Sprintf("%.2f", float64(amountCents)/100)
	occurredAt := parseDateValue(payloadString(row.Payload, "date"))
	if occurredAt == nil {
		occurredAt = time.Now()
	}
	note := payloadString(row.Payload, "fee_name")
	var feeID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_fee_items(
			customer_id, source_type, source_id, fee_type, amount, occurred_at, status, note
		)
		VALUES($1,'customer_fulfillment_import',$2,$3,$4,$5,'unsettled',$6)
		RETURNING id
	`, r.schema), customerID, row.ID, mappedFeeType, amount, occurredAt, note).Scan(&feeID); err != nil {
		return applyTarget{}, err
	}
	return applyTarget{TargetType: "customer_fee_item", TargetID: feeID, FeeItems: 1}, nil
}

func mapSettlementFeeType(feeType string) string {
	switch strings.TrimSpace(feeType) {
	case "roasting", "grinding", "drip_bag":
		return "processing"
	case "bagging", "boxing", "packaging":
		return "packaging"
	case "direct_ship_service":
		return "direct_ship_service"
	case "storage":
		return "storage"
	case "shipping":
		return "shipping"
	case "adjustment":
		return "adjustment"
	default:
		return ""
	}
}

func (r *Repository) CreateSettlement(ctx context.Context, cmd app.CreateSettlementCommand) (app.SettlementResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.SettlementResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	periodFrom, err := time.Parse("2006-01-02", cmd.PeriodFrom)
	if err != nil {
		return app.SettlementResult{}, fmt.Errorf("period from invalid")
	}
	periodTo, err := time.Parse("2006-01-02", cmd.PeriodTo)
	if err != nil {
		return app.SettlementResult{}, fmt.Errorf("period to invalid")
	}
	settlementNo := fmt.Sprintf("CS-%d-%s-%s", cmd.CustomerID, periodFrom.Format("20060102"), periodTo.Format("20060102"))

	var batchID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_settlement_batches(customer_id, settlement_no, period_from, period_to, status, total_amount, created_by)
		VALUES($1,$2,$3,$4,'draft',0,$5)
		ON CONFLICT (settlement_no) WHERE settlement_no <> '' DO UPDATE SET
			period_from=excluded.period_from,
			period_to=excluded.period_to,
			created_by=excluded.created_by
		RETURNING id
	`, r.schema), cmd.CustomerID, settlementNo, periodFrom, periodTo, strings.TrimSpace(cmd.CreatedBy)).Scan(&batchID); err != nil {
		return app.SettlementResult{}, err
	}

	var feeItems int
	var totalCents int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*), COALESCE(ROUND(SUM(amount) * 100),0)::bigint
		FROM %s.customer_fee_items
		WHERE customer_id=$1
			AND status='unsettled'
			AND settlement_batch_id=0
			AND occurred_at::date BETWEEN $2 AND $3
	`, r.schema), cmd.CustomerID, periodFrom, periodTo).Scan(&feeItems, &totalCents); err != nil {
		return app.SettlementResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.customer_fee_items
		SET settlement_batch_id=$4, status='settled'
		WHERE customer_id=$1
			AND status='unsettled'
			AND settlement_batch_id=0
			AND occurred_at::date BETWEEN $2 AND $3
	`, r.schema), cmd.CustomerID, periodFrom, periodTo, batchID); err != nil {
		return app.SettlementResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.customer_settlement_batches
		SET total_amount=$2
		WHERE id=$1
	`, r.schema), batchID, fmt.Sprintf("%.2f", float64(totalCents)/100)); err != nil {
		return app.SettlementResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return app.SettlementResult{}, err
	}
	return app.SettlementResult{
		BatchID:          batchID,
		CustomerID:       cmd.CustomerID,
		PeriodFrom:       periodFrom.Format("2006-01-02"),
		PeriodTo:         periodTo.Format("2006-01-02"),
		FeeItems:         feeItems,
		TotalAmountCents: totalCents,
	}, nil
}

func (r *Repository) Overview(ctx context.Context, query app.OverviewQuery) (app.Overview, error) {
	var overview app.Overview
	overview.CustomerID = query.CustomerID
	_ = r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT name FROM %s.customers WHERE id=$1`, r.schema), query.CustomerID).Scan(&overview.CustomerName)
	imports, err := r.ListImports(ctx, app.ListImportsQuery{CustomerID: query.CustomerID, Limit: 20})
	if err != nil {
		return app.Overview{}, err
	}
	overview.Imports = imports
	if overview.CustodyBalances, err = r.listCustodyBalances(ctx, query.CustomerID); err != nil {
		return app.Overview{}, err
	}
	if overview.FinishedGoods, err = r.listFinishedGoods(ctx, query.CustomerID); err != nil {
		return app.Overview{}, err
	}
	if overview.ProcessingOrders, err = r.listProcessingOrders(ctx, query.CustomerID); err != nil {
		return app.Overview{}, err
	}
	if overview.DirectShipOrders, err = r.listDirectShipOrders(ctx, query.CustomerID); err != nil {
		return app.Overview{}, err
	}
	if overview.Fees, err = r.listFeeItems(ctx, query.CustomerID); err != nil {
		return app.Overview{}, err
	}
	if overview.Settlements, err = r.listSettlements(ctx, query.CustomerID); err != nil {
		return app.Overview{}, err
	}
	return overview, nil
}

func (r *Repository) listCustodyBalances(ctx context.Context, customerID int64) ([]app.CustodyBalance, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT item_type, item_name, spec, quantity_g, quantity_units
		FROM %s.customer_custody_balances
		WHERE customer_id=$1
		ORDER BY item_type, item_name
		LIMIT 200
	`, r.schema), customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]app.CustodyBalance, 0)
	for rows.Next() {
		var row app.CustodyBalance
		if err := rows.Scan(&row.ItemType, &row.ItemName, &row.Spec, &row.QuantityG, &row.QuantityUnits); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repository) listFinishedGoods(ctx context.Context, customerID int64) ([]app.FinishedGoodsBalance, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT item_id, item_name, spec_g, warehouse, qty_g, qty_units, status
		FROM %s.customer_inventory_items
		WHERE customer_id=$1 AND item_type IN ('product','finished_goods')
		ORDER BY item_name, spec_g, warehouse, id
		LIMIT 200
	`, r.schema), customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]app.FinishedGoodsBalance, 0)
	for rows.Next() {
		var row app.FinishedGoodsBalance
		if err := rows.Scan(&row.ProductID, &row.ProductName, &row.SpecG, &row.Warehouse, &row.QuantityG, &row.QuantityUnits, &row.Status); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repository) listProcessingOrders(ctx context.Context, customerID int64) ([]app.ProcessingOrderSummary, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT work_order_no, product_name, status, input_quantity_g, planned_output_units
		FROM %s.customer_processing_work_orders
		WHERE customer_id=$1
		ORDER BY created_at DESC, id DESC
		LIMIT 100
	`, r.schema), customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]app.ProcessingOrderSummary, 0)
	for rows.Next() {
		var row app.ProcessingOrderSummary
		if err := rows.Scan(&row.WorkOrderNo, &row.ProductName, &row.Status, &row.QuantityG, &row.Units); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repository) listDirectShipOrders(ctx context.Context, customerID int64) ([]app.DirectShipOrderSummary, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT o.external_order_no,
			COALESCE(to_char(o.order_date, 'YYYY-MM-DD'), ''),
			o.receiver_address,
			o.status,
			COUNT(i.id)::int
		FROM %s.customer_direct_ship_import_orders o
		LEFT JOIN %s.customer_direct_ship_import_order_items i ON i.import_order_id=o.id
		WHERE o.customer_id=$1
		GROUP BY o.id
		ORDER BY o.created_at DESC, o.id DESC
		LIMIT 100
	`, r.schema, r.schema), customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]app.DirectShipOrderSummary, 0)
	for rows.Next() {
		var row app.DirectShipOrderSummary
		if err := rows.Scan(&row.OrderNo, &row.OrderDate, &row.ReceiverAddress, &row.Status, &row.ItemCount); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repository) listFeeItems(ctx context.Context, customerID int64) ([]app.FeeItemSummary, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT fee_type, note, ROUND(amount * 100)::bigint, source_type
		FROM %s.customer_fee_items
		WHERE customer_id=$1
		ORDER BY occurred_at DESC, id DESC
		LIMIT 100
	`, r.schema), customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]app.FeeItemSummary, 0)
	for rows.Next() {
		var row app.FeeItemSummary
		if err := rows.Scan(&row.FeeType, &row.FeeName, &row.AmountCents, &row.Source); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repository) listSettlements(ctx context.Context, customerID int64) ([]app.SettlementSummary, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,
			COALESCE(to_char(period_from, 'YYYY-MM-DD'), ''),
			COALESCE(to_char(period_to, 'YYYY-MM-DD'), ''),
			status,
			ROUND(total_amount * 100)::bigint
		FROM %s.customer_settlement_batches
		WHERE customer_id=$1
		ORDER BY created_at DESC, id DESC
		LIMIT 100
	`, r.schema), customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]app.SettlementSummary, 0)
	for rows.Next() {
		var row app.SettlementSummary
		if err := rows.Scan(&row.BatchID, &row.PeriodFrom, &row.PeriodTo, &row.Status, &row.TotalAmountCents); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repository) ListImports(ctx context.Context, query app.ListImportsQuery) ([]app.ImportBatch, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	if query.Offset < 0 {
		query.Offset = 0
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, customer_id, import_type, source_filename, source_sha256, status,
			total_rows, valid_rows, invalid_rows, summary_json, created_by, created_at, applied_at
		FROM %s.customer_fulfillment_import_batches
		WHERE customer_id=$1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`, r.schema), query.CustomerID, limit, query.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]app.ImportBatch, 0)
	for rows.Next() {
		batch, err := scanImportBatch(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, batch)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Repository) ImportBatch(ctx context.Context, batchID int64) (app.ImportBatch, error) {
	return scanImportBatch(r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, customer_id, import_type, source_filename, source_sha256, status,
			total_rows, valid_rows, invalid_rows, summary_json, created_by, created_at, applied_at
		FROM %s.customer_fulfillment_import_batches
		WHERE id=$1
	`, r.schema), batchID))
}

func (r *Repository) ListImportRows(ctx context.Context, query app.ListImportRowsQuery) ([]app.ImportRow, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 200
	}
	if limit > 500 {
		limit = 500
	}
	if query.Offset < 0 {
		query.Offset = 0
	}
	status := strings.TrimSpace(query.Status)
	args := []any{query.BatchID}
	statusWhere := ""
	if status != "" {
		statusWhere = " AND status=$2"
		args = append(args, status)
	}
	args = append(args, limit, query.Offset)
	limitArg := len(args) - 1
	offsetArg := len(args)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, batch_id, sheet_name, row_no, row_type, external_key, status, error
		FROM %s.customer_fulfillment_import_rows
		WHERE batch_id=$1%s
		ORDER BY row_no, id
		LIMIT $%d OFFSET $%d
	`, r.schema, statusWhere, limitArg, offsetArg), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]app.ImportRow, 0)
	for rows.Next() {
		var row app.ImportRow
		if err := rows.Scan(
			&row.ID,
			&row.BatchID,
			&row.SheetName,
			&row.RowNo,
			&row.RowType,
			&row.ExternalKey,
			&row.Status,
			&row.Error,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Repository) loadImportBatchTx(ctx context.Context, tx pgx.Tx, id int64) (app.ImportBatch, error) {
	return scanImportBatch(tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, customer_id, import_type, source_filename, source_sha256, status,
			total_rows, valid_rows, invalid_rows, summary_json, created_by, created_at, applied_at
		FROM %s.customer_fulfillment_import_batches
		WHERE id=$1
	`, r.schema), id))
}

type importBatchScanner interface {
	Scan(dest ...any) error
}

func scanImportBatch(row importBatchScanner) (app.ImportBatch, error) {
	var batch app.ImportBatch
	var importType string
	var summaryJSON []byte
	var createdAt time.Time
	var appliedAt *time.Time
	if err := row.Scan(
		&batch.ID,
		&batch.CustomerID,
		&importType,
		&batch.SourceFilename,
		&batch.SourceSHA256,
		&batch.Status,
		&batch.Summary.TotalRows,
		&batch.Summary.ValidRows,
		&batch.Summary.InvalidRows,
		&summaryJSON,
		&batch.CreatedBy,
		&createdAt,
		&appliedAt,
	); err != nil {
		return app.ImportBatch{}, err
	}
	batch.ImportType = app.ImportType(importType)
	var parsedSummary app.ImportSummary
	if len(summaryJSON) > 0 {
		_ = json.Unmarshal(summaryJSON, &parsedSummary)
	}
	parsedSummary.TotalRows = batch.Summary.TotalRows
	parsedSummary.ValidRows = batch.Summary.ValidRows
	parsedSummary.InvalidRows = batch.Summary.InvalidRows
	batch.Summary = parsedSummary
	batch.CreatedAt = createdAt.Format(time.RFC3339)
	if appliedAt != nil {
		batch.AppliedAt = appliedAt.Format(time.RFC3339)
	}
	return batch, nil
}

func payloadString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := payload[key]
		if !ok || value == nil {
			continue
		}
		switch v := value.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		case fmt.Stringer:
			if strings.TrimSpace(v.String()) != "" {
				return strings.TrimSpace(v.String())
			}
		default:
			text := strings.TrimSpace(fmt.Sprint(v))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func payloadInt64(payload map[string]any, key string) int64 {
	value, ok := payload[key]
	if !ok || value == nil {
		return 0
	}
	switch v := value.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case int32:
		return int64(v)
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	case json.Number:
		n, _ := v.Int64()
		return n
	case string:
		n, _ := parseIntText(v)
		return n
	default:
		n, _ := parseIntText(fmt.Sprint(v))
		return n
	}
}

func parseIntText(value string) (int64, bool) {
	value = strings.TrimSpace(strings.ReplaceAll(value, ",", ""))
	if value == "" {
		return 0, false
	}
	var integerText strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' || r == '-' {
			integerText.WriteRune(r)
			continue
		}
		if integerText.Len() > 0 {
			break
		}
	}
	if integerText.Len() == 0 {
		return 0, false
	}
	var n int64
	if _, err := fmt.Sscan(integerText.String(), &n); err != nil {
		return 0, false
	}
	return n, true
}

func parseDateValue(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	ts, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil
	}
	return ts
}

func mustPayloadJSON(payload map[string]any) string {
	if payload == nil {
		return "{}"
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func custodyUnitForItemType(itemType string) string {
	switch strings.TrimSpace(itemType) {
	case "raw_bean":
		return "g"
	default:
		return "unit"
	}
}
