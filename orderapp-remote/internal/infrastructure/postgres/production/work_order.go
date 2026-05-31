package production

import (
	"context"
	"encoding/json"
	"fmt"
	productiondomain "orderapp/internal/domain/production"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	"strings"

	productionapp "orderapp/internal/application/production"

	"github.com/jackc/pgx/v5"
)

func workOrderNo(runningItemID int64) string {
	return fmt.Sprintf("WO-%010d", runningItemID)
}

type processSnapshotOperation struct {
	Seq                  int    `json:"seq"`
	Operation            string `json:"operation"`
	Workstation          string `json:"workstation"`
	DefaultEquipment     string `json:"default_equipment"`
	DefaultMinutes       int    `json:"default_minutes"`
	RecordsLoss          bool   `json:"records_loss"`
	ParameterSchemaJSON  string `json:"parameter_schema_json"`
	QualityChecklistJSON string `json:"quality_checklist_json"`
}

type processTemplateSnapshot struct {
	ID                   int64                      `json:"id"`
	Name                 string                     `json:"name"`
	ProductID            int64                      `json:"product_id"`
	ProductName          string                     `json:"product_name"`
	BomVersionID         int64                      `json:"bom_version_id"`
	BomVersionNo         string                     `json:"bom_version_no"`
	IndustryTemplateID   int64                      `json:"industry_template_id"`
	IndustryTemplateName string                     `json:"industry_template_name"`
	DefaultEquipment     string                     `json:"default_equipment"`
	DefaultMinutes       int                        `json:"default_minutes"`
	KeyParamsJSON        string                     `json:"key_params_json"`
	Operations           []processSnapshotOperation `json:"operations"`
}

type operationTemplateStepRow struct {
	ID          int64
	Position    int
	Operation   string
	Workstation string
	CostType    string
	CostRate    float64
}

func loadOperationTemplateStepsTx(ctx context.Context, tx pgx.Tx, schema string, operationTemplateID int64) ([]operationTemplateStepRow, error) {
	if operationTemplateID <= 0 {
		return nil, nil
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(position,1), COALESCE(NULLIF(operation,''),'production'), COALESCE(workstation,''), COALESCE(cost_type,''), COALESCE(cost_rate,0)::float8
		FROM %s.operation_template_steps
		WHERE template_id=$1 AND active=true
		ORDER BY position,id
	`, schema), operationTemplateID)
	if err != nil {
		if strings.Contains(err.Error(), "operation_template_steps") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	out := make([]operationTemplateStepRow, 0)
	for rows.Next() {
		var row operationTemplateStepRow
		if err := rows.Scan(&row.ID, &row.Position, &row.Operation, &row.Workstation, &row.CostType, &row.CostRate); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func defaultOperationTemplateSteps() []operationTemplateStepRow {
	return []operationTemplateStepRow{{Position: 1, Operation: "production", Workstation: "workstation"}}
}

func loadBoundBomVersionIDForProductTx(ctx context.Context, tx pgx.Tx, schema string, productID int64) (int64, error) {
	var versionID int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(bom_version_id,0)
		FROM %s.product_production_bom_bindings
		WHERE product_id=$1
	`, schema), productID).Scan(&versionID)
	if err == pgx.ErrNoRows {
		return 0, nil
	}
	return versionID, err
}

func createWorkOrderForRunningItemTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID int64, batchID string, productID int64, productName string, specG int64, plannedG int64, materialSnapshot []byte, operationTemplateID int64, operator string) (int64, error) {
	processSnapshot, processSnapshotJSON, err := loadActiveProcessTemplateSnapshotTx(ctx, tx, schema, productID)
	if err != nil {
		return 0, err
	}
	processTemplateID := int64(0)
	processTemplateName := ""
	if processSnapshot != nil {
		processTemplateID = processSnapshot.ID
		processTemplateName = processSnapshot.Name
	}
	if len(processSnapshotJSON) == 0 {
		processSnapshotJSON = []byte("{}")
	}
	customerProductSnapshot, err := loadCustomerProductSnapshotForWorkOrderTx(ctx, tx, schema, runningItemID, productID, specG)
	if err != nil {
		return 0, err
	}
	if len(customerProductSnapshot) == 0 {
		customerProductSnapshot = []byte("[]")
	}
	bomVersionID, err := loadBoundBomVersionIDForProductTx(ctx, tx, schema, productID)
	if err != nil {
		return 0, err
	}

	var workOrderID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.work_orders(
			work_order_no,running_item_id,batch_id,product_id,product_name,spec_g,planned_g,status,
			material_snapshot,bom_version_id,operation_template_id,process_template_id,process_template_name,process_snapshot_json,customer_product_snapshot_json,created_at
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,'running',$8,$9,$10,$11,$12,$13,$14,now())
		ON CONFLICT (running_item_id) DO UPDATE SET
			status='running',
			material_snapshot=excluded.material_snapshot,
			bom_version_id=excluded.bom_version_id,
			operation_template_id=excluded.operation_template_id,
			process_template_id=excluded.process_template_id,
			process_template_name=excluded.process_template_name,
			process_snapshot_json=excluded.process_snapshot_json,
			customer_product_snapshot_json=excluded.customer_product_snapshot_json
		RETURNING id
	`, schema), workOrderNo(runningItemID), runningItemID, batchID, productID, productName, specG, plannedG, materialSnapshot, bomVersionID, operationTemplateID, processTemplateID, processTemplateName, processSnapshotJSON, customerProductSnapshot).Scan(&workOrderID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.job_cards WHERE work_order_id=$1`, schema), workOrderID); err != nil {
		return 0, err
	}

	if processSnapshot != nil && len(processSnapshot.Operations) > 0 {
		for _, op := range processSnapshot.Operations {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`
				INSERT INTO %s.job_cards(
					work_order_id,sequence_no,operation,workstation,status,started_at,operator,
					planned_input_qty,records_loss,parameter_schema_json
				)
				VALUES($1,$2,$3,$4,'running',now(),$5,$6,$7,$8::jsonb)
			`, schema), workOrderID, op.Seq, op.Operation, op.Workstation, operator, plannedG, op.RecordsLoss, defaultJSONObject(op.ParameterSchemaJSON)); err != nil {
				return 0, err
			}
		}
	} else {
		steps, err := loadOperationTemplateStepsTx(ctx, tx, schema, operationTemplateID)
		if err != nil {
			return 0, err
		}
		if len(steps) == 0 {
			steps = defaultOperationTemplateSteps()
		}
		for _, step := range steps {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`
				INSERT INTO %s.job_cards(
					work_order_id,sequence_no,operation,workstation,status,started_at,operator,
					planned_input_qty,records_loss,operation_template_step_id,cost_type,cost_rate
				)
				VALUES($1,$2,$3,$4,'running',now(),$5,$6,$7,$8,$9,$10)
			`, schema), workOrderID, step.Position, step.Operation, step.Workstation, operator, plannedG, true, step.ID, step.CostType, step.CostRate); err != nil {
				return 0, err
			}
		}
	}
	return workOrderID, nil
}

func loadCustomerProductSnapshotForWorkOrderTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID int64, productID int64, specG int64) ([]byte, error) {
	var raw string
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		WITH running AS (
			SELECT string_to_array(replace(COALESCE(order_nos,''),' ',''), ',') AS order_nos
			FROM %[1]s.produce_running_items
			WHERE id=$1
		),
		lines AS (
			SELECT DISTINCT
			       o.order_no,
			       COALESCE(o.customer_id,0) AS customer_id,
			       COALESCE(oi.customer_product_alias_id,0) AS customer_product_alias_id,
			       COALESCE(oi.customer_product_display_name_snapshot,'') AS customer_product_display_name_snapshot,
			       COALESCE(oi.customer_item_code_snapshot,'') AS customer_item_code_snapshot,
			       COALESCE(oi.product_code_snapshot,'') AS product_code_snapshot,
			       COALESCE(oi.product_name_snapshot,'') AS product_name_snapshot
			FROM %[1]s.order_items oi
			JOIN %[1]s.orders o ON o.id=oi.order_id
			JOIN running r ON o.order_no = ANY(r.order_nos)
			CROSS JOIN LATERAL (
				SELECT COALESCE(NULLIF(regexp_replace(COALESCE(oi.spec,''), '[^0-9]', '', 'g'), ''), '0')::bigint AS spec_g
			) spec
			WHERE COALESCE(oi.product_id,0)=$2
			  AND spec.spec_g=$3
			  AND (
			    COALESCE(oi.customer_product_alias_id,0)>0
			    OR COALESCE(oi.customer_product_display_name_snapshot,'')<>''
			    OR COALESCE(oi.product_name_snapshot,'')<>''
			  )
		)
		SELECT COALESCE(jsonb_agg(jsonb_build_object(
			'order_no', order_no,
			'customer_id', customer_id,
			'customer_product_alias_id', customer_product_alias_id,
			'customer_product_display_name_snapshot', customer_product_display_name_snapshot,
			'customer_item_code_snapshot', customer_item_code_snapshot,
			'product_code_snapshot', product_code_snapshot,
			'product_name_snapshot', product_name_snapshot
		)), '[]'::jsonb)::text
		FROM lines
	`, schema), runningItemID, productID, specG).Scan(&raw)
	if err != nil {
		if strings.Contains(err.Error(), "customer_product_alias_id") || strings.Contains(err.Error(), "customer_product_display_name_snapshot") {
			return []byte("[]"), nil
		}
		return nil, err
	}
	return []byte(raw), nil
}

func completeWorkOrderForRunningItemTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID int64, actualCost float64, actualInputQty int64, actualOutputQty int64, operator string) error {
	var workOrderID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s.work_orders
		SET status='completed', actual_cost=$2, completed_at=now()
		WHERE running_item_id=$1
		RETURNING id
	`, schema), runningItemID, actualCost).Scan(&workOrderID); err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		return err
	}
	actualLossQty := 0.0
	actualLossRate := 0.0
	if actualInputQty > 0 {
		lossQty, lossRate, err := productiondomain.ActualLossMetrics(float64(actualInputQty), float64(actualOutputQty))
		if err != nil {
			return err
		}
		actualLossQty = lossQty
		actualLossRate = lossRate
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.job_cards
		SET actual_input_qty=$2,
		    actual_output_qty=$3,
		    actual_loss_qty=$4,
		    actual_loss_rate=$5,
		    operator=COALESCE(NULLIF(operator,''),$6)
		WHERE id = COALESCE(
			(SELECT id FROM %s.job_cards WHERE work_order_id=$1 AND records_loss=true ORDER BY sequence_no DESC, id DESC LIMIT 1),
			(SELECT id FROM %s.job_cards WHERE work_order_id=$1 ORDER BY sequence_no DESC, id DESC LIMIT 1)
		)
	`, schema, schema, schema), workOrderID, actualInputQty, actualOutputQty, actualLossQty, actualLossRate, operator); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.job_cards
		SET status='completed',
		    completed_at=now(),
		    operator=COALESCE(NULLIF(operator,''),$2)
		WHERE work_order_id=$1 AND status <> 'completed'
	`, schema), workOrderID, operator); err != nil {
		return err
	}
	summary, err := operationSummaryJSONForWorkOrderTx(ctx, tx, schema, workOrderID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET operation_summary_json=$2::jsonb WHERE id=$1`, schema), workOrderID, summary)
	return err
}

func cancelWorkOrderForRunningItemTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID int64, operator string) error {
	var workOrderID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s.work_orders
		SET status='cancelled', completed_at=now()
		WHERE running_item_id=$1
		RETURNING id
	`, schema), runningItemID).Scan(&workOrderID); err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		return err
	}
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.job_cards
		SET status='cancelled', completed_at=now(), operator=COALESCE(NULLIF(operator,''),$2)
		WHERE work_order_id=$1 AND status <> 'completed'
	`, schema), workOrderID, operator)
	if err != nil {
		return err
	}
	return releaseMaterialReservationsForRunningItemTx(ctx, tx, schema, runningItemID)
}

func loadActiveProcessTemplateSnapshotTx(ctx context.Context, tx pgx.Tx, schema string, productID int64) (*processTemplateSnapshot, []byte, error) {
	var snapshot processTemplateSnapshot
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT pt.id,pt.name,pt.product_id,COALESCE(p.name,''),
		       pt.bom_version_id,COALESCE(bv.version_no,''),
		       pt.industry_template_id,COALESCE(ift.name,''),
		       pt.default_equipment,pt.default_minutes,
		       COALESCE(pt.key_params_json,'{}'::jsonb)::text
		FROM %[1]s.process_templates pt
		LEFT JOIN %[1]s.products p ON p.id=pt.product_id
		LEFT JOIN %[1]s.bom_versions bv ON bv.id=pt.bom_version_id
		LEFT JOIN %[1]s.industry_field_templates ift ON ift.id=pt.industry_template_id
		WHERE pt.product_id=$1 AND pt.status='active'
		ORDER BY pt.updated_at DESC, pt.id DESC
		LIMIT 1
	`, schema), productID).Scan(
		&snapshot.ID,
		&snapshot.Name,
		&snapshot.ProductID,
		&snapshot.ProductName,
		&snapshot.BomVersionID,
		&snapshot.BomVersionNo,
		&snapshot.IndustryTemplateID,
		&snapshot.IndustryTemplateName,
		&snapshot.DefaultEquipment,
		&snapshot.DefaultMinutes,
		&snapshot.KeyParamsJSON,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil, nil
		}
		if strings.Contains(err.Error(), "process_templates") {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT seq,operation,workstation,default_equipment,default_minutes,records_loss,
		       COALESCE(parameter_schema_json,'{}'::jsonb)::text,
		       COALESCE(quality_checklist_json,'[]'::jsonb)::text
		FROM %s.process_template_operations
		WHERE template_id=$1
		ORDER BY seq, id
	`, schema), snapshot.ID)
	if err != nil {
		if strings.Contains(err.Error(), "process_template_operations") {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var op processSnapshotOperation
		if err := rows.Scan(&op.Seq, &op.Operation, &op.Workstation, &op.DefaultEquipment, &op.DefaultMinutes, &op.RecordsLoss, &op.ParameterSchemaJSON, &op.QualityChecklistJSON); err != nil {
			return nil, nil, err
		}
		snapshot.Operations = append(snapshot.Operations, op)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	b, err := json.Marshal(snapshot)
	if err != nil {
		return nil, nil, err
	}
	return &snapshot, b, nil
}

func defaultJSONObject(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "{}"
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil || obj == nil {
		return "{}"
	}
	return raw
}

func operationSummaryJSONForWorkOrderTx(ctx context.Context, tx pgx.Tx, schema string, workOrderID int64) (string, error) {
	var raw string
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(jsonb_agg(jsonb_build_object(
			'id', id,
			'sequence_no', sequence_no,
			'operation', operation,
			'workstation', workstation,
			'status', status,
			'records_loss', records_loss,
			'planned_input_qty', planned_input_qty,
			'actual_input_qty', actual_input_qty,
			'actual_output_qty', actual_output_qty,
			'actual_loss_qty', actual_loss_qty,
			'actual_loss_rate', actual_loss_rate,
			'exception_reason', exception_reason
		) ORDER BY sequence_no, id), '[]'::jsonb)::text
		FROM %s.job_cards
		WHERE work_order_id=$1
	`, schema), workOrderID).Scan(&raw)
	if err != nil {
		return "", err
	}
	return raw, nil
}

func recordBatchCostForRunningItemTx(ctx context.Context, tx pgx.Tx, schema string, r ProduceRunRow, finishedTotalG int64) (float64, error) {
	var materialCost float64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(
			CASE
				WHEN l.deduct_g > 0 THEN (l.deduct_g::numeric / 1000.0) * COALESCE(NULLIF(mb.unit_cost,0), NULLIF(m.purchase_price,0), 0)
				WHEN l.deduct_units > 0 THEN l.deduct_units::numeric * COALESCE(NULLIF(m.purchase_price,0), 0)
				ELSE 0
			END
		),0)
		FROM %s.material_consumption_logs l
		LEFT JOIN %s.material_batches mb ON mb.id=l.material_batch_id
		LEFT JOIN %s.materials m ON m.id=l.material_id
		WHERE l.running_item_id=$1
	`, schema, schema, schema), r.ID).Scan(&materialCost)
	if err != nil && strings.Contains(err.Error(), "material_batches") {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COALESCE(SUM(
				CASE
					WHEN l.deduct_g > 0 THEN (l.deduct_g::numeric / 1000.0) * COALESCE(NULLIF(m.purchase_price,0), 0)
					WHEN l.deduct_units > 0 THEN l.deduct_units::numeric * COALESCE(NULLIF(m.purchase_price,0), 0)
					ELSE 0
				END
			),0)
			FROM %s.material_consumption_logs l
			LEFT JOIN %s.materials m ON m.id=l.material_id
			WHERE l.running_item_id=$1
		`, schema, schema), r.ID).Scan(&materialCost)
	}
	if err != nil {
		return 0, err
	}
	var operationCost float64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(CASE
			WHEN COALESCE(NULLIF(jc.cost_type,''),'fixed') IN ('per_kg_input','per_input_kg')
				THEN COALESCE(jc.cost_rate,0) * COALESCE(NULLIF(ri.input_g,0), $2)::numeric / 1000.0
			WHEN COALESCE(NULLIF(jc.cost_type,''),'fixed') IN ('per_kg_output','per_finished_kg','per_kg')
				THEN COALESCE(jc.cost_rate,0) * $2::numeric / 1000.0
			WHEN COALESCE(NULLIF(jc.cost_type,''),'fixed') IN ('fixed','per_unit','per_quote_unit')
				THEN COALESCE(jc.cost_rate,0)
			ELSE 0
		END),0)::float8
		FROM %s.work_orders wo
		JOIN %s.job_cards jc ON jc.work_order_id=wo.id
		LEFT JOIN %s.produce_running_items ri ON ri.id=wo.running_item_id
		WHERE wo.running_item_id=$1
	`, schema, schema, schema), r.ID, finishedTotalG).Scan(&operationCost); err != nil {
		return 0, err
	}
	totalCost := materialCost + operationCost
	unitCost := 0.0
	if finishedTotalG > 0 {
		unitCost = totalCost / (float64(finishedTotalG) / 1000.0)
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_batch_costs(running_item_id,batch_id,product_name,material_cost,operation_cost,total_cost,finished_g,unit_cost_per_kg,created_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,now())
		ON CONFLICT (running_item_id) DO UPDATE SET
			material_cost=excluded.material_cost,
			operation_cost=excluded.operation_cost,
			total_cost=excluded.total_cost,
			finished_g=excluded.finished_g,
			unit_cost_per_kg=excluded.unit_cost_per_kg,
			created_at=now()
	`, schema), r.ID, r.BatchID, r.Product, materialCost, operationCost, totalCost, finishedTotalG, unitCost)
	return totalCost, err
}

func (r Repository) ListWorkOrders(ctx context.Context, query productionapp.WorkOrderQuery) ([]productionapp.WorkOrderRow, error) {
	args := []any{}
	where := "1=1"
	if query.Status != "" {
		args = append(args, query.Status)
		where += fmt.Sprintf(" AND wo.status=$%d", len(args))
	}
	args = append(args, query.Limit)
	limitArg := len(args)
	machines, err := r.ListMachines(ctx, true)
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT wo.id,wo.work_order_no,wo.running_item_id,wo.batch_id,wo.product_id,wo.product_name,wo.spec_g,wo.planned_g,wo.status,
		       COALESCE(wo.actual_cost,0),to_char(wo.created_at,'YYYY-MM-DD HH24:MI'),COALESCE(to_char(wo.completed_at,'YYYY-MM-DD HH24:MI'),''),
		       COALESCE(p.roast_level,''),
		       COALESCE(NULLIF(ri.bom_yield_rate,0), pbv.yield_rate, pb.yield_rate, 0),
		       COALESCE(NULLIF(ri.input_g,0), wo.planned_g, 0),
		       COALESCE(ri.planned_units,0),
		       COALESCE(ri.planned_loose_g,0),
		       COALESCE(ri.order_nos,''),
		       COALESCE(wo.material_snapshot,'[]'::jsonb)::text,
		       COALESCE((
		           SELECT SUM(r.reserved_g)::bigint
		           FROM %s.work_order_material_reservations r
		           WHERE r.running_item_id=wo.running_item_id
		       ),0),
		       COALESCE((
		           SELECT SUM(r.consumed_g)::bigint
		           FROM %s.work_order_material_reservations r
		           WHERE r.running_item_id=wo.running_item_id
		       ),0),
		       COALESCE((
		           SELECT SUM(GREATEST(0,r.reserved_g-r.consumed_g-r.returned_g))::bigint
		           FROM %s.work_order_material_reservations r
		           WHERE r.running_item_id=wo.running_item_id AND r.status='reserved'
		       ),0),
		       COALESCE(NULLIF(wo.bom_version_id,0), pbb.bom_version_id,0),
		       COALESCE(wo.process_template_id,0),
		       COALESCE(wo.process_template_name,''),
		       COALESCE(wo.process_snapshot_json,'{}'::jsonb)::text,
		       COALESCE(NULLIF(wo.operation_summary_json,'[]'::jsonb), (
		           SELECT jsonb_agg(jsonb_build_object(
		               'id', jc.id,
		               'sequence_no', jc.sequence_no,
		               'operation', jc.operation,
		               'workstation', jc.workstation,
		               'status', jc.status,
		               'records_loss', jc.records_loss,
		               'planned_input_qty', jc.planned_input_qty,
		               'actual_input_qty', jc.actual_input_qty,
		               'actual_output_qty', jc.actual_output_qty,
		               'actual_loss_qty', jc.actual_loss_qty,
		               'actual_loss_rate', jc.actual_loss_rate,
		               'exception_reason', jc.exception_reason
		           ) ORDER BY jc.sequence_no, jc.id)
		           FROM %s.job_cards jc
		           WHERE jc.work_order_id=wo.id
		       ), '[]'::jsonb)::text,
		       COALESCE((
		           SELECT string_agg(COALESCE(m.name,'') || ' ' || COALESCE(NULLIF(trim(trailing '.' from trim(trailing '0' from COALESCE(bi.ratio_pct,0)::text)), ''), '0') || '%%', '、' ORDER BY bi.id)
		           FROM (
		               SELECT pbi.id,pbi.material_id,pbi.ratio_pct
		               FROM %s.production_bom_version_items pbi
		               WHERE pbb.product_id IS NOT NULL AND pbi.version_id=pbb.bom_version_id
		               UNION ALL
		               SELECT lbi.id,lbi.material_id,lbi.ratio_pct
		               FROM %s.product_bom_items lbi
		               WHERE pbb.product_id IS NULL AND lbi.product_id=wo.product_id
		           ) bi
		           LEFT JOIN %s.materials m ON m.id=bi.material_id
		       ), '')
		FROM %s.work_orders wo
		LEFT JOIN %s.produce_running_items ri ON ri.id=wo.running_item_id
		LEFT JOIN %s.products p ON p.id=wo.product_id
		LEFT JOIN %s.product_production_bom_bindings pbb ON pbb.product_id=wo.product_id
		LEFT JOIN %s.production_bom_versions pbv ON pbv.id=pbb.bom_version_id
		LEFT JOIN %s.product_bom pb ON pb.product_id=wo.product_id
		WHERE %s
		ORDER BY wo.created_at DESC, wo.id DESC
		LIMIT $%d
	`, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, where, limitArg), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]productionapp.WorkOrderRow, 0)
	for rows.Next() {
		var row productionapp.WorkOrderRow
		var snapshotText, fallbackMaterialSummary string
		if err := rows.Scan(
			&row.ID, &row.WorkOrderNo, &row.RunningItemID, &row.BatchID, &row.ProductID, &row.ProductName, &row.SpecG, &row.PlannedG, &row.Status, &row.ActualCost, &row.CreatedAt, &row.CompletedAt,
			&row.RoastLevel, &row.YieldRate, &row.SuggestedInputG, &row.PlannedUnits, &row.PlannedLooseG, &row.OrderNos, &snapshotText, &row.WIPReservedG, &row.WIPConsumedG, &row.WIPRemainingReservedG, &row.BomVersionID, &row.ProcessTemplateID, &row.ProcessTemplateName, &row.ProcessSnapshotJSON, &row.OperationSummaryJSON, &fallbackMaterialSummary,
		); err != nil {
			return nil, err
		}
		row.MaterialSummary = formatMaterialSnapshotSummary(snapshotText)
		if row.MaterialSummary == "" {
			row.MaterialSummary = fallbackMaterialSummary
		}
		row.ExpectedYieldRate = productiondomain.NormalizeYieldRate(row.YieldRate)
		row.ExpectedLossRate = productiondomain.ExpectedLossRate(row.ExpectedYieldRate)
		row.YieldRate = row.ExpectedYieldRate
		if row.SuggestedInputG <= 0 {
			row.SuggestedInputG = row.PlannedG
		}
		machine, batches := pickMachineAndBatches(row.SuggestedInputG, machines)
		row.SuggestedMachine = machine.Name
		row.SuggestedBatchCount = int64(len(batches))
		if row.SuggestedBatchCount == 0 && row.SuggestedInputG > 0 {
			row.SuggestedBatchCount = 1
		}
		if len(batches) > 0 {
			row.SuggestedBatchG = batches[0]
		} else {
			row.SuggestedBatchG = row.SuggestedInputG
		}
		row.SuggestedBatchPlan = formatWorkOrderBatchPlan(batches, row.SuggestedInputG)
		out = append(out, row)
	}
	return out, rows.Err()
}

func formatMaterialSnapshotSummary(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" || raw == "null" {
		return ""
	}
	var rows []materialSnapshotRow
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return ""
	}
	parts := make([]string, 0, len(rows))
	for _, row := range rows {
		name := strings.TrimSpace(row.MaterialName)
		if name == "" {
			continue
		}
		if row.Source == "packaging" {
			parts = append(parts, name+" 包材")
			continue
		}
		ratio := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.4f", row.RatioPct), "0"), ".")
		if ratio == "" || ratio == "0" {
			parts = append(parts, name)
		} else {
			parts = append(parts, name+" "+ratio+"%")
		}
	}
	return strings.Join(parts, "、")
}

func formatWorkOrderBatchPlan(batches []int64, fallbackG int64) string {
	if len(batches) == 0 {
		if fallbackG <= 0 {
			return "0"
		}
		return formatKg(fallbackG) + "kg"
	}
	parts := make([]string, 0, len(batches))
	used := make([]bool, len(batches))
	for i, batch := range batches {
		if used[i] {
			continue
		}
		count := 1
		for j := i + 1; j < len(batches); j++ {
			if batches[j] == batch {
				used[j] = true
				count++
			}
		}
		part := formatKg(batch) + "kg"
		if count > 1 {
			part += fmt.Sprintf(" x %d", count)
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, " + ")
}

func (r Repository) ListJobCards(ctx context.Context, query productionapp.JobCardQuery) ([]productionapp.JobCardRow, error) {
	args := []any{}
	where := "1=1"
	if query.Status != "" {
		args = append(args, query.Status)
		where += fmt.Sprintf(" AND status=$%d", len(args))
	}
	args = append(args, query.Limit)
	limitArg := len(args)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,work_order_id,sequence_no,operation,workstation,status,
		       to_char(started_at,'YYYY-MM-DD HH24:MI'),COALESCE(to_char(completed_at,'YYYY-MM-DD HH24:MI'),''),operator,
		       COALESCE(planned_input_qty,0)::float8,
		       COALESCE(actual_input_qty,0)::float8,
		       COALESCE(actual_output_qty,0)::float8,
		       COALESCE(actual_loss_qty,0)::float8,
		       COALESCE(actual_loss_rate,0)::float8,
		       COALESCE(records_loss,false),
		       COALESCE(exception_reason,''),
		       COALESCE(metrics_json,'{}'::jsonb)::text,
		       COALESCE(parameter_schema_json,'{}'::jsonb)::text
		FROM %s.job_cards
		WHERE %s
		ORDER BY started_at DESC, work_order_id DESC, sequence_no, id
		LIMIT $%d
	`, r.schema, where, limitArg), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]productionapp.JobCardRow, 0)
	for rows.Next() {
		var row productionapp.JobCardRow
		if err := rows.Scan(&row.ID, &row.WorkOrderID, &row.SequenceNo, &row.Operation, &row.Workstation, &row.Status, &row.StartedAt, &row.CompletedAt, &row.Operator, &row.PlannedInputQty, &row.ActualInputQty, &row.ActualOutputQty, &row.ActualLossQty, &row.ActualLossRate, &row.RecordsLoss, &row.ExceptionReason, &row.MetricsJSON, &row.ParameterSchemaJSON); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) UpdateJobCardActuals(ctx context.Context, cmd productionapp.JobCardActualsCommand) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var workOrderID int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s.job_cards
		SET planned_input_qty=$2,
		    actual_input_qty=$3,
		    actual_output_qty=$4,
		    actual_loss_qty=$5,
		    actual_loss_rate=$6,
		    exception_reason=$7,
		    metrics_json=$8::jsonb,
		    operator=COALESCE(NULLIF($9,''), operator)
		WHERE id=$1
		RETURNING work_order_id
	`, r.schema), cmd.ID, cmd.PlannedInputQty, cmd.ActualInputQty, cmd.ActualOutputQty, cmd.ActualLossQty, cmd.ActualLossRate, cmd.ExceptionReason, cmd.MetricsJSON, cmd.Actor).Scan(&workOrderID)
	if err == pgx.ErrNoRows {
		return fmt.Errorf("job card not found")
	}
	if err != nil {
		return err
	}
	summary, err := operationSummaryJSONForWorkOrderTx(ctx, tx, r.schema, workOrderID)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET operation_summary_json=$2::jsonb WHERE id=$1`, r.schema), workOrderID, summary); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "job_card", &cmd.ID, "update_metrics", postgresinfra.StrPtr("actual_loss"), nil, postgresinfra.StrPtr(fmt.Sprintf("%.4f", cmd.ActualLossQty)), postgresinfra.AuditMeta{"work_order_id": workOrderID, "planned_input_qty": cmd.PlannedInputQty, "actual_input_qty": cmd.ActualInputQty, "actual_output_qty": cmd.ActualOutputQty, "actual_loss_qty": cmd.ActualLossQty, "actual_loss_rate": cmd.ActualLossRate, "exception_reason": cmd.ExceptionReason}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) ListBatchCosts(ctx context.Context, query productionapp.BatchCostQuery) ([]productionapp.BatchCostRow, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,running_item_id,batch_id,product_name,COALESCE(material_cost,0),COALESCE(operation_cost,0),
		       COALESCE(total_cost,0),finished_g,COALESCE(unit_cost_per_kg,0),to_char(created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.production_batch_costs
		ORDER BY created_at DESC, id DESC
		LIMIT $1
	`, r.schema), query.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]productionapp.BatchCostRow, 0)
	for rows.Next() {
		var row productionapp.BatchCostRow
		if err := rows.Scan(&row.ID, &row.RunningItemID, &row.BatchID, &row.ProductName, &row.MaterialCost, &row.OperationCost, &row.TotalCost, &row.FinishedG, &row.UnitCostPerKG, &row.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
