package catalog

import (
	catalogapp "orderapp/internal/application/catalog"

	"github.com/labstack/echo/v4"
)

type Dependencies struct {
	Catalog *catalogapp.Service
}

func RegisterRoutes(e *echo.Echo, deps Dependencies) {
	registerProductRoutes(e, deps.Catalog)
}
