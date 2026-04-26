package sales

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func RegisterRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	registerShipExportRoutes(e, pool, schema)
	registerOutsourceSettingsRoutes(e, pool, schema)
	registerSenderSettingsPage(e, pool, schema)
	registerOrderRoutes(e, pool, schema)
	registerOrderAPI(e, pool, schema)
}

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	if err := ensureOrderProcessStatuses(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureSenderSettingsTable(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureOutsourceFeeColumns(ctx, pool, schema); err != nil {
		return err
	}
	return ensureOutsourceTemplateTables(ctx, pool, schema)
}
