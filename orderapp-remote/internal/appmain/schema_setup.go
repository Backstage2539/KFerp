package appmain

import (
	"context"

	postgresinfra "orderapp/internal/infrastructure/postgres"
	postgresbom "orderapp/internal/infrastructure/postgres/bom"
	postgrescatalog "orderapp/internal/infrastructure/postgres/catalog"
	postgrescompany "orderapp/internal/infrastructure/postgres/company"
	postgrescosting "orderapp/internal/infrastructure/postgres/costing"
	postgresinventory "orderapp/internal/infrastructure/postgres/inventory"
	postgresmaterials "orderapp/internal/infrastructure/postgres/materials"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
	supporthttp "orderapp/internal/interfaces/http/support"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ensureAppSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	return postgresinfra.EnsureSchema(ctx, []postgresinfra.SchemaStep{
		{Name: "support", Run: func(ctx context.Context) error { return supporthttp.EnsureSchema(ctx, pool, schema) }},
		{Name: "bom", Run: func(ctx context.Context) error { return postgresbom.EnsureSchema(ctx, pool, schema) }},
		{Name: "catalog", Run: func(ctx context.Context) error { return postgrescatalog.EnsureSchema(ctx, pool, schema) }},
		{Name: "costing", Run: func(ctx context.Context) error { return postgrescosting.EnsureSchema(ctx, pool, schema) }},
		{Name: "materials", Run: func(ctx context.Context) error { return postgresmaterials.EnsureSchema(ctx, pool, schema) }},
		{Name: "inventory", Run: func(ctx context.Context) error { return postgresinventory.EnsureSchema(ctx, pool, schema) }},
		{Name: "production", Run: func(ctx context.Context) error { return postgresproduction.EnsureSchema(ctx, pool, schema) }},
		{Name: "company", Run: func(ctx context.Context) error { return postgrescompany.EnsureSchema(ctx, pool, schema) }},
		{Name: "sales", Run: func(ctx context.Context) error { return postgressales.EnsureSchema(ctx, pool, schema) }},
	})
}
