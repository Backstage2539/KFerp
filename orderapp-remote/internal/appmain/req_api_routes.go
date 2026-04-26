package appmain

import (
	"fmt"
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

	e.GET("/api/req/:type", func(c echo.Context) error {
		spec, err := reqTableSpecByType(c.Param("type"))
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		limit := intParam(c, "limit", 10)
		if limit <= 0 {
			limit = 10
		}
		if limit > 500 {
			limit = 500
		}
		page := intParam(c, "page", 1)
		if page <= 0 {
			page = 1
		}
		total, err := countReqRows(c.Request().Context(), pool, schema, spec.table)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
		}
		totalPages := 1
		if total > 0 {
			totalPages = (total + limit - 1) / limit
		}
		if page > totalPages {
			page = totalPages
		}
		offset := (page - 1) * limit
		rows, hasNext, err := listReqRows(c.Request().Context(), pool, schema, spec.table, limit, offset)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{
			"type":        spec.typ,
			"table":       spec.table,
			"title":       spec.title,
			"rows":        rows,
			"limit":       limit,
			"page":        page,
			"total":       total,
			"total_pages": totalPages,
			"has_prev":    page > 1,
			"has_next":    hasNext,
		})
	})

	e.POST("/api/req/:type", func(c echo.Context) error {
		spec, err := reqTableSpecByType(c.Param("type"))
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		var req reqCreateAPIRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
		}
		if spec.table == "req_review" {
			err = createReviewRow(c.Request().Context(), pool, schema, req.Code, req.PRCode, req.Title, req.Status, req.Assignee)
		} else {
			err = createReqRow(c.Request().Context(), pool, schema, spec.table, req.Code, req.Title, req.Status, req.Assignee)
		}
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})

	e.POST("/api/req/review/status", func(c echo.Context) error {
		var req reqStatusAPIRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
		}
		if err := updateReviewStatusAndCascade(c.Request().Context(), pool, schema, actorOf(c), req.Code, req.Status); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
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

type reqTableSpec struct {
	typ   string
	table string
	title string
}

type reqCreateAPIRequest struct {
	Code     string `json:"code"`
	PRCode   string `json:"pr_code"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Assignee string `json:"assignee"`
}

type reqStatusAPIRequest struct {
	Code   string `json:"code"`
	Status string `json:"status"`
}

func reqTableSpecByType(typ string) (reqTableSpec, error) {
	switch strings.ToLower(strings.TrimSpace(typ)) {
	case "product":
		return reqTableSpec{typ: "product", table: "req_product", title: "产品需求表"}, nil
	case "dev":
		return reqTableSpec{typ: "dev", table: "req_dev", title: "开发需求表"}, nil
	case "unit":
		return reqTableSpec{typ: "unit", table: "req_unit", title: "单元测试表"}, nil
	case "api":
		return reqTableSpec{typ: "api", table: "req_api", title: "API 测试表"}, nil
	case "review":
		return reqTableSpec{typ: "review", table: "req_review", title: "需求审核表"}, nil
	default:
		return reqTableSpec{}, fmt.Errorf("invalid requirement table type")
	}
}
