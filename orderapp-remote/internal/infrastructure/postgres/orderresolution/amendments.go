package orderresolution

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	infra "orderapp/internal/infrastructure/postgres"
)

// AttachAmendmentTx is called only after a new order has been staged for review.
// Locking the draft prevents two copies from being submitted from the same revision.
func AttachAmendmentTx(ctx context.Context, tx pgx.Tx, schema string, draftID, newOrderID int64, actor string) error {
	var source, customer int64
	var state string
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT a.source_order_id,a.customer_id,a.status FROM %[1]s.order_resolution_amendments a JOIN %[1]s.orders o ON o.id=a.source_order_id WHERE a.id=$1 AND o.is_void AND o.resolution_status='cancelled' FOR UPDATE OF a,o`, schema), draftID).Scan(&source, &customer, &state)
	if err != nil {
		return fmt.Errorf("修订草稿不存在或原单尚未完成取消")
	}
	if state != "draft" {
		return fmt.Errorf("此修订草稿已生成新单，请从原单处理记录查看")
	}
	var valid bool
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT customer_id=$2 AND confirmation_required AND confirmation_status='pending' AND NOT is_void AND COALESCE(amended_from_order_id,0)=0 AND COALESCE(prepayment_amount,0)=0 AND COALESCE(payment_goods_amount,0)=0 AND COALESCE(payment_shipping_amount,0)=0 AND COALESCE(ship_tracking_no,'')='' FROM %s.orders WHERE id=$1 FOR UPDATE`, schema), newOrderID, customer).Scan(&valid); err != nil {
		return err
	}
	if !valid || source == newOrderID {
		return fmt.Errorf("修订单必须属于原客户、重新审核，且不能继承收款或发货记录")
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET amended_from_order_id=$2 WHERE id=$1`, schema), newOrderID, source); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_resolution_amendments SET new_order_id=$2,status='submitted' WHERE id=$1`, schema), draftID, newOrderID); err != nil {
		return err
	}
	return infra.AuditInsertTx(ctx, tx, schema, actor, "order", &newOrderID, "amend", nil, nil, nil, infra.AuditMeta{"source_order_id": source, "amendment_id": draftID})
}
