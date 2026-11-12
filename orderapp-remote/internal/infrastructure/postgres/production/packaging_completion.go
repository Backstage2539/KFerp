package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/production"
	stockapp "orderapp/internal/application/stock"
	infra "orderapp/internal/infrastructure/postgres"
	stockrepo "orderapp/internal/infrastructure/postgres/stock"
)

type packagingReturnedComponent struct {
	id, inputID, itemID, specID, variantID, specG, owner, materialBatch, stockBatch int64
	key, kind, warehouse, unit, code, name                                          string
	quantity                                                                        float64
}

// completePackagingFinalInputTx handles each locked input and component batch independently.
// All allocation, return, transfer and audit operations share the caller's tx.
func (r Repository) completePackagingFinalInputTx(ctx context.Context, tx pgx.Tx, p app.OperationCompletionPreview, wo app.WorkOrderRow, cmd app.JobCardActionCommand, eventID int64) (float64, error) {
	if (cmd.RemainderMode != "plan_transfer" && cmd.ActualRemainderG == nil) || cmd.FinishOperation == nil || cmd.ActualOutputQty > p.PlannedQty || math.Abs(cmd.ActualInputQty-cmd.ActualOutputQty) > 1e-8 || (*cmd.FinishOperation != (math.Abs(cmd.ActualOutputQty-p.PlannedQty) < 1e-8)) {
		return 0, fmt.Errorf("请按实际完成件数报工并称重确认余料")
	}
	if p.PredecessorRequired || p.MaterialConsumptionPolicy != "bom_start_v1" {
		return 0, fmt.Errorf("请核对半成品工单及包装投入配置")
	}
	bean, err := app.ResolvePackagingWeightComponent(p.Materials)
	if err != nil {
		return 0, err
	}
	if p.TotalPlannedQty <= 0 {
		return 0, fmt.Errorf("包装计划数量无效")
	}
	inputs, err := r.startedInputsTx(ctx, tx, cmd.ID)
	if err != nil {
		return 0, err
	}
	if len(inputs) == 0 || p.StartedInputQty+1e-8 < cmd.ActualInputQty {
		return 0, fmt.Errorf("当前包装投入来源无法安全结算，请联系管理员")
	}
	inputIDs := make([]int64, 0, len(inputs))
	for _, in := range inputs {
		inputIDs = append(inputIDs, in.ID)
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT c.id,c.input_id,c.component_key,c.component_type,c.component_id,c.bom_spec_id,c.bom_variant_id,c.spec_g,c.warehouse,c.owner_customer_id,c.unit,c.remaining_qty::float8,COALESCE(m.batch_code,s.batch_code,''),c.material_batch_id,c.stock_batch_id,i.item_name FROM %[1]s.production_started_input_components c JOIN %[1]s.stock_entry_items i ON i.id=c.stock_entry_item_id LEFT JOIN %[1]s.material_batches m ON m.id=c.material_batch_id LEFT JOIN %[1]s.stock_batches s ON s.id=c.stock_batch_id WHERE c.input_id=ANY($1::bigint[]) ORDER BY c.input_id,c.id FOR UPDATE OF c`, r.schema), inputIDs)
	if err != nil {
		return 0, err
	}
	var components []packagingReturnedComponent
	availableG := 0.0
	for rows.Next() {
		var c packagingReturnedComponent
		if err = rows.Scan(&c.id, &c.inputID, &c.key, &c.kind, &c.itemID, &c.specID, &c.variantID, &c.specG, &c.warehouse, &c.owner, &c.unit, &c.quantity, &c.code, &c.materialBatch, &c.stockBatch, &c.name); err != nil {
			rows.Close()
			return 0, err
		}
		if c.key == bean.Key {
			availableG += c.quantity
		}
		components = append(components, c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	settlement := app.PackagingRemainderResult{}
	if cmd.RemainderMode == "plan_transfer" {
		if cmd.ActualRemainderG != nil || len(cmd.SelectedUnissuedRemainders) > 0 || cmd.RemainderWarehouse != "" {
			return 0, fmt.Errorf("统一处理模式不能提交工单余料")
		}
		settlement.UsedG, err = app.DeferredPackagingUsage(bean.TotalQty*app.OperationUnitFactor(bean.Unit), p.CompletedOutputQty, cmd.ActualOutputQty, p.TotalPlannedQty, availableG)
	} else {
		settlement, err = app.CalculatePackagingRemainder(app.PackagingRemainderInput{AvailableG: availableG, CompletedQty: cmd.ActualOutputQty, RemainingTargetQty: p.PlannedQty, StandardGPerPiece: bean.TotalQty * app.OperationUnitFactor(bean.Unit) / p.TotalPlannedQty, ActualRemainderG: *cmd.ActualRemainderG, DifferenceReason: cmd.DifferenceReason})
	}
	if err != nil {
		return 0, err
	}
	expected := map[string]app.OperationMaterialUse{}
	for _, m := range p.Materials {
		if _, ok := expected[m.Key]; ok {
			return 0, fmt.Errorf("冻结 BOM 组件身份重复，请核对配置")
		}
		expected[m.Key] = m
	}
	for _, c := range components {
		m, ok := expected[c.key]
		kind := "material"
		itemID := m.MaterialID
		if m.ProductID > 0 {
			kind = "product"
			itemID = m.ProductID
		}
		if !ok || c.code == "" || c.kind != kind || c.itemID != itemID || c.specID != m.BomSpecID || c.variantID != m.BomVariantID || c.specG != m.SpecG || c.owner != m.OwnerCustomerID || c.warehouse != m.Warehouse {
			return 0, fmt.Errorf("包装组件批次、规格、货主或来源仓不一致，请核对配置")
		}
	}
	remainingPieces := cmd.ActualInputQty
	requests := make([]packagingInputComponentRequest, len(inputs))
	for i, in := range inputs {
		count := math.Min(in.RemainingQty, remainingPieces)
		requests[i] = packagingInputComponentRequest{InputID: in.ID, InputQty: count, Components: map[string]float64{}}
		remainingPieces -= count
	}
	if remainingPieces > 1e-8 {
		return 0, fmt.Errorf("包装投入件数不足，请核对")
	}
	for key, m := range expected {
		wanted := settlement.UsedG
		if key != bean.Key {
			total := m.TotalQty * app.OperationUnitFactor(m.Unit)
			if app.OperationUnitFactor(m.Unit) == 0 {
				total = m.TotalQty
			}
			if math.Abs(total-math.Round(total)) > 1e-8 {
				return 0, fmt.Errorf("包装组件用量无法按冻结规格换算为整数")
			}
			wanted = float64(taskQuantitySlice(int64(math.Round(total)), p.CompletedOutputQty, cmd.ActualOutputQty, p.TotalPlannedQty))
		}
		for i, in := range inputs {
			available := 0.0
			for _, c := range components {
				if c.inputID == in.ID && c.key == key {
					available += c.quantity
				}
			}
			take := math.Min(available, wanted)
			if take > 0 {
				requests[i].Components[key] = take
				wanted -= take
			}
		}
		if wanted > 1e-8 {
			return 0, fmt.Errorf("包装组件余额不足，请核对来源批次")
		}
	}
	active := requests[:0]
	for _, request := range requests {
		if request.InputQty > 0 || len(request.Components) > 0 {
			active = append(active, request)
		}
	}
	requests = active
	cost, err := r.allocatePackagingComponentsTx(ctx, tx, cmd.ID, eventID, requests)
	if err != nil {
		return 0, err
	}
	if cmd.RemainderMode == "plan_transfer" {
		return cost, nil
	}
	if !*cmd.FinishOperation {
		if len(cmd.SelectedUnissuedRemainders) > 0 {
			return 0, fmt.Errorf("请在工单完成后选择需要转移的共享库存")
		}
		return cost, nil
	}
	rows, err = tx.Query(ctx, fmt.Sprintf(`SELECT id,remaining_qty::float8 FROM %s.production_started_input_components WHERE input_id=ANY($1::bigint[]) AND remaining_qty>0 ORDER BY id FOR UPDATE`, r.schema), inputIDs)
	if err != nil {
		return 0, err
	}
	leftovers := map[int64]float64{}
	for rows.Next() {
		var id int64
		var q float64
		if err = rows.Scan(&id, &q); err != nil {
			rows.Close()
			return 0, err
		}
		leftovers[id] = q
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	if len(leftovers) == 0 {
		if err = r.transferPackagingUnissuedTx(ctx, tx, p, cmd, eventID, bean); err != nil {
			return 0, err
		}
		return cost, nil
	}
	raw, _ := json.Marshal(cmd)
	var returnEvent int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_operation_events(work_order_id,job_card_id,kind,request_id,expected_version,operator,input_unit,output_unit,warehouse,difference_reason,payload) VALUES($1,$2,'material_return',$3,$4,$5,$6,$7,$8,$9,$10::jsonb) RETURNING id`, r.schema), wo.ID, cmd.ID, cmd.RequestID+":remainder", p.Version, cmd.Operator, p.InputUnit, p.OutputUnit, p.Warehouse, "包装未用余料退回", string(raw)).Scan(&returnEvent); err != nil {
		return 0, err
	}
	target := strings.TrimSpace(cmd.RemainderWarehouse)
	if target == "" {
		target = "wip"
	}
	returnCost := 0.0
	returnCostByInput := map[int64]float64{}
	for _, c := range components {
		q := leftovers[c.id]
		if q <= 0 {
			continue
		}
		amount, e := stockrepo.NewRepository(r.pool, r.schema).ReturnStartedComponentTx(ctx, tx, c.id, returnEvent, q, cmd.Operator)
		if e != nil {
			return 0, e
		}
		returnCost += amount
		returnCostByInput[c.inputID] += amount
		if c.warehouse != target {
			item := stockapp.StockDocumentItemCommand{ItemType: c.kind, ItemName: c.name, BomSpecID: c.specID, BomVariantID: c.variantID, SpecG: c.specG, InventoryUnit: expected[c.key].Unit, FromWarehouse: c.warehouse, ToWarehouse: target, OwnerCustomerID: c.owner, BatchCode: c.code}
			if c.kind == "material" {
				item.MaterialID = c.itemID
			} else {
				item.ItemType = "finished_product"
				item.ProductID = c.itemID
			}
			if c.unit == "g" {
				item.QtyG = int64(q)
			} else {
				item.QtyUnits = int64(q)
			}
			_, e = stockrepo.NewRepository(r.pool, r.schema).CreateAndSubmitStockDocumentTx(ctx, tx, stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialTransfer, SourceType: "packaging_remainder", SourceID: returnEvent, Operator: cmd.Operator, IdempotencyKey: fmt.Sprintf("packaging-return:%d:%d", returnEvent, c.id), Items: []stockapp.StockDocumentItemCommand{item}})
			if e != nil {
				return 0, e
			}
		}
	}
	for inputID, amount := range returnCostByInput {
		tag, e := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_started_inputs SET remaining_cost=remaining_cost-$2 WHERE id=$1 AND remaining_cost+0.000001>=$2`, r.schema), inputID, amount)
		if e != nil {
			return 0, e
		}
		if tag.RowsAffected() != 1 {
			return 0, fmt.Errorf("包装余料成本余额不一致")
		}
	}
	if err = r.recordStartedInputReturnLogsTx(ctx, tx, wo, returnEvent, cmd.Operator); err != nil {
		return 0, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_operation_events SET status='posted',material_cost=$2 WHERE id=$1`, r.schema), returnEvent, -returnCost); err != nil {
		return 0, err
	}
	if err = infra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "job_card", &cmd.ID, "packaging_remainder", nil, nil, nil, infra.AuditMeta{"event_id": returnEvent, "remainder_g": *cmd.ActualRemainderG, "warehouse": target}); err != nil {
		return 0, err
	}
	if err = r.transferPackagingUnissuedTx(ctx, tx, p, cmd, eventID, bean); err != nil {
		return 0, err
	}
	return cost, nil
}
