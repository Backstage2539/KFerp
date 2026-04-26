package appmain

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func registerFinishedInventoryPages(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	e.GET("/products/inventory", func(c echo.Context) error {
		target := "/vue-shell?view=inventory"
		if q := strings.TrimSpace(c.QueryParam("q")); q != "" {
			target += "&q=" + url.QueryEscape(q)
		}
		return c.Redirect(http.StatusFound, target)
	})
	e.GET("/api/products/inventory", func(c echo.Context) error {
		q := strings.TrimSpace(c.QueryParam("q"))
		limit := intParam(c, "limit", 50)
		if limit <= 0 || limit > 200 {
			limit = 50
		}
		offset := intParam(c, "offset", 0)
		if page := intParam(c, "page", 0); page > 0 {
			offset = (page - 1) * limit
		}
		rows, hasNext, err := listFinishedInventory(c.Request().Context(), pool, schema, q, limit, offset)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
		}
		prods, _ := fetchProducts(c.Request().Context(), pool, schema)
		options := make([]apiOption, 0, len(prods))
		for _, p := range prods {
			options = append(options, apiOption{ID: p.ID, Name: p.Name})
		}
		return c.JSON(http.StatusOK, map[string]any{
			"rows":     rows,
			"products": options,
			"page":     (offset / limit) + 1,
			"limit":    limit,
			"has_prev": offset > 0,
			"has_next": hasNext,
		})
	})
	e.POST("/api/products/inventory", func(c echo.Context) error {
		var req struct {
			ProductID int64 `json:"product_id"`
			SpecG     int64 `json:"spec_g"`
			Units     int64 `json:"units"`
			LooseG    int64 `json:"loose_g"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
		}
		if err := upsertFinishedInventory(c.Request().Context(), pool, schema, req.ProductID, req.SpecG, req.Units, req.LooseG); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})
}
