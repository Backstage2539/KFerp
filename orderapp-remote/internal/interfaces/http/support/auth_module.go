package support

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func registerAuthSupportRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string, authz AuthzService) {
	registerAuthzAPI(e, authz)
	registerMobileAuthAPI(e, pool, schema, authz)
}

func ensureAuthSupportSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	return ensureMobileAuthTables(ctx, pool, schema)
}
