package production

import (
	productionapp "orderapp/internal/application/production"

	"github.com/labstack/echo/v4"
)

type Dependencies struct {
	Production *productionapp.Service
}

func RegisterRoutes(e *echo.Echo, deps Dependencies) {
	registerUnprodSummaryPages(e)
	registerUnprodSummaryAPI(e, deps.Production)
	registerProducePlanPages(e)
	registerMachineCapacityPages(e, deps.Production)
	registerProductionFlowPages(e, deps.Production)
	registerProductionLogPages(e, deps.Production)
	registerProduceBatchAPI(e, deps.Production)
	registerWorkOrderAPI(e, deps.Production)
	registerManufacturingGapAPI(e, deps.Production)
}
