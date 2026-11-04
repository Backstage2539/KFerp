package stock

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"math"
)

// ReturnStartedComponentTx restores a locked, unallocated input component to its
// original physical batch. Identity and cost are loaded from the consumption
// ledger; no client-supplied inventory identity or price is accepted.
func (r Repository) ReturnStartedComponentTx(ctx context.Context, tx pgx.Tx, component, event int64, qty float64, actor string) (float64, error) {
	var inputID, workID, itemID, spec, variant, specG, materialBatch, stockBatch, owner, reservationID int64
	var kind, unit, warehouse, name, code string
	var remaining, cost float64
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT c.input_id,b.work_order_id,c.component_type,c.component_id,c.bom_spec_id,c.bom_variant_id,c.spec_g,c.material_batch_id,c.stock_batch_id,c.warehouse,c.owner_customer_id,c.unit,c.remaining_qty::float8,c.remaining_cost::float8,i.item_name,COALESCE((SELECT (m->>'reservation_id')::bigint FROM jsonb_array_elements(b.material_snapshot) m WHERE m->>'key'=c.component_key),0) FROM %[1]s.production_started_input_components c JOIN %[1]s.production_started_inputs b ON b.id=c.input_id JOIN %[1]s.stock_entry_items i ON i.id=c.stock_entry_item_id JOIN %[1]s.production_operation_events e ON e.id=$2 AND e.work_order_id=b.work_order_id AND e.job_card_id=b.job_card_id AND e.kind IN('material_return','start_reversal') AND e.status='posting' WHERE c.id=$1 AND b.status='posted' FOR UPDATE OF c,b`, r.schema), component, event).Scan(&inputID, &workID, &kind, &itemID, &spec, &variant, &specG, &materialBatch, &stockBatch, &warehouse, &owner, &unit, &remaining, &cost, &name,&reservationID)
	if err != nil {
		return 0, err
	}
	if qty <= 0 || math.IsNaN(qty) || math.IsInf(qty, 0) || qty != math.Trunc(qty) || qty > remaining {
		return 0, fmt.Errorf("退料超过未结转数量或精度无效")
	}
	returnedCost := cost
	if qty < remaining {
		returnedCost = math.Round(cost*qty/remaining*1e6) / 1e6
	}
	g, n := int64(0), int64(0)
	if unit == "g" {
		g = int64(qty)
	} else {
		n = int64(qty)
	}
	var beforeG, beforeN, afterG, afterN int64
	if kind == "material" {
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT batch_code FROM %s.material_batches WHERE id=$1 AND material_id=$2 AND COALESCE(owner_customer_id,0)=$3 FOR UPDATE`, r.schema), materialBatch, itemID, owner).Scan(&code); err != nil {
			return 0, err
		}
		beforeG, beforeN, err = materialBatchLocationBalanceTx(ctx, tx, r.schema, materialBatch, warehouse)
		if err != nil {
			return 0, err
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g,qty_units,updated_at) VALUES($1,$2,$3,$4,$5,$6,now()) ON CONFLICT(material_batch_id,warehouse) DO UPDATE SET qty_g=material_batch_locations.qty_g+EXCLUDED.qty_g,qty_units=material_batch_locations.qty_units+EXCLUDED.qty_units,updated_at=now()`, r.schema), materialBatch, code, itemID, warehouse, g, n); err != nil {
			return 0, err
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.material_batches SET remaining_g=remaining_g+$2,remaining_units=remaining_units+$3,status='active' WHERE id=$1`, r.schema), materialBatch, g, n); err != nil {
			return 0, err
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_batches SET remaining_g=remaining_g+$2,remaining_units=remaining_units+$3 WHERE item_type='material' AND item_id=$4 AND batch_code=$1 AND owner_customer_id=$5`, r.schema), code, g, n, itemID, owner); err != nil {
			return 0, err
		}
		if owner == 0 {
			if err = r.adjustMaterialOnhandTx(ctx, tx, itemID, g, n); err != nil {
				return 0, err
			}
		}
		afterG, afterN = beforeG+g, beforeN+n
	} else {
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT batch_code FROM %s.stock_batches WHERE id=$1 AND item_type='finished_product' AND item_id=$2 AND owner_customer_id=$3 FOR UPDATE`, r.schema), stockBatch, itemID, owner).Scan(&code); err != nil {
			return 0, err
		}
		var loose int64
		beforeN, loose, err = finishedInventoryQtyIdentityOwnedStockTx(ctx, tx, r.schema, itemID, spec, specG, warehouse, owner)
		if err != nil {
			return 0, err
		}
		beforeG = beforeN*specG + loose
		afterG, afterN = beforeG+g, beforeN+n
		afterLoose := loose
		if spec == 0 {
			afterN, afterLoose, _, err = normalizeFinishedQty(specG, 0, afterG)
			if err != nil {
				return 0, err
			}
		}
		if err = upsertFinishedInventoryIdentityOwnedStockTx(ctx, tx, r.schema, itemID, spec, variant, specG, warehouse, afterN, afterLoose, owner); err != nil {
			return 0, err
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_batches SET remaining_g=remaining_g+$2,remaining_units=CASE WHEN $4 AND spec_g>0 THEN (remaining_g+$2)/spec_g ELSE remaining_units+$3 END WHERE id=$1`, r.schema), stockBatch, g, n, unit == "g"); err != nil {
			return 0, err
		}
	}
	ledgerKind := kind
	if kind == "product" {
		ledgerKind = itemTypeFinishedProduct
	}
	if err = insertLedgerTx(ctx, tx, r.schema, ledgerEntry{ItemType: ledgerKind, ItemID: itemID, ItemName: name, SpecG: specG, BomSpecID: spec, BomVariantID: variant, OwnerCustomerID: owner, Warehouse: warehouse, SourceDocType: "production_input_return", SourceDocID: event, SourceBatchCode: code, BeforeG: beforeG, ChangeG: afterG - beforeG, AfterG: afterG, BeforeUnits: beforeN, ChangeUnits: afterN - beforeN, AfterUnits: afterN, Operator: actor}); err != nil {
		return 0, err
	}
	// Release only this work order's consumed projection, retaining other orders'
	// reservations and the original picking/consumption documents.
	var reservation int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`UPDATE %s.work_order_material_reservation_batches SET consumed_g=consumed_g-$5,consumed_units=consumed_units-$6,reserved_g=reserved_g-$5,reserved_units=reserved_units-$6,updated_at=now() WHERE work_order_id=$1 AND material_batch_id=$2 AND stock_batch_id=$3 AND warehouse=$4 AND owner_customer_id=$7 AND consumed_g>=$5 AND consumed_units>=$6 AND reservation_id=$8 RETURNING reservation_id`, r.schema), workID, materialBatch, stockBatch, warehouse, g, n, owner,reservationID).Scan(&reservation)
	if err != nil {
		return 0, fmt.Errorf("原耗料预留记录不一致：%w", err)
	}
	tag, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_order_material_reservations SET consumed_g=consumed_g-$2,consumed_units=consumed_units-$3,reserved_g=reserved_g-$2,reserved_units=reserved_units-$3,updated_at=now() WHERE id=$1 AND consumed_g>=$2 AND consumed_units>=$3`, r.schema), reservation, g, n)
	if err != nil {
		return 0, err
	}
	if tag.RowsAffected() != 1 {
		return 0, fmt.Errorf("原耗料汇总不一致")
	}
	if err = returnProcessingConsumptionTx(ctx, tx, r.schema, workID, materialBatch, stockBatch, itemID, warehouse, owner, g, n); err != nil {
		return 0, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_started_input_returns(component_id,event_id,qty,cost) VALUES($1,$2,$3,$4)`, r.schema), component, event, qty, returnedCost); err != nil {
		return 0, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_started_input_components SET remaining_qty=remaining_qty-$2,remaining_cost=remaining_cost-$3 WHERE id=$1`, r.schema), component, qty, returnedCost); err != nil {
		return 0, err
	}
	return returnedCost, nil
}

func returnProcessingConsumptionTx(ctx context.Context, tx pgx.Tx, schema string, work, mb, sb, item int64, warehouse string, owner, g, n int64) error {
	has, err := stockSchemaColumnExistsTx(ctx, tx, schema, "customer_processing_material_reservations", "id")
	if err != nil || !has {
		return err
	}
	batchColumn, itemColumn, batch := "material_batch_id", "material_id", mb
	if sb > 0 {
		batchColumn, itemColumn, batch = "finished_stock_batch_id", "component_product_id", sb
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id,consumed_g,consumed_units FROM %s.customer_processing_material_reservations WHERE work_order_id=$1 AND %s=$2 AND %s=$3 AND source_warehouse_code=$4 AND CASE WHEN source_owner_type='customer' THEN source_customer_id ELSE 0 END=$5 AND (consumed_g>0 OR consumed_units>0) ORDER BY id FOR UPDATE`, schema, batchColumn, itemColumn), work, batch, item, warehouse, owner)
	if err != nil {
		return err
	}
	type balance struct{ id, g, n int64 }
	all := []balance{}
	for rows.Next() {
		var b balance
		if err = rows.Scan(&b.id, &b.g, &b.n); err != nil {
			rows.Close()
			return err
		}
		all = append(all, b)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, b := range all {
		tg, tn := minInt64(g, b.g), minInt64(n, b.n)
		g -= tg
		n -= tn
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_processing_material_reservations SET consumed_g=consumed_g-$2,consumed_units=consumed_units-$3,reserved_g=reserved_g-$2,reserved_units=reserved_units-$3,status='reserved',updated_at=now() WHERE id=$1`, schema), b.id, tg, tn); err != nil {
			return err
		}
	}
	return nil
}
