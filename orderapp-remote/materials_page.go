package main

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func registerMaterialsPages(e *echo.Echo, _ *pgxpool.Pool, _ string) {
	e.GET("/materials", func(c echo.Context) error {
		target := "/vue-shell?view=materials"
		if q := strings.TrimSpace(c.QueryParam("q")); q != "" {
			target += "&q=" + url.QueryEscape(q)
		}
		return c.Redirect(http.StatusFound, target)
	})
}
