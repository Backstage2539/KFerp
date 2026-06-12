package support

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func registerRequirementSupportRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	registerRequirementPages(e, pool, schema)
	registerRequirementAPIs(e, pool, schema)
}

func ensureRequirementSupportSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	if err := ensureReqTables(ctx, pool, schema); err != nil {
		return err
	}
	return seedReqWorkflowA(ctx, pool, schema)
}
