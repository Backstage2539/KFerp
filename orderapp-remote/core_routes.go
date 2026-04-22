package main

import (
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func registerCoreRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	e.GET("/", func(c echo.Context) error {
		return c.Redirect(http.StatusSeeOther, "/orders")
	})
	// legacy aliases for old entrypoints
	e.GET("/order-list", func(c echo.Context) error {
		return c.Redirect(http.StatusSeeOther, "/orders")
	})
	e.GET("/order-detail/:id", func(c echo.Context) error {
		id := strings.TrimSpace(c.Param("id"))
		if id == "" {
			return c.Redirect(http.StatusSeeOther, "/orders")
		}
		return c.Redirect(http.StatusSeeOther, "/orders/"+id)
	})
	e.GET("/login", func(c echo.Context) error {
		return c.Render(http.StatusOK, "login.html", map[string]any{})
	})

	// Products
	e.GET("/audit", func(c echo.Context) error {
		data := AuditPageData{
			From:       strings.TrimSpace(c.QueryParam("from")),
			To:         strings.TrimSpace(c.QueryParam("to")),
			Q:          strings.TrimSpace(c.QueryParam("q")),
			EntityType: strings.TrimSpace(c.QueryParam("type")),
		}
		rows, err := fetchAuditPage(c.Request().Context(), pool, schema, data.From, data.To, data.Q, data.EntityType, 200)
		if err != nil {
			data.Error = err.Error()
		} else {
			data.Rows = rows
		}
		return c.Render(http.StatusOK, "audit.html", data)
	})

}
