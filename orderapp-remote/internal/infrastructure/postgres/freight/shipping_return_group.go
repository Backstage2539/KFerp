package freight

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	domain "orderapp/internal/domain/freight"
	"orderapp/internal/domain/shippingreturn"
)

// ConfirmShippingReturnCostsTx posts a cost per parcel and a single adjustment
// for the order. The shipment transaction owns the freight-posting and order locks.
func (r *Repository) ConfirmShippingReturnCostsTx(ctx context.Context, tx pgx.Tx, oid, carrier int64, rows []shippingreturn.Row, actor string) (string, error) {
	if len(rows) == 0 {
		return "", fmt.Errorf("请选择寄件行")
	}
	keyFor := func(id int64) string { return fmt.Sprintf("shipping-return:%d", id) }
	if len(rows) == 1 {
		v := rows[0]
		return r.ConfirmActualCostTx(ctx, tx, oid, carrier, v.TrackingNo, v.ActualCents, keyFor(v.ID), actor)
	}
	groupID := rows[0].ID
	var total int64
	byKey := make(map[string]shippingreturn.Row, len(rows))
	tracks := make(map[string]bool, len(rows))
	for _, v := range rows {
		key := keyFor(v.ID)
		if _, duplicate := byKey[key]; duplicate || v.ID <= 0 || v.TrackingNo == "" || tracks[v.TrackingNo] {
			return "", fmt.Errorf("寄件行或运单号无效或重复")
		}
		if v.ActualCents < 0 || v.ActualCents > domain.MaxCents-total {
			return "", fmt.Errorf("实际运费无效或合计超出范围")
		}
		total += v.ActualCents
		groupID = min(groupID, v.ID)
		byKey[key] = v
		tracks[v.TrackingNo] = true
	}

	// Replays must agree with the whole financial posting, including amounts.
	// A previous fee for this order cannot be attributed to a new set of parcels.
	posted, err := tx.Query(ctx, fmt.Sprintf(`SELECT e.request_key,coalesce(e.shipping_return_group_id,0),s.carrier_id,s.tracking_no,e.actual_cents
	 FROM %[1]s.freight_actual_entries e JOIN %[1]s.freight_actual_shipments s ON s.id=e.shipment_id WHERE s.order_id=$1`, r.schema), oid)
	if err != nil {
		return "", err
	}
	defer posted.Close()
	n := 0
	for posted.Next() {
		var key, tracking string
		var priorGroup, priorCarrier, cost int64
		if err := posted.Scan(&key, &priorGroup, &priorCarrier, &tracking, &cost); err != nil {
			return "", err
		}
		v, ok := byKey[key]
		if !ok || priorGroup != groupID || priorCarrier != carrier || v.TrackingNo != tracking || v.ActualCents != cost {
			return "", fmt.Errorf("订单已有不同的实际运费记录，请核对整组寄件")
		}
		n++
	}
	if err := posted.Err(); err != nil {
		return "", err
	}
	if n > 0 {
		if n != len(rows) {
			return "", fmt.Errorf("订单已有不同的寄件组合，请核对整组寄件")
		}
		return "已处理", nil
	}

	posting, err := r.prepareActualPosting(ctx, tx, oid, carrier, total, actor)
	if err != nil {
		return "", err
	}
	posting.groupID = groupID
	for _, v := range rows {
		var delta int64
		if v.ID == groupID {
			delta = posting.decision.TargetCents - posting.state.Current
		}
		if err := r.recordActualCostTx(ctx, tx, posting, v.TrackingNo, v.ActualCents, keyFor(v.ID), delta); err != nil {
			return "", err
		}
	}
	return posting.decision.Reason, nil
}
