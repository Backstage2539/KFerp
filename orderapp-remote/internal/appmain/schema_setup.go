package appmain

import (
	"context"

	postgresinfra "orderapp/internal/infrastructure/postgres"
	bomhttp "orderapp/internal/interfaces/http/bom"
	cataloghttp "orderapp/internal/interfaces/http/catalog"
	companyhttp "orderapp/internal/interfaces/http/company"
	inventoryhttp "orderapp/internal/interfaces/http/inventory"
	materialshttp "orderapp/internal/interfaces/http/materials"
	productionhttp "orderapp/internal/interfaces/http/production"
	saleshttp "orderapp/internal/interfaces/http/sales"
	supporthttp "orderapp/internal/interfaces/http/support"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ensureAppSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	return postgresinfra.EnsureSchema(ctx, []postgresinfra.SchemaStep{
		{Name: "support", Run: func(ctx context.Context) error { return supporthttp.EnsureSchema(ctx, pool, schema) }},
		{Name: "bom", Run: func(ctx context.Context) error { return bomhttp.EnsureSchema(ctx, pool, schema) }},
		{Name: "catalog", Run: func(ctx context.Context) error { return cataloghttp.EnsureSchema(ctx, pool, schema) }},
		{Name: "materials", Run: func(ctx context.Context) error { return materialshttp.EnsureSchema(ctx, pool, schema) }},
		{Name: "inventory", Run: func(ctx context.Context) error { return inventoryhttp.EnsureSchema(ctx, pool, schema) }},
		{Name: "production", Run: func(ctx context.Context) error { return productionhttp.EnsureSchema(ctx, pool, schema) }},
		{Name: "company", Run: func(ctx context.Context) error { return companyhttp.EnsureSchema(ctx, pool, schema) }},
		{Name: "sales", Run: func(ctx context.Context) error { return saleshttp.EnsureSchema(ctx, pool, schema) }},
	})
}
