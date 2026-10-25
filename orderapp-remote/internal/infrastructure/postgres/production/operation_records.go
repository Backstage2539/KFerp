package production

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/production"
)

func (r Repository) ListOperationRecords(ctx context.Context, workID int64) ([]app.OperationRecord, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT e.id,e.job_card_id,e.work_order_id,w.work_order_no,w.product_name,COALESCE(NULLIF(w.sales_spec_snapshot_json->>'spec_label',''),CASE WHEN w.spec_g>0 THEN w.spec_g||'g/'||e.output_unit ELSE '' END),j.operation,e.output_unit,e.output_qty::float8,CASE WHEN e.payload->>'legacy'='true' THEN COALESCE((SELECT output_qty FROM %[1]s.production_operation_events WHERE reversal_of=e.id AND status='posted' ORDER BY id DESC LIMIT 1),e.output_qty) ELSE COALESCE((SELECT x.output_qty FROM %[1]s.production_operation_events x WHERE x.reversal_of=e.id AND x.kind='correct' AND x.status='posted' ORDER BY x.id DESC LIMIT 1),e.output_qty) END::float8,e.warehouse,wh.name,e.final_output,to_char(e.created_at,'YYYY-MM-DD HH24:MI:SS'),e.operator,e.estimated_cost,
 COALESCE((SELECT jsonb_agg(jsonb_build_object('id',x.id,'kind',x.kind,'quantity',x.output_qty,'reason',x.difference_reason,'material_cost',x.material_cost,'operation_cost',x.operation_cost,'operator',x.operator,'created_at',x.created_at,'result',x.result) ORDER BY x.id) FROM %[1]s.production_operation_events x WHERE x.id=e.id OR x.reversal_of=e.id),'[]'::jsonb)
 FROM %[1]s.production_operation_events e JOIN %[1]s.job_cards j ON j.id=e.job_card_id JOIN %[1]s.work_orders w ON w.id=e.work_order_id JOIN %[1]s.warehouses wh ON wh.code=e.warehouse WHERE e.kind='complete' AND e.status='posted' AND ($1::bigint=0 OR w.id=$1) ORDER BY e.id DESC LIMIT 500`, r.schema), workID)
	if err != nil {
		return nil, err
	}
	out := []app.OperationRecord{}
	for rows.Next() {
		var row app.OperationRecord
		if err = rows.Scan(&row.ID, &row.JobCardID, &row.WorkOrderID, &row.WorkOrderNo, &row.ProductName, &row.PackageSpecLabel, &row.Operation, &row.Unit, &row.OriginalQty, &row.Quantity, &row.Warehouse, &row.WarehouseName, &row.Final, &row.CreatedAt, &row.Operator, &row.EstimatedCost, &row.Details); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, row)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i := range out {
		cards, e := tx.Query(ctx, fmt.Sprintf(`SELECT j.id,j.operation,j.actual_output_qty::float8 FROM %[1]s.job_cards j JOIN %[1]s.production_operation_events event ON event.id=$1 WHERE j.id=event.job_card_id OR (event.payload->>'legacy'='true' AND j.work_order_id=event.work_order_id AND j.metrics_json->>'completion_event_id'=$1::text) ORDER BY j.id`, r.schema), out[i].ID)
		if e != nil {
			return nil, e
		}
		for cards.Next() {
			var c app.ReceiptCorrectionCard
			if e = cards.Scan(&c.ID, &c.Operation, &c.ActualOutputQty); e != nil {
				cards.Close()
				return nil, e
			}
			out[i].CorrectionCards = append(out[i].CorrectionCards, c)
		}
		e = cards.Err()
		cards.Close()
		if e != nil {
			return nil, e
		}
		out[i].Version, err = r.operationRecordVersionTx(ctx, tx, out[i].ID)
		if err != nil {
			return nil, err
		}
	}
	legacyRows, e := tx.Query(ctx, fmt.Sprintf(`SELECT -w.id,w.id,w.work_order_no,w.product_name,COALESCE(NULLIF(w.sales_spec_snapshot_json->>'spec_label',''),CASE WHEN w.spec_g>0 THEN w.spec_g||'g/'||COALESCE(NULLIF(w.output_unit,''),w.inventory_unit) ELSE '' END),COALESCE(NULLIF(w.output_unit,''),NULLIF(w.inventory_unit,''),'袋'),wh.code,wh.name,SUM(CASE WHEN COALESCE(NULLIF(w.output_unit,''),w.inventory_unit) IN('kg','千克','公斤') THEN i.qty_g/1000.0 WHEN COALESCE(NULLIF(w.output_unit,''),w.inventory_unit) IN('g','克') THEN i.qty_g ELSE i.qty_units END)::float8,MAX(to_char(d.created_at,'YYYY-MM-DD HH24:MI:SS')),MAX(d.operator),jsonb_agg(DISTINCT jsonb_build_object('id',d.id,'kind','legacy','reason',d.note,'result',jsonb_build_object('stock_entry_ids',jsonb_build_array(d.id)))) FROM %[1]s.work_orders w JOIN %[1]s.stock_entries d ON d.work_order_id=w.id AND d.status='submitted' AND d.entry_type='finished_receipt' JOIN %[1]s.stock_entry_items i ON i.stock_entry_id=d.id JOIN %[1]s.warehouses wh ON wh.code=i.to_warehouse WHERE ($1::bigint=0 OR w.id=$1) AND NOT EXISTS(SELECT 1 FROM %[1]s.production_operation_events e WHERE e.work_order_id=w.id AND e.status='posted') GROUP BY w.id,wh.code,wh.name ORDER BY w.id DESC LIMIT 500`, r.schema), workID)
	if e != nil {
		return nil, e
	}
	for legacyRows.Next() {
		row := app.OperationRecord{Legacy: true, Final: true, Operation: "历史完工入库"}
		if e = legacyRows.Scan(&row.ID, &row.WorkOrderID, &row.WorkOrderNo, &row.ProductName, &row.PackageSpecLabel, &row.Unit, &row.Warehouse, &row.WarehouseName, &row.Quantity, &row.CreatedAt, &row.Operator, &row.Details); e != nil {
			legacyRows.Close()
			return nil, e
		}
		row.OriginalQty = row.Quantity
		out = append(out, row)
	}
	e = legacyRows.Err()
	legacyRows.Close()
	if e != nil {
		return nil, e
	}
	return out, tx.Commit(ctx)
}
func (r Repository) operationRecordVersionTx(ctx context.Context, tx pgx.Tx, id int64) (string, error) {
	var version string
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT md5(e.xmin::text||':'||j.xmin::text||':'||w.xmin::text||':'||COALESCE((SELECT string_agg(b.id||':'||b.xmin::text,',' ORDER BY b.id) FROM %[1]s.stock_batches b WHERE b.batch_code IN(SELECT i.batch_code FROM %[1]s.stock_entry_items i JOIN %[1]s.stock_entries d ON d.id=i.stock_entry_id JOIN %[1]s.production_operation_events x ON x.id=d.source_id WHERE d.source_type IN('operation_completion','operation_correction') AND (x.id=e.id OR x.reversal_of=e.id))),'')||':'||COALESCE((SELECT xmin::text FROM %[1]s.production_operation_balances WHERE event_id=e.id),'')) FROM %[1]s.production_operation_events e JOIN %[1]s.job_cards j ON j.id=e.job_card_id JOIN %[1]s.work_orders w ON w.id=e.work_order_id WHERE e.id=$1`, r.schema), id).Scan(&version)
	return version, err
}
func (r Repository) ListOperationBalances(ctx context.Context) ([]app.OperationBatchBalance, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT b.id,b.work_order_id,b.job_card_id,b.owner_customer_id,b.remaining_qty::float8,b.unit,b.warehouse,wh.name,j.operation,w.product_name,w.work_order_no,b.remaining_cost::float8 FROM %[1]s.production_operation_balances b JOIN %[1]s.job_cards j ON j.id=b.job_card_id JOIN %[1]s.work_orders w ON w.id=b.work_order_id JOIN %[1]s.warehouses wh ON wh.code=b.warehouse WHERE b.remaining_qty>0 ORDER BY b.id`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []app.OperationBatchBalance{}
	for rows.Next() {
		var row app.OperationBatchBalance
		if err = rows.Scan(&row.ID, &row.WorkOrderID, &row.JobCardID, &row.OwnerCustomerID, &row.Remaining, &row.Unit, &row.Warehouse, &row.WarehouseName, &row.Operation, &row.ProductName, &row.WorkOrderNo, &row.TotalCost); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
