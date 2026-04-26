package materials

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func RegisterRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	registerMaterialsPages(e, pool, schema)
	registerMaterialsAPI(e, pool, schema)
}

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	return ensureMaterialTables(ctx, pool, schema)
}
