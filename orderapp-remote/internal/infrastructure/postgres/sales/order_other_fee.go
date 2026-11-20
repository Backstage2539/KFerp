package sales

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/sales"
)

// The caller holds the same order row lock used when issuing a customer bill.
func ensureOtherFeeEditableTx(ctx context.Context, tx pgx.Tx, schema string, orderID int64) error {
	var configured bool
	if err := tx.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL AND to_regclass($2) IS NOT NULL", schema+".customer_settlement_lines", schema+".customer_settlement_batches").Scan(&configured); err != nil || !configured {
		return err
	}
	var issued bool
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.customer_settlement_lines l
 JOIN %[1]s.customer_settlement_batches b ON b.id=l.settlement_batch_id
 WHERE l.source_type='order' AND l.source_id=$1 AND l.charge_type='order_goods' AND l.reversed_at IS NULL
 AND b.status IN ('confirmed','paid','settled') AND COALESCE(b.processing_billing_run_id,0)=0)`, schema), orderID).Scan(&issued)
	if err != nil {
		return err
	}
	if issued {
		return app.NewOrderEditConflictError("订单款项已出账，请通过账务调整处理其他费用")
	}
	return nil
}
