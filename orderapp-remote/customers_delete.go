package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// soft-delete: set active=false (keep history). If you need hard delete later, add a separate admin action.
func deleteCustomer(ctx context.Context, pool *pgxpool.Pool, schema, actor string, id int64) error {
	q := fmt.Sprintf(`UPDATE %s.customers SET active=false, updated_at=$2 WHERE id=$1`, schema)
	if _, err := pool.Exec(ctx, q, id, time.Now()); err != nil {
		return err
	}
	auditInsert(ctx, pool, schema, actor, "customer", &id, "delete", nil, nil, nil, nil)
	return nil
}
