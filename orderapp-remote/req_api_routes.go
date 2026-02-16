package main

import (
	"net/http"
	"os"
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

	// Migration APIs (idempotent)
	e.POST("/api/migrate/requirements", func(c echo.Context) error {
		bs, err := os.ReadFile("/app/docs/REQUIREMENTS.md")
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
		}
		n, err := migrateRequirements(c.Request().Context(), pool, schema, string(bs))
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "inserted": n})
	})
	e.POST("/api/migrate/acceptance_tests", func(c echo.Context) error {
		bs, err := os.ReadFile("/app/docs/ACCEPTANCE_TESTS.md")
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
		}
		n, err := migrateAcceptanceTests(c.Request().Context(), pool, schema, string(bs))
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "inserted": n})
	})
}

