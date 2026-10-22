package sales

import (
	"context"
	"github.com/jackc/pgx/v5"
	salesapp "orderapp/internal/application/sales"
)

// These transaction helpers let the dedicated fulfillment editor share the
// ordinary editor's concurrency, execution guards and document invalidation.
func CurrentOrderEditRevisionTx(ctx context.Context, tx pgx.Tx, schema string, id int64) (string, error) {
	return currentOrderEditRevisionTx(ctx, tx, schema, id)
}
func LoadOrderEditStateTx(ctx context.Context, tx pgx.Tx, schema string, id int64, lock bool) (salesapp.OrderEditState, error) {
	return loadOrderEditState(ctx, tx, schema, id, lock)
}
func InvalidateConfirmedOrderDocumentsTx(ctx context.Context, tx pgx.Tx, schema string, id int64) error {
	return invalidateConfirmedOrderDocumentsTx(ctx, tx, schema, id)
}

func ProtectOrderReceiptsTx(ctx context.Context, tx pgx.Tx, schema string, id int64, c *salesapp.SaveOrderCommand, shipping, total float64) error {
	return protectFreightReceiptsTx(ctx, tx, schema, id, c, shipping, total)
}
