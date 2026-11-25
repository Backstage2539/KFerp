package sales

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	d "orderapp/internal/domain/shippingreturn"
	progresspg "orderapp/internal/infrastructure/postgres/orderprogress"
)

// A read-only snapshot keeps each row's current candidates, readiness and applied
// order consistent if someone confirms a shipment while the report is loading.
func (r Repository) ShippingReturnBatch(ctx context.Context, id int64) (d.Batch, error) {
	batch, err := r.returnBatch(ctx, id)
	batch.OrderIDs = nil // order_ids on an upload response is reserved for shipment notifications.
	return batch, err
}

func (r Repository) returnBatch(ctx context.Context, id int64) (d.Batch, error) {
	b := d.Batch{ID: id, Rows: []d.Row{}}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return b, err
	}
	defer tx.Rollback(ctx)
	var cfg d.Template
	if err = tx.QueryRow(ctx, fmt.Sprintf("SELECT template FROM %s.shipping_return_batches WHERE id=$1", r.schema), id).Scan(&cfg); err != nil {
		if err == pgx.ErrNoRows {
			return b, d.ErrBatchNotFound
		}
		return b, err
	}
	reasons := map[int64]string{}
	candidates, err := r.readShippingReturnCandidates(ctx, tx, 0, id, reasons)
	if err != nil {
		return b, err
	}
	byRow := make(map[int64][]d.Row)
	for _, v := range candidates {
		byRow[v.ID] = append(byRow[v.ID], v)
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT r.id,r.payload,r.status,COALESCE(o.id,0),COALESCE(o.order_no,''),COALESCE(to_char(o.order_date,'YYYY-MM-DD'),''),COALESCE(c.name,''),r.result
 FROM %[1]s.shipping_return_rows r LEFT JOIN %[1]s.orders o ON o.id=r.selected_order_id LEFT JOIN %[1]s.customers c ON c.id=o.customer_id WHERE r.batch_id=$1 ORDER BY r.id`, r.schema), id)
	if err != nil {
		return b, err
	}
	defer rows.Close()
	seenApplied := map[int64]bool{}
	for rows.Next() {
		var v d.Row
		var rid, oid int64
		var status, no, date, customer, result string
		if err = rows.Scan(&rid, &v, &status, &oid, &no, &date, &customer, &result); err != nil {
			return b, err
		}
		v.ID, v.BatchID, v.Status = rid, id, status
		v.OrderID, v.OrderNo, v.OrderDate, v.CustomerName = oid, no, date, customer
		switch status {
		case "applied":
			b.Updated++
			v.Error = result
			if !seenApplied[oid] {
				b.OrderIDs = append(b.OrderIDs, oid)
				seenApplied[oid] = true
			}
		case "pending":
			v.Candidates = byRow[rid]
			if len(v.Candidates) > 0 {
				b.Pending++
				// Selection failures (e.g. carrier configuration) are useful. Old
				// no-match messages from a previous rematch are superseded by live data.
				if result != "" && result != "未匹配收件信息一致的待发订单" && result != "该运单已被使用，请核对" {
					v.Error = "上次回填提示：" + result
				}
			} else {
				v.Status = "unmatched"
				v.Error = reasons[rid]
				if v.Error == "" {
					v.Error = "未匹配收件信息一致的待发订单"
				}
				b.Skipped++
			}
		default:
			// Older uploads saved a generic message even when the waybill was present.
			if v.Error == "单号或运费为空" && v.TrackingNo != "" {
				v.Error = cfg.MissingColumn("amount")
			}
			b.Skipped++
		}
		b.Rows = append(b.Rows, v)
	}
	if err = rows.Err(); err != nil {
		return b, err
	}
	b.Total = len(b.Rows)
	return b, tx.Commit(ctx)
}

func (r Repository) ShippingReturnCandidates(ctx context.Context, oid int64) ([]d.Row, error) {
	return r.liveShippingReturnCandidates(ctx, r.pool, oid)
}

func (r Repository) liveShippingReturnCandidates(ctx context.Context, q progresspg.Querier, oid int64) ([]d.Row, error) {
	return r.readShippingReturnCandidates(ctx, q, oid, 0, nil)
}

// A batch ID of zero deliberately spans all batches for the order picker and
// the automatic selection uniqueness check. Stock readiness never excludes a match.
func (r Repository) readShippingReturnCandidates(ctx context.Context, q progresspg.Querier, oid, batchID int64, reasons map[int64]string) ([]d.Row, error) {
	query := returnOrderSQL(r.schema)
	args := []any{}
	if oid > 0 {
		query += " AND o.id=$1"
		args = append(args, oid)
	}
	orders, err := q.Query(ctx, query+" ORDER BY o.id", args...)
	if err != nil {
		return nil, err
	}
	targets := map[string][]returnOrder{}
	for orders.Next() {
		var o returnOrder
		if err = orders.Scan(&o.id, &o.name, &o.phone, &o.version); err != nil {
			orders.Close()
			return nil, err
		}
		if key := d.PhoneMatchKey(o.phone); key != "" {
			targets[key] = append(targets[key], o)
		}
	}
	orders.Close()
	if err = orders.Err(); err != nil {
		return nil, err
	}
	rows, err := q.Query(ctx, fmt.Sprintf(`SELECT r.id,r.batch_id,r.payload,EXISTS(SELECT 1 FROM %[1]s.orders o WHERE o.ship_tracking_no=r.payload->>'tracking_no' UNION ALL SELECT 1 FROM %[1]s.order_shipping_trackings t WHERE t.tracking_no=r.payload->>'tracking_no')
 FROM %[1]s.shipping_return_rows r WHERE r.status='pending' AND ($1::bigint=0 OR r.batch_id=$1) ORDER BY r.id DESC`, r.schema), batchID)
	if err != nil {
		return nil, err
	}
	out := []d.Row{}
	matched := map[int64]bool{}
	for rows.Next() {
		var id, bid int64
		var occupied bool
		var item d.Row
		if err = rows.Scan(&id, &bid, &item, &occupied); err != nil {
			rows.Close()
			return nil, err
		}
		if occupied || item.Error != "" {
			if reasons != nil {
				reasons[id] = item.Error
				if occupied {
					reasons[id] = "该运单已被使用，请核对"
				}
			}
			continue
		}
		options := targets[d.PhoneMatchKey(item.Phone)]
		count := len(out)
		for _, o := range options {
			if !d.RecipientMatches(item.Receiver, item.Phone, o.name, o.phone) {
				continue
			}
			v := item
			v.ID, v.BatchID, v.OrderID, v.Version = id, bid, o.id, o.version
			out = append(out, v)
			matched[o.id] = true
		}
		if reasons != nil && len(out) == count {
			if len(options) == 0 {
				reasons[id] = "待发订单中没有收件手机号前三位和后四位一致的记录"
			} else {
				reasons[id] = "待发订单手机号可见位一致，但收件姓名不符"
			}
		}
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}
	ids := make([]int64, 0, len(matched))
	for id := range matched {
		ids = append(ids, id)
	}
	progress, err := progresspg.Load(ctx, q, r.schema, ids)
	if err != nil {
		return nil, err
	}
	type metadata struct{ no, date, customer string }
	details := map[int64]metadata{}
	meta, err := q.Query(ctx, fmt.Sprintf(`SELECT o.id,o.order_no,coalesce(to_char(o.order_date,'YYYY-MM-DD'),''),coalesce(c.name,'') FROM %[1]s.orders o LEFT JOIN %[1]s.customers c ON c.id=o.customer_id WHERE o.id=ANY($1)`, r.schema), ids)
	if err != nil {
		return nil, err
	}
	defer meta.Close()
	for meta.Next() {
		var id int64
		var m metadata
		if err = meta.Scan(&id, &m.no, &m.date, &m.customer); err != nil {
			return nil, err
		}
		details[id] = m
	}
	if err = meta.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		v := &out[i]
		m := details[v.OrderID]
		v.OrderNo, v.OrderDate, v.CustomerName = m.no, m.date, m.customer
		v.BlockReason = "订单尚未满足发货条件"
		if p := progress[v.OrderID]; p != nil {
			v.CanSelect, v.BlockReason = p.CanShip, p.BlockReason
		}
	}
	return out, nil
}
