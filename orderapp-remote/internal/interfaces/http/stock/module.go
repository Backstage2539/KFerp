package stock

import (
	stockapp "orderapp/internal/application/stock"

	"github.com/labstack/echo/v4"
)

type Dependencies struct {
	Stock *stockapp.Service
}

func RegisterRoutes(e *echo.Echo, deps Dependencies) {
	registerStockPages(e)
	registerStockAPI(e, deps.Stock)
}
