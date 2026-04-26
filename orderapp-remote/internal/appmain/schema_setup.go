package appmain

import (
	"context"

	postgresinfra "orderapp/internal/infrastructure/postgres"
	companyhttp "orderapp/internal/interfaces/http/company"
	productionhttp "orderapp/internal/interfaces/http/production"
	saleshttp "orderapp/internal/interfaces/http/sales"
	supporthttp "orderapp/internal/interfaces/http/support"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ensureAppSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	return postgresinfra.EnsureSchema(ctx, []postgresinfra.SchemaStep{
		{Name: "support", Run: func(ctx context.Context) error { return supporthttp.EnsureSchema(ctx, pool, schema) }},
		{Name: "production", Run: func(ctx context.Context) error { return productionhttp.EnsureSchema(ctx, pool, schema) }},
		{Name: "company", Run: func(ctx context.Context) error { return companyhttp.EnsureSchema(ctx, pool, schema) }},
		{Name: "sales", Run: func(ctx context.Context) error { return saleshttp.EnsureSchema(ctx, pool, schema) }},
	})
}
