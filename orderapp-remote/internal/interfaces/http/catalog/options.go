package catalog

import (
	"context"

	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Option = postgresinfra.Option
type ProductTierOption = postgresinfra.ProductTierOption
type ProductOption = postgresinfra.ProductOption

type APIOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func FetchOptions(ctx context.Context, pool *pgxpool.Pool, sqlstr string) ([]Option, error) {
	return postgresinfra.FetchOptions(ctx, pool, sqlstr)
}

func FetchProducts(ctx context.Context, pool *pgxpool.Pool, schema string) ([]ProductOption, error) {
	return postgresinfra.FetchProducts(ctx, pool, schema)
}
