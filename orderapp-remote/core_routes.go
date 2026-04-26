package main

import (
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func registerCoreRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	h := coreHandler{pool: pool, schema: schema}

	e.GET("/", h.home)
	// legacy aliases for old entrypoints
	e.GET("/order-list", h.orderListAlias)
	e.GET("/order-detail/:id", h.orderDetailAlias)
	e.GET("/login", h.login)

	e.GET("/audit", h.audit)
	e.GET("/api/audit", h.auditAPI)
}

type coreHandler struct {
	pool   *pgxpool.Pool
	schema string
}

func (h coreHandler) home(c echo.Context) error {
	return c.Redirect(http.StatusSeeOther, "/orders")
}

func (h coreHandler) orderListAlias(c echo.Context) error {
	return c.Redirect(http.StatusSeeOther, "/orders")
}

func (h coreHandler) orderDetailAlias(c echo.Context) error {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return c.Redirect(http.StatusSeeOther, "/orders")
	}
	return c.Redirect(http.StatusSeeOther, "/orders/"+id)
}

func (h coreHandler) login(c echo.Context) error {
	return c.Render(http.StatusOK, "login.html", map[string]any{})
}

func (h coreHandler) audit(c echo.Context) error {
	if strings.TrimSpace(c.QueryParam("legacy")) != "1" {
		return vueShellRedirect(c, "audit")
	}

	data := AuditPageData{
		From:       strings.TrimSpace(c.QueryParam("from")),
		To:         strings.TrimSpace(c.QueryParam("to")),
		Q:          strings.TrimSpace(c.QueryParam("q")),
		EntityType: strings.TrimSpace(c.QueryParam("type")),
	}
	rows, err := fetchAuditPage(c.Request().Context(), h.pool, h.schema, data.From, data.To, data.Q, data.EntityType, 200)
	if err != nil {
		data.Error = err.Error()
	} else {
		data.Rows = rows
	}
	return c.Render(http.StatusOK, "audit.html", data)
}

func (h coreHandler) auditAPI(c echo.Context) error {
	limit := intParam(c, "limit", 200)
	if limit <= 0 {
		limit = 200
	}
	if limit > 500 {
		limit = 500
	}
	data := AuditPageData{
		From:       strings.TrimSpace(c.QueryParam("from")),
		To:         strings.TrimSpace(c.QueryParam("to")),
		Q:          strings.TrimSpace(c.QueryParam("q")),
		EntityType: strings.TrimSpace(c.QueryParam("type")),
	}
	rows, err := fetchAuditPage(c.Request().Context(), h.pool, h.schema, data.From, data.To, data.Q, data.EntityType, limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"rows": rows,
		"filters": map[string]any{
			"from": data.From,
			"to":   data.To,
			"q":    data.Q,
			"type": data.EntityType,
		},
	})
}
