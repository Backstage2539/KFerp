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

func (r Repository) PreviewOperationConversion(ctx context.Context, id int64) (app.OperationConversionPreview, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return app.OperationConversionPreview{}, err
	}
	defer tx.Rollback(ctx)
	p, _, err := r.operationConversionContextTx(ctx, tx, id)
	if err != nil {
		return p, err
	}
	return p, tx.Commit(ctx)
}
func (r Repository) operationConversionContextTx(ctx context.Context, tx pgx.Tx, id int64) (app.OperationConversionPreview, app.WorkOrderRow, error) {
	review, wo, err := r.legacyReviewContextTx(ctx, tx, id)
	p := app.OperationConversionPreview{LegacyReview: review, Cards: []app.OperationConversionCard{}}
	if err != nil {
		return p, wo, err
	}
	if review.ReviewBlockedReason == "本单仍有未完成工序，请先核对执行中的工序" {
		p.ReviewBlockedReason = ""
	}
	if review.ReceivedQty > 0 {
		p.ReviewBlockedReason = "本单已有历史入库，请先核对关联入库流水；不自动重建已入库批次"
	}
	var snapshot processTemplateSnapshot
	if err = json.Unmarshal([]byte(wo.ProcessSnapshotJSON), &snapshot); err != nil {
		return p, wo, err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id,sequence_no,operation,status,actual_input_qty::float8,actual_output_qty::float8 FROM %s.job_cards WHERE work_order_id=$1 AND status<>'cancelled' ORDER BY sequence_no,id`, r.schema), id)
	if err != nil {
		return p, wo, err
	}
	for rows.Next() {
		var c app.OperationConversionCard
		if err = rows.Scan(&c.ID, &c.Sequence, &c.Operation, &c.Status, &c.OriginalInput, &c.OriginalOutput); err != nil {
			rows.Close()
			return p, wo, err
		}
		p.Cards = append(p.Cards, c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return p, wo, err
	}
	if len(p.Cards) == 0 {
		p.ReviewBlockedReason = "本单缺少工序记录"
		return p, wo, nil
	}
	last := p.Cards[len(p.Cards)-1].Sequence
	for i := range p.Cards {
		c := &p.Cards[i]
		c.Execution = app.OperationExecution{Version: 1, NodeKey: fmt.Sprintf("legacy-%d-step-%d", id, c.Sequence), SourceWarehouse: "wip", OutputUnit: "g", ReceiptEnabled: c.Sequence == last}
		if c.Sequence == last {
			c.Execution.OutputUnit = review.OutputUnit
		}
		for _, op := range snapshot.Operations {
			if op.Seq == c.Sequence && op.ExecutionJSON != "" {
				var cfg app.OperationExecution
				if json.Unmarshal([]byte(op.ExecutionJSON), &cfg) == nil && cfg.Version == 1 {
					c.Execution = cfg
				}
			}
		}
		c.InputUnit = "g"
	}
	raw, _ := json.Marshal([]any{review.Version, p.Cards, wo.ProcessSnapshotJSON})
	p.Version = fmt.Sprintf("%x", sha256.Sum256(raw))
	return p, wo, nil
}
func (r Repository) SubmitOperationConversion(ctx context.Context, c app.OperationConversionCommand) ([]app.OperationCompletionReceipt, error) {
	return retryOperationTransaction(ctx, func() ([]app.OperationCompletionReceipt, error) { return r.submitOperationConversionOnce(ctx, c) })
}
func (r Repository) submitOperationConversionOnce(ctx context.Context, c app.OperationConversionCommand) ([]app.OperationCompletionReceipt, error) {
	if c.EmployeeID <= 0 || c.WorkOrderID <= 0 || c.RequestID == "" || c.ExpectedVersion == "" || strings.TrimSpace(c.Reason) == "" {
		return nil, fmt.Errorf("请填写核对原因，并由已绑定员工核实后提交")
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var run int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT running_item_id FROM %s.work_orders WHERE id=$1`, r.schema), c.WorkOrderID).Scan(&run); err != nil {
		return nil, err
	}
	if run > 0 {
		if err = customerstock.LockRunningFulfillmentCustomersTx(ctx, tx, r.schema, run); err != nil {
			return nil, err
		}
	}
	if err = lockReceiptWorkOrderTx(ctx, tx, r.schema, c.WorkOrderID); err != nil {
		return nil, err
	}
	replayID, replay, err := requestProductionKeyTx(ctx, tx, r.schema, "operation_conversion", c.Operator, c.RequestID, c)
	if err != nil {
		return nil, err
	}
	out := []app.OperationCompletionReceipt{}
	if replayID > 0 {
		err = json.Unmarshal(replay, &out)
		return out, err
	}
	p, wo, err := r.operationConversionContextTx(ctx, tx, c.WorkOrderID)
	if err != nil {
		return nil, err
	}
	if p.Version != c.ExpectedVersion {
		return nil, fmt.Errorf("工单或库存已变化，请刷新核对")
	}
	if p.ReviewBlockedReason != "" {
		return nil, fmt.Errorf("%s", p.ReviewBlockedReason)
	}
	if len(c.Cards) != len(p.Cards) || len(c.Materials) != len(p.Materials) {
		return nil, fmt.Errorf("请核对全部工序和物料归属")
	}
	submitted := map[int64]app.OperationConversionCard{}
	for _, card := range c.Cards {
		if _, ok := submitted[card.ID]; ok {
			return nil, fmt.Errorf("工序重复")
		}
		submitted[card.ID] = card
	}
	nodes := map[string]int{}
	nodeConfig := map[int]app.OperationExecution{}
	completedNodes := map[string]bool{}
	last := p.Cards[len(p.Cards)-1].Sequence
	for i, original := range p.Cards {
		card, ok := submitted[original.ID]
		if !ok || card.Sequence != original.Sequence {
			return nil, fmt.Errorf("工序身份已变化")
		}
		cfg := card.Execution
		if cfg.Version != 1 || cfg.NodeKey == "" || cfg.OutputUnit == "" {
			return nil, fmt.Errorf("请确认每道工序的数量单位")
		}
		if seq, exists := nodes[cfg.NodeKey]; exists && seq != original.Sequence {
			return nil, fmt.Errorf("不同工序不能使用同一身份")
		}
		nodes[cfg.NodeKey] = original.Sequence
		if old, exists := nodeConfig[original.Sequence]; exists && old != cfg {
			return nil, fmt.Errorf("同道工序的批次配置应一致")
		}
		nodeConfig[original.Sequence] = cfg
		if original.Sequence == last && cfg.OutputUnit != p.OutputUnit {
			return nil, fmt.Errorf("末道产出单位应为冻结单位 %s", p.OutputUnit)
		}
		if original.Sequence == last && !cfg.ReceiptEnabled {
			return nil, fmt.Errorf("末道工序需启用完成后入库")
		}
		if cfg.ReceiptEnabled && cfg.ReceiptWarehouse == "" && original.Sequence != last {
			return nil, fmt.Errorf("请确认中间工序实际存放位置")
		}
		for _, warehouse := range []string{firstNonEmpty(cfg.SourceWarehouse, "wip"), firstNonEmpty(cfg.ReceiptWarehouse, wo.TargetWarehouse, "finished_goods")} {
			var active bool
			if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.warehouses WHERE code=$1 AND active)`, r.schema), warehouse).Scan(&active); err != nil {
				return nil, err
			}
			if !active {
				return nil, fmt.Errorf("仓库不存在或已停用：%s", warehouse)
			}
		}
		card.Status = original.Status
		card.Operation = original.Operation
		if original.Status == "completed" {
			if card.InputQty == nil || card.OutputQty == nil {
				return nil, fmt.Errorf("请填写工序 #%d 核实的投入与产出", card.ID)
			}
			if err = app.ValidateOperationQuantity(*card.InputQty, card.InputUnit); err != nil {
				return nil, err
			}
			if err = app.ValidateOperationQuantity(*card.OutputQty, cfg.OutputUnit); err != nil {
				return nil, err
			}
			completedNodes[cfg.NodeKey] = true
		}
		p.Cards[i] = card
	}
	uses := map[string][]app.OperationMaterialUse{}
	bindings := map[string]app.OperationMaterialBinding{}
	given := map[int64]app.OperationMaterialUse{}
	for _, m := range c.Materials {
		if _, ok := given[m.ReservationID]; ok {
			return nil, fmt.Errorf("物料重复")
		}
		given[m.ReservationID] = m
	}
	for _, original := range p.Materials {
		m, ok := given[original.ReservationID]
		if !ok || m.Key != original.Key || m.Unit != original.Unit || m.OwnerCustomerID != original.OwnerCustomerID {
			return nil, fmt.Errorf("冻结物料身份已变化")
		}
		if _, ok = nodes[m.NodeKey]; !ok {
			return nil, fmt.Errorf("请指定 %s 使用工序", original.Name)
		}
		if err = app.ValidateOperationQuantity(m.Qty, m.Unit); err != nil {
			return nil, err
		}
		if m.Qty < original.ConsumedQty {
			return nil, fmt.Errorf("%s 核实用量少于已有耗料 %g %s，请先核对对应流水", m.Name, original.ConsumedQty, m.Unit)
		}
		if !completedNodes[m.NodeKey] && m.Qty > 0 {
			return nil, fmt.Errorf("%s 已用数量需归入已完成工序", m.Name)
		}
		use := original.OperationMaterialUse
		use.NodeKey = m.NodeKey
		use.Warehouse = firstNonEmpty(m.Warehouse, "wip")
		use.Qty = m.Qty - original.ConsumedQty
		uses[m.NodeKey] = append(uses[m.NodeKey], use)
		bindings[m.Key] = app.OperationMaterialBinding{NodeKey: m.NodeKey, SourceWarehouse: use.Warehouse}
	}
	if err = r.creditLegacyConsumptionTx(ctx, tx, p.Materials); err != nil {
		return nil, err
	}
	if wo.RunningItemID <= 0 {
		started, e := r.startWorkOrderTx(ctx, tx, app.WorkOrderStartCommand{ID: wo.ID, Operator: c.Operator}, false)
		if e != nil {
			return nil, e
		}
		wo = started.WorkOrder
	}
	var snapshot processTemplateSnapshot
	if err = json.Unmarshal([]byte(wo.ProcessSnapshotJSON), &snapshot); err != nil {
		return nil, err
	}
	if len(snapshot.Operations) == 0 {
		for _, card := range p.Cards {
			found := false
			for _, op := range snapshot.Operations {
				if op.Seq == card.Sequence {
					found = true
				}
			}
			if !found {
				snapshot.Operations = append(snapshot.Operations, processSnapshotOperation{Seq: card.Sequence, Operation: card.Operation})
			}
		}
	}
	for i := range snapshot.Operations {
		cfg, ok := nodeConfig[snapshot.Operations[i].Seq]
		if !ok {
			return nil, fmt.Errorf("冻结路线与工序卡不一致")
		}
		raw, _ := json.Marshal(cfg)
		snapshot.Operations[i].ExecutionJSON = string(raw)
	}
	frozen, _ := json.Marshal(snapshot)
	mapped, _ := json.Marshal(bindings)
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET process_snapshot_json=$2::jsonb,operation_material_bindings=$3::jsonb WHERE id=$1`, r.schema), wo.ID, string(frozen), string(mapped)); err != nil {
		return nil, err
	}
	var legacyCost float64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT GREATEST(COALESCE((SELECT material_cost FROM %[1]s.production_batch_costs WHERE running_item_id=$1),0),COALESCE((SELECT SUM(CASE WHEN l.deduct_g>0 THEN l.deduct_g*b.unit_cost/1000 ELSE l.deduct_units*b.unit_cost END) FROM %[1]s.material_consumption_logs l JOIN %[1]s.material_batches b ON b.id=l.material_batch_id WHERE l.running_item_id=$1),0),COALESCE((SELECT SUM(i.total_cost) FROM %[1]s.stock_entry_items i JOIN %[1]s.stock_entries d ON d.id=i.stock_entry_id WHERE d.work_order_id=$2 AND d.status='submitted' AND d.purpose='material_consumption_for_manufacture'),0))::float8`, r.schema), wo.RunningItemID, wo.ID).Scan(&legacyCost); err != nil {
		return nil, err
	}
	// Distribute each operation's cumulative verified material over its completed
	// batches by actual input, using cumulative rounding so no gram/unit is lost.
	nodeTotal, nodeBefore := map[string]float64{}, map[string]float64{}
	for _, card := range p.Cards {
		if card.Status == "completed" {
			w := math.Max(*card.InputQty, *card.OutputQty)
			if w <= 0 {
				w = 1
			}
			nodeTotal[card.Execution.NodeKey] += w
		}
	}
	legacyNodeCosts, err := r.legacyOperationCostsTx(ctx, tx, wo, p.Materials, bindings, legacyCost)
	if err != nil {
		return nil, err
	}
	allBalances := map[int][]app.OperationBatchBalance{}
	for _, card := range p.Cards {
		if card.Status != "completed" {
			continue
		}
		cfg := card.Execution
		var opCost float64
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT CASE WHEN cost_method='piece' THEN piece_rate*$2 ELSE actual_operation_cost END::float8 FROM %s.job_cards WHERE id=$1`, r.schema), card.ID, *card.OutputQty).Scan(&opCost); err != nil {
			return nil, err
		}
		warehouse := firstNonEmpty(cfg.ReceiptWarehouse, wo.TargetWarehouse, "finished_goods")
		if !cfg.ReceiptEnabled {
			warehouse = "wip"
		}
		prevSeq := 0
		for seq := range nodeConfig {
			if seq < card.Sequence && seq > prevSeq {
				prevSeq = seq
			}
		}
		allocations := []app.OperationHandoffAllocation{}
		carried := 0.0
		var ownerID int64
		if prevSeq > 0 {
			allocations, err = app.AllocateOperationHandoff(allBalances[prevSeq], *card.InputQty, card.InputUnit)
			if err != nil {
				return nil, fmt.Errorf("工序 #%d：%w", card.ID, err)
			}
			for _, a := range allocations {
				carried += a.Cost
				for i := range allBalances[prevSeq] {
					b := &allBalances[prevSeq][i]
					if b.ID == a.BatchID {
						if ownerID > 0 && b.OwnerCustomerID > 0 && ownerID != b.OwnerCustomerID {
							return nil, fmt.Errorf("不同货主不能混用")
						}
						if b.OwnerCustomerID > 0 {
							ownerID = b.OwnerCustomerID
						}
						b.Remaining -= a.Qty
						b.TotalCost -= a.Cost
					}
				}
			}
			if !cfg.ReceiptEnabled && len(allocations) > 0 {
				warehouse = allocations[0].Warehouse
			}
		}
		weight := math.Max(*card.InputQty, *card.OutputQty)
		if weight <= 0 {
			weight = 1
		}
		beforeShare := nodeBefore[cfg.NodeKey] / nodeTotal[cfg.NodeKey]
		afterShare := (nodeBefore[cfg.NodeKey] + weight) / nodeTotal[cfg.NodeKey]
		materialCost := legacyNodeCosts[cfg.NodeKey] * (afterShare - beforeShare)
		nodeBefore[cfg.NodeKey] += weight
		payload, _ := json.Marshal(map[string]any{"conversion": true, "original": p.LegacyReview, "confirmed": c, "source_card": card})
		receipt := app.OperationCompletionReceipt{OutputWeightG: app.OperationOutputWeight(*card.OutputQty, cfg.OutputUnit, wo.SpecG, wo.PlannedOutputG, operationPlannedTotal(wo)), OutputQty: *card.OutputQty, OutputUnit: cfg.OutputUnit, Warehouse: warehouse, StockEntryIDs: []int64{}, Handoffs: allocations, Message: "已交下一工序"}
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT name FROM %s.warehouses WHERE code=$1`, r.schema), warehouse).Scan(&receipt.WarehouseName); err != nil {
			return nil, err
		}
		if cfg.ReceiptEnabled {
			receipt.Message = "已入" + receipt.WarehouseName
		}
		if err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_operation_events(work_order_id,job_card_id,request_id,expected_version,operator,input_qty,input_unit,output_qty,output_unit,planned_qty,difference_reason,warehouse,receipt_enabled,final_output,material_cost,operation_cost,payload) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$8,$10,$11,$12,$13,$14,$15,$16::jsonb) RETURNING id`, r.schema), wo.ID, card.ID, c.RequestID, c.ExpectedVersion, c.Operator, *card.InputQty, card.InputUnit, *card.OutputQty, cfg.OutputUnit, c.Reason, warehouse, cfg.ReceiptEnabled, card.Sequence == last, materialCost, opCost, string(payload)).Scan(&receipt.EventID); err != nil {
			return nil, err
		}
		stockCmd := stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialConsumption, WorkOrderID: wo.ID, JobCardID: card.ID, RunningItemID: wo.RunningItemID, SourceType: "operation_completion", SourceID: receipt.EventID, Operator: c.Operator, IdempotencyKey: fmt.Sprintf("convert:%d", receipt.EventID), Note: "执行中工单核对补齐用料；" + wo.WorkOrderNo}
		{
			for _, m := range uses[cfg.NodeKey] {
				factor := app.OperationUnitFactor(m.Unit)
				if factor == 0 {
					factor = 1
				}
				m.Qty = (math.Round(m.Qty*factor*afterShare) - math.Round(m.Qty*factor*beforeShare)) / factor
				if m.OwnerCustomerID > 0 {
					if ownerID > 0 && ownerID != m.OwnerCustomerID {
						return nil, fmt.Errorf("不同货主不能混用")
					}
					ownerID = m.OwnerCustomerID
				}
				if m.Qty <= 0 {
					continue
				}
				item := stockapp.StockDocumentItemCommand{ItemType: "material", MaterialID: m.MaterialID, ProductID: m.ProductID, BomSpecID: m.BomSpecID, BomVariantID: m.BomVariantID, SpecG: m.SpecG, OwnerCustomerID: m.OwnerCustomerID, ItemName: m.Name, InventoryUnit: m.Unit, FromWarehouse: m.Warehouse}
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
		}
		if len(stockCmd.Items) > 0 {
			doc, e := stockrepo.NewRepository(r.pool, r.schema).ConsumeOperationMaterialsTx(ctx, tx, stockCmd)
			if e != nil {
				return nil, e
			}
			if err = r.recordOperationConsumptionLogsTx(ctx, tx, wo, doc.ID, c.Operator); err != nil {
				return nil, err
			}
			materialCost += doc.TotalCost
			receipt.StockEntryIDs = append(receipt.StockEntryIDs, doc.ID)
		}
		for _, a := range allocations {
			if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_operation_balances SET remaining_qty=remaining_qty-$2,remaining_cost=remaining_cost-$3 WHERE id=$1`, r.schema), a.BatchID, a.Qty, a.Cost); err != nil {
				return nil, err
			}
			if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_operation_movements(event_id,source_balance_id,qty,unit,warehouse,cost) VALUES($1,$2,$3,$4,$5,$6)`, r.schema), receipt.EventID, a.BatchID, a.Qty, a.Unit, a.Warehouse, a.Cost); err != nil {
				return nil, err
			}
		}
		warehouseOwner, e := warehouseCustomerID(ctx, tx, r.schema, warehouse)
		if e != nil {
			return nil, e
		}
		if ownerID > 0 && ((card.Sequence == last && warehouseOwner != ownerID) || (warehouseOwner > 0 && warehouseOwner != ownerID)) {
			return nil, fmt.Errorf("产出货主与存放位置不一致")
		}
		if card.Sequence == last && *card.OutputQty > 0 {
			entry, e := r.receiveOperationOutputTx(ctx, tx, wo, receipt, carried+materialCost+opCost, c.Operator)
			if e != nil {
				return nil, e
			}
			receipt.StockEntryIDs = append(receipt.StockEntryIDs, entry)
		} else if *card.OutputQty > 0 {
			balance := app.OperationBatchBalance{WorkOrderID: wo.ID, JobCardID: card.ID, OwnerCustomerID: ownerID, Warehouse: warehouse, Unit: cfg.OutputUnit, Remaining: *card.OutputQty, TotalCost: carried + materialCost + opCost}
			if err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_operation_balances(event_id,work_order_id,job_card_id,sequence_no,owner_customer_id,warehouse,unit,output_qty,remaining_qty,total_cost,remaining_cost) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$8,$9,$9) RETURNING id`, r.schema), receipt.EventID, wo.ID, card.ID, card.Sequence, ownerID, warehouse, cfg.OutputUnit, *card.OutputQty, balance.TotalCost).Scan(&balance.ID); err != nil {
				return nil, err
			}
			allBalances[card.Sequence] = append(allBalances[card.Sequence], balance)
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET actual_input_qty=$2,actual_output_qty=$3,actual_loss_qty=CASE WHEN $4::text=$5::text THEN GREATEST(0,$2::numeric-$3::numeric) ELSE 0 END,actual_operation_cost=$6,metrics_json=metrics_json||jsonb_build_object('quantity_basis','inventory_unit','inventory_unit',$5::text,'input_unit',$4::text,'completion_event_id',$7::bigint) WHERE id=$1`, r.schema), card.ID, *card.InputQty, *card.OutputQty, card.InputUnit, cfg.OutputUnit, opCost, receipt.EventID); err != nil {
			return nil, err
		}
		raw, _ := json.Marshal(receipt)
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_operation_events SET status='posted',material_cost=$2,result=$3::jsonb WHERE id=$1`, r.schema), receipt.EventID, materialCost, string(raw)); err != nil {
			return nil, err
		}
		out = append(out, receipt)
	}
	if len(out) > 0 {
		if err = r.finishOperationWorkOrderTx(ctx, tx, wo, c.Operator); err != nil {
			return nil, err
		}
	}
	if err = infra.AuditInsertTx(ctx, tx, r.schema, c.Operator, "work_order", &wo.ID, "operation_conversion", nil, nil, nil, infra.AuditMeta{"original": p, "confirmed": c, "results": out, "credited_material_cost": legacyCost}); err != nil {
		return nil, err
	}
	if err = finishProductionKeyTx(ctx, tx, r.schema, "operation_conversion", c.Operator, c.RequestID, wo.ID, out); err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}
