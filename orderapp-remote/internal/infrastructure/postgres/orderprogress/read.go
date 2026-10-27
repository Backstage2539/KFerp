// Package orderprogress reads fulfillment progress without changing business state.
package orderprogress

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	d "orderapp/internal/domain/orderprogress"
)

type Querier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// StockReadySQL counts the order's own allocation, using stable BOM spec identity.
// All active allocations sharing a batch must fit its physical remaining balance.
func StockReadySQL(s string) string {
	return fmt.Sprintf(`EXISTS(SELECT 1 FROM %[1]s.order_items i WHERE i.order_id=o.id AND i.qty>0) AND NOT EXISTS(
 SELECT 1 FROM %[1]s.order_items i WHERE i.order_id=o.id AND i.qty>0 AND COALESCE((
 SELECT SUM(a.allocated_qty) FROM %[1]s.customer_direct_ship_request_allocations a
 JOIN %[1]s.stock_batches b ON b.id=a.batch_id
 WHERE a.order_id=o.id AND a.order_item_id=i.id AND a.status='reserved'
 AND a.product_id=i.product_id AND a.bom_spec_id=COALESCE(i.bom_spec_id,0)
 AND (a.bom_spec_id>0 OR a.spec_g=b.spec_g)
 AND b.item_type='finished_product' AND b.item_id=a.product_id AND b.bom_spec_id=a.bom_spec_id
 AND lower(trim(COALESCE(b.quality_status,'unchecked'))) NOT IN ('hold','held','frozen','blocked','rejected','reject','failed','冻结','冻结批次','不合格','拒收') AND a.warehouse_code=o.source_warehouse
 AND EXISTS(SELECT 1 FROM %[1]s.warehouses w WHERE w.code=a.warehouse_code AND w.active AND (w.customer_id=o.customer_id OR (w.customer_id=0 AND NOT w.is_virtual AND b.owner_customer_id=o.customer_id)))
 AND (SELECT l.warehouse FROM %[1]s.stock_ledger_entries l WHERE l.source_batch_code=b.batch_code AND l.item_type=b.item_type AND l.item_id=b.item_id AND l.bom_spec_id=b.bom_spec_id AND l.spec_g=b.spec_g ORDER BY l.id DESC LIMIT 1)=a.warehouse_code
 AND COALESCE(NULLIF(to_jsonb(b)->>'owner_customer_id','')::bigint,0) IN(0,o.customer_id)
 AND (a.allocated_units>0 OR a.allocated_g>0)
 AND b.remaining_g >= (SELECT COALESCE(SUM(all_a.allocated_g),0) FROM %[1]s.customer_direct_ship_request_allocations all_a WHERE all_a.batch_id=b.id AND all_a.status='reserved')
 AND b.remaining_units >= (SELECT COALESCE(SUM(all_a.allocated_units),0) FROM %[1]s.customer_direct_ship_request_allocations all_a WHERE all_a.batch_id=b.id AND all_a.status='reserved')
 ),0)<i.qty)`, s)
}

func Load(ctx context.Context, q Querier, s string, ids []int64) (map[int64]*d.Progress, error) {
	out := map[int64]*d.Progress{}
	if len(ids) == 0 {
		return out, nil
	}
	var stockTables, supplyTable, processingTable bool
	if err := q.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL AND to_regclass($2) IS NOT NULL, to_regclass($3) IS NOT NULL,to_regclass($4) IS NOT NULL`, s+".customer_direct_ship_request_allocations", s+".stock_batches", s+".customer_stock_supply_reservations", s+".customer_processing_output_reservations").Scan(&stockTables, &supplyTable, &processingTable); err != nil {
		return nil, err
	}
	ready := "false"
	if stockTables {
		ready = StockReadySQL(s)
		if supplyTable {
			ready += fmt.Sprintf(` AND NOT EXISTS(SELECT 1 FROM %s.customer_stock_supply_reservations r WHERE r.order_id=o.id AND r.reserved_qty>r.converted_qty+r.released_qty)`, s)
		}
		if processingTable {
			ready += fmt.Sprintf(` AND NOT EXISTS(SELECT 1 FROM %s.customer_processing_output_reservations r WHERE r.order_id=o.id AND r.status<>'released' AND r.reserved_qty>r.converted_qty)`, s)
		}
	}
	rows, err := q.Query(ctx, fmt.Sprintf(`SELECT o.id,COALESCE(to_jsonb(o)->>'delivery_mode','ship'),COALESCE(to_jsonb(o)->>'delivery_status',''),COALESCE(to_jsonb(o)->>'fulfillment_mode',''),COALESCE(ops.name,''),COALESCE(ss.name,''),COALESCE(to_jsonb(o)->>'ship_method',''),COALESCE(to_jsonb(o)->>'confirmation_status','accepted'),o.is_void,CASE WHEN to_jsonb(o)->>'fulfillment_mode'='stock' THEN (%[2]s) ELSE false END FROM %[1]s.orders o LEFT JOIN %[1]s.order_process_statuses ops ON ops.id=o.process_status_id LEFT JOIN %[1]s.ship_statuses ss ON ss.id=o.ship_status_id WHERE o.id=ANY($1::bigint[])`, s, ready), ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var in d.Input
		if err = rows.Scan(&id, &in.DeliveryMode, &in.DeliveryStatus, &in.FulfillmentMode, &in.ProcessStatus, &in.ShipStatus, &in.ShipMethod, &in.ConfirmationStatus, &in.IsVoid, &in.StockReady); err != nil {
			return nil, err
		}
		v := d.Compute(in)
		out[id] = &v
	}
	return out, rows.Err()
}
