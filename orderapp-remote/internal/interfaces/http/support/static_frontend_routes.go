package support

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func registerStaticFrontendRoutes(e *echo.Echo) {
	e.GET("/p/:key", func(c echo.Context) error { c.Response().Header().Set("Cache-Control", "no-store"); return c.File("frontend-vue-shell/dist/pages.html") })
	// Caddy strips the /app/ prefix before proxying to orderapp.
	e.Static("/vue-shell/assets", "frontend-vue-shell/dist/assets")
	e.GET("/vue-shell", func(c echo.Context) error {
		return c.File("frontend-vue-shell/dist/index.html")
	})
	e.GET("/vue-shell/*", func(c echo.Context) error {
		return c.File("frontend-vue-shell/dist/index.html")
	})
	e.GET("/produce/unproduced", func(c echo.Context) error {
		target := "/vue-shell?view=producePlan"
		if raw := c.QueryString(); raw != "" {
			target += "&" + raw
		}
		return c.Redirect(http.StatusFound, PrefixRelativeLocation(c, target))
	})
}
