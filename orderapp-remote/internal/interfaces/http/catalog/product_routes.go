package catalog

import (
	"net/http"
	support "orderapp/internal/interfaces/http/support"
	"strconv"

	catalogapp "orderapp/internal/application/catalog"

	"github.com/labstack/echo/v4"
)

func registerProductRoutes(e *echo.Echo, catalogSvc *catalogapp.Service) {
	h := productHandler{
		catalog: catalogSvc,
	}

	e.GET("/products", h.index)
	e.GET("/products/print", h.print)
	e.GET("/api/products", h.listAPI)
	e.GET("/api/products/:id", h.detailAPI)
	e.PUT("/api/products/:id", h.updateAPI)
	e.GET("/api/product-settings", h.productSettingsAPI)
	e.GET("/api/product-settings/categories", h.productCategoriesAPI)
	e.POST("/api/product-settings/categories", h.saveProductCategoryAPI)
	e.PUT("/api/product-settings/categories/:id", h.saveProductCategoryAPI)
	e.POST("/api/product-settings/categories/:id/move", h.moveProductCategoryAPI)
	e.POST("/api/product-settings/products/:id/category", h.assignProductCategoryAPI)
	e.GET("/products/:id", h.edit)
}

type productHandler struct {
	catalog *catalogapp.Service
}

type productUpdateAPIRequest struct {
	RoastLevel      string                    `json:"roast_level"`
	RetailPrice100G float64                   `json:"retail_price_100g"`
	RetailPrice200G float64                   `json:"retail_price_200g"`
	RetailPrice227G float64                   `json:"retail_price_227g"`
	RetailPrice250G float64                   `json:"retail_price_250g"`
	Tiers           []productTierAPIUpsertRow `json:"tiers"`
}

type productTierAPIUpsertRow struct {
	SpecG     int64    `json:"spec_g"`
	MinQty    float64  `json:"min_qty"`
	MaxQty    *float64 `json:"max_qty"`
	UnitPrice float64  `json:"unit_price"`
}

type productCategoryAPIRequest struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	ParentID int64  `json:"parent_id"`
	Position int    `json:"position"`
}

type productCategoryMoveAPIRequest struct {
	ParentID int64 `json:"parent_id"`
	Position int   `json:"position"`
}

type productAssignCategoryAPIRequest struct {
	CategoryID int64 `json:"category_id"`
	Position   int   `json:"position"`
}

func (h productHandler) index(c echo.Context) error {
	return support.VueShellRedirect(c, "productSettings")
}

func (h productHandler) print(c echo.Context) error {
	return support.VueShellRedirect(c, "quotePrint")
}

func (h productHandler) listAPI(c echo.Context) error {
	ps, err := h.catalog.ListProducts(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"rows": productOptionsFromCatalog(ps)})
}

func (h productHandler) detailAPI(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	p, err := h.catalog.GetProduct(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if p == nil {
		return c.JSON(http.StatusNotFound, map[string]any{"error": "not found"})
	}
	return c.JSON(http.StatusOK, map[string]any{"product": productOptionFromCatalog(*p)})
}

func (h productHandler) updateAPI(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	var req productUpdateAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	roastLevel := NormalizeRoastLevel(req.RoastLevel)
	if roastLevel == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid roast_level"})
	}
	if err := h.catalog.UpdateProductBasics(c.Request().Context(), catalogapp.UpdateProductBasicsCommand{
		Actor:           support.ActorOf(c),
		ProductID:       id,
		RoastLevel:      roastLevel,
		RetailPrice100G: req.RetailPrice100G,
		RetailPrice200G: req.RetailPrice200G,
		RetailPrice227G: req.RetailPrice227G,
		RetailPrice250G: req.RetailPrice250G,
	}); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	p, err := h.catalog.GetProduct(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if p == nil {
		return c.JSON(http.StatusNotFound, map[string]any{"error": "not found"})
	}
	return c.JSON(http.StatusOK, map[string]any{"product": productOptionFromCatalog(*p)})
}

func (h productHandler) productSettingsAPI(c echo.Context) error {
	data, err := h.catalog.ProductSettings(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, data)
}

func (h productHandler) productCategoriesAPI(c echo.Context) error {
	data, err := h.catalog.ProductSettings(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"categories": data.Categories})
}

func (h productHandler) saveProductCategoryAPI(c echo.Context) error {
	var req productCategoryAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	if idText := c.Param("id"); idText != "" {
		id, err := strconv.ParseInt(idText, 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
		}
		req.ID = id
	}
	if req.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "name required"})
	}
	row, err := h.catalog.SaveProductCategory(c.Request().Context(), catalogapp.SaveProductCategoryCommand{
		Actor:    support.ActorOf(c),
		ID:       req.ID,
		ParentID: req.ParentID,
		Name:     req.Name,
		Position: req.Position,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"category": row})
}

func (h productHandler) moveProductCategoryAPI(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	var req productCategoryMoveAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	if err := h.catalog.MoveProductCategory(c.Request().Context(), catalogapp.MoveProductCategoryCommand{
		Actor:    support.ActorOf(c),
		ID:       id,
		ParentID: req.ParentID,
		Position: req.Position,
	}); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

func (h productHandler) assignProductCategoryAPI(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	var req productAssignCategoryAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	if err := h.catalog.AssignProductCategory(c.Request().Context(), catalogapp.AssignProductCategoryCommand{
		Actor:      support.ActorOf(c),
		ProductID:  id,
		CategoryID: req.CategoryID,
		Position:   req.Position,
	}); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

func (h productHandler) edit(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	return support.VueShellRedirectWith(c, "productSettings", map[string]string{"edit_id": strconv.FormatInt(id, 10)})
}
