package support

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func registerViewContextSupportRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string, authz AuthzService) {
	registerViewContextAPI(e, pool, schema, authz)
}

func ensureViewContextSupportSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	return ensureViewContextPresetTables(ctx, pool, schema)
}
