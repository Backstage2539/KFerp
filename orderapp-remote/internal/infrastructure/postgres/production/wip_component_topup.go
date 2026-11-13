package production

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/production"
	stockapp "orderapp/internal/application/stock"
	infra "orderapp/internal/infrastructure/postgres"
	"orderapp/internal/infrastructure/postgres/customerstock"
	stockrepo "orderapp/internal/infrastructure/postgres/stock"
)

type wipTopUpState struct {
	preview    app.WIPTopUpPreview
	completion app.OperationCompletionPreview
	work       app.WorkOrderRow
	materials  map[string]app.OperationMaterialUse
	inputID    int64
}

// Mirror completion's cumulative allocation: packaging weight rounds, other
// components use the existing ceiling slices. Values are in material units.
func wipExpectedSlice(m app.OperationMaterialUse, packagingBean bool, before, current, denominator float64) (float64, error) {
	factor := app.OperationUnitFactor(m.Unit)
	if factor <= 0 {
		factor = 1
	}
	total := m.TotalQty * factor
	if denominator <= 0 || before < 0 || current < 0 || math.IsNaN(total) || math.IsInf(total, 0) || total < 0 || math.Abs(total-math.Round(total)) > 1e-6 {
		return 0, fmt.Errorf("冻结物料数量或单位无效，请先核对")
	}
	if packagingBean {
		grams, err := app.DeferredPackagingUsage(total, before, math.Min(current, math.Max(0, denominator-before)), denominator, math.MaxFloat64)
		if current == 0 || before >= denominator {
			return 0, nil
		}
		if err != nil {
			return 0, err
		}
		return grams / factor, nil
	}
	return float64(taskQuantitySlice(int64(math.Round(total)), before, current, denominator)) / factor, nil
}

func (r Repository) PreviewWIPTopUp(ctx context.Context, id int64) (app.WIPTopUpPreview, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return app.WIPTopUpPreview{}, err
	}
	defer tx.Rollback(ctx)
	state, err := r.wipTopUpSnapshotTx(ctx, tx, id, false)
	if err != nil {
		return app.WIPTopUpPreview{}, err
	}
	return state.preview, tx.Commit(ctx)
}

func (r Repository) wipTopUpSnapshotTx(ctx context.Context, tx pgx.Tx, id int64, lock bool) (wipTopUpState, error) {
	state := wipTopUpState{preview: app.WIPTopUpPreview{Items: []app.WIPTopUpItem{}}, materials: map[string]app.OperationMaterialUse{}}
	p, wo, err := r.operationCompletionContextTx(ctx, tx, id, lock)
	if err != nil {
		return state, err
	}
	state.completion = p
	state.work = wo
	var status string
	var plannedInput float64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT status,planned_input_qty::float8 FROM %s.job_cards WHERE id=$1`, r.schema), id).Scan(&status, &plannedInput); err != nil {
		return state, err
	}
	if p.MaterialConsumptionPolicy != "bom_start_v1" || p.PredecessorRequired || (status != "running" && status != "paused") || wo.Status == "completed" || wo.Status == "cancelled" {
		state.preview.BlockedReason = "仅支持已开工、按冻结 BOM 直接领料的任务；承接前序产出的任务请先核对来源"
		return state, nil
	}
	if p.BlockedReason != "" {
		state.preview.BlockedReason = p.BlockedReason
		return state, nil
	}
	if p.StartedInputQty <= 0 {
		state.preview.BlockedReason = "该任务没有可追加组件的已确认投入"
		return state, nil
	}
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_started_inputs WHERE job_card_id=$1 AND status='posted' AND remaining_qty>0 ORDER BY id LIMIT 1`, r.schema), id).Scan(&state.inputID); err != nil {
		return state, fmt.Errorf("未找到可追加的已确认投入")
	}
	expected := append([]app.OperationMaterialUse(nil), p.Materials...)
	if p.RecipeRatioPct > 0 && app.OperationUnitFactor(p.InputUnit) > 0 {
		expected, err = app.ResolveBOMStartMaterialUses(expected, p.StartedInputQty, p.InputUnit)
		if err != nil {
			return state, err
		}
	} else {
		denominator := p.TotalPlannedQty
		before := p.CompletedOutputQty
		if app.OperationUnitFactor(p.InputUnit) > 0 {
			denominator = plannedInput / app.OperationUnitFactor(p.InputUnit)
			if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(input_qty),0)::float8 FROM %s.production_operation_events WHERE job_card_id=$1 AND kind='complete' AND status='posted'`, r.schema), id).Scan(&before); err != nil {
				return state, err
			}
		}
		if denominator <= 0 {
			state.preview.BlockedReason = "冻结计划投入数量缺失，请先核对"
			return state, nil
		}
		beanKey := ""
		if p.Execution.ReportingType == "piece" {
			bean, e := app.ResolvePackagingWeightComponent(p.Materials)
			if e != nil {
				state.preview.BlockedReason = "冻结包装重量组件无法确定，请先核对"
				return state, nil
			}
			beanKey = bean.Key
		}
		for i := range expected {
			expected[i].Qty, err = wipExpectedSlice(expected[i], expected[i].Key == beanKey, before, p.StartedInputQty, denominator)
			if err != nil {
				return state, err
			}
		}
	}
	for _, m := range expected {
		if m.Key == "" || m.Warehouse != "wip" {
			state.preview.BlockedReason = "冻结用料来源不在 WIP 仓，请先核对来源"
			return state, nil
		}
		if _, exists := state.materials[m.Key]; exists {
			return state, fmt.Errorf("冻结组件身份重复，请先核对")
		}
		state.materials[m.Key] = m
		var raw float64
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(c.remaining_qty),0)::float8 FROM %[1]s.production_started_input_components c JOIN %[1]s.production_started_inputs i ON i.id=c.input_id WHERE i.job_card_id=$1 AND i.status='posted' AND c.component_key=$2 AND c.owner_customer_id=$3 AND c.warehouse=$4 AND c.bom_spec_id=$5 AND c.bom_variant_id=$6 AND c.spec_g=$7`, r.schema), id, m.Key, m.OwnerCustomerID, m.Warehouse, m.BomSpecID, m.BomVariantID, m.SpecG).Scan(&raw); err != nil {
			return state, err
		}
		issued := raw
		if factor := app.OperationUnitFactor(m.Unit); factor > 0 {
			issued = raw / factor
		}
		shortage := operationLedgerQuantity(math.Max(0, m.Qty-issued), m.Unit)
		if shortage <= 0 {
			continue
		}
		item := app.WIPTopUpItem{Key: m.Key, Name: m.Name, Unit: m.Unit, RequiredQty: m.Qty, IssuedQty: issued, ShortageQty: shortage, Candidates: []app.WIPTopUpCandidate{}}
		batches, e := startedMaterialBatchesTx(ctx, tx, r.schema, wo.ID, m)
		if e != nil {
			return state, e
		}
		for _, batch := range batches {
			var quality string
			var cost float64
			var unavailable bool
			query := fmt.Sprintf(`SELECT COALESCE(m.quality_status,'unchecked'),m.unit_cost::float8,COALESCE(m.cost_unavailable,false) OR EXISTS(SELECT 1 FROM %[1]s.stock_batches b WHERE b.item_type='material' AND b.item_id=m.material_id AND b.batch_code=m.batch_code AND b.owner_customer_id=m.owner_customer_id AND b.cost_unavailable) FROM %[1]s.material_batches m WHERE m.id=$1`, r.schema)
			if m.ProductID > 0 {
				query = fmt.Sprintf(`SELECT COALESCE(quality_status,'unchecked'),unit_cost::float8,COALESCE(cost_unavailable,false) FROM %s.stock_batches WHERE id=$1`, r.schema)
			}
			if e = tx.QueryRow(ctx, query, batch.BatchID).Scan(&quality, &cost, &unavailable); e != nil {
				return state, e
			}
			item.Candidates = append(item.Candidates, app.WIPTopUpCandidate{BatchCode: batch.BatchCode, Warehouse: m.Warehouse, OwnerCustomerID: m.OwnerCustomerID, AvailableQty: batch.AvailableQty, QualityStatus: quality, UnitCost: cost, CostUnavailable: unavailable})
		}
		state.preview.Items = append(state.preview.Items, item)
	}
	state.preview.Supported = true
	raw, _ := json.Marshal([]any{p.Version, state.preview.Items, state.inputID, p.StartedInputQty})
	state.preview.Version = fmt.Sprintf("%x", sha256.Sum256(raw))
	return state, nil
}

func (r Repository) ConfirmWIPTopUp(ctx context.Context, c app.WIPTopUpCommand) (app.JobCardActionResult, error) {
	return retryOperationTransaction(ctx, func() (app.JobCardActionResult, error) {
		var out app.JobCardActionResult
		if c.ID <= 0 || c.EmployeeID <= 0 || strings.TrimSpace(c.Operator) == "" || strings.TrimSpace(c.RequestID) == "" || strings.TrimSpace(c.ExpectedVersion) == "" || len(c.Materials) == 0 {
			return out, fmt.Errorf("请确认员工、批次和任务版本")
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
		var work, run int64
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT w.id,w.running_item_id FROM %[1]s.job_cards j JOIN %[1]s.work_orders w ON w.id=j.work_order_id WHERE j.id=$1`, r.schema), c.ID).Scan(&work, &run); err != nil {
			return out, err
		}
		if run > 0 {
			if err = customerstock.LockRunningFulfillmentCustomersTx(ctx, tx, r.schema, run); err != nil {
				return out, err
			}
		}
		if err = lockReceiptWorkOrderTx(ctx, tx, r.schema, work); err != nil {
			return out, err
		}
		rid, replay, err := requestProductionKeyTx(ctx, tx, r.schema, "wip_component_topup", c.Operator, c.RequestID, c)
		if err != nil {
			return out, err
		}
		if rid > 0 {
			err = json.Unmarshal(replay, &out)
			return out, err
		}
		state, err := r.wipTopUpSnapshotTx(ctx, tx, c.ID, true)
		if err != nil {
			return out, err
		}
		if !state.preview.Supported {
			return out, fmt.Errorf("%s", state.preview.BlockedReason)
		}
		if state.preview.Version != c.ExpectedVersion {
			return out, fmt.Errorf("任务或库存已变化，请重新核对")
		}
		var station, status string
		var stationID int64
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT status,workstation,workstation_id FROM %s.job_cards WHERE id=$1 FOR UPDATE`, r.schema), c.ID).Scan(&status, &station, &stationID); err != nil {
			return out, err
		}
		owner, roster, err := resolveProductionWorkstationOwnerTx(ctx, tx, r.schema, stationID, station, productionWorkDate())
		if err != nil {
			return out, err
		}
		if roster && owner.EmployeeID != c.EmployeeID {
			return out, fmt.Errorf("请由今日工位负责人使用物料")
		}
		var blocked bool
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM (SELECT DISTINCT ON(job_card_id,scope,reference_type,reference_no) result FROM %s.quality_inspections WHERE (work_order_id=$1 AND job_card_id=0) OR job_card_id=$2 ORDER BY job_card_id,scope,reference_type,reference_no,created_at DESC,id DESC) latest WHERE lower(trim(result)) IN('hold','reject','failed','fail','blocked'))`, r.schema), work, c.ID).Scan(&blocked); err != nil {
			return out, err
		}
		if blocked {
			return out, fmt.Errorf("本任务被质检冻结，请先处理质检记录")
		}
		items := map[string]app.WIPTopUpItem{}
		for _, item := range state.preview.Items {
			items[item.Key] = item
		}
		used := map[string]float64{}
		seen := map[string]bool{}
		var stockItems []stockapp.StockDocumentItemCommand
		var display []app.OperationMaterialUse
		for _, sel := range c.Materials {
			item, ok := items[sel.Key]
			m, identity := state.materials[sel.Key]
			if !ok || !identity || sel.BatchCode == "" || sel.Warehouse != "wip" || sel.OwnerCustomerID != m.OwnerCustomerID || math.IsNaN(sel.Qty) || math.IsInf(sel.Qty, 0) || sel.Qty <= 0 {
				return out, fmt.Errorf("补料来源或数量无效，请重新核对")
			}
			key := sel.Key + "\x00" + sel.BatchCode
			if seen[key] {
				return out, fmt.Errorf("同一物料批次不能重复选择")
			}
			seen[key] = true
			if err = app.ValidateOperationQuantity(sel.Qty, m.Unit); err != nil {
				return out, err
			}
			found := false
			for _, candidate := range item.Candidates {
				if candidate.BatchCode == sel.BatchCode && candidate.Warehouse == sel.Warehouse && candidate.OwnerCustomerID == sel.OwnerCustomerID && sel.Qty <= candidate.AvailableQty+1e-8 {
					found = true
					break
				}
			}
			if !found {
				return out, fmt.Errorf("所选批次可用量、货主或质检状态已变化，请重新核对")
			}
			used[sel.Key] += sel.Qty
			if used[sel.Key] > item.ShortageQty+1e-8 {
				return out, fmt.Errorf("补料数量不能超过该工单实际缺口")
			}
			entry := stockapp.StockDocumentItemCommand{MaterialID: m.MaterialID, ProductID: m.ProductID, BomSpecID: m.BomSpecID, BomVariantID: m.BomVariantID, SpecG: m.SpecG, ItemName: m.Name, InventoryUnit: m.Unit, FromWarehouse: m.Warehouse, OwnerCustomerID: m.OwnerCustomerID, ItemType: "material", BatchCode: sel.BatchCode}
			if m.ProductID > 0 {
				entry.ItemType = "finished_product"
			}
			if factor := app.OperationUnitFactor(m.Unit); factor > 0 {
				grams := sel.Qty * factor
				if math.Abs(grams-math.Round(grams)) > 1e-8 || grams < 1 {
					return out, fmt.Errorf("重量须精确到整克")
				}
				entry.QtyG = int64(math.Round(grams))
			} else {
				entry.QtyUnits = int64(sel.Qty)
			}
			stockItems = append(stockItems, entry)
			m.Qty = sel.Qty
			display = append(display, m)
		}
		payload, _ := json.Marshal(map[string]any{"request_id": c.RequestID, "materials": display, "selected_batches": c.Materials, "component_only": true})
		var eventID int64
		if err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_operation_events(work_order_id,job_card_id,kind,request_id,expected_version,operator,input_unit,output_unit,warehouse,difference_reason,payload) VALUES($1,$2,'material_add',$3,$4,$5,$6,$7,'wip','使用 WIP 可用物料',$8::jsonb) RETURNING id`, r.schema), work, c.ID, c.RequestID, c.ExpectedVersion, c.Operator, state.completion.InputUnit, state.completion.OutputUnit, string(payload)).Scan(&eventID); err != nil {
			return out, err
		}
		cmd := stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialConsumption, WorkOrderID: work, JobCardID: c.ID, RunningItemID: run, SourceType: "operation_material_add", SourceID: eventID, Operator: c.Operator, IdempotencyKey: fmt.Sprintf("wip-topup:%d", eventID), Note: "工位直接使用 WIP 可用物料", Items: stockItems}
		doc, err := stockrepo.NewRepository(r.pool, r.schema).ConsumeOperationMaterialsTx(ctx, tx, cmd)
		if err != nil {
			return out, err
		}
		if err = r.recordOperationConsumptionLogsTx(ctx, tx, state.work, doc.ID, c.Operator); err != nil {
			return out, err
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`WITH pieces AS (SELECT i.*,a.id AS allocation_id,a.material_batch_id,a.stock_batch_id,a.unit_cost AS batch_unit_cost,CASE WHEN i.qty_g>0 THEN a.qty_g ELSE a.qty_units END AS take_qty,row_number() OVER(PARTITION BY i.id ORDER BY a.id DESC) AS last_piece FROM %[1]s.stock_entry_items i JOIN %[1]s.stock_entry_batch_allocations a ON a.stock_entry_item_id=i.id WHERE i.stock_entry_id=$2), costs AS (SELECT *,round(take_qty*batch_unit_cost/CASE WHEN qty_g>0 THEN 1000 ELSE 1 END,6) AS part_cost FROM pieces), amounts AS (SELECT *,CASE WHEN last_piece=1 THEN total_cost-COALESCE(SUM(part_cost) FILTER(WHERE last_piece<>1) OVER(PARTITION BY id),0) ELSE part_cost END AS applied_cost FROM costs) INSERT INTO %[1]s.production_started_input_components(input_id,component_key,component_type,component_id,bom_spec_id,bom_variant_id,spec_g,stock_entry_item_id,material_batch_id,stock_batch_id,warehouse,owner_customer_id,qty,remaining_qty,unit,cost,remaining_cost) SELECT $1,CASE WHEN material_id>0 THEN 'material:'||material_id||':0:0' ELSE 'product:'||product_id||':'||bom_spec_id||':'||spec_g END,CASE WHEN material_id>0 THEN 'material' ELSE 'product' END,CASE WHEN material_id>0 THEN material_id ELSE product_id END,bom_spec_id,bom_variant_id,spec_g,id,material_batch_id,stock_batch_id,from_warehouse,owner_customer_id,take_qty,take_qty,CASE WHEN qty_g>0 THEN 'g' ELSE inventory_unit END,applied_cost,applied_cost FROM amounts`, r.schema), state.inputID, doc.ID); err != nil {
			return out, err
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_started_inputs SET material_cost=material_cost+$2,remaining_cost=remaining_cost+$2 WHERE id=$1 AND status='posted'`, r.schema), state.inputID, doc.TotalCost); err != nil {
			return out, err
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_operation_events SET status='posted',material_cost=$2 WHERE id=$1`, r.schema), eventID, doc.TotalCost); err != nil {
			return out, err
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET operator=$2 WHERE id=$1`, r.schema), c.ID, c.Operator); err != nil {
			return out, err
		}
		if err = infra.AuditInsertTx(ctx, tx, r.schema, c.Operator, "job_card", &c.ID, "wip_component_topup", nil, nil, nil, infra.AuditMeta{"event_id": eventID, "stock_entry_id": doc.ID, "materials": c.Materials, "material_cost": doc.TotalCost}); err != nil {
			return out, err
		}
		out.JobCard, err = loadJobCardRowTx(ctx, tx, r.schema, c.ID)
		if err != nil {
			return out, err
		}
		out.WorkOrder, err = loadWorkOrderExecutionRowTx(ctx, tx, r.schema, work)
		if err != nil {
			return out, err
		}
		if err = finishProductionKeyTx(ctx, tx, r.schema, "wip_component_topup", c.Operator, c.RequestID, c.ID, out); err != nil {
			return out, err
		}
		return out, tx.Commit(ctx)
	})
}
