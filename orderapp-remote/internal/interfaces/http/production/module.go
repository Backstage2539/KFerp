package production

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func RegisterRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	registerUnprodSummaryPages(e, pool, schema)
	registerUnprodSummaryAPI(e, pool, schema)
	registerProducePlanPages(e, pool, schema)
	registerMachineCapacityPages(e, pool, schema)
	registerProductionFlowPages(e, pool, schema)
	registerProductionLogPages(e, pool, schema)
	registerProduceBatchAPI(e, pool, schema)
}

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	if err := ensureStockLedgerTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureProduceBatchTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureMachineCapacityTable(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureProductionRunTable(ctx, pool, schema); err != nil {
		return err
	}
	return ensureProductionLogTable(ctx, pool, schema)
}
