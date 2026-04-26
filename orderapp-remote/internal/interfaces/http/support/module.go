package support

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func RegisterRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	registerStaticFrontendRoutes(e)
	registerRequirementPages(e, pool, schema)
	registerRequirementAPIs(e, pool, schema)
	registerMobileAuthAPI(e, pool, schema)
	registerCoreRoutes(e, pool, schema)
	registerDocsRoutes(e)
}

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	if err := EnsureAuditTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureReqTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := seedReqWorkflowA(ctx, pool, schema); err != nil {
		return err
	}
	return ensureMobileAuthTables(ctx, pool, schema)
}
