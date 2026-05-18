package catalog

import (
	"fmt"
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
	e.GET("/api/products", h.listAPI)
	e.GET("/api/products/:id", h.detailAPI)
	e.PUT("/api/products/:id", h.updateAPI)
	e.GET("/api/product-settings", h.productSettingsAPI)
	e.GET("/api/product-settings/categories", h.productCategoriesAPI)
	e.GET("/api/pricing-gradient-templates", h.gradientTemplatesAPI)
	e.POST("/api/pricing-gradient-templates", h.saveGradientTemplateAPI)
	e.PUT("/api/pricing-gradient-templates/:id", h.saveGradientTemplateAPI)
	e.POST("/api/pricing-gradient-templates/:id/deactivate", h.deactivateGradientTemplateAPI)
	e.POST("/api/product-settings/products", h.createProductAPI)
	e.POST("/api/product-settings/products/deactivate", h.deactivateProductsAPI)
	e.POST("/api/product-settings/categories", h.saveProductCategoryAPI)
	e.POST("/api/product-settings/custom-products", h.createCustomProductAPI)
	e.PUT("/api/product-settings/categories/:id", h.saveProductCategoryAPI)
	e.DELETE("/api/product-settings/categories/:id", h.deleteProductCategoryAPI)
	e.POST("/api/product-settings/categories/:id/move", h.moveProductCategoryAPI)
	e.POST("/api/product-settings/categories/:id/gradient-template", h.bindCategoryGradientTemplateAPI)
	e.POST("/api/product-settings/products/:id/category", h.assignProductCategoryAPI)
	e.GET("/products/:id", h.edit)
}

type productHandler struct {
	catalog *catalogapp.Service
}

type productUpdateAPIRequest struct {
	RoastLevel         string                    `json:"roast_level"`
	RetailPrice100G    float64                   `json:"retail_price_100g"`
	RetailPrice200G    float64                   `json:"retail_price_200g"`
	RetailPrice227G    float64                   `json:"retail_price_227g"`
	RetailPrice250G    float64                   `json:"retail_price_250g"`
	YieldRate          float64                   `json:"yield_rate"`
	MarginRateOverride *float64                  `json:"margin_rate_override"`
	Tiers              []productTierAPIUpsertRow `json:"tiers"`
}

type productCreateAPIRequest struct {
	Name            string  `json:"name"`
	RoastLevel      string  `json:"roast_level"`
	DefaultPrice    float64 `json:"default_price"`
	RetailPrice100G float64 `json:"retail_price_100g"`
	RetailPrice200G float64 `json:"retail_price_200g"`
	RetailPrice227G float64 `json:"retail_price_227g"`
	RetailPrice250G float64 `json:"retail_price_250g"`
	YieldRate       float64 `json:"yield_rate"`
}

type productDeactivateAPIRequest struct {
	ProductIDs []int64 `json:"product_ids"`
}

type productTierAPIUpsertRow struct {
	SpecG     int64    `json:"spec_g"`
	MinQty    float64  `json:"min_qty"`
	MaxQty    *float64 `json:"max_qty"`
	UnitPrice float64  `json:"unit_price"`
}

type productCategoryAPIRequest struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	ParentID   int64  `json:"parent_id"`
	CustomerID int64  `json:"customer_id"`
	Position   int    `json:"position"`
}

type productCategoryMoveAPIRequest struct {
	ParentID int64 `json:"parent_id"`
	Position int   `json:"position"`
}

type productAssignCategoryAPIRequest struct {
	CategoryID int64 `json:"category_id"`
	Position   int   `json:"position"`
}

type customProductAPIRequest struct {
	CustomerID     int64  `json:"customer_id"`
	BaseProductID  int64  `json:"base_product_id"`
	Name           string `json:"name"`
	RoastLevel     string `json:"roast_level"`
	CustomType     string `json:"custom_type"`
	CopyBOM        bool   `json:"copy_bom"`
	CopyPriceTiers bool   `json:"copy_price_tiers"`
}

type gradientTemplateAPIRequest struct {
	Name        string                            `json:"name"`
	DisplayUnit string                            `json:"display_unit"`
	Tiers       []catalogapp.GradientTemplateTier `json:"tiers"`
}

type bindCategoryGradientTemplateAPIRequest struct {
	GradientTemplateID int64 `json:"gradient_template_id"`
}

func (h productHandler) index(c echo.Context) error {
	return support.VueShellRedirect(c, "productSettings")
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
	yieldRate := normalizeProductYieldRate(req.YieldRate)
	if req.YieldRate > 0 && yieldRate <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid yield_rate"})
	}
	marginRateOverride, err := normalizeProductMarginRateOverride(req.MarginRateOverride)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	if err := h.catalog.UpdateProductBasics(c.Request().Context(), catalogapp.UpdateProductBasicsCommand{
		Actor:              support.ActorOf(c),
		ProductID:          id,
		RoastLevel:         roastLevel,
		RetailPrice100G:    req.RetailPrice100G,
		RetailPrice200G:    req.RetailPrice200G,
		RetailPrice227G:    req.RetailPrice227G,
		RetailPrice250G:    req.RetailPrice250G,
		YieldRate:          yieldRate,
		MarginRateOverride: marginRateOverride,
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

func (h productHandler) createProductAPI(c echo.Context) error {
	var req productCreateAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	roastLevel := NormalizeRoastLevel(req.RoastLevel)
	if roastLevel == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid roast_level"})
	}
	yieldRate := normalizeProductYieldRate(req.YieldRate)
	if req.YieldRate > 0 && yieldRate <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid yield_rate"})
	}
	product, err := h.catalog.CreateProduct(c.Request().Context(), catalogapp.CreateProductCommand{
		Actor:           support.ActorOf(c),
		Name:            req.Name,
		RoastLevel:      roastLevel,
		DefaultPrice:    req.DefaultPrice,
		RetailPrice100G: req.RetailPrice100G,
		RetailPrice200G: req.RetailPrice200G,
		RetailPrice227G: req.RetailPrice227G,
		RetailPrice250G: req.RetailPrice250G,
		YieldRate:       yieldRate,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"product": productOptionFromCatalog(product)})
}

func (h productHandler) deactivateProductsAPI(c echo.Context) error {
	var req productDeactivateAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	if err := h.catalog.DeactivateProducts(c.Request().Context(), catalogapp.DeactivateProductsCommand{
		Actor:      support.ActorOf(c),
		ProductIDs: req.ProductIDs,
	}); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

func normalizeProductYieldRate(value float64) float64 {
	if value <= 0 {
		return 0
	}
	if value > 1 && value <= 100 {
		value = value / 100
	}
	if value <= 0 || value > 1 {
		return 0
	}
	return value
}

func normalizeProductMarginRateOverride(value *float64) (*float64, error) {
	if value == nil {
		return nil, nil
	}
	if *value < 0 {
		return nil, fmt.Errorf("invalid margin_rate_override")
	}
	normalized := *value
	return &normalized, nil
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

func (h productHandler) gradientTemplatesAPI(c echo.Context) error {
	rows, err := h.catalog.ListGradientTemplates(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"rows": rows})
}

func (h productHandler) saveGradientTemplateAPI(c echo.Context) error {
	var req gradientTemplateAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	var id int64
	if idText := c.Param("id"); idText != "" {
		parsed, err := strconv.ParseInt(idText, 10, 64)
		if err != nil || parsed <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
		}
		id = parsed
	}
	row, err := h.catalog.SaveGradientTemplate(c.Request().Context(), catalogapp.SaveGradientTemplateCommand{
		Actor:       support.ActorOf(c),
		ID:          id,
		Name:        req.Name,
		DisplayUnit: req.DisplayUnit,
		Tiers:       req.Tiers,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"template": row})
}

func (h productHandler) deactivateGradientTemplateAPI(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	if err := h.catalog.DeactivateGradientTemplate(c.Request().Context(), catalogapp.DeactivateGradientTemplateCommand{
		Actor: support.ActorOf(c),
		ID:    id,
	}); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
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
		Actor:      support.ActorOf(c),
		ID:         req.ID,
		ParentID:   req.ParentID,
		CustomerID: req.CustomerID,
		Name:       req.Name,
		Position:   req.Position,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"category": row})
}

func (h productHandler) createCustomProductAPI(c echo.Context) error {
	var req customProductAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	product, err := h.catalog.CreateCustomProduct(c.Request().Context(), catalogapp.CreateCustomProductCommand{
		Actor:          support.ActorOf(c),
		CustomerID:     req.CustomerID,
		BaseProductID:  req.BaseProductID,
		Name:           req.Name,
		RoastLevel:     req.RoastLevel,
		CustomType:     req.CustomType,
		CopyBOM:        req.CopyBOM,
		CopyPriceTiers: req.CopyPriceTiers,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"product": productOptionFromCatalog(product)})
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

func (h productHandler) bindCategoryGradientTemplateAPI(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	var req bindCategoryGradientTemplateAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	if err := h.catalog.BindCategoryGradientTemplate(c.Request().Context(), catalogapp.BindCategoryGradientTemplateCommand{
		Actor:              support.ActorOf(c),
		CategoryID:         id,
		GradientTemplateID: req.GradientTemplateID,
	}); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

func (h productHandler) deleteProductCategoryAPI(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	if err := h.catalog.DeleteProductCategory(c.Request().Context(), catalogapp.DeleteProductCategoryCommand{
		Actor: support.ActorOf(c),
		ID:    id,
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
		return echo.ErrNotFound
	}
	return support.VueShellRedirectWith(c, "productSettings", map[string]string{"edit_id": strconv.FormatInt(id, 10)})
}
