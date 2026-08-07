package sales

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	salesapp "orderapp/internal/application/sales"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
)

func (r Repository) ListProcessingBillingCustomerOptions(ctx context.Context) ([]salesapp.ProcessingBillingCustomerOption, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT d.customer_id,COALESCE(c.name,'')
		FROM %s.customer_processing_production_demands d
		JOIN %s.customers c ON c.id=d.customer_id
		WHERE d.customer_id>0
		GROUP BY d.customer_id,c.name
		ORDER BY c.name,d.customer_id
	`, r.schema, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]salesapp.ProcessingBillingCustomerOption, 0)
	for rows.Next() {
		var row salesapp.ProcessingBillingCustomerOption
		if err := rows.Scan(&row.CustomerID, &row.CustomerName); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) ListProcessingBillingCandidates(ctx context.Context, customerID int64) ([]salesapp.ProcessingBillingCandidate, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT DISTINCT ON (wo.id)
		       wo.id,COALESCE(wo.work_order_no,''),d.customer_id,COALESCE(c.name,''),COALESCE(d.request_no,''),
		       COALESCE(wo.product_id,0),COALESCE(wo.product_name,''),COALESCE(wo.spec_g,0),COALESCE(wo.status,''),
		       COALESCE(to_char(wo.completed_at,'YYYY-MM-DD HH24:MI'),''),
		       (bwo.id IS NOT NULL),COALESCE(bwo.billing_run_id,0)
		FROM %s.customer_processing_production_demands d
		JOIN %s.work_orders wo ON wo.id=d.linked_work_order_id
		JOIN %s.customers c ON c.id=d.customer_id
		LEFT JOIN %s.processing_billing_work_orders bwo ON bwo.work_order_id=wo.id AND bwo.run_kind='standard'
		WHERE d.customer_id=$1 AND wo.status='completed'
		ORDER BY wo.id,d.id
	`, r.schema, r.schema, r.schema, r.schema), customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]salesapp.ProcessingBillingCandidate, 0)
	for rows.Next() {
		var row salesapp.ProcessingBillingCandidate
		if err := rows.Scan(
			&row.WorkOrderID, &row.WorkOrderNo, &row.CustomerID, &row.CustomerName, &row.RequestNo,
			&row.ProductID, &row.ProductName, &row.SpecG, &row.Status, &row.CompletedAt,
			&row.AlreadyBilled, &row.BillingRunID,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) PreviewProcessingBilling(ctx context.Context, cmd salesapp.PreviewProcessingBillingCommand) (salesapp.ProcessingBillingPreview, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return salesapp.ProcessingBillingPreview{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	template, rules, err := r.loadProcessingBillingTemplateTx(ctx, tx, cmd.TemplateID, cmd.TemplateVersionID)
	if err != nil {
		return salesapp.ProcessingBillingPreview{}, err
	}
	preview, err := r.buildProcessingBillingPreviewTx(ctx, tx, cmd.CustomerID, cmd.WorkOrderIDs, template, rules, false)
	if err != nil {
		return salesapp.ProcessingBillingPreview{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return salesapp.ProcessingBillingPreview{}, err
	}
	return preview, nil
}

func (r Repository) ConfirmProcessingBilling(ctx context.Context, cmd salesapp.ConfirmProcessingBillingCommand) (salesapp.ProcessingBillingConfirmation, error) {
	workOrderIDs := append([]int64(nil), cmd.WorkOrderIDs...)
	sort.Slice(workOrderIDs, func(i, j int) bool { return workOrderIDs[i] < workOrderIDs[j] })
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return salesapp.ProcessingBillingConfirmation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	locked, err := tx.Query(ctx, fmt.Sprintf(`SELECT id FROM %s.work_orders WHERE id=ANY($1) ORDER BY id FOR UPDATE`, r.schema), workOrderIDs)
	if err != nil {
		return salesapp.ProcessingBillingConfirmation{}, err
	}
	lockedCount := 0
	for locked.Next() {
		lockedCount++
	}
	if err := locked.Err(); err != nil {
		locked.Close()
		return salesapp.ProcessingBillingConfirmation{}, err
	}
	locked.Close()
	if lockedCount != len(workOrderIDs) {
		return salesapp.ProcessingBillingConfirmation{}, fmt.Errorf("work order not found")
	}

	if existing, found, err := r.existingProcessingBillingConfirmationTx(ctx, tx, cmd.CustomerID, cmd.TemplateVersionID, workOrderIDs); err != nil {
		return salesapp.ProcessingBillingConfirmation{}, err
	} else if found {
		if err := tx.Commit(ctx); err != nil {
			return salesapp.ProcessingBillingConfirmation{}, err
		}
		return existing, nil
	}

	template, rules, err := r.loadProcessingBillingTemplateTx(ctx, tx, 0, cmd.TemplateVersionID)
	if err != nil {
		return salesapp.ProcessingBillingConfirmation{}, err
	}
	preview, err := r.buildProcessingBillingPreviewTx(ctx, tx, cmd.CustomerID, workOrderIDs, template, rules, true)
	if err != nil {
		return salesapp.ProcessingBillingConfirmation{}, err
	}

	var periodFrom, periodTo any
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT MIN(completed_at)::date,MAX(completed_at)::date FROM %s.work_orders WHERE id=ANY($1)`, r.schema), workOrderIDs).Scan(&periodFrom, &periodTo); err != nil {
		return salesapp.ProcessingBillingConfirmation{}, err
	}
	var settlementBatchID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_settlement_batches(
			customer_id,settlement_no,period_from,period_to,status,total_amount,confirmed_at,created_by,processing_billing_run_id
		) VALUES($1,'',$2,$3,'confirmed',$4,now(),$5,0)
		RETURNING id
	`, r.schema), cmd.CustomerID, periodFrom, periodTo, preview.TotalAmount, cmd.Actor).Scan(&settlementBatchID); err != nil {
		return salesapp.ProcessingBillingConfirmation{}, err
	}
	var billingRunID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.processing_billing_runs(
			customer_id,template_id,template_version_id,settlement_batch_id,run_kind,status,total_amount,currency,created_by,confirmed_at
		) VALUES($1,$2,$3,$4,'standard','confirmed',$5,'CNY',$6,now())
		RETURNING id
	`, r.schema), cmd.CustomerID, template.ID, template.CurrentVersionID, settlementBatchID, preview.TotalAmount, cmd.Actor).Scan(&billingRunID); err != nil {
		return salesapp.ProcessingBillingConfirmation{}, err
	}
	settlementNo := fmt.Sprintf("CPB-%d-%08d", cmd.CustomerID, billingRunID)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.customer_settlement_batches
		SET settlement_no=$2,processing_billing_run_id=$3,total_amount=$4,status='confirmed',confirmed_at=now()
		WHERE id=$1
	`, r.schema), settlementBatchID, settlementNo, billingRunID, preview.TotalAmount); err != nil {
		return salesapp.ProcessingBillingConfirmation{}, err
	}

	metricsByWorkOrder := make(map[int64]salesapp.ProcessingBillingMetrics, len(preview.WorkOrders))
	for _, metrics := range preview.WorkOrders {
		metricsByWorkOrder[metrics.WorkOrderID] = metrics
		var requestNo string
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COALESCE(MIN(request_no),'') FROM %s.customer_processing_production_demands WHERE linked_work_order_id=$1
		`, r.schema), metrics.WorkOrderID).Scan(&requestNo); err != nil {
			return salesapp.ProcessingBillingConfirmation{}, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.processing_billing_work_orders(
				billing_run_id,run_kind,work_order_id,work_order_no,request_no,product_name,completed_at,
				actual_input_kg,actual_output_kg,actual_minutes,actual_units,factory_material_actual_cost
			) VALUES($1,'standard',$2,$3,$4,$5,(SELECT completed_at FROM %s.work_orders WHERE id=$2),$6,$7,$8,$9,$10)
		`, r.schema, r.schema), billingRunID, metrics.WorkOrderID, metrics.WorkOrderNo, requestNo, metrics.ProductName,
			metrics.ActualInputKG, metrics.ActualOutputKG, metrics.ActualMinutes, metrics.ActualUnits, metrics.FactoryMaterialActualCost); err != nil {
			return salesapp.ProcessingBillingConfirmation{}, err
		}
	}

	for _, line := range preview.Lines {
		metrics := metricsByWorkOrder[line.WorkOrderID]
		var feeItemID int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.customer_fee_items(
				customer_id,source_type,source_id,fee_type,amount,currency,occurred_at,settlement_batch_id,status,note,processing_billing_run_id
			) VALUES($1,'production_work_order',$2,$3,$4,'CNY',
				(SELECT COALESCE(completed_at,now()) FROM %s.work_orders WHERE id=$2),$5,'settled',$6,$7)
			RETURNING id
		`, r.schema, r.schema), cmd.CustomerID, line.WorkOrderID, line.FeeType, line.Amount, settlementBatchID,
			strings.TrimSpace(line.FeeName+" / "+line.WorkOrderNo), billingRunID).Scan(&feeItemID); err != nil {
			return salesapp.ProcessingBillingConfirmation{}, err
		}
		calculationJSON, err := json.Marshal(map[string]any{
			"work_order_id": line.WorkOrderID,
			"work_order_no": line.WorkOrderNo,
			"basis":         line.Basis,
			"base_quantity": line.BaseQuantity,
			"unit_price":    line.UnitPrice,
			"amount":        line.Amount,
			"metrics":       metrics,
		})
		if err != nil {
			return salesapp.ProcessingBillingConfirmation{}, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.processing_billing_line_snapshots(
				billing_run_id,work_order_id,rule_id,line_kind,fee_item_id,fee_type,fee_name,basis,base_quantity,unit_price,amount,calculation_json
			) VALUES($1,$2,$3,'calculated',$4,$5,$6,$7,$8,$9,$10,$11::jsonb)
		`, r.schema), billingRunID, line.WorkOrderID, line.RuleID, feeItemID, line.FeeType, line.FeeName, line.Basis,
			line.BaseQuantity, line.UnitPrice, line.Amount, calculationJSON); err != nil {
			return salesapp.ProcessingBillingConfirmation{}, err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "customer_processing_bill", &billingRunID, "confirm", postgresinfra.StrPtr("status"), nil, postgresinfra.StrPtr("confirmed"), postgresinfra.AuditMeta{
		"customer_id": cmd.CustomerID, "template_id": template.ID, "template_version_id": template.CurrentVersionID,
		"settlement_batch_id": settlementBatchID, "settlement_no": settlementNo, "work_order_ids": workOrderIDs,
		"total_amount": preview.TotalAmount,
	}); err != nil {
		return salesapp.ProcessingBillingConfirmation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return salesapp.ProcessingBillingConfirmation{}, err
	}
	return salesapp.ProcessingBillingConfirmation{
		BillingRunID: billingRunID, SettlementBatchID: settlementBatchID, SettlementNo: settlementNo,
		TotalAmount: preview.TotalAmount, Currency: "CNY",
	}, nil
}

func (r Repository) loadProcessingBillingTemplateTx(ctx context.Context, tx pgx.Tx, templateID, versionID int64) (salesapp.OutsourceTemplate, []salesapp.OutsourceTemplateRule, error) {
	var row salesapp.OutsourceTemplate
	var err error
	if versionID > 0 {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT t.id,t.name,v.id,v.version_no,v.status,
			       COALESCE(v.roast_unit_price,0)::float8,COALESCE(v.bean_pack_unit_price,0)::float8,
			       COALESCE(v.drip_pack_unit_price,0)::float8,COALESCE(v.sc_unit_price,0)::float8
			FROM %s.outsource_template_versions v
			JOIN %s.outsource_templates t ON t.id=v.template_id
			WHERE v.id=$1 AND v.status='published' AND t.active=true
		`, r.schema, r.schema), versionID).Scan(
			&row.ID, &row.Name, &row.CurrentVersionID, &row.CurrentVersionNo, &row.CurrentVersionStatus,
			&row.RoastUnitPrice, &row.BeanPackUnitPrice, &row.DripPackUnitPrice, &row.SCUnitPrice,
		)
	} else {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT t.id,t.name,v.id,v.version_no,v.status,
			       COALESCE(v.roast_unit_price,0)::float8,COALESCE(v.bean_pack_unit_price,0)::float8,
			       COALESCE(v.drip_pack_unit_price,0)::float8,COALESCE(v.sc_unit_price,0)::float8
			FROM %s.outsource_templates t
			JOIN LATERAL (
				SELECT * FROM %s.outsource_template_versions
				WHERE template_id=t.id AND status='published'
				ORDER BY version_no DESC,id DESC LIMIT 1
			) v ON true
			WHERE t.id=$1 AND t.active=true
		`, r.schema, r.schema), templateID).Scan(
			&row.ID, &row.Name, &row.CurrentVersionID, &row.CurrentVersionNo, &row.CurrentVersionStatus,
			&row.RoastUnitPrice, &row.BeanPackUnitPrice, &row.DripPackUnitPrice, &row.SCUnitPrice,
		)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return salesapp.OutsourceTemplate{}, nil, fmt.Errorf("published outsource template not found")
	}
	if err != nil {
		return salesapp.OutsourceTemplate{}, nil, err
	}
	ruleRows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id,version_id,fee_type,name,basis,COALESCE(unit_price,0)::float8,sort_order
		FROM %s.outsource_template_rules WHERE version_id=$1 ORDER BY sort_order,id
	`, r.schema), row.CurrentVersionID)
	if err != nil {
		return salesapp.OutsourceTemplate{}, nil, err
	}
	defer ruleRows.Close()
	rules := make([]salesapp.OutsourceTemplateRule, 0)
	for ruleRows.Next() {
		var rule salesapp.OutsourceTemplateRule
		if err := ruleRows.Scan(&rule.ID, &rule.VersionID, &rule.FeeType, &rule.Name, &rule.Basis, &rule.UnitPrice, &rule.SortOrder); err != nil {
			return salesapp.OutsourceTemplate{}, nil, err
		}
		rules = append(rules, rule)
	}
	if err := ruleRows.Err(); err != nil {
		return salesapp.OutsourceTemplate{}, nil, err
	}
	if len(rules) == 0 {
		return salesapp.OutsourceTemplate{}, nil, fmt.Errorf("published outsource template has no billing rules")
	}
	row.Rules = rules
	return row, rules, nil
}

func (r Repository) buildProcessingBillingPreviewTx(ctx context.Context, tx pgx.Tx, customerID int64, workOrderIDs []int64, template salesapp.OutsourceTemplate, rules []salesapp.OutsourceTemplateRule, allowLocked bool) (salesapp.ProcessingBillingPreview, error) {
	var customerName string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(name,'') FROM %s.customers WHERE id=$1`, r.schema), customerID).Scan(&customerName); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return salesapp.ProcessingBillingPreview{}, fmt.Errorf("customer not found")
		}
		return salesapp.ProcessingBillingPreview{}, err
	}
	preview := salesapp.ProcessingBillingPreview{
		CustomerID: customerID, CustomerName: customerName,
		TemplateID: template.ID, TemplateName: template.Name,
		TemplateVersionID: template.CurrentVersionID, TemplateVersionNo: template.CurrentVersionNo,
		Currency: "CNY", WorkOrders: make([]salesapp.ProcessingBillingMetrics, 0, len(workOrderIDs)),
	}
	requiresFactoryActualCost := false
	for _, rule := range rules {
		if rule.Basis == salesapp.BillingBasisFactoryMaterialActualCost {
			requiresFactoryActualCost = true
			break
		}
	}
	for _, workOrderID := range workOrderIDs {
		if !allowLocked {
			var billed bool
			if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.processing_billing_work_orders WHERE work_order_id=$1 AND run_kind='standard')`, r.schema), workOrderID).Scan(&billed); err != nil {
				return salesapp.ProcessingBillingPreview{}, err
			}
			if billed {
				return salesapp.ProcessingBillingPreview{}, fmt.Errorf("work order already billed: %d", workOrderID)
			}
		}
		if requiresFactoryActualCost {
			if err := r.validateFactoryMaterialActualCostTx(ctx, tx, workOrderID); err != nil {
				return salesapp.ProcessingBillingPreview{}, err
			}
		}
		metrics, err := r.processingBillingMetricsTx(ctx, tx, customerID, workOrderID)
		if err != nil {
			return salesapp.ProcessingBillingPreview{}, err
		}
		lines, subtotal, err := salesapp.CalculateProcessingBillingLines(metrics, rules)
		if err != nil {
			return salesapp.ProcessingBillingPreview{}, err
		}
		preview.WorkOrders = append(preview.WorkOrders, metrics)
		preview.Lines = append(preview.Lines, lines...)
		preview.TotalAmount = roundProcessingBillingMoney(preview.TotalAmount + subtotal)
	}
	return preview, nil
}

func roundProcessingBillingMoney(value float64) float64 {
	return math.Round((value+1e-9)*100) / 100
}

func (r Repository) processingBillingMetricsTx(ctx context.Context, tx pgx.Tx, customerID, workOrderID int64) (salesapp.ProcessingBillingMetrics, error) {
	var linkedCustomerMin, linkedCustomerMax int64
	var status string
	var runningItemID, specG int64
	var metrics salesapp.ProcessingBillingMetrics
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT wo.id,COALESCE(wo.work_order_no,''),COALESCE(wo.product_name,''),COALESCE(wo.status,''),
		       COALESCE(to_char(wo.completed_at,'YYYY-MM-DD HH24:MI'),''),COALESCE(wo.running_item_id,0),COALESCE(wo.spec_g,0),
		       MIN(d.customer_id),MAX(d.customer_id)
		FROM %s.work_orders wo
		JOIN %s.customer_processing_production_demands d ON d.linked_work_order_id=wo.id
		WHERE wo.id=$1
		GROUP BY wo.id,wo.work_order_no,wo.product_name,wo.status,wo.completed_at,wo.running_item_id,wo.spec_g
	`, r.schema, r.schema), workOrderID).Scan(
		&metrics.WorkOrderID, &metrics.WorkOrderNo, &metrics.ProductName, &status, &metrics.CompletedAt,
		&runningItemID, &specG, &linkedCustomerMin, &linkedCustomerMax,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return salesapp.ProcessingBillingMetrics{}, fmt.Errorf("customer work order not found: %d", workOrderID)
	}
	if err != nil {
		return salesapp.ProcessingBillingMetrics{}, err
	}
	if linkedCustomerMin != customerID || linkedCustomerMax != customerID {
		return salesapp.ProcessingBillingMetrics{}, fmt.Errorf("work order does not belong exclusively to customer: %d", workOrderID)
	}
	if status != "completed" || strings.TrimSpace(metrics.CompletedAt) == "" {
		return salesapp.ProcessingBillingMetrics{}, fmt.Errorf("work order is not completed: %d", workOrderID)
	}
	var inputG, outputG int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(NULLIF((
		           SELECT SUM(completion_input_g)::bigint
		           FROM (
		               SELECT completion_no,MAX(input_g)::bigint AS completion_input_g
		               FROM %s.production_logs
		               WHERE running_item_id=wo.running_item_id
		               GROUP BY completion_no
		           ) actual_completions
		       ),0),NULLIF((
		           SELECT MAX(jc_input.actual_input_qty)::bigint
		           FROM %s.job_cards jc_input
		           WHERE jc_input.work_order_id=wo.id AND jc_input.actual_input_qty>0
		       ),0),NULLIF(ri.input_g,0),0),
		       COALESCE(NULLIF(pbc.finished_g,0),(
		           SELECT SUM(pl.finished_total_g)::bigint FROM %s.production_logs pl WHERE pl.running_item_id=wo.running_item_id
		       ),0),
		       COALESCE((SELECT SUM(jc.actual_minutes)::float8 FROM %s.job_cards jc WHERE jc.work_order_id=wo.id),0),
		       COALESCE(NULLIF((
		           SELECT SUM(pro.finished_units)::float8 FROM %s.produce_running_outputs pro
		           WHERE pro.running_item_id=wo.running_item_id AND pro.product_id=wo.product_id AND pro.spec_g=wo.spec_g
		       ),0),CASE WHEN COALESCE(wo.spec_g,0)>0 THEN COALESCE(NULLIF(pbc.finished_g,0),0)::float8/wo.spec_g ELSE 0 END,0)
		FROM %s.work_orders wo
		LEFT JOIN %s.produce_running_items ri ON ri.id=wo.running_item_id
		LEFT JOIN %s.production_batch_costs pbc ON pbc.running_item_id=wo.running_item_id
		WHERE wo.id=$1
	`, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema), workOrderID).Scan(
		&inputG, &outputG, &metrics.ActualMinutes, &metrics.ActualUnits,
	); err != nil {
		return salesapp.ProcessingBillingMetrics{}, err
	}
	metrics.ActualInputKG = float64(inputG) / 1000
	metrics.ActualOutputKG = float64(outputG) / 1000
	factoryCost, err := r.factoryMaterialActualCostTx(ctx, tx, workOrderID, runningItemID)
	if err != nil {
		return salesapp.ProcessingBillingMetrics{}, err
	}
	metrics.FactoryMaterialActualCost = factoryCost
	_ = specG
	return metrics, nil
}

// factoryMaterialActualCostTx is the ownership boundary for customer processing
// charges. Only reservations frozen as factory-owned are chargeable. Customer
// reservations are deliberately excluded even when the same material appears in
// the same work order.
func (r Repository) factoryMaterialActualCostTx(ctx context.Context, tx pgx.Tx, workOrderID, runningItemID int64) (float64, error) {
	var reservationTable *string
	if err := tx.QueryRow(ctx, `SELECT to_regclass($1)::text`, r.schema+".customer_processing_material_reservations").Scan(&reservationTable); err != nil {
		return 0, err
	}
	if reservationTable == nil {
		return 0, nil
	}
	var total float64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		WITH reservation_actual AS (
			SELECT COALESCE(NULLIF(component_type,''),'material') AS component_type,
			       CASE WHEN component_type='finished_product' THEN COALESCE(NULLIF(component_product_id,0),material_id) ELSE material_id END AS item_id,
			       material_batch_id,finished_stock_batch_id,source_owner_type,source_customer_id,
			       consumed_g::numeric AS actual_g,
			       consumed_units::numeric AS actual_units
			FROM %s.customer_processing_material_reservations
			WHERE work_order_id=$1 AND status IN ('reserved','consumed')
		), reservation_ownership AS (
			SELECT component_type,item_id,material_batch_id,finished_stock_batch_id,
			       SUM(CASE WHEN source_owner_type='factory' AND source_customer_id=0 THEN actual_g ELSE 0 END)::numeric AS factory_g,
			       SUM(actual_g)::numeric AS total_g,
			       SUM(CASE WHEN source_owner_type='factory' AND source_customer_id=0 THEN actual_units ELSE 0 END)::numeric AS factory_units,
			       SUM(actual_units)::numeric AS total_units
			FROM reservation_actual
			GROUP BY component_type,item_id,material_batch_id,finished_stock_batch_id
		), material_ownership AS (
			SELECT item_id,SUM(factory_g) AS factory_g,SUM(total_g) AS total_g,
			       SUM(factory_units) AS factory_units,SUM(total_units) AS total_units
			FROM reservation_ownership WHERE component_type<>'finished_product' GROUP BY item_id
		), finished_ownership AS (
			SELECT item_id,SUM(factory_g) AS factory_g,SUM(total_g) AS total_g,
			       SUM(factory_units) AS factory_units,SUM(total_units) AS total_units
			FROM reservation_ownership WHERE component_type='finished_product' GROUP BY item_id
		)
		SELECT COALESCE(SUM(CASE
			WHEN fsb.id IS NOT NULL AND l.deduct_g>0 THEN (l.deduct_g::numeric / 1000.0) * COALESCE(fsb.unit_cost,0) *
				COALESCE(
					CASE WHEN fbro.total_g>0 THEN fbro.factory_g/fbro.total_g END,
					CASE WHEN fmo.total_g>0 THEN fmo.factory_g/fmo.total_g END,
					0
				)
			WHEN fsb.id IS NOT NULL AND l.deduct_units>0 THEN l.deduct_units::numeric * COALESCE(fsb.unit_cost,0) *
				COALESCE(
					CASE WHEN fbro.total_units>0 THEN fbro.factory_units/fbro.total_units END,
					CASE WHEN fmo.total_units>0 THEN fmo.factory_units/fmo.total_units END,
					0
				)
			WHEN l.deduct_g > 0 THEN (l.deduct_g::numeric / 1000.0) * COALESCE(NULLIF(mb.unit_cost,0),NULLIF(m.purchase_price,0),0) *
				COALESCE(
					CASE WHEN bro.total_g>0 THEN bro.factory_g/bro.total_g END,
					CASE WHEN mo.total_g>0 THEN mo.factory_g/mo.total_g END,
					0
				)
			WHEN l.deduct_units > 0 THEN l.deduct_units::numeric * COALESCE(NULLIF(m.purchase_price,0),0) *
				COALESCE(
					CASE WHEN bro.total_units>0 THEN bro.factory_units/bro.total_units END,
					CASE WHEN mo.total_units>0 THEN mo.factory_units/mo.total_units END,
					0
				)
			ELSE 0 END),0)::float8
		FROM %s.material_consumption_logs l
		LEFT JOIN %s.material_batches mb ON mb.id=l.material_batch_id
		LEFT JOIN %s.materials m ON m.id=l.material_id
		LEFT JOIN %s.stock_batches fsb
		  ON l.material_batch_id=0 AND fsb.item_type='finished_product' AND fsb.batch_code=l.material_batch_code
		LEFT JOIN reservation_ownership bro
		  ON bro.component_type<>'finished_product' AND bro.item_id=l.material_id AND bro.material_batch_id>0 AND bro.material_batch_id=l.material_batch_id
		LEFT JOIN material_ownership mo ON mo.item_id=l.material_id
		LEFT JOIN reservation_ownership fbro
		  ON fbro.component_type='finished_product' AND fbro.item_id=fsb.item_id AND fbro.finished_stock_batch_id=fsb.id
		LEFT JOIN finished_ownership fmo ON fmo.item_id=fsb.item_id
		WHERE l.running_item_id=$2
	`, r.schema, r.schema, r.schema, r.schema, r.schema), workOrderID, runningItemID).Scan(&total)
	return total, err
}

func (r Repository) validateFactoryMaterialActualCostTx(ctx context.Context, tx pgx.Tx, workOrderID int64) error {
	var reservationTable *string
	if err := tx.QueryRow(ctx, `SELECT to_regclass($1)::text`, r.schema+".customer_processing_material_reservations").Scan(&reservationTable); err != nil {
		return err
	}
	if reservationTable == nil {
		return nil
	}
	var missing bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT EXISTS(
			SELECT 1
			FROM %s.customer_processing_material_reservations
			WHERE work_order_id=$1
			  AND status='consumed'
			  AND source_owner_type='factory' AND source_customer_id=0
			  AND consumed_g=0 AND consumed_units=0
			  AND (
				(GREATEST(reserved_g,required_g)>0 AND returned_g<GREATEST(reserved_g,required_g))
				OR (GREATEST(reserved_units,required_units)>0 AND returned_units<GREATEST(reserved_units,required_units))
			  )
		)
	`, r.schema), workOrderID).Scan(&missing); err != nil {
		return err
	}
	if missing {
		return fmt.Errorf("工单 %d 缺少实际工厂物料耗用数据，无法按实际成本计费", workOrderID)
	}
	return nil
}

func (r Repository) existingProcessingBillingConfirmationTx(ctx context.Context, tx pgx.Tx, customerID, templateVersionID int64, workOrderIDs []int64) (salesapp.ProcessingBillingConfirmation, bool, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT DISTINCT billing_run_id FROM %s.processing_billing_work_orders WHERE run_kind='standard' AND work_order_id=ANY($1)
	`, r.schema), workOrderIDs)
	if err != nil {
		return salesapp.ProcessingBillingConfirmation{}, false, err
	}
	runIDs := make([]int64, 0, 2)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return salesapp.ProcessingBillingConfirmation{}, false, err
		}
		runIDs = append(runIDs, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return salesapp.ProcessingBillingConfirmation{}, false, err
	}
	rows.Close()
	if len(runIDs) == 0 {
		return salesapp.ProcessingBillingConfirmation{}, false, nil
	}
	if len(runIDs) != 1 {
		return salesapp.ProcessingBillingConfirmation{}, false, fmt.Errorf("selected work orders are already billed by different bills")
	}
	var matched, billedWorkOrderCount int
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT (COUNT(*) FILTER (WHERE work_order_id=ANY($2)))::int,COUNT(*)::int
		FROM %s.processing_billing_work_orders
		WHERE billing_run_id=$1 AND run_kind='standard'
	`, r.schema), runIDs[0], workOrderIDs).Scan(&matched, &billedWorkOrderCount); err != nil {
		return salesapp.ProcessingBillingConfirmation{}, false, err
	}
	if matched != len(workOrderIDs) || billedWorkOrderCount != len(workOrderIDs) {
		return salesapp.ProcessingBillingConfirmation{}, false, fmt.Errorf("some selected work orders are already billed")
	}
	var result salesapp.ProcessingBillingConfirmation
	var storedCustomerID, storedVersionID int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT r.id,r.customer_id,r.template_version_id,r.settlement_batch_id,COALESCE(s.settlement_no,''),
		       COALESCE(r.total_amount,0)::float8,COALESCE(r.currency,'CNY')
		FROM %s.processing_billing_runs r
		JOIN %s.customer_settlement_batches s ON s.id=r.settlement_batch_id
		WHERE r.id=$1 AND r.run_kind='standard' AND r.status IN ('confirmed','paid')
	`, r.schema, r.schema), runIDs[0]).Scan(
		&result.BillingRunID, &storedCustomerID, &storedVersionID, &result.SettlementBatchID,
		&result.SettlementNo, &result.TotalAmount, &result.Currency,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return salesapp.ProcessingBillingConfirmation{}, false, fmt.Errorf("work order bill is not reusable")
	}
	if err != nil {
		return salesapp.ProcessingBillingConfirmation{}, false, err
	}
	if storedCustomerID != customerID || storedVersionID != templateVersionID {
		return salesapp.ProcessingBillingConfirmation{}, false, fmt.Errorf("work order already billed with another customer or template version")
	}
	result.Reused = true
	return result, true, nil
}
