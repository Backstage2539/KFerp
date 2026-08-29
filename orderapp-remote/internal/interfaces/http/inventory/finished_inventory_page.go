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
	ID                   int64                            `json:"id"`
	Name                 string                           `json:"name"`
	MigrationState       string                           `json:"migration_state,omitempty"`
	SpecIdentityMode     string                           `json:"spec_identity_mode,omitempty"`
	BomSpecAuthoritative bool                             `json:"bom_spec_authoritative"`
	BOMSpecs             []inventoryapp.ProductSpecOption `json:"bom_specs,omitempty"`
}

func registerFinishedInventoryPages(e *echo.Echo, inventorySvc *inventoryapp.Service) {
	e.GET("/products/inventory", func(c echo.Context) error {
		target := "/vue-shell?view=inventory"
		if q := strings.TrimSpace(c.QueryParam("q")); q != "" {
			target += "&q=" + url.QueryEscape(q)
		}
		return c.Redirect(http.StatusFound, support.PrefixRelativeLocation(c, target))
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
		page := (offset / limit) + 1
		totalPages := finishedInventoryPageCount(result.Total, limit)
		options := make([]productAPIOption, 0, len(result.Products))
		for _, p := range result.Products {
			options = append(options, productAPIOption{
				ID: p.ID, Name: p.Name, MigrationState: p.MigrationState,
				SpecIdentityMode: p.SpecIdentityMode, BomSpecAuthoritative: p.BomSpecAuthoritative,
				BOMSpecs: p.BOMSpecs,
			})
		}
		return c.JSON(http.StatusOK, map[string]any{
			"rows":        result.Rows,
			"products":    options,
			"page":        page,
			"limit":       limit,
			"total":       result.Total,
			"total_pages": totalPages,
			"has_prev":    offset > 0,
			"has_next":    page < totalPages,
		})
	})
	e.POST("/api/products/inventory", func(c echo.Context) error {
		var req struct {
			ProductID    int64  `json:"product_id"`
			SpecG        int64  `json:"spec_g"`
			BomSpecID    int64  `json:"bom_spec_id"`
			BomVariantID int64  `json:"bom_variant_id"`
			UnitCode     string `json:"unit_code"`
			Warehouse    string `json:"warehouse"`
			Units        int64  `json:"units"`
			LooseG       int64  `json:"loose_g"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
		}
		if err := inventorySvc.AdjustFinished(c.Request().Context(), inventoryapp.AdjustFinishedInventoryCommand{
			ProductID:    req.ProductID,
			SpecG:        req.SpecG,
			BomSpecID:    req.BomSpecID,
			BomVariantID: req.BomVariantID,
			UnitCode:     req.UnitCode,
			Warehouse:    req.Warehouse,
			Units:        req.Units,
			LooseG:       req.LooseG,
			Operator:     support.ActorOf(c),
		}); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})
}

func finishedInventoryPageCount(total, limit int) int {
	if limit <= 0 {
		limit = 50
	}
	if total <= 0 {
		return 1
	}
	return (total + limit - 1) / limit
}
