package production

import (
	"context"
	messagecenterapp "orderapp/internal/application/messagecenter"
	productionapp "orderapp/internal/application/production"
	stockapp "orderapp/internal/application/stock"

	"github.com/labstack/echo/v4"
)

type Dependencies struct {
	Production    *productionapp.Service
	Stock         *stockapp.Service
	MessageCenter MessagePublisher
}

type MessagePublisher interface {
	Publish(context.Context, messagecenterapp.PublishCommand) (int64, error)
}

func RegisterRoutes(e *echo.Echo, deps Dependencies) {
	registerUnprodSummaryPages(e)
	registerUnprodSummaryAPI(e, deps.Production)
	registerProducePlanPages(e)
	registerMachineCapacityPages(e, deps.Production)
	registerProductionPlanAPI(e, deps.Production)
	registerProductionScheduleAPI(e, deps.Production)
	registerProductionWorkstationAPI(e, deps.Production)
	registerStockEntryAPI(e, deps.Production, deps.Stock)
	registerProductionFlowPages(e, deps.Production, deps.MessageCenter)
	registerProductionLogPages(e, deps.Production)
	registerProduceBatchAPI(e, deps.Production)
	registerWorkOrderAPI(e, deps.Production, deps.Stock)
	registerManufacturingGapAPI(e, deps.Production)
}
