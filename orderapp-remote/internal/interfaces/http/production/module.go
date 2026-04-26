package production

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func RegisterRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	registerFinishedInventoryPages(e, pool, schema)
	registerMaterialsPages(e, pool, schema)
	registerMaterialsAPI(e, pool, schema)
	registerBomPages(e, pool, schema)
	registerBomAPI(e, pool, schema)
	registerUnprodSummaryPages(e, pool, schema)
	registerUnprodSummaryAPI(e, pool, schema)
	registerProducePlanPages(e, pool, schema)
	registerMachineCapacityPages(e, pool, schema)
	registerProducePlanAllocate(e, pool, schema)
	registerProductionFlowPages(e, pool, schema)
	registerProductionLogPages(e, pool, schema)
	registerProduceBatchAPI(e, pool, schema)
	registerAllocationLogPages(e, pool, schema)
	registerProductRoutes(e, pool, schema)
}

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	if err := ensureBomTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureProductPricingColumns(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureFinishedInventoryTable(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureFinishedAllocationLogTable(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureMaterialTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureStockLedgerTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureBagSpecMappingTable(ctx, pool, schema); err != nil {
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
