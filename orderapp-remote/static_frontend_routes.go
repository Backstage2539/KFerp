package main

import "github.com/labstack/echo/v4"

func registerStaticFrontendRoutes(e *echo.Echo) {
	// Caddy strips the /app/ prefix before proxying to orderapp.
	e.Static("/bom-react/assets", "frontend/dist/assets")
	e.GET("/bom-react", func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		c.Response().Header().Set("Pragma", "no-cache")
		c.Response().Header().Set("Expires", "0")
		return c.File("frontend/dist/index.html")
	})
	e.GET("/bom-react/*", func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		c.Response().Header().Set("Pragma", "no-cache")
		c.Response().Header().Set("Expires", "0")
		return c.File("frontend/dist/index.html")
	})

	e.Static("/vue-shell/assets", "frontend-vue-shell/dist/assets")
	e.GET("/vue-shell", func(c echo.Context) error {
		return c.File("frontend-vue-shell/dist/index.html")
	})
	e.GET("/vue-shell/*", func(c echo.Context) error {
		return c.File("frontend-vue-shell/dist/index.html")
	})
}
