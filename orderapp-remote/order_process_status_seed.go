package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ensureOrderProcessStatuses keeps required process statuses available in DB.
func ensureOrderProcessStatuses(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
		INSERT INTO %s.order_process_statuses(name, sort, active)
		SELECT $1, $2, true
		WHERE NOT EXISTS (
			SELECT 1 FROM %s.order_process_statuses WHERE name=$1
		)
	`, schema, schema)
	_, err := pool.Exec(ctx, q, "生产完成", 35)
	return err
}
