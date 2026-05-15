package sales

import (
	"context"
	messagecenterapp "orderapp/internal/application/messagecenter"
	salesapp "orderapp/internal/application/sales"

	"github.com/labstack/echo/v4"
)

type Dependencies struct {
	Sales         *salesapp.Service
	MessageCenter MessagePublisher
	AssetDir      string
}

type MessagePublisher interface {
	Publish(context.Context, messagecenterapp.PublishCommand) (int64, error)
}

func RegisterRoutes(e *echo.Echo, deps Dependencies) {
	registerShipExportRoutes(e, deps.Sales)
	registerOutsourceSettingsRoutes(e, deps.Sales)
	registerSenderSettingsPage(e, deps.Sales)
	registerOrderRoutes(e, deps.Sales)
	registerOrderAPI(e, deps.Sales, deps.MessageCenter)
	registerOrderShippingExcelRoutes(e, deps.Sales, deps.MessageCenter)
	registerSalesOrderSettingsRoutes(e, deps.Sales, deps.AssetDir)
	registerSalesOrderDocumentRoutes(e, deps.Sales)
	registerDeliveryNoteDocumentRoutes(e, deps.Sales)
	registerExternalShareResourceRoutes(e, deps.Sales)
	registerOrderInvoiceRoutes(e, deps.Sales, deps.AssetDir)
}
