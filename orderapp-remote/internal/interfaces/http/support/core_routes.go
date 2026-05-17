package support

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
	return c.Redirect(http.StatusSeeOther, PrefixRelativeLocation(c, "/orders"))
}

func (h coreHandler) orderListAlias(c echo.Context) error {
	return c.Redirect(http.StatusSeeOther, PrefixRelativeLocation(c, "/orders"))
}

func (h coreHandler) orderDetailAlias(c echo.Context) error {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return c.Redirect(http.StatusSeeOther, PrefixRelativeLocation(c, "/orders"))
	}
	return c.Redirect(http.StatusSeeOther, PrefixRelativeLocation(c, "/orders/"+id))
}

func (h coreHandler) login(c echo.Context) error {
	return c.Render(http.StatusOK, "login.html", map[string]any{})
}

func (h coreHandler) audit(c echo.Context) error {
	return VueShellRedirect(c, "audit")
}

func (h coreHandler) auditAPI(c echo.Context) error {
	limit := IntParam(c, "limit", 200)
	if limit <= 0 {
		limit = 200
	}
	if limit > 500 {
		limit = 500
	}
	page := IntParam(c, "page", 1)
	if page <= 0 {
		page = 1
	}
	offset := IntParam(c, "offset", (page-1)*limit)
	if offset < 0 {
		offset = 0
	}
	page = (offset / limit) + 1
	data := AuditPageData{
		From:       strings.TrimSpace(c.QueryParam("from")),
		To:         strings.TrimSpace(c.QueryParam("to")),
		Q:          strings.TrimSpace(c.QueryParam("q")),
		EntityType: strings.TrimSpace(c.QueryParam("type")),
	}
	result, err := fetchAuditPage(c.Request().Context(), h.pool, h.schema, data.From, data.To, data.Q, data.EntityType, limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	totalPages := auditPageCount(result.Total, limit)
	return c.JSON(http.StatusOK, map[string]any{
		"rows": result.Rows,
		"filters": map[string]any{
			"from": data.From,
			"to":   data.To,
			"q":    data.Q,
			"type": data.EntityType,
		},
		"page":        page,
		"limit":       limit,
		"offset":      offset,
		"total":       result.Total,
		"total_pages": totalPages,
		"has_prev":    page > 1,
		"has_next":    page < totalPages,
	})
}

func auditPageCount(total, limit int) int {
	if limit <= 0 {
		limit = 200
	}
	if total <= 0 {
		return 1
	}
	return (total + limit - 1) / limit
}
