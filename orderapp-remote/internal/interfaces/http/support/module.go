package support

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type Dependencies struct {
	Authz AuthzService
}

func RegisterRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string, deps ...Dependencies) {
	var d Dependencies
	if len(deps) > 0 {
		d = deps[0]
	}
	registerStaticFrontendRoutes(e)
	registerAuthzAPI(e, d.Authz)
	registerRequirementPages(e, pool, schema)
	registerRequirementAPIs(e, pool, schema)
	registerMobileAuthAPI(e, pool, schema, d.Authz)
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
