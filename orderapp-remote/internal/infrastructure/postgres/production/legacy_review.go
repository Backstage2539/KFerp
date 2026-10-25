package production

import (
	"context"
	"crypto/sha256"
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

func (r Repository) PreviewLegacyReview(ctx context.Context, ids []int64) ([]app.LegacyReview, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if len(ids) == 0 {
		rows, e := tx.Query(ctx, fmt.Sprintf(`SELECT w.id FROM %[1]s.work_orders w WHERE w.status<>'cancelled' AND EXISTS(SELECT 1 FROM %[1]s.job_cards j WHERE j.work_order_id=w.id AND j.status='completed') AND NOT EXISTS(SELECT 1 FROM %[1]s.production_operation_events e WHERE e.work_order_id=w.id AND e.status='posted') AND NOT EXISTS(SELECT 1 FROM %[1]s.job_cards j WHERE j.work_order_id=w.id AND j.status NOT IN('completed','cancelled')) ORDER BY w.id LIMIT 500`, r.schema))
		if e != nil {
			return nil, e
		}
		for rows.Next() {
			var id int64
			if e = rows.Scan(&id); e != nil {
				rows.Close()
				return nil, e
			}
			ids = append(ids, id)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
	}
	if len(ids) > 500 {
		return nil, fmt.Errorf("一次最多核对 500 张工单")
	}
	out := []app.LegacyReview{}
	for _, id := range ids {
		row, _, e := r.legacyReviewContextTx(ctx, tx, id)
		if e != nil {
			return nil, e
		}
		if row.ReceivedQty >= row.ReportedQty && row.ReceivedQty > 0 {
			continue
		}
		out = append(out, row)
	}
	return out, tx.Commit(ctx)
}
func (r Repository) legacyReviewContextTx(ctx context.Context, tx pgx.Tx, id int64) (app.LegacyReview, app.WorkOrderRow, error) {
	task, wo, err := loadReceiptContextTx(ctx, tx, r.schema, id)
	out := app.LegacyReview{FinishedReceiptTask: task, Materials: []app.LegacyReviewMaterial{}}
	if err != nil {
		return out, wo, err
	}
	var unfinished, posted bool
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.job_cards WHERE work_order_id=$1 AND status NOT IN('completed','cancelled')),EXISTS(SELECT 1 FROM %[1]s.production_operation_events WHERE work_order_id=$1 AND status='posted')`, r.schema), id).Scan(&unfinished, &posted); err != nil {
		return out, wo, err
	}
	if unfinished {
		out.ReviewBlockedReason = "本单仍有未完成工序，请先核对执行中的工序"
	}
	if posted {
		out.ReviewBlockedReason = "本单已有自动记账记录，请从生产记录更正"
	}
	if wo.Status == "cancelled" {
		out.ReviewBlockedReason = "工单已取消"
	}
	if task.QualityBlocked {
		out.ReviewBlockedReason = "本单处于质量冻结"
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT r.id,r.component_type,CASE WHEN r.component_id>0 THEN r.component_id ELSE r.material_id END,r.component_bom_spec_id,r.component_bom_variant_id,r.component_spec_g,r.material_name,r.unit,r.source_owner_customer_id,r.required_g,r.required_units,
 GREATEST(r.consumed_g,COALESCE((SELECT SUM(i.qty_g) FROM %[1]s.stock_entry_items i JOIN %[1]s.stock_entries d ON d.id=i.stock_entry_id WHERE d.work_order_id=r.work_order_id AND d.purpose='material_consumption_for_manufacture' AND d.status='submitted' AND i.material_id=r.material_id AND i.product_id=CASE WHEN r.component_type='product' THEN r.component_id ELSE 0 END AND i.owner_customer_id=r.source_owner_customer_id AND (r.component_type<>'product' OR (i.bom_spec_id=r.component_bom_spec_id AND i.spec_g=r.component_spec_g))),0),COALESCE((SELECT SUM(l.deduct_g) FROM %[1]s.material_consumption_logs l WHERE l.running_item_id=$2 AND r.component_type<>'product' AND l.material_id=r.material_id),0))::bigint,
 GREATEST(r.consumed_units,COALESCE((SELECT SUM(i.qty_units) FROM %[1]s.stock_entry_items i JOIN %[1]s.stock_entries d ON d.id=i.stock_entry_id WHERE d.work_order_id=r.work_order_id AND d.purpose='material_consumption_for_manufacture' AND d.status='submitted' AND i.material_id=r.material_id AND i.product_id=CASE WHEN r.component_type='product' THEN r.component_id ELSE 0 END AND i.owner_customer_id=r.source_owner_customer_id AND (r.component_type<>'product' OR (i.bom_spec_id=r.component_bom_spec_id AND i.spec_g=r.component_spec_g))),0),COALESCE((SELECT SUM(l.deduct_units) FROM %[1]s.material_consumption_logs l WHERE l.running_item_id=$2 AND r.component_type<>'product' AND l.material_id=r.material_id),0))::bigint
 FROM %[1]s.work_order_material_reservations r WHERE r.work_order_id=$1 ORDER BY r.id`, r.schema), id, wo.RunningItemID)
	if err != nil {
		return out, wo, err
	}
	for rows.Next() {
		var m app.LegacyReviewMaterial
		var kind string
		var component, g, n, cg, cn int64
		if err = rows.Scan(&m.ReservationID, &kind, &component, &m.BomSpecID, &m.BomVariantID, &m.SpecG, &m.Name, &m.Unit, &m.OwnerCustomerID, &g, &n, &cg, &cn); err != nil {
			rows.Close()
			return out, wo, err
		}
		if kind == "product" {
			m.ProductID = component
		} else {
			kind = "material"
			m.MaterialID = component
		}
		m.Key = operationMaterialKey(kind, component, m.BomSpecID, m.SpecG)
		m.Warehouse = "wip"
		m.WarehouseName = "在制仓"
		m.Qty = float64(n)
		m.ConsumedQty = float64(cn)
		if factor := app.OperationUnitFactor(m.Unit); factor > 0 {
			m.Qty = float64(g) / factor
			m.ConsumedQty = float64(cg) / factor
		}
		out.Materials = append(out.Materials, m)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, wo, err
	}
	var unmatched bool
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.material_consumption_logs l WHERE l.running_item_id=$1 AND NOT EXISTS(SELECT 1 FROM %[1]s.work_order_material_reservations r WHERE r.work_order_id=$2 AND (r.material_id=l.material_id OR (r.component_type='product' AND r.component_id=l.material_id))))`, r.schema), wo.RunningItemID, id).Scan(&unmatched); err != nil {
		return out, wo, err
	}
	if unmatched {
		out.ReviewBlockedReason = "已有耗料流水无法对应冻结物料，请核对来源物料与单据后再补账"
	}
	if len(out.Materials) == 0 {
		out.ReviewBlockedReason = "本单缺少冻结用料记录，需要先核对原始物料归属"
	}
	inventoryVersion, err := r.operationInventoryVersionTx(ctx, tx, id)
	if err != nil {
		return out, wo, err
	}
	raw, _ := json.Marshal([]any{task.ReceiptVersion, out.Materials, inventoryVersion})
	out.Version = fmt.Sprintf("%x", sha256.Sum256(raw))
	return out, wo, nil
}

func (r Repository) SubmitLegacyReview(ctx context.Context, c app.LegacyReviewItem) (app.LegacyReviewResult, error) {
	return retryOperationTransaction(ctx, func() (app.LegacyReviewResult, error) { return r.SubmitLegacyReviewOnce(ctx, c) })
}

func (r Repository) SubmitLegacyReviewOnce(ctx context.Context, c app.LegacyReviewItem) (app.LegacyReviewResult, error) {
	out := app.LegacyReviewResult{WorkOrderID: c.WorkOrderID}
	if c.EmployeeID <= 0 || c.RequestID == "" || c.ExpectedVersion == "" || (!c.RecordedOutputOnly && (c.ActualOutputQty == nil || strings.TrimSpace(c.Reason) == "")) {
		return out, fmt.Errorf("请由已绑定员工填写现场核实的数量、用料和原因")
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	var runID int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT running_item_id FROM %s.work_orders WHERE id=$1`, r.schema), c.WorkOrderID).Scan(&runID); err != nil {
		return out, err
	}
	if runID > 0 {
		if err = customerstock.LockRunningFulfillmentCustomersTx(ctx, tx, r.schema, runID); err != nil {
			return out, err
		}
	}
	if err = lockReceiptWorkOrderTx(ctx, tx, r.schema, c.WorkOrderID); err != nil {
		return out, err
	}
	replayID, replay, err := requestProductionKeyTx(ctx, tx, r.schema, "legacy_reconcile", c.Operator, c.RequestID, c)
	if err != nil {
		return out, err
	}
	if replayID > 0 {
		err = json.Unmarshal(replay, &out)
		return out, err
	}
	review, wo, err := r.legacyReviewContextTx(ctx, tx, c.WorkOrderID)
	if err != nil {
		return out, err
	}
	if review.Version != c.ExpectedVersion {
		return out, fmt.Errorf("旧单或库存已变化，请刷新核对")
	}
	if c.RecordedOutputOnly {
		if review.ReportedQty <= 0 || len(review.CorrectionCards) == 0 {
			return out, fmt.Errorf("本单没有可直接入库的已完成数量")
		}
		quantity := review.ReportedQty
		c.ActualOutputQty = &quantity
		c.Reason = "按已记录的完工数量直接入库"
		c.CardQuantities = make(map[int64]float64, len(review.CorrectionCards))
		for _, card := range review.CorrectionCards {
			c.CardQuantities[card.ID] = card.ActualOutputQty
		}
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(CASE WHEN lower(trim(COALESCE(NULLIF(metrics_json->>'input_unit',''),NULLIF(batch_size_unit,''),'g'))) IN('kg','千克','公斤') THEN actual_input_qty*1000 ELSE actual_input_qty END),0)::float8 FROM %s.job_cards WHERE work_order_id=$1 AND status='completed'`, r.schema), wo.ID).Scan(&c.ActualInputG); err != nil {
			return out, err
		}
		if c.ActualInputG <= 0 {
			return out, fmt.Errorf("已完成工序没有记录实际投料，不能自动补齐耗料")
		}
		c.Materials = make([]app.OperationMaterialUse, 0, len(review.Materials))
		for _, row := range review.Materials {
			c.Materials = append(c.Materials, row.OperationMaterialUse)
		}
		var route processTemplateSnapshot
		if err = json.Unmarshal([]byte(wo.ProcessSnapshotJSON), &route); err != nil {
			return out, fmt.Errorf("冻结工艺路线无法读取，不能自动补齐耗料")
		}
		roasting := len(route.Operations) == 1 && isRoastingOperationName(route.Operations[0].Operation)
		if roasting {
			var raw []byte
			if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(material_snapshot,'[]'::jsonb) FROM %s.work_orders WHERE id=$1`, r.schema), wo.ID).Scan(&raw); err != nil {
				return out, err
			}
			var frozen []materialSnapshotRow
			if err = json.Unmarshal(raw, &frozen); err != nil {
				return out, err
			}
			for i := range c.Materials {
				for _, material := range frozen {
					component := material.MaterialID
					if strings.EqualFold(material.ComponentType, "product") {
						component = material.ComponentProductID
					}
					want := c.Materials[i].MaterialID
					if c.Materials[i].ProductID > 0 {
						want = c.Materials[i].ProductID
					}
					if component == want && material.ComponentBomSpecID == c.Materials[i].BomSpecID && material.ComponentBomVariantID == c.Materials[i].BomVariantID && material.ComponentSpecG == c.Materials[i].SpecG {
						c.Materials[i].RatioPct = material.RatioPct
						break
					}
				}
			}
			resolved, e := app.ResolveRoastingMaterialUses(c.Materials, c.ActualInputG, "g")
			if e != nil {
				return out, e
			}
			for i := range c.Materials {
				c.Materials[i].Qty = resolved[i].TotalQty
			}
		} else {
			for i := range c.Materials {
				c.Materials[i].Qty = review.Materials[i].Qty
			}
		}
	}
	if review.ReviewBlockedReason != "" {
		return out, fmt.Errorf("%s", review.ReviewBlockedReason)
	}
	quantity := *c.ActualOutputQty
	if err = app.ValidateOperationQuantity(quantity, review.OutputUnit); err != nil {
		return out, err
	}
	if err = app.ValidateOperationQuantity(c.ActualInputG, "g"); err != nil {
		return out, err
	}
	if quantity < review.ReceivedQty {
		return out, fmt.Errorf("核实数量 %g 小于已入库 %g %s，请先处理关联入库流水", quantity, review.ReceivedQty, review.OutputUnit)
	}
	if len(review.CorrectionCards) == 0 {
		return out, fmt.Errorf("缺少已完成的来源工序")
	}
	quantities := c.CardQuantities
	if len(review.CorrectionCards) == 1 {
		quantities = map[int64]float64{review.CorrectionCards[0].ID: quantity}
	} else {
		sum := 0.0
		for _, card := range review.CorrectionCards {
			q, ok := quantities[card.ID]
			if !ok {
				return out, fmt.Errorf("请核实每张来源工序卡的数量")
			}
			if err = app.ValidateOperationQuantity(q, review.OutputUnit); err != nil {
				return out, err
			}
			sum += q
		}
		if math.Abs(sum-quantity) > 1e-8 {
			return out, fmt.Errorf("来源工序卡数量合计与核实总量不一致")
		}
	}
	if len(c.Materials) != len(review.Materials) {
		return out, fmt.Errorf("请逐项核实本单累计实际用料")
	}
	desired := map[int64]app.OperationMaterialUse{}
	for _, m := range c.Materials {
		if _, exists := desired[m.ReservationID]; exists {
			return out, fmt.Errorf("用料行重复")
		}
		desired[m.ReservationID] = m
	}
	deltaMaterials := []app.OperationMaterialUse{}
	for _, row := range review.Materials {
		m, ok := desired[row.ReservationID]
		if !ok || m.Key != row.Key || m.OwnerCustomerID != row.OwnerCustomerID || m.Unit != row.Unit {
			return out, fmt.Errorf("冻结物料身份已变化")
		}
		if err = app.ValidateOperationQuantity(m.Qty, row.Unit); err != nil {
			return out, err
		}
		if m.Qty < row.ConsumedQty {
			return out, fmt.Errorf("%s 核实用量小于已有耗料 %g %s，请核对关联耗料流水", row.Name, row.ConsumedQty, row.Unit)
		}
		use := row.OperationMaterialUse
		use.Qty = m.Qty - row.ConsumedQty
		use.Warehouse = firstNonEmpty(m.Warehouse, "wip")
		if use.Qty > 0 {
			deltaMaterials = append(deltaMaterials, use)
		}
	}
	if err = r.creditLegacyConsumptionTx(ctx, tx, review.Materials); err != nil {
		return out, err
	}
	if wo.RunningItemID <= 0 {
		started, e := r.startWorkOrderTx(ctx, tx, app.WorkOrderStartCommand{ID: wo.ID, Operator: c.Operator}, false)
		if e != nil {
			return out, e
		}
		wo = started.WorkOrder
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET status='running' WHERE id=$1`, r.schema), wo.ID); err != nil {
		return out, err
	}
	for id, q := range quantities {
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET actual_operation_cost=CASE WHEN cost_method='piece' THEN piece_rate*$2 ELSE actual_operation_cost END WHERE id=$1`, r.schema), id, q); err != nil {
			return out, err
		}
	}
	var priorMaterialCost, operationCost, receivedCost float64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT GREATEST(COALESCE((SELECT material_cost FROM %[1]s.production_batch_costs WHERE running_item_id=$1),0),COALESCE((SELECT SUM(CASE WHEN l.deduct_g>0 THEN l.deduct_g*b.unit_cost/1000 ELSE l.deduct_units*b.unit_cost END) FROM %[1]s.material_consumption_logs l JOIN %[1]s.material_batches b ON b.id=l.material_batch_id WHERE l.running_item_id=$1),0),COALESCE((SELECT SUM(i.total_cost) FROM %[1]s.stock_entry_items i JOIN %[1]s.stock_entries e ON e.id=i.stock_entry_id WHERE e.work_order_id=$2 AND e.status='submitted' AND e.purpose='material_consumption_for_manufacture'),0))::float8,COALESCE((SELECT SUM(actual_operation_cost) FROM %[1]s.job_cards WHERE work_order_id=$2),0)::float8,COALESCE((SELECT SUM(i.total_cost) FROM %[1]s.stock_entry_items i JOIN %[1]s.stock_entries e ON e.id=i.stock_entry_id WHERE e.work_order_id=$2 AND e.status='submitted' AND e.entry_type='finished_receipt'),0)::float8`, r.schema), wo.RunningItemID, wo.ID).Scan(&priorMaterialCost, &operationCost, &receivedCost); err != nil {
		return out, err
	}
	jobID := review.CorrectionCards[0].ID
	payload, _ := json.Marshal(map[string]any{"legacy": true, "review": c, "original": review, "previous_received_qty": review.ReceivedQty, "previous_received_cost": receivedCost})
	var eventID int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_operation_events(work_order_id,job_card_id,request_id,expected_version,operator,input_qty,input_unit,output_qty,output_unit,planned_qty,difference_reason,warehouse,receipt_enabled,final_output,material_cost,operation_cost,payload) VALUES($1,$2,$3,$4,$5,$6,'g',$7,$8,$9,$10,$11,true,true,$12,$13,$14::jsonb) RETURNING id`, r.schema), wo.ID, jobID, c.RequestID, c.ExpectedVersion, c.Operator, c.ActualInputG, quantity, review.OutputUnit, review.PlannedQty, c.Reason, review.TargetWarehouse, priorMaterialCost, operationCost, string(payload)).Scan(&eventID); err != nil {
		return out, err
	}
	receipt := app.OperationCompletionReceipt{OutputWeightG: app.OperationOutputWeight(quantity-review.ReceivedQty, review.OutputUnit, wo.SpecG, wo.PlannedOutputG, operationPlannedTotal(wo)), EventID: eventID, OutputQty: quantity - review.ReceivedQty, OutputUnit: review.OutputUnit, Warehouse: review.TargetWarehouse, WarehouseName: review.TargetWarehouseName, StockEntryIDs: []int64{}, Message: "核对完成，已入" + review.TargetWarehouseName}
	stockCmd := stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialConsumption, WorkOrderID: wo.ID, JobCardID: jobID, RunningItemID: wo.RunningItemID, SourceType: "operation_completion", SourceID: eventID, Operator: c.Operator, IdempotencyKey: fmt.Sprintf("legacy:%d:consume", eventID), Note: "旧单核对补齐耗料；" + wo.WorkOrderNo}
	for _, m := range deltaMaterials {
		item := stockapp.StockDocumentItemCommand{ItemType: "material", MaterialID: m.MaterialID, ProductID: m.ProductID, BomSpecID: m.BomSpecID, BomVariantID: m.BomVariantID, SpecG: m.SpecG, ItemName: m.Name, InventoryUnit: m.Unit, OwnerCustomerID: m.OwnerCustomerID, FromWarehouse: m.Warehouse}
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
		doc, e := stockrepo.NewRepository(r.pool, r.schema).ConsumeOperationMaterialsTx(ctx, tx, stockCmd)
		if e != nil {
			return out, e
		}
		if err = r.recordOperationConsumptionLogsTx(ctx, tx, wo, doc.ID, c.Operator); err != nil {
			return out, err
		}
		priorMaterialCost += doc.TotalCost
		receipt.StockEntryIDs = append(receipt.StockEntryIDs, doc.ID)
	}
	if priorMaterialCost+operationCost+1e-6 < receivedCost {
		return out, fmt.Errorf("已有入库成本超过已核实生产成本，需核对原成本记录")
	}
	if !c.RecordedOutputOnly {
		for _, card := range review.CorrectionCards {
			q := quantities[card.ID]
			if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET actual_output_qty=$2,metrics_json=metrics_json||jsonb_build_object('quantity_basis','inventory_unit','inventory_unit',$3::text,'completion_event_id',$4::bigint) WHERE id=$1`, r.schema), card.ID, q, review.OutputUnit, eventID); err != nil {
				return out, err
			}
		}
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_operation_events SET material_cost=$2 WHERE id=$1`, r.schema), eventID, priorMaterialCost); err != nil {
		return out, err
	}
	if receipt.OutputQty > 0 {
		entryID, e := r.receiveOperationOutputTx(ctx, tx, wo, receipt, priorMaterialCost+operationCost-receivedCost, c.Operator)
		if e != nil {
			return out, e
		}
		receipt.StockEntryIDs = append(receipt.StockEntryIDs, entryID)
	}
	if err = r.finishOperationWorkOrderTx(ctx, tx, wo, c.Operator); err != nil {
		return out, err
	}
	if err = infra.AuditInsertTx(ctx, tx, r.schema, c.Operator, "work_order", &wo.ID, "legacy_reconcile", infra.StrPtr("actual_output_qty"), infra.StrPtr(fmt.Sprint(review.ReportedQty)), infra.StrPtr(fmt.Sprint(quantity)), infra.AuditMeta{"original": review, "confirmed": c, "event_id": eventID, "stock_entry_ids": receipt.StockEntryIDs, "material_cost": priorMaterialCost, "operation_cost": operationCost, "existing_receipt_cost": receivedCost}); err != nil {
		return out, err
	}
	raw, _ := json.Marshal(receipt)
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_operation_events SET status='posted',result=$2::jsonb WHERE id=$1`, r.schema), eventID, string(raw)); err != nil {
		return out, err
	}
	out.Succeeded = true
	out.Completion = &receipt
	if err = finishProductionKeyTx(ctx, tx, r.schema, "legacy_reconcile", c.Operator, c.RequestID, eventID, out); err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
