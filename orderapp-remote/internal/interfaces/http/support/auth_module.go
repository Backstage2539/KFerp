package support

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type ERPWorkbenchLoginEligibility interface {
	RequireERPWorkbenchLogin(context.Context, int64) error
}

func registerAuthSupportRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string, authz AuthzService, eligibility ERPWorkbenchLoginEligibility) {
	registerAuthzAPI(e, authz)
	registerMobileAuthAPI(e, pool, schema, authz, eligibility)
}

func ensureAuthSupportSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	return ensureMobileAuthTables(ctx, pool, schema)
}
