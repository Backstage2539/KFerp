package bom

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func RegisterRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	registerBomPages(e, pool, schema)
	registerBomAPI(e, pool, schema)
}

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	if err := ensureBomTables(ctx, pool, schema); err != nil {
		return err
	}
	return ensureBagSpecMappingTable(ctx, pool, schema)
}
