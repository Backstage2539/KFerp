package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
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
	Seq                     int     `json:"seq"`
	OperationID             int64   `json:"operation_id"`
	WorkstationID           int64   `json:"workstation_id"`
	WorkstationCapacityID   int64   `json:"workstation_capacity_id"`
	Operation               string  `json:"operation"`
	Workstation             string  `json:"workstation"`
	WorkstationCapacityName string  `json:"workstation_capacity_name"`
	DefaultEquipment        string  `json:"default_equipment"`
	DefaultMinutes          int     `json:"default_minutes"`
	BatchSizeQty            float64 `json:"batch_size_qty"`
	BatchSizeUnit           string  `json:"batch_size_unit"`
	StandardMinutes         int     `json:"standard_minutes"`
	HourlyRate              float64 `json:"hourly_rate"`
	PlannedBatchCount       int     `json:"planned_batch_count"`
	PlannedMinutes          int     `json:"planned_minutes"`
	PlannedOperationCost    float64 `json:"planned_operation_cost"`
	RecordsLoss             bool    `json:"records_loss"`
	ParameterSchemaJSON     string  `json:"parameter_schema_json"`
	QualityChecklistJSON    string  `json:"quality_checklist_json"`
}

type plannedOperationMetrics struct {
	PlannedBatchCount    int
	PlannedMinutes       int
	PlannedOperationCost float64
}

func plannedJobCardMetrics(op processSnapshotOperation, plannedG int64) plannedOperationMetrics {
	standardMinutes := op.StandardMinutes
	if standardMinutes <= 0 {
		standardMinutes = op.DefaultMinutes
	}
	batchCount := op.PlannedBatchCount
	if batchCount <= 0 {
		batchCount = ceilPlannedBatchCount(plannedG, op.BatchSizeQty, op.BatchSizeUnit)
	}
	plannedMinutes := op.PlannedMinutes
	if plannedMinutes <= 0 && batchCount > 0 && standardMinutes > 0 {
		plannedMinutes = batchCount * standardMinutes
	}
	plannedOperationCost := op.PlannedOperationCost
	if plannedOperationCost <= 0 {
		plannedOperationCost = plannedJobCardOperationCost(plannedMinutes, op.HourlyRate)
	}
	return plannedOperationMetrics{PlannedBatchCount: batchCount, PlannedMinutes: plannedMinutes, PlannedOperationCost: plannedOperationCost}
}

func ceilPlannedBatchCount(plannedG int64, batchSizeQty float64, batchSizeUnit string) int {
	if plannedG <= 0 || batchSizeQty <= 0 {
		return 0
	}
	plannedQty := float64(plannedG)
	switch strings.ToLower(strings.TrimSpace(batchSizeUnit)) {
	case "kg", "千克", "公斤":
		plannedQty = plannedQty / 1000
	case "g", "克":
	default:
		plannedQty = plannedQty / 1000
	}
	return int(math.Ceil(plannedQty / batchSizeQty))
}

func plannedJobCardOperationCost(plannedMinutes int, hourlyRate float64) float64 {
	if plannedMinutes <= 0 || hourlyRate <= 0 {
		return 0
	}
	return math.Round((float64(plannedMinutes)/60*hourlyRate)*100) / 100
}

type processTemplateSnapshot struct {
	Source               string                     `json:"source"`
	ID                   int64                      `json:"id"`
	RouteID              int64                      `json:"route_id"`
	RouteName            string                     `json:"route_name"`
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

type latestUsableBomRoute struct {
	ProductID        int64
	ProductName      string
	BomID            int64
	BomCode          string
	BomName          string
	BomVersionID     int64
	BomVersionNo     string
	ProcessRouteID   int64
	ProcessRouteName string
	YieldRate        float64
}

func resolveLatestUsableBomRouteForProductTx(ctx context.Context, tx pgx.Tx, schema string, productID int64, productName string) (latestUsableBomRoute, error) {
	out := latestUsableBomRoute{ProductID: productID, ProductName: strings.TrimSpace(productName)}
	if out.ProductName == "" {
		out.ProductName = fmt.Sprintf("product#%d", productID)
	}
	var defaultBomID int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(NULLIF(ppc.production_bom_id,0), pbb.bom_id, 0)
		FROM %s.products p
		LEFT JOIN %s.product_production_configs ppc ON ppc.product_id=p.id
		LEFT JOIN %s.product_production_bom_bindings pbb ON pbb.product_id=p.id
		WHERE p.id=$1
	`, schema, schema, schema), productID).Scan(&defaultBomID)
	if err != nil && err != pgx.ErrNoRows {
		return out, err
	}
	if defaultBomID > 0 {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT pb.id, COALESCE(pb.code,''), COALESCE(pb.name,''), COALESCE(p.name,'')
			FROM %s.production_boms pb
			LEFT JOIN %s.products p ON p.id=pb.output_product_id
			WHERE pb.id=$1
			  AND pb.output_product_id=$2
			  AND COALESCE(NULLIF(pb.status,''),'active')='active'
			LIMIT 1
		`, schema, schema), defaultBomID, productID).Scan(&out.BomID, &out.BomCode, &out.BomName, &out.ProductName)
		if err == pgx.ErrNoRows {
			return out, fmt.Errorf("default production BOM is no longer an output BOM: %s", out.ProductName)
		}
		if err != nil {
			return out, err
		}
	} else {
		rows, err := tx.Query(ctx, fmt.Sprintf(`
			SELECT pb.id, COALESCE(pb.code,''), COALESCE(pb.name,''), COALESCE(p.name,'')
			FROM %s.production_boms pb
			LEFT JOIN %s.products p ON p.id=pb.output_product_id
			WHERE pb.output_product_id=$1
			  AND COALESCE(NULLIF(pb.status,''),'active')='active'
			ORDER BY pb.updated_at DESC, pb.id DESC
		`, schema, schema), productID)
		if err != nil {
			return out, err
		}
		defer rows.Close()
		for rows.Next() {
			var candidate latestUsableBomRoute
			if err := rows.Scan(&candidate.BomID, &candidate.BomCode, &candidate.BomName, &candidate.ProductName); err != nil {
				return out, err
			}
			if out.BomID == 0 {
				out.BomID = candidate.BomID
				out.BomCode = candidate.BomCode
				out.BomName = candidate.BomName
				out.ProductName = candidate.ProductName
			} else {
				return out, fmt.Errorf("multiple active production BOMs found: %s, please set default production BOM", out.ProductName)
			}
		}
		if err := rows.Err(); err != nil {
			return out, err
		}
		if out.BomID <= 0 {
			return out, fmt.Errorf("latest usable production BOM version not found: %s", out.ProductName)
		}
	}
	if strings.TrimSpace(out.ProductName) == "" {
		out.ProductName = fmt.Sprintf("product#%d", productID)
	}
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT v.id,
		       COALESCE(v.version_no,''),
		       COALESCE(v.process_route_id,0),
		       COALESCE(pr.name,''),
		       COALESCE(v.yield_rate,0.8)::float8
		FROM %s.production_bom_versions v
		LEFT JOIN %s.process_routes pr ON pr.id=v.process_route_id AND pr.status='active'
		WHERE v.bom_id=$1 AND v.status='published'
		ORDER BY v.published_at DESC NULLS LAST, v.created_at DESC, v.id DESC
		LIMIT 1
	`, schema, schema), out.BomID).Scan(&out.BomVersionID, &out.BomVersionNo, &out.ProcessRouteID, &out.ProcessRouteName, &out.YieldRate)
	if err == pgx.ErrNoRows {
		return out, fmt.Errorf("latest usable production BOM version not found: %s/%s", firstNonEmpty(out.BomName, out.BomCode), out.ProductName)
	}
	if err != nil {
		return out, err
	}
	if out.ProcessRouteID <= 0 || strings.TrimSpace(out.ProcessRouteName) == "" {
		return out, fmt.Errorf("最新可用 BOM 版本未配置工艺路线: %s/%s/%s", firstNonEmpty(out.BomName, out.BomCode), out.BomVersionNo, out.ProductName)
	}
	return out, nil
}

func loadBoundBomVersionIDForProductTx(ctx context.Context, tx pgx.Tx, schema string, productID int64) (int64, error) {
	var versionID int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(output_bom.bom_version_id, 0)
		FROM %s.products p
		LEFT JOIN %s.product_production_configs ppc ON ppc.product_id=p.id
		LEFT JOIN %s.product_production_bom_bindings pbb ON pbb.product_id=p.id
		LEFT JOIN LATERAL (
			SELECT latest.id AS bom_version_id
			FROM %s.production_boms pb
			JOIN LATERAL (
				SELECT v.id, v.published_at, v.created_at
				FROM %s.production_bom_versions v
				WHERE v.bom_id=pb.id
				  AND v.status='published'
				  AND EXISTS (SELECT 1 FROM %s.production_bom_version_items item WHERE item.version_id=v.id)
				ORDER BY v.published_at DESC NULLS LAST, v.created_at DESC, v.id DESC
				LIMIT 1
			) latest ON true
			WHERE pb.output_product_id=p.id
			  AND COALESCE(NULLIF(pb.status,''),'active')='active'
			ORDER BY CASE WHEN pb.id=COALESCE(NULLIF(ppc.production_bom_id,0), pbb.bom_id, 0) THEN 0 ELSE 1 END,
			         latest.published_at DESC NULLS LAST, latest.created_at DESC, latest.id DESC, pb.id DESC
			LIMIT 1
		) output_bom ON true
		WHERE p.id=$1
	`, schema, schema, schema, schema, schema, schema), productID).Scan(&versionID)
	if err == pgx.ErrNoRows {
		return 0, nil
	}
	return versionID, err
}

func loadProductProductionConfigSnapshotForWorkOrderTx(ctx context.Context, tx pgx.Tx, schema string, productID int64) ([]byte, error) {
	var raw string
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		WITH config AS (
			SELECT ppc.product_id,
				       COALESCE(ppc.production_bom_id,0) AS production_bom_id,
				       COALESCE(ppc.production_bom_version_id,0) AS production_bom_version_id,
				       COALESCE(ppc.process_route_id,0) AS process_route_id,
				       COALESCE(ppc.industry_field_template_id,0) AS industry_field_template_id,
				       COALESCE(ppc.expected_loss_rate,0)::float8 AS expected_loss_rate,
			       COALESCE(ppc.note,'') AS note
			FROM %[1]s.product_production_configs ppc
			WHERE ppc.product_id=$1
		),
		fields AS (
			SELECT ppcf.product_id,
			       jsonb_agg(jsonb_build_object(
			         'field_key', ppcf.field_key,
			         'label', ppcf.label,
			         'field_type', ppcf.field_type,
			         'unit', ppcf.unit,
			         'value_text', ppcf.value_text,
				         'value_number', ppcf.value_number,
				         'value_bool', ppcf.value_bool,
				         'template_field_key', COALESCE(ppcf.template_field_key,''),
				         'required', COALESCE(ppcf.required,false),
				         'options_json', COALESCE(ppcf.options_json, '[]'::jsonb),
				         'show_in_price_list', ppcf.show_in_price_list,
			         'sort_order', ppcf.sort_order
			       ) ORDER BY ppcf.sort_order, ppcf.id) AS fields_json
			FROM %[1]s.product_production_config_fields ppcf
			WHERE ppcf.product_id=$1
			GROUP BY ppcf.product_id
		)
		SELECT COALESCE(jsonb_build_object(
			'product_id', c.product_id,
			'production_bom_id', c.production_bom_id,
				'production_bom_version_id', c.production_bom_version_id,
				'process_route_id', c.process_route_id,
				'industry_field_template_id', c.industry_field_template_id,
				'expected_loss_rate', c.expected_loss_rate,
			'note', c.note,
			'fields', COALESCE(f.fields_json, '[]'::jsonb)
		), '{}'::jsonb)::text
		FROM config c
		LEFT JOIN fields f ON f.product_id=c.product_id
	`, schema), productID).Scan(&raw)
	if err == pgx.ErrNoRows {
		return []byte("{}"), nil
	}
	if err != nil {
		if strings.Contains(err.Error(), "product_production_configs") {
			return []byte("{}"), nil
		}
		return nil, err
	}
	return []byte(raw), nil
}

func loadProcessRouteSnapshotByIDTx(ctx context.Context, tx pgx.Tx, schema string, routeID int64, productID int64) (*processTemplateSnapshot, []byte, error) {
	if routeID <= 0 {
		return nil, nil, nil
	}
	var snapshot processTemplateSnapshot
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT pr.id, pr.name, pr.default_equipment, pr.default_minutes
		FROM %[1]s.process_routes pr
		WHERE pr.id=$1 AND pr.status='active'
		LIMIT 1
	`, schema), routeID).Scan(&snapshot.ID, &snapshot.Name, &snapshot.DefaultEquipment, &snapshot.DefaultMinutes)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	snapshot.Source = "process_route"
	snapshot.RouteID = snapshot.ID
	snapshot.RouteName = snapshot.Name
	snapshot.ProductID = productID
	snapshot.DefaultEquipment = ""
	snapshot.DefaultMinutes = 0
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT pro.seq,pro.operation_id,pro.workstation_id,COALESCE(pro.workstation_capacity_id,0),
		       COALESCE(NULLIF(mo.name,''), pro.operation),COALESCE(NULLIF(mw.name,''), pro.workstation),COALESCE(pro.workstation_capacity_name,''),
		       pro.default_equipment,pro.default_minutes,
		       COALESCE(pro.batch_size_qty,0)::float8,
		       COALESCE(pro.batch_size_unit,''),
		       COALESCE(pro.standard_minutes,0),
		       COALESCE(pro.hourly_rate,0)::float8,
		       COALESCE(pro.planned_batch_count,0),
		       COALESCE(pro.planned_minutes,0),
		       COALESCE(pro.planned_operation_cost,0)::float8,
		       pro.records_loss,
		       COALESCE(pro.quality_checklist_json,'[]'::jsonb)::text
		FROM %[1]s.process_route_operations pro
		LEFT JOIN %[1]s.manufacturing_operations mo ON mo.id=pro.operation_id
		LEFT JOIN %[1]s.manufacturing_workstations mw ON mw.id=pro.workstation_id
		WHERE pro.route_id=$1
		ORDER BY pro.seq, pro.id
	`, schema), snapshot.ID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var op processSnapshotOperation
		if err := rows.Scan(
			&op.Seq, &op.OperationID, &op.WorkstationID, &op.WorkstationCapacityID,
			&op.Operation, &op.Workstation, &op.WorkstationCapacityName, &op.DefaultEquipment, &op.DefaultMinutes,
			&op.BatchSizeQty, &op.BatchSizeUnit, &op.StandardMinutes, &op.HourlyRate, &op.PlannedBatchCount, &op.PlannedMinutes, &op.PlannedOperationCost,
			&op.RecordsLoss, &op.QualityChecklistJSON,
		); err != nil {
			return nil, nil, err
		}
		op = routeSequenceOnlyOperation(op)
		op.ParameterSchemaJSON = "{}"
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

func routeSequenceOnlyOperation(op processSnapshotOperation) processSnapshotOperation {
	op.WorkstationID = 0
	op.WorkstationCapacityID = 0
	op.Workstation = ""
	op.WorkstationCapacityName = ""
	op.DefaultEquipment = ""
	op.DefaultMinutes = 0
	op.BatchSizeQty = 0
	op.BatchSizeUnit = ""
	op.StandardMinutes = 0
	op.HourlyRate = 0
	op.PlannedBatchCount = 0
	op.PlannedMinutes = 0
	op.PlannedOperationCost = 0
	return op
}

func createWorkOrderForRunningItemTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID int64, batchID string, productID int64, productName string, specG int64, plannedG int64, materialSnapshot []byte, operationTemplateID int64, operator string) (int64, error) {
	processSnapshot, processSnapshotJSON, err := loadProcessRouteSnapshotForWorkOrderTx(ctx, tx, schema, productID)
	if err != nil {
		return 0, err
	}
	if processSnapshot == nil {
		processSnapshot, processSnapshotJSON, err = loadActiveProcessTemplateSnapshotTx(ctx, tx, schema, productID)
		if err != nil {
			return 0, err
		}
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
	productionConfigSnapshot, err := loadProductProductionConfigSnapshotForWorkOrderTx(ctx, tx, schema, productID)
	if err != nil {
		return 0, err
	}
	if len(productionConfigSnapshot) == 0 {
		productionConfigSnapshot = []byte("{}")
	}
	bomVersionID, err := loadBoundBomVersionIDForProductTx(ctx, tx, schema, productID)
	if err != nil {
		return 0, err
	}

	var workOrderID int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.work_orders WHERE running_item_id=$1 FOR UPDATE`, schema), runningItemID).Scan(&workOrderID)
	if err == nil {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.work_orders
			SET status='running',
			    batch_id=$2,
			    product_id=$3,
			    product_name=$4,
			    spec_g=$5,
			    planned_g=$6,
			    material_snapshot=$7,
			    bom_version_id=$8,
			    operation_template_id=$9,
			    process_template_id=$10,
			    process_template_name=$11,
			    process_snapshot_json=$12,
			    production_config_snapshot_json=$13,
			    customer_product_snapshot_json=$14
			WHERE id=$1
		`, schema), workOrderID, batchID, productID, productName, specG, plannedG, materialSnapshot, bomVersionID, operationTemplateID, processTemplateID, processTemplateName, processSnapshotJSON, productionConfigSnapshot, customerProductSnapshot); err != nil {
			return 0, err
		}
	} else if err == pgx.ErrNoRows {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.work_orders(
				work_order_no,running_item_id,batch_id,product_id,product_name,spec_g,planned_g,status,
				material_snapshot,bom_version_id,operation_template_id,process_template_id,process_template_name,process_snapshot_json,production_config_snapshot_json,customer_product_snapshot_json,created_at
			)
			VALUES($1,$2,$3,$4,$5,$6,$7,'running',$8,$9,$10,$11,$12,$13,$14,$15,now())
			RETURNING id
		`, schema), workOrderNo(runningItemID), runningItemID, batchID, productID, productName, specG, plannedG, materialSnapshot, bomVersionID, operationTemplateID, processTemplateID, processTemplateName, processSnapshotJSON, productionConfigSnapshot, customerProductSnapshot).Scan(&workOrderID); err != nil {
			return 0, err
		}
	} else {
		return 0, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.job_cards WHERE work_order_id=$1`, schema), workOrderID); err != nil {
		return 0, err
	}

	if processSnapshot != nil && len(processSnapshot.Operations) > 0 {
		for _, op := range processSnapshot.Operations {
			metrics := plannedJobCardMetrics(op, plannedG)
			if _, err := tx.Exec(ctx, fmt.Sprintf(`
				INSERT INTO %s.job_cards(
					work_order_id,sequence_no,operation_id,workstation_id,operation,workstation,
					workstation_capacity_id,workstation_capacity_name,batch_size_qty,batch_size_unit,
					planned_batch_count,planned_minutes,hourly_rate,planned_operation_cost,
					status,started_at,operator,planned_input_qty,records_loss,parameter_schema_json
				)
				VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,'running',now(),$15,$16,$17,$18::jsonb)
			`, schema), workOrderID, op.Seq, op.OperationID, op.WorkstationID, op.Operation, op.Workstation, op.WorkstationCapacityID, op.WorkstationCapacityName, op.BatchSizeQty, op.BatchSizeUnit, metrics.PlannedBatchCount, metrics.PlannedMinutes, op.HourlyRate, metrics.PlannedOperationCost, operator, plannedG, op.RecordsLoss, defaultJSONObject(op.ParameterSchemaJSON)); err != nil {
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
	var productionPlanID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s.work_orders
		SET status='completed', actual_cost=$2, completed_at=now()
		WHERE running_item_id=$1
		RETURNING id,production_plan_id
	`, schema), runningItemID, actualCost).Scan(&workOrderID, &productionPlanID); err != nil {
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
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET operation_summary_json=$2::jsonb WHERE id=$1`, schema), workOrderID, summary); err != nil {
		return err
	}
	if productionPlanID > 0 {
		return completeProductionPlanIfAllWorkOrdersDoneTx(ctx, tx, schema, productionPlanID, operator)
	}
	return nil
}

func completeProductionPlanIfAllWorkOrdersDoneTx(ctx context.Context, tx pgx.Tx, schema string, productionPlanID int64, operator string) error {
	var status string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.production_plans WHERE id=$1 FOR UPDATE`, schema), productionPlanID).Scan(&status); err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		return err
	}
	if status == "completed" || status == "cancelled" {
		return nil
	}
	var remaining int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(1)
		FROM %s.work_orders
		WHERE production_plan_id=$1 AND status <> 'completed'
	`, schema), productionPlanID).Scan(&remaining); err != nil {
		return err
	}
	if remaining > 0 {
		return nil
	}
	tag, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.production_plans
		SET status='completed', completed_at=now()
		WHERE id=$1 AND status <> 'completed'
	`, schema), productionPlanID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	return postgresinfra.AuditInsertTx(ctx, tx, schema, operator, "production_plan", &productionPlanID, "complete", postgresinfra.StrPtr("status"), postgresinfra.StrPtr(status), postgresinfra.StrPtr("completed"), postgresinfra.AuditMeta{"all_work_orders_completed": true})
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

func loadProcessRouteSnapshotForWorkOrderTx(ctx context.Context, tx pgx.Tx, schema string, productID int64) (*processTemplateSnapshot, []byte, error) {
	var snapshot processTemplateSnapshot
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT pr.id, pr.name, pr.default_equipment, pr.default_minutes
		FROM %[1]s.product_production_configs ppc
		JOIN %[1]s.process_routes pr ON pr.id=ppc.process_route_id
		WHERE ppc.product_id=$1
		  AND COALESCE(ppc.process_route_id,0)>0
		  AND pr.status='active'
		LIMIT 1
	`, schema), productID).Scan(&snapshot.ID, &snapshot.Name, &snapshot.DefaultEquipment, &snapshot.DefaultMinutes)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil, nil
		}
		if strings.Contains(err.Error(), "process_routes") || strings.Contains(err.Error(), "product_production_configs") {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	snapshot.Source = "process_route"
	snapshot.RouteID = snapshot.ID
	snapshot.RouteName = snapshot.Name
	snapshot.ProductID = productID
	snapshot.DefaultEquipment = ""
	snapshot.DefaultMinutes = 0
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT pro.seq,pro.operation_id,pro.workstation_id,COALESCE(pro.workstation_capacity_id,0),
		       COALESCE(NULLIF(mo.name,''), pro.operation),COALESCE(NULLIF(mw.name,''), pro.workstation),COALESCE(pro.workstation_capacity_name,''),
		       pro.default_equipment,pro.default_minutes,
		       COALESCE(pro.batch_size_qty,0)::float8,
		       COALESCE(pro.batch_size_unit,''),
		       COALESCE(pro.standard_minutes,0),
		       COALESCE(pro.hourly_rate,0)::float8,
		       COALESCE(pro.planned_batch_count,0),
		       COALESCE(pro.planned_minutes,0),
		       COALESCE(pro.planned_operation_cost,0)::float8,
		       pro.records_loss,
		       COALESCE(pro.quality_checklist_json,'[]'::jsonb)::text
		FROM %[1]s.process_route_operations pro
		LEFT JOIN %[1]s.manufacturing_operations mo ON mo.id=pro.operation_id
		LEFT JOIN %[1]s.manufacturing_workstations mw ON mw.id=pro.workstation_id
		WHERE pro.route_id=$1
		ORDER BY pro.seq, pro.id
	`, schema), snapshot.ID)
	if err != nil {
		if strings.Contains(err.Error(), "process_route_operations") {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var op processSnapshotOperation
		if err := rows.Scan(
			&op.Seq, &op.OperationID, &op.WorkstationID, &op.WorkstationCapacityID,
			&op.Operation, &op.Workstation, &op.WorkstationCapacityName, &op.DefaultEquipment, &op.DefaultMinutes,
			&op.BatchSizeQty, &op.BatchSizeUnit, &op.StandardMinutes, &op.HourlyRate, &op.PlannedBatchCount, &op.PlannedMinutes, &op.PlannedOperationCost,
			&op.RecordsLoss, &op.QualityChecklistJSON,
		); err != nil {
			return nil, nil, err
		}
		op = routeSequenceOnlyOperation(op)
		op.ParameterSchemaJSON = "{}"
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
	snapshot.Source = "process_template"
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT seq,operation_id,workstation_id,COALESCE(workstation_capacity_id,0),
		       operation,workstation,COALESCE(workstation_capacity_name,''),
		       default_equipment,default_minutes,
		       COALESCE(batch_size_qty,0)::float8,
		       COALESCE(batch_size_unit,''),
		       COALESCE(standard_minutes,0),
		       COALESCE(hourly_rate,0)::float8,
		       COALESCE(planned_batch_count,0),
		       COALESCE(planned_minutes,0),
		       COALESCE(planned_operation_cost,0)::float8,
		       records_loss,
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
		if err := rows.Scan(
			&op.Seq, &op.OperationID, &op.WorkstationID, &op.WorkstationCapacityID,
			&op.Operation, &op.Workstation, &op.WorkstationCapacityName, &op.DefaultEquipment, &op.DefaultMinutes,
			&op.BatchSizeQty, &op.BatchSizeUnit, &op.StandardMinutes, &op.HourlyRate, &op.PlannedBatchCount, &op.PlannedMinutes, &op.PlannedOperationCost,
			&op.RecordsLoss, &op.ParameterSchemaJSON, &op.QualityChecklistJSON,
		); err != nil {
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
			'operation_id', operation_id,
			'workstation_id', workstation_id,
			'operation', operation,
			'workstation', workstation,
			'workstation_capacity_id', workstation_capacity_id,
			'workstation_capacity_name', workstation_capacity_name,
			'batch_size_qty', batch_size_qty,
			'batch_size_unit', batch_size_unit,
			'planned_batch_count', planned_batch_count,
			'planned_minutes', planned_minutes,
			'hourly_rate', hourly_rate,
			'planned_operation_cost', planned_operation_cost,
			'actual_minutes', actual_minutes,
			'actual_operation_cost', actual_operation_cost,
			'status', status,
			'records_loss', records_loss,
			'planned_input_qty', planned_input_qty,
			'actual_input_qty', actual_input_qty,
			'actual_output_qty', actual_output_qty,
			'actual_loss_qty', actual_loss_qty,
			'actual_loss_rate', actual_loss_rate,
			'loss_reason', loss_reason,
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
			WHEN COALESCE(jc.actual_operation_cost,0) > 0
				THEN COALESCE(jc.actual_operation_cost,0)
			WHEN COALESCE(jc.planned_operation_cost,0) > 0
				THEN COALESCE(jc.planned_operation_cost,0)
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
	if query.ID > 0 {
		args = append(args, query.ID)
		where += fmt.Sprintf(" AND wo.id=$%d", len(args))
	}
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
		SELECT wo.id,wo.work_order_no,wo.running_item_id,wo.production_plan_id,wo.production_plan_item_id,
		       wo.batch_id,wo.product_id,wo.product_name,wo.spec_g,wo.planned_g,COALESCE(NULLIF(wo.planned_output_g,0),wo.planned_g),wo.status,
		       COALESCE(wo.actual_cost,0),to_char(wo.created_at,'YYYY-MM-DD HH24:MI'),COALESCE(to_char(wo.completed_at,'YYYY-MM-DD HH24:MI'),''),
		       COALESCE(NULLIF(wo.production_config_snapshot_json->'fields'->0->>'value_text',''), NULLIF(bound_bv.special_attrs_json->>'roast_level',''), COALESCE(p.roast_level,'')),
		       COALESCE(NULLIF(ri.bom_yield_rate,0), CASE WHEN ppc.product_id IS NOT NULL THEN 1 - COALESCE(NULLIF(ppc.expected_loss_rate,0), 0) ELSE NULL END, bound_bv.yield_rate, pb.yield_rate, 0),
		       COALESCE(NULLIF(ri.input_g,0), wo.planned_g, 0),
		       COALESCE(ri.planned_units,0),
		       COALESCE(ri.planned_loose_g,0),
		       COALESCE(NULLIF(ri.order_nos,''), wo.order_nos, ''),
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
		       COALESCE(NULLIF(wo.bom_version_id,0), NULLIF(ppc.production_bom_version_id,0), pbb.bom_version_id,0),
		       COALESCE(wo.process_template_id,0),
		       COALESCE(wo.process_template_name,''),
		       COALESCE(wo.process_snapshot_json,'{}'::jsonb)::text,
		       COALESCE(NULLIF(wo.operation_summary_json,'[]'::jsonb), (
		           SELECT jsonb_agg(jsonb_build_object(
		               'id', jc.id,
		               'sequence_no', jc.sequence_no,
		               'operation_id', jc.operation_id,
		               'workstation_id', jc.workstation_id,
		               'operation', jc.operation,
		               'workstation', jc.workstation,
		               'workstation_capacity_id', jc.workstation_capacity_id,
		               'workstation_capacity_name', jc.workstation_capacity_name,
		               'batch_size_qty', jc.batch_size_qty,
		               'batch_size_unit', jc.batch_size_unit,
		               'planned_batch_count', jc.planned_batch_count,
		               'planned_minutes', jc.planned_minutes,
		               'hourly_rate', jc.hourly_rate,
		               'planned_operation_cost', jc.planned_operation_cost,
		               'actual_minutes', jc.actual_minutes,
		               'actual_operation_cost', jc.actual_operation_cost,
		               'status', jc.status,
		               'records_loss', jc.records_loss,
		               'planned_input_qty', jc.planned_input_qty,
		               'actual_input_qty', jc.actual_input_qty,
		               'actual_output_qty', jc.actual_output_qty,
		               'actual_loss_qty', jc.actual_loss_qty,
		               'actual_loss_rate', jc.actual_loss_rate,
		               'loss_reason', jc.loss_reason,
		               'exception_reason', jc.exception_reason
		           ) ORDER BY jc.sequence_no, jc.id)
		           FROM %s.job_cards jc
		           WHERE jc.work_order_id=wo.id
		       ), '[]'::jsonb)::text,
		       COALESCE(to_char(wo.planned_start_at,'YYYY-MM-DD HH24:MI'),''),
		       COALESCE(to_char(wo.planned_end_at,'YYYY-MM-DD HH24:MI'),''),
		       COALESCE(wo.shift_code,''),
		       COALESCE(wo.assigned_to,''),
		       COALESCE(wo.priority,0),
		       COALESCE(wo.scheduling_note,''),
		       COALESCE(wo.work_center,''),
		       COALESCE((
		           SELECT string_agg(COALESCE(m.name,'') || ' ' || COALESCE(NULLIF(trim(trailing '.' from trim(trailing '0' from COALESCE(bi.ratio_pct,0)::text)), ''), '0') || '%%', '、' ORDER BY bi.id)
		           FROM (
		               SELECT pbi.id,pbi.material_id,pbi.ratio_pct
		               FROM %s.production_bom_version_items pbi
		               WHERE COALESCE(NULLIF(wo.bom_version_id,0), NULLIF(ppc.production_bom_version_id,0), pbb.bom_version_id,0) > 0
		                 AND pbi.version_id=COALESCE(NULLIF(wo.bom_version_id,0), NULLIF(ppc.production_bom_version_id,0), pbb.bom_version_id)
		               UNION ALL
		               SELECT lbi.id,lbi.material_id,lbi.ratio_pct
		               FROM %s.product_bom_items lbi
		               WHERE COALESCE(NULLIF(wo.bom_version_id,0), NULLIF(ppc.production_bom_version_id,0), pbb.bom_version_id,0)=0 AND lbi.product_id=wo.product_id
		           ) bi
		           LEFT JOIN %s.materials m ON m.id=bi.material_id
		       ), '')
		FROM %s.work_orders wo
		LEFT JOIN %s.produce_running_items ri ON ri.id=wo.running_item_id
		LEFT JOIN %s.products p ON p.id=wo.product_id
		LEFT JOIN %s.product_production_configs ppc ON ppc.product_id=wo.product_id
		LEFT JOIN %s.product_production_bom_bindings pbb ON pbb.product_id=wo.product_id
		LEFT JOIN %s.production_bom_versions bound_bv ON bound_bv.id=COALESCE(NULLIF(wo.bom_version_id,0), NULLIF(ppc.production_bom_version_id,0), pbb.bom_version_id)
		LEFT JOIN %s.product_bom pb ON pb.product_id=wo.product_id
		WHERE %s
		ORDER BY wo.created_at DESC, wo.id DESC
		LIMIT $%d
	`, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, where, limitArg), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]productionapp.WorkOrderRow, 0)
	for rows.Next() {
		var row productionapp.WorkOrderRow
		var snapshotText, fallbackMaterialSummary string
		if err := rows.Scan(
			&row.ID, &row.WorkOrderNo, &row.RunningItemID, &row.ProductionPlanID, &row.ProductionPlanItemID, &row.BatchID, &row.ProductID, &row.ProductName, &row.SpecG, &row.PlannedG, &row.PlannedOutputG, &row.Status, &row.ActualCost, &row.CreatedAt, &row.CompletedAt,
			&row.RoastLevel, &row.YieldRate, &row.SuggestedInputG, &row.PlannedUnits, &row.PlannedLooseG, &row.OrderNos, &snapshotText, &row.WIPReservedG, &row.WIPConsumedG, &row.WIPRemainingReservedG, &row.BomVersionID, &row.ProcessTemplateID, &row.ProcessTemplateName, &row.ProcessSnapshotJSON, &row.OperationSummaryJSON,
			&row.PlannedStartAt, &row.PlannedEndAt, &row.ShiftCode, &row.AssignedTo, &row.Priority, &row.SchedulingNote, &row.WorkCenter, &fallbackMaterialSummary,
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
	if query.WorkOrderID > 0 {
		args = append(args, query.WorkOrderID)
		where += fmt.Sprintf(" AND jc.work_order_id=$%d", len(args))
	}
	if query.Status != "" {
		args = append(args, query.Status)
		where += fmt.Sprintf(" AND jc.status=$%d", len(args))
	}
	args = append(args, query.Limit)
	limitArg := len(args)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT jc.id,jc.work_order_id,
		       COALESCE(wo.work_order_no,''),COALESCE(wo.product_id,0),COALESCE(wo.product_name,''),
		       COALESCE(wo.spec_g,0),COALESCE(wo.order_nos,''),COALESCE(wo.planned_g,0),
		       COALESCE(NULLIF(wo.planned_output_g,0),wo.planned_g,0),COALESCE(wo.bom_version_id,0),
		       COALESCE(wo.material_snapshot,'[]'::jsonb)::text,
		       COALESCE(wo.process_snapshot_json,'{}'::jsonb)::text,
		       COALESCE(wo.production_config_snapshot_json,'{}'::jsonb)::text,
		       COALESCE(wo.customer_product_snapshot_json,'{}'::jsonb)::text,
		       jc.sequence_no,
		       COALESCE(jc.operation_id,0),COALESCE(jc.workstation_id,0),
		       jc.operation,jc.workstation,
		       COALESCE(jc.workstation_capacity_id,0),COALESCE(jc.workstation_capacity_name,''),
		       COALESCE(jc.batch_size_qty,0)::float8,COALESCE(jc.batch_size_unit,''),
		       COALESCE(jc.planned_batch_count,0),COALESCE(jc.planned_minutes,0),
		       COALESCE(jc.hourly_rate,0)::float8,COALESCE(jc.planned_operation_cost,0)::float8,
		       COALESCE(jc.actual_minutes,0),COALESCE(jc.actual_operation_cost,0)::float8,
		       jc.status,
		       COALESCE(to_char(jc.started_at,'YYYY-MM-DD HH24:MI'),''),COALESCE(to_char(jc.paused_at,'YYYY-MM-DD HH24:MI'),''),COALESCE(to_char(jc.resumed_at,'YYYY-MM-DD HH24:MI'),''),COALESCE(to_char(jc.completed_at,'YYYY-MM-DD HH24:MI'),''),jc.operator,
		       COALESCE(jc.planned_input_qty,0)::float8,
		       COALESCE(jc.actual_input_qty,0)::float8,
		       COALESCE(jc.actual_output_qty,0)::float8,
		       COALESCE(jc.actual_loss_qty,0)::float8,
		       COALESCE(jc.actual_loss_rate,0)::float8,
		       COALESCE(jc.records_loss,false),
		       COALESCE(jc.loss_reason,''),
		       COALESCE(jc.exception_reason,''),
		       COALESCE(jc.metrics_json,'{}'::jsonb)::text,
		       COALESCE(jc.parameter_schema_json,'{}'::jsonb)::text,
		       COALESCE(to_char(jc.planned_start_at,'YYYY-MM-DD HH24:MI'),''),
		       COALESCE(to_char(jc.planned_end_at,'YYYY-MM-DD HH24:MI'),''),
		       COALESCE(jc.shift_code,''),
		       COALESCE(jc.assigned_to,''),
		       COALESCE(jc.priority,0),
		       COALESCE(jc.scheduling_note,''),
		       COALESCE(jc.work_center,'')
		FROM %s.job_cards jc
		LEFT JOIN %s.work_orders wo ON wo.id=jc.work_order_id
		WHERE %s
		ORDER BY jc.started_at DESC, jc.work_order_id DESC, jc.sequence_no, jc.id
		LIMIT $%d
	`, r.schema, r.schema, where, limitArg), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]productionapp.JobCardRow, 0)
	for rows.Next() {
		var row productionapp.JobCardRow
		if err := rows.Scan(
			&row.ID, &row.WorkOrderID, &row.WorkOrderNo, &row.ProductID, &row.ProductName,
			&row.SpecG, &row.OrderNos, &row.PlannedG, &row.PlannedOutputG, &row.BomVersionID,
			&row.MaterialSnapshot, &row.ProcessSnapshotJSON, &row.ProductionConfigSnapshotJSON, &row.CustomerProductSnapshotJSON,
			&row.SequenceNo, &row.OperationID, &row.WorkstationID,
			&row.Operation, &row.Workstation, &row.WorkstationCapacityID, &row.WorkstationCapacityName,
			&row.BatchSizeQty, &row.BatchSizeUnit, &row.PlannedBatchCount, &row.PlannedMinutes,
			&row.HourlyRate, &row.PlannedOperationCost, &row.ActualMinutes, &row.ActualOperationCost,
			&row.Status, &row.StartedAt, &row.PausedAt, &row.ResumedAt, &row.CompletedAt, &row.Operator,
			&row.PlannedInputQty, &row.ActualInputQty, &row.ActualOutputQty, &row.ActualLossQty, &row.ActualLossRate,
			&row.RecordsLoss, &row.LossReason, &row.ExceptionReason, &row.MetricsJSON, &row.ParameterSchemaJSON,
			&row.PlannedStartAt, &row.PlannedEndAt, &row.ShiftCode, &row.AssignedTo, &row.Priority, &row.SchedulingNote, &row.WorkCenter,
		); err != nil {
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
		    operator=COALESCE(NULLIF($9,''), operator),
		    actual_minutes=$10,
		    actual_operation_cost=CASE
		        WHEN $10 > 0 THEN ROUND(($10::numeric / 60.0) * COALESCE(hourly_rate,0), 4)
		        ELSE actual_operation_cost
		    END
		WHERE id=$1
		RETURNING work_order_id
	`, r.schema), cmd.ID, cmd.PlannedInputQty, cmd.ActualInputQty, cmd.ActualOutputQty, cmd.ActualLossQty, cmd.ActualLossRate, cmd.ExceptionReason, cmd.MetricsJSON, cmd.Actor, cmd.ActualMinutes).Scan(&workOrderID)
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
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "job_card", &cmd.ID, "update_metrics", postgresinfra.StrPtr("actual_loss"), nil, postgresinfra.StrPtr(fmt.Sprintf("%.4f", cmd.ActualLossQty)), postgresinfra.AuditMeta{"work_order_id": workOrderID, "planned_input_qty": cmd.PlannedInputQty, "actual_input_qty": cmd.ActualInputQty, "actual_output_qty": cmd.ActualOutputQty, "actual_loss_qty": cmd.ActualLossQty, "actual_loss_rate": cmd.ActualLossRate, "actual_minutes": cmd.ActualMinutes, "exception_reason": cmd.ExceptionReason}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) ListBatchCosts(ctx context.Context, query productionapp.BatchCostQuery) ([]productionapp.BatchCostRow, error) {
	args := []any{}
	where := "1=1"
	if query.RunningItemID > 0 {
		args = append(args, query.RunningItemID)
		where += fmt.Sprintf(" AND running_item_id=$%d", len(args))
	}
	args = append(args, query.Limit)
	limitArg := len(args)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,running_item_id,batch_id,product_name,COALESCE(material_cost,0),COALESCE(operation_cost,0),
		       COALESCE(total_cost,0),finished_g,COALESCE(unit_cost_per_kg,0),to_char(created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.production_batch_costs
		WHERE %s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d
	`, r.schema, where, limitArg), args...)
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

func (r Repository) CancelWorkOrder(ctx context.Context, cmd productionapp.WorkOrderCancelCommand) (productionapp.WorkOrderRow, error) {
	var runningItemID int64
	var status string
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT running_item_id,status
		FROM %s.work_orders
		WHERE id=$1
	`, r.schema), cmd.ID).Scan(&runningItemID, &status); err != nil {
		if err == pgx.ErrNoRows {
			return productionapp.WorkOrderRow{}, fmt.Errorf("work order not found")
		}
		return productionapp.WorkOrderRow{}, err
	}
	if status == "completed" {
		return productionapp.WorkOrderRow{}, fmt.Errorf("work order already completed")
	}
	if status == "running" && runningItemID > 0 {
		if err := r.Cancel(ctx, productionapp.CancelCommand{ID: runningItemID, Operator: cmd.Operator}); err != nil {
			return productionapp.WorkOrderRow{}, err
		}
		tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return productionapp.WorkOrderRow{}, err
		}
		defer func() { _ = tx.Rollback(ctx) }()
		row, err := loadWorkOrderExecutionRowTx(ctx, tx, r.schema, cmd.ID)
		if err != nil {
			return productionapp.WorkOrderRow{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return productionapp.WorkOrderRow{}, err
		}
		return row, nil
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return productionapp.WorkOrderRow{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.work_orders
		SET status='cancelled', completed_at=now()
		WHERE id=$1 AND status <> 'cancelled'
	`, r.schema), cmd.ID)
	if err != nil {
		return productionapp.WorkOrderRow{}, err
	}
	if tag.RowsAffected() == 0 && status != "cancelled" {
		return productionapp.WorkOrderRow{}, fmt.Errorf("work order not found")
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.job_cards
		SET status='cancelled', completed_at=now(), operator=COALESCE(NULLIF(operator,''),$2)
		WHERE work_order_id=$1 AND status NOT IN ('completed','cancelled')
	`, r.schema), cmd.ID, cmd.Operator); err != nil {
		return productionapp.WorkOrderRow{}, err
	}
	if runningItemID > 0 {
		if err := releaseMaterialReservationsForRunningItemTx(ctx, tx, r.schema, runningItemID); err != nil {
			return productionapp.WorkOrderRow{}, err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "work_order", &cmd.ID, "cancel", postgresinfra.StrPtr("status"), postgresinfra.StrPtr(status), postgresinfra.StrPtr("cancelled"), postgresinfra.AuditMeta{"note": cmd.Note, "running_item_id": runningItemID}); err != nil {
		return productionapp.WorkOrderRow{}, err
	}
	row, err := loadWorkOrderExecutionRowTx(ctx, tx, r.schema, cmd.ID)
	if err != nil {
		return productionapp.WorkOrderRow{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productionapp.WorkOrderRow{}, err
	}
	return row, nil
}
