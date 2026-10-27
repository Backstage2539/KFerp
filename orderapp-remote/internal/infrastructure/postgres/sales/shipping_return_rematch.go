package sales

import (
	"context"
	"encoding/json"
	"fmt"
	d "orderapp/internal/domain/shippingreturn"
	"orderapp/internal/infrastructure/postgres/freight"
	progresspg "orderapp/internal/infrastructure/postgres/orderprogress"
)

// RematchShippingReturn only updates pending candidates. It never invokes selection.
func (r Repository) RematchShippingReturn(ctx context.Context, actor string, batchID int64, rowIDs []int64) (d.Batch, error) {
	if batchID <= 0 {
		return d.Batch{}, fmt.Errorf("请选择导入批次")
	}
	for _, id := range rowIDs {
		if id <= 0 {
			return d.Batch{}, fmt.Errorf("无效的寄件行")
		}
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return d.Batch{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('freight-actual-posting'))`); err != nil {
		return d.Batch{}, err
	}
	var found bool
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.shipping_return_batches WHERE id=$1)`, r.schema), batchID).Scan(&found); err != nil {
		return d.Batch{}, err
	}
	if !found {
		return d.Batch{}, fmt.Errorf("导入批次不存在")
	}
	if len(rowIDs) > 0 {
		var count int
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT count(DISTINCT id) FROM %s.shipping_return_rows WHERE batch_id=$1 AND id=ANY($2::bigint[])`, r.schema), batchID, rowIDs).Scan(&count); err != nil {
			return d.Batch{}, err
		}
		unique := map[int64]bool{}
		for _, id := range rowIDs {
			unique[id] = true
		}
		if count != len(unique) {
			return d.Batch{}, fmt.Errorf("寄件行不属于所选批次")
		}
	}
	pending, err := tx.Query(ctx, fmt.Sprintf(`SELECT id,payload FROM %s.shipping_return_rows WHERE batch_id=$1 AND status='pending' AND ($2::bigint[] IS NULL OR id=ANY($2)) ORDER BY id FOR UPDATE`, r.schema), batchID, rowIDs)
	if err != nil {
		return d.Batch{}, err
	}
	items := []d.Row{}
	for pending.Next() {
		var id int64
		var raw []byte
		var v d.Row
		if err = pending.Scan(&id, &raw); err == nil {
			err = json.Unmarshal(raw, &v)
		}
		if err != nil {
			pending.Close()
			return d.Batch{}, err
		}
		v.ID = id
		items = append(items, v)
	}
	err = pending.Err()
	pending.Close()
	if err != nil {
		return d.Batch{}, err
	}
	orders, err := tx.Query(ctx, returnOrderSQL(r.schema))
	if err != nil {
		return d.Batch{}, err
	}
	targets := map[string][]returnOrder{}
	for orders.Next() {
		var o returnOrder
		if err = orders.Scan(&o.id, &o.name, &o.phone, &o.version); err != nil {
			orders.Close()
			return d.Batch{}, err
		}
		key := d.PhoneMatchKey(o.phone)
		if key != "" {
			targets[key] = append(targets[key], o)
		}
	}
	err = orders.Err()
	orders.Close()
	if err != nil {
		return d.Batch{}, err
	}
	ids := []int64{}
	for _, list := range targets {
		for _, o := range list {
			ids = append(ids, o.id)
		}
	}
	progress, err := progresspg.Load(ctx, tx, r.schema, ids)
	if err != nil {
		return d.Batch{}, err
	}
	changed := []int64{}
	for _, item := range items {
		if _, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.shipping_return_candidates WHERE row_id=$1`, r.schema), item.ID); err != nil {
			return d.Batch{}, err
		}
		var occupied bool
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.orders WHERE ship_tracking_no=$1 UNION ALL SELECT 1 FROM %[1]s.order_shipping_trackings WHERE tracking_no=$1)`, r.schema), item.TrackingNo).Scan(&occupied); err != nil {
			return d.Batch{}, err
		}
		reason := "未匹配当前可发货订单"
		if occupied {
			reason = "该运单已被使用，请核对"
		} else if item.Error != "" {
			reason = item.Error
		} else {
			for _, o := range targets[d.PhoneMatchKey(item.Phone)] {
				if progress[o.id] == nil || !progress[o.id].CanShip || !d.RecipientMatches(item.Receiver, item.Phone, o.name, o.phone) {
					continue
				}
				if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.shipping_return_candidates(row_id,order_id,version) VALUES($1,$2,$3)`, r.schema), item.ID, o.id, o.version); err != nil {
					return d.Batch{}, err
				}
				reason = ""
			}
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.shipping_return_rows SET result=$2 WHERE id=$1`, r.schema), item.ID, reason); err != nil {
			return d.Batch{}, err
		}
		changed = append(changed, item.ID)
	}
	if err = freight.NewRepository(r.pool, r.schema).Audit(ctx, tx, actor, "shipping_return", batchID, "rematch", nil, map[string]any{"row_ids": changed, "matched_only": true}); err != nil {
		return d.Batch{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return d.Batch{}, err
	}
	out, err := r.returnBatch(ctx, batchID)
	out.OrderIDs = nil
	out.Rematched = true
	return out, err
}
