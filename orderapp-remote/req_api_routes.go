package main

import (
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func registerRequirementAPIs(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	e.GET("/api/req/next_code", func(c echo.Context) error {
		typ := strings.TrimSpace(c.QueryParam("type"))
		code, err := nextReqCodeByType(c.Request().Context(), pool, schema, typ)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "code": code, "type": typ})
	})
}
