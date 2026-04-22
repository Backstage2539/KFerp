package main

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func registerAppRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string, assetDir string) {

	registerShipExportRoutes(e, pool, schema)
	registerRequirementPages(e, pool, schema)
	registerRequirementAPIs(e, pool, schema)
	registerFinishedInventoryPages(e, pool, schema)
	registerMaterialsPages(e, pool, schema)
	registerBomPages(e, pool, schema)
	registerBomAPI(e, pool, schema)
	registerOutsourceSettingsRoutes(e, pool, schema)
	registerStaticFrontendRoutes(e)

	registerUnprodSummaryPages(e, pool, schema)
	registerProducePlanPages(e, pool, schema)
	registerMachineCapacityPages(e, pool, schema)
	registerSenderSettingsPage(e, pool, schema)
	registerProducePlanAllocate(e, pool, schema)
	registerProductionFlowPages(e, pool, schema)
	registerProduceBatchAPI(e, pool, schema)
	registerCompanyStaffPages(e, pool, schema)
	registerCompanyStaffAPI(e, pool, schema)
	registerMobileAuthAPI(e, pool, schema)
	registerAllocationLogPages(e, pool, schema)

	registerCoreRoutes(e, pool, schema)
	registerDocsRoutes(e)
	registerCustomerRoutes(e, pool, schema, assetDir)
	registerProductRoutes(e, pool, schema)
	registerOrderRoutes(e, pool, schema)
}
