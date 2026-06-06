package customerfulfillment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	app "orderapp/internal/application/customerfulfillment"
	customerportalapp "orderapp/internal/application/customerportal"
	catalogdomain "orderapp/internal/domain/catalog"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	"orderapp/internal/infrastructure/postgres/orderbeans"

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
	if capability := capabilityForImportType(app.ImportType(importType)); capability != "" {
		if err := r.requireCustomerCapability(ctx, customerID, capability); err != nil {
			return app.ApplyResult{}, err
		}
	}
	rows, err := r.loadValidImportRowsTx(ctx, tx, cmd.BatchID)
	if err != nil {
		return app.ApplyResult{}, err
	}
	result := app.ApplyResult{BatchID: cmd.BatchID}
	processingState := newProcessingApplyState()
	directShipState := newDirectShipApplyState()
	for _, row := range rows {
		var target applyTarget
		switch app.ImportType(importType) {
		case app.ImportTypeProcessingWorkbook:
			target, err = r.applyProcessingImportRow(ctx, tx, customerID, cmd.BatchID, processingState, row)
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
	if app.ImportType(importType) == app.ImportTypeProcessingWorkbook {
		if err := r.trimProcessingWorkOrderStaleInputsTx(ctx, tx, processingState); err != nil {
			return app.ApplyResult{}, err
		}
	}
	if app.ImportType(importType) == app.ImportTypeDirectShipWorkbook {
		if err := r.trimDirectShipStaleLinesTx(ctx, tx, directShipState); err != nil {
			return app.ApplyResult{}, err
		}
		if err := r.trimDirectShipStaleTrackingsTx(ctx, tx, directShipState); err != nil {
			return app.ApplyResult{}, err
		}
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
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT b.employee_id,
		       b.customer_id,
		       COALESCE(NULLIF(cp.display_name,''), c.name, ''),
		       b.role,
		       b.status,
		       COALESCE(cp.capability_template_key,'')
		FROM %s.customer_erp_user_bindings b
		JOIN %s.customers c ON c.id=b.customer_id
		JOIN %s.company_employees e ON e.id=b.employee_id
		LEFT JOIN %s.employee_login_passwords lp ON lp.employee_id=e.id
		LEFT JOIN %s.customer_portal_profiles cp ON cp.customer_id=b.customer_id
		WHERE b.employee_id=$1
		  AND b.status='active'
		  AND c.active=true
		  AND e.active=true
		  AND e.account_type='channel_customer'
		  AND COALESCE(lp.login_disabled,false)=false
		ORDER BY b.id
	`, r.schema, r.schema, r.schema, r.schema, r.schema), employeeID)
	if err != nil {
		return app.CustomerERPContext{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var row app.CustomerERPContext
		var templateKey string
		if err := rows.Scan(&row.EmployeeID, &row.CustomerID, &row.CustomerName, &row.BindingRole, &row.BindingStatus, &templateKey); err != nil {
			return app.CustomerERPContext{}, err
		}
		available, err := r.customerERPWorkbenchAvailableForTemplateKey(ctx, r.pool, templateKey)
		if err != nil {
			return app.CustomerERPContext{}, err
		}
		if !available {
			continue
		}
		return row, nil
	}
	if err := rows.Err(); err != nil {
		return app.CustomerERPContext{}, err
	}
	return app.CustomerERPContext{}, app.ErrCustomerERPBindingNotFound
}

func (r *Repository) requireActiveCustomerERPWorkbenchBinding(ctx context.Context, customerID int64) error {
	var templateKey string
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(cp.capability_template_key,'')
		FROM %s.customer_erp_user_bindings b
		JOIN %s.customers c ON c.id=b.customer_id
		JOIN %s.company_employees e ON e.id=b.employee_id
		LEFT JOIN %s.employee_login_passwords lp ON lp.employee_id=e.id
		LEFT JOIN %s.customer_portal_profiles cp ON cp.customer_id=b.customer_id
		WHERE b.customer_id=$1
		  AND b.status='active'
		  AND c.active=true
		  AND e.active=true
		  AND e.account_type='channel_customer'
		  AND COALESCE(lp.login_disabled,false)=false
		ORDER BY b.id
		LIMIT 1
	`, r.schema, r.schema, r.schema, r.schema, r.schema), customerID).Scan(&templateKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.ErrCustomerERPBindingNotFound
	}
	if err != nil {
		return err
	}
	available, err := r.customerERPWorkbenchAvailableForTemplateKey(ctx, r.pool, templateKey)
	if err != nil {
		return err
	}
	if !available {
		return app.ErrCustomerERPBindingNotFound
	}
	return nil
}

func (r *Repository) CustomerPortalOverview(ctx context.Context, employeeID int64) (app.CustomerPortalOverview, error) {
	current, err := r.CustomerPortalContext(ctx, employeeID)
	if err != nil {
		return app.CustomerPortalOverview{}, err
	}
	return r.buildOverview(ctx, current.CustomerID, current.CustomerName)
}

func (r *Repository) InternalCustomerPortalOverview(ctx context.Context, customerID int64) (app.CustomerPortalOverview, error) {
	customerName, err := r.resolveCustomerName(ctx, customerID)
	if err != nil {
		return app.CustomerPortalOverview{}, err
	}
	if err := r.requirePortalCustomerWithWorkbench(ctx, customerID); err != nil {
		return app.CustomerPortalOverview{}, err
	}
	return r.buildOverview(ctx, customerID, customerName)
}

func (r *Repository) InternalCustomerPortalOptions(ctx context.Context, customerID int64) (app.CustomerFulfillmentOptions, error) {
	if err := r.requirePortalCustomerWithWorkbench(ctx, customerID); err != nil {
		return app.CustomerFulfillmentOptions{}, err
	}
	return r.CustomerFulfillmentOptions(ctx, customerID)
}

func (r *Repository) resolveCustomerName(ctx context.Context, customerID int64) (string, error) {
	var name string
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(name,'')
		FROM %s.customers
		WHERE id=$1 AND active=true
	`, r.schema), customerID).Scan(&name)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", fmt.Errorf("customer not found")
		}
		return "", err
	}
	return name, nil
}

func (r *Repository) requirePortalCustomerWithWorkbench(ctx context.Context, customerID int64) error {
	var templateKey string
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(cp.capability_template_key,'')
		FROM %s.customer_portal_profiles cp
		JOIN %s.customers c ON c.id=cp.customer_id
		WHERE cp.customer_id=$1
		  AND cp.enabled=true
		  AND c.active=true
	`, r.schema, r.schema), customerID).Scan(&templateKey)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("customer portal not found")
		}
		return err
	}
	if templateKey == "" {
		return fmt.Errorf("customer portal template not configured")
	}
	available, err := r.customerERPWorkbenchAvailableForTemplateKey(ctx, r.pool, templateKey)
	if err != nil {
		return err
	}
	if !available {
		return fmt.Errorf("customer processing portal unavailable for this customer")
	}
	return nil
}

func (r *Repository) buildOverview(ctx context.Context, customerID int64, customerName string) (app.CustomerPortalOverview, error) {
	overview := app.CustomerPortalOverview{
		CustomerID:   customerID,
		CustomerName: customerName,
	}
	var err error
	if overview.Capabilities, err = r.listCustomerCapabilityCodes(ctx, customerID); err != nil {
		return app.CustomerPortalOverview{}, err
	}
	if overview.CustodyBalances, err = r.listCustodyBalances(ctx, customerID); err != nil {
		return app.CustomerPortalOverview{}, err
	}
	if overview.FinishedGoods, err = r.listFinishedGoods(ctx, customerID); err != nil {
		return app.CustomerPortalOverview{}, err
	}
	if overview.ProcessingOrders, err = r.listProcessingOrders(ctx, customerID); err != nil {
		return app.CustomerPortalOverview{}, err
	}
	if overview.DirectShipOrders, err = r.listDirectShipOrders(ctx, customerID); err != nil {
		return app.CustomerPortalOverview{}, err
	}
	if overview.Fees, err = r.listFeeItems(ctx, customerID); err != nil {
		return app.CustomerPortalOverview{}, err
	}
	if overview.Settlements, err = r.listSettlements(ctx, customerID); err != nil {
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
	if err := r.requireCustomerCapability(ctx, customerID, "processing"); err != nil {
		return app.ProcessingOrderSummary{}, err
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
	if err := r.requireCustomerCapability(ctx, customerID, "direct_ship"); err != nil {
		if altErr := r.requireCustomerCapability(ctx, customerID, "product_order"); altErr != nil {
			return app.DirectShipOrderSummary{}, err
		}
	}
	items := normalizeSubmittedDirectShipItems(cmd)
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.DirectShipOrderSummary{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	quotedItems := make([]submittedDirectShipQuotedItem, 0, len(items))
	for _, item := range items {
		quoted, err := r.quoteSubmittedDirectShipItemTx(ctx, tx, customerID, item)
		if err != nil {
			return app.DirectShipOrderSummary{}, err
		}
		quotedItems = append(quotedItems, quoted)
	}

	receiverSnapshot := strings.TrimSpace(strings.Join([]string{cmd.ReceiverName, cmd.ReceiverPhone, cmd.ReceiverAddress}, " "))
	payload := map[string]any{
		"submitted_by_employee_id": cmd.EmployeeID,
		"receiver_name":            cmd.ReceiverName,
		"receiver_phone":           cmd.ReceiverPhone,
		"receiver_address":         cmd.ReceiverAddress,
		"receiver_company":         cmd.ReceiverCompany,
		"shipping_amount":          cmd.ShippingAmount,
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
	for idx, item := range quotedItems {
		itemPayload := map[string]any{
			"submitted_by_employee_id":               cmd.EmployeeID,
			"product_id":                             item.ProductID,
			"customer_product_alias_id":              item.CustomerProductAliasID,
			"customer_product_display_name_snapshot": item.CustomerProductDisplayNameSnapshot,
			"customer_item_code_snapshot":            item.CustomerItemCodeSnapshot,
			"product_code_snapshot":                  item.ProductCodeSnapshot,
			"product_name_snapshot":                  item.ProductNameSnapshot,
			"product_name":                           item.ProductName,
			"product_kind":                           item.ProductKind,
			"spec":                                   item.Spec,
			"spec_g":                                 item.SpecG,
			"sales_unit":                             item.SalesUnit,
			"unit_bag_count":                         item.UnitBagCount,
			"unit_bean_g":                            item.UnitBeanG,
			"matched_price_qty":                      item.MatchedPriceQty,
			"price_source":                           item.PriceSource,
			"price_source_snapshot":                  item.PriceSourceSnapshot,
			"bean_list_publication_id":               item.BeanListUsage.PublicationID,
			"bean_list_version_no":                   item.BeanListUsage.VersionNo,
			"quantity_units":                         item.QuantityUnits,
			"unit_price":                             item.UnitPrice,
			"line_total_before_discount":             item.BaseLineTotal,
			"discount_type":                          item.DiscountType,
			"discount_value":                         item.DiscountValue,
			"discount_amount":                        item.DiscountAmount,
			"line_total":                             item.LineTotal,
			"note":                                   item.Note,
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.customer_direct_ship_import_order_items(
				import_order_id, batch_id, customer_id, line_no, product_title, spec, quantity_units, payload
			)
			VALUES($1,0,$2,$3,$4,$5,$6,$7::jsonb)
		`, r.schema), importOrderID, customerID, idx+1, item.ProductName, item.Spec, item.QuantityUnits, mustPayloadJSON(itemPayload)); err != nil {
			return app.DirectShipOrderSummary{}, err
		}
	}
	orderID, err := r.createSubmittedDirectShipERPOrderTx(ctx, tx, importOrderID)
	if err != nil {
		return app.DirectShipOrderSummary{}, err
	}
	for _, item := range quotedItems {
		if item.ProductKind != catalogdomain.ProductKindDripBag {
			continue
		}
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, customerFulfillmentOrderActor(cmd), "customer_fulfillment_order", &orderID, "fulfillment customer drip submit", nil, nil, nil, postgresinfra.AuditMeta{
			"product_id":     item.ProductID,
			"sales_unit":     item.SalesUnit,
			"qty":            item.QuantityUnits,
			"unit_bag_count": item.UnitBagCount,
			"unit_bean_g":    item.UnitBeanG,
			"price_source":   item.PriceSource,
			"total":          item.LineTotal,
		}); err != nil {
			return app.DirectShipOrderSummary{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return app.DirectShipOrderSummary{}, err
	}
	return app.DirectShipOrderSummary{
		OrderID:         orderID,
		OrderNo:         orderNo,
		OrderDate:       time.Now().Format("2006-01-02"),
		ReceiverAddress: receiverSnapshot,
		Status:          "submitted",
		ItemCount:       len(quotedItems),
	}, nil
}

type submittedDirectShipItem struct {
	ProductID                          int64
	CustomerProductAliasID             int64
	CustomerProductDisplayNameSnapshot string
	CustomerItemCodeSnapshot           string
	ProductCodeSnapshot                string
	ProductNameSnapshot                string
	ProductName                        string
	Spec                               string
	SpecG                              int64
	SalesUnit                          string
	QuantityUnits                      int64
	DiscountType                       string
	DiscountValue                      float64
	Note                               string
}

type submittedDirectShipQuotedItem struct {
	ProductID                          int64
	CustomerProductAliasID             int64
	CustomerProductDisplayNameSnapshot string
	CustomerItemCodeSnapshot           string
	ProductCodeSnapshot                string
	ProductNameSnapshot                string
	ProductName                        string
	ProductKind                        string
	Spec                               string
	SpecG                              int64
	SalesUnit                          string
	UnitBagCount                       float64
	UnitBeanG                          float64
	MatchedPriceQty                    float64
	PriceSource                        string
	PriceSourceSnapshot                string
	QuantityUnits                      int64
	UnitPrice                          float64
	DiscountType                       string
	DiscountValue                      float64
	DiscountAmount                     float64
	BaseLineTotal                      float64
	LineTotal                          float64
	BeanListUsage                      orderbeans.Usage
	Note                               string
}

func normalizeSubmittedDirectShipItems(cmd app.SubmitCustomerDirectShipOrderCommand) []submittedDirectShipItem {
	out := make([]submittedDirectShipItem, 0, len(cmd.Items))
	for _, item := range cmd.Items {
		out = append(out, submittedDirectShipItem{
			ProductID:                          item.ProductID,
			CustomerProductAliasID:             item.CustomerProductAliasID,
			CustomerProductDisplayNameSnapshot: strings.TrimSpace(item.CustomerProductDisplayNameSnapshot),
			CustomerItemCodeSnapshot:           strings.TrimSpace(item.CustomerItemCodeSnapshot),
			ProductCodeSnapshot:                strings.TrimSpace(item.ProductCodeSnapshot),
			ProductNameSnapshot:                strings.TrimSpace(item.ProductNameSnapshot),
			ProductName:                        strings.TrimSpace(item.ProductName),
			Spec:                               strings.TrimSpace(item.Spec),
			SpecG:                              item.SpecG,
			SalesUnit:                          strings.TrimSpace(item.SalesUnit),
			QuantityUnits:                      item.QuantityUnits,
			DiscountType:                       strings.TrimSpace(strings.ToLower(item.DiscountType)),
			DiscountValue:                      item.DiscountValue,
			Note:                               strings.TrimSpace(item.Note),
		})
	}
	if len(out) > 0 {
		return out
	}
	return []submittedDirectShipItem{{
		ProductID:     cmd.ProductID,
		ProductName:   strings.TrimSpace(cmd.ProductName),
		Spec:          strings.TrimSpace(cmd.Spec),
		SpecG:         parseSubmittedDirectShipSpecG(cmd.Spec),
		SalesUnit:     "bag",
		QuantityUnits: cmd.QuantityUnits,
		DiscountType:  "",
		DiscountValue: 0,
		Note:          strings.TrimSpace(cmd.Note),
	}}
}

type directShipCustomerAliasSnapshot struct {
	CustomerProductAliasID             int64
	CustomerProductDisplayNameSnapshot string
	CustomerItemCodeSnapshot           string
	ProductCodeSnapshot                string
	ProductNameSnapshot                string
}

func requireCustomerAliasDirectShipPrice(aliasSnapshot directShipCustomerAliasSnapshot, unitPrice float64) error {
	if aliasSnapshot.CustomerProductAliasID <= 0 || unitPrice > 0 {
		return nil
	}
	return fmt.Errorf("customer product price unpublished")
}

func (r *Repository) validateCustomerProductAliasForDirectShipTx(ctx context.Context, tx pgx.Tx, customerID, productID, aliasID int64) (directShipCustomerAliasSnapshot, error) {
	if productID <= 0 || !relationExists(ctx, tx, fmt.Sprintf("%s.customer_product_aliases", r.schema)) {
		return directShipCustomerAliasSnapshot{}, nil
	}
	q := fmt.Sprintf(`
		SELECT a.id,
		       COALESCE(NULLIF(a.display_name,''), p.name, ''),
		       COALESCE(a.customer_item_code,''),
		       'SKU-' || p.id::text,
		       COALESCE(p.name,'')
		FROM %[1]s.customer_product_aliases a
		JOIN %[1]s.products p ON p.id=a.product_id
		WHERE a.customer_id=$1
		  AND a.product_id=$2
		  AND a.active=true
	`, r.schema)
	args := []any{customerID, productID}
	if aliasID > 0 {
		q += " AND a.id=$3"
		args = append(args, aliasID)
	}
	q += " ORDER BY a.include_in_price_list DESC, a.sort_order, a.id LIMIT 1"
	var snap directShipCustomerAliasSnapshot
	err := tx.QueryRow(ctx, q, args...).Scan(
		&snap.CustomerProductAliasID,
		&snap.CustomerProductDisplayNameSnapshot,
		&snap.CustomerItemCodeSnapshot,
		&snap.ProductCodeSnapshot,
		&snap.ProductNameSnapshot,
	)
	if err == pgx.ErrNoRows {
		if aliasID > 0 {
			return directShipCustomerAliasSnapshot{}, fmt.Errorf("customer_product_alias invalid")
		}
		return directShipCustomerAliasSnapshot{}, nil
	}
	if err != nil {
		return directShipCustomerAliasSnapshot{}, err
	}
	return snap, nil
}

func (r *Repository) quoteSubmittedDirectShipItemTx(ctx context.Context, tx pgx.Tx, customerID int64, item submittedDirectShipItem) (submittedDirectShipQuotedItem, error) {
	specG := item.SpecG
	if specG <= 0 {
		specG = parseSubmittedDirectShipSpecG(item.Spec)
	}
	if specG <= 0 {
		specG = 454
	}
	if item.QuantityUnits <= 0 {
		item.QuantityUnits = 1
	}
	aliasSnapshot, err := r.validateCustomerProductAliasForDirectShipTx(ctx, tx, customerID, item.ProductID, item.CustomerProductAliasID)
	if err != nil {
		return submittedDirectShipQuotedItem{}, err
	}
	var dbProductName, productKind string
	var dripBagGrams float64
	var dripBoxBagCount int
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COALESCE(name,''),
			       COALESCE(NULLIF(product_kind,''), 'roasted_bean'),
			       COALESCE(drip_bag_grams,10)::float8,
			       COALESCE(drip_box_bag_count,10)
		FROM %s.products
		WHERE id=$1 AND active=true
		  AND %s
		`, r.schema, customerFulfillmentProductVisibleToCustomerSQL(r.schema+".products", "$2")), item.ProductID, customerID).Scan(&dbProductName, &productKind, &dripBagGrams, &dripBoxBagCount); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return submittedDirectShipQuotedItem{}, fmt.Errorf("product unavailable")
		}
		return submittedDirectShipQuotedItem{}, err
	}
	productKind = catalogdomain.NormalizeProductKind(productKind)
	productName := item.ProductName
	if aliasSnapshot.CustomerProductAliasID > 0 {
		item.CustomerProductAliasID = aliasSnapshot.CustomerProductAliasID
		item.CustomerProductDisplayNameSnapshot = aliasSnapshot.CustomerProductDisplayNameSnapshot
		item.CustomerItemCodeSnapshot = aliasSnapshot.CustomerItemCodeSnapshot
		item.ProductCodeSnapshot = aliasSnapshot.ProductCodeSnapshot
		item.ProductNameSnapshot = aliasSnapshot.ProductNameSnapshot
		productName = aliasSnapshot.CustomerProductDisplayNameSnapshot
	}
	if productName == "" {
		productName = strings.TrimSpace(dbProductName)
	}
	if item.ProductCodeSnapshot == "" {
		item.ProductCodeSnapshot = fmt.Sprintf("SKU-%d", item.ProductID)
	}
	if item.ProductNameSnapshot == "" {
		item.ProductNameSnapshot = strings.TrimSpace(dbProductName)
	}
	specText := item.Spec
	if specText == "" {
		specText = fmt.Sprintf("%dg", specG)
	}
	salesUnit := strings.TrimSpace(item.SalesUnit)
	unitBagCount := 0.0
	unitBeanG := 0.0
	matchedPriceQty := 0.0
	priceSource := "published_price_snapshot"
	priceSourceSnapshot := ""
	var beanListUsage orderbeans.Usage
	var unitPrice, baseLineTotal float64
	if productKind == catalogdomain.ProductKindDripBag {
		if err := r.ensureDripProductHasActiveBOMTx(ctx, tx, item.ProductID); err != nil {
			return submittedDirectShipQuotedItem{}, err
		}
		if salesUnit == "" {
			salesUnit = "bag"
		}
		if salesUnit != "bag" && salesUnit != "box" {
			return submittedDirectShipQuotedItem{}, fmt.Errorf("sales_unit invalid")
		}
		unitBagCount = 1
		if salesUnit == "box" {
			unitBagCount = float64(dripBoxBagCount)
		}
		unitBeanG = dripBagGrams
		if specG <= 0 || specG == 454 {
			specG = int64(math.Round(dripBagGrams))
		}
		if specText == "" || specText == "454g" {
			specText = customerFulfillmentDripSpecText(salesUnit, dripBagGrams, dripBoxBagCount)
		}
	}
	usage, pricing, err := r.customerFulfillmentPublishedPricingTx(ctx, tx, customerID, item.ProductID, productKind, specG, item.QuantityUnits, salesUnit, int64(math.Round(unitBagCount)))
	if err != nil {
		return submittedDirectShipQuotedItem{}, err
	}
	beanListUsage = usage
	unitPrice = pricing.UnitPrice
	if productKind == catalogdomain.ProductKindDripBag {
		baseLineTotal = pricing.UnitPrice * float64(item.QuantityUnits)
		matchedPriceQty = float64(item.QuantityUnits)
	} else {
		baseLineTotal = customerFulfillmentLineTotalFromPriceUnit(pricing.UnitPrice, specG, item.QuantityUnits, pricing.UnitG)
	}
	priceSourceSnapshot = customerFulfillmentPublishedPriceSourceSnapshot(orderbeans.ListTypeForProductKind(productKind, false), beanListUsage, item.ProductID, pricing)
	if err := requireCustomerAliasDirectShipPrice(aliasSnapshot, unitPrice); err != nil {
		return submittedDirectShipQuotedItem{}, err
	}
	discountType := normalizeSubmittedDirectShipDiscountType(item.DiscountType)
	discountValue := item.DiscountValue
	if discountValue < 0 {
		discountValue = 0
	}
	discountAmount, lineTotal := submittedDirectShipLineDiscount(baseLineTotal, discountType, discountValue)
	return submittedDirectShipQuotedItem{
		ProductID:                          item.ProductID,
		CustomerProductAliasID:             item.CustomerProductAliasID,
		CustomerProductDisplayNameSnapshot: item.CustomerProductDisplayNameSnapshot,
		CustomerItemCodeSnapshot:           item.CustomerItemCodeSnapshot,
		ProductCodeSnapshot:                item.ProductCodeSnapshot,
		ProductNameSnapshot:                item.ProductNameSnapshot,
		ProductName:                        productName,
		ProductKind:                        productKind,
		Spec:                               specText,
		SpecG:                              specG,
		SalesUnit:                          salesUnit,
		UnitBagCount:                       unitBagCount,
		UnitBeanG:                          unitBeanG,
		MatchedPriceQty:                    matchedPriceQty,
		PriceSource:                        priceSource,
		PriceSourceSnapshot:                priceSourceSnapshot,
		QuantityUnits:                      item.QuantityUnits,
		UnitPrice:                          unitPrice,
		DiscountType:                       discountType,
		DiscountValue:                      discountValue,
		DiscountAmount:                     discountAmount,
		BaseLineTotal:                      baseLineTotal,
		LineTotal:                          lineTotal,
		BeanListUsage:                      beanListUsage,
		Note:                               item.Note,
	}, nil
}

func (r *Repository) quoteSubmittedDirectShipItemForERPRebuildTx(ctx context.Context, tx pgx.Tx, customerID int64, item submittedDirectShipItem) (submittedDirectShipQuotedItem, bool, error) {
	if item.ProductID <= 0 {
		return submittedDirectShipQuotedItem{}, false, nil
	}
	quoted, err := r.quoteSubmittedDirectShipItemTx(ctx, tx, customerID, item)
	if err != nil {
		if strings.Contains(err.Error(), "product unavailable") {
			return submittedDirectShipQuotedItem{}, false, nil
		}
		return submittedDirectShipQuotedItem{}, false, err
	}
	return quoted, true, nil
}

func normalizeSubmittedDirectShipDiscountType(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "amount", "fixed", "minus":
		return "amount"
	case "percent", "discount":
		return "percent"
	case "free":
		return "free"
	default:
		return ""
	}
}

func submittedDirectShipLineDiscount(baseLineTotal float64, discountType string, discountValue float64) (float64, float64) {
	if baseLineTotal <= 0 {
		return 0, 0
	}
	if discountValue < 0 {
		discountValue = 0
	}
	switch discountType {
	case "free":
		return baseLineTotal, 0
	case "amount":
		discountAmount := math.Min(baseLineTotal, discountValue)
		return discountAmount, math.Max(baseLineTotal-discountAmount, 0)
	case "percent":
		rate := math.Max(0, math.Min(discountValue, 100))
		lineTotal := baseLineTotal * rate / 100
		return math.Max(baseLineTotal-lineTotal, 0), math.Max(lineTotal, 0)
	default:
		return 0, baseLineTotal
	}
}

func parseSubmittedDirectShipSpecG(spec string) int64 {
	spec = strings.TrimSpace(strings.TrimSuffix(strings.ToLower(spec), "g"))
	if spec == "" {
		return 0
	}
	n, err := strconv.ParseInt(spec, 10, 64)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func customerFulfillmentOrderActor(cmd app.SubmitCustomerDirectShipOrderCommand) string {
	if actor := strings.TrimSpace(cmd.Actor); actor != "" {
		return actor
	}
	if cmd.EmployeeID > 0 {
		return fmt.Sprintf("employee:%d", cmd.EmployeeID)
	}
	return "customer_fulfillment"
}

func (r *Repository) ensureDripProductHasActiveBOMTx(ctx context.Context, tx pgx.Tx, productID int64) error {
	var exists bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT EXISTS (
			SELECT 1
			FROM %s.products p
			LEFT JOIN %s.product_bom_sources bs ON bs.product_id=p.id
			JOIN %s.product_bom b ON b.product_id=CASE
				WHEN COALESCE(NULLIF(bs.source_type,''),'') IN ('inherit_current','inherit_version') AND COALESCE(bs.source_product_id,0)>0 THEN bs.source_product_id
				ELSE p.id
			END
			WHERE p.id=$1 AND b.status='active'
			  AND EXISTS (SELECT 1 FROM %s.product_bom_items bi WHERE bi.product_id=b.product_id)
		)
	`, r.schema, r.schema, r.schema, r.schema), productID).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	productionBomExists, err := r.productionBOMConfiguredForProductTx(ctx, tx, productID)
	if err != nil {
		return err
	}
	if productionBomExists {
		return nil
	}
	return fmt.Errorf("product BOM not configured")
}

func (r *Repository) productionBOMConfiguredForProductTx(ctx context.Context, tx pgx.Tx, productID int64) (bool, error) {
	for _, relation := range []string{"production_boms", "production_bom_versions", "production_bom_version_items"} {
		if !relationExists(ctx, tx, fmt.Sprintf("%s.%s", r.schema, relation)) {
			return false, nil
		}
	}
	var exists bool
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT EXISTS (
			SELECT 1
			FROM %[1]s.production_boms pb
			JOIN %[1]s.production_bom_versions v ON v.bom_id=pb.id AND v.status='published'
			JOIN %[1]s.production_bom_version_items i ON i.version_id=v.id
			WHERE pb.status='active'
			  AND (pb.output_product_id=$1 OR pb.legacy_product_id=$1)
		)
	`, r.schema), productID).Scan(&exists)
	return exists, err
}

func (r *Repository) customerFulfillmentPublishedPricingTx(ctx context.Context, tx pgx.Tx, customerID, productID int64, productKind string, specG, qty int64, salesUnit string, unitBagCount int64) (orderbeans.Usage, orderbeans.PublishedPricing, error) {
	listType := orderbeans.ListTypeForProductKind(productKind, false)
	usage, err := orderbeans.ResolveUsageForPublication(ctx, tx, r.schema, customerID, productID, listType, 0)
	if err != nil {
		return orderbeans.Usage{}, orderbeans.PublishedPricing{}, err
	}
	if usage.PublicationID <= 0 {
		return orderbeans.Usage{}, orderbeans.PublishedPricing{}, fmt.Errorf(customerFulfillmentMissingPublishedPriceMessage(listType))
	}
	pricing, err := orderbeans.ResolvePublishedPricingForPublicationWithUnit(ctx, tx, r.schema, customerID, productID, listType, usage.PublicationID, specG, qty, salesUnit, unitBagCount)
	if err != nil {
		return orderbeans.Usage{}, orderbeans.PublishedPricing{}, err
	}
	if pricing.UnitPrice <= 0 {
		return orderbeans.Usage{}, orderbeans.PublishedPricing{}, fmt.Errorf(customerFulfillmentMissingPublishedPriceMessage(listType))
	}
	return usage, pricing, nil
}

func customerFulfillmentMissingPublishedPriceMessage(listType string) string {
	switch strings.TrimSpace(listType) {
	case orderbeans.ListTypeDrip:
		return "缺少挂耳价格表价格"
	case orderbeans.ListTypeGreen:
		return "缺少生豆豆单价格"
	default:
		return "缺少商品价格表价格"
	}
}

func customerFulfillmentPublishedPriceSourceSnapshot(listType string, usage orderbeans.Usage, productID int64, pricing orderbeans.PublishedPricing) string {
	conversion := json.RawMessage(`{}`)
	if raw := strings.TrimSpace(pricing.InventoryConversionJSON); raw != "" && json.Valid([]byte(raw)) {
		conversion = json.RawMessage(raw)
	}
	b, _ := json.Marshal(map[string]any{
		"source":                    "published_price_snapshot",
		"price_source":              "published_price_snapshot",
		"list_type":                 strings.TrimSpace(listType),
		"product_id":                productID,
		"bean_list_publication_id":  usage.PublicationID,
		"bean_list_version_no":      usage.VersionNo,
		"unit_price":                pricing.UnitPrice,
		"price_unit":                pricing.PriceUnit,
		"source_price_record_id":    pricing.SourcePriceRecordID,
		"inventory_unit":            pricing.InventoryUnit,
		"inventory_conversion_json": conversion,
	})
	return string(b)
}

func customerFulfillmentDripSpecText(salesUnit string, bagGrams float64, boxBagCount int) string {
	if bagGrams <= 0 {
		bagGrams = 10
	}
	if boxBagCount <= 0 {
		boxBagCount = 10
	}
	bagText := fmt.Sprintf("%.0fg", bagGrams)
	if math.Abs(bagGrams-math.Round(bagGrams)) > 0.001 {
		bagText = fmt.Sprintf("%.1fg", bagGrams)
	}
	if salesUnit == "box" {
		return fmt.Sprintf("%s*%d袋/盒", bagText, boxBagCount)
	}
	return bagText + "/袋"
}

func customerFulfillmentDisplayUnit(salesUnit string) string {
	switch strings.TrimSpace(salesUnit) {
	case "bag":
		return "袋"
	case "box":
		return "盒"
	default:
		return "件"
	}
}

func customerFulfillmentJSONOrEmpty(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return "{}"
	}
	return raw
}

func customerFulfillmentProductVisibleToCustomerSQL(productTable, customerPlaceholder string) string {
	productTable = strings.TrimSpace(productTable)
	if productTable == "" {
		productTable = "products"
	}
	return fmt.Sprintf(`(
		(
			CASE
				WHEN COALESCE(customer_id,0)>0 THEN COALESCE(NULLIF(visibility,''),'customer_only')
				ELSE COALESCE(NULLIF(visibility,''),'public')
			END <> 'customer_only'
			OR COALESCE(customer_id,0)=%[1]s
		)
		AND NOT (
			COALESCE(customer_id,0)=0
			AND EXISTS (
				SELECT 1 FROM %[2]s alias_products
				WHERE alias_products.active=true
				  AND COALESCE(alias_products.customer_id,0)=%[1]s
				  AND COALESCE(alias_products.base_product_id,0)=id
				  AND COALESCE(NULLIF(alias_products.visibility,''),'customer_only')='customer_only'
			)
		)
	)`, customerPlaceholder, productTable)
}

func customerFulfillmentTierQuantityForSpec(specG int64, units int64) float64 {
	if specG >= 1000 {
		return float64(specG*units) / 1000.0
	}
	return float64(units)
}

func customerFulfillmentDisplayUnitG(specG int64) float64 {
	if specG >= 1000 {
		return 1000
	}
	return 454
}

func customerFulfillmentDisplayUnitPriceFromLb(pricePerLb float64, specG int64) float64 {
	if pricePerLb <= 0 || specG <= 0 {
		return 0
	}
	unitG := customerFulfillmentDisplayUnitG(specG)
	displayUnitPrice := pricePerLb * unitG / 454.0
	if unitG == 1000 {
		displayUnitPrice = math.Round(displayUnitPrice)
	}
	return displayUnitPrice
}

func customerFulfillmentLineTotalFromDisplayUnit(unitPrice float64, specG int64, units int64) float64 {
	if unitPrice <= 0 || specG <= 0 || units <= 0 {
		return 0
	}
	return unitPrice * float64(specG*units) / customerFulfillmentDisplayUnitG(specG)
}

func customerFulfillmentLineTotalFromPriceUnit(unitPrice float64, specG int64, units int64, unitG float64) float64 {
	if unitPrice <= 0 || specG <= 0 || units <= 0 {
		return 0
	}
	if unitG <= 0 {
		unitG = customerFulfillmentDisplayUnitG(specG)
	}
	return unitPrice * float64(specG*units) / unitG
}

func (r *Repository) customerFulfillmentDirectShipSmallBatchPriceRuleTx(ctx context.Context, tx pgx.Tx, customerID int64) customerportalapp.SmallBatchPriceRule {
	if customerID <= 0 {
		return customerportalapp.SmallBatchPriceRule{}
	}
	if template, ok, err := r.customerCapabilityTemplateForCustomer(ctx, tx, customerID); err == nil && ok {
		for _, capability := range template.Capabilities {
			if strings.TrimSpace(capability.Code) != customerportalapp.CapabilityDirectShip || !capability.Enabled {
				continue
			}
			raw, _ := capability.Config["small_batch_price_rule"].(map[string]any)
			return customerFulfillmentNormalizeSmallBatchPriceRule(customerportalapp.SmallBatchPriceRule{
				Enabled:     boolFromAny(raw["enabled"]),
				ThresholdLB: floatFromAny(raw["threshold_lb"]),
				TierMinLB:   floatFromAny(raw["tier_min_lb"]),
				TierMaxLB:   floatFromAny(raw["tier_max_lb"]),
			})
		}
		return customerportalapp.SmallBatchPriceRule{}
	}
	var raw []byte
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT config_json
		FROM %s.customer_service_capabilities
		WHERE customer_id=$1 AND capability_code=$2 AND enabled=true
	`, r.schema), customerID, customerportalapp.CapabilityDirectShip).Scan(&raw)
	if err != nil {
		return customerportalapp.SmallBatchPriceRule{}
	}
	var config struct {
		SmallBatchPriceRule customerportalapp.SmallBatchPriceRule `json:"small_batch_price_rule"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return customerportalapp.SmallBatchPriceRule{}
	}
	return customerFulfillmentNormalizeSmallBatchPriceRule(config.SmallBatchPriceRule)
}

func customerFulfillmentNormalizeSmallBatchPriceRule(rule customerportalapp.SmallBatchPriceRule) customerportalapp.SmallBatchPriceRule {
	if !rule.Enabled {
		return customerportalapp.SmallBatchPriceRule{}
	}
	if rule.ThresholdLB <= 0 {
		rule.ThresholdLB = 14
	}
	if rule.TierMinLB <= 0 {
		rule.TierMinLB = 15
	}
	if rule.TierMaxLB <= 0 {
		rule.TierMaxLB = 28
	}
	if rule.TierMaxLB < rule.TierMinLB {
		rule.TierMaxLB = rule.TierMinLB
	}
	return rule
}

func customerFulfillmentSmallBatchTierQuantity(specG int64, qtyLb float64, rule customerportalapp.SmallBatchPriceRule) (int64, bool) {
	if specG <= 0 || !rule.Enabled || qtyLb <= 0 {
		return 0, false
	}
	if qtyLb >= rule.ThresholdLB {
		return 0, false
	}
	targetLb := rule.TierMinLB
	if targetLb <= 0 {
		return 0, false
	}
	if rule.TierMaxLB > 0 && targetLb > rule.TierMaxLB {
		targetLb = rule.TierMaxLB
	}
	units := int64(math.Ceil(targetLb * 454.0 / float64(specG)))
	if units <= 0 {
		return 0, false
	}
	return units, true
}

func (r *Repository) refreshSubmittedDirectShipOrderAmountsTx(ctx context.Context, tx pgx.Tx, importOrderID, orderID int64) error {
	var shippingAmount float64
	_ = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(NULLIF(payload->>'shipping_amount','')::numeric, 0)::float8
		FROM %s.customer_direct_ship_import_orders
		WHERE id=$1
	`, r.schema), importOrderID).Scan(&shippingAmount)
	if shippingAmount < 0 {
		shippingAmount = 0
	}
	items, err := r.submittedDirectShipERPItemSeedsTx(ctx, tx, importOrderID)
	if err != nil {
		return err
	}
	totalAmount, discountAmount := submittedDirectShipERPOrderAmounts(items)
	grandTotal := totalAmount + shippingAmount - discountAmount
	_, err = tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.orders
		SET total_amount=$2,
		    shipping_amount=$3,
		    discount_amount=$4,
		    grand_total=$5
		WHERE id=$1
	`, r.schema), orderID, totalAmount, shippingAmount, discountAmount, grandTotal)
	return err
}

func boolFromAny(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	default:
		return false
	}
}

func floatFromAny(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case int32:
		return float64(v)
	case json.Number:
		n, _ := v.Float64()
		return n
	case string:
		n, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return n
	default:
		return 0
	}
}

func (r *Repository) requireCustomerCapability(ctx context.Context, customerID int64, capability string) error {
	capability = strings.TrimSpace(capability)
	if customerID <= 0 || capability == "" {
		return fmt.Errorf("customer capability required")
	}
	codes, err := r.listCustomerCapabilityCodes(ctx, customerID)
	if err != nil {
		return err
	}
	for _, code := range codes {
		if code == capability {
			return nil
		}
	}
	return fmt.Errorf("customer capability %s unavailable", capability)
}

func capabilityForImportType(importType app.ImportType) string {
	switch importType {
	case app.ImportTypeProcessingWorkbook:
		return "processing"
	case app.ImportTypeDirectShipWorkbook:
		return "direct_ship"
	case app.ImportTypeSettlementWorkbook:
		return "settlement"
	default:
		return ""
	}
}

func backfillSubmittedDirectShipERPOrders(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	repo := &Repository{pool: pool, schema: schema}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id
		FROM %s.customer_direct_ship_import_orders
		WHERE batch_id=0
		  AND status='submitted'
		  AND COALESCE(order_id,0)=0
		ORDER BY id
	`, schema))
	if err != nil {
		return err
	}
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, id := range ids {
		if _, err := repo.createSubmittedDirectShipERPOrderTx(ctx, tx, id); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func repairSubmittedDirectShipERPOrderReceivers(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT order_id, receiver_address, payload
		FROM %s.customer_direct_ship_import_orders
		WHERE batch_id=0
		  AND status='submitted'
		  AND COALESCE(order_id,0)>0
		ORDER BY id
	`, schema))
	if err != nil {
		return err
	}
	type receiverSeed struct {
		orderID  int64
		snapshot string
		payload  map[string]any
	}
	seeds := make([]receiverSeed, 0)
	for rows.Next() {
		var seed receiverSeed
		var payloadJSON []byte
		if err := rows.Scan(&seed.orderID, &seed.snapshot, &payloadJSON); err != nil {
			rows.Close()
			return err
		}
		seed.payload = map[string]any{}
		if len(payloadJSON) > 0 {
			_ = json.Unmarshal(payloadJSON, &seed.payload)
		}
		seeds = append(seeds, seed)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, seed := range seeds {
		receiverName, receiverPhone, receiverAddress, receiverCompany := submittedDirectShipReceiver(seed.payload, seed.snapshot)
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.orders
			SET receiver_name=$2,
			    receiver_phone=$3,
			    receiver_address=$4,
			    receiver_company=$5
			WHERE id=$1
			  AND portal_service_code='direct_ship'
		`, schema), seed.orderID, receiverName, receiverPhone, receiverAddress, receiverCompany); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func repairSubmittedDirectShipERPOrderDiscounts(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	repo := NewRepository(pool, schema)
	rows, err := pool.Query(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(order_id,0)
		FROM %s.customer_direct_ship_import_orders
		WHERE COALESCE(order_id,0)>0
		ORDER BY id
	`, schema))
	if err != nil {
		return err
	}
	type seed struct {
		importOrderID int64
		orderID       int64
	}
	seeds := make([]seed, 0)
	for rows.Next() {
		var s seed
		if err := rows.Scan(&s.importOrderID, &s.orderID); err != nil {
			rows.Close()
			return err
		}
		seeds = append(seeds, s)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, s := range seeds {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.order_items WHERE order_id=$1`, schema), s.orderID); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := repo.createSubmittedDirectShipERPOrderItemsTx(ctx, tx, s.importOrderID, s.orderID); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := repo.refreshSubmittedDirectShipOrderAmountsTx(ctx, tx, s.importOrderID, s.orderID); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) createSubmittedDirectShipERPOrderTx(ctx context.Context, tx pgx.Tx, importOrderID int64) (int64, error) {
	var customerID, existingOrderID int64
	var orderNo, orderDate, receiverSnapshot string
	var payloadJSON []byte
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT customer_id,
		       COALESCE(NULLIF(external_order_no,''), 'CDS-' || to_char(created_at,'YYYYMMDD') || '-' || id::text),
		       COALESCE(to_char(order_date, 'YYYY-MM-DD'), ''),
		       receiver_address,
		       payload,
		       COALESCE(order_id,0)
		FROM %s.customer_direct_ship_import_orders
		WHERE id=$1
		FOR UPDATE
	`, r.schema), importOrderID).Scan(&customerID, &orderNo, &orderDate, &receiverSnapshot, &payloadJSON, &existingOrderID); err != nil {
		return 0, err
	}
	if existingOrderID > 0 {
		return existingOrderID, nil
	}
	payload := map[string]any{}
	if len(payloadJSON) > 0 {
		_ = json.Unmarshal(payloadJSON, &payload)
	}
	receiverName, receiverPhone, receiverAddress, receiverCompany := submittedDirectShipReceiver(payload, receiverSnapshot)
	payStatusID := customerFulfillmentStatusID(ctx, tx, r.schema, "pay_statuses", "未付款", "未收款")
	shipStatusID := customerFulfillmentStatusID(ctx, tx, r.schema, "ship_statuses", "未发货")
	processStatusID := customerFulfillmentStatusID(ctx, tx, r.schema, "order_process_statuses", "待处理", "待生产")

	var orderID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.orders(
			order_no, order_date, customer_id, pay_status_id, ship_status_id, process_status_id,
			portal_service_code, source_warehouse,
			receiver_name, receiver_phone, receiver_address, receiver_company,
			notes, created_at
		)
		VALUES($1,$2,$3,$4,$5,$6,'direct_ship','finished_goods',$7,$8,$9,$10,$11,now())
		RETURNING id
	`, r.schema),
		orderNo,
		parseDateValue(orderDate),
		customerID,
		nullableCustomerFulfillmentID(payStatusID),
		nullableCustomerFulfillmentID(shipStatusID),
		nullableCustomerFulfillmentID(processStatusID),
		receiverName,
		receiverPhone,
		receiverAddress,
		receiverCompany,
		payloadString(payload, "note"),
	).Scan(&orderID); err != nil {
		return 0, err
	}
	if err := r.createSubmittedDirectShipERPOrderItemsTx(ctx, tx, importOrderID, orderID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.customer_direct_ship_import_orders
		SET order_id=$2
		WHERE id=$1
	`, r.schema), importOrderID, orderID); err != nil {
		return 0, err
	}
	if err := r.refreshSubmittedDirectShipOrderAmountsTx(ctx, tx, importOrderID, orderID); err != nil {
		return 0, err
	}
	return orderID, nil
}

func (r *Repository) createSubmittedDirectShipERPOrderItemsTx(ctx context.Context, tx pgx.Tx, importOrderID, orderID int64) error {
	items, err := r.submittedDirectShipERPItemSeedsTx(ctx, tx, importOrderID)
	if err != nil {
		return err
	}

	for _, item := range items {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.order_items(
				order_id,line_no,product_id,customer_product_alias_id,customer_product_display_name_snapshot,customer_item_code_snapshot,product_code_snapshot,product_name_snapshot,item_name,qty,unit,spec,unit_price,
				line_total_before_discount,discount_type,discount_value,discount_amount,line_total,
				product_kind,sales_unit,unit_bag_count,unit_bean_g,matched_price_qty,price_source_json,
				bean_list_publication_id,bean_list_version_no
			)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24::jsonb,NULLIF($25,0),$26)
		`, r.schema), orderID, item.lineNo, nullableCustomerFulfillmentID(item.productID), item.customerProductAliasID, item.customerProductDisplayNameSnapshot, item.customerItemCodeSnapshot, item.productCodeSnapshot, item.productNameSnapshot, item.productTitle, item.quantity, customerFulfillmentDisplayUnit(item.salesUnit), item.spec, item.unitPrice, item.baseLineTotal, item.discountType, item.discountValue, item.discountAmount, item.lineTotal, item.productKind, item.salesUnit, item.unitBagCount, item.unitBeanG, item.matchedPriceQty, customerFulfillmentJSONOrEmpty(item.priceSourceSnapshot), item.beanListUsage.PublicationID, item.beanListUsage.VersionNo); err != nil {
			return err
		}
	}
	return nil
}

type submittedDirectShipERPItemSeed struct {
	lineNo                             int
	productID                          int64
	customerProductAliasID             int64
	customerProductDisplayNameSnapshot string
	customerItemCodeSnapshot           string
	productCodeSnapshot                string
	productNameSnapshot                string
	productTitle                       string
	productKind                        string
	spec                               string
	quantity                           int64
	salesUnit                          string
	unitBagCount                       float64
	unitBeanG                          float64
	matchedPriceQty                    float64
	priceSourceSnapshot                string
	unitPrice                          float64
	baseLineTotal                      float64
	discountType                       string
	discountValue                      float64
	discountAmount                     float64
	lineTotal                          float64
	beanListUsage                      orderbeans.Usage
}

func (r *Repository) submittedDirectShipERPItemSeedsTx(ctx context.Context, tx pgx.Tx, importOrderID int64) ([]submittedDirectShipERPItemSeed, error) {
	var customerID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT customer_id
		FROM %s.customer_direct_ship_import_orders
		WHERE id=$1
	`, r.schema), importOrderID).Scan(&customerID); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT line_no, product_title, spec, quantity_units, payload
		FROM %s.customer_direct_ship_import_order_items
		WHERE import_order_id=$1
		ORDER BY line_no, id
	`, r.schema), importOrderID)
	if err != nil {
		return nil, err
	}
	items := make([]submittedDirectShipERPItemSeed, 0)
	lineNo := 0
	for rows.Next() {
		lineNo++
		var rowLineNo int
		var productTitle, spec string
		var quantity int64
		var payloadJSON []byte
		if err := rows.Scan(&rowLineNo, &productTitle, &spec, &quantity, &payloadJSON); err != nil {
			rows.Close()
			return nil, err
		}
		payload := map[string]any{}
		if len(payloadJSON) > 0 {
			_ = json.Unmarshal(payloadJSON, &payload)
		}
		productID := payloadInt64(payload, "product_id")
		customerProductAliasID := payloadInt64(payload, "customer_product_alias_id")
		customerProductDisplayNameSnapshot := payloadString(payload, "customer_product_display_name_snapshot")
		customerItemCodeSnapshot := payloadString(payload, "customer_item_code_snapshot")
		productCodeSnapshot := payloadString(payload, "product_code_snapshot")
		productNameSnapshot := payloadString(payload, "product_name_snapshot")
		if productTitle == "" {
			productTitle = payloadString(payload, "product_name", "product_title")
		}
		if spec == "" {
			spec = payloadString(payload, "spec")
		}
		if quantity <= 0 {
			quantity = payloadInt64(payload, "quantity_units")
		}
		if quantity <= 0 {
			quantity = 1
		}
		productKind := catalogdomain.NormalizeProductKind(payloadString(payload, "product_kind"))
		salesUnit := strings.TrimSpace(payloadString(payload, "sales_unit"))
		unitBagCount := payloadFloat(payload, "unit_bag_count")
		unitBeanG := payloadFloat(payload, "unit_bean_g")
		matchedPriceQty := payloadFloat(payload, "matched_price_qty")
		priceSourceSnapshot := payloadString(payload, "price_source_snapshot")
		beanListUsage := orderbeans.Usage{
			PublicationID: payloadInt64(payload, "bean_list_publication_id"),
			VersionNo:     payloadString(payload, "bean_list_version_no"),
		}
		unitPrice := payloadFloat(payload, "unit_price")
		baseLineTotal := payloadFloat(payload, "line_total_before_discount")
		lineTotal := payloadFloat(payload, "line_total")
		if baseLineTotal <= 0 && unitPrice > 0 {
			baseLineTotal = unitPrice * float64(quantity)
		}
		discountType := normalizeSubmittedDirectShipDiscountType(payloadString(payload, "discount_type"))
		discountValue := payloadFloat(payload, "discount_value")
		discountAmount := payloadFloat(payload, "discount_amount")
		if lineTotal <= 0 && discountType == "" && discountAmount <= 0 && unitPrice > 0 {
			lineTotal = unitPrice * float64(quantity)
		}
		if discountAmount <= 0 && baseLineTotal > 0 {
			discountAmount, lineTotal = submittedDirectShipLineDiscount(baseLineTotal, discountType, discountValue)
		}
		if rowLineNo <= 0 {
			rowLineNo = lineNo
		}
		items = append(items, submittedDirectShipERPItemSeed{
			lineNo:                             rowLineNo,
			productID:                          productID,
			customerProductAliasID:             customerProductAliasID,
			customerProductDisplayNameSnapshot: customerProductDisplayNameSnapshot,
			customerItemCodeSnapshot:           customerItemCodeSnapshot,
			productCodeSnapshot:                productCodeSnapshot,
			productNameSnapshot:                productNameSnapshot,
			productTitle:                       productTitle,
			productKind:                        productKind,
			spec:                               spec,
			quantity:                           quantity,
			salesUnit:                          salesUnit,
			unitBagCount:                       unitBagCount,
			unitBeanG:                          unitBeanG,
			matchedPriceQty:                    matchedPriceQty,
			priceSourceSnapshot:                priceSourceSnapshot,
			beanListUsage:                      beanListUsage,
			unitPrice:                          unitPrice,
			baseLineTotal:                      baseLineTotal,
			discountType:                       discountType,
			discountValue:                      discountValue,
			discountAmount:                     discountAmount,
			lineTotal:                          lineTotal,
		})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for idx := range items {
		specG := parseSubmittedDirectShipSpecG(items[idx].spec)
		quoted, quoteOK, quoteErr := r.quoteSubmittedDirectShipItemForERPRebuildTx(ctx, tx, customerID, submittedDirectShipItem{
			ProductID:                          items[idx].productID,
			CustomerProductAliasID:             items[idx].customerProductAliasID,
			CustomerProductDisplayNameSnapshot: items[idx].customerProductDisplayNameSnapshot,
			CustomerItemCodeSnapshot:           items[idx].customerItemCodeSnapshot,
			ProductCodeSnapshot:                items[idx].productCodeSnapshot,
			ProductNameSnapshot:                items[idx].productNameSnapshot,
			ProductName:                        items[idx].productTitle,
			Spec:                               items[idx].spec,
			SpecG:                              specG,
			SalesUnit:                          items[idx].salesUnit,
			QuantityUnits:                      items[idx].quantity,
			DiscountType:                       items[idx].discountType,
			DiscountValue:                      items[idx].discountValue,
		})
		if quoteErr != nil {
			return nil, quoteErr
		}
		if !quoteOK {
			continue
		}
		if quoted.ProductName != "" {
			items[idx].productTitle = quoted.ProductName
		}
		if quoted.CustomerProductAliasID > 0 {
			items[idx].customerProductAliasID = quoted.CustomerProductAliasID
			items[idx].customerProductDisplayNameSnapshot = quoted.CustomerProductDisplayNameSnapshot
			items[idx].customerItemCodeSnapshot = quoted.CustomerItemCodeSnapshot
			items[idx].productCodeSnapshot = quoted.ProductCodeSnapshot
			items[idx].productNameSnapshot = quoted.ProductNameSnapshot
		}
		if quoted.Spec != "" {
			items[idx].spec = quoted.Spec
		}
		if quoted.ProductKind != "" {
			items[idx].productKind = quoted.ProductKind
		}
		if quoted.SalesUnit != "" {
			items[idx].salesUnit = quoted.SalesUnit
		}
		if quoted.UnitBagCount > 0 {
			items[idx].unitBagCount = quoted.UnitBagCount
		}
		if quoted.UnitBeanG > 0 {
			items[idx].unitBeanG = quoted.UnitBeanG
		}
		if quoted.MatchedPriceQty > 0 {
			items[idx].matchedPriceQty = quoted.MatchedPriceQty
		}
		if quoted.PriceSourceSnapshot != "" {
			items[idx].priceSourceSnapshot = quoted.PriceSourceSnapshot
		}
		items[idx].unitPrice = quoted.UnitPrice
		items[idx].baseLineTotal = quoted.BaseLineTotal
		items[idx].discountType = quoted.DiscountType
		items[idx].discountValue = quoted.DiscountValue
		items[idx].discountAmount = quoted.DiscountAmount
		items[idx].lineTotal = quoted.LineTotal
		items[idx].beanListUsage = quoted.BeanListUsage
	}
	return items, nil
}

func submittedDirectShipERPOrderAmounts(items []submittedDirectShipERPItemSeed) (float64, float64) {
	totalAmount := 0.0
	discountAmount := 0.0
	for _, item := range items {
		base := item.baseLineTotal
		if base <= 0 {
			base = item.lineTotal + item.discountAmount
		}
		totalAmount += base
		discountAmount += item.discountAmount
	}
	return totalAmount, discountAmount
}

func submittedDirectShipReceiver(payload map[string]any, snapshot string) (string, string, string, string) {
	receiverName := payloadString(payload, "receiver_name")
	receiverPhone := payloadString(payload, "receiver_phone")
	receiverAddress := payloadString(payload, "receiver_address")
	receiverCompany := payloadString(payload, "receiver_company")
	if receiverAddress == "" && snapshot != "" {
		receiverAddress = strings.TrimSpace(snapshot)
		if receiverPhone != "" {
			receiverAddress = strings.TrimSpace(strings.Replace(receiverAddress, receiverPhone, "", 1))
		}
		if receiverName != "" {
			receiverAddress = strings.TrimSpace(strings.Replace(receiverAddress, receiverName, "", 1))
		}
	}
	if looksLikeCustomerFulfillmentAddress(receiverName) && receiverAddress != "" && !looksLikeCustomerFulfillmentAddress(receiverAddress) {
		receiverName, receiverAddress = receiverAddress, receiverName
	}
	if receiverName == "" || receiverPhone == "" || receiverAddress == "" {
		name, phone, address := parseReceiverSnapshot(snapshot)
		if receiverName == "" {
			receiverName = name
		}
		if receiverPhone == "" {
			receiverPhone = phone
		}
		if receiverAddress == "" {
			receiverAddress = address
		}
	}
	return receiverName, receiverPhone, receiverAddress, receiverCompany
}

func looksLikeCustomerFulfillmentAddress(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	for _, marker := range []string{"省", "市", "区", "县", "镇", "街道", "路", "号", "室", "村", "组"} {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}

func customerFulfillmentStatusID(ctx context.Context, tx pgx.Tx, schema, table string, names ...string) int64 {
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		var id int64
		q := fmt.Sprintf("SELECT id FROM %s.%s WHERE name=$1 ORDER BY id LIMIT 1", schema, table)
		if err := tx.QueryRow(ctx, q, name).Scan(&id); err == nil && id > 0 {
			return id
		}
	}
	return 0
}

func nullableCustomerFulfillmentID(id int64) any {
	if id <= 0 {
		return nil
	}
	return id
}

func (r *Repository) AdjustCustodyInventory(ctx context.Context, cmd app.AdjustCustodyInventoryCommand) (app.CustodyBalance, error) {
	if err := r.requireCustomerCapability(ctx, cmd.CustomerID, "inventory_custody"); err != nil {
		return app.CustodyBalance{}, err
	}

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
		if err := r.requireCustomerERPWorkbenchTemplateTx(ctx, tx, cmd.CustomerID); err != nil {
			return app.CustomerERPBinding{}, err
		}
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
		LEFT JOIN %s.employee_login_passwords p ON p.employee_id=e.id
		WHERE e.id=$2 AND e.active=true AND e.account_type='channel_customer' AND COALESCE(p.login_disabled,false)=false
		ON CONFLICT (employee_id, customer_id) DO UPDATE SET
			role=excluded.role,
			status=excluded.status,
			updated_by=excluded.updated_by,
			updated_at=now()
		RETURNING customer_id, employee_id, role, status, updated_by, to_char(updated_at,'YYYY-MM-DD HH24:MI')
	`, r.schema, r.schema, r.schema), cmd.CustomerID, cmd.EmployeeID, cmd.Role, cmd.Status, cmd.Actor).Scan(&row.CustomerID, &row.EmployeeID, &row.Role, &row.Status, &row.UpdatedBy, &row.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.CustomerERPBinding{}, fmt.Errorf("login-enabled channel customer account required")
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

func (r *Repository) requireCustomerERPWorkbenchTemplateTx(ctx context.Context, tx pgx.Tx, customerID int64) error {
	var templateKey string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(capability_template_key,'')
		FROM %s.customer_portal_profiles
		WHERE customer_id=$1 AND enabled=true
	`, r.schema), customerID).Scan(&templateKey); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return customerportalapp.ErrCapabilityTemplateERPWorkbenchUnavailable
		}
		return err
	}
	available, err := r.customerERPWorkbenchAvailableForTemplateKey(ctx, tx, templateKey)
	if err != nil {
		return err
	}
	if !available {
		return customerportalapp.ErrCapabilityTemplateERPWorkbenchUnavailable
	}
	return nil
}

func (r *Repository) grantCustomerTemplateRolesForEmployeeTx(ctx context.Context, tx pgx.Tx, customerID, employeeID int64) error {
	var templateKey string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(capability_template_key,'')
		FROM %s.customer_portal_profiles
		WHERE customer_id=$1 AND enabled=true
	`, r.schema), customerID).Scan(&templateKey); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	template, ok, err := r.customerCapabilityTemplateForKey(ctx, tx, templateKey)
	if err != nil {
		return err
	}
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
		LEFT JOIN %s.employee_login_passwords p ON p.employee_id=e.id
		WHERE b.customer_id=$1
		  AND (b.status<>'active' OR (e.active=true AND e.account_type='channel_customer' AND COALESCE(p.login_disabled,false)=false))
		ORDER BY b.updated_at DESC, b.id DESC
	`, r.schema, r.schema, r.schema), customerID)
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

func (r *Repository) ListExternalUsers(ctx context.Context, customerID int64) ([]app.CustomerExternalUser, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT b.customer_id, e.id, COALESCE(e.name,''), COALESCE(e.phone,''),
		       COALESCE(p.password_hash,'') <> '' AS has_password,
		       COALESCE(p.login_disabled,false) AS login_disabled,
		       b.status, to_char(b.updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.customer_erp_user_bindings b
		JOIN %s.company_employees e ON e.id=b.employee_id
		LEFT JOIN %s.employee_login_passwords p ON p.employee_id=e.id
		WHERE b.customer_id=$1 AND e.account_type='channel_customer'
		ORDER BY b.updated_at DESC, b.id DESC
	`, r.schema, r.schema, r.schema), customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]app.CustomerExternalUser, 0)
	for rows.Next() {
		var row app.CustomerExternalUser
		var loginDisabled bool
		if err := rows.Scan(&row.CustomerID, &row.EmployeeID, &row.Name, &row.Phone, &row.HasPassword, &loginDisabled, &row.BindingStatus, &row.UpdatedAt); err != nil {
			return nil, err
		}
		row.LoginEnabled = !loginDisabled
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repository) CreateExternalUser(ctx context.Context, cmd app.CreateExternalUserCommand) (app.CustomerExternalUser, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.CustomerExternalUser{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := r.requireCustomerERPWorkbenchTemplateTx(ctx, tx, cmd.CustomerID); err != nil {
		return app.CustomerExternalUser{}, err
	}
	var depID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.company_departments WHERE active=true ORDER BY id LIMIT 1`, r.schema)).Scan(&depID); err != nil {
		return app.CustomerExternalUser{}, fmt.Errorf("department not found")
	}
	var employeeID int64
	var accountType string
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id, COALESCE(NULLIF(account_type,''),'internal_employee') FROM %s.company_employees WHERE phone=$1 LIMIT 1`, r.schema), cmd.Phone).Scan(&employeeID, &accountType)
	if err == nil {
		if accountType != "channel_customer" {
			return app.CustomerExternalUser{}, fmt.Errorf("phone already belongs to internal employee")
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.company_employees
			SET name=$2, department_id=$3, active=true, updated_at=now()
			WHERE id=$1
		`, r.schema), employeeID, cmd.Name, depID); err != nil {
			return app.CustomerExternalUser{}, err
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.company_employees(name, phone, department_id, account_type, active, updated_at)
			VALUES($1,$2,$3,'channel_customer',true,now())
			RETURNING id
		`, r.schema), cmd.Name, cmd.Phone, depID).Scan(&employeeID); err != nil {
			return app.CustomerExternalUser{}, err
		}
	} else {
		return app.CustomerExternalUser{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.employee_login_passwords(employee_id,password_hash,login_disabled,must_reset_password,updated_at)
		VALUES($1,$2,false,true,now())
		ON CONFLICT (employee_id) DO UPDATE SET password_hash=excluded.password_hash,login_disabled=false,must_reset_password=true,updated_at=now()
	`, r.schema), employeeID, hashCustomerFulfillmentPassword(cmd.Password)); err != nil {
		return app.CustomerExternalUser{}, err
	}
	if err := r.activateExternalUserBindingTx(ctx, tx, cmd.CustomerID, employeeID, cmd.Actor); err != nil {
		return app.CustomerExternalUser{}, err
	}
	row, err := r.externalUserByID(ctx, tx, cmd.CustomerID, employeeID)
	if err != nil {
		return app.CustomerExternalUser{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return app.CustomerExternalUser{}, err
	}
	return row, nil
}

func (r *Repository) ResetExternalUserPassword(ctx context.Context, cmd app.ResetExternalUserPasswordCommand) (app.CustomerExternalUser, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.CustomerExternalUser{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := r.externalUserByID(ctx, tx, cmd.CustomerID, cmd.EmployeeID); err != nil {
		return app.CustomerExternalUser{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.employee_login_passwords(employee_id,password_hash,login_disabled,must_reset_password,updated_at)
		VALUES($1,$2,false,true,now())
		ON CONFLICT (employee_id) DO UPDATE SET password_hash=excluded.password_hash,login_disabled=false,must_reset_password=true,updated_at=now()
	`, r.schema), cmd.EmployeeID, hashCustomerFulfillmentPassword(cmd.Password)); err != nil {
		return app.CustomerExternalUser{}, err
	}
	row, err := r.externalUserByID(ctx, tx, cmd.CustomerID, cmd.EmployeeID)
	if err != nil {
		return app.CustomerExternalUser{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return app.CustomerExternalUser{}, err
	}
	return row, nil
}

func (r *Repository) SetExternalUserLoginEnabled(ctx context.Context, cmd app.SetExternalUserLoginEnabledCommand) (app.CustomerExternalUser, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.CustomerExternalUser{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := r.externalUserByID(ctx, tx, cmd.CustomerID, cmd.EmployeeID); err != nil {
		return app.CustomerExternalUser{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.employee_login_passwords(employee_id,password_hash,login_disabled,updated_at)
		VALUES($1,'',$2,now())
		ON CONFLICT (employee_id) DO UPDATE SET login_disabled=excluded.login_disabled,updated_at=now()
	`, r.schema), cmd.EmployeeID, !cmd.LoginEnabled); err != nil {
		return app.CustomerExternalUser{}, err
	}
	row, err := r.externalUserByID(ctx, tx, cmd.CustomerID, cmd.EmployeeID)
	if err != nil {
		return app.CustomerExternalUser{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return app.CustomerExternalUser{}, err
	}
	return row, nil
}

func (r *Repository) activateExternalUserBindingTx(ctx context.Context, tx pgx.Tx, customerID, employeeID int64, actor string) error {
	var otherCustomerID int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT customer_id
		FROM %s.customer_erp_user_bindings
		WHERE employee_id=$1 AND status='active' AND customer_id<>$2
		LIMIT 1
	`, r.schema), employeeID, customerID).Scan(&otherCustomerID)
	if err == nil && otherCustomerID > 0 {
		return fmt.Errorf("external user already bound to another customer")
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.customer_erp_user_bindings
		SET status='inactive', updated_by=$3, updated_at=now()
		WHERE customer_id=$1 AND status='active' AND employee_id<>$2
	`, r.schema), customerID, employeeID, actor); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_erp_user_bindings(customer_id, employee_id, role, status, updated_by, updated_at)
		VALUES($1,$2,'customer','active',$3,now())
		ON CONFLICT (employee_id, customer_id) DO UPDATE SET
			role='customer',
			status='active',
			updated_by=excluded.updated_by,
			updated_at=now()
	`, r.schema), customerID, employeeID, actor)
	return err
}

func (r *Repository) externalUserByID(ctx context.Context, q queryRower, customerID, employeeID int64) (app.CustomerExternalUser, error) {
	var row app.CustomerExternalUser
	var loginDisabled bool
	err := q.QueryRow(ctx, fmt.Sprintf(`
		SELECT b.customer_id, e.id, COALESCE(e.name,''), COALESCE(e.phone,''),
		       COALESCE(p.password_hash,'') <> '' AS has_password,
		       COALESCE(p.login_disabled,false) AS login_disabled,
		       b.status, to_char(b.updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.customer_erp_user_bindings b
		JOIN %s.company_employees e ON e.id=b.employee_id
		LEFT JOIN %s.employee_login_passwords p ON p.employee_id=e.id
		WHERE b.customer_id=$1 AND e.id=$2 AND e.account_type='channel_customer'
		LIMIT 1
	`, r.schema, r.schema, r.schema), customerID, employeeID).Scan(&row.CustomerID, &row.EmployeeID, &row.Name, &row.Phone, &row.HasPassword, &loginDisabled, &row.BindingStatus, &row.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.CustomerExternalUser{}, fmt.Errorf("external user not found")
	}
	if err != nil {
		return app.CustomerExternalUser{}, err
	}
	row.LoginEnabled = !loginDisabled
	return row, nil
}

func hashCustomerFulfillmentPassword(raw string) string {
	sum := sha256.Sum256([]byte("orderapp-mobile-auth:" + raw))
	return hex.EncodeToString(sum[:])
}

func (r *Repository) CustomerERPWorkbenchAvailable(ctx context.Context, customerID int64) (bool, error) {
	var templateKey string
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(capability_template_key,'')
		FROM %s.customer_portal_profiles
		WHERE customer_id=$1 AND enabled=true
	`, r.schema), customerID).Scan(&templateKey); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return r.customerERPWorkbenchAvailableForTemplateKey(ctx, r.pool, templateKey)
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
		row.CustomerProductDisplayName = strings.Join(strings.Fields(strings.TrimSpace(row.CustomerProductDisplayName)), " ")
		row.CustomerItemCode = strings.TrimSpace(row.CustomerItemCode)
		row.BrandName = strings.TrimSpace(row.BrandName)
		row.ProductCode = strings.TrimSpace(row.ProductCode)
		row.ProductRecordName = strings.Join(strings.Fields(strings.TrimSpace(row.ProductRecordName)), " ")
		row.Spec = strings.TrimSpace(row.Spec)
		row.RoastDegree = strings.TrimSpace(row.RoastDegree)
		row.Source = strings.TrimSpace(row.Source)
		row.ProductKind = catalogdomain.NormalizeProductKind(row.ProductKind)
		if row.ProductKind == catalogdomain.ProductKindDripBag {
			row.SalesUnits = []string{"bag", "box"}
		}
		if row.ProductName == "" {
			return
		}
		key := fmt.Sprintf("%d|%s|%s", row.ProductID, row.ProductName, row.Spec)
		if row.CustomerProductAliasID > 0 {
			key = fmt.Sprintf("alias:%d", row.CustomerProductAliasID)
		} else if row.ProductID > 0 {
			key = fmt.Sprintf("product:%d", row.ProductID)
		} else if row.SKUCode != "" {
			key = "sku:" + row.SKUCode
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, row)
	}

	if relationExists(ctx, r.pool, fmt.Sprintf("%s.customer_product_aliases", r.schema)) {
		rows, err := r.pool.Query(ctx, fmt.Sprintf(`
			SELECT p.id,
			       a.id,
			       COALESCE(NULLIF(a.display_name,''), p.name, ''),
			       COALESCE(a.customer_item_code,''),
			       COALESCE(a.brand_name,''),
			       'SKU-' || p.id::text,
			       COALESCE(p.name,''),
			       COALESCE(p.base_product_id,0),
			       COALESCE(a.customer_item_code,''),
			       COALESCE(NULLIF(a.display_name,''), p.name, ''),
			       COALESCE(NULLIF(p.product_kind,''), 'roasted_bean'),
			       COALESCE(p.drip_bag_grams,10)::float8,
			       COALESCE(p.drip_box_bag_count,10),
			       '', COALESCE(p.roast_level,''), COALESCE(p.default_price,0), 'customer_product_alias'
			FROM %s.customer_product_aliases a
			JOIN %s.products p ON p.id=a.product_id
			WHERE a.customer_id=$1
			  AND a.active=true
			  AND p.active=true
			ORDER BY a.sort_order, a.id
			LIMIT 300
		`, r.schema, r.schema), customerID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var row app.CustomerSKUOption
			if err := rows.Scan(&row.ProductID, &row.CustomerProductAliasID, &row.CustomerProductDisplayName, &row.CustomerItemCode, &row.BrandName, &row.ProductCode, &row.ProductRecordName, &row.BaseProductID, &row.SKUCode, &row.ProductName, &row.ProductKind, &row.DripBagGrams, &row.DripBoxBagCount, &row.Spec, &row.RoastDegree, &row.DefaultPrice, &row.Source); err != nil {
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
	}

	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT COALESCE(NULLIF(r.target_id,0), p.id, 0),
		       COALESCE(p.base_product_id,0),
		       COALESCE(r.payload->>'sku_code', ''),
		       COALESCE(NULLIF(r.payload->>'sku_name',''), NULLIF(r.payload->>'product_name',''), p.name, ''),
		       COALESCE(NULLIF(p.product_kind,''), 'roasted_bean'),
		       COALESCE(p.drip_bag_grams,10)::float8,
		       COALESCE(p.drip_box_bag_count,10),
		       COALESCE(NULLIF(r.payload->>'spec',''), ''),
		       COALESCE(NULLIF(r.payload->>'roast_degree',''), p.roast_level, ''),
		       COALESCE(p.default_price,0),
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
		if err := rows.Scan(&row.ProductID, &row.BaseProductID, &row.SKUCode, &row.ProductName, &row.ProductKind, &row.DripBagGrams, &row.DripBoxBagCount, &row.Spec, &row.RoastDegree, &row.DefaultPrice, &row.Source); err != nil {
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
		SELECT id, COALESCE(base_product_id,0), '', name,
		       COALESCE(NULLIF(product_kind,''), 'roasted_bean'),
		       COALESCE(drip_bag_grams,10)::float8,
		       COALESCE(drip_box_bag_count,10),
		       '', COALESCE(roast_level,''), COALESCE(default_price,0), COALESCE(NULLIF(custom_type,''),'customer_product')
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
		if err := rows.Scan(&row.ProductID, &row.BaseProductID, &row.SKUCode, &row.ProductName, &row.ProductKind, &row.DripBagGrams, &row.DripBoxBagCount, &row.Spec, &row.RoastDegree, &row.DefaultPrice, &row.Source); err != nil {
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
			SELECT id, 0, '公共SKU', name,
			       COALESCE(NULLIF(product_kind,''), 'roasted_bean'),
			       COALESCE(drip_bag_grams,10)::float8,
			       COALESCE(drip_box_bag_count,10),
			       '', COALESCE(roast_level,''), COALESCE(default_price,0), 'public_sku'
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
			if err := rows.Scan(&row.ProductID, &row.BaseProductID, &row.SKUCode, &row.ProductName, &row.ProductKind, &row.DripBagGrams, &row.DripBoxBagCount, &row.Spec, &row.RoastDegree, &row.DefaultPrice, &row.Source); err != nil {
				return nil, err
			}
			add(row)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	tiersByProduct, err := r.listProductTierOptions(ctx, customerID, out)
	if err != nil {
		return nil, err
	}
	for i := range out {
		if out[i].ProductID > 0 {
			out[i].Tiers = tiersByProduct[out[i].ProductID]
		}
	}
	return out, nil
}

func (r *Repository) listProductTierOptions(ctx context.Context, customerID int64, options []app.CustomerSKUOption) (map[int64][]app.CustomerSKUPriceTier, error) {
	productIDs := make([]int64, 0)
	seen := map[int64]struct{}{}
	for _, row := range options {
		if row.ProductID <= 0 {
			continue
		}
		if _, ok := seen[row.ProductID]; ok {
			continue
		}
		seen[row.ProductID] = struct{}{}
		productIDs = append(productIDs, row.ProductID)
	}
	if len(productIDs) == 0 {
		return map[int64][]app.CustomerSKUPriceTier{}, nil
	}
	if !relationExists(ctx, r.pool, fmt.Sprintf("%s.bean_list_publications", r.schema)) {
		return map[int64][]app.CustomerSKUPriceTier{}, nil
	}
	productSet := map[int64]struct{}{}
	for _, id := range productIDs {
		productSet[id] = struct{}{}
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
			SELECT id, list_type, COALESCE(content_json, '{}'::jsonb)
			FROM %s.bean_list_publications
			WHERE status='published'
			  AND list_type IN ('commercial','retail','green','drip')
			  AND (
			    (owner_type='customer' AND owner_key=$1)
			    OR owner_type='official'
			  )
			ORDER BY CASE WHEN owner_type='customer' AND owner_key=$1 THEN 0 ELSE 1 END,
			         published_at DESC,
			         id DESC
			LIMIT 50
		`, r.schema), fmt.Sprint(customerID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int64][]app.CustomerSKUPriceTier, len(productIDs))
	seenPublicationType := map[string]struct{}{}
	for rows.Next() {
		var publicationID int64
		var listType string
		var content []byte
		if err := rows.Scan(&publicationID, &listType, &content); err != nil {
			return nil, err
		}
		for productID, tiers := range customerFulfillmentPriceTiersFromPublication(publicationID, listType, content, productSet) {
			key := fmt.Sprintf("%d|%s", productID, strings.TrimSpace(listType))
			if _, ok := seenPublicationType[key]; ok {
				continue
			}
			seenPublicationType[key] = struct{}{}
			out[productID] = append(out[productID], tiers...)
		}
	}
	return out, rows.Err()
}

type customerFulfillmentPublishedContent struct {
	Groups []struct {
		Items []json.RawMessage `json:"items"`
	} `json:"groups"`
}

type customerFulfillmentPublishedTier struct {
	SourcePriceRecordID int64    `json:"source_price_record_id"`
	SpecG               int64    `json:"spec_g"`
	MinQty              float64  `json:"min_qty"`
	MaxQty              *float64 `json:"max_qty"`
	MinLb               float64  `json:"min_lb"`
	MaxLb               *float64 `json:"max_lb"`
	FinalUnitPrice      float64  `json:"final_unit_price"`
	PricePerUnit        float64  `json:"price_per_unit"`
	PricePerKg          float64  `json:"price_per_kg"`
	PricePerLb          float64  `json:"price_per_lb"`
	SalesUnit           string   `json:"sales_unit"`
	UnitBagCount        float64  `json:"unit_bag_count"`
	PackedPricePerBag   float64  `json:"packed_price_per_bag"`
	PackedPricePerBox   float64  `json:"packed_price_per_box"`
	PriceUnit           string   `json:"price_unit"`
	DisplayUnit         string   `json:"display_unit"`
}

func customerFulfillmentPriceTiersFromPublication(publicationID int64, listType string, content []byte, productSet map[int64]struct{}) map[int64][]app.CustomerSKUPriceTier {
	var parsed customerFulfillmentPublishedContent
	if len(content) == 0 || json.Unmarshal(content, &parsed) != nil {
		return nil
	}
	out := map[int64][]app.CustomerSKUPriceTier{}
	for _, group := range parsed.Groups {
		for _, item := range group.Items {
			productID := customerFulfillmentPublishedItemProductID(item)
			if productID <= 0 {
				continue
			}
			if _, ok := productSet[productID]; !ok {
				continue
			}
			tiers := customerFulfillmentPublishedItemTiers(item, listType)
			for _, tier := range tiers {
				option := customerFulfillmentPublishedTierOption(publicationID, listType, tier)
				if option.UnitPrice <= 0 {
					continue
				}
				out[productID] = append(out[productID], option)
			}
		}
	}
	return out
}

func customerFulfillmentPublishedItemProductID(raw json.RawMessage) int64 {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return 0
	}
	for _, key := range []string{"productId", "product_id", "productID"} {
		var id int64
		if data, ok := fields[key]; ok && json.Unmarshal(data, &id) == nil && id > 0 {
			return id
		}
	}
	return 0
}

func customerFulfillmentPublishedItemTiers(raw json.RawMessage, listType string) []customerFulfillmentPublishedTier {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return nil
	}
	key := "commercial_wholesale_tiers"
	switch strings.TrimSpace(listType) {
	case orderbeans.ListTypeRetail:
		key = "retail_bean_tiers"
	case orderbeans.ListTypeGreen:
		key = "green_bean_sale_tiers"
	case orderbeans.ListTypeDrip:
		key = "drip_wholesale_tiers"
	}
	var tiers []customerFulfillmentPublishedTier
	if data, ok := fields[key]; ok {
		_ = json.Unmarshal(data, &tiers)
	}
	return tiers
}

func customerFulfillmentPublishedTierOption(publicationID int64, listType string, tier customerFulfillmentPublishedTier) app.CustomerSKUPriceTier {
	specG := tier.SpecG
	if specG <= 0 {
		specG = 1000
	}
	productKind := catalogdomain.ProductKindRoastedBean
	if strings.TrimSpace(listType) == orderbeans.ListTypeGreen {
		productKind = catalogdomain.ProductKindGreenBean
	}
	if strings.TrimSpace(listType) == orderbeans.ListTypeDrip {
		productKind = catalogdomain.ProductKindDripBag
	}
	min := tier.MinQty
	if min <= 0 {
		min = tier.MinLb
	}
	unitPrice := tier.FinalUnitPrice
	if unitPrice <= 0 {
		unitPrice = tier.PricePerUnit
	}
	if unitPrice <= 0 && strings.TrimSpace(listType) == orderbeans.ListTypeDrip {
		if strings.TrimSpace(tier.SalesUnit) == "box" {
			unitPrice = tier.PackedPricePerBox
		} else {
			unitPrice = tier.PackedPricePerBag
		}
	}
	if unitPrice <= 0 {
		unitPrice = tier.PricePerKg
	}
	if unitPrice <= 0 {
		unitPrice = tier.PricePerLb
	}
	id := tier.SourcePriceRecordID
	if id <= 0 {
		id = publicationID
	}
	return app.CustomerSKUPriceTier{
		ID:           id,
		ProductKind:  productKind,
		SalesUnit:    strings.TrimSpace(tier.SalesUnit),
		SpecG:        specG,
		Min:          min,
		Max:          tier.MaxQty,
		UnitPrice:    unitPrice,
		UnitBagCount: tier.UnitBagCount,
		PriceSource:  "published_price_snapshot",
	}
}

type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func relationExists(ctx context.Context, q queryRower, relation string) bool {
	var exists bool
	if err := q.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, relation).Scan(&exists); err != nil {
		return false
	}
	return exists
}

func (r *Repository) customerCapabilityTemplateForCustomer(ctx context.Context, q queryRower, customerID int64) (customerportalapp.CapabilityTemplate, bool, error) {
	var raw string
	err := q.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(capability_template_key,'')
		FROM %s.customer_portal_profiles
		WHERE customer_id=$1 AND enabled=true
	`, r.schema), customerID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return customerportalapp.CapabilityTemplate{}, false, nil
	}
	if err != nil {
		return customerportalapp.CapabilityTemplate{}, false, err
	}
	key := customerportalapp.NormalizeCapabilityTemplateKey(raw)
	if strings.TrimSpace(raw) != "" && key == "" {
		return customerportalapp.CapabilityTemplate{}, false, customerportalapp.ErrCapabilityTemplateInvalid
	}
	if key == "" {
		return customerportalapp.CapabilityTemplate{}, false, nil
	}
	return r.customerCapabilityTemplateForKey(ctx, q, key)
}

func (r *Repository) customerCapabilityTemplateForKey(ctx context.Context, q queryRower, key string) (customerportalapp.CapabilityTemplate, bool, error) {
	key = customerportalapp.NormalizeCapabilityTemplateKey(key)
	if key == "" {
		return customerportalapp.CapabilityTemplate{}, false, customerportalapp.ErrCapabilityTemplateInvalid
	}
	var hasTable bool
	if err := q.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, fmt.Sprintf("%s.customer_capability_templates", r.schema)).Scan(&hasTable); err != nil {
		return customerportalapp.CapabilityTemplate{}, false, err
	}
	if hasTable {
		row := q.QueryRow(ctx, fmt.Sprintf(`
			SELECT template_key,
			       parent_template_key,
			       label,
			       description,
			       theme_key,
			       miniapp_entry_mode,
			       erp_role_codes,
			       erp_permissions,
			       erp_view_keys,
			       capabilities_json,
			       active,
			       sort_order,
			       to_char(updated_at,'YYYY-MM-DD HH24:MI'),
			       updated_by
			FROM %s.customer_capability_templates
			WHERE template_key=$1
		`, r.schema), key)
		template, err := scanCustomerCapabilityTemplate(row)
		if err == nil {
			if !template.Active {
				return customerportalapp.CapabilityTemplate{}, false, customerportalapp.ErrCapabilityTemplateInvalid
			}
			return template, true, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return customerportalapp.CapabilityTemplate{}, false, err
		}
	}
	if template, ok := customerportalapp.CustomerCapabilityTemplateByKey(key); ok && template.Active {
		return template, true, nil
	}
	return customerportalapp.CapabilityTemplate{}, false, customerportalapp.ErrCapabilityTemplateInvalid
}

func (r *Repository) customerERPWorkbenchAvailableForTemplateKey(ctx context.Context, q queryRower, templateKey string) (bool, error) {
	if strings.TrimSpace(templateKey) == "" {
		return false, nil
	}
	template, ok, err := r.customerCapabilityTemplateForKey(ctx, q, templateKey)
	if err != nil {
		if errors.Is(err, customerportalapp.ErrCapabilityTemplateInvalid) {
			return false, nil
		}
		return false, err
	}
	return ok && template.ExposesERPWorkbench(), nil
}

type capabilityTemplateScanner interface {
	Scan(dest ...any) error
}

func scanCustomerCapabilityTemplate(row capabilityTemplateScanner) (customerportalapp.CapabilityTemplate, error) {
	var template customerportalapp.CapabilityTemplate
	var roleCodesRaw, permissionsRaw, viewKeysRaw, capabilitiesRaw []byte
	if err := row.Scan(
		&template.Key,
		&template.ParentTemplateKey,
		&template.Label,
		&template.Description,
		&template.ThemeKey,
		&template.MiniappEntryMode,
		&roleCodesRaw,
		&permissionsRaw,
		&viewKeysRaw,
		&capabilitiesRaw,
		&template.Active,
		&template.SortOrder,
		&template.UpdatedAt,
		&template.UpdatedBy,
	); err != nil {
		return customerportalapp.CapabilityTemplate{}, err
	}
	template.ERPRoleCodes = decodeCustomerTemplateStringSlice(roleCodesRaw)
	template.ERPPermissions = decodeCustomerTemplateStringSlice(permissionsRaw)
	template.ERPViewKeys = decodeCustomerTemplateStringSlice(viewKeysRaw)
	if len(capabilitiesRaw) > 0 {
		_ = json.Unmarshal(capabilitiesRaw, &template.Capabilities)
	}
	if template.Capabilities == nil {
		template.Capabilities = []customerportalapp.CapabilityOption{}
	}
	return template, nil
}

func decodeCustomerTemplateStringSlice(raw []byte) []string {
	if len(raw) == 0 {
		return []string{}
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return []string{}
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func (r *Repository) listCustomerCapabilityCodes(ctx context.Context, customerID int64) ([]string, error) {
	if customerID <= 0 {
		return []string{}, nil
	}
	if template, ok, err := r.customerCapabilityTemplateForCustomer(ctx, r.pool, customerID); err != nil {
		return nil, err
	} else if ok {
		out := make([]string, 0, len(template.Capabilities))
		for _, capability := range template.Capabilities {
			if capability.Enabled && strings.TrimSpace(capability.Code) != "" {
				out = append(out, strings.TrimSpace(capability.Code))
			}
		}
		return out, nil
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
	if template, ok, err := r.customerCapabilityTemplateForCustomer(ctx, r.pool, customerID); err != nil {
		return false, err
	} else if ok {
		for _, capability := range template.Capabilities {
			if !capability.Enabled {
				continue
			}
			if capability.Code != "direct_ship" && capability.Code != "product_order" {
				continue
			}
			if strings.EqualFold(fmt.Sprint(capability.Config["public_sku_aliases"]), "true") {
				return true, nil
			}
		}
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
	SheetName   string
	RowNo       int
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
		SELECT id, sheet_name, row_no, row_type, external_key, payload
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
		if err := rows.Scan(&row.ID, &row.SheetName, &row.RowNo, &row.RowType, &row.ExternalKey, &payloadJSON); err != nil {
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

func (r *Repository) applyProcessingImportRow(ctx context.Context, tx pgx.Tx, customerID, batchID int64, state *processingApplyState, row importRowRecord) (applyTarget, error) {
	switch row.RowType {
	case "customer_sku":
		id, err := r.upsertCustomerProductTx(ctx, tx, customerID, row)
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
		id, err := r.applyProcessingWorkOrderTx(ctx, tx, customerID, batchID, state, row)
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

func (r *Repository) upsertCustomerProductTx(ctx context.Context, tx pgx.Tx, customerID int64, row importRowRecord) (int64, error) {
	name := payloadString(row.Payload, "sku_name", "product_name", "name")
	if name == "" {
		return 0, fmt.Errorf("customer sku name required")
	}
	roastDegree := payloadString(row.Payload, "roast_degree")
	if existingID, err := r.appliedCustomerProductIDByExternalKeyTx(ctx, tx, customerID, row.ExternalKey); err != nil {
		return 0, err
	} else if existingID > 0 {
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.products
			SET name=$2,
				roast_level=$3,
				active=true,
				visibility='customer_only',
				custom_type='processing_customer_sku'
			WHERE id=$1 AND customer_id=$4
		`, r.schema), existingID, name, roastDegree, customerID)
		return existingID, err
	}
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

func (r *Repository) appliedCustomerProductIDByExternalKeyTx(ctx context.Context, tx pgx.Tx, customerID int64, externalKey string) (int64, error) {
	externalKey = strings.TrimSpace(externalKey)
	if customerID <= 0 || externalKey == "" {
		return 0, nil
	}
	var productID int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT p.id
		FROM %s.customer_fulfillment_import_rows r
		JOIN %s.customer_fulfillment_import_batches b ON b.id=r.batch_id
		JOIN %s.products p ON p.id=r.target_id
		WHERE b.customer_id=$1
		  AND p.customer_id=$1
		  AND r.row_type='customer_sku'
		  AND r.external_key=$2
		  AND r.status='applied'
		  AND r.target_type='product'
		  AND r.target_id>0
		ORDER BY r.applied_at DESC NULLS LAST, r.id DESC
		LIMIT 1
	`, r.schema, r.schema, r.schema), customerID, externalKey).Scan(&productID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return productID, err
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
	previousItemID, previousItemName, previousDeltaG, previousDeltaUnits, appliedDeltaG, appliedDeltaUnits, err := r.upsertCustodyMovementLedgerTx(ctx, tx, customerID, itemID, "raw_bean", row, movementType, deltaG, 0)
	if err != nil {
		return 0, err
	}
	if previousItemID > 0 && (previousDeltaG != 0 || previousDeltaUnits != 0) {
		if _, err := r.addCustodyBalanceTx(ctx, tx, customerID, previousItemID, "raw_bean", previousItemName, "", previousDeltaG, previousDeltaUnits); err != nil {
			return 0, err
		}
	}
	if appliedDeltaG == 0 && appliedDeltaUnits == 0 {
		return itemID, nil
	}
	if _, err := r.addCustodyBalanceTx(ctx, tx, customerID, itemID, "raw_bean", name, "", appliedDeltaG, appliedDeltaUnits); err != nil {
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
	previousItemID, previousItemName, previousDeltaG, previousDeltaUnits, err := r.upsertCustodyBalanceAdjustmentLedgerTx(ctx, tx, customerID, itemID, "raw_bean", row, "balance_adjustment", qtyG, 0)
	if err != nil {
		return 0, err
	}
	if previousItemID > 0 && (previousDeltaG != 0 || previousDeltaUnits != 0) {
		if _, err := r.addCustodyBalanceTx(ctx, tx, customerID, previousItemID, "raw_bean", previousItemName, "", previousDeltaG, previousDeltaUnits); err != nil {
			return 0, err
		}
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
	previousItemID, previousItemName, previousDeltaG, previousDeltaUnits, err := r.upsertCustodyBalanceAdjustmentLedgerTx(ctx, tx, customerID, itemID, "packaging", row, "balance_adjustment", 0, units)
	if err != nil {
		return 0, err
	}
	if previousItemID > 0 && (previousDeltaG != 0 || previousDeltaUnits != 0) {
		if _, err := r.addCustodyBalanceTx(ctx, tx, customerID, previousItemID, "packaging", previousItemName, "", previousDeltaG, previousDeltaUnits); err != nil {
			return 0, err
		}
	}
	return r.setCustodyBalanceTx(ctx, tx, customerID, itemID, "packaging", name, "", 0, units)
}

func (r *Repository) applyProcessingWorkOrderTx(ctx context.Context, tx pgx.Tx, customerID, batchID int64, state *processingApplyState, row importRowRecord) (int64, error) {
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
	if state != nil {
		state.recordWorkOrder(id)
	}
	if rawBeanName != "" || inputQtyG != 0 {
		rawItemID, err := r.upsertCustodyItemTx(ctx, tx, customerID, "raw_bean", rawBeanName, rawBeanName, "g", map[string]any{"raw_bean_name": rawBeanName})
		if err != nil {
			return 0, err
		}
		if err := r.upsertProcessingWorkOrderInputTx(ctx, tx, id, rawItemID, rawBeanName, inputQtyG, row.Payload); err != nil {
			return 0, err
		}
		if state != nil {
			state.recordInput(id, rawBeanName)
		}
	}
	return id, nil
}

func (r *Repository) upsertProcessingWorkOrderInputTx(ctx context.Context, tx pgx.Tx, workOrderID, rawItemID int64, rawBeanName string, quantityG int64, payload map[string]any) error {
	var inputID int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id
		FROM %s.customer_processing_work_order_inputs
		WHERE work_order_id=$1 AND raw_bean_name=$2
		ORDER BY id
		LIMIT 1
		FOR UPDATE
	`, r.schema), workOrderID, rawBeanName).Scan(&inputID)
	if err == nil {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.customer_processing_work_order_inputs
			SET raw_bean_item_id=$2,
				quantity_g=$3,
				payload=$4::jsonb
			WHERE id=$1
		`, r.schema), inputID, rawItemID, quantityG, mustPayloadJSON(payload)); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			DELETE FROM %s.customer_processing_work_order_inputs
			WHERE work_order_id=$1 AND raw_bean_name=$2 AND id<>$3
		`, r.schema), workOrderID, rawBeanName, inputID)
		return err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_processing_work_order_inputs(work_order_id, raw_bean_item_id, raw_bean_name, quantity_g, payload)
		VALUES($1,$2,$3,$4,$5::jsonb)
	`, r.schema), workOrderID, rawItemID, rawBeanName, quantityG, mustPayloadJSON(payload))
	return err
}

func (r *Repository) applyPackagingJobTx(ctx context.Context, tx pgx.Tx, customerID, batchID int64, row importRowRecord) (int64, error) {
	workOrderNo := payloadString(row.Payload, "work_order_no")
	productName := payloadString(row.Payload, "product_name")
	packagingName := payloadString(row.Payload, "packaging_name")
	quantityUnits := payloadInt64(row.Payload, "quantity_units")
	if packagingName == "" {
		return 0, fmt.Errorf("packaging name required")
	}
	if _, err := r.upsertCustodyItemTx(ctx, tx, customerID, "packaging", packagingName, packagingName, "unit", row.Payload); err != nil {
		return 0, err
	}
	if workOrderNo != "" {
		id, found, err := r.updatePackagingJobByWorkOrderNoTx(ctx, tx, customerID, batchID, row.ExternalKey, workOrderNo, productName, packagingName, quantityUnits, row.Payload)
		if err != nil || found {
			return id, err
		}
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
		workOrderNo,
		productName,
		packagingName,
		quantityUnits,
		mustPayloadJSON(row.Payload),
	).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *Repository) updatePackagingJobByWorkOrderNoTx(ctx context.Context, tx pgx.Tx, customerID, batchID int64, externalKey, workOrderNo, productName, packagingName string, quantityUnits int64, payload map[string]any) (int64, bool, error) {
	var id int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id
		FROM %s.customer_processing_packaging_jobs
		WHERE customer_id=$1 AND work_order_no=$2
		ORDER BY id
		LIMIT 1
		FOR UPDATE
	`, r.schema), customerID, workOrderNo).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		DELETE FROM %s.customer_processing_packaging_jobs
		WHERE customer_id=$1 AND work_order_no=$2 AND id<>$3
	`, r.schema), customerID, workOrderNo, id); err != nil {
		return 0, false, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.customer_processing_packaging_jobs
		SET batch_id=$2,
			external_key=$3,
			product_name=$4,
			packaging_name=$5,
			quantity_units=$6,
			payload=$7::jsonb
		WHERE id=$1
	`, r.schema), id, batchID, externalKey, productName, packagingName, quantityUnits, mustPayloadJSON(payload)); err != nil {
		return 0, false, err
	}
	return id, true, nil
}

func (r *Repository) applyConversionJobTx(ctx context.Context, tx pgx.Tx, customerID, batchID int64, row importRowRecord) (int64, error) {
	jobNo := payloadString(row.Payload, "job_no")
	fromProduct := payloadString(row.Payload, "from_product")
	toProduct := payloadString(row.Payload, "to_product")
	quantityUnits := payloadInt64(row.Payload, "quantity_units")
	if jobNo != "" {
		id, found, err := r.updateConversionJobByJobNoTx(ctx, tx, customerID, batchID, row.ExternalKey, jobNo, fromProduct, toProduct, quantityUnits, row.Payload)
		if err != nil || found {
			return id, err
		}
	}
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
		jobNo,
		fromProduct,
		toProduct,
		quantityUnits,
		mustPayloadJSON(row.Payload),
	).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *Repository) updateConversionJobByJobNoTx(ctx context.Context, tx pgx.Tx, customerID, batchID int64, externalKey, jobNo, fromProduct, toProduct string, quantityUnits int64, payload map[string]any) (int64, bool, error) {
	var id int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id
		FROM %s.customer_inventory_conversion_jobs
		WHERE customer_id=$1 AND job_no=$2
		ORDER BY id
		LIMIT 1
		FOR UPDATE
	`, r.schema), customerID, jobNo).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.customer_inventory_conversion_jobs
		SET batch_id=$2,
			external_key=$3,
			from_product=$4,
			to_product=$5,
			quantity_units=$6,
			payload=$7::jsonb
		WHERE id=$1
	`, r.schema), id, batchID, externalKey, fromProduct, toProduct, quantityUnits, mustPayloadJSON(payload)); err != nil {
		return 0, false, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		DELETE FROM %s.customer_inventory_conversion_jobs
		WHERE customer_id=$1 AND job_no=$2 AND id<>$3
	`, r.schema), customerID, jobNo, id); err != nil {
		return 0, false, err
	}
	return id, true, nil
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

func (r *Repository) insertCustodyLedgerOnceTx(ctx context.Context, tx pgx.Tx, customerID, itemID int64, itemType, sourceType, sourceExternalKey, movementType string, deltaG, deltaUnits int64) (bool, error) {
	tag, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_custody_ledger_entries(
			customer_id, item_id, item_type, source_type, source_external_key, movement_type, qty_g_delta, qty_units_delta
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (customer_id, source_type, source_external_key, item_id, movement_type)
			WHERE source_external_key <> ''
		DO NOTHING
	`, r.schema), customerID, itemID, itemType, sourceType, sourceExternalKey, movementType, deltaG, deltaUnits)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *Repository) upsertCustodyMovementLedgerTx(ctx context.Context, tx pgx.Tx, customerID, itemID int64, itemType string, row importRowRecord, movementType string, deltaG, deltaUnits int64) (int64, string, int64, int64, int64, int64, error) {
	sourceType := row.RowType
	sourceExternalKey := strings.TrimSpace(row.ExternalKey)
	if strings.TrimSpace(sourceExternalKey) == "" {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.customer_custody_ledger_entries(
				customer_id, item_id, item_type, source_type, source_external_key, movement_type, qty_g_delta, qty_units_delta
			)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8)
		`, r.schema), customerID, itemID, itemType, sourceType, sourceExternalKey, movementType, deltaG, deltaUnits); err != nil {
			return 0, "", 0, 0, 0, 0, err
		}
		return 0, "", 0, 0, deltaG, deltaUnits, nil
	}

	var existingID, oldDeltaG, oldDeltaUnits int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, qty_g_delta, qty_units_delta
		FROM %s.customer_custody_ledger_entries
		WHERE customer_id=$1
		  AND source_type=$2
		  AND source_external_key=$3
		  AND item_id=$4
		  AND movement_type=$5
		FOR UPDATE
	`, r.schema), customerID, sourceType, sourceExternalKey, itemID, movementType).Scan(&existingID, &oldDeltaG, &oldDeltaUnits)
	if err == nil {
		if oldDeltaG != deltaG || oldDeltaUnits != deltaUnits {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`
				UPDATE %s.customer_custody_ledger_entries
				SET qty_g_delta=$2, qty_units_delta=$3
				WHERE id=$1
			`, r.schema), existingID, deltaG, deltaUnits); err != nil {
				return 0, "", 0, 0, 0, 0, err
			}
		}
		return 0, "", 0, 0, deltaG - oldDeltaG, deltaUnits - oldDeltaUnits, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, "", 0, 0, 0, 0, err
	}

	if row.SheetName != "" && row.RowNo > 0 {
		var previousID, previousItemID, previousDeltaG, previousDeltaUnits int64
		var previousItemName string
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT l.id, l.item_id, COALESCE(i.item_name,''), l.qty_g_delta, l.qty_units_delta
			FROM %s.customer_fulfillment_import_rows r
			JOIN %s.customer_fulfillment_import_batches b ON b.id=r.batch_id
			JOIN %s.customer_custody_ledger_entries l
			  ON l.customer_id=b.customer_id
			 AND l.item_id=r.target_id
			 AND l.source_type=r.row_type
			 AND l.source_external_key=r.external_key
			 AND l.movement_type=$5
			LEFT JOIN %s.customer_custody_items i ON i.id=l.item_id
			WHERE b.customer_id=$1
			  AND r.row_type=$2
			  AND r.sheet_name=$3
			  AND r.row_no=$4
			  AND r.status='applied'
			  AND r.target_type='customer_custody_item'
			  AND r.target_id>0
			ORDER BY r.applied_at DESC NULLS LAST, r.id DESC
			LIMIT 1
			FOR UPDATE OF l
		`, r.schema, r.schema, r.schema, r.schema), customerID, sourceType, row.SheetName, row.RowNo, movementType).Scan(&previousID, &previousItemID, &previousItemName, &previousDeltaG, &previousDeltaUnits)
		if err == nil {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`
				UPDATE %s.customer_custody_ledger_entries
				SET item_id=$2,
					item_type=$3,
					source_external_key=$4,
					qty_g_delta=$5,
					qty_units_delta=$6
				WHERE id=$1
			`, r.schema), previousID, itemID, itemType, sourceExternalKey, deltaG, deltaUnits); err != nil {
				return 0, "", 0, 0, 0, 0, err
			}
			if previousItemID != itemID {
				return previousItemID, previousItemName, -previousDeltaG, -previousDeltaUnits, deltaG, deltaUnits, nil
			}
			return 0, "", 0, 0, deltaG - previousDeltaG, deltaUnits - previousDeltaUnits, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return 0, "", 0, 0, 0, 0, err
		}
	}

	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_custody_ledger_entries(
			customer_id, item_id, item_type, source_type, source_external_key, movement_type, qty_g_delta, qty_units_delta
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)
	`, r.schema), customerID, itemID, itemType, sourceType, sourceExternalKey, movementType, deltaG, deltaUnits); err != nil {
		return 0, "", 0, 0, 0, 0, err
	}
	return 0, "", 0, 0, deltaG, deltaUnits, nil
}

func (r *Repository) upsertCustodyBalanceAdjustmentLedgerTx(ctx context.Context, tx pgx.Tx, customerID, itemID int64, itemType string, row importRowRecord, movementType string, targetG, targetUnits int64) (int64, string, int64, int64, error) {
	currentG, currentUnits, err := r.currentCustodyBalanceTx(ctx, tx, customerID, itemID, itemType)
	if err != nil {
		return 0, "", 0, 0, err
	}
	sourceType := row.RowType
	sourceExternalKey := strings.TrimSpace(row.ExternalKey)
	if sourceExternalKey == "" {
		_, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.customer_custody_ledger_entries(
				customer_id, item_id, item_type, source_type, source_external_key, movement_type, qty_g_delta, qty_units_delta
			)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8)
		`, r.schema), customerID, itemID, itemType, sourceType, sourceExternalKey, movementType, targetG-currentG, targetUnits-currentUnits)
		return 0, "", 0, 0, err
	}

	var existingID, oldDeltaG, oldDeltaUnits int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, qty_g_delta, qty_units_delta
		FROM %s.customer_custody_ledger_entries
		WHERE customer_id=$1
		  AND source_type=$2
		  AND source_external_key=$3
		  AND item_id=$4
		  AND movement_type=$5
		FOR UPDATE
	`, r.schema), customerID, sourceType, sourceExternalKey, itemID, movementType).Scan(&existingID, &oldDeltaG, &oldDeltaUnits)
	if err == nil {
		baseG := currentG - oldDeltaG
		baseUnits := currentUnits - oldDeltaUnits
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.customer_custody_ledger_entries
			SET qty_g_delta=$2, qty_units_delta=$3
			WHERE id=$1
		`, r.schema), existingID, targetG-baseG, targetUnits-baseUnits)
		return 0, "", 0, 0, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, "", 0, 0, err
	}

	if row.SheetName != "" && row.RowNo > 0 {
		var previousID, previousItemID, previousDeltaG, previousDeltaUnits int64
		var previousItemName string
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT l.id, l.item_id, COALESCE(i.item_name, bal.item_name, ''), l.qty_g_delta, l.qty_units_delta
			FROM %s.customer_fulfillment_import_rows r
			JOIN %s.customer_fulfillment_import_batches b ON b.id=r.batch_id
			JOIN %s.customer_custody_balances bal
			  ON bal.id=r.target_id
			 AND bal.customer_id=b.customer_id
			 AND bal.item_type=$6
			JOIN %s.customer_custody_ledger_entries l
			  ON l.customer_id=b.customer_id
			 AND l.item_id=bal.item_id
			 AND l.source_type=r.row_type
			 AND l.source_external_key=r.external_key
			 AND l.movement_type=$5
			LEFT JOIN %s.customer_custody_items i ON i.id=l.item_id
			WHERE b.customer_id=$1
			  AND r.row_type=$2
			  AND r.sheet_name=$3
			  AND r.row_no=$4
			  AND r.status='applied'
			  AND r.target_type='customer_custody_balance'
			  AND r.target_id>0
			ORDER BY r.applied_at DESC NULLS LAST, r.id DESC
			LIMIT 1
			FOR UPDATE OF l
		`, r.schema, r.schema, r.schema, r.schema, r.schema), customerID, sourceType, row.SheetName, row.RowNo, movementType, itemType).Scan(&previousID, &previousItemID, &previousItemName, &previousDeltaG, &previousDeltaUnits)
		if err == nil {
			newDeltaG := targetG - currentG
			newDeltaUnits := targetUnits - currentUnits
			if previousItemID == itemID {
				newDeltaG = targetG - (currentG - previousDeltaG)
				newDeltaUnits = targetUnits - (currentUnits - previousDeltaUnits)
			}
			if _, err := tx.Exec(ctx, fmt.Sprintf(`
				UPDATE %s.customer_custody_ledger_entries
				SET item_id=$2,
					item_type=$3,
					source_external_key=$4,
					qty_g_delta=$5,
					qty_units_delta=$6
				WHERE id=$1
			`, r.schema), previousID, itemID, itemType, sourceExternalKey, newDeltaG, newDeltaUnits); err != nil {
				return 0, "", 0, 0, err
			}
			if previousItemID != itemID {
				return previousItemID, previousItemName, -previousDeltaG, -previousDeltaUnits, nil
			}
			return 0, "", 0, 0, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return 0, "", 0, 0, err
		}
	}

	_, err = tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_custody_ledger_entries(
			customer_id, item_id, item_type, source_type, source_external_key, movement_type, qty_g_delta, qty_units_delta
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)
	`, r.schema), customerID, itemID, itemType, sourceType, sourceExternalKey, movementType, targetG-currentG, targetUnits-currentUnits)
	return 0, "", 0, 0, err
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

type processingApplyState struct {
	inputNamesByWorkOrderID map[int64]map[string]struct{}
}

func newProcessingApplyState() *processingApplyState {
	return &processingApplyState{inputNamesByWorkOrderID: map[int64]map[string]struct{}{}}
}

func (s *processingApplyState) recordWorkOrder(workOrderID int64) {
	if workOrderID <= 0 {
		return
	}
	if s.inputNamesByWorkOrderID[workOrderID] == nil {
		s.inputNamesByWorkOrderID[workOrderID] = map[string]struct{}{}
	}
}

func (s *processingApplyState) recordInput(workOrderID int64, rawBeanName string) {
	rawBeanName = strings.TrimSpace(rawBeanName)
	if workOrderID <= 0 || rawBeanName == "" {
		return
	}
	s.recordWorkOrder(workOrderID)
	s.inputNamesByWorkOrderID[workOrderID][rawBeanName] = struct{}{}
}

func (r *Repository) trimProcessingWorkOrderStaleInputsTx(ctx context.Context, tx pgx.Tx, state *processingApplyState) error {
	for workOrderID, inputNames := range state.inputNamesByWorkOrderID {
		if workOrderID <= 0 {
			continue
		}
		if len(inputNames) == 0 {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`
				DELETE FROM %s.customer_processing_work_order_inputs
				WHERE work_order_id=$1
			`, r.schema), workOrderID); err != nil {
				return err
			}
			if err := r.refreshProcessingWorkOrderInputTotalTx(ctx, tx, workOrderID); err != nil {
				return err
			}
			continue
		}
		args := []any{workOrderID}
		placeholders := make([]string, 0, len(inputNames))
		for name := range inputNames {
			args = append(args, name)
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			DELETE FROM %s.customer_processing_work_order_inputs
			WHERE work_order_id=$1
			  AND raw_bean_name NOT IN (%s)
		`, r.schema, strings.Join(placeholders, ",")), args...); err != nil {
			return err
		}
		if err := r.refreshProcessingWorkOrderInputTotalTx(ctx, tx, workOrderID); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) refreshProcessingWorkOrderInputTotalTx(ctx context.Context, tx pgx.Tx, workOrderID int64) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.customer_processing_work_orders
		SET input_quantity_g=(
				SELECT COALESCE(SUM(quantity_g),0)
				FROM %s.customer_processing_work_order_inputs
				WHERE work_order_id=$1
			),
			updated_at=now()
		WHERE id=$1
	`, r.schema, r.schema), workOrderID)
	return err
}

type directShipApplyState struct {
	importOrderIDs       map[string]int64
	orderIDs             map[string]int64
	lineNoByOrder        map[string]int
	trackingNosByOrderID map[int64]map[string]struct{}
}

func newDirectShipApplyState() *directShipApplyState {
	return &directShipApplyState{
		importOrderIDs:       map[string]int64{},
		orderIDs:             map[string]int64{},
		lineNoByOrder:        map[string]int{},
		trackingNosByOrderID: map[int64]map[string]struct{}{},
	}
}

func (s *directShipApplyState) recordTrackings(orderID int64, raw string) {
	if orderID <= 0 {
		return
	}
	if s.trackingNosByOrderID[orderID] == nil {
		s.trackingNosByOrderID[orderID] = map[string]struct{}{}
	}
	numbers := normalizeCustomerFulfillmentTrackings(raw)
	for _, no := range numbers {
		s.trackingNosByOrderID[orderID][no] = struct{}{}
	}
}

func (r *Repository) trimDirectShipStaleLinesTx(ctx context.Context, tx pgx.Tx, state *directShipApplyState) error {
	for orderNo, lineCount := range state.lineNoByOrder {
		if lineCount <= 0 {
			continue
		}
		importOrderID := state.importOrderIDs[orderNo]
		orderID := state.orderIDs[orderNo]
		if importOrderID <= 0 || orderID <= 0 {
			continue
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			DELETE FROM %s.customer_direct_ship_import_order_items
			WHERE import_order_id=$1 AND line_no>$2
		`, r.schema), importOrderID, lineCount); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			DELETE FROM %s.order_items
			WHERE order_id=$1 AND line_no>$2
		`, r.schema), orderID, lineCount); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) trimDirectShipStaleTrackingsTx(ctx context.Context, tx pgx.Tx, state *directShipApplyState) error {
	for orderID, trackingSet := range state.trackingNosByOrderID {
		if orderID <= 0 {
			continue
		}
		args := []any{orderID}
		retainClause := ""
		if len(trackingSet) > 0 {
			placeholders := make([]string, 0, len(trackingSet))
			for no := range trackingSet {
				args = append(args, no)
				placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
			}
			retainClause = fmt.Sprintf("AND tracking_no NOT IN (%s)", strings.Join(placeholders, ","))
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			DELETE FROM %s.order_shipping_trackings
			WHERE order_id=$1
			  AND source IN ('customer_fulfillment_direct_ship','customer_fulfillment_direct_ship_item')
			  %s
		`, r.schema, retainClause), args...); err != nil {
			return err
		}
		if _, err := refreshCustomerFulfillmentOrderTrackingSummaryTx(ctx, tx, r.schema, orderID); err != nil {
			return err
		}
	}
	return nil
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
		state.recordTrackings(orderID, payloadString(row.Payload, "waybill_no"))
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
	orderDate := parseDateValue(payloadString(row.Payload, "order_date"))
	remark := payloadString(row.Payload, "remark")
	warehouse, err := r.customerProcessingWarehouseTx(ctx, tx, customerID)
	if err != nil {
		return 0, 0, err
	}
	shipStatusID := directShipImportShipStatusID(ctx, tx, r.schema, payloadString(row.Payload, "status"))
	waybillNo := trackingSummaryFromCustomerFulfillment(payloadString(row.Payload, "waybill_no"))
	var importOrderID, orderID int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, order_id
		FROM %s.customer_direct_ship_import_orders
		WHERE customer_id=$1 AND external_order_no=$2
		ORDER BY id
		LIMIT 1
		FOR UPDATE
	`, r.schema), customerID, orderNo).Scan(&importOrderID, &orderID)
	if err == nil {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.customer_direct_ship_import_orders
			SET batch_id=$2,
				external_seq=$3,
				order_date=$4,
				receiver_address=$5,
				status=$6,
				payload=$7::jsonb
			WHERE id=$1
		`, r.schema),
			importOrderID,
			batchID,
			seq,
			orderDate,
			payloadString(row.Payload, "receiver_address"),
			payloadString(row.Payload, "status"),
			mustPayloadJSON(row.Payload),
		); err != nil {
			return 0, 0, err
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
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
			orderDate,
			payloadString(row.Payload, "receiver_address"),
			payloadString(row.Payload, "status"),
			mustPayloadJSON(row.Payload),
		).Scan(&importOrderID, &orderID); err != nil {
			return 0, 0, err
		}
	} else {
		return 0, 0, err
	}
	if orderID <= 0 {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.orders(
				order_no, order_date, customer_id, ship_status_id, portal_service_code,
				receiver_name, receiver_phone, receiver_address,
				ship_tracking_no, source_warehouse, notes, created_at
			)
			VALUES($1,$2,$3,$4,'direct_ship',$5,$6,$7,$8,$9,$10,now())
			RETURNING id
		`, r.schema),
			orderNo,
			orderDate,
			customerID,
			nullableCustomerFulfillmentID(shipStatusID),
			receiverName,
			receiverPhone,
			receiverAddress,
			waybillNo,
			warehouse,
			remark,
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
				order_date=$7,
				notes=$8,
				ship_status_id=COALESCE($9::bigint, ship_status_id),
				portal_service_code='direct_ship'
			WHERE id=$1
		`, r.schema), orderID, receiverName, receiverPhone, receiverAddress, waybillNo, warehouse, orderDate, remark, nullableCustomerFulfillmentID(shipStatusID)); err != nil {
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

func directShipImportShipStatusID(ctx context.Context, tx pgx.Tx, schema, rawStatus string) int64 {
	status := strings.TrimSpace(rawStatus)
	switch {
	case status == "" || strings.Contains(status, "未发货"):
		return customerFulfillmentStatusID(ctx, tx, schema, "ship_statuses", "未发货", "待发货")
	case strings.Contains(status, "已发货") || strings.Contains(status, "已发") || strings.EqualFold(status, "shipped"):
		return customerFulfillmentStatusID(ctx, tx, schema, "ship_statuses", "已发货")
	case strings.Contains(status, "待发货"):
		return customerFulfillmentStatusID(ctx, tx, schema, "ship_statuses", "待发货", "未发货")
	default:
		return 0
	}
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
	usage := orderbeans.Usage{}
	if matchedProductID > 0 {
		var usageErr error
		usage, usageErr = orderbeans.ResolveUsage(ctx, tx, r.schema, customerID, matchedProductID, orderbeans.ListTypeCommercial)
		if usageErr != nil {
			return 0, usageErr
		}
	}
	quantity := payloadInt64(row.Payload, "quantity_units")
	var importItemID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id
		FROM %s.customer_direct_ship_import_order_items
		WHERE import_order_id=$1 AND line_no=$2
		ORDER BY id
		LIMIT 1
		FOR UPDATE
	`, r.schema), importOrderID, lineNo).Scan(&importItemID); err == nil {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.customer_direct_ship_import_order_items
			SET batch_id=$2,
				customer_id=$3,
				product_title=$4,
				spec=$5,
				quantity_units=$6,
				payload=$7::jsonb
			WHERE id=$1
		`, r.schema), importItemID, batchID, customerID, productTitle, payloadString(row.Payload, "spec"), quantity, mustPayloadJSON(row.Payload)); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			DELETE FROM %s.customer_direct_ship_import_order_items
			WHERE import_order_id=$1 AND line_no=$2 AND id<>$3
		`, r.schema), importOrderID, lineNo, importItemID); err != nil {
			return 0, err
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
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
	} else {
		return 0, err
	}
	var orderItemID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id
		FROM %s.order_items
		WHERE order_id=$1 AND line_no=$2
		ORDER BY id
		LIMIT 1
		FOR UPDATE
	`, r.schema), orderID, lineNo).Scan(&orderItemID); err == nil {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.order_items
			SET product_id=$2,
				item_name=$3,
				qty=$4,
				unit='件',
				spec=$5,
				unit_price=0,
				line_total=0,
				bean_list_publication_id=NULLIF($6,0),
				bean_list_version_no=$7
			WHERE id=$1
		`, r.schema), orderItemID, productID, productTitle, quantity, payloadString(row.Payload, "spec"), usage.PublicationID, usage.VersionNo); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			DELETE FROM %s.order_items
			WHERE order_id=$1 AND line_no=$2 AND id<>$3
		`, r.schema), orderID, lineNo, orderItemID); err != nil {
			return 0, err
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.order_items(order_id, line_no, product_id, item_name, qty, unit, spec, unit_price, line_total, bean_list_publication_id, bean_list_version_no)
			VALUES($1,$2,$3,$4,$5,'件',$6,0,0,NULLIF($7,0),$8)
		`, r.schema), orderID, lineNo, productID, productTitle, quantity, payloadString(row.Payload, "spec"), usage.PublicationID, usage.VersionNo); err != nil {
			return 0, err
		}
	} else {
		return 0, err
	}
	if waybillNo := payloadString(row.Payload, "waybill_no"); waybillNo != "" {
		state.recordTrackings(orderID, waybillNo)
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
	return refreshCustomerFulfillmentOrderTrackingSummaryTx(ctx, tx, schema, orderID)
}

func refreshCustomerFulfillmentOrderTrackingSummaryTx(ctx context.Context, tx pgx.Tx, schema string, orderID int64) (string, error) {
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
	amountCents := payloadInt64(row.Payload, "amount_cents")
	amount := fmt.Sprintf("%.2f", float64(amountCents)/100)
	occurredAt := parseDateValue(payloadString(row.Payload, "date"))
	if occurredAt == nil {
		occurredAt = time.Now()
	}
	note := payloadString(row.Payload, "fee_name")
	if existingID, err := r.appliedSettlementFeeItemIDByExternalKeyTx(ctx, tx, customerID, row.ExternalKey); err != nil {
		return applyTarget{}, err
	} else if existingID > 0 {
		if err := r.refreshImportedSettlementFeeItemTx(ctx, tx, customerID, existingID, row.ID, mappedFeeType, amount, occurredAt, note); err != nil {
			return applyTarget{}, err
		}
		return applyTarget{TargetType: "customer_fee_item", TargetID: existingID, FeeItems: 1}, nil
	}
	if existingID, err := r.appliedSettlementFeeItemIDByFeeLineTx(ctx, tx, customerID, row.SheetName, row.RowNo); err != nil {
		return applyTarget{}, err
	} else if existingID > 0 {
		if err := r.refreshImportedSettlementFeeItemTx(ctx, tx, customerID, existingID, row.ID, mappedFeeType, amount, occurredAt, note); err != nil {
			return applyTarget{}, err
		}
		return applyTarget{TargetType: "customer_fee_item", TargetID: existingID, FeeItems: 1}, nil
	}
	var existingID int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id
		FROM %s.customer_fee_items
		WHERE source_type='customer_fulfillment_import' AND source_id=$1
	`, r.schema), row.ID).Scan(&existingID)
	if err == nil {
		if err := r.refreshImportedSettlementFeeItemTx(ctx, tx, customerID, existingID, row.ID, mappedFeeType, amount, occurredAt, note); err != nil {
			return applyTarget{}, err
		}
		return applyTarget{TargetType: "customer_fee_item", TargetID: existingID, FeeItems: 1}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return applyTarget{}, err
	}
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

func (r *Repository) refreshImportedSettlementFeeItemTx(ctx context.Context, tx pgx.Tx, customerID, feeID, rowID int64, feeType, amount string, occurredAt any, note string) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.customer_fee_items
		SET source_id=$3,
			fee_type=$4,
			amount=$5,
			occurred_at=$6,
			note=$7
		WHERE id=$1
		  AND customer_id=$2
		  AND source_type='customer_fulfillment_import'
		  AND status='unsettled'
		  AND settlement_batch_id=0
	`, r.schema), feeID, customerID, rowID, feeType, amount, occurredAt, note)
	return err
}

func (r *Repository) appliedSettlementFeeItemIDByExternalKeyTx(ctx context.Context, tx pgx.Tx, customerID int64, externalKey string) (int64, error) {
	externalKey = strings.TrimSpace(externalKey)
	if customerID <= 0 || externalKey == "" {
		return 0, nil
	}
	var feeID int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT f.id
		FROM %s.customer_fulfillment_import_rows r
		JOIN %s.customer_fulfillment_import_batches b ON b.id=r.batch_id
		JOIN %s.customer_fee_items f ON f.id=r.target_id
		WHERE b.customer_id=$1
		  AND f.customer_id=$1
		  AND r.row_type='fee_item'
		  AND r.external_key=$2
		  AND r.status='applied'
		  AND r.target_type='customer_fee_item'
		  AND r.target_id>0
		ORDER BY r.applied_at DESC NULLS LAST, r.id DESC
		LIMIT 1
	`, r.schema, r.schema, r.schema), customerID, externalKey).Scan(&feeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return feeID, err
}

func (r *Repository) appliedSettlementFeeItemIDByFeeLineTx(ctx context.Context, tx pgx.Tx, customerID int64, sheetName string, rowNo int) (int64, error) {
	sheetName = strings.TrimSpace(sheetName)
	if customerID <= 0 || sheetName == "" || rowNo <= 0 {
		return 0, nil
	}
	var feeID int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT f.id
		FROM %s.customer_fulfillment_import_rows r
		JOIN %s.customer_fulfillment_import_batches b ON b.id=r.batch_id
		JOIN %s.customer_fee_items f ON f.id=r.target_id
		WHERE b.customer_id=$1
		  AND f.customer_id=$1
		  AND r.row_type='fee_item'
		  AND r.sheet_name=$2
		  AND r.row_no=$3
		  AND r.status='applied'
		  AND r.target_type='customer_fee_item'
		  AND r.target_id>0
		ORDER BY r.applied_at DESC NULLS LAST, r.id DESC
		LIMIT 1
	`, r.schema, r.schema, r.schema), customerID, sheetName, rowNo).Scan(&feeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return feeID, err
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
	if err := r.requireCustomerCapability(ctx, cmd.CustomerID, "settlement"); err != nil {
		return app.SettlementResult{}, err
	}
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
	var batchStatus string
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, status
		FROM %s.customer_settlement_batches
		WHERE settlement_no=$1
		FOR UPDATE
	`, r.schema), settlementNo).Scan(&batchID, &batchStatus)
	if err == nil {
		if strings.TrimSpace(batchStatus) != "draft" {
			return app.SettlementResult{}, fmt.Errorf("settlement batch is not draft")
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.customer_settlement_batches
			SET period_from=$2,
				period_to=$3,
				created_by=$4
			WHERE id=$1
		`, r.schema), batchID, periodFrom, periodTo, strings.TrimSpace(cmd.CreatedBy)); err != nil {
			return app.SettlementResult{}, err
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.customer_settlement_batches(customer_id, settlement_no, period_from, period_to, status, total_amount, created_by)
			VALUES($1,$2,$3,$4,'draft',0,$5)
			RETURNING id
		`, r.schema), cmd.CustomerID, settlementNo, periodFrom, periodTo, strings.TrimSpace(cmd.CreatedBy)).Scan(&batchID); err != nil {
			return app.SettlementResult{}, err
		}
	} else {
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
	var feeItems int
	var totalCents int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*), COALESCE(ROUND(SUM(amount) * 100),0)::bigint
		FROM %s.customer_fee_items
		WHERE customer_id=$1
			AND settlement_batch_id=$2
	`, r.schema), cmd.CustomerID, batchID).Scan(&feeItems, &totalCents); err != nil {
		return app.SettlementResult{}, err
	}
	if feeItems == 0 {
		return app.SettlementResult{}, fmt.Errorf("no fees for settlement period")
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
	if err := r.requireActiveCustomerERPWorkbenchBinding(ctx, query.CustomerID); err != nil {
		return app.Overview{}, err
	}
	var overview app.Overview
	overview.CustomerID = query.CustomerID
	_ = r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT name FROM %s.customers WHERE id=$1`, r.schema), query.CustomerID).Scan(&overview.CustomerName)
	var err error
	if overview.Capabilities, err = r.listCustomerCapabilityCodes(ctx, query.CustomerID); err != nil {
		return app.Overview{}, err
	}
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
		WITH direct_ship_rows AS (
			SELECT o.external_order_no AS order_no,
				COALESCE(to_char(o.order_date, 'YYYY-MM-DD'), '') AS order_date,
				o.receiver_address AS receiver_address,
				o.status AS status,
				COUNT(i.id)::int AS item_count,
				o.created_at AS sort_at,
				o.id AS sort_id
			FROM %[1]s.customer_direct_ship_import_orders o
			LEFT JOIN %[1]s.customer_direct_ship_import_order_items i ON i.import_order_id=o.id
			WHERE o.customer_id=$1
			GROUP BY o.id

			UNION ALL

			SELECT o.order_no,
				COALESCE(to_char(o.order_date, 'YYYY-MM-DD'), ''),
				concat_ws(' ', NULLIF(o.receiver_name,''), NULLIF(o.receiver_phone,''), NULLIF(o.receiver_address,'')),
				COALESCE(NULLIF(ss.name,''), NULLIF(ops.name,''), CASE WHEN o.is_void THEN '已作废' ELSE '' END),
				COUNT(i.id)::int,
				o.created_at,
				o.id
			FROM %[1]s.orders o
			LEFT JOIN %[1]s.order_items i ON i.order_id=o.id
			LEFT JOIN %[1]s.ship_statuses ss ON ss.id=o.ship_status_id
			LEFT JOIN %[1]s.order_process_statuses ops ON ops.id=o.process_status_id
			WHERE o.customer_id=$1
			  AND o.portal_service_code IN ('direct_ship','processing_ship')
			  AND NOT EXISTS (
			      SELECT 1
			      FROM %[1]s.customer_direct_ship_import_orders imported
			      WHERE imported.order_id=o.id
			  )
			GROUP BY o.id, ss.name, ops.name
		)
		SELECT order_no, order_date, receiver_address, status, item_count
		FROM direct_ship_rows
		ORDER BY sort_at DESC, sort_id DESC
		LIMIT 100
	`, r.schema), customerID)
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

func payloadFloat(payload map[string]any, key string) float64 {
	value, ok := payload[key]
	if !ok || value == nil {
		return 0
	}
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case int32:
		return float64(v)
	case json.Number:
		n, _ := v.Float64()
		return n
	case string:
		n, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return n
	default:
		n, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(v)), 64)
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
