package production

import (
	"net/http"
	support "orderapp/internal/interfaces/http/support"

	"github.com/labstack/echo/v4"
)

func registerProducePlanPages(e *echo.Echo) {
	redirect := func(c echo.Context) error {
		target := "/vue-shell?view=producePlan"
		if raw := c.QueryString(); raw != "" {
			target += "&" + raw
		}
		return c.Redirect(http.StatusFound, support.PrefixRelativeLocation(c, target))
	}
	e.GET("/produce/plan", redirect)
	e.GET("/app/produce/plan", redirect)
}
