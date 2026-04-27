package bom

import (
	bomapp "orderapp/internal/application/bom"

	"github.com/labstack/echo/v4"
)

type Dependencies struct {
	Bom *bomapp.Service
}

func RegisterRoutes(e *echo.Echo, deps Dependencies) {
	registerBomPages(e)
	registerBomAPI(e, deps.Bom)
}
