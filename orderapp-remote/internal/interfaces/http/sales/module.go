package sales

import (
	salesapp "orderapp/internal/application/sales"

	"github.com/labstack/echo/v4"
)

type Dependencies struct {
	Sales *salesapp.Service
}

func RegisterRoutes(e *echo.Echo, deps Dependencies) {
	registerShipExportRoutes(e, deps.Sales)
	registerOutsourceSettingsRoutes(e, deps.Sales)
	registerSenderSettingsPage(e, deps.Sales)
	registerOrderRoutes(e, deps.Sales)
	registerOrderAPI(e, deps.Sales)
}
