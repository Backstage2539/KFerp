package materials

import (
	"net/http"
	"net/url"
	support "orderapp/internal/interfaces/http/support"
	"strings"

	"github.com/labstack/echo/v4"
)

func registerMaterialsPages(e *echo.Echo) {
	e.GET("/materials", func(c echo.Context) error {
		target := "/vue-shell?view=materials"
		if q := strings.TrimSpace(c.QueryParam("q")); q != "" {
			target += "&q=" + url.QueryEscape(q)
		}
		return c.Redirect(http.StatusFound, support.PrefixRelativeLocation(c, target))
	})
}
