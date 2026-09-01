package appmain

import (
	"context"

	postgresinfra "orderapp/internal/infrastructure/postgres"
	postgresauthz "orderapp/internal/infrastructure/postgres/authz"
	postgresbom "orderapp/internal/infrastructure/postgres/bom"
	postgrescatalog "orderapp/internal/infrastructure/postgres/catalog"
	postgrescompany "orderapp/internal/infrastructure/postgres/company"
	postgrescontracts "orderapp/internal/infrastructure/postgres/contracts"
	postgrescore "orderapp/internal/infrastructure/postgres/core"
	postgrescosting "orderapp/internal/infrastructure/postgres/costing"
	postgrescustomer "orderapp/internal/infrastructure/postgres/customer"
	postgrescustomerfulfillment "orderapp/internal/infrastructure/postgres/customerfulfillment"
	postgrescustomerportal "orderapp/internal/infrastructure/postgres/customerportal"
	postgresfinance "orderapp/internal/infrastructure/postgres/finance"
	postgresinventory "orderapp/internal/infrastructure/postgres/inventory"
	postgresmanufacturing "orderapp/internal/infrastructure/postgres/manufacturing"
	postgresmaterials "orderapp/internal/infrastructure/postgres/materials"
	postgresmessagecenter "orderapp/internal/infrastructure/postgres/messagecenter"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"
	postgresproductspecmigration "orderapp/internal/infrastructure/postgres/productspecmigration"
	postgrespurchase "orderapp/internal/infrastructure/postgres/purchase"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
	postgresstock "orderapp/internal/infrastructure/postgres/stock"
	supporthttp "orderapp/internal/interfaces/http/support"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ensureAppSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	return postgresinfra.EnsureSchema(ctx, []postgresinfra.SchemaStep{
		{Name: "company", Run: func(ctx context.Context) error { return postgrescompany.EnsureSchema(ctx, pool, schema) }},
		{Name: "core", Run: func(ctx context.Context) error { return postgrescore.EnsureSchema(ctx, pool, schema) }},
		{Name: "customer", Run: func(ctx context.Context) error { return postgrescustomer.EnsureSchema(ctx, pool, schema) }},
		{Name: "customerportal", Run: func(ctx context.Context) error { return postgrescustomerportal.EnsureSchema(ctx, pool, schema) }},
		{Name: "customerfulfillment", Run: func(ctx context.Context) error { return postgrescustomerfulfillment.EnsureSchema(ctx, pool, schema) }},
		{Name: "messagecenter", Run: func(ctx context.Context) error { return postgresmessagecenter.EnsureSchema(ctx, pool, schema) }},
		{Name: "support", Run: func(ctx context.Context) error { return supporthttp.EnsureSchema(ctx, pool, schema) }},
		{Name: "authz", Run: func(ctx context.Context) error { return postgresauthz.EnsureSchema(ctx, pool, schema) }},
		{Name: "materials", Run: func(ctx context.Context) error { return postgresmaterials.EnsureSchema(ctx, pool, schema) }},
		{Name: "bom", Run: func(ctx context.Context) error { return postgresbom.EnsureSchema(ctx, pool, schema) }},
		{Name: "catalog", Run: func(ctx context.Context) error { return postgrescatalog.EnsureSchema(ctx, pool, schema) }},
		{Name: "costing", Run: func(ctx context.Context) error { return postgrescosting.EnsureSchema(ctx, pool, schema) }},
		{Name: "finance", Run: func(ctx context.Context) error { return postgresfinance.EnsureSchema(ctx, pool, schema) }},
		{Name: "stock", Run: func(ctx context.Context) error { return postgresstock.EnsureSchema(ctx, pool, schema) }},
		{Name: "purchase", Run: func(ctx context.Context) error { return postgrespurchase.EnsureSchema(ctx, pool, schema) }},
		{Name: "inventory", Run: func(ctx context.Context) error { return postgresinventory.EnsureSchema(ctx, pool, schema) }},
		{Name: "manufacturing", Run: func(ctx context.Context) error { return postgresmanufacturing.EnsureSchema(ctx, pool, schema) }},
		{Name: "production", Run: func(ctx context.Context) error { return postgresproduction.EnsureSchema(ctx, pool, schema) }},
		{Name: "sales", Run: func(ctx context.Context) error { return postgressales.EnsureSchema(ctx, pool, schema) }},
		{Name: "contracts", Run: func(ctx context.Context) error { return postgrescontracts.EnsureSchema(ctx, pool, schema) }},
		{Name: "product-bom-spec-migration", Run: func(ctx context.Context) error {
			return postgresproductspecmigration.EnsureAuthorityProjection(ctx, pool, schema)
		}},
		{Name: "serial-id-sequences", Run: func(ctx context.Context) error { return postgresinfra.SyncSerialIDSequences(ctx, pool, schema) }},
	})
}
