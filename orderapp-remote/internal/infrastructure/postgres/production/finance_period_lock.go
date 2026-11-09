package production

import (
	"context"
	"github.com/jackc/pgx/v5"
)

// Serialize cost updates with monthly closing without stopping production.
func lockProductionFinancePeriodTx(ctx context.Context, tx pgx.Tx, schema string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, schema+":finance-period")
	return err
}
