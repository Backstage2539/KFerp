package customer

import (
	customerapp "orderapp/internal/application/customer"

	"github.com/labstack/echo/v4"
)

type Dependencies struct {
	Customer *customerapp.Service
	AssetDir string
}

func RegisterRoutes(e *echo.Echo, deps Dependencies) {
	registerCustomerRoutes(e, deps)
}
