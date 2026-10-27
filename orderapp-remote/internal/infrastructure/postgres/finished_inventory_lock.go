package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// LockFinishedInventoryIdentityTx serializes finished-stock writers and
// batch-based adjustments for one product/spec identity across warehouses.
// Call it before taking inventory-summary or stock-batch locks.
func LockFinishedInventoryIdentityTx(ctx context.Context, tx pgx.Tx, schema string, productID, bomSpecID, specG int64) error {
	identity := fmt.Sprintf("finished_inventory:%s:%d:%d:%d", schema, productID, bomSpecID, specG)
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, identity)
	return err
}
