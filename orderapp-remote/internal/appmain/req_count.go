package appmain

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func countReqRows(ctx context.Context, pool *pgxpool.Pool, schema, table string) (int, error) {
	q := fmt.Sprintf(`SELECT count(*) FROM %s.%s`, schema, table)
	var n int
	if err := pool.QueryRow(ctx, q).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}
