package manufacturing

import (
	"net/http"

	manufacturingapp "orderapp/internal/application/manufacturing"
	"orderapp/internal/interfaces/http/support"

	"github.com/labstack/echo/v4"
)

type Dependencies struct {
	Manufacturing *manufacturingapp.Service
}

func RegisterRoutes(e *echo.Echo, deps Dependencies) {
	e.GET("/process/templates", func(c echo.Context) error {
		return c.Redirect(http.StatusFound, support.PrefixRelativeLocation(c, "/vue-shell?view=processTemplates"))
	})
	e.GET("/process/industry-fields", func(c echo.Context) error {
		return c.Redirect(http.StatusFound, support.PrefixRelativeLocation(c, "/vue-shell?view=industryFieldTemplates"))
	})
	registerAPI(e, deps.Manufacturing)
}
