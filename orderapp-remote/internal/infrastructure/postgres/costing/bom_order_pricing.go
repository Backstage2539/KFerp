package costing

import (
	"context"
	"github.com/jackc/pgx/v5"
	postgresinfra "orderapp/internal/infrastructure/postgres"
)

type costingReader interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (r Repository) readDB(ctx context.Context) costingReader {
	if tx, ok := postgresinfra.TransactionFromContext(ctx); ok {
		return tx
	}
	return r.pool
}
