package production

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"math"
	app "orderapp/internal/application/production"
	infra "orderapp/internal/infrastructure/postgres"
	"orderapp/internal/infrastructure/postgres/customerstock"
)

func (r Repository) correctOperationOutput(ctx context.Context, c app.CorrectJobCardOutputCommand) (app.FinishedReceiptTask, error) {
	return retryOperationTransaction(ctx, func() (app.FinishedReceiptTask, error) { return r.correctOperationOutputOnce(ctx, c) })
}

func (r Repository) correctOperationOutputOnce(ctx context.Context, c app.CorrectJobCardOutputCommand) (app.FinishedReceiptTask, error) {
	var out app.FinishedReceiptTask
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	var workID, runID int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT w.id,w.running_item_id FROM %[1]s.job_cards j JOIN %[1]s.work_orders w ON w.id=j.work_order_id WHERE j.id=$1`, r.schema), c.ID).Scan(&workID, &runID); err != nil {
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
	replayID, replay, err := requestProductionKeyTx(ctx, tx, r.schema, "operation_correction", c.Operator, c.RequestID, c)
	if err != nil {
		return out, err
	}
	if replayID > 0 {
		err = json.Unmarshal(replay, &out)
		return out, err
	}
	var original int64
	var previous, planned, input, totalCost, pieceRate float64
	var unit, inputUnit, warehouse, method string
	var final, receiptEnabled, estimated bool
	var seq int
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT e.id,j.actual_output_qty::float8,e.planned_qty::float8,e.input_qty::float8,e.output_unit,e.input_unit,e.warehouse,e.final_output,e.receipt_enabled,e.estimated_cost,e.material_cost::float8+e.operation_cost::float8+COALESCE((SELECT SUM(cost)::float8 FROM %[1]s.production_operation_movements WHERE event_id=e.id),0)+COALESCE((SELECT SUM(operation_cost)::float8 FROM %[1]s.production_operation_events WHERE reversal_of=e.id),0),j.cost_method,j.piece_rate::float8,j.sequence_no FROM %[1]s.production_operation_events e JOIN %[1]s.job_cards j ON j.id=e.job_card_id WHERE e.job_card_id=$1 AND e.kind='complete' AND e.status='posted' FOR UPDATE OF e,j`, r.schema), c.ID).Scan(&original, &previous, &planned, &input, &unit, &inputUnit, &warehouse, &final, &receiptEnabled, &estimated, &totalCost, &method, &pieceRate, &seq); err != nil {
		return out, err
	}
	var legacy bool
	var receivedBefore, receivedCost float64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(payload->>'legacy','false')::boolean,COALESCE((payload->>'previous_received_qty')::numeric,0)::float8,COALESCE((payload->>'previous_received_cost')::numeric,0)::float8 FROM %s.production_operation_events WHERE id=$1`, r.schema), original).Scan(&legacy, &receivedBefore, &receivedCost); err != nil {
		return out, err
	}
	cardQuantities := map[int64]float64{c.ID: c.ActualOutputQty}
	if legacy {
		if c.ActualOutputQty < receivedBefore {
			return out, fmt.Errorf("核实数量小于历史已入库 %g %s，请先处理历史入库记录", receivedBefore, unit)
		}
		rows, e := tx.Query(ctx, fmt.Sprintf(`SELECT id,actual_output_qty::float8 FROM %s.job_cards WHERE work_order_id=$1 AND metrics_json->>'completion_event_id'=$2::text ORDER BY id`, r.schema), workID, original)
		if e != nil {
			return out, e
		}
		ids := []int64{}
		previous = 0
		for rows.Next() {
			var id int64
			var qty float64
			if e = rows.Scan(&id, &qty); e != nil {
				rows.Close()
				return out, e
			}
			ids = append(ids, id)
			previous += qty
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return out, e
		}
		if len(ids) > 1 {
			cardQuantities = map[int64]float64{}
			sum := 0.0
			for _, id := range ids {
				q, ok := c.CardQuantities[id]
				if !ok {
					return out, fmt.Errorf("请填写每张来源工序的正确数量")
				}
				if e = app.ValidateOperationQuantity(q, unit); e != nil {
					return out, e
				}
				cardQuantities[id] = q
				sum += q
			}
			if math.Abs(sum-c.ActualOutputQty) > 1e-8 {
				return out, fmt.Errorf("来源工序合计数量不一致")
			}
		}
	}
	version, err := r.operationRecordVersionTx(ctx, tx, original)
	if err != nil {
		return out, err
	}
	if version != c.ExpectedVersion {
		return out, fmt.Errorf("完成记录或库存已变化，请刷新后更正")
	}
	if err = app.ValidateOperationQuantity(c.ActualOutputQty, unit); err != nil {
		return out, err
	}
	wo, err := loadWorkOrderExecutionRowTx(ctx, tx, r.schema, workID)
	if err != nil {
		return out, err
	}
	var qualityBlocked bool
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM (SELECT DISTINCT ON(job_card_id,scope,reference_type,reference_no) result FROM %s.quality_inspections WHERE (work_order_id=$1 AND job_card_id=0) OR job_card_id=$2 ORDER BY job_card_id,scope,reference_type,reference_no,created_at DESC,id DESC) latest WHERE lower(trim(result)) IN('hold','reject','failed','fail','blocked'))`, r.schema), workID, c.ID).Scan(&qualityBlocked); err != nil {
		return out, err
	}
	if qualityBlocked {
		return out, fmt.Errorf("本记录关联质量冻结，请先处理质量记录")
	}
	if has, e := schemaColumnExistsTx(ctx, tx, r.schema, "customer_fee_items", "id"); e != nil {
		return out, e
	} else if has {
		var conflicts string
		if e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(string_agg(id::text,',' ORDER BY id),'') FROM %s.customer_fee_items WHERE ((source_type IN('work_order','work_order_complete') AND source_id=$1) OR (source_type IN('production_run','running_item') AND source_id=$2)) AND (settlement_batch_id>0 OR status NOT IN('unsettled','void','cancelled'))`, r.schema), workID, runID).Scan(&conflicts); e != nil {
			return out, e
		}
		if conflicts != "" {
			return out, fmt.Errorf("已关联结算费用记录 %s，请先处理结算后更正", conflicts)
		}
	}
	opDelta := 0.0
	cardCostDeltas := map[int64]float64{}
	for id, q := range cardQuantities {
		var delta float64
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT CASE WHEN cost_method='piece' THEN ($2-actual_output_qty)*piece_rate ELSE 0 END::float8 FROM %s.job_cards WHERE id=$1`, r.schema), id, q).Scan(&delta); err != nil {
			return out, err
		}
		cardCostDeltas[id] = delta
		opDelta += delta
	}
	totalCost += opDelta
	payload, _ := json.Marshal(c)
	var eventID int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_operation_events(work_order_id,job_card_id,kind,status,request_id,expected_version,operator,input_qty,input_unit,output_qty,output_unit,planned_qty,difference_reason,warehouse,receipt_enabled,final_output,operation_cost,estimated_cost,reversal_of,payload) VALUES($1,$2,'correct','posting',$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18::jsonb) RETURNING id`, r.schema), workID, c.ID, c.RequestID, c.ExpectedVersion, c.Operator, input, inputUnit, c.ActualOutputQty, unit, planned, c.Reason, warehouse, receiptEnabled, final, opDelta, estimated, original, string(payload)).Scan(&eventID)
	if err != nil {
		return out, err
	}
	var entryID int64
	if final {
		entryID, err = r.correctOperationReceiptTx(ctx, tx, wo, original, eventID, previous-receivedBefore, c.ActualOutputQty-receivedBefore, unit, warehouse, totalCost-receivedCost, c.Operator)
		if err != nil {
			return out, err
		}
	} else {
		var used float64
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(qty),0)::float8 FROM %[1]s.production_operation_movements WHERE source_balance_id IN(SELECT id FROM %[1]s.production_operation_balances WHERE event_id=$1)`, r.schema), original).Scan(&used); err != nil {
			return out, err
		}
		if used > 0 {
			return out, fmt.Errorf("原产出已有 %g %s 被后序使用，请先更正关联交接记录（来源完成记录 #%d）", used, unit, original)
		}
		var owner int64
		var quality string
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT owner_customer_id,quality_status FROM %s.production_operation_balances WHERE event_id=$1 FOR UPDATE`, r.schema), original).Scan(&owner, &quality)
		if err != nil && err != pgx.ErrNoRows {
			return out, err
		}
		if quality == "hold" || quality == "reject" {
			return out, fmt.Errorf("本批次处于质量冻结")
		}
		if err == pgx.ErrNoRows {
			if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(MAX(source_owner_customer_id),0) FROM %s.work_order_material_reservations WHERE work_order_id=$1`, r.schema), workID).Scan(&owner); err != nil {
				return out, err
			}
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_operation_balances(event_id,work_order_id,job_card_id,sequence_no,owner_customer_id,warehouse,unit,output_qty,remaining_qty,total_cost,remaining_cost) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$8,$9,$9) ON CONFLICT(event_id) DO UPDATE SET output_qty=EXCLUDED.output_qty,remaining_qty=EXCLUDED.remaining_qty,total_cost=EXCLUDED.total_cost,remaining_cost=EXCLUDED.remaining_cost`, r.schema), original, workID, c.ID, seq, owner, warehouse, unit, c.ActualOutputQty, totalCost); err != nil {
			return out, err
		}
	}
	for id, q := range cardQuantities {
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET actual_output_qty=$2,actual_operation_cost=actual_operation_cost+$3,actual_loss_qty=CASE WHEN $4::text=$5::text THEN GREATEST(0,actual_input_qty-$2) ELSE 0 END WHERE id=$1`, r.schema), id, q, cardCostDeltas[id], inputUnit, unit); err != nil {
			return out, err
		}
	}

	result := map[string]any{"output_weight_g": app.OperationOutputWeight(c.ActualOutputQty, unit, wo.SpecG, wo.PlannedOutputG, operationPlannedTotal(wo)), "previous_qty": previous, "quantity": c.ActualOutputQty, "delta": c.ActualOutputQty - previous, "stock_entry_id": entryID, "reason": c.Reason}
	raw, _ := json.Marshal(result)
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_operation_events SET status='posted',result=$2::jsonb WHERE id=$1`, r.schema), eventID, string(raw)); err != nil {
		return out, err
	}
	if err = r.finishOperationWorkOrderTx(ctx, tx, wo, c.Operator); err != nil {
		return out, err
	}
	if err = infra.AuditInsertTx(ctx, tx, r.schema, c.Operator, "job_card", &c.ID, "correct_output", infra.StrPtr("actual_output_qty"), infra.StrPtr(fmt.Sprint(previous)), infra.StrPtr(fmt.Sprint(c.ActualOutputQty)), infra.AuditMeta{"event_id": eventID, "original_event_id": original, "reason": c.Reason, "stock_entry_id": entryID, "request_id": c.RequestID, "inventory_delta": c.ActualOutputQty - previous}); err != nil {
		return out, err
	}
	out = app.FinishedReceiptTask{WorkOrderID: workID, WorkOrderNo: wo.WorkOrderNo, OutputName: wo.ProductName, OutputUnit: unit, TargetWarehouse: warehouse, ReportedQty: c.ActualOutputQty, ReceivedQty: c.ActualOutputQty, ReceiptStatus: "received"}
	out.ReceiptVersion, err = r.operationRecordVersionTx(ctx, tx, original)
	if err != nil {
		return out, err
	}
	if err = finishProductionKeyTx(ctx, tx, r.schema, "operation_correction", c.Operator, c.RequestID, eventID, out); err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}

func (r Repository) correctOperationReceiptTx(ctx context.Context, tx pgx.Tx, wo app.WorkOrderRow, original, eventID int64, previous, next float64, unit, warehouse string, cost float64, actor string) (int64, error) {
	var batchID, owner, qtyG, qtyN, remainingG, remainingN int64
	var code, quality string
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT b.id,b.batch_code,b.owner_customer_id,b.qty_g,b.qty_units,b.remaining_g,b.remaining_units,b.quality_status FROM %[1]s.stock_batches b WHERE b.batch_code IN(SELECT i.batch_code FROM %[1]s.stock_entry_items i JOIN %[1]s.stock_entries d ON d.id=i.stock_entry_id JOIN %[1]s.production_operation_events x ON x.id=d.source_id WHERE d.source_type IN('operation_completion','operation_correction') AND d.entry_type='finished_receipt' AND (x.id=$1 OR x.reversal_of=$1)) ORDER BY b.id LIMIT 1 FOR UPDATE OF b`, r.schema), original).Scan(&batchID, &code, &owner, &qtyG, &qtyN, &remainingG, &remainingN, &quality)
	if err == pgx.ErrNoRows && previous == 0 {
		if next == 0 {
			return 0, nil
		}
		return r.receiveOperationOutputTx(ctx, tx, wo, app.OperationCompletionReceipt{EventID: eventID, OutputQty: next, OutputUnit: unit, Warehouse: warehouse}, cost, actor)
	}
	if err != nil {
		return 0, err
	}
	if quality == "hold" || quality == "reject" {
		return 0, fmt.Errorf("批次 %s 处于质量冻结，不能更正库存", code)
	}
	if qtyG != remainingG || qtyN != remainingN {
		return 0, fmt.Errorf("批次 %s 已出库或被使用，请先处理关联库存流水", code)
	}
	var reserved bool
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.work_order_material_reservation_batches WHERE (stock_batch_id=$1 OR batch_code=$2) AND status='reserved' AND (reserved_g>consumed_g+returned_g OR reserved_units>consumed_units+returned_units)) OR EXISTS(SELECT 1 FROM %[1]s.customer_stock_entitlement_receipts WHERE stock_batch_id=$1)`, r.schema), batchID, code).Scan(&reserved); err != nil {
		return 0, err
	}
	if has, e := schemaColumnExistsTx(ctx, tx, r.schema, "customer_processing_material_reservations", "id"); e != nil {
		return 0, e
	} else if has {
		var conflicts string
		if e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(string_agg(id::text,',' ORDER BY id),'') FROM %s.customer_processing_material_reservations WHERE finished_stock_batch_id=$1 AND status='reserved' AND (reserved_g>consumed_g+returned_g OR reserved_units>consumed_units+returned_units)`, r.schema), batchID).Scan(&conflicts); e != nil {
			return 0, e
		}
		if conflicts != "" {
			return 0, fmt.Errorf("批次 %s 已关联加工预留记录 %s，请先处理关联记录", code, conflicts)
		}
	}
	if reserved {
		return 0, fmt.Errorf("批次 %s 已关联下游预留或客户交付记录，需先处理关联记录", code)
	}
	var nextG, nextN int64
	if factor := app.OperationUnitFactor(unit); factor > 0 {
		nextG = int64(math.Round(next * factor))
	} else {
		nextN = int64(next)
		if wo.OutputType != "material" {
			nextG = nextN * wo.SpecG
		}
	}
	deltaG, deltaN := nextG-qtyG, nextN-qtyN
	unitCost := 0.0
	if nextG > 0 {
		unitCost = cost * 1000 / float64(nextG)
	} else if nextN > 0 {
		unitCost = cost / float64(nextN)
	}
	item := app.StockEntryItemCommand{ItemName: wo.ProductName, OwnerCustomerID: owner, SpecG: wo.SpecG, BomSpecID: wo.BomSpecID, BomVariantID: wo.BomVariantID, ToWarehouse: warehouse, QtyG: deltaG, QtyUnits: deltaN, BatchCode: code}
	if wo.OutputType == "material" {
		item.MaterialID = wo.OutputMaterialID
		item.ItemType = stockItemTypeMaterial
		var materialBatch int64
		var beforeG, beforeN int64
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT b.id,l.qty_g,l.qty_units FROM %[1]s.material_batches b JOIN %[1]s.material_batch_locations l ON l.material_batch_id=b.id WHERE b.batch_code=$1 AND l.warehouse=$2 FOR UPDATE OF b,l`, r.schema), code, warehouse).Scan(&materialBatch, &beforeG, &beforeN); err != nil {
			return 0, err
		}
		if beforeG != qtyG || beforeN != qtyN {
			return 0, fmt.Errorf("批次 %s 已转仓或使用，需先处理库存记录", code)
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.material_batch_locations SET qty_g=$2,qty_units=$3,updated_at=now() WHERE material_batch_id=$1 AND warehouse=$4`, r.schema), materialBatch, nextG, nextN, warehouse); err != nil {
			return 0, err
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.material_batches SET qty_g=$2,received_g=$2,qty_units=$3,remaining_g=$2,remaining_units=$3,unit_cost=$4 WHERE id=$1`, r.schema), materialBatch, nextG, nextN, unitCost); err != nil {
			return 0, err
		}
		if owner == 0 {
			if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET onhand_g=onhand_g+$2,onhand_units=onhand_units+$3 WHERE id=$1`, r.schema), wo.OutputMaterialID, deltaG, deltaN); err != nil {
				return 0, err
			}
		}
		if err = insertStockLedgerEntryOwnedTx(ctx, tx, r.schema, item.ItemType, item.MaterialID, item.ItemName, owner, 0, warehouse, "operation_correction", eventID, code, wo.BatchID, stockLedgerQty{BeforeG: beforeG, ChangeG: deltaG, AfterG: nextG, BeforeUnits: beforeN, ChangeUnits: deltaN, AfterUnits: nextN}, actor); err != nil {
			return 0, err
		}
	} else {
		item.ProductID = wo.ProductID
		if wo.OutputProductID > 0 {
			item.ProductID = wo.OutputProductID
		}
		item.ItemType = stockItemTypeFinishedProduct
		var actualWarehouse string
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT warehouse FROM %s.stock_ledger_entries WHERE source_batch_code=$1 AND item_type='finished_product' AND item_id=$2 ORDER BY id DESC LIMIT 1`, r.schema), code, item.ProductID).Scan(&actualWarehouse); err != nil {
			return 0, err
		}
		if actualWarehouse != warehouse {
			return 0, fmt.Errorf("批次 %s 已转仓至 %s，请核对关联转仓记录", code, actualWarehouse)
		}
		n, g, err := finishedInventoryQtyIdentityOwnedTx(ctx, tx, r.schema, item.ProductID, wo.BomSpecID, wo.SpecG, warehouse, owner)
		if err != nil {
			return 0, err
		}
		afterN, afterG := n+deltaN, g
		if app.OperationUnitFactor(unit) > 0 {
			afterG += deltaG
		}
		if afterN < 0 || afterG < 0 {
			return 0, fmt.Errorf("更正会造成负库存")
		}
		if err = r.upsertOperationInventoryTx(ctx, tx, item.ProductID, wo.BomSpecID, wo.BomVariantID, wo.SpecG, warehouse, afterN, afterG, owner); err != nil {
			return 0, err
		}
		if err = insertStockLedgerEntryWithBomSpecOwnedTx(ctx, tx, r.schema, item.ItemType, item.ProductID, item.ItemName, wo.BomSpecID, wo.BomVariantID, owner, wo.SpecG, warehouse, "operation_correction", eventID, code, wo.BatchID, stockLedgerQty{BeforeG: n*wo.SpecG + g, ChangeG: deltaG, AfterG: afterN*wo.SpecG + afterG, BeforeUnits: n, ChangeUnits: deltaN, AfterUnits: afterN}, actor); err != nil {
			return 0, err
		}
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_batches SET qty_g=$2,qty_units=$3,remaining_g=$2,remaining_units=$3,unit_cost=$4 WHERE id=$1`, r.schema), batchID, nextG, nextN, unitCost); err != nil {
		return 0, err
	}
	entry, err := createStockEntryRecordTx(ctx, tx, r.schema, app.StockEntryCommand{EntryType: "finished_receipt", Purpose: "reconciliation", WorkOrderID: wo.ID, RunningItemID: wo.RunningItemID, SourceType: "operation_correction", SourceID: eventID, Operator: actor, Note: fmt.Sprintf("来源工单 %s；完成记录 #%d 更正：%g → %g %s", wo.WorkOrderNo, original, previous, next, unit), Items: []app.StockEntryItemCommand{item}}, false)
	if err != nil {
		return 0, err
	}
	var costDelta float64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT operation_cost::float8 FROM %s.production_operation_events WHERE id=$1`, r.schema), eventID).Scan(&costDelta); err != nil {
		return 0, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_entry_items SET total_cost=$2::numeric,unit_cost=CASE WHEN qty_g<>0 THEN $2::numeric*1000/qty_g WHEN qty_units<>0 THEN $2::numeric/qty_units ELSE 0 END WHERE stock_entry_id=$1`, r.schema), entry.ID, costDelta); err != nil {
		return 0, err
	}
	if wo.OutputType == "material" {
		_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.production_material_receipts(work_order_id,running_item_id,material_batch_id,stock_entry_id,finished_g,finished_units,input_g,material_cost,operation_cost,completion_mode) SELECT $1,$2,id,$3,$4,$5,0,0,$6,'correction' FROM %[1]s.material_batches WHERE batch_code=$7 AND material_id=$8`, r.schema), wo.ID, wo.RunningItemID, entry.ID, deltaG, deltaN, costDelta, code, wo.OutputMaterialID)
	} else {
		no, e := nextCompletionNoForRunningItemTx(ctx, tx, r.schema, wo.RunningItemID)
		if e != nil {
			return 0, e
		}
		loose := int64(0)
		if app.OperationUnitFactor(unit) > 0 {
			loose = deltaG
		}
		_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.production_logs(running_item_id,completion_no,batch_id,product_id,bom_spec_id,bom_variant_id,product_name,spec_g,order_nos,planned_need_g,input_g,bom_yield_rate,finished_units,finished_loose_g,finished_total_g,actual_yield_rate,started_by,started_at,finished_by,finished_at,inventory_units_before,inventory_loose_g_before,inventory_units_after,inventory_loose_g_after,material_summary,created_at)
       SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9,0,0,1,$10,$11,$12,1,run.started_by,run.started_at,$13,now(),fi.onhand_units-$10,fi.onhand_loose_g-$11,fi.onhand_units,fi.onhand_loose_g,jsonb_build_array(jsonb_build_object('correction_event_id',$14::bigint,'original_event_id',$15::bigint)),now() FROM %[1]s.produce_running_items run JOIN %[1]s.finished_inventory fi ON fi.product_id=$4 AND fi.bom_spec_id=$5 AND fi.spec_g=$8 AND fi.warehouse=$16 AND fi.owner_customer_id=$17 WHERE run.id=$1`, r.schema), wo.RunningItemID, no, wo.BatchID, item.ProductID, wo.BomSpecID, wo.BomVariantID, wo.ProductName, wo.SpecG, wo.OrderNos, deltaN, loose, deltaG, actor, eventID, original, warehouse, owner)
	}
	return entry.ID, err
}
