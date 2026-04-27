package materials

import (
	materialsapp "orderapp/internal/application/materials"

	"github.com/labstack/echo/v4"
)

type Dependencies struct {
	Materials *materialsapp.Service
}

func RegisterRoutes(e *echo.Echo, deps Dependencies) {
	registerMaterialsPages(e)
	registerMaterialsAPI(e, deps.Materials)
}
