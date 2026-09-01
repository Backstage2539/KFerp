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
	registerBusinessGroupRoutes(e, h)
	e.GET("/api/product-customer-references", h.productCustomerReferencesAPI)
	e.POST("/api/product-customer-references", h.saveProductCustomerReferenceAPI)
	e.PUT("/api/product-customer-references/:id", h.saveProductCustomerReferenceAPI)
	registerPricingRoutes(e, h)
	e.GET("/api/customer-product-aliases", h.customerProductAliasesAPI)
	e.GET("/api/customer-product-aliases/migration-candidates", h.customerProductAliasMigrationCandidatesAPI)
	e.POST("/api/customer-product-aliases/batch", h.batchCustomerProductAliasesAPI)
	e.POST("/api/customer-product-aliases/batch-disable", h.batchDisableCustomerProductAliasesAPI)
	e.POST("/api/customer-product-aliases", h.saveCustomerProductAliasAPI)
	e.PUT("/api/customer-product-aliases/:id", h.saveCustomerProductAliasAPI)
	e.GET("/api/customer-product-aliases/:id/industry-fields", h.customerProductAliasIndustryFieldsAPI)
	e.PUT("/api/customer-product-aliases/:id/industry-fields", h.saveCustomerProductAliasIndustryFieldsAPI)
	e.POST("/api/customer-product-aliases/:id/disable", h.disableCustomerProductAliasAPI)
	e.GET("/api/product-settings/categories", h.productCategoriesAPI)
	e.GET("/api/product-production-configs", h.productProductionConfigsAPI)
	e.GET("/api/product-production-configs/:product_id", h.productProductionConfigAPI)
	e.POST("/api/product-production-configs/:product_id", h.saveProductProductionConfigAPI)
	e.PUT("/api/product-production-configs/:product_id", h.saveProductProductionConfigAPI)
	registerClassificationRoutes(e, h)
	e.POST("/api/product-settings/product-config-templates", h.saveProductConfigTemplateAPI)
	e.PUT("/api/product-settings/product-config-templates/:id", h.saveProductConfigTemplateAPI)
	e.DELETE("/api/product-settings/product-config-templates/:id", h.deleteProductConfigTemplateAPI)
	e.POST("/api/product-settings/product-config-templates/derive", h.deriveProductConfigTemplateAPI)
	e.GET("/api/product-settings/units", h.productUnitDefinitionsAPI)
	e.POST("/api/product-settings/units", h.saveProductUnitDefinitionAPI)
	e.PUT("/api/product-settings/units/:code", h.saveProductUnitDefinitionAPI)
	e.DELETE("/api/product-settings/units/:code", h.deleteProductUnitDefinitionAPI)
	e.POST("/api/product-settings/products", h.createProductAPI)
	e.POST("/api/product-settings/products/:id/copy", h.copyProductAPI)
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

func (h productHandler) retiredSalesSpecWriteAPI(c echo.Context) error {
	return c.JSON(http.StatusGone, map[string]any{
		"error":   "销售规格、默认子 SKU 和销售规格模板已下线",
		"code":    "bom_spec_authority_required",
		"message": "请在商品的默认已发布 BOM 中维护规格；全局单位请使用 /api/product-settings/units",
	})
}

type productDefaultSKUAPIRequest struct {
	SKUID        int64 `json:"sku_id"`
	DefaultSKUID int64 `json:"default_sku_id"`
}

const (
	customerProductsLegacyReadonlyError      = "customer products are legacy readonly"
	productPriceRecordsLegacyReadonlyError   = "product price records are legacy readonly"
	productClassificationLegacyReadonlyError = "classification write APIs are legacy readonly"
)

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
	InventoryUnit               *string                   `json:"inventory_unit"`
	IntegerInventoryUnit        *bool                     `json:"integer_inventory_unit"`
	DefaultSalesUnit            *string                   `json:"default_sales_unit"`
	UnitConversionJSON          json.RawMessage           `json:"unit_conversion_json"`
	SalesUnitRulesJSON          json.RawMessage           `json:"sales_unit_rules"`
	UnitTemplateID              *int64                    `json:"unit_template_id"`
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
	InventoryUnit            *string                   `json:"inventory_unit"`
	IntegerInventoryUnit     *bool                     `json:"integer_inventory_unit"`
	DefaultSalesUnit         *string                   `json:"default_sales_unit"`
	UnitConversionJSON       json.RawMessage           `json:"unit_conversion_json"`
	SalesUnitRulesJSON       json.RawMessage           `json:"sales_unit_rules"`
	UnitTemplateID           *int64                    `json:"unit_template_id"`
	Tiers                    []productTierAPIUpsertRow `json:"tiers"`
}

type skuCreateAPIRequest struct {
	CustomerID               int64           `json:"customer_id"`
	ParentProductID          int64           `json:"parent_product_id"`
	Name                     string          `json:"name"`
	SKUName                  string          `json:"sku_name"`
	SKUCode                  string          `json:"sku_code"`
	Barcode                  string          `json:"barcode"`
	SpecLabel                string          `json:"spec_label"`
	NetContentQty            float64         `json:"net_content_qty"`
	NetContentUnit           string          `json:"net_content_unit"`
	IsDefaultSKU             *bool           `json:"is_default_sku"`
	Remark                   string          `json:"remark"`
	ProductTypeCategoryID    int64           `json:"product_type_category_id"`
	ProductSubtypeCategoryID int64           `json:"product_subtype_category_id"`
	SpecialAttrsJSON         string          `json:"special_attrs_json"`
	ProductConfigTemplateID  int64           `json:"product_config_template_id"`
	ClassificationTemplateID int64           `json:"classification_template_id"`
	InventoryUnit            *string         `json:"inventory_unit"`
	IntegerInventoryUnit     *bool           `json:"integer_inventory_unit"`
	DefaultSalesUnit         *string         `json:"default_sales_unit"`
	UnitConversionJSON       json.RawMessage `json:"unit_conversion_json"`
	SalesUnitRulesJSON       json.RawMessage `json:"sales_unit_rules"`
	UnitTemplateID           int64           `json:"unit_template_id"`
	Active                   *bool           `json:"active"`
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

var retiredProductMasterFields = []string{
	"sku_id", "parent_product_id", "effective_parent_product_id", "sku_name", "sku_code", "barcode",
	"spec_label", "net_content_qty", "net_content_unit", "is_default_sku", "default_sku_id",
	"effective_default_sku_id", "default_spec_label", "auto_derived_sku", "derived_unit_template_id",
	"derived_spec_key", "derived_spec_name", "derived_sales_unit", "derived_spec_status", "sales_units",
	"unit_rule_override_json", "inventory_unit", "integer_inventory_unit", "default_sales_unit",
	"unit_conversion_json", "sales_unit_rules", "sales_unit_rules_json", "unit_template_id", "unit_template_name", "unit_rule_source",
	"retail_specs", "tiers", "spec_identity_mode", "bom_spec_authoritative", "migration_state", "legacy_catalog_product",
}

func productMasterPayload(value any) any {
	raw, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return value
	}
	cleaned, _ := sanitizeProductMasterPayload(decoded)
	return cleaned
}

func sanitizeProductMasterPayload(value any) (any, bool) {
	switch typed := value.(type) {
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			cleaned, drop := sanitizeProductMasterPayload(item)
			if !drop {
				out = append(out, cleaned)
			}
		}
		return out, false
	case map[string]any:
		if _, isProduct := typed["product_kind"]; isProduct {
			if parentID, ok := typed["parent_product_id"].(float64); ok && parentID > 0 {
				return nil, true
			}
			for _, key := range retiredProductMasterFields {
				delete(typed, key)
			}
		}
		for key, item := range typed {
			cleaned, _ := sanitizeProductMasterPayload(item)
			typed[key] = cleaned
		}
		return typed, false
	default:
		return value, false
	}
}

func productUpdateContainsLegacyUnitFields(req productUpdateAPIRequest) bool {
	return req.UnitRuleOverrideJSON != nil || req.InventoryUnit != nil || req.IntegerInventoryUnit != nil ||
		req.DefaultSalesUnit != nil || len(req.UnitConversionJSON) > 0 || len(req.SalesUnitRulesJSON) > 0 ||
		req.UnitTemplateID != nil || len(req.Tiers) > 0
}

func productCreateContainsLegacyUnitFields(req productCreateAPIRequest) bool {
	return req.InventoryUnit != nil || req.IntegerInventoryUnit != nil || req.DefaultSalesUnit != nil ||
		len(req.UnitConversionJSON) > 0 || len(req.SalesUnitRulesJSON) > 0 || req.UnitTemplateID != nil || len(req.Tiers) > 0
}

func productUnitOwnedByBOMSpecError(c echo.Context) error {
	return c.JSON(http.StatusConflict, map[string]any{
		"error":   "商品规格、条码和库存单位由默认已发布 BOM 规格维护",
		"code":    "product_unit_owned_by_bom_spec",
		"message": "请到生产 BOM 配置规格；商品档案只保存商品身份",
	})
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
	ProductConfigTemplateID  int64  `json:"product_config_template_id"`
	GradientTemplateID       int64  `json:"gradient_template_id"`
	UnitTemplateID           int64  `json:"unit_template_id"`
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

type customerProductAliasBatchDisableAPIRequest struct {
	IDs []int64 `json:"ids"`
}

type customerProductAliasIndustryFieldsAPIRequest struct {
	Fields []catalogapp.ProductProductionConfigField `json:"fields"`
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
	CustomerID          int64                             `json:"customer_id"`
	Name                string                            `json:"name"`
	DisplayUnit         string                            `json:"display_unit"`
	UnitTemplateID      int64                             `json:"unit_template_id"`
	AllowCustomerResale bool                              `json:"allow_customer_resale"`
	Tiers               []catalogapp.GradientTemplateTier `json:"tiers"`
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
	ProductionBomID          int64                                     `json:"production_bom_id"`
	ProductionBomVersionID   int64                                     `json:"production_bom_version_id"`
	ProcessRouteID           int64                                     `json:"process_route_id"`
	IndustryFieldTemplateID  int64                                     `json:"industry_field_template_id"`
	IndustryFieldTemplateIDs json.RawMessage                           `json:"industry_field_template_ids"`
	ExpectedLossRate         float64                                   `json:"expected_loss_rate"`
	Note                     string                                    `json:"note"`
	Fields                   []catalogapp.ProductProductionConfigField `json:"fields"`
}

type productClassificationTemplateAPIRequest struct {
	CustomerID              int64  `json:"customer_id"`
	SourceTemplateID        int64  `json:"source_template_id"`
	Name                    string `json:"name"`
	Remark                  string `json:"remark"`
	ProductConfigTemplateID int64  `json:"product_config_template_id"`
	GradientTemplateID      int64  `json:"gradient_template_id"`
	UnitTemplateID          int64  `json:"unit_template_id"`
	Active                  *bool  `json:"active"`
	SortOrder               int    `json:"sort_order"`
}

type productClassificationCategoryAPIRequest struct {
	TemplateID              int64  `json:"template_id"`
	ParentID                int64  `json:"parent_id"`
	Name                    string `json:"name"`
	Level                   int    `json:"level"`
	SortOrder               int    `json:"sort_order"`
	ProductConfigTemplateID int64  `json:"product_config_template_id"`
	GradientTemplateID      int64  `json:"gradient_template_id"`
	UnitTemplateID          int64  `json:"unit_template_id"`
}

type productClassificationAssignmentAPIRequest struct {
	ProductID  int64 `json:"product_id"`
	TemplateID int64 `json:"template_id"`
	CategoryID int64 `json:"category_id"`
	SortOrder  int   `json:"sort_order"`
}

type productClassificationTemplateUsageAPIRequest struct {
	CustomerID               int64 `json:"customer_id"`
	ClassificationTemplateID int64 `json:"classification_template_id"`
	SortOrder                int   `json:"sort_order"`
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
	Name               string                        `json:"name"`
	InventoryUnit      string                        `json:"inventory_unit"`
	SalesUnit          string                        `json:"sales_unit"`
	DefaultSalesUnit   string                        `json:"default_sales_unit"`
	SalesUnits         []string                      `json:"sales_units"`
	SalesSpecs         []catalogapp.ProductSalesSpec `json:"sales_specs"`
	QuoteUnit          string                        `json:"quote_unit"`
	OrderUnit          string                        `json:"order_unit"`
	UnitConversionJSON string                        `json:"unit_conversion_json"`
	IntegerUnit        bool                          `json:"integer_unit"`
	Active             *bool                         `json:"active"`
}

type productPriceGroupAPIRequest struct {
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
	Active    *bool  `json:"active"`
}

type productPriceRecordAPIRequest struct {
	ProductID               int64           `json:"product_id"`
	CustomerProductAliasID  int64           `json:"customer_product_alias_id"`
	FinalUnitPrice          float64         `json:"final_unit_price"`
	PriceUnit               string          `json:"price_unit"`
	Currency                string          `json:"currency"`
	PriceGroupID            int64           `json:"price_group_id"`
	PriceGroupName          string          `json:"price_group_name"`
	InventoryUnit           string          `json:"inventory_unit"`
	InventoryConversionJSON json.RawMessage `json:"inventory_conversion_json"`
	Status                  string          `json:"status"`
	Remark                  string          `json:"remark"`
	Active                  *bool           `json:"active"`
}

type productTierPriceSchemeAPIRequest struct {
	Name                   string                                  `json:"name"`
	ProductID              int64                                   `json:"product_id"`
	CustomerProductAliasID int64                                   `json:"customer_product_alias_id"`
	PriceGroupID           int64                                   `json:"price_group_id"`
	Active                 *bool                                   `json:"active"`
	Remark                 string                                  `json:"remark"`
	Tiers                  []catalogapp.ProductTierPriceSchemeTier `json:"tiers"`
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
	return c.JSON(http.StatusOK, map[string]any{"rows": productMasterPayload(productOptionsFromCatalog(ps))})
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
	return c.JSON(http.StatusOK, map[string]any{"product": productMasterPayload(productOptionFromCatalog(*p))})
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
	if productUpdateContainsLegacyUnitFields(req) {
		return productUnitOwnedByBOMSpecError(c)
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
	yieldRate := existing.YieldRate
	if req.YieldRate > 0 {
		yieldRate = normalizeProductYieldRate(req.YieldRate)
	}
	if catalogdomain.ProductKindSupportsBomParams(productKind) && req.YieldRate > 0 && yieldRate <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid yield_rate"})
	}
	marginRateOverride := existing.MarginRateOverride
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
	gradientTemplateIDOverride := int64(0)
	operationTemplateIDOverride := int64(0)
	productConfigTemplateID := int64(0)
	classificationTemplateID := int64(0)
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
		UnitTemplateID:              0,
		UnitRuleOverrideJSON:        "{}",
		ProductConfigTemplateID:     productConfigTemplateID,
		ClassificationTemplateID:    classificationTemplateID,
		SpecialAttrsJSON:            specialAttrsJSON,
	}); err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	p, err := h.catalog.GetProduct(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if p == nil {
		return c.JSON(http.StatusNotFound, map[string]any{"error": "not found"})
	}
	return c.JSON(http.StatusOK, map[string]any{"product": productMasterPayload(productOptionFromCatalog(*p))})
}

func optionalFloat64(value *float64, fallback float64) float64 {
	if value == nil {
		return fallback
	}
	return *value
}

func int64Value(value *int64) int64 {
	if value == nil {
		return 0
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

func productInventoryUnitRuleJSON(raw string, inventoryUnit *string, integerInventoryUnit *bool, defaultSalesUnit *string, unitConversionJSON json.RawMessage, salesUnitRulesJSON json.RawMessage, defaultInventoryUnit string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "{}"
	}
	rule := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &rule); err != nil {
		return "", err
	}
	if rule == nil {
		rule = map[string]any{}
	}
	if inventoryUnit != nil {
		unit := strings.TrimSpace(*inventoryUnit)
		if unit == "" {
			unit = strings.TrimSpace(defaultInventoryUnit)
		}
		if unit == "" {
			unit = "kg"
		}
		rule["inventory_unit"] = unit
	}
	if integerInventoryUnit != nil {
		rule["integer_inventory_unit"] = *integerInventoryUnit
	}
	if defaultSalesUnit != nil {
		unit := strings.TrimSpace(*defaultSalesUnit)
		if unit == "" {
			unit = strings.TrimSpace(firstNonEmptyString(stringValueFromRule(rule, "inventory_unit"), defaultInventoryUnit, "kg"))
		}
		rule["default_sales_unit"] = unit
	}
	if len(unitConversionJSON) > 0 {
		conversion, err := rawJSONObject(unitConversionJSON)
		if err != nil {
			return "", err
		}
		rule["unit_conversion_json"] = conversion
	}
	if len(salesUnitRulesJSON) > 0 {
		salesRules, err := rawJSONObject(salesUnitRulesJSON)
		if err != nil {
			return "", err
		}
		rule["sales_unit_rules"] = salesRules
	}
	effectiveInventoryUnit := stringValueFromRule(rule, "inventory_unit")
	effectiveDefaultSalesUnit := stringValueFromRule(rule, "default_sales_unit")
	if effectiveInventoryUnit != "" && effectiveDefaultSalesUnit == effectiveInventoryUnit {
		conversion, _ := rule["unit_conversion_json"].(map[string]any)
		if conversion == nil {
			conversion = map[string]any{}
		}
		inventoryBySalesUnit, _ := conversion[effectiveDefaultSalesUnit].(map[string]any)
		if inventoryBySalesUnit == nil {
			inventoryBySalesUnit = map[string]any{}
		}
		if _, exists := inventoryBySalesUnit[effectiveInventoryUnit]; !exists {
			inventoryBySalesUnit[effectiveInventoryUnit] = 1
		}
		conversion[effectiveDefaultSalesUnit] = inventoryBySalesUnit
		rule["unit_conversion_json"] = conversion
	}
	encoded, err := json.Marshal(rule)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func stringValueFromRule(rule map[string]any, key string) string {
	if value, ok := rule[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func rawJSONObject(raw json.RawMessage) (map[string]any, error) {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 || string(raw) == "null" {
		return map[string]any{}, nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	if text, ok := value.(string); ok {
		var out map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &out); err != nil {
			return nil, err
		}
		if out == nil {
			return map[string]any{}, nil
		}
		return out, nil
	}
	out, ok := value.(map[string]any)
	if !ok || out == nil {
		return nil, fmt.Errorf("expected json object")
	}
	return out, nil
}

func stringPtrOrNil(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func (h productHandler) createProductAPI(c echo.Context) error {
	var req productCreateAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	if productCreateContainsLegacyUnitFields(req) {
		return productUnitOwnedByBOMSpecError(c)
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
		ProductConfigTemplateID:  0,
		ClassificationTemplateID: 0,
		UnitTemplateID:           0,
		Tiers:                    nil,
		SpecialAttrsJSON:         req.SpecialAttrsJSON,
		UnitRuleOverrideJSON:     "{}",
	})
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"product": productMasterPayload(productOptionFromCatalog(product))})
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
	isDefaultSKU := false
	if req.IsDefaultSKU != nil {
		isDefaultSKU = *req.IsDefaultSKU
	}
	unitRuleOverrideJSON, err := productInventoryUnitRuleJSON("{}", req.InventoryUnit, req.IntegerInventoryUnit, req.DefaultSalesUnit, req.UnitConversionJSON, req.SalesUnitRulesJSON, "kg")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid unit_rule_override_json"})
	}
	product, err := h.catalog.CreateSKU(c.Request().Context(), catalogapp.CreateSKUCommand{
		Actor:                    support.ActorOf(c),
		CustomerID:               req.CustomerID,
		ParentProductID:          req.ParentProductID,
		Name:                     req.Name,
		SKUName:                  req.SKUName,
		SKUCode:                  req.SKUCode,
		Barcode:                  req.Barcode,
		SpecLabel:                req.SpecLabel,
		NetContentQty:            req.NetContentQty,
		NetContentUnit:           req.NetContentUnit,
		IsDefaultSKU:             isDefaultSKU,
		Remark:                   req.Remark,
		ProductTypeCategoryID:    req.ProductTypeCategoryID,
		ProductSubtypeCategoryID: req.ProductSubtypeCategoryID,
		SpecialAttrsJSON:         req.SpecialAttrsJSON,
		ProductConfigTemplateID:  0,
		ClassificationTemplateID: 0,
		UnitTemplateID:           req.UnitTemplateID,
		UnitRuleOverrideJSON:     unitRuleOverrideJSON,
		Active:                   active,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"product": productOptionFromCatalog(product)})
}

func (h productHandler) setProductDefaultSKUAPI(c echo.Context) error {
	parentID, err := parseOptionalInt64(c.Param("id"))
	if err != nil || parentID <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid product id"})
	}
	var req productDefaultSKUAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	if req.SKUID <= 0 {
		req.SKUID = req.DefaultSKUID
	}
	product, err := h.catalog.SetProductDefaultSKU(c.Request().Context(), catalogapp.SetProductDefaultSKUCommand{
		Actor:           support.ActorOf(c),
		ParentProductID: parentID,
		SKUID:           req.SKUID,
	})
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
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

func (h productHandler) copyProductAPI(c echo.Context) error {
	sourceProductID, err := parseOptionalInt64(c.Param("id"))
	if err != nil || sourceProductID <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid product id"})
	}
	product, err := h.catalog.CopyProduct(c.Request().Context(), catalogapp.CopyProductCommand{
		Actor:           support.ActorOf(c),
		SourceProductID: sourceProductID,
	})
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"product": productOptionFromCatalog(product)})
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
	return c.JSON(http.StatusOK, productMasterPayload(data))
}

func (h productHandler) businessGroupsAPI(c echo.Context) error {
	rows, err := h.catalog.ListBusinessGroups(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	usageKey := strings.TrimSpace(c.QueryParam("usage_key"))
	if usageKey != "" {
		if strings.EqualFold(usageKey, catalogapp.BusinessGroupUsagePriceList) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "price_list follows product_catalog feature selection"})
		}
		filtered := make([]catalogapp.BusinessGroup, 0, len(rows))
		for _, row := range rows {
			if !row.Active {
				continue
			}
			for _, usage := range row.Usages {
				if strings.EqualFold(usage.UsageKey, usageKey) && usage.Active {
					filtered = append(filtered, row)
					break
				}
			}
		}
		rows = filtered
	}
	return c.JSON(http.StatusOK, map[string]any{"groups": rows, "rows": rows})
}

func (h productHandler) saveBusinessGroupAPI(c echo.Context) error {
	var req catalogapp.BusinessGroup
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	id, err := parseOptionalInt64(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	req.ID = id
	req.Actor = support.ActorOf(c)
	row, err := h.catalog.SaveBusinessGroup(c.Request().Context(), req)
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"group": row})
}

func (h productHandler) deleteBusinessGroupAPI(c echo.Context) error {
	id, err := parseOptionalInt64(c.Param("id"))
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	if err := h.catalog.DeleteBusinessGroup(c.Request().Context(), catalogapp.DeleteBusinessGroupCommand{ID: id, Actor: support.ActorOf(c)}); err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

func (h productHandler) saveBusinessGroupItemAPI(c echo.Context) error {
	var req catalogapp.BusinessGroupItem
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	id, err := parseOptionalInt64(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	req.ID = id
	req.Actor = support.ActorOf(c)
	row, err := h.catalog.SaveBusinessGroupItem(c.Request().Context(), req)
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"item": row})
}

func (h productHandler) deleteBusinessGroupItemAPI(c echo.Context) error {
	id, err := parseOptionalInt64(c.Param("id"))
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	if err := h.catalog.DeleteBusinessGroupItem(c.Request().Context(), catalogapp.DeleteBusinessGroupItemCommand{ID: id, Actor: support.ActorOf(c)}); err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

func (h productHandler) moveBusinessGroupItemAPI(c echo.Context) error {
	id, err := parseOptionalInt64(c.Param("id"))
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	var req catalogapp.MoveBusinessGroupItemCommand
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	req.ID = id
	req.Actor = support.ActorOf(c)
	row, err := h.catalog.MoveBusinessGroupItem(c.Request().Context(), req)
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"item": row})
}

func (h productHandler) ensureBusinessGroupUsageAPI(c echo.Context) error {
	id, err := parseOptionalInt64(c.Param("id"))
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	var req struct {
		UsageKey string `json:"usage_key"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	if req.UsageKey == "" {
		req.UsageKey = c.QueryParam("usage_key")
	}
	if err := h.catalog.EnsureBusinessGroupUsage(c.Request().Context(), id, req.UsageKey, support.ActorOf(c)); err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

func (h productHandler) businessGroupFeatureSelectionAPI(c echo.Context) error {
	selection, err := h.catalog.GetBusinessGroupFeatureSelection(c.Request().Context(), c.Param("feature_key"))
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, selection)
}

func (h productHandler) saveBusinessGroupFeatureSelectionAPI(c echo.Context) error {
	var req struct {
		FeatureKey       string          `json:"feature_key"`
		GroupTemplateIDs json.RawMessage `json:"group_template_ids"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	featureKey := strings.ToLower(strings.TrimSpace(c.Param("feature_key")))
	if req.FeatureKey != "" && !strings.EqualFold(strings.TrimSpace(req.FeatureKey), featureKey) {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "feature_key mismatch"})
	}
	rawIDs := strings.TrimSpace(string(req.GroupTemplateIDs))
	if rawIDs == "" || rawIDs == "null" {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "group_template_ids required"})
	}
	var groupTemplateIDs []int64
	if err := json.Unmarshal(req.GroupTemplateIDs, &groupTemplateIDs); err != nil || groupTemplateIDs == nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid group_template_ids"})
	}
	selection, err := h.catalog.SaveBusinessGroupFeatureSelection(c.Request().Context(), catalogapp.SaveBusinessGroupFeatureSelectionCommand{
		Actor:            support.ActorOf(c),
		FeatureKey:       featureKey,
		GroupTemplateIDs: groupTemplateIDs,
	})
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, selection)
}

func (h productHandler) businessGroupAssignmentsAPI(c echo.Context) error {
	objectID, err := parseOptionalInt64(c.QueryParam("object_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid object_id"})
	}
	groupID, err := parseOptionalInt64(c.QueryParam("group_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid group_id"})
	}
	groupItemID, err := parseOptionalInt64(c.QueryParam("group_item_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid group_item_id"})
	}
	rows, err := h.catalog.ListBusinessGroupAssignments(c.Request().Context(), catalogapp.BusinessGroupAssignmentQuery{
		UsageKey:    c.QueryParam("usage_key"),
		ObjectKey:   c.QueryParam("object_key"),
		ObjectID:    objectID,
		ObjectRef:   c.QueryParam("object_ref"),
		GroupID:     groupID,
		GroupItemID: groupItemID,
	})
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"assignments": rows, "rows": rows})
}

func (h productHandler) saveBusinessGroupAssignmentAPI(c echo.Context) error {
	var req catalogapp.BusinessGroupAssignment
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	id, err := parseOptionalInt64(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	req.ID = id
	req.Actor = support.ActorOf(c)
	row, err := h.catalog.SaveBusinessGroupAssignment(c.Request().Context(), req)
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"assignment": row})
}

func (h productHandler) deleteBusinessGroupAssignmentAPI(c echo.Context) error {
	id, err := parseOptionalInt64(c.Param("id"))
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	if err := h.catalog.DeleteBusinessGroupAssignment(c.Request().Context(), catalogapp.DeleteBusinessGroupAssignmentCommand{ID: id, Actor: support.ActorOf(c)}); err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

func (h productHandler) productCustomerReferencesAPI(c echo.Context) error {
	productID, err := parseOptionalInt64(c.QueryParam("product_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid product_id"})
	}
	rows, err := h.catalog.ListProductCustomerReferences(c.Request().Context(), productID)
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"references": rows, "rows": rows})
}

func (h productHandler) saveProductCustomerReferenceAPI(c echo.Context) error {
	var req catalogapp.ProductCustomerReference
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	id, err := parseOptionalInt64(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	req.ID = id
	req.Actor = support.ActorOf(c)
	row, err := h.catalog.SaveProductCustomerReference(c.Request().Context(), req)
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"reference": row})
}

func (h productHandler) productPricingRulesAPI(c echo.Context) error {
	rows, err := h.catalog.ListProductPricingRules(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"rules": rows, "rows": rows})
}

func (h productHandler) saveProductPricingRuleAPI(c echo.Context) error {
	var req catalogapp.ProductPricingRule
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	id, err := parseOptionalInt64(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	req.ID = id
	req.Actor = support.ActorOf(c)
	row, err := h.catalog.SaveProductPricingRule(c.Request().Context(), req)
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"rule": row})
}

func (h productHandler) priceTierTemplatesAPI(c echo.Context) error {
	rows, err := h.catalog.ListPriceTierTemplates(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"templates": rows, "rows": rows})
}

func (h productHandler) savePriceTierTemplateAPI(c echo.Context) error {
	var req catalogapp.PriceTierTemplate
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	id, err := parseOptionalInt64(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	req.ID = id
	req.Actor = support.ActorOf(c)
	row, err := h.catalog.SavePriceTierTemplate(c.Request().Context(), req)
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"template": row})
}

func (h productHandler) deletePriceTierTemplateAPI(c echo.Context) error {
	id, err := parseOptionalInt64(c.Param("id"))
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	if err := h.catalog.DeletePriceTierTemplate(c.Request().Context(), id, support.ActorOf(c)); err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true, "id": id, "active": false})
}

func (h productHandler) productUnitDefinitionsAPI(c echo.Context) error {
	rows, err := h.catalog.ListProductUnitDefinitions(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, rows)
}

func (h productHandler) productPriceGroupsAPI(c echo.Context) error {
	rows, err := h.catalog.ListProductPriceGroups(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"groups": rows, "rows": rows})
}

func (h productHandler) saveProductPriceGroupAPI(c echo.Context) error {
	var req productPriceGroupAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	id, err := parseOptionalInt64(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	row, err := h.catalog.SaveProductPriceGroup(c.Request().Context(), catalogapp.SaveProductPriceGroupCommand{
		Actor:     support.ActorOf(c),
		ID:        id,
		Name:      req.Name,
		SortOrder: req.SortOrder,
		Active:    req.Active,
	})
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"group": row})
}

func (h productHandler) productPriceRecordsAPI(c echo.Context) error {
	productID, err := parseOptionalInt64(c.QueryParam("product_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid product_id"})
	}
	aliasID, err := parseOptionalInt64(c.QueryParam("customer_product_alias_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid customer_product_alias_id"})
	}
	groupID, err := parseOptionalInt64(c.QueryParam("price_group_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid price_group_id"})
	}
	records, err := h.catalog.ListProductPriceRecords(c.Request().Context(), catalogapp.ProductPriceRecordQuery{
		ProductID:              productID,
		CustomerProductAliasID: aliasID,
		PriceGroupID:           groupID,
		ActiveMode:             c.QueryParam("active"),
		Status:                 c.QueryParam("status"),
	})
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	groups, err := h.catalog.ListProductPriceGroups(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"records": records, "rows": records, "groups": groups})
}

func (h productHandler) saveProductPriceRecordAPI(c echo.Context) error {
	var req productPriceRecordAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	id, err := parseOptionalInt64(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	inventoryConversionJSON, err := rawJSONText(req.InventoryConversionJSON, "{}")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid inventory_conversion_json"})
	}
	row, err := h.catalog.SaveProductPriceRecord(c.Request().Context(), catalogapp.SaveProductPriceRecordCommand{
		Actor:                   support.ActorOf(c),
		ID:                      id,
		ProductID:               req.ProductID,
		CustomerProductAliasID:  req.CustomerProductAliasID,
		FinalUnitPrice:          req.FinalUnitPrice,
		PriceUnit:               req.PriceUnit,
		Currency:                req.Currency,
		PriceGroupID:            req.PriceGroupID,
		PriceGroupName:          req.PriceGroupName,
		InventoryUnit:           req.InventoryUnit,
		InventoryConversionJSON: inventoryConversionJSON,
		Status:                  req.Status,
		Remark:                  req.Remark,
		Active:                  req.Active,
	})
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"record": row})
}

func (h productHandler) productTierPriceSchemesAPI(c echo.Context) error {
	productID, err := parseOptionalInt64(c.QueryParam("product_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid product_id"})
	}
	aliasID, err := parseOptionalInt64(c.QueryParam("customer_product_alias_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid customer_product_alias_id"})
	}
	groupID, err := parseOptionalInt64(c.QueryParam("price_group_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid price_group_id"})
	}
	rows, err := h.catalog.ListProductTierPriceSchemes(c.Request().Context(), catalogapp.ProductTierPriceSchemeQuery{
		ProductID:              productID,
		CustomerProductAliasID: aliasID,
		PriceGroupID:           groupID,
		ActiveMode:             c.QueryParam("active"),
	})
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"schemes": rows, "rows": rows})
}

func (h productHandler) saveProductTierPriceSchemeAPI(c echo.Context) error {
	var req productTierPriceSchemeAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	id, err := parseOptionalInt64(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	row, err := h.catalog.SaveProductTierPriceScheme(c.Request().Context(), catalogapp.SaveProductTierPriceSchemeCommand{
		Actor:                  support.ActorOf(c),
		ID:                     id,
		Name:                   req.Name,
		ProductID:              req.ProductID,
		CustomerProductAliasID: req.CustomerProductAliasID,
		PriceGroupID:           req.PriceGroupID,
		Active:                 req.Active,
		Remark:                 req.Remark,
		Tiers:                  req.Tiers,
	})
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"scheme": row})
}

func (h productHandler) customerProductAliasesAPI(c echo.Context) error {
	customerID, err := parseOptionalInt64(c.QueryParam("customer_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid customer_id"})
	}
	activeParam := strings.ToLower(strings.TrimSpace(c.QueryParam("active")))
	if activeParam == "" {
		activeParam = "active"
	}
	rows, err := h.catalog.ListCustomerProductAliases(c.Request().Context(), catalogapp.CustomerProductAliasQuery{
		CustomerID:  customerID,
		ActiveOnly:  activeParam == "active",
		ActiveMode:  activeParam,
		SearchQuery: c.QueryParam("q"),
	})
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"rows": rows})
}

func (h productHandler) batchDisableCustomerProductAliasesAPI(c echo.Context) error {
	var req customerProductAliasBatchDisableAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	result, err := h.catalog.BatchDisableCustomerProductAliases(c.Request().Context(), catalogapp.BatchDisableCustomerProductAliasesCommand{
		Actor: support.ActorOf(c),
		IDs:   req.IDs,
	})
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, result)
}

func (h productHandler) customerProductAliasIndustryFieldsAPI(c echo.Context) error {
	id, err := parseOptionalInt64(c.Param("id"))
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	rows, err := h.catalog.ListCustomerProductAliasIndustryFields(c.Request().Context(), catalogapp.CustomerProductAliasIndustryFieldQuery{AliasID: id})
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"fields": rows})
}

func (h productHandler) saveCustomerProductAliasIndustryFieldsAPI(c echo.Context) error {
	id, err := parseOptionalInt64(c.Param("id"))
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	var req customerProductAliasIndustryFieldsAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	rows, err := h.catalog.SaveCustomerProductAliasIndustryFields(c.Request().Context(), catalogapp.SaveCustomerProductAliasIndustryFieldsCommand{
		Actor:   support.ActorOf(c),
		AliasID: id,
		Fields:  req.Fields,
	})
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"fields": rows})
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
		ClassificationTemplateID: 0,
		ProductConfigTemplateID:  0,
		GradientTemplateID:       0,
		UnitTemplateID:           0,
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

func rawJSONText(raw json.RawMessage, fallback string) (string, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return fallback, nil
	}
	if strings.HasPrefix(trimmed, `"`) {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", err
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return fallback, nil
		}
		return value, nil
	}
	var parsed any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", err
	}
	encoded, err := json.Marshal(parsed)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
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
	var industryFieldTemplateIDs []int64
	if req.IndustryFieldTemplateIDs != nil {
		if strings.TrimSpace(string(req.IndustryFieldTemplateIDs)) == "null" {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "industry_field_template_ids must be an array"})
		}
		if err := json.Unmarshal(req.IndustryFieldTemplateIDs, &industryFieldTemplateIDs); err != nil || industryFieldTemplateIDs == nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "industry_field_template_ids must be an array"})
		}
	}
	row, err := h.catalog.SaveProductProductionConfig(c.Request().Context(), catalogapp.SaveProductProductionConfigCommand{
		Actor:                    support.ActorOf(c),
		ProductID:                productID,
		ProductionBomID:          req.ProductionBomID,
		ProductionBomVersionID:   req.ProductionBomVersionID,
		ProcessRouteID:           req.ProcessRouteID,
		IndustryFieldTemplateID:  req.IndustryFieldTemplateID,
		IndustryFieldTemplateIDs: industryFieldTemplateIDs,
		ExpectedLossRate:         req.ExpectedLossRate,
		Note:                     req.Note,
		Fields:                   req.Fields,
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
	return c.JSON(http.StatusGone, map[string]any{"error": productClassificationLegacyReadonlyError})
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
		Actor:                   support.ActorOf(c),
		ID:                      id,
		CustomerID:              req.CustomerID,
		SourceTemplateID:        req.SourceTemplateID,
		Name:                    req.Name,
		Remark:                  req.Remark,
		ProductConfigTemplateID: req.ProductConfigTemplateID,
		GradientTemplateID:      req.GradientTemplateID,
		UnitTemplateID:          req.UnitTemplateID,
		Active:                  active,
		SortOrder:               req.SortOrder,
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
	return c.JSON(http.StatusGone, map[string]any{"error": productClassificationLegacyReadonlyError})
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
	return c.JSON(http.StatusGone, map[string]any{"error": productClassificationLegacyReadonlyError})
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
		Actor:                   support.ActorOf(c),
		ID:                      id,
		TemplateID:              req.TemplateID,
		ParentID:                req.ParentID,
		Name:                    req.Name,
		Level:                   req.Level,
		SortOrder:               req.SortOrder,
		ProductConfigTemplateID: req.ProductConfigTemplateID,
		GradientTemplateID:      req.GradientTemplateID,
		UnitTemplateID:          req.UnitTemplateID,
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
	return c.JSON(http.StatusGone, map[string]any{"error": productClassificationLegacyReadonlyError})
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

func (h productHandler) productClassificationTemplateUsagesAPI(c echo.Context) error {
	rows, err := h.catalog.ListProductClassificationTemplateUsages(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"rows": rows})
}

func (h productHandler) saveProductClassificationTemplateUsageAPI(c echo.Context) error {
	return c.JSON(http.StatusGone, map[string]any{"error": productClassificationLegacyReadonlyError})
	var req productClassificationTemplateUsageAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	row, err := h.catalog.SaveProductClassificationTemplateUsage(c.Request().Context(), catalogapp.SaveProductClassificationTemplateUsageCommand{
		Actor:                    support.ActorOf(c),
		ClassificationTemplateID: req.ClassificationTemplateID,
		SortOrder:                req.SortOrder,
	})
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"usage": row})
}

func (h productHandler) deleteProductClassificationTemplateUsageAPI(c echo.Context) error {
	return c.JSON(http.StatusGone, map[string]any{"error": productClassificationLegacyReadonlyError})
	templateID, err := parseOptionalInt64(c.Param("template_id"))
	if err != nil || templateID <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid classification_template_id"})
	}
	if err := h.catalog.DeleteProductClassificationTemplateUsage(c.Request().Context(), catalogapp.DeleteProductClassificationTemplateUsageCommand{Actor: support.ActorOf(c), ClassificationTemplateID: templateID}); err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

func (h productHandler) customerProductAliasClassificationTemplateUsagesAPI(c echo.Context) error {
	customerID, err := parseOptionalInt64(c.QueryParam("customer_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid customer_id"})
	}
	rows, err := h.catalog.ListCustomerProductAliasClassificationTemplateUsages(c.Request().Context(), customerID)
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"rows": rows})
}

func (h productHandler) saveCustomerProductAliasClassificationTemplateUsageAPI(c echo.Context) error {
	return c.JSON(http.StatusGone, map[string]any{"error": productClassificationLegacyReadonlyError})
	var req productClassificationTemplateUsageAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	row, err := h.catalog.SaveCustomerProductAliasClassificationTemplateUsage(c.Request().Context(), catalogapp.SaveCustomerProductAliasClassificationTemplateUsageCommand{
		Actor:                    support.ActorOf(c),
		CustomerID:               req.CustomerID,
		ClassificationTemplateID: req.ClassificationTemplateID,
		SortOrder:                req.SortOrder,
	})
	if err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"usage": row})
}

func (h productHandler) deleteCustomerProductAliasClassificationTemplateUsageAPI(c echo.Context) error {
	return c.JSON(http.StatusGone, map[string]any{"error": productClassificationLegacyReadonlyError})
	templateID, err := parseOptionalInt64(c.Param("template_id"))
	if err != nil || templateID <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid classification_template_id"})
	}
	customerID, err := parseOptionalInt64(c.QueryParam("customer_id"))
	if err != nil || customerID <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "customer_id required"})
	}
	if err := h.catalog.DeleteCustomerProductAliasClassificationTemplateUsage(c.Request().Context(), catalogapp.DeleteCustomerProductAliasClassificationTemplateUsageCommand{Actor: support.ActorOf(c), CustomerID: customerID, ClassificationTemplateID: templateID}); err != nil {
		if catalogapp.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

func (h productHandler) saveProductClassificationAssignmentAPI(c echo.Context) error {
	return c.JSON(http.StatusGone, map[string]any{"error": productClassificationLegacyReadonlyError})
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
	return c.JSON(http.StatusGone, map[string]any{"error": productClassificationLegacyReadonlyError})
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
		Actor:               support.ActorOf(c),
		ID:                  id,
		CustomerID:          req.CustomerID,
		Name:                req.Name,
		DisplayUnit:         req.DisplayUnit,
		UnitTemplateID:      req.UnitTemplateID,
		AllowCustomerResale: req.AllowCustomerResale,
		Tiers:               req.Tiers,
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

func (h productHandler) deleteProductConfigTemplateAPI(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	if err := h.catalog.DeleteProductConfigTemplate(c.Request().Context(), catalogapp.DeleteProductConfigTemplateCommand{
		Actor: support.ActorOf(c),
		ID:    id,
	}); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
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
		SalesUnit:          req.SalesUnit,
		DefaultSalesUnit:   req.DefaultSalesUnit,
		SalesUnits:         req.SalesUnits,
		SalesSpecs:         req.SalesSpecs,
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

func (h productHandler) deleteProductUnitDefinitionAPI(c echo.Context) error {
	code := strings.TrimSpace(c.Param("code"))
	if code == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "unit code required"})
	}
	if err := h.catalog.DeleteProductUnitDefinition(c.Request().Context(), catalogapp.DeleteProductUnitDefinitionCommand{
		Actor: support.ActorOf(c),
		Code:  code,
	}); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

func (h productHandler) deleteProductUnitTemplateAPI(c echo.Context) error {
	idText := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	if err := h.catalog.DeleteProductUnitTemplate(c.Request().Context(), catalogapp.DeleteProductUnitTemplateCommand{
		Actor: support.ActorOf(c),
		ID:    id,
	}); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
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
	return c.JSON(http.StatusGone, map[string]any{"error": productClassificationLegacyReadonlyError})
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
	return c.JSON(http.StatusGone, map[string]any{"error": productClassificationLegacyReadonlyError})
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
	return c.JSON(http.StatusGone, map[string]any{"error": productClassificationLegacyReadonlyError})
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
	return c.JSON(http.StatusGone, map[string]any{"error": productClassificationLegacyReadonlyError})
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
