package inventory

import (
	"net/http"
	"net/url"
	inventoryapp "orderapp/internal/application/inventory"
	support "orderapp/internal/interfaces/http/support"
	"strings"

	"github.com/labstack/echo/v4"
)

type productAPIOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func registerFinishedInventoryPages(e *echo.Echo, inventorySvc *inventoryapp.Service) {
	e.GET("/products/inventory", func(c echo.Context) error {
		target := "/vue-shell?view=inventory"
		if q := strings.TrimSpace(c.QueryParam("q")); q != "" {
			target += "&q=" + url.QueryEscape(q)
		}
		return c.Redirect(http.StatusFound, target)
	})
	e.GET("/api/products/inventory", func(c echo.Context) error {
		q := strings.TrimSpace(c.QueryParam("q"))
		limit := support.IntParam(c, "limit", 50)
		if limit <= 0 || limit > 200 {
			limit = 50
		}
		offset := support.IntParam(c, "offset", 0)
		if page := support.IntParam(c, "page", 0); page > 0 {
			offset = (page - 1) * limit
		}
		result, err := inventorySvc.ListFinished(c.Request().Context(), inventoryapp.FinishedInventoryQuery{Q: q, Limit: limit, Offset: offset})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
		}
		options := make([]productAPIOption, 0, len(result.Products))
		for _, p := range result.Products {
			options = append(options, productAPIOption{ID: p.ID, Name: p.Name})
		}
		return c.JSON(http.StatusOK, map[string]any{
			"rows":     result.Rows,
			"products": options,
			"page":     (offset / limit) + 1,
			"limit":    limit,
			"has_prev": offset > 0,
			"has_next": result.HasNext,
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
		if err := inventorySvc.AdjustFinished(c.Request().Context(), inventoryapp.AdjustFinishedInventoryCommand{
			ProductID: req.ProductID,
			SpecG:     req.SpecG,
			Units:     req.Units,
			LooseG:    req.LooseG,
			Operator:  support.ActorOf(c),
		}); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})
}
