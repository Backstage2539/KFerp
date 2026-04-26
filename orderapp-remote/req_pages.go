package main

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func registerRequirementPages(e *echo.Echo, _ *pgxpool.Pool, _ string) {
	for _, route := range []struct {
		path string
		view string
	}{
		{path: "/req/product", view: "reqProduct"},
		{path: "/req/dev", view: "reqDev"},
		{path: "/req/unit", view: "reqUnit"},
		{path: "/req/api", view: "reqApi"},
		{path: "/req/review", view: "reqReview"},
	} {
		path := route.path
		view := route.view
		e.GET(path, func(c echo.Context) error {
			return vueShellRedirect(c, view)
		})
	}
}
