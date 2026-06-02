package catalog

import (
	"encoding/json"
	"fmt"
	"net/http"
	catalogdomain "orderapp/internal/domain/catalog"
	support "orderapp/internal/interfaces/http/support"
	"strconv"
	"strings"

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
	e.GET("/api/customer-product-aliases", h.customerProductAliasesAPI)
	e.GET("/api/customer-product-aliases/migration-candidates", h.customerProductAliasMigrationCandidatesAPI)
	e.POST("/api/customer-product-aliases/batch", h.batchCustomerProductAliasesAPI)
	e.POST("/api/customer-product-aliases", h.saveCustomerProductAliasAPI)
	e.PUT("/api/customer-product-aliases/:id", h.saveCustomerProductAliasAPI)
	e.POST("/api/customer-product-aliases/:id/disable", h.disableCustomerProductAliasAPI)
	e.GET("/api/product-settings/categories", h.productCategoriesAPI)
	e.GET("/api/product-production-configs", h.productProductionConfigsAPI)
	e.GET("/api/product-production-configs/:product_id", h.productProductionConfigAPI)
	e.POST("/api/product-production-configs/:product_id", h.saveProductProductionConfigAPI)
	e.PUT("/api/product-production-configs/:product_id", h.saveProductProductionConfigAPI)
	e.GET("/api/product-classification-templates", h.productClassificationTemplatesAPI)
	e.POST("/api/product-classification-templates", h.saveProductClassificationTemplateAPI)
	e.PUT("/api/product-classification-templates/:id", h.saveProductClassificationTemplateAPI)
	e.DELETE("/api/product-classification-templates/:id", h.deleteProductClassificationTemplateAPI)
	e.POST("/api/product-classification-template-categories", h.saveProductClassificationCategoryAPI)
	e.PUT("/api/product-classification-template-categories/:id", h.saveProductClassificationCategoryAPI)
	e.DELETE("/api/product-classification-template-categories/:id", h.deleteProductClassificationCategoryAPI)
	e.POST("/api/product-classification-assignments/products", h.saveProductClassificationAssignmentAPI)
	e.POST("/api/product-classification-assignments/customer-aliases", h.saveCustomerProductAliasClassificationAssignmentAPI)
	e.GET("/api/pricing-gradient-templates", h.gradientTemplatesAPI)
	e.POST("/api/pricing-gradient-templates", h.saveGradientTemplateAPI)
	e.PUT("/api/pricing-gradient-templates/:id", h.saveGradientTemplateAPI)
	e.POST("/api/product-settings/product-config-templates", h.saveProductConfigTemplateAPI)
	e.PUT("/api/product-settings/product-config-templates/:id", h.saveProductConfigTemplateAPI)
	e.POST("/api/product-settings/product-config-templates/derive", h.deriveProductConfigTemplateAPI)
	e.POST("/api/product-settings/units", h.saveProductUnitDefinitionAPI)
	e.PUT("/api/product-settings/units/:code", h.saveProductUnitDefinitionAPI)
	e.POST("/api/product-settings/unit-templates", h.saveProductUnitTemplateAPI)
	e.PUT("/api/product-settings/unit-templates/:id", h.saveProductUnitTemplateAPI)
	e.POST("/api/pricing-gradient-templates/:id/deactivate", h.deactivateGradientTemplateAPI)
	e.POST("/api/product-settings/skus", h.createSKUAPI)
	e.GET("/api/product-settings/skus/copy-options", h.skuCopyOptionsAPI)
	e.POST("/api/product-settings/skus/copy", h.copySKUsAPI)
	e.POST("/api/product-settings/products", h.createProductAPI)
	e.POST("/api/product-settings/products/deactivate", h.deactivateProductsAPI)
	e.POST("/api/product-settings/categories", h.saveProductCategoryAPI)
	e.POST("/api/product-settings/custom-products", h.createCustomProductAPI)
	e.POST("/api/product-settings/customer-products/derive", h.deriveCustomerProductAPI)
	e.POST("/api/product-settings/customer-categories/derive", h.deriveProductCategoryAPI)
	e.POST("/api/product-settings/customer-gradient-templates/derive", h.deriveGradientTemplateAPI)
	e.POST("/api/product-settings/customer-public-usage", h.saveCustomerPublicUsageAPI)
	e.POST("/api/product-settings/customer-rule-templates", h.saveCustomerProductRuleTemplateAPI)
	e.PUT("/api/product-settings/customer-rule-templates/:id", h.saveCustomerProductRuleTemplateAPI)
	e.POST("/api/product-settings/customer-rule-overrides", h.saveCustomerProductRuleOverrideAPI)
	e.PUT("/api/product-settings/customer-rule-overrides/:id", h.saveCustomerProductRuleOverrideAPI)
	e.POST("/api/product-settings/customers/:id/rule-template", h.bindCustomerProductRuleTemplateAPI)
	e.PUT("/api/product-settings/categories/:id", h.saveProductCategoryAPI)
	e.DELETE("/api/product-settings/categories/:id", h.deleteProductCategoryAPI)
	e.POST("/api/product-settings/categories/:id/move", h.moveProductCategoryAPI)
	e.POST("/api/product-settings/categories/:id/gradient-template", h.bindCategoryGradientTemplateAPI)
	e.POST("/api/product-settings/products/:id/category", h.assignProductCategoryAPI)
	e.GET("/products/:id", h.edit)
}

type optionalNullableFloat64 struct {
	Set   bool
	Value *float64
}

func (o *optionalNullableFloat64) UnmarshalJSON(data []byte) error {
	o.Set = true
	var value *float64
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	o.Value = value
	return nil
}

type productHandler struct {
	catalog *catalogapp.Service
}

type productUpdateAPIRequest struct {
	Name                        *string                   `json:"name"`
	ProductKind                 string                    `json:"product_kind"`
	Remark                      *string                   `json:"remark"`
	GreenBeanType               string                    `json:"green_bean_type"`
	GreenBeanBomProductID       int64                     `json:"green_bean_bom_product_id"`
	DefaultPrice                *float64                  `json:"default_price"`
	RoastLevel                  *string                   `json:"roast_level"`
	SpecialAttrsJSON            *string                   `json:"special_attrs_json"`
	DripBagGrams                *float64                  `json:"drip_bag_grams"`
	DripBoxBagCount             *int                      `json:"drip_box_bag_count"`
	AllowFulfillmentOrder       *bool                     `json:"allow_fulfillment_order"`
	AllowMallOrder              *bool                     `json:"allow_mall_order"`
	RetailPrice100G             *float64                  `json:"retail_price_100g"`
	RetailPrice200G             *float64                  `json:"retail_price_200g"`
	RetailPrice227G             *float64                  `json:"retail_price_227g"`
	RetailPrice250G             *float64                  `json:"retail_price_250g"`
	YieldRate                   float64                   `json:"yield_rate"`
	MarginRateOverride          optionalNullableFloat64   `json:"margin_rate_override"`
	GradientTemplateIDOverride  *int64                    `json:"gradient_template_id_override"`
	OperationTemplateIDOverride *int64                    `json:"operation_template_id_override"`
	UnitRuleOverrideJSON        *string                   `json:"unit_rule_override_json"`
	ProductConfigTemplateID     *int64                    `json:"product_config_template_id"`
	ClassificationTemplateID    *int64                    `json:"classification_template_id"`
	Tiers                       []productTierAPIUpsertRow `json:"tiers"`
}

type productCreateAPIRequest struct {
	Name                     string                    `json:"name"`
	Remark                   string                    `json:"remark"`
	ProductKind              string                    `json:"product_kind"`
	GreenBeanType            string                    `json:"green_bean_type"`
	GreenBeanBomProductID    int64                     `json:"green_bean_bom_product_id"`
	RoastLevel               *string                   `json:"roast_level"`
	SpecialAttrsJSON         string                    `json:"special_attrs_json"`
	DripBagGrams             *float64                  `json:"drip_bag_grams"`
	DripBoxBagCount          *int                      `json:"drip_box_bag_count"`
	AllowFulfillmentOrder    *bool                     `json:"allow_fulfillment_order"`
	AllowMallOrder           *bool                     `json:"allow_mall_order"`
	DefaultPrice             float64                   `json:"default_price"`
	RetailPrice100G          float64                   `json:"retail_price_100g"`
	RetailPrice200G          float64                   `json:"retail_price_200g"`
	RetailPrice227G          float64                   `json:"retail_price_227g"`
	RetailPrice250G          float64                   `json:"retail_price_250g"`
	YieldRate                float64                   `json:"yield_rate"`
	ProductConfigTemplateID  int64                     `json:"product_config_template_id"`
	ClassificationTemplateID int64                     `json:"classification_template_id"`
	Tiers                    []productTierAPIUpsertRow `json:"tiers"`
}

type skuCreateAPIRequest struct {
	CustomerID               int64  `json:"customer_id"`
	Name                     string `json:"name"`
	Remark                   string `json:"remark"`
	ProductTypeCategoryID    int64  `json:"product_type_category_id"`
	ProductSubtypeCategoryID int64  `json:"product_subtype_category_id"`
	SpecialAttrsJSON         string `json:"special_attrs_json"`
	ProductConfigTemplateID  int64  `json:"product_config_template_id"`
	ClassificationTemplateID int64  `json:"classification_template_id"`
	Active                   *bool  `json:"active"`
}

type skuCopyAPIRequest struct {
	TargetCustomerID int64   `json:"target_customer_id"`
	SourceCustomerID int64   `json:"source_customer_id"`
	SourceSKUIDs     []int64 `json:"source_sku_ids"`
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
	ID                      int64  `json:"id"`
	Name                    string `json:"name"`
	ParentID                int64  `json:"parent_id"`
	CustomerID              int64  `json:"customer_id"`
	Position                int    `json:"position"`
	ProductConfigTemplateID int64  `json:"product_config_template_id"`
	GradientTemplateID      int64  `json:"gradient_template_id"`
	OperationTemplateID     int64  `json:"operation_template_id"`
	PriceListRuleJSON       string `json:"price_list_rule_json"`
	InventoryUnit           string `json:"inventory_unit"`
	QuoteUnit               string `json:"quote_unit"`
	OrderUnit               string `json:"order_unit"`
	UnitConversionJSON      string `json:"unit_conversion_json"`
	IntegerUnit             bool   `json:"integer_unit"`
}

type productCategoryMoveAPIRequest struct {
	ParentID int64 `json:"parent_id"`
	Position int   `json:"position"`
}

type productAssignCategoryAPIRequest struct {
	CategoryID           int64 `json:"category_id"`
	CustomerID           int64 `json:"customer_id"`
	Position             int   `json:"position"`
	DerivePublicCategory bool  `json:"derive_public_category"`
	DerivePublicProduct  bool  `json:"derive_public_product"`
}

type customProductAPIRequest struct {
	CustomerID            int64   `json:"customer_id"`
	BaseProductID         int64   `json:"base_product_id"`
	Name                  string  `json:"name"`
	Remark                string  `json:"remark"`
	ProductKind           string  `json:"product_kind"`
	GreenBeanType         string  `json:"green_bean_type"`
	GreenBeanBomProductID int64   `json:"green_bean_bom_product_id"`
	RoastLevel            string  `json:"roast_level"`
	SpecialAttrsJSON      string  `json:"special_attrs_json"`
	YieldRate             float64 `json:"yield_rate"`
	DripBagGrams          float64 `json:"drip_bag_grams"`
	DripBoxBagCount       int     `json:"drip_box_bag_count"`
	CustomType            string  `json:"custom_type"`
	CopyBOM               bool    `json:"copy_bom"`
	CopyPriceTiers        bool    `json:"copy_price_tiers"`
}

type customerPublicUsageAPIRequest struct {
	CustomerID                 int64 `json:"customer_id"`
	UsePublicSKU               bool  `json:"use_public_sku"`
	UsePublicCategories        bool  `json:"use_public_categories"`
	UsePublicGradientTemplates bool  `json:"use_public_gradient_templates"`
}

type customerProductAliasAPIRequest struct {
	CustomerID               int64  `json:"customer_id"`
	ProductID                int64  `json:"product_id"`
	DisplayName              string `json:"display_name"`
	CustomerItemCode         string `json:"customer_item_code"`
	BrandName                string `json:"brand_name"`
	DisplayCategoryID        int64  `json:"display_category_id"`
	ClassificationTemplateID int64  `json:"classification_template_id"`
	SortOrder                int    `json:"sort_order"`
	IncludeInPriceList       *bool  `json:"include_in_price_list"`
	Active                   *bool  `json:"active"`
	Remark                   string `json:"remark"`
}

type customerProductAliasBatchAPIRequest struct {
	CustomerID               int64   `json:"customer_id"`
	ProductIDs               []int64 `json:"product_ids"`
	IncludeInPriceList       *bool   `json:"include_in_price_list"`
	BrandName                string  `json:"brand_name"`
	DisplayCategoryID        int64   `json:"display_category_id"`
	ClassificationTemplateID int64   `json:"classification_template_id"`
}

type customerProductRuleTemplateAPIRequest struct {
	CustomerID int64                                        `json:"customer_id"`
	Name       string                                       `json:"name"`
	Active     *bool                                        `json:"active"`
	Items      []catalogapp.CustomerProductRuleTemplateItem `json:"items"`
}

type customerProductRuleOverrideAPIRequest struct {
	CustomerID               int64  `json:"customer_id"`
	ProductSubtypeCategoryID int64  `json:"product_subtype_category_id"`
	GradientTemplateID       int64  `json:"gradient_template_id"`
	OperationTemplateID      int64  `json:"operation_template_id"`
	PriceListRuleJSON        string `json:"price_list_rule_json"`
	UnitRuleJSON             string `json:"unit_rule_json"`
	Active                   *bool  `json:"active"`
}

type customerProductRuleTemplateBindingAPIRequest struct {
	TemplateID int64 `json:"template_id"`
}

type deriveProductCategoryAPIRequest struct {
	CustomerID       int64 `json:"customer_id"`
	SourceCategoryID int64 `json:"source_category_id"`
}

type deriveCustomerProductAPIRequest struct {
	CustomerID     int64  `json:"customer_id"`
	BaseProductID  int64  `json:"base_product_id"`
	CategoryID     int64  `json:"category_id"`
	Position       int    `json:"position"`
	Name           string `json:"name"`
	CopyBOM        bool   `json:"copy_bom"`
	CopyPriceTiers bool   `json:"copy_price_tiers"`
}

type deriveGradientTemplateAPIRequest struct {
	CustomerID       int64  `json:"customer_id"`
	SourceTemplateID int64  `json:"source_template_id"`
	Name             string `json:"name"`
}

type deriveProductConfigTemplateAPIRequest struct {
	CustomerID       int64  `json:"customer_id"`
	SourceTemplateID int64  `json:"source_template_id"`
	Name             string `json:"name"`
}

type gradientTemplateAPIRequest struct {
	CustomerID     int64                             `json:"customer_id"`
	Name           string                            `json:"name"`
	DisplayUnit    string                            `json:"display_unit"`
	UnitTemplateID int64                             `json:"unit_template_id"`
	Tiers          []catalogapp.GradientTemplateTier `json:"tiers"`
}

type productConfigTemplateAPIRequest struct {
	CustomerID             int64  `json:"customer_id"`
	Name                   string `json:"name"`
	GradientTemplateID     int64  `json:"gradient_template_id"`
	OperationTemplateID    int64  `json:"operation_template_id"`
	UnitTemplateID         int64  `json:"unit_template_id"`
	PriceListRuleJSON      string `json:"price_list_rule_json"`
	SpecialAttrsSchemaJSON string `json:"special_attrs_schema_json"`
	InventoryUnit          string `json:"inventory_unit"`
	QuoteUnit              string `json:"quote_unit"`
	OrderUnit              string `json:"order_unit"`
	UnitConversionJSON     string `json:"unit_conversion_json"`
	IntegerUnit            bool   `json:"integer_unit"`
	Active                 *bool  `json:"active"`
}

type productProductionConfigAPIRequest struct {
	ProductionBomID         int64                                     `json:"production_bom_id"`
	ProductionBomVersionID  int64                                     `json:"production_bom_version_id"`
	ProcessRouteID          int64                                     `json:"process_route_id"`
	IndustryFieldTemplateID int64                                     `json:"industry_field_template_id"`
	ExpectedLossRate        float64                                   `json:"expected_loss_rate"`
	Note                    string                                    `json:"note"`
	Fields                  []catalogapp.ProductProductionConfigField `json:"fields"`
}

type productClassificationTemplateAPIRequest struct {
	CustomerID       int64  `json:"customer_id"`
	SourceTemplateID int64  `json:"source_template_id"`
	Name             string `json:"name"`
	Active           *bool  `json:"active"`
	SortOrder        int    `json:"sort_order"`
}

type productClassificationCategoryAPIRequest struct {
	TemplateID int64  `json:"template_id"`
	ParentID   int64  `json:"parent_id"`
	Name       string `json:"name"`
	Level      int    `json:"level"`
	SortOrder  int    `json:"sort_order"`
}

type productClassificationAssignmentAPIRequest struct {
	ProductID  int64 `json:"product_id"`
	TemplateID int64 `json:"template_id"`
	CategoryID int64 `json:"category_id"`
	SortOrder  int   `json:"sort_order"`
}

type customerProductAliasClassificationAssignmentAPIRequest struct {
	AliasID    int64 `json:"alias_id"`
	TemplateID int64 `json:"template_id"`
	CategoryID int64 `json:"category_id"`
	SortOrder  int   `json:"sort_order"`
}

type productUnitDefinitionAPIRequest struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	UnitType     string `json:"unit_type"`
	AllowDecimal bool   `json:"allow_decimal"`
	Active       *bool  `json:"active"`
}

type productUnitTemplateAPIRequest struct {
	Name               string `json:"name"`
	InventoryUnit      string `json:"inventory_unit"`
	QuoteUnit          string `json:"quote_unit"`
	OrderUnit          string `json:"order_unit"`
	UnitConversionJSON string `json:"unit_conversion_json"`
	IntegerUnit        bool   `json:"integer_unit"`
	Active             *bool  `json:"active"`
}

type bindCategoryGradientTemplateAPIRequest struct {
	GradientTemplateID int64 `json:"gradient_template_id"`
}

func productTiersFromAPI(rows []productTierAPIUpsertRow) []catalogapp.PriceTier {
	out := make([]catalogapp.PriceTier, 0, len(rows))
	for _, row := range rows {
		out = append(out, catalogapp.PriceTier{
			SpecG:     row.SpecG,
			MinQty:    row.MinQty,
			MaxQty:    row.MaxQty,
			UnitPrice: row.UnitPrice,
		})
	}
	return out
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
	existing, err := h.catalog.GetProduct(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if existing == nil {
		return c.JSON(http.StatusNotFound, map[string]any{"error": "not found"})
	}
	productKind := catalogdomain.NormalizeProductKind(firstNonEmptyString(req.ProductKind, existing.ProductKind))
	greenBeanType := firstNonEmptyString(req.GreenBeanType, existing.GreenBeanType)
	greenBeanBomProductID := req.GreenBeanBomProductID
	if greenBeanBomProductID <= 0 {
		greenBeanBomProductID = existing.GreenBeanBomProductID
	}
	roastLevelInput := existing.RoastLevel
	if req.RoastLevel != nil {
		roastLevelInput = *req.RoastLevel
	}
	roastLevel := NormalizeRoastLevel(roastLevelInput)
	if strings.TrimSpace(roastLevelInput) != "" && roastLevel == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid roast_level"})
	}
	if catalogdomain.ProductKindRequiresRoast(productKind) && roastLevel == "" {
		roastLevel = "中烘"
	}
	defaultPrice := optionalFloat64(req.DefaultPrice, existing.DefaultPrice)
	retailPrice100G := optionalFloat64(req.RetailPrice100G, existing.RetailPrice100G)
	retailPrice200G := optionalFloat64(req.RetailPrice200G, existing.RetailPrice200G)
	retailPrice227G := optionalFloat64(req.RetailPrice227G, existing.RetailPrice227G)
	retailPrice250G := optionalFloat64(req.RetailPrice250G, existing.RetailPrice250G)
	if defaultPrice < 0 || retailPrice100G < 0 || retailPrice200G < 0 || retailPrice227G < 0 || retailPrice250G < 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "price must not be negative"})
	}
	dripBagGrams := existing.DripBagGrams
	dripBoxBagCount := existing.DripBoxBagCount
	allowFulfillmentOrder := existing.AllowFulfillmentOrder
	allowMallOrder := existing.AllowMallOrder
	if req.DripBagGrams != nil {
		dripBagGrams = *req.DripBagGrams
	}
	if req.DripBoxBagCount != nil {
		dripBoxBagCount = *req.DripBoxBagCount
	}
	if req.AllowFulfillmentOrder != nil {
		allowFulfillmentOrder = *req.AllowFulfillmentOrder
	}
	if req.AllowMallOrder != nil {
		allowMallOrder = *req.AllowMallOrder
	}
	if err := validateExplicitDripConfig(productKind, req.DripBagGrams, req.DripBoxBagCount); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	yieldRate := normalizeProductYieldRate(req.YieldRate)
	if catalogdomain.ProductKindSupportsBomParams(productKind) && req.YieldRate > 0 && yieldRate <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid yield_rate"})
	}
	marginRateOverride := existing.MarginRateOverride
	if req.MarginRateOverride.Set {
		marginRateOverride, err = normalizeProductMarginRateOverride(req.MarginRateOverride.Value)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
	}
	name := existing.Name
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
		if name == "" {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "name is required"})
		}
	}
	remark := existing.Remark
	if req.Remark != nil {
		remark = strings.TrimSpace(*req.Remark)
	}
	gradientTemplateIDOverride := existing.GradientTemplateIDOverride
	if req.GradientTemplateIDOverride != nil {
		gradientTemplateIDOverride = *req.GradientTemplateIDOverride
	}
	operationTemplateIDOverride := existing.OperationTemplateIDOverride
	if req.OperationTemplateIDOverride != nil {
		operationTemplateIDOverride = *req.OperationTemplateIDOverride
	}
	unitRuleOverrideJSON := existing.UnitRuleOverrideJSON
	if req.UnitRuleOverrideJSON != nil {
		unitRuleOverrideJSON = strings.TrimSpace(*req.UnitRuleOverrideJSON)
	}
	productConfigTemplateID := existing.ProductConfigTemplateID
	if req.ProductConfigTemplateID != nil {
		productConfigTemplateID = *req.ProductConfigTemplateID
	}
	classificationTemplateID := existing.ClassificationTemplateID
	if req.ClassificationTemplateID != nil {
		classificationTemplateID = *req.ClassificationTemplateID
	}
	specialAttrsJSON := existing.SpecialAttrsJSON
	if req.SpecialAttrsJSON != nil {
		specialAttrsJSON = strings.TrimSpace(*req.SpecialAttrsJSON)
	}
	if err := h.catalog.UpdateProductBasics(c.Request().Context(), catalogapp.UpdateProductBasicsCommand{
		Actor:                       support.ActorOf(c),
		ProductID:                   id,
		Name:                        name,
		Remark:                      remark,
		RoastLevel:                  roastLevel,
		ProductKind:                 productKind,
		GreenBeanType:               greenBeanType,
		GreenBeanBomProductID:       greenBeanBomProductID,
		DefaultPrice:                defaultPrice,
		DripBagGrams:                dripBagGrams,
		DripBoxBagCount:             dripBoxBagCount,
		AllowFulfillmentOrder:       allowFulfillmentOrder,
		AllowMallOrder:              allowMallOrder,
		RetailPrice100G:             retailPrice100G,
		RetailPrice200G:             retailPrice200G,
		RetailPrice227G:             retailPrice227G,
		RetailPrice250G:             retailPrice250G,
		YieldRate:                   yieldRate,
		MarginRateOverride:          marginRateOverride,
		GradientTemplateIDOverride:  gradientTemplateIDOverride,
		OperationTemplateIDOverride: operationTemplateIDOverride,
		UnitRuleOverrideJSON:        unitRuleOverrideJSON,
		ProductConfigTemplateID:     productConfigTemplateID,
		ClassificationTemplateID:    classificationTemplateID,
		SpecialAttrsJSON:            specialAttrsJSON,
	}); err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if productKind != catalogdomain.ProductKindGreenBean && len(req.Tiers) > 0 {
		if err := h.catalog.ReplacePriceTiers(c.Request().Context(), catalogapp.ReplacePriceTiersCommand{
			Actor:                 support.ActorOf(c),
			ProductID:             id,
			ProductKind:           productKind,
			GreenBeanType:         greenBeanType,
			GreenBeanBomProductID: greenBeanBomProductID,
			DefaultPrice:          defaultPrice,
			RoastLevel:            roastLevel,
			RetailPrice100G:       retailPrice100G,
			RetailPrice200G:       retailPrice200G,
			RetailPrice227G:       retailPrice227G,
			RetailPrice250G:       retailPrice250G,
			YieldRate:             yieldRate,
			Tiers:                 productTiersFromAPI(req.Tiers),
		}); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
		}
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

func optionalFloat64(value *float64, fallback float64) float64 {
	if value == nil {
		return fallback
	}
	return *value
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if s := strings.TrimSpace(value); s != "" {
			return s
		}
	}
	return ""
}

func (h productHandler) createProductAPI(c echo.Context) error {
	var req productCreateAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	productKind := catalogdomain.NormalizeProductKind(req.ProductKind)
	roastLevelInput := ""
	if req.RoastLevel != nil {
		roastLevelInput = *req.RoastLevel
	}
	roastLevel := NormalizeRoastLevel(roastLevelInput)
	if strings.TrimSpace(roastLevelInput) != "" && roastLevel == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid roast_level"})
	}
	if catalogdomain.ProductKindRequiresRoast(productKind) && roastLevel == "" {
		roastLevel = "中烘"
	}
	yieldRate := normalizeProductYieldRate(req.YieldRate)
	if catalogdomain.ProductKindSupportsBomParams(productKind) && req.YieldRate > 0 && yieldRate <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid yield_rate"})
	}
	if err := validateExplicitDripConfig(productKind, req.DripBagGrams, req.DripBoxBagCount); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	dripBagGrams := 0.0
	if req.DripBagGrams != nil {
		dripBagGrams = *req.DripBagGrams
	}
	dripBoxBagCount := 0
	if req.DripBoxBagCount != nil {
		dripBoxBagCount = *req.DripBoxBagCount
	}
	allowFulfillmentOrder := true
	if req.AllowFulfillmentOrder != nil {
		allowFulfillmentOrder = *req.AllowFulfillmentOrder
	}
	allowMallOrder := false
	if req.AllowMallOrder != nil {
		allowMallOrder = *req.AllowMallOrder
	}
	product, err := h.catalog.CreateProduct(c.Request().Context(), catalogapp.CreateProductCommand{
		Actor:                    support.ActorOf(c),
		Name:                     req.Name,
		Remark:                   req.Remark,
		RoastLevel:               roastLevel,
		ProductKind:              productKind,
		GreenBeanType:            req.GreenBeanType,
		GreenBeanBomProductID:    req.GreenBeanBomProductID,
		DripBagGrams:             dripBagGrams,
		DripBoxBagCount:          dripBoxBagCount,
		AllowFulfillmentOrder:    allowFulfillmentOrder,
		AllowFulfillmentOrderSet: req.AllowFulfillmentOrder != nil,
		AllowMallOrder:           allowMallOrder,
		DefaultPrice:             req.DefaultPrice,
		RetailPrice100G:          req.RetailPrice100G,
		RetailPrice200G:          req.RetailPrice200G,
		RetailPrice227G:          req.RetailPrice227G,
		RetailPrice250G:          req.RetailPrice250G,
		YieldRate:                yieldRate,
		ProductConfigTemplateID:  req.ProductConfigTemplateID,
		ClassificationTemplateID: req.ClassificationTemplateID,
		Tiers:                    productTiersFromAPI(req.Tiers),
		SpecialAttrsJSON:         req.SpecialAttrsJSON,
	})
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"product": productOptionFromCatalog(product)})
}

func (h productHandler) createSKUAPI(c echo.Context) error {
	var req skuCreateAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	product, err := h.catalog.CreateSKU(c.Request().Context(), catalogapp.CreateSKUCommand{
		Actor:                    support.ActorOf(c),
		CustomerID:               req.CustomerID,
		Name:                     req.Name,
		Remark:                   req.Remark,
		ProductTypeCategoryID:    req.ProductTypeCategoryID,
		ProductSubtypeCategoryID: req.ProductSubtypeCategoryID,
		SpecialAttrsJSON:         req.SpecialAttrsJSON,
		ProductConfigTemplateID:  req.ProductConfigTemplateID,
		ClassificationTemplateID: req.ClassificationTemplateID,
		Active:                   active,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"product": productOptionFromCatalog(product)})
}

func (h productHandler) skuCopyOptionsAPI(c echo.Context) error {
	targetCustomerID, err := strconv.ParseInt(strings.TrimSpace(c.QueryParam("target_customer_id")), 10, 64)
	if err != nil && strings.TrimSpace(c.QueryParam("target_customer_id")) != "" {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid target_customer_id"})
	}
	sourceCustomerID, err := strconv.ParseInt(strings.TrimSpace(c.QueryParam("source_customer_id")), 10, 64)
	if err != nil && strings.TrimSpace(c.QueryParam("source_customer_id")) != "" {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid source_customer_id"})
	}
	options, err := h.catalog.ListSKUCopyOptions(c.Request().Context(), catalogapp.SKUCopyOptionsQuery{
		TargetCustomerID: targetCustomerID,
		SourceCustomerID: sourceCustomerID,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, options)
}

func (h productHandler) copySKUsAPI(c echo.Context) error {
	var req skuCopyAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	result, err := h.catalog.CopySKUs(c.Request().Context(), catalogapp.CopySKUsCommand{
		Actor:            support.ActorOf(c),
		TargetCustomerID: req.TargetCustomerID,
		SourceCustomerID: req.SourceCustomerID,
		SourceSKUIDs:     req.SourceSKUIDs,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, result)
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

func validateExplicitDripConfig(productKind string, dripBagGrams *float64, dripBoxBagCount *int) error {
	if catalogdomain.NormalizeProductKind(productKind) != catalogdomain.ProductKindDripBag {
		return nil
	}
	if dripBagGrams != nil && *dripBagGrams <= 0 {
		return fmt.Errorf("drip_bag_grams must be > 0")
	}
	if dripBoxBagCount != nil && *dripBoxBagCount <= 0 {
		return fmt.Errorf("drip_box_bag_count must be > 0")
	}
	return nil
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

func (h productHandler) customerProductAliasesAPI(c echo.Context) error {
	customerID, err := parseOptionalInt64(c.QueryParam("customer_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid customer_id"})
	}
	activeParam := strings.ToLower(strings.TrimSpace(c.QueryParam("active")))
	rows, err := h.catalog.ListCustomerProductAliases(c.Request().Context(), catalogapp.CustomerProductAliasQuery{
		CustomerID: customerID,
		ActiveOnly: activeParam == "" || activeParam == "active",
	})
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"rows": rows})
}

func (h productHandler) customerProductAliasMigrationCandidatesAPI(c echo.Context) error {
	customerID, err := parseOptionalInt64(c.QueryParam("customer_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid customer_id"})
	}
	rows, err := h.catalog.ListCustomerProductAliasMigrationCandidates(c.Request().Context(), catalogapp.CustomerProductAliasMigrationCandidateQuery{CustomerID: customerID})
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"rows": rows})
}

func (h productHandler) saveCustomerProductAliasAPI(c echo.Context) error {
	var req customerProductAliasAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	id, err := parseOptionalInt64(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	includeInPriceList := true
	if req.IncludeInPriceList != nil {
		includeInPriceList = *req.IncludeInPriceList
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	row, err := h.catalog.SaveCustomerProductAlias(c.Request().Context(), catalogapp.CustomerProductAliasCommand{
		Actor:                    support.ActorOf(c),
		ID:                       id,
		CustomerID:               req.CustomerID,
		ProductID:                req.ProductID,
		DisplayName:              req.DisplayName,
		CustomerItemCode:         req.CustomerItemCode,
		BrandName:                req.BrandName,
		DisplayCategoryID:        req.DisplayCategoryID,
		ClassificationTemplateID: req.ClassificationTemplateID,
		SortOrder:                req.SortOrder,
		IncludeInPriceList:       includeInPriceList,
		Active:                   active,
		Remark:                   req.Remark,
	})
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"alias": row})
}

func (h productHandler) batchCustomerProductAliasesAPI(c echo.Context) error {
	var req customerProductAliasBatchAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	includeInPriceList := true
	if req.IncludeInPriceList != nil {
		includeInPriceList = *req.IncludeInPriceList
	}
	result, err := h.catalog.BatchCreateCustomerProductAliases(c.Request().Context(), catalogapp.BatchCustomerProductAliasesCommand{
		Actor:                    support.ActorOf(c),
		CustomerID:               req.CustomerID,
		ProductIDs:               req.ProductIDs,
		IncludeInPriceList:       includeInPriceList,
		BrandName:                req.BrandName,
		DisplayCategoryID:        req.DisplayCategoryID,
		ClassificationTemplateID: req.ClassificationTemplateID,
	})
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, result)
}

func (h productHandler) disableCustomerProductAliasAPI(c echo.Context) error {
	id, err := parseOptionalInt64(c.Param("id"))
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	if err := h.catalog.DisableCustomerProductAlias(c.Request().Context(), catalogapp.DisableCustomerProductAliasCommand{Actor: support.ActorOf(c), ID: id}); err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

func parseOptionalInt64(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	return strconv.ParseInt(raw, 10, 64)
}

func (h productHandler) productCategoriesAPI(c echo.Context) error {
	data, err := h.catalog.ProductSettings(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"categories": data.Categories})
}

func (h productHandler) productProductionConfigsAPI(c echo.Context) error {
	rows, err := h.catalog.ListProductProductionConfigs(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"rows": rows})
}

func (h productHandler) productProductionConfigAPI(c echo.Context) error {
	productID, err := strconv.ParseInt(c.Param("product_id"), 10, 64)
	if err != nil || productID <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid product_id"})
	}
	row, err := h.catalog.GetProductProductionConfig(c.Request().Context(), productID)
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"config": row})
}

func (h productHandler) saveProductProductionConfigAPI(c echo.Context) error {
	productID, err := strconv.ParseInt(c.Param("product_id"), 10, 64)
	if err != nil || productID <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid product_id"})
	}
	var req productProductionConfigAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	row, err := h.catalog.SaveProductProductionConfig(c.Request().Context(), catalogapp.SaveProductProductionConfigCommand{
		Actor:                   support.ActorOf(c),
		ProductID:               productID,
		ProductionBomID:         req.ProductionBomID,
		ProductionBomVersionID:  req.ProductionBomVersionID,
		ProcessRouteID:          req.ProcessRouteID,
		IndustryFieldTemplateID: req.IndustryFieldTemplateID,
		ExpectedLossRate:        req.ExpectedLossRate,
		Note:                    req.Note,
		Fields:                  req.Fields,
	})
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"config": row})
}

func (h productHandler) productClassificationTemplatesAPI(c echo.Context) error {
	rows, err := h.catalog.ListProductClassificationTemplates(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"rows": rows})
}

func (h productHandler) saveProductClassificationTemplateAPI(c echo.Context) error {
	var req productClassificationTemplateAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	var id int64
	if idText := c.Param("id"); strings.TrimSpace(idText) != "" {
		parsed, err := strconv.ParseInt(idText, 10, 64)
		if err != nil || parsed <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
		}
		id = parsed
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	row, err := h.catalog.SaveProductClassificationTemplate(c.Request().Context(), catalogapp.SaveProductClassificationTemplateCommand{
		Actor:            support.ActorOf(c),
		ID:               id,
		CustomerID:       req.CustomerID,
		SourceTemplateID: req.SourceTemplateID,
		Name:             req.Name,
		Active:           active,
		SortOrder:        req.SortOrder,
	})
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"template": row})
}

func (h productHandler) deleteProductClassificationTemplateAPI(c echo.Context) error {
	id, err := parseOptionalInt64(c.Param("id"))
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	if err := h.catalog.DeleteProductClassificationTemplate(c.Request().Context(), catalogapp.DeleteProductClassificationTemplateCommand{Actor: support.ActorOf(c), ID: id}); err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

func (h productHandler) saveProductClassificationCategoryAPI(c echo.Context) error {
	var req productClassificationCategoryAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	var id int64
	if idText := c.Param("id"); strings.TrimSpace(idText) != "" {
		parsed, err := strconv.ParseInt(idText, 10, 64)
		if err != nil || parsed <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
		}
		id = parsed
	}
	row, err := h.catalog.SaveProductClassificationCategory(c.Request().Context(), catalogapp.SaveProductClassificationCategoryCommand{
		Actor:      support.ActorOf(c),
		ID:         id,
		TemplateID: req.TemplateID,
		ParentID:   req.ParentID,
		Name:       req.Name,
		Level:      req.Level,
		SortOrder:  req.SortOrder,
	})
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"category": row})
}

func (h productHandler) deleteProductClassificationCategoryAPI(c echo.Context) error {
	id, err := parseOptionalInt64(c.Param("id"))
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	templateID, err := parseOptionalInt64(c.QueryParam("template_id"))
	if err != nil || templateID <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid template_id"})
	}
	if err := h.catalog.DeleteProductClassificationCategory(c.Request().Context(), catalogapp.DeleteProductClassificationCategoryCommand{Actor: support.ActorOf(c), ID: id, TemplateID: templateID}); err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

func (h productHandler) saveProductClassificationAssignmentAPI(c echo.Context) error {
	var req productClassificationAssignmentAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	row, err := h.catalog.SaveProductClassificationAssignment(c.Request().Context(), catalogapp.SaveProductClassificationAssignmentCommand{
		Actor:      support.ActorOf(c),
		ProductID:  req.ProductID,
		TemplateID: req.TemplateID,
		CategoryID: req.CategoryID,
		SortOrder:  req.SortOrder,
	})
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"assignment": row})
}

func (h productHandler) saveCustomerProductAliasClassificationAssignmentAPI(c echo.Context) error {
	var req customerProductAliasClassificationAssignmentAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	row, err := h.catalog.SaveCustomerProductAliasClassificationAssignment(c.Request().Context(), catalogapp.SaveCustomerProductAliasClassificationAssignmentCommand{
		Actor:      support.ActorOf(c),
		AliasID:    req.AliasID,
		TemplateID: req.TemplateID,
		CategoryID: req.CategoryID,
		SortOrder:  req.SortOrder,
	})
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"assignment": row})
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
		Actor:          support.ActorOf(c),
		ID:             id,
		CustomerID:     req.CustomerID,
		Name:           req.Name,
		DisplayUnit:    req.DisplayUnit,
		UnitTemplateID: req.UnitTemplateID,
		Tiers:          req.Tiers,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"template": row})
}

func (h productHandler) saveProductConfigTemplateAPI(c echo.Context) error {
	var req productConfigTemplateAPIRequest
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
	template, err := h.catalog.SaveProductConfigTemplate(c.Request().Context(), catalogapp.SaveProductConfigTemplateCommand{
		Actor:                  support.ActorOf(c),
		ID:                     id,
		CustomerID:             req.CustomerID,
		Name:                   req.Name,
		GradientTemplateID:     req.GradientTemplateID,
		OperationTemplateID:    req.OperationTemplateID,
		UnitTemplateID:         req.UnitTemplateID,
		PriceListRuleJSON:      req.PriceListRuleJSON,
		SpecialAttrsSchemaJSON: req.SpecialAttrsSchemaJSON,
		InventoryUnit:          req.InventoryUnit,
		QuoteUnit:              req.QuoteUnit,
		OrderUnit:              req.OrderUnit,
		UnitConversionJSON:     req.UnitConversionJSON,
		IntegerUnit:            req.IntegerUnit,
		Active:                 req.Active,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"template": template})
}

func (h productHandler) saveProductUnitDefinitionAPI(c echo.Context) error {
	var req productUnitDefinitionAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	if code := strings.TrimSpace(c.Param("code")); code != "" {
		req.Code = code
	}
	row, err := h.catalog.SaveProductUnitDefinition(c.Request().Context(), catalogapp.SaveProductUnitDefinitionCommand{
		Actor:        support.ActorOf(c),
		Code:         req.Code,
		Name:         req.Name,
		UnitType:     req.UnitType,
		AllowDecimal: req.AllowDecimal,
		Active:       req.Active,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"unit": row})
}

func (h productHandler) saveProductUnitTemplateAPI(c echo.Context) error {
	var req productUnitTemplateAPIRequest
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
	template, err := h.catalog.SaveProductUnitTemplate(c.Request().Context(), catalogapp.SaveProductUnitTemplateCommand{
		Actor:              support.ActorOf(c),
		ID:                 id,
		Name:               req.Name,
		InventoryUnit:      req.InventoryUnit,
		QuoteUnit:          req.QuoteUnit,
		OrderUnit:          req.OrderUnit,
		UnitConversionJSON: req.UnitConversionJSON,
		IntegerUnit:        req.IntegerUnit,
		Active:             req.Active,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"template": template})
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
		Actor:                   support.ActorOf(c),
		ID:                      req.ID,
		ParentID:                req.ParentID,
		CustomerID:              req.CustomerID,
		Name:                    req.Name,
		Position:                req.Position,
		ProductConfigTemplateID: req.ProductConfigTemplateID,
		GradientTemplateID:      req.GradientTemplateID,
		OperationTemplateID:     req.OperationTemplateID,
		PriceListRuleJSON:       req.PriceListRuleJSON,
		InventoryUnit:           req.InventoryUnit,
		QuoteUnit:               req.QuoteUnit,
		OrderUnit:               req.OrderUnit,
		UnitConversionJSON:      req.UnitConversionJSON,
		IntegerUnit:             req.IntegerUnit,
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
		Actor:                 support.ActorOf(c),
		CustomerID:            req.CustomerID,
		BaseProductID:         req.BaseProductID,
		Name:                  req.Name,
		Remark:                req.Remark,
		ProductKind:           req.ProductKind,
		GreenBeanType:         req.GreenBeanType,
		GreenBeanBomProductID: req.GreenBeanBomProductID,
		RoastLevel:            req.RoastLevel,
		SpecialAttrsJSON:      req.SpecialAttrsJSON,
		YieldRate:             normalizeProductYieldRate(req.YieldRate),
		DripBagGrams:          req.DripBagGrams,
		DripBoxBagCount:       req.DripBoxBagCount,
		CustomType:            req.CustomType,
		CopyBOM:               req.CopyBOM,
		CopyPriceTiers:        req.CopyPriceTiers,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"product": productOptionFromCatalog(product)})
}

func (h productHandler) deriveProductCategoryAPI(c echo.Context) error {
	var req deriveProductCategoryAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	category, err := h.catalog.DeriveProductCategory(c.Request().Context(), catalogapp.DeriveProductCategoryCommand{
		Actor:            support.ActorOf(c),
		CustomerID:       req.CustomerID,
		SourceCategoryID: req.SourceCategoryID,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"category": category})
}

func (h productHandler) deriveCustomerProductAPI(c echo.Context) error {
	var req deriveCustomerProductAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	product, err := h.catalog.DeriveCustomerProduct(c.Request().Context(), catalogapp.DeriveCustomerProductCommand{
		Actor:          support.ActorOf(c),
		CustomerID:     req.CustomerID,
		BaseProductID:  req.BaseProductID,
		CategoryID:     req.CategoryID,
		Position:       req.Position,
		Name:           req.Name,
		CopyBOM:        req.CopyBOM,
		CopyPriceTiers: req.CopyPriceTiers,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"product": productOptionFromCatalog(product)})
}

func (h productHandler) deriveGradientTemplateAPI(c echo.Context) error {
	var req deriveGradientTemplateAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	template, err := h.catalog.DeriveGradientTemplate(c.Request().Context(), catalogapp.DeriveGradientTemplateCommand{
		Actor:            support.ActorOf(c),
		CustomerID:       req.CustomerID,
		SourceTemplateID: req.SourceTemplateID,
		Name:             req.Name,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"template": template})
}

func (h productHandler) deriveProductConfigTemplateAPI(c echo.Context) error {
	var req deriveProductConfigTemplateAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	template, err := h.catalog.DeriveProductConfigTemplate(c.Request().Context(), catalogapp.DeriveProductConfigTemplateCommand{
		Actor:            support.ActorOf(c),
		CustomerID:       req.CustomerID,
		SourceTemplateID: req.SourceTemplateID,
		Name:             req.Name,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"template": template})
}

func (h productHandler) saveCustomerPublicUsageAPI(c echo.Context) error {
	var req customerPublicUsageAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	usage, err := h.catalog.SaveCustomerPublicUsage(c.Request().Context(), catalogapp.CustomerPublicUsageCommand{
		Actor:                      support.ActorOf(c),
		CustomerID:                 req.CustomerID,
		UsePublicSKU:               req.UsePublicSKU,
		UsePublicCategories:        req.UsePublicCategories,
		UsePublicGradientTemplates: req.UsePublicGradientTemplates,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"usage": usage})
}

func (h productHandler) saveCustomerProductRuleTemplateAPI(c echo.Context) error {
	var req customerProductRuleTemplateAPIRequest
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
	template, err := h.catalog.SaveCustomerProductRuleTemplate(c.Request().Context(), catalogapp.SaveCustomerProductRuleTemplateCommand{
		Actor:      support.ActorOf(c),
		ID:         id,
		CustomerID: req.CustomerID,
		Name:       req.Name,
		Active:     req.Active,
		Items:      req.Items,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"template": template})
}

func (h productHandler) saveCustomerProductRuleOverrideAPI(c echo.Context) error {
	var req customerProductRuleOverrideAPIRequest
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
	override, err := h.catalog.SaveCustomerProductRuleOverride(c.Request().Context(), catalogapp.SaveCustomerProductRuleOverrideCommand{
		Actor:                    support.ActorOf(c),
		ID:                       id,
		CustomerID:               req.CustomerID,
		ProductSubtypeCategoryID: req.ProductSubtypeCategoryID,
		GradientTemplateID:       req.GradientTemplateID,
		OperationTemplateID:      req.OperationTemplateID,
		PriceListRuleJSON:        req.PriceListRuleJSON,
		UnitRuleJSON:             req.UnitRuleJSON,
		Active:                   req.Active,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"override": override})
}

func (h productHandler) bindCustomerProductRuleTemplateAPI(c echo.Context) error {
	customerID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || customerID <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid customer id"})
	}
	var req customerProductRuleTemplateBindingAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	binding, err := h.catalog.BindCustomerProductRuleTemplate(c.Request().Context(), catalogapp.CustomerProductRuleTemplateBindingCommand{
		Actor:      support.ActorOf(c),
		CustomerID: customerID,
		TemplateID: req.TemplateID,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"binding": binding})
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
	assignment, err := h.catalog.AssignProductCategory(c.Request().Context(), catalogapp.AssignProductCategoryCommand{
		Actor:                support.ActorOf(c),
		ProductID:            id,
		CategoryID:           req.CategoryID,
		CustomerID:           req.CustomerID,
		Position:             req.Position,
		DerivePublicCategory: req.DerivePublicCategory,
		DerivePublicProduct:  req.DerivePublicProduct,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true, "assignment": assignment})
}

func (h productHandler) edit(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return echo.ErrNotFound
	}
	return support.VueShellRedirectWith(c, "productSettings", map[string]string{"edit_id": strconv.FormatInt(id, 10)})
}
