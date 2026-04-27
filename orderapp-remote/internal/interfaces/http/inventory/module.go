package inventory

import (
	inventoryapp "orderapp/internal/application/inventory"

	"github.com/labstack/echo/v4"
)

type Dependencies struct {
	Inventory *inventoryapp.Service
}

func RegisterRoutes(e *echo.Echo, deps Dependencies) {
	registerFinishedInventoryPages(e, deps.Inventory)
	registerAllocationLogPages(e, deps.Inventory)
}
