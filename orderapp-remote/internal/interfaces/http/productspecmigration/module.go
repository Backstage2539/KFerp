package productspecmigration

import (
	productspecmigrationapp "orderapp/internal/application/productspecmigration"

	"github.com/labstack/echo/v4"
)

type Dependencies struct {
	Migration *productspecmigrationapp.Service
}

func RegisterRoutes(e *echo.Echo, deps Dependencies) {
	registerMigrationAPI(e, deps.Migration)
}
