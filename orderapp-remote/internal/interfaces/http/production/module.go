package production

import (
	"context"
	messagecenterapp "orderapp/internal/application/messagecenter"
	productionapp "orderapp/internal/application/production"

	"github.com/labstack/echo/v4"
)

type Dependencies struct {
	Production    *productionapp.Service
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
	registerStockEntryAPI(e, deps.Production)
	registerProductionFlowPages(e, deps.Production, deps.MessageCenter)
	registerProductionLogPages(e, deps.Production)
	registerProduceBatchAPI(e, deps.Production)
	registerWorkOrderAPI(e, deps.Production)
	registerManufacturingGapAPI(e, deps.Production)
}
