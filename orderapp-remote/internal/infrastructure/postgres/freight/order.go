package freight

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"math"
	domain "orderapp/internal/domain/freight"
)

// All ordinary order editors and actual-bill writers take this lock before orders.
func LockOrderFreight(ctx context.Context, tx pgx.Tx) error {
	_, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('freight-actual-posting'))`)
	return e
}
func (r *Repository) SnapshotTx(ctx context.Context, q Queryer, id int64) (*domain.Snapshot, error) {
	var raw []byte
	e := q.QueryRow(ctx, fmt.Sprintf(`SELECT snapshot_json FROM %s.order_freight_snapshots WHERE order_id=$1`, r.schema), id).Scan(&raw)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	var s domain.Snapshot
	e = json.Unmarshal(raw, &s)
	return &s, e
}
func (r *Repository) EditableTx(ctx context.Context, q Queryer, id int64) error {
	if !relationExists(ctx, q, r.schema+".freight_actual_shipments") {
		return nil
	}
	var locked bool
	e := q.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.freight_actual_shipments WHERE order_id=$1 AND revision>0) OR EXISTS(SELECT 1 FROM %[1]s.customer_fee_items f JOIN %[1]s.freight_actual_entries e ON f.source_type='freight_actual' AND f.source_id=e.id JOIN %[1]s.freight_actual_shipments s ON s.id=e.shipment_id WHERE s.order_id=$1)`, r.schema), id).Scan(&locked)
	if e != nil {
		return e
	}
	if locked {
		return fmt.Errorf("运费已出账或已确认实际费用，不能修改")
	}
	return nil
}
func (r *Repository) QuoteOrderTx(ctx context.Context, tx pgx.Tx, customerID, orderID int64, expected string, d domain.Destination, items []WeightItem, o *domain.Override) (domain.Snapshot, error) {
	var out domain.Snapshot
	var previous *domain.Snapshot
	if orderID > 0 {
		if e := LockOrderFreight(ctx, tx); e != nil {
			return out, e
		}
		var cid int64
		var mode string
		if e := tx.QueryRow(ctx, fmt.Sprintf(`SELECT customer_id,COALESCE(to_jsonb(o)->>'fulfillment_mode','') FROM %s.orders o WHERE id=$1 AND NOT COALESCE(is_void,false) FOR UPDATE`, r.schema), orderID).Scan(&cid, &mode); e != nil || cid != customerID || mode != "" {
			return out, fmt.Errorf("订单不可通过普通录单调整运费")
		}
		if e := r.EditableTx(ctx, tx, orderID); e != nil {
			return out, e
		}
		var e error
		previous, e = r.SnapshotTx(ctx, tx, orderID)
		if e != nil {
			return out, e
		}
		if previous != nil && previous.Token != expected || previous == nil && expected != "" {
			return out, fmt.Errorf("订单运费已变化，请重新打开")
		}
	}
	if orderID > 0 {
		var pending []byte
		if e := tx.QueryRow(ctx, fmt.Sprintf(`SELECT to_jsonb(o)->'confirmation_pending'->'freight' FROM %s.orders o WHERE id=$1 AND to_jsonb(o)->>'confirmation_status'='pending'`, r.schema), orderID).Scan(&pending); e == nil && len(pending) > 0 && string(pending) != "null" {
			var saved domain.Snapshot
			if e = json.Unmarshal(pending, &saved); e != nil {
				return out, e
			}
			previous = &saved
		} else if e != nil && !errors.Is(e, pgx.ErrNoRows) {
			return out, e
		}
	}
	p, e := r.ResolvePolicy(ctx, tx, customerID, "sale", true)
	if e != nil {
		return out, e
	}
	var t domain.Template
	// Existing quotes retain their frozen pricing rules when their basis changes.
	if previous != nil {
		p = previous.Policy
		if previous.TemplateID > 0 {
			var raw []byte
			e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT snapshot_json FROM %s.shipping_fee_template_versions WHERE template_id=$1 AND revision=$2`, r.schema), previous.TemplateID, previous.TemplateRevision).Scan(&raw)
			if e == nil {
				e = json.Unmarshal(raw, &t)
			}
			if e != nil {
				return out, fmt.Errorf("原订单运费模板版本不可用")
			}
		}
	} else {
		t, e = r.LoadTemplate(ctx, tx, p.TemplateID, true)
		if e != nil {
			t = domain.Template{}
		}
	}
	grams, basis, e := r.orderWeights(ctx, tx, customerID, items, previous)
	if e != nil {
		return out, e
	}
	out, e = domain.QuoteOrder(p, t, d, grams, o, true)
	if e != nil {
		return out, e
	}
	out = AttachWeightBasis(out, basis)
	out.OrderID = orderID
	out.PreviousToken = expected
	out.Token = domain.Token(out)
	return out, nil
}
func (r *Repository) orderWeights(ctx context.Context, q Queryer, cid int64, items []WeightItem, old *domain.Snapshot) (int64, []domain.WeightBasis, error) {
	var basis []domain.WeightBasis
	total := float64(0)
	unknown := false
	for _, i := range items {
		if i.Qty <= 0 || i.Qty > 1000000 {
			return 0, nil, fmt.Errorf("商品数量无效")
		}
		var b *domain.WeightBasis
		if old != nil {
			for _, saved := range old.ItemWeights {
				if saved.ProductID == i.ProductID && saved.BomSpecID == i.BomSpecID && saved.BomVariantID == i.BomVariantID {
					v := saved
					v.Qty = i.Qty
					b = &v
					break
				}
			}
		}
		if b == nil {
			_, rows, e := r.WeightTotals(ctx, q, cid, []WeightItem{i}, true)
			if e != nil {
				return 0, nil, e
			}
			if len(rows) != 1 {
				return 0, nil, fmt.Errorf("商品物流重量无效")
			}
			b = &rows[0]
		}
		basis = append(basis, *b)
		if b.LogisticsUnitWeightG == nil {
			unknown = true
		} else {
			total += *b.LogisticsUnitWeightG * float64(i.Qty)
		}
	}
	if total > 1e9 {
		return 0, nil, fmt.Errorf("商品物流重量超出可处理范围")
	}
	if unknown {
		return 0, basis, nil
	}
	return int64(math.Ceil(math.Round(total*1e6) / 1e6)), basis, nil
}
