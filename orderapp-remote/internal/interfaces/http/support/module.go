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
	registerAuthSupportRoutes(e, pool, schema, d.Authz)
	registerRequirementSupportRoutes(e, pool, schema)
	registerUISettingsAPI(e, newPGUISettingsStore(pool, schema), d.Authz)
	registerViewContextSupportRoutes(e, pool, schema, d.Authz)
	registerCoreRoutes(e, pool, schema)
	registerDocsRoutes(e)
}

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	if err := EnsureAuditTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureAppConfigTable(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureRequirementSupportSchema(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureViewContextSupportSchema(ctx, pool, schema); err != nil {
		return err
	}
	return ensureAuthSupportSchema(ctx, pool, schema)
}
