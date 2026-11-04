package production

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"math"
	app "orderapp/internal/application/production"
	stockapp "orderapp/internal/application/stock"
	infra "orderapp/internal/infrastructure/postgres"
	"orderapp/internal/infrastructure/postgres/customerstock"
	stockrepo "orderapp/internal/infrastructure/postgres/stock"
	"strings"
)

func (r Repository) PreviewOperationStart(ctx context.Context, id int64) (app.OperationCompletionPreview, error) {
	return r.PreviewOperationCompletion(ctx, id)
}

func (r Repository) startOperation(ctx context.Context, cmd app.JobCardActionCommand) (app.JobCardActionResult, error) {
	return retryOperationTransaction(ctx, func() (app.JobCardActionResult, error) {
		var out app.JobCardActionResult
		if cmd.EmployeeID <= 0 {
			return out, fmt.Errorf("请先绑定员工")
		}
		if strings.TrimSpace(cmd.RequestID) == "" || strings.TrimSpace(cmd.ExpectedVersion) == "" {
			return out, fmt.Errorf("请先确认用料并开工")
		}
		tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
		if err != nil {
			return out, err
		}
		defer tx.Rollback(ctx)
		week, _, err := app.NormalizeProductionRosterWeek(productionWorkDate(), nil)
		if err != nil {
			return out, err
		}
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, r.schema+":production_roster:"+week); err != nil {
			return out, err
		}
		var workID, runID int64
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT w.id,w.running_item_id FROM %[1]s.job_cards j JOIN %[1]s.work_orders w ON w.id=j.work_order_id WHERE j.id=$1`, r.schema), cmd.ID).Scan(&workID, &runID); err != nil {
			return out, err
		}
		if runID > 0 {
			if err = customerstock.LockRunningFulfillmentCustomersTx(ctx, tx, r.schema, runID); err != nil {
				return out, err
			}
		}
		if err = lockReceiptWorkOrderTx(ctx, tx, r.schema, workID); err != nil {
			return out, err
		}
		replayID, replay, err := requestProductionKeyTx(ctx, tx, r.schema, "operation_start", cmd.Operator, cmd.RequestID, cmd)
		if err != nil {
			return out, err
		}
		if replayID > 0 {
			err = json.Unmarshal(replay, &out)
			return out, err
		}
		out, err = r.startOperationLockedTx(ctx, tx, cmd)
		if err != nil {
			return out, err
		}
		if err = finishProductionKeyTx(ctx, tx, r.schema, "operation_start", cmd.Operator, cmd.RequestID, cmd.ID, out); err != nil {
			return out, err
		}
		return out, tx.Commit(ctx)
	})
}

func (r Repository) startOperationLockedTx(ctx context.Context, tx pgx.Tx, cmd app.JobCardActionCommand) (app.JobCardActionResult, error) {
	var out app.JobCardActionResult
	p, wo, err := r.operationCompletionContextTx(ctx, tx, cmd.ID, true)
	if err != nil {
		return out, err
	}
	if p.MaterialConsumptionPolicy != "bom_start_v1" {
		return out, fmt.Errorf("本工单不使用按批开工扣料")
	}
	if p.Version != cmd.ExpectedVersion {
		return out, fmt.Errorf("任务或库存已变化，请刷新；本次未扣料")
	}
	if !p.Enabled || p.BlockedReason != "" {
		return out, fmt.Errorf("不能开工：%s", p.BlockedReason)
	}
	if wo.Status == "completed" || wo.Status == "cancelled" {
		return out, fmt.Errorf("工单已结束")
	}
	var status, station string
	var stationID int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT status,workstation,workstation_id FROM %s.job_cards WHERE id=$1 FOR UPDATE`, r.schema), cmd.ID).Scan(&status, &station, &stationID); err != nil {
		return out, err
	}
	if !validJobCardTransition(status, "start") {
		return out, fmt.Errorf("本批已经开工或结束，请刷新")
	}
	owner, roster, err := resolveProductionWorkstationOwnerTx(ctx, tx, r.schema, stationID, station, productionWorkDate())
	if err != nil {
		return out, err
	}
	if roster && owner.EmployeeID != cmd.EmployeeID {
		return out, fmt.Errorf("请由今日工位负责人开工")
	}
	var qualityBlocked bool
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM (SELECT DISTINCT ON(job_card_id,scope,reference_type,reference_no) result FROM %s.quality_inspections WHERE (work_order_id=$1 AND job_card_id=0) OR job_card_id=$2 ORDER BY job_card_id,scope,reference_type,reference_no,created_at DESC,id DESC) latest WHERE lower(trim(result)) IN('hold','reject','failed','fail','blocked'))`, r.schema), wo.ID, cmd.ID).Scan(&qualityBlocked); err != nil {
		return out, err
	}
	if qualityBlocked {
		return out, fmt.Errorf("本任务被质检冻结，请先处理质检记录")
	}
	if cmd.InputUnit != p.InputUnit || cmd.ActualInputQty <= 0 {
		return out, fmt.Errorf("请确认本批实际投入数量和单位")
	}
	if math.Round(cmd.ActualInputQty*1e8) != math.Round(p.InputQty*1e8) && strings.TrimSpace(cmd.DifferenceReason) == "" {
		return out, fmt.Errorf("实际投入与建议数量不一致，请填写差异原因")
	}

	if err = app.ValidateOperationQuantity(cmd.ActualInputQty, cmd.InputUnit); err != nil {
		return out, err
	}
	expected := append([]app.OperationMaterialUse(nil), p.Materials...)
	if p.PredecessorRequired {
		if _, err = app.AllocateOperationHandoff(p.PredecessorBatches, cmd.ActualInputQty, p.InputUnit); err != nil {
			return out, err
		}
	} else if p.RecipeRatioPct > 0 {
		expected, err = app.ResolveBOMStartMaterialUses(expected, cmd.ActualInputQty, p.InputUnit)
		if err != nil {
			return out, err
		}
	} else {
		if p.InputQty <= 0 {
			return out, fmt.Errorf("本批计划数量无效")
		}
		for i := range expected {
			expected[i].Qty = operationLedgerQuantity(expected[i].Qty*cmd.ActualInputQty/p.InputQty, expected[i].Unit)
		}
	}
	if err = app.ValidateStartedInputMaterials(expected, cmd.Materials, cmd.DifferenceReason); err != nil {
		return out, err
	}
	if !p.PredecessorRequired {
		if err = app.ValidateStartedInputTotals(cmd.Materials, cmd.ActualInputQty, cmd.InputUnit, p.RecipeRatioPct > 0); err != nil {
			return out, err
		}
	}
	if !p.PredecessorRequired && len(cmd.Materials) == 0 {
		return out, fmt.Errorf("冻结 BOM 没有投入物料")
	}
	if wo.RunningItemID <= 0 {
		started, e := r.startWorkOrderTx(ctx, tx, app.WorkOrderStartCommand{ID: wo.ID, Operator: cmd.Operator}, false, true)
		if e != nil {
			return out, e
		}
		wo = started.WorkOrder
	}
	if p.BomRefreshPending {
		cmd.Materials, err = r.refreshRoastingBOMBeforeFirstConsumptionTx(ctx, tx, wo, p, cmd.Materials, cmd.Operator, cmd.RequestID)
		if err != nil {
			return out, err
		}
	} else if !p.PredecessorRequired && p.RecipeRatioPct > 0 {
		if err = r.ensureRoastingBatchMaterialReservationsTx(ctx, tx, wo, p, cmd.Materials, cmd.Operator, cmd.RequestID); err != nil {
			return out, err
		}
	}
	payload, err := json.Marshal(cmd)
	if err != nil {
		return out, err
	}
	var eventID int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_operation_events(work_order_id,job_card_id,kind,request_id,expected_version,operator,input_qty,input_unit,output_unit,warehouse,payload) VALUES($1,$2,'start',$3,$4,$5,$6,$7,$8,$9,$10::jsonb) RETURNING id`, r.schema), wo.ID, cmd.ID, cmd.RequestID, cmd.ExpectedVersion, cmd.Operator, cmd.ActualInputQty, cmd.InputUnit, p.OutputUnit, p.Warehouse, string(payload)).Scan(&eventID)
	if err != nil {
		return out, err
	}
	if p.PredecessorRequired {
		allocations, e := app.AllocateOperationHandoff(p.PredecessorBatches, cmd.ActualInputQty, p.InputUnit)
		if e != nil {
			return out, e
		}
		for _, a := range allocations {
			if _, e = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_started_handoffs(event_id,job_card_id,source_balance_id,qty,remaining_qty) VALUES($1,$2,$3,$4,$4)`, r.schema), eventID, cmd.ID, a.BatchID, a.Qty); e != nil {
				return out, e
			}
		}
	}
	doc, err := r.consumeStartedMaterialsTx(ctx, tx, wo, p, cmd, eventID, "start")
	if err != nil {
		return out, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_operation_events SET status='posted',material_cost=$2 WHERE id=$1`, r.schema), eventID, doc.TotalCost); err != nil {
		return out, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET status='running',started_at=now(),resumed_at=now(),paused_at=NULL,metrics_json=metrics_json-'timer_elapsed_seconds',operator=$2,assigned_employee_id=$3,assigned_to=$2,assigned_employee_name=$2 WHERE id=$1`, r.schema), cmd.ID, cmd.Operator, cmd.EmployeeID); err != nil {
		return out, err
	}
	if err = infra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "job_card", &cmd.ID, "start_consumption", nil, nil, nil, infra.AuditMeta{"event_id": eventID, "stock_entry_id": doc.ID, "materials": cmd.Materials, "input_qty": cmd.ActualInputQty}); err != nil {
		return out, err
	}
	out.JobCard, err = loadJobCardRowTx(ctx, tx, r.schema, cmd.ID)
	if err != nil {
		return out, err
	}
	out.WorkOrder, err = loadWorkOrderExecutionRowTx(ctx, tx, r.schema, wo.ID)
	return out, err
}

func (r Repository) allocateStartedInputTx(ctx context.Context, tx pgx.Tx, card, event int64, qty float64) (float64, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id,remaining_qty::float8,remaining_cost::float8 FROM %s.production_started_inputs WHERE job_card_id=$1 AND status='posted' AND remaining_qty>0 ORDER BY id FOR UPDATE`, r.schema), card)
	if err != nil {
		return 0, err
	}
	balances := []app.StartedInputBalance{}
	for rows.Next() {
		var b app.StartedInputBalance
		if err = rows.Scan(&b.ID, &b.RemainingQty, &b.RemainingCost); err != nil {
			rows.Close()
			return 0, err
		}
		balances = append(balances, b)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	allocations, err := app.AllocateStartedInputCost(balances, qty)
	if err != nil {
		return 0, err
	}
	cost := 0.0
	for _, a := range allocations {
		if _, err = tx.Exec(ctx, fmt.Sprintf(`WITH amounts AS (SELECT c.id,CASE WHEN b.remaining_qty=$2 THEN c.remaining_qty ELSE trunc(c.remaining_qty*$2/b.remaining_qty) END AS qty,CASE WHEN b.remaining_qty=$2 THEN c.remaining_cost ELSE round(c.remaining_cost*trunc(c.remaining_qty*$2/b.remaining_qty)/c.remaining_qty,6) END AS cost FROM %[1]s.production_started_input_components c JOIN %[1]s.production_started_inputs b ON b.id=c.input_id WHERE c.input_id=$1 AND c.remaining_qty>0), saved AS (INSERT INTO %[1]s.production_started_input_component_allocations(component_id,event_id,qty,cost) SELECT id,$3,qty,cost FROM amounts WHERE qty>0 RETURNING component_id,qty,cost) UPDATE %[1]s.production_started_input_components c SET remaining_qty=c.remaining_qty-s.qty,remaining_cost=c.remaining_cost-s.cost FROM saved s WHERE c.id=s.component_id`, r.schema), a.ID, a.Qty, event); err != nil {
			return 0, err
		}
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(a.cost),0)::float8 FROM %[1]s.production_started_input_component_allocations a JOIN %[1]s.production_started_input_components c ON c.id=a.component_id WHERE c.input_id=$1 AND a.event_id=$2`, r.schema), a.ID, event).Scan(&a.Cost); err != nil {
			return 0, err
		}

		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_started_input_allocations(input_id,event_id,qty,cost) VALUES($1,$2,$3,$4)`, r.schema), a.ID, event, a.Qty, a.Cost); err != nil {
			return 0, err
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_started_inputs SET remaining_qty=remaining_qty-$2,remaining_cost=remaining_cost-$3 WHERE id=$1`, r.schema), a.ID, a.Qty, a.Cost); err != nil {
			return 0, err
		}
		cost += a.Cost
	}
	return cost, nil
}

func (r Repository) WorkOrderConsumptionPolicies(ctx context.Context, ids []int64) (map[int64]string, error) {
	out := map[int64]string{}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT id,material_consumption_policy FROM %s.work_orders WHERE id=ANY($1::bigint[])`, r.schema), ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var policy string
		if err = rows.Scan(&id, &policy); err != nil {
			return nil, err
		}
		out[id] = policy
	}
	return out, rows.Err()
}

func (r Repository) consumeStartedMaterialsTx(ctx context.Context, tx pgx.Tx, wo app.WorkOrderRow, p app.OperationCompletionPreview, cmd app.JobCardActionCommand, eventID int64, kind string) (stockapp.StockDocumentDetail, error) {
	var doc stockapp.StockDocumentDetail
	var err error
	stockCmd := stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialConsumption, WorkOrderID: wo.ID, JobCardID: cmd.ID, RunningItemID: wo.RunningItemID, SourceType: "operation_" + kind, SourceID: eventID, Operator: cmd.Operator, IdempotencyKey: fmt.Sprintf("start:%d:consume", eventID), Note: "确认本批开工用料；来源工单 " + wo.WorkOrderNo}
	for _, m := range cmd.Materials {
		if m.Qty <= 0 {
			continue
		}
		item := stockapp.StockDocumentItemCommand{MaterialID: m.MaterialID, ProductID: m.ProductID, BomSpecID: m.BomSpecID, BomVariantID: m.BomVariantID, SpecG: m.SpecG, ItemName: m.Name, InventoryUnit: m.Unit, FromWarehouse: m.Warehouse, OwnerCustomerID: m.OwnerCustomerID, ItemType: "material"}
		if m.ProductID > 0 {
			item.ItemType = "finished_product"
		}
		if factor := app.OperationUnitFactor(m.Unit); factor > 0 {
			item.QtyG = int64(math.Round(m.Qty * factor))
		} else {
			item.QtyUnits = int64(m.Qty)
		}
		stockCmd.Items = append(stockCmd.Items, item)
	}
	if len(stockCmd.Items) > 0 {
		doc, err = stockrepo.NewRepository(r.pool, r.schema).ConsumeOperationMaterialsTx(ctx, tx, stockCmd)
		if err != nil {
			return doc, err
		}
		if err = r.recordOperationConsumptionLogsTx(ctx, tx, wo, doc.ID, cmd.Operator); err != nil {
			return doc, err
		}
	}
	if !p.PredecessorRequired {
		materials, _ := json.Marshal(cmd.Materials)
		var inputID int64
		err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_started_inputs(work_order_id,job_card_id,request_id,input_qty,remaining_qty,input_unit,material_cost,remaining_cost,stock_entry_id,material_snapshot,operator,event_id) VALUES($1,$2,$3,$4,$4,$5,$6,$6,$7,$8::jsonb,$9,$10) RETURNING id`, r.schema), wo.ID, cmd.ID, cmd.RequestID, cmd.ActualInputQty, cmd.InputUnit, doc.TotalCost, doc.ID, string(materials), cmd.Operator, eventID).Scan(&inputID)
		if err != nil {
			return doc, err
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`WITH pieces AS (
 SELECT i.*,a.id AS allocation_id,a.material_batch_id,a.stock_batch_id,a.unit_cost AS batch_unit_cost,
 CASE WHEN i.qty_g>0 THEN a.qty_g ELSE a.qty_units END AS take_qty,
 CASE WHEN i.qty_g>0 THEN i.qty_g ELSE i.qty_units END AS item_qty,
 row_number() OVER(PARTITION BY i.id ORDER BY a.id DESC) AS last_piece
 FROM %[1]s.stock_entry_items i JOIN %[1]s.stock_entry_batch_allocations a ON a.stock_entry_item_id=i.id WHERE i.stock_entry_id=$2
 ), costs AS (SELECT *,round(take_qty*batch_unit_cost/CASE WHEN qty_g>0 THEN 1000 ELSE 1 END,6) AS part_cost FROM pieces), amounts AS (
 SELECT *,CASE WHEN last_piece=1 THEN total_cost-COALESCE(SUM(part_cost) FILTER(WHERE last_piece<>1) OVER(PARTITION BY id),0) ELSE part_cost END AS applied_cost FROM costs)
 INSERT INTO %[1]s.production_started_input_components(input_id,component_key,component_type,component_id,bom_spec_id,bom_variant_id,spec_g,stock_entry_item_id,material_batch_id,stock_batch_id,warehouse,owner_customer_id,qty,remaining_qty,unit,cost,remaining_cost)
 SELECT $1,CASE WHEN material_id>0 THEN 'material:'||material_id||':0:0' ELSE 'product:'||product_id||':'||bom_spec_id||':'||spec_g END,CASE WHEN material_id>0 THEN 'material' ELSE 'product' END,CASE WHEN material_id>0 THEN material_id ELSE product_id END,bom_spec_id,bom_variant_id,spec_g,id,material_batch_id,stock_batch_id,from_warehouse,owner_customer_id,take_qty,take_qty,CASE WHEN qty_g>0 THEN 'g' ELSE inventory_unit END,applied_cost,applied_cost FROM amounts`, r.schema), inputID, doc.ID); err != nil {
			return doc, err
		}
	}

	return doc, nil
}
