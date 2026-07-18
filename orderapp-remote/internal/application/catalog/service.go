package catalog

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	catalogdomain "orderapp/internal/domain/catalog"
	"sort"
	"strconv"
	"strings"
)

type PriceTier struct {
	ID        int64
	SpecG     int64
	MinQty    float64
	MaxQty    *float64
	UnitPrice float64
}

type Product struct {
	ID                          int64
	SKUID                       int64
	ParentProductID             int64
	EffectiveParentProductID    int64
	SKUName                     string
	SKUCode                     string
	Barcode                     string
	SpecLabel                   string
	NetContentQty               float64
	NetContentUnit              string
	IsDefaultSKU                bool
	AutoDerivedSKU              bool
	DerivedUnitTemplateID       int64
	DerivedSpecKey              string
	DerivedSpecName             string
	DerivedSalesUnit            string
	DerivedSpecStatus           string
	Name                        string
	Remark                      string
	ProductKind                 string
	GreenBeanType               string
	GreenBeanBomProductID       int64
	RoastLevel                  string
	SpecialAttrsJSON            string
	DripBagGrams                float64
	DripBoxBagCount             int
	AllowFulfillmentOrder       bool
	AllowMallOrder              bool
	SalesUnits                  []string
	DefaultPrice                float64
	RetailPrice100G             float64
	RetailPrice200G             float64
	RetailPrice227G             float64
	RetailPrice250G             float64
	YieldRate                   float64
	ExpectedLossRate            float64
	ProcessRouteID              int64
	ProductionConfigNote        string
	ProductCategoryID           int64
	ProductCategoryPosition     int
	ClassificationTemplateID    int64
	CustomerID                  int64
	BaseProductID               int64
	Visibility                  string
	CustomType                  string
	MarginRateOverride          *float64
	GradientTemplateIDOverride  int64
	OperationTemplateIDOverride int64
	UnitRuleOverrideJSON        string
	InventoryUnit               string
	IntegerInventoryUnit        bool
	DefaultSalesUnit            string
	UnitConversionJSON          string
	SalesUnitRulesJSON          string
	UnitTemplateID              int64
	UnitTemplateName            string
	UnitRuleSource              string
	ProductConfigTemplateID     int64
	Active                      bool
	BomItemCount                int
	BomStatus                   string
	BomSourceType               string
	EffectiveProductID          int64
	EffectiveBomVersionID       int64
	SourceProductID             int64
	SourceProductCode           string
	SourceProductName           string
	SourceBomVersionID          int64
	SourceBomVersionNo          string
	DerivedFromLabel            string
	CanEditBOM                  bool
	ProductionBomID             int64
	ProductionBomCode           string
	ProductionBomName           string
	ProductionBomVersionID      int64
	ProductionBomVersionNo      string
	LatestBomVersionID          int64
	LatestBomVersionNo          string
	IsLatestBomVersion          bool
	ProductionBomGroupID        int64
	ProductionBomGroupName      string
	GroupID                     int64
	GroupName                   string
	GroupItemID                 int64
	GroupItemName               string
	ParentGroupItemID           int64
	ParentGroupItemName         string
	GroupSource                 string
	OrderUsageCount             int
	Tiers                       []PriceTier
	PriceSummary                PriceSummary
}

type PriceSummary struct {
	FinalPrice          float64 `json:"final_price,omitempty"`
	PriceUnit           string  `json:"price_unit,omitempty"`
	PriceTableVersion   string  `json:"price_table_version,omitempty"`
	TierLabel           string  `json:"tier_label,omitempty"`
	UpdatedAt           string  `json:"updated_at,omitempty"`
	SourcePriceRecordID int64   `json:"source_price_record_id,omitempty"`
	PublicationID       int64   `json:"publication_id,omitempty"`
}

type BusinessGroup struct {
	ID        int64                `json:"id"`
	Actor     string               `json:"-"`
	Name      string               `json:"name"`
	Code      string               `json:"code"`
	Remark    string               `json:"remark"`
	Active    bool                 `json:"active"`
	SortOrder int                  `json:"sort_order"`
	Usages    []BusinessGroupUsage `json:"usages,omitempty"`
	Items     []BusinessGroupItem  `json:"items,omitempty"`
}

type BusinessGroupUsage struct {
	ID         int64  `json:"id"`
	GroupID    int64  `json:"group_id"`
	UsageKey   string `json:"usage_key"`
	UsageLabel string `json:"usage_label"`
	Active     bool   `json:"active"`
}

type BusinessGroupItem struct {
	ID        int64               `json:"id"`
	Actor     string              `json:"-"`
	GroupID   int64               `json:"group_id"`
	ParentID  int64               `json:"parent_id"`
	Name      string              `json:"name"`
	Code      string              `json:"code"`
	Remark    string              `json:"remark"`
	Active    bool                `json:"active"`
	SortOrder int                 `json:"sort_order"`
	Children  []BusinessGroupItem `json:"children,omitempty"`
}

type BusinessGroupAssignment struct {
	ID                  int64  `json:"id"`
	Actor               string `json:"-"`
	GroupID             int64  `json:"group_id"`
	GroupName           string `json:"group_name,omitempty"`
	GroupItemID         int64  `json:"group_item_id"`
	GroupItemName       string `json:"group_item_name,omitempty"`
	ParentGroupItemID   int64  `json:"parent_group_item_id,omitempty"`
	ParentGroupItemName string `json:"parent_group_item_name,omitempty"`
	UsageKey            string `json:"usage_key"`
	ObjectKey           string `json:"object_key"`
	ObjectID            int64  `json:"object_id"`
	ObjectRef           string `json:"object_ref"`
	SortOrder           int    `json:"sort_order"`
}

const (
	BusinessGroupUsageProductCatalog     = "product_catalog"
	BusinessGroupUsageProductionBOM      = "production_bom"
	BusinessGroupUsageWarehouseInventory = "warehouse_inventory"
	BusinessGroupUsagePriceList          = "price_list"
)

type BusinessGroupAssignmentQuery struct {
	UsageKey    string
	ObjectKey   string
	ObjectID    int64
	ObjectRef   string
	GroupID     int64
	GroupItemID int64
}

type DeleteBusinessGroupAssignmentCommand struct {
	Actor string
	ID    int64
}

type DeleteBusinessGroupCommand struct {
	Actor string
	ID    int64
}

type DeleteBusinessGroupItemCommand struct {
	Actor string
	ID    int64
}

type MoveBusinessGroupItemCommand struct {
	Actor    string
	ID       int64
	ParentID int64 `json:"parent_id"`
	Position int   `json:"position"`
}

type ProductCustomerReference struct {
	ID                  int64  `json:"id"`
	Actor               string `json:"-"`
	ProductID           int64  `json:"product_id"`
	CustomerID          int64  `json:"customer_id"`
	CustomerItemCode    string `json:"customer_item_code"`
	CustomerDisplayName string `json:"customer_display_name"`
	Active              bool   `json:"active"`
	Remark              string `json:"remark"`
}

type ProductPricingRule struct {
	ID              int64          `json:"id"`
	Actor           string         `json:"-"`
	Name            string         `json:"name"`
	Code            string         `json:"code"`
	CostSourceMode  string         `json:"cost_source_mode"`
	MarginRate      float64        `json:"margin_rate"`
	TaxRate         float64        `json:"tax_rate"`
	RoundingMode    string         `json:"rounding_mode"`
	FormulaVersion  string         `json:"formula_version"`
	CalculationJSON map[string]any `json:"calculation_json"`
	Active          bool           `json:"active"`
	Remark          string         `json:"remark"`
}

type PriceTierTemplate struct {
	ID     int64                   `json:"id"`
	Actor  string                  `json:"-"`
	Name   string                  `json:"name"`
	Active bool                    `json:"active"`
	Remark string                  `json:"remark"`
	Tiers  []PriceTierTemplateTier `json:"tiers"`
}

type PriceTierTemplateTier struct {
	ID            int64    `json:"id"`
	TemplateID    int64    `json:"template_id"`
	Label         string   `json:"label"`
	MinQty        float64  `json:"min_qty"`
	MaxQty        *float64 `json:"max_qty,omitempty"`
	QuantityUnit  string   `json:"quantity_unit"`
	PricingRuleID int64    `json:"pricing_rule_id"`
	Position      int      `json:"position"`
	Active        bool     `json:"active"`
	Remark        string   `json:"remark"`
}

type PriceTableTemplateResolutionInput struct {
	DefaultPricingMode    string
	DefaultTierTemplateID int64
	DefaultPricingRuleID  int64
	DefaultFixedUnitPrice float64
	GroupAssignments      []PriceTableGroupTemplateAssignment
	ProductOverrides      []PriceTableProductTemplateOverride
	ProductID             int64
	GroupItemID           int64
}

type PriceTableGroupTemplateAssignment struct {
	GroupItemID       int64
	ParentGroupItemID int64
	PricingMode       string
	TierTemplateID    int64
	PricingRuleID     int64
	FixedUnitPrice    float64
}

type PriceTableProductTemplateOverride struct {
	ProductID      int64
	GroupItemID    int64
	PricingMode    string
	TierTemplateID int64
	PricingRuleID  int64
	FixedUnitPrice float64
}

type PriceTableTemplateResolution struct {
	PricingMode          string  `json:"pricing_mode"`
	PricingModeSource    string  `json:"pricing_mode_source"`
	TierTemplateID       int64   `json:"tier_template_id"`
	TierTemplateSource   string  `json:"tier_template_source"`
	PricingRuleID        int64   `json:"pricing_rule_id"`
	PricingRuleSource    string  `json:"pricing_rule_source"`
	FixedUnitPrice       float64 `json:"fixed_unit_price"`
	FixedUnitPriceSource string  `json:"fixed_unit_price_source"`
}

const (
	TemplateStatePublic   = "public_template"
	TemplateStateDerived  = "derived_from_public"
	TemplateStateCustomer = "customer_owned"
)

type ProductCategory struct {
	ID                      int64  `json:"id"`
	ParentID                int64  `json:"parent_id"`
	CustomerID              int64  `json:"customer_id"`
	SourceCategoryID        int64  `json:"source_category_id"`
	Name                    string `json:"name"`
	Level                   int    `json:"level"`
	Position                int    `json:"position"`
	Number                  int    `json:"number"`
	ProductConfigTemplateID int64  `json:"product_config_template_id"`
	GradientTemplateID      int64  `json:"gradient_template_id"`
	OperationTemplateID     int64  `json:"operation_template_id"`
	PriceListRuleJSON       string `json:"price_list_rule_json"`
	InventoryUnit           string `json:"inventory_unit"`
	QuoteUnit               string `json:"quote_unit"`
	OrderUnit               string `json:"order_unit"`
	UnitConversionJSON      string `json:"unit_conversion_json"`
	IntegerUnit             bool   `json:"integer_unit"`
	TemplateState           string `json:"template_state"`
}

type ProductSettingsProduct struct {
	ID                          int64        `json:"id"`
	SKUID                       int64        `json:"sku_id"`
	ParentProductID             int64        `json:"parent_product_id"`
	EffectiveParentProductID    int64        `json:"effective_parent_product_id"`
	SKUName                     string       `json:"sku_name"`
	SKUCode                     string       `json:"sku_code"`
	Barcode                     string       `json:"barcode"`
	SpecLabel                   string       `json:"spec_label"`
	NetContentQty               float64      `json:"net_content_qty"`
	NetContentUnit              string       `json:"net_content_unit"`
	IsDefaultSKU                bool         `json:"is_default_sku"`
	AutoDerivedSKU              bool         `json:"auto_derived_sku"`
	DerivedUnitTemplateID       int64        `json:"derived_unit_template_id"`
	DerivedSpecKey              string       `json:"derived_spec_key"`
	DerivedSpecName             string       `json:"derived_spec_name"`
	DerivedSalesUnit            string       `json:"derived_sales_unit"`
	DerivedSpecStatus           string       `json:"derived_spec_status"`
	Name                        string       `json:"name"`
	ProductCode                 string       `json:"product_code"`
	Remark                      string       `json:"remark"`
	ProductKind                 string       `json:"product_kind"`
	GreenBeanType               string       `json:"green_bean_type"`
	GreenBeanBomProductID       int64        `json:"green_bean_bom_product_id"`
	RoastLevel                  string       `json:"roast_level"`
	SpecialAttrsJSON            string       `json:"special_attrs_json"`
	DripBagGrams                float64      `json:"drip_bag_grams"`
	DripBoxBagCount             int          `json:"drip_box_bag_count"`
	AllowFulfillmentOrder       bool         `json:"allow_fulfillment_order"`
	AllowMallOrder              bool         `json:"allow_mall_order"`
	SalesUnits                  []string     `json:"sales_units"`
	DefaultPrice                float64      `json:"default_price"`
	RetailPrice100G             float64      `json:"retail_price_100g"`
	RetailPrice200G             float64      `json:"retail_price_200g"`
	RetailPrice227G             float64      `json:"retail_price_227g"`
	RetailPrice250G             float64      `json:"retail_price_250g"`
	YieldRate                   float64      `json:"yield_rate"`
	ExpectedLossRate            float64      `json:"expected_loss_rate"`
	ProcessRouteID              int64        `json:"process_route_id"`
	ProductionConfigNote        string       `json:"production_config_note"`
	ProductCategoryID           int64        `json:"product_category_id"`
	ProductCategoryPosition     int          `json:"product_category_position"`
	ClassificationTemplateID    int64        `json:"classification_template_id"`
	CustomerID                  int64        `json:"customer_id"`
	BaseProductID               int64        `json:"base_product_id"`
	Visibility                  string       `json:"visibility"`
	CustomType                  string       `json:"custom_type"`
	MarginRateOverride          *float64     `json:"margin_rate_override"`
	GradientTemplateIDOverride  int64        `json:"gradient_template_id_override"`
	OperationTemplateIDOverride int64        `json:"operation_template_id_override"`
	UnitRuleOverrideJSON        string       `json:"unit_rule_override_json"`
	InventoryUnit               string       `json:"inventory_unit"`
	IntegerInventoryUnit        bool         `json:"integer_inventory_unit"`
	DefaultSalesUnit            string       `json:"default_sales_unit"`
	UnitConversionJSON          string       `json:"unit_conversion_json"`
	SalesUnitRulesJSON          string       `json:"sales_unit_rules"`
	UnitTemplateID              int64        `json:"unit_template_id"`
	UnitTemplateName            string       `json:"unit_template_name"`
	UnitRuleSource              string       `json:"unit_rule_source"`
	ProductConfigTemplateID     int64        `json:"product_config_template_id"`
	Active                      bool         `json:"active"`
	BomItemCount                int          `json:"bom_item_count"`
	BomStatus                   string       `json:"bom_status"`
	BomSourceType               string       `json:"bom_source_type"`
	EffectiveProductID          int64        `json:"effective_product_id"`
	EffectiveBomVersionID       int64        `json:"effective_bom_version_id"`
	SourceProductID             int64        `json:"source_product_id"`
	SourceProductCode           string       `json:"source_product_code"`
	SourceProductName           string       `json:"source_product_name"`
	SourceBomVersionID          int64        `json:"source_bom_version_id"`
	SourceBomVersionNo          string       `json:"source_bom_version_no"`
	DerivedFromLabel            string       `json:"derived_from_label"`
	CanEditBOM                  bool         `json:"can_edit_bom"`
	ProductionBomID             int64        `json:"production_bom_id"`
	ProductionBomCode           string       `json:"production_bom_code"`
	ProductionBomName           string       `json:"production_bom_name"`
	ProductionBomVersionID      int64        `json:"production_bom_version_id"`
	ProductionBomVersionNo      string       `json:"production_bom_version_no"`
	LatestBomVersionID          int64        `json:"latest_bom_version_id"`
	LatestBomVersionNo          string       `json:"latest_bom_version_no"`
	IsLatestBomVersion          bool         `json:"is_latest_bom_version"`
	ProductionBomGroupID        int64        `json:"production_bom_group_id"`
	ProductionBomGroupName      string       `json:"production_bom_group_name"`
	GroupID                     int64        `json:"group_id"`
	GroupName                   string       `json:"group_name"`
	GroupItemID                 int64        `json:"group_item_id"`
	GroupItemName               string       `json:"group_item_name"`
	ParentGroupItemID           int64        `json:"parent_group_item_id"`
	ParentGroupItemName         string       `json:"parent_group_item_name"`
	GroupSource                 string       `json:"group_source"`
	OrderUsageCount             int          `json:"order_usage_count"`
	Number                      int          `json:"number"`
	PriceSummary                PriceSummary `json:"price_summary,omitempty"`
}

type ProductCategoryNode struct {
	ProductCategory
	Children []ProductCategoryNode    `json:"children"`
	Products []ProductSettingsProduct `json:"products"`
}

type ProductSettingsData struct {
	Categories                          []ProductCategoryNode                `json:"categories"`
	Products                            []ProductSettingsProduct             `json:"products"`
	ProductProductionConfigs            []ProductProductionConfig            `json:"product_production_configs"`
	ProductClassificationTemplates      []ProductClassificationTemplate      `json:"product_classification_templates"`
	ProductClassificationTemplateUsages []ProductClassificationTemplateUsage `json:"product_classification_template_usages"`
	GradientTemplates                   []GradientTemplate                   `json:"gradient_templates"`
	ProductConfigTemplates              []ProductConfigTemplate              `json:"product_config_templates"`
	ProductUnitDefinitions              []ProductUnitDefinition              `json:"product_unit_definitions"`
	ProductUnitTemplates                []ProductUnitTemplate                `json:"product_unit_templates"`
	ProductPriceGroups                  []ProductPriceGroup                  `json:"product_price_groups"`
	ProductPriceRecords                 []ProductPriceRecord                 `json:"product_price_records"`
	ProductTierPriceSchemes             []ProductTierPriceScheme             `json:"product_tier_price_schemes"`
	BusinessGroups                      []BusinessGroup                      `json:"business_groups"`
	BusinessGroupAssignments            []BusinessGroupAssignment            `json:"business_group_assignments"`
	ProductCustomerReferences           []ProductCustomerReference           `json:"product_customer_references"`
	ProductPricingRules                 []ProductPricingRule                 `json:"product_pricing_rules"`
	PriceTierTemplates                  []PriceTierTemplate                  `json:"price_tier_templates"`
	CustomerPublicUsages                []CustomerPublicUsage                `json:"customer_public_usages"`
	CustomerProductRuleTemplates        []CustomerProductRuleTemplate        `json:"customer_product_rule_templates"`
	CustomerProductRuleOverrides        []CustomerProductRuleOverride        `json:"customer_product_rule_overrides"`
	CustomerProductRuleBindings         []CustomerProductRuleBinding         `json:"customer_product_rule_bindings"`
}

type ProductProductionConfigField struct {
	ID               int64    `json:"id"`
	ProductID        int64    `json:"product_id"`
	FieldKey         string   `json:"field_key"`
	Label            string   `json:"label"`
	FieldType        string   `json:"field_type"`
	Unit             string   `json:"unit"`
	ValueText        string   `json:"value_text"`
	ValueNumber      *float64 `json:"value_number,omitempty"`
	ValueBool        *bool    `json:"value_bool,omitempty"`
	TemplateFieldKey string   `json:"template_field_key"`
	Required         bool     `json:"required"`
	OptionsJSON      string   `json:"options_json"`
	ShowInPriceList  bool     `json:"show_in_price_list"`
	SortOrder        int      `json:"sort_order"`
}

type ProductProductionConfig struct {
	ProductID               int64                          `json:"product_id"`
	ProductionBomID         int64                          `json:"production_bom_id"`
	ProductionBomVersionID  int64                          `json:"production_bom_version_id"`
	ProcessRouteID          int64                          `json:"process_route_id"`
	IndustryFieldTemplateID int64                          `json:"industry_field_template_id"`
	ExpectedLossRate        float64                        `json:"expected_loss_rate"`
	Note                    string                         `json:"note"`
	Fields                  []ProductProductionConfigField `json:"fields"`
}

type ProductConfigTemplate struct {
	ID                     int64  `json:"id"`
	CustomerID             int64  `json:"customer_id"`
	SourceTemplateID       int64  `json:"source_template_id"`
	TemplateState          string `json:"template_state"`
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
	Active                 bool   `json:"active"`
}

type ProductClassificationTemplate struct {
	ID                       int64                                          `json:"id"`
	CustomerID               int64                                          `json:"customer_id"`
	SourceTemplateID         int64                                          `json:"source_template_id"`
	TemplateState            string                                         `json:"template_state"`
	Name                     string                                         `json:"name"`
	Remark                   string                                         `json:"remark"`
	ProductConfigTemplateID  int64                                          `json:"product_config_template_id"`
	GradientTemplateID       int64                                          `json:"gradient_template_id"`
	UnitTemplateID           int64                                          `json:"unit_template_id"`
	Active                   bool                                           `json:"active"`
	SortOrder                int                                            `json:"sort_order"`
	Categories               []ProductClassificationCategory                `json:"categories"`
	ProductAssignments       []ProductClassificationAssignment              `json:"product_assignments"`
	CustomerAliasAssignments []CustomerProductAliasClassificationAssignment `json:"customer_alias_assignments"`
}

type ProductClassificationCategory struct {
	ID                      int64  `json:"id"`
	TemplateID              int64  `json:"template_id"`
	ParentID                int64  `json:"parent_id"`
	Name                    string `json:"name"`
	Level                   int    `json:"level"`
	SortOrder               int    `json:"sort_order"`
	ProductConfigTemplateID int64  `json:"product_config_template_id"`
	GradientTemplateID      int64  `json:"gradient_template_id"`
	UnitTemplateID          int64  `json:"unit_template_id"`
	Active                  bool   `json:"active"`
}

type ProductClassificationAssignment struct {
	ProductID  int64 `json:"product_id"`
	TemplateID int64 `json:"template_id"`
	CategoryID int64 `json:"category_id"`
	SortOrder  int   `json:"sort_order"`
}

type CustomerProductAliasClassificationAssignment struct {
	AliasID    int64 `json:"alias_id"`
	TemplateID int64 `json:"template_id"`
	CategoryID int64 `json:"category_id"`
	SortOrder  int   `json:"sort_order"`
}

type ProductClassificationTemplateUsage struct {
	ClassificationTemplateID int64 `json:"classification_template_id"`
	Active                   bool  `json:"active"`
	SortOrder                int   `json:"sort_order"`
}

type CustomerProductAliasClassificationTemplateUsage struct {
	CustomerID               int64 `json:"customer_id"`
	ClassificationTemplateID int64 `json:"classification_template_id"`
	Active                   bool  `json:"active"`
	SortOrder                int   `json:"sort_order"`
}

type ProductUnitDefinition struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	UnitType     string `json:"unit_type"`
	AllowDecimal bool   `json:"allow_decimal"`
	Active       bool   `json:"active"`
}

type ProductUnitTemplate struct {
	ID                 int64              `json:"id"`
	Name               string             `json:"name"`
	InventoryUnit      string             `json:"inventory_unit"`
	SalesUnit          string             `json:"sales_unit"`
	DefaultSalesUnit   string             `json:"default_sales_unit"`
	SalesUnits         []string           `json:"sales_units"`
	SalesSpecs         []ProductSalesSpec `json:"sales_specs"`
	QuoteUnit          string             `json:"quote_unit"`
	OrderUnit          string             `json:"order_unit"`
	UnitConversionJSON string             `json:"unit_conversion_json"`
	IntegerUnit        bool               `json:"integer_unit"`
	Active             bool               `json:"active"`
}

type ProductSalesSpec struct {
	SpecKey           string  `json:"spec_key"`
	SpecName          string  `json:"spec_name"`
	SalesUnit         string  `json:"sales_unit"`
	NetContentQty     float64 `json:"net_content_qty"`
	NetContentUnit    string  `json:"net_content_unit"`
	Default           bool    `json:"default"`
	Active            bool    `json:"active"`
	DerivedSKUID      int64   `json:"derived_sku_id,omitempty"`
	DerivedSKUCode    string  `json:"derived_sku_code,omitempty"`
	DerivedSpecStatus string  `json:"derived_spec_status,omitempty"`
}

type ProductPriceGroup struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
	Active    bool   `json:"active"`
}

type ProductPriceRecord struct {
	ID                      int64   `json:"id"`
	ProductID               int64   `json:"product_id"`
	CustomerProductAliasID  int64   `json:"customer_product_alias_id"`
	FinalUnitPrice          float64 `json:"final_unit_price"`
	PriceUnit               string  `json:"price_unit"`
	Currency                string  `json:"currency"`
	PriceGroupID            int64   `json:"price_group_id"`
	PriceGroupName          string  `json:"price_group_name"`
	InventoryUnit           string  `json:"inventory_unit"`
	InventoryConversionJSON string  `json:"inventory_conversion_json"`
	Status                  string  `json:"status"`
	Remark                  string  `json:"remark"`
	Active                  bool    `json:"active"`
}

type ProductPriceRecordQuery struct {
	ProductID              int64
	CustomerProductAliasID int64
	PriceGroupID           int64
	ActiveMode             string
	Status                 string
}

type ProductTierPriceScheme struct {
	ID                     int64                        `json:"id"`
	Name                   string                       `json:"name"`
	ProductID              int64                        `json:"product_id"`
	CustomerProductAliasID int64                        `json:"customer_product_alias_id"`
	PriceGroupID           int64                        `json:"price_group_id"`
	Active                 bool                         `json:"active"`
	Remark                 string                       `json:"remark"`
	Tiers                  []ProductTierPriceSchemeTier `json:"tiers"`
}

type ProductTierPriceSchemeTier struct {
	ID                  int64    `json:"id"`
	SchemeID            int64    `json:"scheme_id"`
	Label               string   `json:"label"`
	MinQty              float64  `json:"min_qty"`
	MaxQty              *float64 `json:"max_qty,omitempty"`
	SourcePriceRecordID int64    `json:"source_price_record_id"`
	FinalUnitPrice      float64  `json:"final_unit_price"`
	PriceUnit           string   `json:"price_unit"`
	Currency            string   `json:"currency"`
	Position            int      `json:"position"`
}

type ProductTierPriceSchemeQuery struct {
	ProductID              int64
	CustomerProductAliasID int64
	PriceGroupID           int64
	ActiveMode             string
}

type GradientTemplate struct {
	ID                  int64                  `json:"id"`
	Name                string                 `json:"name"`
	CustomerID          int64                  `json:"customer_id"`
	SourceTemplateID    int64                  `json:"source_template_id"`
	TemplateState       string                 `json:"template_state"`
	DisplayUnit         string                 `json:"display_unit"`
	UnitTemplateID      int64                  `json:"unit_template_id"`
	AllowCustomerResale bool                   `json:"allow_customer_resale"`
	Active              bool                   `json:"active"`
	Tiers               []GradientTemplateTier `json:"tiers"`
}

type GradientTemplateTier struct {
	ID            int64    `json:"id"`
	TemplateID    int64    `json:"template_id,omitempty"`
	Label         string   `json:"label"`
	MinDisplayQty *float64 `json:"min_display_qty,omitempty"`
	MaxDisplayQty *float64 `json:"max_display_qty,omitempty"`
	MinWeightG    float64  `json:"min_weight_g"`
	MaxWeightG    *float64 `json:"max_weight_g,omitempty"`
	MarginRate    float64  `json:"margin_rate"`
	Position      int      `json:"position"`
}

type ReplacePriceTiersCommand struct {
	Actor                 string
	ProductID             int64
	ProductKind           string
	GreenBeanType         string
	GreenBeanBomProductID int64
	DefaultPrice          float64
	RoastLevel            string
	RetailPrice100G       float64
	RetailPrice200G       float64
	RetailPrice227G       float64
	RetailPrice250G       float64
	YieldRate             float64
	Tiers                 []PriceTier
}

type UpdateProductBasicsCommand struct {
	Actor                       string
	ProductID                   int64
	Name                        string
	ProductKind                 string
	Remark                      string
	GreenBeanType               string
	GreenBeanBomProductID       int64
	DefaultPrice                float64
	RoastLevel                  string
	SpecialAttrsJSON            string
	DripBagGrams                float64
	DripBoxBagCount             int
	AllowFulfillmentOrder       bool
	AllowMallOrder              bool
	SalesUnits                  []string
	RetailPrice100G             float64
	RetailPrice200G             float64
	RetailPrice227G             float64
	RetailPrice250G             float64
	YieldRate                   float64
	MarginRateOverride          *float64
	GradientTemplateIDOverride  int64
	OperationTemplateIDOverride int64
	UnitTemplateID              int64
	UnitRuleOverrideJSON        string
	ProductConfigTemplateID     int64
	ClassificationTemplateID    int64
}

type CreateProductCommand struct {
	Actor                    string
	Name                     string
	Remark                   string
	ProductKind              string
	GreenBeanType            string
	GreenBeanBomProductID    int64
	RoastLevel               string
	SpecialAttrsJSON         string
	DripBagGrams             float64
	DripBoxBagCount          int
	AllowFulfillmentOrder    bool
	AllowFulfillmentOrderSet bool
	AllowMallOrder           bool
	SalesUnits               []string
	DefaultPrice             float64
	RetailPrice100G          float64
	RetailPrice200G          float64
	RetailPrice227G          float64
	RetailPrice250G          float64
	YieldRate                float64
	ProductConfigTemplateID  int64
	ClassificationTemplateID int64
	UnitTemplateID           int64
	UnitRuleOverrideJSON     string
	Tiers                    []PriceTier
}

type CopyProductCommand struct {
	Actor           string
	SourceProductID int64
}

type DeactivateProductsCommand struct {
	Actor      string
	ProductIDs []int64
}

type CreateSKUCommand struct {
	Actor                    string
	CustomerID               int64
	ParentProductID          int64
	Name                     string
	SKUName                  string
	SKUCode                  string
	Barcode                  string
	SpecLabel                string
	NetContentQty            float64
	NetContentUnit           string
	IsDefaultSKU             bool
	Remark                   string
	ProductTypeCategoryID    int64
	ProductSubtypeCategoryID int64
	SpecialAttrsJSON         string
	ProductConfigTemplateID  int64
	ClassificationTemplateID int64
	UnitTemplateID           int64
	UnitRuleOverrideJSON     string
	Active                   bool
}

type CreateCustomProductCommand struct {
	Actor                 string
	CustomerID            int64
	BaseProductID         int64
	Name                  string
	Remark                string
	ProductKind           string
	GreenBeanType         string
	GreenBeanBomProductID int64
	RoastLevel            string
	SpecialAttrsJSON      string
	YieldRate             float64
	DripBagGrams          float64
	DripBoxBagCount       int
	CustomType            string
	CopyBOM               bool
	CopyPriceTiers        bool
}

type CustomerPublicUsage struct {
	CustomerID                 int64 `json:"customer_id"`
	UsePublicSKU               bool  `json:"use_public_sku"`
	UsePublicCategories        bool  `json:"use_public_categories"`
	UsePublicGradientTemplates bool  `json:"use_public_gradient_templates"`
}

type CustomerPublicUsageCommand struct {
	Actor                      string
	CustomerID                 int64
	UsePublicSKU               bool
	UsePublicCategories        bool
	UsePublicGradientTemplates bool
}

type CustomerProductAlias struct {
	ID                       int64                          `json:"id"`
	CustomerID               int64                          `json:"customer_id"`
	CustomerName             string                         `json:"customer_name"`
	ProductID                int64                          `json:"product_id"`
	ProductCode              string                         `json:"product_code"`
	ProductName              string                         `json:"product_name"`
	ProductActive            bool                           `json:"product_active"`
	DisplayName              string                         `json:"display_name"`
	CustomerItemCode         string                         `json:"customer_item_code"`
	BrandName                string                         `json:"brand_name"`
	DisplayCategoryID        int64                          `json:"display_category_id"`
	DisplayCategoryName      string                         `json:"display_category_name"`
	ClassificationTemplateID int64                          `json:"classification_template_id"`
	ProductConfigTemplateID  int64                          `json:"product_config_template_id"`
	GradientTemplateID       int64                          `json:"gradient_template_id"`
	UnitTemplateID           int64                          `json:"unit_template_id"`
	SortOrder                int                            `json:"sort_order"`
	IncludeInPriceList       bool                           `json:"include_in_price_list"`
	Active                   bool                           `json:"active"`
	Remark                   string                         `json:"remark"`
	CreatedBy                string                         `json:"created_by"`
	UpdatedBy                string                         `json:"updated_by"`
	IndustryFields           []ProductProductionConfigField `json:"industry_fields,omitempty"`
	PriceSummary             PriceSummary                   `json:"price_summary,omitempty"`
}

type CustomerProductAliasQuery struct {
	CustomerID  int64
	ActiveOnly  bool
	ActiveMode  string
	SearchQuery string
}

type CustomerProductAliasCommand struct {
	Actor                    string
	ID                       int64
	CustomerID               int64
	ProductID                int64
	DisplayName              string
	CustomerItemCode         string
	BrandName                string
	DisplayCategoryID        int64
	ClassificationTemplateID int64
	ProductConfigTemplateID  int64
	GradientTemplateID       int64
	UnitTemplateID           int64
	SortOrder                int
	IncludeInPriceList       bool
	Active                   bool
	Remark                   string
}

type DisableCustomerProductAliasCommand struct {
	Actor string
	ID    int64
}

type BatchDisableCustomerProductAliasesCommand struct {
	Actor string
	IDs   []int64
}

type BatchDisableCustomerProductAliasesResult struct {
	DisabledCount int     `json:"disabled_count"`
	SkippedCount  int     `json:"skipped_count"`
	Disabled      []int64 `json:"disabled"`
	Skipped       []int64 `json:"skipped"`
}

type CustomerProductAliasIndustryFieldQuery struct {
	AliasID int64
}

type SaveCustomerProductAliasIndustryFieldsCommand struct {
	Actor   string
	AliasID int64
	Fields  []ProductProductionConfigField
}

type BatchCustomerProductAliasesCommand struct {
	Actor                    string
	CustomerID               int64
	ProductIDs               []int64
	IncludeInPriceList       bool
	BrandName                string
	DisplayCategoryID        int64
	ClassificationTemplateID int64
}

type CustomerProductAliasBatchSkipped struct {
	ProductID int64  `json:"product_id"`
	Reason    string `json:"reason"`
}

type BatchCustomerProductAliasesResult struct {
	CreatedCount int                                `json:"created_count"`
	SkippedCount int                                `json:"skipped_count"`
	Created      []CustomerProductAlias             `json:"created"`
	Skipped      []CustomerProductAliasBatchSkipped `json:"skipped"`
}

type CustomerProductAliasMigrationCandidateQuery struct {
	CustomerID int64
}

type CustomerProductAliasMigrationCandidate struct {
	CustomerID          int64  `json:"customer_id"`
	ProductID           int64  `json:"product_id"`
	ProductCode         string `json:"product_code"`
	ProductName         string `json:"product_name"`
	BaseProductID       int64  `json:"base_product_id"`
	BaseProductCode     string `json:"base_product_code"`
	BaseProductName     string `json:"base_product_name"`
	BomSourceType       string `json:"bom_source_type"`
	SuggestedAction     string `json:"suggested_action"`
	SuggestedReason     string `json:"suggested_reason"`
	CanAutoRecommend    bool   `json:"can_auto_recommend"`
	HasOwnBom           bool   `json:"has_own_bom"`
	HasProductionRecord bool   `json:"has_production_record"`
	HasInventoryRecord  bool   `json:"has_inventory_record"`
}

type CustomerProductRuleTemplateItem struct {
	ID                       int64  `json:"id,omitempty"`
	TemplateID               int64  `json:"template_id,omitempty"`
	ProductSubtypeCategoryID int64  `json:"product_subtype_category_id"`
	GradientTemplateID       int64  `json:"gradient_template_id"`
	OperationTemplateID      int64  `json:"operation_template_id"`
	PriceListRuleJSON        string `json:"price_list_rule_json"`
	UnitRuleJSON             string `json:"unit_rule_json"`
}

type CustomerProductRuleTemplate struct {
	ID         int64                             `json:"id"`
	CustomerID int64                             `json:"customer_id"`
	Name       string                            `json:"name"`
	Active     bool                              `json:"active"`
	Items      []CustomerProductRuleTemplateItem `json:"items"`
}

type CustomerProductRuleOverride struct {
	ID                       int64  `json:"id"`
	CustomerID               int64  `json:"customer_id"`
	ProductSubtypeCategoryID int64  `json:"product_subtype_category_id"`
	GradientTemplateID       int64  `json:"gradient_template_id"`
	OperationTemplateID      int64  `json:"operation_template_id"`
	PriceListRuleJSON        string `json:"price_list_rule_json"`
	UnitRuleJSON             string `json:"unit_rule_json"`
	Active                   bool   `json:"active"`
}

type CustomerProductRuleBinding struct {
	CustomerID int64 `json:"customer_id"`
	TemplateID int64 `json:"template_id"`
}

type SaveCustomerProductRuleTemplateCommand struct {
	Actor      string
	ID         int64
	CustomerID int64
	Name       string
	Active     *bool
	Items      []CustomerProductRuleTemplateItem
}

type SaveCustomerProductRuleOverrideCommand struct {
	Actor                    string
	ID                       int64
	CustomerID               int64
	ProductSubtypeCategoryID int64
	GradientTemplateID       int64
	OperationTemplateID      int64
	PriceListRuleJSON        string
	UnitRuleJSON             string
	Active                   *bool
}

type CustomerProductRuleTemplateBindingCommand struct {
	Actor      string
	CustomerID int64
	TemplateID int64
}

type SaveProductCategoryCommand struct {
	Actor                   string
	ID                      int64
	ParentID                int64
	CustomerID              int64
	Name                    string
	Position                int
	ProductConfigTemplateID int64
	GradientTemplateID      int64
	OperationTemplateID     int64
	PriceListRuleJSON       string
	InventoryUnit           string
	QuoteUnit               string
	OrderUnit               string
	UnitConversionJSON      string
	IntegerUnit             bool
}

type SaveProductConfigTemplateCommand struct {
	Actor                  string
	ID                     int64
	CustomerID             int64
	Name                   string
	GradientTemplateID     int64
	OperationTemplateID    int64
	UnitTemplateID         int64
	PriceListRuleJSON      string
	SpecialAttrsSchemaJSON string
	InventoryUnit          string
	QuoteUnit              string
	OrderUnit              string
	UnitConversionJSON     string
	IntegerUnit            bool
	Active                 *bool
}

type SaveProductClassificationTemplateCommand struct {
	Actor                   string
	ID                      int64
	CustomerID              int64
	SourceTemplateID        int64
	Name                    string
	Remark                  string
	ProductConfigTemplateID int64
	GradientTemplateID      int64
	UnitTemplateID          int64
	Active                  bool
	SortOrder               int
}

type DeleteProductClassificationTemplateCommand struct {
	Actor string
	ID    int64
}

type DeleteProductConfigTemplateCommand struct {
	Actor string
	ID    int64
}

type SaveProductClassificationCategoryCommand struct {
	Actor                   string
	ID                      int64
	TemplateID              int64
	ParentID                int64
	Name                    string
	Level                   int
	SortOrder               int
	ProductConfigTemplateID int64
	GradientTemplateID      int64
	UnitTemplateID          int64
}

type DeleteProductClassificationCategoryCommand struct {
	Actor      string
	ID         int64
	TemplateID int64
}

type SaveProductClassificationAssignmentCommand struct {
	Actor      string
	ProductID  int64
	TemplateID int64
	CategoryID int64
	SortOrder  int
}

type SaveCustomerProductAliasClassificationAssignmentCommand struct {
	Actor      string
	AliasID    int64
	TemplateID int64
	CategoryID int64
	SortOrder  int
}

type SaveProductClassificationTemplateUsageCommand struct {
	Actor                    string
	ClassificationTemplateID int64
	SortOrder                int
}

type DeleteProductClassificationTemplateUsageCommand struct {
	Actor                    string
	ClassificationTemplateID int64
}

type SaveCustomerProductAliasClassificationTemplateUsageCommand struct {
	Actor                    string
	CustomerID               int64
	ClassificationTemplateID int64
	SortOrder                int
}

type DeleteCustomerProductAliasClassificationTemplateUsageCommand struct {
	Actor                    string
	CustomerID               int64
	ClassificationTemplateID int64
}

type SaveProductUnitDefinitionCommand struct {
	Actor        string
	Code         string
	Name         string
	UnitType     string
	AllowDecimal bool
	Active       *bool
}

type DeleteProductUnitDefinitionCommand struct {
	Actor string
	Code  string
}

type SaveProductUnitTemplateCommand struct {
	Actor              string
	ID                 int64
	Name               string
	InventoryUnit      string
	SalesUnit          string
	DefaultSalesUnit   string
	SalesUnits         []string
	SalesSpecs         []ProductSalesSpec
	QuoteUnit          string
	OrderUnit          string
	UnitConversionJSON string
	IntegerUnit        bool
	Active             *bool
}

type SaveProductPriceGroupCommand struct {
	Actor     string
	ID        int64
	Name      string
	SortOrder int
	Active    *bool
}

type SaveProductPriceRecordCommand struct {
	Actor                   string
	ID                      int64
	ProductID               int64
	CustomerProductAliasID  int64
	FinalUnitPrice          float64
	PriceUnit               string
	Currency                string
	PriceGroupID            int64
	PriceGroupName          string
	InventoryUnit           string
	InventoryConversionJSON string
	Status                  string
	Remark                  string
	Active                  *bool
}

type SaveProductTierPriceSchemeCommand struct {
	Actor                  string
	ID                     int64
	Name                   string
	ProductID              int64
	CustomerProductAliasID int64
	PriceGroupID           int64
	Active                 *bool
	Remark                 string
	Tiers                  []ProductTierPriceSchemeTier
}

type DeleteProductUnitTemplateCommand struct {
	Actor string
	ID    int64
}

type SaveProductProductionConfigCommand struct {
	Actor                   string
	ProductID               int64
	ProductionBomID         int64
	ProductionBomVersionID  int64
	ProcessRouteID          int64
	IndustryFieldTemplateID int64
	ExpectedLossRate        float64
	Note                    string
	Fields                  []ProductProductionConfigField
}

type DeriveProductConfigTemplateCommand struct {
	Actor            string
	CustomerID       int64
	SourceTemplateID int64
	Name             string
}

type MoveProductCategoryCommand struct {
	Actor    string
	ID       int64
	ParentID int64
	Position int
}

type DeleteProductCategoryCommand struct {
	Actor string
	ID    int64
}

type AssignProductCategoryCommand struct {
	Actor                string
	ProductID            int64
	CategoryID           int64
	CustomerID           int64
	Position             int
	DerivePublicCategory bool
	DerivePublicProduct  bool
}

type AssignProductCategoryResult struct {
	ProductID          int64 `json:"product_id"`
	CategoryID         int64 `json:"category_id"`
	DerivedProductID   int64 `json:"derived_product_id"`
	DerivedCategoryID  int64 `json:"derived_category_id"`
	UsedPublicProduct  bool  `json:"used_public_product"`
	UsedPublicCategory bool  `json:"used_public_category"`
}

type DeriveProductCategoryCommand struct {
	Actor            string
	CustomerID       int64
	SourceCategoryID int64
}

type DeriveCustomerProductCommand struct {
	Actor          string
	CustomerID     int64
	BaseProductID  int64
	CategoryID     int64
	Position       int
	Name           string
	CopyBOM        bool
	CopyPriceTiers bool
}

type DeriveGradientTemplateCommand struct {
	Actor            string
	CustomerID       int64
	SourceTemplateID int64
	Name             string
}

type SaveGradientTemplateCommand struct {
	Actor               string
	ID                  int64
	CustomerID          int64
	Name                string
	DisplayUnit         string
	UnitTemplateID      int64
	AllowCustomerResale bool
	Tiers               []GradientTemplateTier
}

type DeactivateGradientTemplateCommand struct {
	Actor string
	ID    int64
}

type BindCategoryGradientTemplateCommand struct {
	Actor              string
	CategoryID         int64
	GradientTemplateID int64
}

type Service struct {
	repo Repository
}

type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

func IsValidationError(err error) bool {
	var validationErr ValidationError
	return errors.As(err, &validationErr)
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListProducts(ctx context.Context) ([]Product, error) {
	return s.repo.ListProducts(ctx)
}

func (s *Service) GetProduct(ctx context.Context, id int64) (*Product, error) {
	return s.repo.GetProduct(ctx, id)
}

func (s *Service) ListProductProductionConfigs(ctx context.Context) ([]ProductProductionConfig, error) {
	return s.repo.ListProductProductionConfigs(ctx)
}

func (s *Service) GetProductProductionConfig(ctx context.Context, productID int64) (ProductProductionConfig, error) {
	if productID <= 0 {
		return ProductProductionConfig{}, ValidationError{Message: "product_id required"}
	}
	return s.repo.GetProductProductionConfig(ctx, productID)
}

func (s *Service) SaveProductProductionConfig(ctx context.Context, cmd SaveProductProductionConfigCommand) (ProductProductionConfig, error) {
	if cmd.ProductID <= 0 {
		return ProductProductionConfig{}, ValidationError{Message: "product_id required"}
	}
	if cmd.ExpectedLossRate < 0 || cmd.ExpectedLossRate >= 1 {
		return ProductProductionConfig{}, ValidationError{Message: "expected_loss_rate must be [0,1)"}
	}
	if cmd.IndustryFieldTemplateID < 0 {
		return ProductProductionConfig{}, ValidationError{Message: "invalid industry_field_template_id"}
	}
	if cmd.IndustryFieldTemplateID == 0 {
		cmd.Fields = []ProductProductionConfigField{}
	}
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	cmd.Note = strings.TrimSpace(cmd.Note)
	for i := range cmd.Fields {
		field := cmd.Fields[i]
		field.FieldKey = strings.TrimSpace(field.FieldKey)
		field.TemplateFieldKey = strings.TrimSpace(field.TemplateFieldKey)
		field.Label = strings.TrimSpace(field.Label)
		field.FieldType = strings.ToLower(strings.TrimSpace(field.FieldType))
		field.Unit = strings.TrimSpace(field.Unit)
		field.OptionsJSON = strings.TrimSpace(field.OptionsJSON)
		if field.FieldType == "" {
			field.FieldType = "text"
		}
		if field.FieldKey == "" {
			return ProductProductionConfig{}, ValidationError{Message: "field_key required"}
		}
		if field.Label == "" {
			field.Label = field.FieldKey
		}
		if field.SortOrder <= 0 {
			field.SortOrder = i + 1
		}
		cmd.Fields[i] = field
	}
	return s.repo.SaveProductProductionConfig(ctx, cmd)
}

func (s *Service) ReplacePriceTiers(ctx context.Context, cmd ReplacePriceTiersCommand) error {
	cmd.ProductKind = catalogdomain.NormalizeProductKind(cmd.ProductKind)
	if err := normalizeProductSalesShape(&cmd.ProductKind, &cmd.GreenBeanType, &cmd.GreenBeanBomProductID, &cmd.RoastLevel, &cmd.DefaultPrice, &cmd.RetailPrice100G, &cmd.RetailPrice200G, &cmd.RetailPrice227G, &cmd.RetailPrice250G, &cmd.YieldRate, &cmd.Tiers); err != nil {
		return err
	}
	if cmd.ProductKind == catalogdomain.ProductKindGreenBean {
		if err := s.validateGreenBeanBomProduct(ctx, cmd.GreenBeanBomProductID); err != nil {
			return err
		}
	}
	return s.repo.ReplacePriceTiers(ctx, cmd)
}

func (s *Service) UpdateProductBasics(ctx context.Context, cmd UpdateProductBasicsCommand) error {
	var err error
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Remark = strings.TrimSpace(cmd.Remark)
	cmd.ProductKind, cmd.DripBagGrams, cmd.DripBoxBagCount, cmd.SalesUnits, err = normalizeProductKindSettings(cmd.ProductKind, cmd.DripBagGrams, cmd.DripBoxBagCount)
	if err != nil {
		return err
	}
	if err := normalizeProductSalesShape(&cmd.ProductKind, &cmd.GreenBeanType, &cmd.GreenBeanBomProductID, &cmd.RoastLevel, &cmd.DefaultPrice, &cmd.RetailPrice100G, &cmd.RetailPrice200G, &cmd.RetailPrice227G, &cmd.RetailPrice250G, &cmd.YieldRate, nil); err != nil {
		return err
	}
	if cmd.ProductKind == catalogdomain.ProductKindGreenBean {
		if err := s.validateGreenBeanBomProduct(ctx, cmd.GreenBeanBomProductID); err != nil {
			return err
		}
	}
	if cmd.GradientTemplateIDOverride < 0 || cmd.OperationTemplateIDOverride < 0 {
		return ValidationError{Message: "invalid template override"}
	}
	if cmd.ProductConfigTemplateID < 0 {
		return ValidationError{Message: "invalid product_config_template_id"}
	}
	if cmd.UnitTemplateID < 0 {
		return ValidationError{Message: "invalid unit_template_id"}
	}
	cmd.MarginRateOverride = nil
	cmd.GradientTemplateIDOverride = 0
	cmd.OperationTemplateIDOverride = 0
	cmd.ProductConfigTemplateID = 0
	cmd.ClassificationTemplateID = 0
	unitRuleOverrideJSON, err := normalizeJSONObjectText(cmd.UnitRuleOverrideJSON)
	if err != nil {
		return ValidationError{Message: "invalid unit_rule_override_json"}
	}
	cmd.UnitRuleOverrideJSON = unitRuleOverrideJSON
	specialAttrsJSON, err := normalizeJSONObjectText(cmd.SpecialAttrsJSON)
	if err != nil {
		return ValidationError{Message: "invalid special_attrs_json"}
	}
	cmd.SpecialAttrsJSON = specialAttrsJSON
	return s.repo.UpdateProductBasics(ctx, cmd)
}

func (s *Service) DeactivateProducts(ctx context.Context, cmd DeactivateProductsCommand) error {
	ids := make([]int64, 0, len(cmd.ProductIDs))
	seen := map[int64]bool{}
	for _, id := range cmd.ProductIDs {
		if id <= 0 || seen[id] {
			continue
		}
		ids = append(ids, id)
		seen[id] = true
	}
	if len(ids) == 0 {
		return fmt.Errorf("product_ids required")
	}
	cmd.ProductIDs = ids
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	return s.repo.DeactivateProducts(ctx, cmd)
}

func (s *Service) CreateProduct(ctx context.Context, cmd CreateProductCommand) (Product, error) {
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Remark = strings.TrimSpace(cmd.Remark)
	if cmd.Name == "" {
		return Product{}, ValidationError{Message: "name required"}
	}
	var err error
	cmd.ProductKind, cmd.DripBagGrams, cmd.DripBoxBagCount, cmd.SalesUnits, err = normalizeProductKindSettings(cmd.ProductKind, cmd.DripBagGrams, cmd.DripBoxBagCount)
	if err != nil {
		return Product{}, err
	}
	if !cmd.AllowFulfillmentOrderSet {
		cmd.AllowFulfillmentOrder = true
	}
	if cmd.DefaultPrice < 0 || cmd.RetailPrice100G < 0 || cmd.RetailPrice200G < 0 || cmd.RetailPrice227G < 0 || cmd.RetailPrice250G < 0 {
		return Product{}, ValidationError{Message: "price must not be negative"}
	}
	if cmd.RetailPrice227G <= 0 && cmd.DefaultPrice > 0 {
		cmd.RetailPrice227G = cmd.DefaultPrice
	}
	if err := normalizeProductSalesShape(&cmd.ProductKind, &cmd.GreenBeanType, &cmd.GreenBeanBomProductID, &cmd.RoastLevel, &cmd.DefaultPrice, &cmd.RetailPrice100G, &cmd.RetailPrice200G, &cmd.RetailPrice227G, &cmd.RetailPrice250G, &cmd.YieldRate, &cmd.Tiers); err != nil {
		return Product{}, err
	}
	if cmd.ProductKind == catalogdomain.ProductKindGreenBean {
		if err := s.validateGreenBeanBomProduct(ctx, cmd.GreenBeanBomProductID); err != nil {
			return Product{}, err
		}
	}
	specialAttrsJSON, err := normalizeJSONObjectText(cmd.SpecialAttrsJSON)
	if err != nil {
		return Product{}, ValidationError{Message: "invalid special_attrs_json"}
	}
	if cmd.ProductConfigTemplateID < 0 {
		return Product{}, ValidationError{Message: "invalid product_config_template_id"}
	}
	if cmd.UnitTemplateID < 0 {
		return Product{}, ValidationError{Message: "invalid unit_template_id"}
	}
	unitRuleOverrideJSON, err := normalizeJSONObjectText(cmd.UnitRuleOverrideJSON)
	if err != nil {
		return Product{}, ValidationError{Message: "invalid unit_rule_override_json"}
	}
	cmd.ProductConfigTemplateID = 0
	cmd.ClassificationTemplateID = 0
	cmd.Tiers = nil
	cmd.SpecialAttrsJSON = specialAttrsJSON
	cmd.UnitRuleOverrideJSON = unitRuleOverrideJSON
	return s.repo.CreateProduct(ctx, cmd)
}

func (s *Service) CopyProduct(ctx context.Context, cmd CopyProductCommand) (Product, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.SourceProductID <= 0 {
		return Product{}, ValidationError{Message: "source_product_id required"}
	}
	return s.repo.CopyProduct(ctx, cmd)
}

func (s *Service) CreateSKU(ctx context.Context, cmd CreateSKUCommand) (Product, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.SKUName = strings.TrimSpace(cmd.SKUName)
	cmd.SKUCode = strings.TrimSpace(cmd.SKUCode)
	cmd.Barcode = strings.TrimSpace(cmd.Barcode)
	cmd.SpecLabel = strings.TrimSpace(cmd.SpecLabel)
	cmd.NetContentUnit = strings.TrimSpace(cmd.NetContentUnit)
	cmd.Remark = strings.TrimSpace(cmd.Remark)
	if cmd.Name == "" {
		return Product{}, ValidationError{Message: "name required"}
	}
	if cmd.ParentProductID < 0 {
		return Product{}, ValidationError{Message: "invalid parent_product_id"}
	}
	if cmd.CustomerID < 0 {
		return Product{}, ValidationError{Message: "invalid customer_id"}
	}
	if cmd.NetContentQty < 0 || math.IsNaN(cmd.NetContentQty) || math.IsInf(cmd.NetContentQty, 0) {
		return Product{}, ValidationError{Message: "invalid net_content_qty"}
	}
	if cmd.ProductTypeCategoryID < 0 || cmd.ProductSubtypeCategoryID < 0 {
		return Product{}, ValidationError{Message: "invalid category"}
	}
	if cmd.ProductConfigTemplateID < 0 {
		return Product{}, ValidationError{Message: "invalid product_config_template_id"}
	}
	if cmd.UnitTemplateID < 0 {
		return Product{}, ValidationError{Message: "invalid unit_template_id"}
	}
	unitRuleOverrideJSON, err := normalizeJSONObjectText(cmd.UnitRuleOverrideJSON)
	if err != nil {
		return Product{}, ValidationError{Message: "invalid unit_rule_override_json"}
	}
	cmd.ProductConfigTemplateID = 0
	cmd.ClassificationTemplateID = 0
	specialAttrsJSON, err := normalizeJSONObjectText(cmd.SpecialAttrsJSON)
	if err != nil {
		return Product{}, ValidationError{Message: "invalid special_attrs_json"}
	}
	cmd.SpecialAttrsJSON = specialAttrsJSON
	cmd.UnitRuleOverrideJSON = unitRuleOverrideJSON
	return s.repo.CreateSKU(ctx, cmd)
}

func (s *Service) validateGreenBeanBomProduct(ctx context.Context, productID int64) error {
	product, err := s.repo.GetProduct(ctx, productID)
	if err != nil {
		return err
	}
	if product == nil || product.ID <= 0 {
		return ValidationError{Message: "green_bean_bom_product_id not found"}
	}
	if catalogdomain.NormalizeProductKind(product.ProductKind) == catalogdomain.ProductKindGreenBean {
		return ValidationError{Message: "green_bean_bom_product_id must reference roasted product"}
	}
	return nil
}

func normalizeProductSalesShape(productKind *string, greenBeanType *string, greenBeanBomProductID *int64, roastLevel *string, defaultPrice *float64, retailPrice100G *float64, retailPrice200G *float64, retailPrice227G *float64, retailPrice250G *float64, yieldRate *float64, tiers *[]PriceTier) error {
	kind := catalogdomain.NormalizeProductKind(*productKind)
	*productKind = kind
	if kind != catalogdomain.ProductKindGreenBean {
		*greenBeanType = ""
		*greenBeanBomProductID = 0
		inputRoast := strings.TrimSpace(*roastLevel)
		normalizedRoast := catalogdomain.NormalizeRoastLevel(inputRoast)
		if inputRoast != "" && normalizedRoast == "" {
			return ValidationError{Message: "invalid roast_level"}
		}
		if catalogdomain.ProductKindRequiresRoast(kind) {
			if normalizedRoast == "" {
				normalizedRoast = "中烘"
			}
			*roastLevel = normalizedRoast
			if *yieldRate <= 0 {
				*yieldRate = catalogdomain.ResolveYieldRate(*roastLevel, 0.8)
			}
			if *yieldRate <= 0 || *yieldRate > 1 {
				return ValidationError{Message: "invalid yield_rate"}
			}
			return nil
		}
		*roastLevel = ""
		if *yieldRate <= 0 {
			*yieldRate = 0
		}
		if *yieldRate > 1 {
			return ValidationError{Message: "invalid yield_rate"}
		}
		return nil
	}
	*greenBeanType = normalizeGreenBeanType(*greenBeanType)
	if *greenBeanBomProductID <= 0 {
		return fmt.Errorf("green_bean_bom_product_id required")
	}
	*roastLevel = ""
	*defaultPrice = 0
	*retailPrice100G = 0
	*retailPrice200G = 0
	*retailPrice227G = 0
	*retailPrice250G = 0
	*yieldRate = 0
	if tiers != nil {
		*tiers = nil
	}
	return nil
}

func normalizeGreenBeanType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "blend", "拼配":
		return "blend"
	case "single_origin", "single", "单品":
		return "single_origin"
	default:
		return "single_origin"
	}
}

func (s *Service) ProductSettings(ctx context.Context) (ProductSettingsData, error) {
	categories, err := s.repo.ListProductCategories(ctx)
	if err != nil {
		return ProductSettingsData{}, err
	}
	products, err := s.repo.ListProducts(ctx)
	if err != nil {
		return ProductSettingsData{}, err
	}
	productionConfigs, err := s.repo.ListProductProductionConfigs(ctx)
	if err != nil {
		return ProductSettingsData{}, err
	}
	classificationTemplates, err := s.repo.ListProductClassificationTemplates(ctx)
	if err != nil {
		return ProductSettingsData{}, err
	}
	classificationTemplateUsages, err := s.repo.ListProductClassificationTemplateUsages(ctx)
	if err != nil {
		return ProductSettingsData{}, err
	}
	templates, err := s.repo.ListGradientTemplates(ctx)
	if err != nil {
		return ProductSettingsData{}, err
	}
	configTemplates, err := s.repo.ListProductConfigTemplates(ctx)
	if err != nil {
		return ProductSettingsData{}, err
	}
	unitDefinitions, err := s.repo.ListProductUnitDefinitions(ctx)
	if err != nil {
		return ProductSettingsData{}, err
	}
	unitTemplates, err := s.repo.ListProductUnitTemplates(ctx)
	if err != nil {
		return ProductSettingsData{}, err
	}
	unitTemplates = decorateProductUnitTemplates(unitTemplates)
	priceGroups, err := s.repo.ListProductPriceGroups(ctx)
	if err != nil {
		return ProductSettingsData{}, err
	}
	priceRecords, err := s.repo.ListProductPriceRecords(ctx, ProductPriceRecordQuery{ActiveMode: "all"})
	if err != nil {
		return ProductSettingsData{}, err
	}
	tierPriceSchemes, err := s.repo.ListProductTierPriceSchemes(ctx, ProductTierPriceSchemeQuery{ActiveMode: "all"})
	if err != nil {
		return ProductSettingsData{}, err
	}
	usages, err := s.repo.ListCustomerPublicUsages(ctx)
	if err != nil {
		return ProductSettingsData{}, err
	}
	ruleTemplates, err := s.repo.ListCustomerProductRuleTemplates(ctx)
	if err != nil {
		return ProductSettingsData{}, err
	}
	ruleOverrides, err := s.repo.ListCustomerProductRuleOverrides(ctx)
	if err != nil {
		return ProductSettingsData{}, err
	}
	ruleBindings, err := s.repo.ListCustomerProductRuleBindings(ctx)
	if err != nil {
		return ProductSettingsData{}, err
	}
	businessGroups, err := s.repo.ListBusinessGroups(ctx)
	if err != nil {
		return ProductSettingsData{}, err
	}
	businessGroupAssignments, err := s.repo.ListBusinessGroupAssignments(ctx, BusinessGroupAssignmentQuery{})
	if err != nil {
		return ProductSettingsData{}, err
	}
	productCustomerReferences, err := s.repo.ListProductCustomerReferences(ctx, 0)
	if err != nil {
		return ProductSettingsData{}, err
	}
	productPricingRules, err := s.ListProductPricingRules(ctx)
	if err != nil {
		return ProductSettingsData{}, err
	}
	priceTierTemplates, err := s.repo.ListPriceTierTemplates(ctx)
	if err != nil {
		return ProductSettingsData{}, err
	}
	data := BuildProductSettings(categories, products)
	data.ProductProductionConfigs = productionConfigs
	data.ProductClassificationTemplates = classificationTemplates
	data.ProductClassificationTemplateUsages = classificationTemplateUsages
	data.GradientTemplates = templates
	data.ProductConfigTemplates = configTemplates
	data.ProductUnitDefinitions = unitDefinitions
	data.ProductUnitTemplates = unitTemplates
	data.ProductPriceGroups = priceGroups
	data.ProductPriceRecords = priceRecords
	data.ProductTierPriceSchemes = tierPriceSchemes
	data.BusinessGroups = businessGroups
	data.BusinessGroupAssignments = businessGroupAssignments
	data.ProductCustomerReferences = productCustomerReferences
	data.ProductPricingRules = productPricingRules
	data.PriceTierTemplates = priceTierTemplates
	data.CustomerPublicUsages = usages
	data.CustomerProductRuleTemplates = ruleTemplates
	data.CustomerProductRuleOverrides = ruleOverrides
	data.CustomerProductRuleBindings = ruleBindings
	return data, nil
}

func (s *Service) ListBusinessGroups(ctx context.Context) ([]BusinessGroup, error) {
	return s.repo.ListBusinessGroups(ctx)
}

func (s *Service) SaveBusinessGroup(ctx context.Context, cmd BusinessGroup) (BusinessGroup, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Code = strings.TrimSpace(cmd.Code)
	cmd.Remark = strings.TrimSpace(cmd.Remark)
	if cmd.ID < 0 {
		return BusinessGroup{}, ValidationError{Message: "invalid group"}
	}
	if cmd.Name == "" {
		return BusinessGroup{}, ValidationError{Message: "name required"}
	}
	if cmd.SortOrder <= 0 {
		cmd.SortOrder = 100
	}
	if cmd.ID == 0 {
		cmd.Active = true
	}
	return s.repo.SaveBusinessGroup(ctx, cmd)
}

func (s *Service) DeleteBusinessGroup(ctx context.Context, cmd DeleteBusinessGroupCommand) error {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.ID <= 0 {
		return ValidationError{Message: "invalid business group"}
	}
	return s.repo.DeleteBusinessGroup(ctx, cmd)
}

func (s *Service) SaveBusinessGroupItem(ctx context.Context, cmd BusinessGroupItem) (BusinessGroupItem, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Code = strings.TrimSpace(cmd.Code)
	cmd.Remark = strings.TrimSpace(cmd.Remark)
	if cmd.ID < 0 || cmd.GroupID < 0 || cmd.ParentID < 0 {
		return BusinessGroupItem{}, ValidationError{Message: "invalid business group item"}
	}
	if cmd.Name == "" {
		return BusinessGroupItem{}, ValidationError{Message: "name required"}
	}
	if cmd.ID == 0 && cmd.GroupID <= 0 {
		return BusinessGroupItem{}, ValidationError{Message: "group required"}
	}
	if cmd.SortOrder <= 0 {
		cmd.SortOrder = 100
	}
	if cmd.ID == 0 {
		cmd.Active = true
	}
	return s.repo.SaveBusinessGroupItem(ctx, cmd)
}

func (s *Service) DeleteBusinessGroupItem(ctx context.Context, cmd DeleteBusinessGroupItemCommand) error {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.ID <= 0 {
		return ValidationError{Message: "invalid business group item"}
	}
	return s.repo.DeleteBusinessGroupItem(ctx, cmd)
}

func (s *Service) MoveBusinessGroupItem(ctx context.Context, cmd MoveBusinessGroupItemCommand) (BusinessGroupItem, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.ID <= 0 || cmd.ParentID < 0 {
		return BusinessGroupItem{}, ValidationError{Message: "invalid business group item"}
	}
	if cmd.Position <= 0 {
		cmd.Position = 1
	}
	return s.repo.MoveBusinessGroupItem(ctx, cmd)
}

func (s *Service) EnsureBusinessGroupUsage(ctx context.Context, groupID int64, usageKey string, actor string) error {
	usageKey = strings.TrimSpace(usageKey)
	actor = strings.TrimSpace(actor)
	if groupID <= 0 || usageKey == "" {
		return ValidationError{Message: "invalid business group usage"}
	}
	switch usageKey {
	case BusinessGroupUsageProductCatalog, BusinessGroupUsageProductionBOM, BusinessGroupUsageWarehouseInventory, BusinessGroupUsagePriceList:
	default:
		return ValidationError{Message: "invalid business group usage"}
	}
	return s.repo.EnsureBusinessGroupUsage(ctx, groupID, usageKey, actor)
}

func (s *Service) ListBusinessGroupAssignments(ctx context.Context, query BusinessGroupAssignmentQuery) ([]BusinessGroupAssignment, error) {
	query.UsageKey = strings.TrimSpace(query.UsageKey)
	query.ObjectKey = strings.TrimSpace(query.ObjectKey)
	query.ObjectRef = strings.TrimSpace(query.ObjectRef)
	if query.ObjectID < 0 || query.GroupID < 0 || query.GroupItemID < 0 {
		return nil, ValidationError{Message: "invalid business group assignment query"}
	}
	return s.repo.ListBusinessGroupAssignments(ctx, query)
}

func (s *Service) SaveBusinessGroupAssignment(ctx context.Context, cmd BusinessGroupAssignment) (BusinessGroupAssignment, error) {
	cmd.UsageKey = strings.TrimSpace(cmd.UsageKey)
	cmd.ObjectKey = strings.TrimSpace(cmd.ObjectKey)
	cmd.ObjectRef = strings.TrimSpace(cmd.ObjectRef)
	if cmd.ID < 0 || cmd.GroupID <= 0 || cmd.GroupItemID <= 0 {
		return BusinessGroupAssignment{}, ValidationError{Message: "invalid business group assignment"}
	}
	if cmd.UsageKey == "" || cmd.ObjectKey == "" {
		return BusinessGroupAssignment{}, ValidationError{Message: "invalid business group assignment"}
	}
	if cmd.ObjectID <= 0 && cmd.ObjectRef == "" {
		return BusinessGroupAssignment{}, ValidationError{Message: "object required"}
	}
	if cmd.ObjectID > 0 {
		cmd.ObjectRef = ""
	}
	if cmd.SortOrder <= 0 {
		cmd.SortOrder = 100
	}
	return s.repo.SaveBusinessGroupAssignment(ctx, cmd)
}

func (s *Service) DeleteBusinessGroupAssignment(ctx context.Context, cmd DeleteBusinessGroupAssignmentCommand) error {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.ID <= 0 {
		return ValidationError{Message: "invalid business group assignment"}
	}
	return s.repo.DeleteBusinessGroupAssignment(ctx, cmd)
}

func (s *Service) ListProductCustomerReferences(ctx context.Context, productID int64) ([]ProductCustomerReference, error) {
	if productID < 0 {
		return nil, ValidationError{Message: "invalid product_id"}
	}
	return s.repo.ListProductCustomerReferences(ctx, productID)
}

func (s *Service) SaveProductCustomerReference(ctx context.Context, cmd ProductCustomerReference) (ProductCustomerReference, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	cmd.CustomerItemCode = strings.TrimSpace(cmd.CustomerItemCode)
	cmd.CustomerDisplayName = strings.TrimSpace(cmd.CustomerDisplayName)
	cmd.Remark = strings.TrimSpace(cmd.Remark)
	if cmd.ID < 0 || cmd.ProductID <= 0 || cmd.CustomerID <= 0 {
		return ProductCustomerReference{}, ValidationError{Message: "invalid product customer reference"}
	}
	if cmd.CustomerItemCode == "" && cmd.CustomerDisplayName == "" {
		return ProductCustomerReference{}, ValidationError{Message: "customer reference required"}
	}
	if cmd.ID == 0 {
		cmd.Active = true
	}
	return s.repo.SaveProductCustomerReference(ctx, cmd)
}

func (s *Service) ListProductPricingRules(ctx context.Context) ([]ProductPricingRule, error) {
	rules, err := s.repo.ListProductPricingRules(ctx)
	if err != nil {
		return nil, err
	}
	for i := range rules {
		legacyMethod := pricingRuleProfitMethod(rules[i].CalculationJSON)
		legacyRate := rules[i].MarginRate
		switch legacyMethod {
		case "", "gross_margin", "markup":
			rules[i].MarginRate = normalizeLegacyPricingRuleMarkupRate(legacyRate, legacyMethod)
			rules[i].CalculationJSON, err = normalizePricingRuleCalculationJSON(rules[i].CalculationJSON)
			if err != nil {
				return nil, err
			}
		default:
			calculationJSON, cloneErr := clonePricingRuleCalculationJSON(rules[i].CalculationJSON)
			if cloneErr != nil {
				return nil, cloneErr
			}
			calculationJSON["legacy_profit_method"] = legacyMethod
			calculationJSON["legacy_margin_rate"] = legacyRate
			calculationJSON["profit_method"] = "markup"
			calculationJSON["migration_warning"] = "only markup rate is supported; review this template before enabling it"
			rules[i].CalculationJSON = calculationJSON
			rules[i].MarginRate = 0
			rules[i].Active = false
		}
	}
	return rules, nil
}

func (s *Service) SaveProductPricingRule(ctx context.Context, cmd ProductPricingRule) (ProductPricingRule, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Code = strings.TrimSpace(cmd.Code)
	cmd.CostSourceMode = normalizePricingRuleCostSourceMode(cmd.CostSourceMode)
	cmd.RoundingMode = strings.TrimSpace(cmd.RoundingMode)
	cmd.FormulaVersion = strings.TrimSpace(cmd.FormulaVersion)
	cmd.Remark = strings.TrimSpace(cmd.Remark)
	if cmd.ID < 0 {
		return ProductPricingRule{}, ValidationError{Message: "invalid pricing rule"}
	}
	if cmd.Name == "" {
		return ProductPricingRule{}, ValidationError{Message: "name required"}
	}
	if cmd.RoundingMode == "" {
		cmd.RoundingMode = "none"
	}
	if cmd.FormulaVersion == "" {
		cmd.FormulaVersion = "v1"
	}
	legacyProfitMethod := pricingRuleProfitMethod(cmd.CalculationJSON)
	if math.IsNaN(cmd.MarginRate) || math.IsInf(cmd.MarginRate, 0) || math.IsNaN(cmd.TaxRate) || math.IsInf(cmd.TaxRate, 0) || cmd.MarginRate < 0 || cmd.TaxRate < 0 {
		return ProductPricingRule{}, ValidationError{Message: "rate must not be negative"}
	}
	if pricingRuleCalculationHasLegacyQuarantine(cmd.CalculationJSON) {
		return ProductPricingRule{}, ValidationError{Message: "quarantined legacy pricing rule must be replaced with a new markup template"}
	}
	if cmd.ID > 0 {
		existingRules, err := s.repo.ListProductPricingRules(ctx)
		if err != nil {
			return ProductPricingRule{}, err
		}
		for _, existing := range existingRules {
			if existing.ID == cmd.ID && pricingRuleCalculationNeedsQuarantine(existing.CalculationJSON) {
				return ProductPricingRule{}, ValidationError{Message: "quarantined legacy pricing rule must be replaced with a new markup template"}
			}
		}
	}
	calculationJSON, err := normalizePricingRuleCalculationJSON(cmd.CalculationJSON)
	if err != nil {
		return ProductPricingRule{}, ValidationError{Message: err.Error()}
	}
	cmd.CalculationJSON = calculationJSON
	cmd.MarginRate = normalizeLegacyPricingRuleMarkupRate(cmd.MarginRate, legacyProfitMethod)
	if cmd.ID == 0 {
		cmd.Active = true
	}
	return s.repo.SaveProductPricingRule(ctx, cmd)
}

func (s *Service) ListPriceTierTemplates(ctx context.Context) ([]PriceTierTemplate, error) {
	return s.repo.ListPriceTierTemplates(ctx)
}

func (s *Service) SavePriceTierTemplate(ctx context.Context, cmd PriceTierTemplate) (PriceTierTemplate, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Remark = strings.TrimSpace(cmd.Remark)
	if cmd.ID < 0 {
		return PriceTierTemplate{}, ValidationError{Message: "invalid price tier template"}
	}
	if cmd.Name == "" {
		return PriceTierTemplate{}, ValidationError{Message: "name required"}
	}
	if len(cmd.Tiers) == 0 {
		return PriceTierTemplate{}, ValidationError{Message: "tiers required"}
	}
	for i := range cmd.Tiers {
		cmd.Tiers[i].Label = strings.TrimSpace(cmd.Tiers[i].Label)
		cmd.Tiers[i].QuantityUnit = strings.TrimSpace(cmd.Tiers[i].QuantityUnit)
		cmd.Tiers[i].Remark = strings.TrimSpace(cmd.Tiers[i].Remark)
		if cmd.Tiers[i].MinQty < 0 {
			return PriceTierTemplate{}, ValidationError{Message: "min_qty must not be negative"}
		}
		if cmd.Tiers[i].MaxQty != nil && *cmd.Tiers[i].MaxQty <= cmd.Tiers[i].MinQty {
			return PriceTierTemplate{}, ValidationError{Message: "max_qty must be greater than min_qty"}
		}
		if cmd.Tiers[i].QuantityUnit == "" {
			cmd.Tiers[i].QuantityUnit = "kg"
		}
		if cmd.Tiers[i].Position <= 0 {
			cmd.Tiers[i].Position = i + 1
		}
		if cmd.Tiers[i].ID == 0 && !cmd.Tiers[i].Active {
			cmd.Tiers[i].Active = true
		}
		if cmd.Tiers[i].Active && cmd.Tiers[i].PricingRuleID <= 0 {
			return PriceTierTemplate{}, ValidationError{Message: "pricing_rule_id required"}
		}
	}
	sort.SliceStable(cmd.Tiers, func(i, j int) bool {
		if cmd.Tiers[i].Position != cmd.Tiers[j].Position {
			return cmd.Tiers[i].Position < cmd.Tiers[j].Position
		}
		return cmd.Tiers[i].MinQty < cmd.Tiers[j].MinQty
	})
	if cmd.ID == 0 {
		cmd.Active = true
	}
	return s.repo.SavePriceTierTemplate(ctx, cmd)
}

func (s *Service) DeletePriceTierTemplate(ctx context.Context, id int64, actor string) error {
	if id <= 0 {
		return ValidationError{Message: "invalid price tier template"}
	}
	return s.repo.DeletePriceTierTemplate(ctx, id, strings.TrimSpace(actor))
}

func ResolvePriceTableTemplateInheritance(input PriceTableTemplateResolutionInput) PriceTableTemplateResolution {
	findGroup := func(id int64) (PriceTableGroupTemplateAssignment, bool) {
		for _, item := range input.GroupAssignments {
			if item.GroupItemID == id {
				return item, true
			}
		}
		return PriceTableGroupTemplateAssignment{}, false
	}
	var override PriceTableProductTemplateOverride
	for _, item := range input.ProductOverrides {
		if item.ProductID == input.ProductID {
			override = item
			break
		}
	}
	subgroup, _ := findGroup(input.GroupItemID)
	parent, _ := findGroup(subgroup.ParentGroupItemID)
	tierID, tierSource := firstTemplateSource(
		templateCandidate{"product", override.TierTemplateID},
		templateCandidate{"subgroup", subgroup.TierTemplateID},
		templateCandidate{"parent_group", parent.TierTemplateID},
		templateCandidate{"default", input.DefaultTierTemplateID},
	)
	ruleID, ruleSource := firstTemplateSource(
		templateCandidate{"product", override.PricingRuleID},
		templateCandidate{"subgroup", subgroup.PricingRuleID},
		templateCandidate{"parent_group", parent.PricingRuleID},
		templateCandidate{"default", input.DefaultPricingRuleID},
	)
	fixedPrice, fixedSource := firstNumberSource(
		numberCandidate{"product", override.FixedUnitPrice},
		numberCandidate{"subgroup", subgroup.FixedUnitPrice},
		numberCandidate{"parent_group", parent.FixedUnitPrice},
		numberCandidate{"default", input.DefaultFixedUnitPrice},
	)
	mode, modeSource := firstTextSource(
		textCandidate{"product", normalizePriceTablePricingMode(override.PricingMode)},
		textCandidate{"subgroup", normalizePriceTablePricingMode(subgroup.PricingMode)},
		textCandidate{"parent_group", normalizePriceTablePricingMode(parent.PricingMode)},
		textCandidate{"default", normalizePriceTablePricingMode(input.DefaultPricingMode)},
	)
	if mode == "" {
		switch {
		case tierID > 0:
			mode, modeSource = "tier_template", tierSource
		case ruleID > 0:
			mode, modeSource = "pricing_rule", ruleSource
		case fixedPrice > 0:
			mode, modeSource = "fixed_price", fixedSource
		default:
			mode, modeSource = "tier_template", "default"
		}
	}
	return PriceTableTemplateResolution{
		PricingMode:          mode,
		PricingModeSource:    modeSource,
		TierTemplateID:       tierID,
		TierTemplateSource:   tierSource,
		PricingRuleID:        ruleID,
		PricingRuleSource:    ruleSource,
		FixedUnitPrice:       fixedPrice,
		FixedUnitPriceSource: fixedSource,
	}
}

type templateCandidate struct {
	source string
	id     int64
}

func firstTemplateSource(candidates ...templateCandidate) (int64, string) {
	for _, candidate := range candidates {
		if candidate.id > 0 {
			return candidate.id, candidate.source
		}
	}
	return 0, "default"
}

type numberCandidate struct {
	source string
	value  float64
}

func firstNumberSource(candidates ...numberCandidate) (float64, string) {
	for _, candidate := range candidates {
		if candidate.value > 0 {
			return candidate.value, candidate.source
		}
	}
	return 0, "default"
}

type textCandidate struct {
	source string
	value  string
}

func firstTextSource(candidates ...textCandidate) (string, string) {
	for _, candidate := range candidates {
		if candidate.value != "" {
			return candidate.value, candidate.source
		}
	}
	return "", "default"
}

func normalizePriceTablePricingMode(value string) string {
	switch strings.TrimSpace(value) {
	case "tier_template", "inherit_gradient_template":
		return "tier_template"
	case "pricing_rule", "cost_plus":
		return "pricing_rule"
	case "fixed_price", "fixed_unit_price":
		return "fixed_price"
	default:
		return ""
	}
}

func (s *Service) ListGradientTemplates(ctx context.Context) ([]GradientTemplate, error) {
	return s.repo.ListGradientTemplates(ctx)
}

func (s *Service) ListProductConfigTemplates(ctx context.Context) ([]ProductConfigTemplate, error) {
	return s.repo.ListProductConfigTemplates(ctx)
}

func (s *Service) ListProductClassificationTemplates(ctx context.Context) ([]ProductClassificationTemplate, error) {
	return s.repo.ListProductClassificationTemplates(ctx)
}

func (s *Service) SaveProductClassificationTemplate(ctx context.Context, cmd SaveProductClassificationTemplateCommand) (ProductClassificationTemplate, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Remark = strings.TrimSpace(cmd.Remark)
	cmd.CustomerID = 0
	if cmd.ID < 0 || cmd.SourceTemplateID < 0 {
		return ProductClassificationTemplate{}, ValidationError{Message: "invalid classification template"}
	}
	if cmd.Name == "" {
		return ProductClassificationTemplate{}, ValidationError{Message: "name required"}
	}
	if cmd.SortOrder <= 0 {
		cmd.SortOrder = 100
	}
	return s.repo.SaveProductClassificationTemplate(ctx, cmd)
}

func (s *Service) ListProductClassificationTemplateUsages(ctx context.Context) ([]ProductClassificationTemplateUsage, error) {
	return s.repo.ListProductClassificationTemplateUsages(ctx)
}

func (s *Service) SaveProductClassificationTemplateUsage(ctx context.Context, cmd SaveProductClassificationTemplateUsageCommand) (ProductClassificationTemplateUsage, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.ClassificationTemplateID <= 0 {
		return ProductClassificationTemplateUsage{}, ValidationError{Message: "invalid classification_template_id"}
	}
	if cmd.SortOrder <= 0 {
		cmd.SortOrder = 100
	}
	return s.repo.SaveProductClassificationTemplateUsage(ctx, cmd)
}

func (s *Service) DeleteProductClassificationTemplateUsage(ctx context.Context, cmd DeleteProductClassificationTemplateUsageCommand) error {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.ClassificationTemplateID <= 0 {
		return ValidationError{Message: "invalid classification_template_id"}
	}
	return s.repo.DeleteProductClassificationTemplateUsage(ctx, cmd)
}

func (s *Service) ListCustomerProductAliasClassificationTemplateUsages(ctx context.Context, customerID int64) ([]CustomerProductAliasClassificationTemplateUsage, error) {
	if customerID < 0 {
		return nil, ValidationError{Message: "invalid customer_id"}
	}
	return s.repo.ListCustomerProductAliasClassificationTemplateUsages(ctx, customerID)
}

func (s *Service) SaveCustomerProductAliasClassificationTemplateUsage(ctx context.Context, cmd SaveCustomerProductAliasClassificationTemplateUsageCommand) (CustomerProductAliasClassificationTemplateUsage, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.CustomerID <= 0 {
		return CustomerProductAliasClassificationTemplateUsage{}, ValidationError{Message: "customer_id required"}
	}
	if cmd.ClassificationTemplateID <= 0 {
		return CustomerProductAliasClassificationTemplateUsage{}, ValidationError{Message: "invalid classification_template_id"}
	}
	if cmd.SortOrder <= 0 {
		cmd.SortOrder = 100
	}
	return s.repo.SaveCustomerProductAliasClassificationTemplateUsage(ctx, cmd)
}

func (s *Service) DeleteCustomerProductAliasClassificationTemplateUsage(ctx context.Context, cmd DeleteCustomerProductAliasClassificationTemplateUsageCommand) error {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.CustomerID <= 0 {
		return ValidationError{Message: "customer_id required"}
	}
	if cmd.ClassificationTemplateID <= 0 {
		return ValidationError{Message: "invalid classification_template_id"}
	}
	return s.repo.DeleteCustomerProductAliasClassificationTemplateUsage(ctx, cmd)
}

func (s *Service) DeleteProductClassificationTemplate(ctx context.Context, cmd DeleteProductClassificationTemplateCommand) error {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.ID <= 0 {
		return ValidationError{Message: "invalid id"}
	}
	return s.repo.DeleteProductClassificationTemplate(ctx, cmd)
}

func (s *Service) SaveProductClassificationCategory(ctx context.Context, cmd SaveProductClassificationCategoryCommand) (ProductClassificationCategory, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	cmd.Name = strings.TrimSpace(cmd.Name)
	if cmd.ID < 0 || cmd.TemplateID <= 0 || cmd.ParentID < 0 {
		return ProductClassificationCategory{}, ValidationError{Message: "invalid classification category"}
	}
	if cmd.Name == "" {
		return ProductClassificationCategory{}, ValidationError{Message: "name required"}
	}
	if cmd.Level <= 0 {
		cmd.Level = 1
	}
	if cmd.SortOrder <= 0 {
		cmd.SortOrder = 100
	}
	return s.repo.SaveProductClassificationCategory(ctx, cmd)
}

func (s *Service) DeleteProductClassificationCategory(ctx context.Context, cmd DeleteProductClassificationCategoryCommand) error {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.ID <= 0 || cmd.TemplateID <= 0 {
		return ValidationError{Message: "invalid classification category"}
	}
	return s.repo.DeleteProductClassificationCategory(ctx, cmd)
}

func (s *Service) SaveProductClassificationAssignment(ctx context.Context, cmd SaveProductClassificationAssignmentCommand) (ProductClassificationAssignment, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.ProductID <= 0 || cmd.TemplateID <= 0 || cmd.CategoryID < 0 {
		return ProductClassificationAssignment{}, ValidationError{Message: "invalid classification assignment"}
	}
	if cmd.SortOrder <= 0 {
		cmd.SortOrder = 100
	}
	return s.repo.SaveProductClassificationAssignment(ctx, cmd)
}

func (s *Service) SaveCustomerProductAliasClassificationAssignment(ctx context.Context, cmd SaveCustomerProductAliasClassificationAssignmentCommand) (CustomerProductAliasClassificationAssignment, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.AliasID <= 0 || cmd.TemplateID <= 0 || cmd.CategoryID < 0 {
		return CustomerProductAliasClassificationAssignment{}, ValidationError{Message: "invalid customer alias classification assignment"}
	}
	if cmd.SortOrder <= 0 {
		cmd.SortOrder = 100
	}
	return s.repo.SaveCustomerProductAliasClassificationAssignment(ctx, cmd)
}

func (s *Service) ListProductUnitDefinitions(ctx context.Context) ([]ProductUnitDefinition, error) {
	return s.repo.ListProductUnitDefinitions(ctx)
}

func (s *Service) ListProductUnitTemplates(ctx context.Context) ([]ProductUnitTemplate, error) {
	rows, err := s.repo.ListProductUnitTemplates(ctx)
	if err != nil {
		return nil, err
	}
	return decorateProductUnitTemplates(rows), nil
}

func (s *Service) ListProductPriceGroups(ctx context.Context) ([]ProductPriceGroup, error) {
	return s.repo.ListProductPriceGroups(ctx)
}

func (s *Service) SaveProductPriceGroup(ctx context.Context, cmd SaveProductPriceGroupCommand) (ProductPriceGroup, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	cmd.Name = strings.TrimSpace(cmd.Name)
	if cmd.ID < 0 {
		return ProductPriceGroup{}, ValidationError{Message: "invalid id"}
	}
	if cmd.Name == "" {
		return ProductPriceGroup{}, ValidationError{Message: "name required"}
	}
	if cmd.SortOrder <= 0 {
		cmd.SortOrder = 100
	}
	return s.repo.SaveProductPriceGroup(ctx, cmd)
}

func (s *Service) ListProductPriceRecords(ctx context.Context, query ProductPriceRecordQuery) ([]ProductPriceRecord, error) {
	if query.ProductID < 0 || query.CustomerProductAliasID < 0 || query.PriceGroupID < 0 {
		return nil, ValidationError{Message: "invalid price record query"}
	}
	query.ActiveMode = strings.TrimSpace(query.ActiveMode)
	if strings.TrimSpace(query.Status) != "" {
		query.Status = normalizeProductPriceStatus(query.Status)
	}
	return s.repo.ListProductPriceRecords(ctx, query)
}

func (s *Service) SaveProductPriceRecord(ctx context.Context, cmd SaveProductPriceRecordCommand) (ProductPriceRecord, error) {
	return ProductPriceRecord{}, ValidationError{Message: "product price records are legacy readonly; use product pricing rules and price lists"}
	normalized, err := normalizeProductPriceRecordCommand(cmd)
	if err != nil {
		return ProductPriceRecord{}, err
	}
	return s.repo.SaveProductPriceRecord(ctx, normalized)
}

func (s *Service) ListProductTierPriceSchemes(ctx context.Context, query ProductTierPriceSchemeQuery) ([]ProductTierPriceScheme, error) {
	if query.ProductID < 0 || query.CustomerProductAliasID < 0 || query.PriceGroupID < 0 {
		return nil, ValidationError{Message: "invalid tier price scheme query"}
	}
	query.ActiveMode = strings.TrimSpace(query.ActiveMode)
	return s.repo.ListProductTierPriceSchemes(ctx, query)
}

func (s *Service) SaveProductTierPriceScheme(ctx context.Context, cmd SaveProductTierPriceSchemeCommand) (ProductTierPriceScheme, error) {
	return ProductTierPriceScheme{}, ValidationError{Message: "product tier price schemes are legacy readonly; use price tier templates on price lists"}
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Remark = strings.TrimSpace(cmd.Remark)
	if cmd.ID < 0 || cmd.ProductID < 0 || cmd.CustomerProductAliasID < 0 || cmd.PriceGroupID < 0 {
		return ProductTierPriceScheme{}, ValidationError{Message: "invalid tier price scheme"}
	}
	if cmd.ProductID <= 0 && cmd.CustomerProductAliasID <= 0 {
		return ProductTierPriceScheme{}, ValidationError{Message: "product or customer product required"}
	}
	if cmd.Name == "" {
		return ProductTierPriceScheme{}, ValidationError{Message: "name required"}
	}
	if len(cmd.Tiers) == 0 {
		return ProductTierPriceScheme{}, ValidationError{Message: "tiers required"}
	}
	normalized := make([]ProductTierPriceSchemeTier, 0, len(cmd.Tiers))
	for i, tier := range cmd.Tiers {
		tier.Label = strings.TrimSpace(tier.Label)
		if tier.SourcePriceRecordID <= 0 {
			return ProductTierPriceScheme{}, ValidationError{Message: "source_price_record_id required"}
		}
		if tier.MinQty < 0 {
			return ProductTierPriceScheme{}, ValidationError{Message: "min_qty must not be negative"}
		}
		if tier.MaxQty != nil && *tier.MaxQty <= tier.MinQty {
			return ProductTierPriceScheme{}, ValidationError{Message: "max_qty must be greater than min_qty"}
		}
		if tier.Position <= 0 {
			tier.Position = i + 1
		}
		source, err := s.repo.GetProductPriceRecord(ctx, tier.SourcePriceRecordID)
		if err != nil {
			return ProductTierPriceScheme{}, err
		}
		if source.ID <= 0 || !source.Active {
			return ProductTierPriceScheme{}, ValidationError{Message: "source price record not found"}
		}
		if source.FinalUnitPrice <= 0 {
			return ProductTierPriceScheme{}, ValidationError{Message: "source price record final_unit_price required"}
		}
		if strings.TrimSpace(source.PriceUnit) == "" {
			return ProductTierPriceScheme{}, ValidationError{Message: "source price record price_unit required"}
		}
		tier.FinalUnitPrice = source.FinalUnitPrice
		tier.PriceUnit = strings.TrimSpace(source.PriceUnit)
		tier.Currency = normalizeCurrency(source.Currency)
		if tier.Label == "" {
			tier.Label = fmt.Sprintf("价格记录 %d", tier.SourcePriceRecordID)
		}
		normalized = append(normalized, tier)
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		if normalized[i].Position != normalized[j].Position {
			return normalized[i].Position < normalized[j].Position
		}
		return normalized[i].MinQty < normalized[j].MinQty
	})
	cmd.Tiers = normalized
	return s.repo.SaveProductTierPriceScheme(ctx, cmd)
}

func (s *Service) SaveProductUnitDefinition(ctx context.Context, cmd SaveProductUnitDefinitionCommand) (ProductUnitDefinition, error) {
	normalized, err := normalizeProductUnitDefinitionCommand(cmd)
	if err != nil {
		return ProductUnitDefinition{}, err
	}
	return s.repo.SaveProductUnitDefinition(ctx, normalized)
}

func (s *Service) SaveProductUnitTemplate(ctx context.Context, cmd SaveProductUnitTemplateCommand) (ProductUnitTemplate, error) {
	normalized, err := normalizeProductUnitTemplateCommand(cmd)
	if err != nil {
		return ProductUnitTemplate{}, err
	}
	row, err := s.repo.SaveProductUnitTemplate(ctx, normalized)
	if err != nil {
		return ProductUnitTemplate{}, err
	}
	return decorateProductUnitTemplate(row), nil
}

func (s *Service) DeleteProductUnitDefinition(ctx context.Context, cmd DeleteProductUnitDefinitionCommand) error {
	normalized, err := normalizeDeleteProductUnitDefinitionCommand(cmd)
	if err != nil {
		return err
	}
	return s.repo.DeleteProductUnitDefinition(ctx, normalized)
}

func (s *Service) DeleteProductUnitTemplate(ctx context.Context, cmd DeleteProductUnitTemplateCommand) error {
	normalized, err := normalizeDeleteProductUnitTemplateCommand(cmd)
	if err != nil {
		return err
	}
	return s.repo.DeleteProductUnitTemplate(ctx, normalized)
}

func (s *Service) SaveProductConfigTemplate(ctx context.Context, cmd SaveProductConfigTemplateCommand) (ProductConfigTemplate, error) {
	normalized, err := normalizeProductConfigTemplateCommand(cmd)
	if err != nil {
		return ProductConfigTemplate{}, err
	}
	return s.repo.SaveProductConfigTemplate(ctx, normalized)
}

func (s *Service) DeleteProductConfigTemplate(ctx context.Context, cmd DeleteProductConfigTemplateCommand) error {
	if cmd.ID <= 0 {
		return ValidationError{Message: "invalid id"}
	}
	return s.repo.DeleteProductConfigTemplate(ctx, cmd)
}

func (s *Service) DeriveProductConfigTemplate(ctx context.Context, cmd DeriveProductConfigTemplateCommand) (ProductConfigTemplate, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	cmd.Name = strings.TrimSpace(cmd.Name)
	if cmd.CustomerID <= 0 {
		return ProductConfigTemplate{}, ValidationError{Message: "customer_id required"}
	}
	if cmd.SourceTemplateID <= 0 {
		return ProductConfigTemplate{}, ValidationError{Message: "source_template_id required"}
	}
	template, err := s.repo.DeriveProductConfigTemplate(ctx, cmd)
	if err != nil {
		return ProductConfigTemplate{}, err
	}
	if err := s.enableCustomerPublicProductReference(ctx, cmd.Actor, cmd.CustomerID); err != nil {
		return ProductConfigTemplate{}, err
	}
	return template, nil
}

func (s *Service) enableCustomerPublicProductReference(ctx context.Context, actor string, customerID int64) error {
	usages, err := s.repo.ListCustomerPublicUsages(ctx)
	if err != nil {
		return err
	}
	current := CustomerPublicUsage{CustomerID: customerID}
	for _, usage := range usages {
		if usage.CustomerID == customerID {
			current = usage
			break
		}
	}
	if current.UsePublicSKU && current.UsePublicCategories {
		return nil
	}
	_, err = s.repo.SaveCustomerPublicUsage(ctx, CustomerPublicUsageCommand{
		Actor:                      actor,
		CustomerID:                 customerID,
		UsePublicSKU:               true,
		UsePublicCategories:        true,
		UsePublicGradientTemplates: current.UsePublicGradientTemplates,
	})
	return err
}

func (s *Service) SaveGradientTemplate(ctx context.Context, cmd SaveGradientTemplateCommand) (GradientTemplate, error) {
	normalized, err := normalizeGradientTemplateCommand(cmd)
	if err != nil {
		return GradientTemplate{}, err
	}
	return s.repo.SaveGradientTemplate(ctx, normalized)
}

func (s *Service) DeactivateGradientTemplate(ctx context.Context, cmd DeactivateGradientTemplateCommand) error {
	if cmd.ID <= 0 {
		return fmt.Errorf("invalid id")
	}
	return s.repo.DeactivateGradientTemplate(ctx, cmd)
}

func (s *Service) BindCategoryGradientTemplate(ctx context.Context, cmd BindCategoryGradientTemplateCommand) error {
	if cmd.CategoryID <= 0 {
		return fmt.Errorf("invalid category")
	}
	if cmd.GradientTemplateID < 0 {
		return fmt.Errorf("invalid gradient_template_id")
	}
	return s.repo.BindCategoryGradientTemplate(ctx, cmd)
}

func (s *Service) SaveProductCategory(ctx context.Context, cmd SaveProductCategoryCommand) (ProductCategory, error) {
	cmd.Name = strings.TrimSpace(cmd.Name)
	if cmd.Name == "" {
		return ProductCategory{}, ValidationError{Message: "name required"}
	}
	if cmd.GradientTemplateID < 0 || cmd.OperationTemplateID < 0 {
		return ProductCategory{}, ValidationError{Message: "invalid template id"}
	}
	if cmd.ProductConfigTemplateID < 0 {
		return ProductCategory{}, ValidationError{Message: "invalid product_config_template_id"}
	}
	priceRuleJSON, err := normalizeJSONText(cmd.PriceListRuleJSON)
	if err != nil {
		return ProductCategory{}, ValidationError{Message: "invalid price_list_rule_json"}
	}
	unitRule := catalogdomain.NormalizeProductUnitRule(catalogdomain.ProductUnitRule{
		InventoryUnit:  cmd.InventoryUnit,
		QuoteUnit:      cmd.QuoteUnit,
		OrderUnit:      cmd.OrderUnit,
		ConversionJSON: cmd.UnitConversionJSON,
		IntegerUnit:    cmd.IntegerUnit,
	})
	unitConversionJSON, err := normalizeJSONText(unitRule.ConversionJSON)
	if err != nil {
		return ProductCategory{}, ValidationError{Message: "invalid unit_conversion_json"}
	}
	cmd.PriceListRuleJSON = priceRuleJSON
	cmd.InventoryUnit = unitRule.InventoryUnit
	cmd.QuoteUnit = unitRule.QuoteUnit
	cmd.OrderUnit = unitRule.OrderUnit
	cmd.UnitConversionJSON = unitConversionJSON
	cmd.IntegerUnit = unitRule.IntegerUnit
	return s.repo.SaveProductCategory(ctx, cmd)
}

func (s *Service) MoveProductCategory(ctx context.Context, cmd MoveProductCategoryCommand) error {
	return s.repo.MoveProductCategory(ctx, cmd)
}

func (s *Service) DeleteProductCategory(ctx context.Context, cmd DeleteProductCategoryCommand) error {
	return s.repo.DeleteProductCategory(ctx, cmd)
}

func (s *Service) AssignProductCategory(ctx context.Context, cmd AssignProductCategoryCommand) (AssignProductCategoryResult, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	return s.repo.AssignProductCategory(ctx, cmd)
}

func (s *Service) CreateCustomProduct(ctx context.Context, cmd CreateCustomProductCommand) (Product, error) {
	if cmd.CustomerID <= 0 {
		return Product{}, fmt.Errorf("customer_id required")
	}
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Remark = strings.TrimSpace(cmd.Remark)
	if cmd.Name == "" {
		return Product{}, fmt.Errorf("name required")
	}
	cmd.CustomType = strings.TrimSpace(cmd.CustomType)
	if cmd.CustomType != "custom_blend" && cmd.CustomType != "custom_roast" && cmd.CustomType != "public_sku_alias" {
		return Product{}, fmt.Errorf("invalid custom_type")
	}
	requestedKind := strings.TrimSpace(cmd.ProductKind)
	cmd.ProductKind = catalogdomain.NormalizeProductKind(cmd.ProductKind)
	var base *Product
	var err error
	requiresBaseProduct := cmd.CustomType != "custom_roast" && (requestedKind == "" || cmd.ProductKind != catalogdomain.ProductKindGreenBean)
	if requiresBaseProduct {
		if cmd.BaseProductID <= 0 {
			return Product{}, fmt.Errorf("base_product_id required")
		}
		base, err = s.repo.GetProduct(ctx, cmd.BaseProductID)
		if err != nil {
			return Product{}, err
		}
		if base == nil || base.ID <= 0 {
			return Product{}, fmt.Errorf("base product not found")
		}
		baseKind := catalogdomain.NormalizeProductKind(base.ProductKind)
		if requestedKind == "" {
			cmd.ProductKind = baseKind
		} else if cmd.ProductKind != baseKind {
			return Product{}, fmt.Errorf("product_kind must match base product")
		}
	}
	if cmd.CustomType == "custom_roast" {
		cmd.BaseProductID = 0
		cmd.CopyBOM = false
		cmd.CopyPriceTiers = false
	}
	if cmd.ProductKind == catalogdomain.ProductKindGreenBean {
		cmd.BaseProductID = 0
		cmd.RoastLevel = ""
		cmd.GreenBeanType = normalizeGreenBeanType(cmd.GreenBeanType)
		if cmd.GreenBeanBomProductID <= 0 && base != nil {
			cmd.GreenBeanBomProductID = base.GreenBeanBomProductID
		}
		if cmd.GreenBeanBomProductID <= 0 {
			return Product{}, ValidationError{Message: "green_bean_bom_product_id required"}
		}
		if err := s.validateGreenBeanBomProduct(ctx, cmd.GreenBeanBomProductID); err != nil {
			return Product{}, err
		}
		cmd.CopyBOM = false
		cmd.CopyPriceTiers = false
	} else if catalogdomain.ProductKindSupportsBomParams(cmd.ProductKind) {
		if catalogdomain.ProductKindRequiresRoast(cmd.ProductKind) {
			cmd.RoastLevel = catalogdomain.NormalizeRoastLevel(cmd.RoastLevel)
			if cmd.RoastLevel == "" {
				if base != nil {
					cmd.RoastLevel = catalogdomain.NormalizeRoastLevel(base.RoastLevel)
				}
				if cmd.RoastLevel == "" {
					cmd.RoastLevel = "中烘"
				}
			}
			if cmd.YieldRate <= 0 && base != nil {
				cmd.YieldRate = base.YieldRate
			}
			if cmd.YieldRate <= 0 {
				cmd.YieldRate = catalogdomain.ResolveYieldRate(cmd.RoastLevel, 0.8)
			}
		} else {
			cmd.RoastLevel = ""
			if cmd.YieldRate <= 0 {
				cmd.YieldRate = 0
			}
		}
		if cmd.YieldRate > 1 {
			return Product{}, ValidationError{Message: "invalid yield_rate"}
		}
		cmd.GreenBeanType = ""
		cmd.GreenBeanBomProductID = 0
	} else {
		cmd.RoastLevel = ""
		cmd.GreenBeanType = ""
		cmd.GreenBeanBomProductID = 0
		cmd.CopyBOM = false
	}
	if cmd.ProductKind == catalogdomain.ProductKindDripBag {
		if cmd.DripBagGrams <= 0 && base != nil {
			cmd.DripBagGrams = base.DripBagGrams
		}
		if cmd.DripBoxBagCount <= 0 && base != nil {
			cmd.DripBoxBagCount = base.DripBoxBagCount
		}
		var salesUnits []string
		cmd.ProductKind, cmd.DripBagGrams, cmd.DripBoxBagCount, salesUnits, err = normalizeProductKindSettings(cmd.ProductKind, cmd.DripBagGrams, cmd.DripBoxBagCount)
		if err != nil {
			return Product{}, err
		}
		_ = salesUnits
		cmd.CopyBOM = false
	}
	specialAttrsJSON, err := normalizeJSONObjectText(cmd.SpecialAttrsJSON)
	if err != nil {
		return Product{}, ValidationError{Message: "invalid special_attrs_json"}
	}
	cmd.SpecialAttrsJSON = specialAttrsJSON
	return s.repo.CreateCustomProduct(ctx, cmd)
}

func (s *Service) DeriveProductCategory(ctx context.Context, cmd DeriveProductCategoryCommand) (ProductCategory, error) {
	if cmd.CustomerID <= 0 {
		return ProductCategory{}, fmt.Errorf("customer_id required")
	}
	if cmd.SourceCategoryID <= 0 {
		return ProductCategory{}, fmt.Errorf("source_category_id required")
	}
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	category, err := s.repo.DeriveProductCategory(ctx, cmd)
	if err != nil {
		return ProductCategory{}, err
	}
	if err := s.enableCustomerPublicProductReference(ctx, cmd.Actor, cmd.CustomerID); err != nil {
		return ProductCategory{}, err
	}
	return category, nil
}

func (s *Service) DeriveCustomerProduct(ctx context.Context, cmd DeriveCustomerProductCommand) (Product, error) {
	if cmd.CustomerID <= 0 {
		return Product{}, fmt.Errorf("customer_id required")
	}
	if cmd.BaseProductID <= 0 {
		return Product{}, fmt.Errorf("base_product_id required")
	}
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	cmd.Name = strings.TrimSpace(cmd.Name)
	return s.repo.DeriveCustomerProduct(ctx, cmd)
}

func (s *Service) DeriveGradientTemplate(ctx context.Context, cmd DeriveGradientTemplateCommand) (GradientTemplate, error) {
	if cmd.CustomerID <= 0 {
		return GradientTemplate{}, fmt.Errorf("customer_id required")
	}
	if cmd.SourceTemplateID <= 0 {
		return GradientTemplate{}, fmt.Errorf("source_template_id required")
	}
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	cmd.Name = strings.TrimSpace(cmd.Name)
	return s.repo.DeriveGradientTemplate(ctx, cmd)
}

func (s *Service) SaveCustomerPublicUsage(ctx context.Context, cmd CustomerPublicUsageCommand) (CustomerPublicUsage, error) {
	if cmd.CustomerID <= 0 {
		return CustomerPublicUsage{}, fmt.Errorf("customer_id required")
	}
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	return s.repo.SaveCustomerPublicUsage(ctx, cmd)
}

func (s *Service) EnsureFactoryCustomer(ctx context.Context, actor string) (int64, error) {
	return s.repo.EnsureFactoryCustomer(ctx, strings.TrimSpace(actor))
}

func (s *Service) ListCustomerProductAliases(ctx context.Context, query CustomerProductAliasQuery) ([]CustomerProductAlias, error) {
	if query.CustomerID < 0 {
		return nil, ValidationError{Message: "invalid customer_id"}
	}
	query.ActiveMode = strings.ToLower(strings.TrimSpace(query.ActiveMode))
	if query.ActiveMode == "" {
		if query.ActiveOnly {
			query.ActiveMode = "active"
		} else {
			query.ActiveMode = "all"
		}
	}
	if query.ActiveMode != "active" && query.ActiveMode != "inactive" && query.ActiveMode != "all" {
		return nil, ValidationError{Message: "invalid active"}
	}
	query.SearchQuery = strings.TrimSpace(query.SearchQuery)
	return s.repo.ListCustomerProductAliases(ctx, query)
}

func (s *Service) SaveCustomerProductAlias(ctx context.Context, cmd CustomerProductAliasCommand) (CustomerProductAlias, error) {
	return CustomerProductAlias{}, ValidationError{Message: "customer products are legacy readonly; use product customer references"}
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	cmd.DisplayName = strings.TrimSpace(cmd.DisplayName)
	cmd.CustomerItemCode = strings.TrimSpace(cmd.CustomerItemCode)
	cmd.BrandName = strings.TrimSpace(cmd.BrandName)
	cmd.Remark = strings.TrimSpace(cmd.Remark)
	if cmd.ID < 0 {
		return CustomerProductAlias{}, ValidationError{Message: "invalid id"}
	}
	if cmd.CustomerID <= 0 {
		return CustomerProductAlias{}, ValidationError{Message: "customer_id required"}
	}
	if cmd.ProductID <= 0 {
		return CustomerProductAlias{}, ValidationError{Message: "product_id required"}
	}
	if cmd.DisplayName == "" {
		return CustomerProductAlias{}, ValidationError{Message: "display_name required"}
	}
	if cmd.DisplayCategoryID < 0 {
		return CustomerProductAlias{}, ValidationError{Message: "invalid display_category_id"}
	}
	if cmd.ProductConfigTemplateID < 0 {
		return CustomerProductAlias{}, ValidationError{Message: "invalid product_config_template_id"}
	}
	if cmd.GradientTemplateID < 0 {
		return CustomerProductAlias{}, ValidationError{Message: "invalid gradient_template_id"}
	}
	if cmd.UnitTemplateID < 0 {
		return CustomerProductAlias{}, ValidationError{Message: "invalid unit_template_id"}
	}
	cmd.ProductConfigTemplateID = 0
	cmd.GradientTemplateID = 0
	cmd.UnitTemplateID = 0
	cmd.ClassificationTemplateID = 0
	return s.repo.SaveCustomerProductAlias(ctx, cmd)
}

func (s *Service) DisableCustomerProductAlias(ctx context.Context, cmd DisableCustomerProductAliasCommand) error {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.ID <= 0 {
		return ValidationError{Message: "invalid id"}
	}
	return s.repo.DisableCustomerProductAlias(ctx, cmd)
}

func (s *Service) BatchDisableCustomerProductAliases(ctx context.Context, cmd BatchDisableCustomerProductAliasesCommand) (BatchDisableCustomerProductAliasesResult, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	seen := map[int64]bool{}
	ids := make([]int64, 0, len(cmd.IDs))
	for _, id := range cmd.IDs {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return BatchDisableCustomerProductAliasesResult{}, ValidationError{Message: "ids required"}
	}
	cmd.IDs = ids
	return s.repo.BatchDisableCustomerProductAliases(ctx, cmd)
}

func (s *Service) ListCustomerProductAliasIndustryFields(ctx context.Context, query CustomerProductAliasIndustryFieldQuery) ([]ProductProductionConfigField, error) {
	if query.AliasID <= 0 {
		return nil, ValidationError{Message: "invalid alias_id"}
	}
	return s.repo.ListCustomerProductAliasIndustryFields(ctx, query)
}

func (s *Service) SaveCustomerProductAliasIndustryFields(ctx context.Context, cmd SaveCustomerProductAliasIndustryFieldsCommand) ([]ProductProductionConfigField, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.AliasID <= 0 {
		return nil, ValidationError{Message: "invalid alias_id"}
	}
	fields := make([]ProductProductionConfigField, 0, len(cmd.Fields))
	seen := map[string]bool{}
	for _, field := range cmd.Fields {
		key := strings.TrimSpace(field.FieldKey)
		if key == "" || seen[strings.ToLower(key)] {
			continue
		}
		field.FieldKey = key
		field.ValueText = strings.TrimSpace(field.ValueText)
		fields = append(fields, field)
		seen[strings.ToLower(key)] = true
	}
	cmd.Fields = fields
	return s.repo.SaveCustomerProductAliasIndustryFields(ctx, cmd)
}

func (s *Service) BatchCreateCustomerProductAliases(ctx context.Context, cmd BatchCustomerProductAliasesCommand) (BatchCustomerProductAliasesResult, error) {
	return BatchCustomerProductAliasesResult{}, ValidationError{Message: "customer products are legacy readonly; use product customer references"}
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	cmd.BrandName = strings.TrimSpace(cmd.BrandName)
	if cmd.CustomerID <= 0 {
		return BatchCustomerProductAliasesResult{}, ValidationError{Message: "customer_id required"}
	}
	if cmd.DisplayCategoryID < 0 {
		return BatchCustomerProductAliasesResult{}, ValidationError{Message: "invalid display_category_id"}
	}
	cmd.ClassificationTemplateID = 0
	seen := map[int64]bool{}
	ids := make([]int64, 0, len(cmd.ProductIDs))
	for _, id := range cmd.ProductIDs {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return BatchCustomerProductAliasesResult{}, ValidationError{Message: "product_ids required"}
	}
	cmd.ProductIDs = ids
	return s.repo.BatchCreateCustomerProductAliases(ctx, cmd)
}

func (s *Service) ListCustomerProductAliasMigrationCandidates(ctx context.Context, query CustomerProductAliasMigrationCandidateQuery) ([]CustomerProductAliasMigrationCandidate, error) {
	if query.CustomerID <= 0 {
		return nil, ValidationError{Message: "customer_id required"}
	}
	return s.repo.ListCustomerProductAliasMigrationCandidates(ctx, query)
}

func (s *Service) SaveCustomerProductRuleTemplate(ctx context.Context, cmd SaveCustomerProductRuleTemplateCommand) (CustomerProductRuleTemplate, error) {
	normalized, err := normalizeCustomerProductRuleTemplateCommand(cmd)
	if err != nil {
		return CustomerProductRuleTemplate{}, err
	}
	return s.repo.SaveCustomerProductRuleTemplate(ctx, normalized)
}

func (s *Service) SaveCustomerProductRuleOverride(ctx context.Context, cmd SaveCustomerProductRuleOverrideCommand) (CustomerProductRuleOverride, error) {
	normalized, err := normalizeCustomerProductRuleOverrideCommand(cmd)
	if err != nil {
		return CustomerProductRuleOverride{}, err
	}
	return s.repo.SaveCustomerProductRuleOverride(ctx, normalized)
}

func (s *Service) BindCustomerProductRuleTemplate(ctx context.Context, cmd CustomerProductRuleTemplateBindingCommand) (CustomerProductRuleBinding, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.CustomerID <= 0 {
		return CustomerProductRuleBinding{}, ValidationError{Message: "customer_id required"}
	}
	if cmd.TemplateID < 0 {
		return CustomerProductRuleBinding{}, ValidationError{Message: "invalid template_id"}
	}
	return s.repo.BindCustomerProductRuleTemplate(ctx, cmd)
}

func normalizeCustomerProductRuleTemplateCommand(cmd SaveCustomerProductRuleTemplateCommand) (SaveCustomerProductRuleTemplateCommand, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	cmd.Name = strings.TrimSpace(cmd.Name)
	if cmd.ID < 0 {
		return SaveCustomerProductRuleTemplateCommand{}, ValidationError{Message: "invalid id"}
	}
	if cmd.CustomerID < 0 {
		return SaveCustomerProductRuleTemplateCommand{}, ValidationError{Message: "invalid customer_id"}
	}
	if cmd.Name == "" {
		return SaveCustomerProductRuleTemplateCommand{}, ValidationError{Message: "name required"}
	}
	if len(cmd.Items) == 0 {
		return SaveCustomerProductRuleTemplateCommand{}, ValidationError{Message: "items required"}
	}
	for i := range cmd.Items {
		item, err := normalizeCustomerProductRuleTemplateItem(cmd.Items[i])
		if err != nil {
			return SaveCustomerProductRuleTemplateCommand{}, err
		}
		cmd.Items[i] = item
	}
	return cmd, nil
}

func normalizeCustomerProductRuleTemplateItem(item CustomerProductRuleTemplateItem) (CustomerProductRuleTemplateItem, error) {
	if item.ProductSubtypeCategoryID <= 0 {
		return CustomerProductRuleTemplateItem{}, ValidationError{Message: "product_subtype_category_id required"}
	}
	if item.GradientTemplateID < 0 || item.OperationTemplateID < 0 {
		return CustomerProductRuleTemplateItem{}, ValidationError{Message: "invalid template id"}
	}
	priceRule, err := normalizeJSONText(item.PriceListRuleJSON)
	if err != nil {
		return CustomerProductRuleTemplateItem{}, ValidationError{Message: "invalid price_list_rule_json"}
	}
	unitRule, err := normalizeJSONText(item.UnitRuleJSON)
	if err != nil {
		return CustomerProductRuleTemplateItem{}, ValidationError{Message: "invalid unit_rule_json"}
	}
	item.PriceListRuleJSON = priceRule
	item.UnitRuleJSON = unitRule
	return item, nil
}

func normalizeCustomerProductRuleOverrideCommand(cmd SaveCustomerProductRuleOverrideCommand) (SaveCustomerProductRuleOverrideCommand, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.ID < 0 {
		return SaveCustomerProductRuleOverrideCommand{}, ValidationError{Message: "invalid id"}
	}
	if cmd.CustomerID <= 0 {
		return SaveCustomerProductRuleOverrideCommand{}, ValidationError{Message: "customer_id required"}
	}
	item, err := normalizeCustomerProductRuleTemplateItem(CustomerProductRuleTemplateItem{
		ProductSubtypeCategoryID: cmd.ProductSubtypeCategoryID,
		GradientTemplateID:       cmd.GradientTemplateID,
		OperationTemplateID:      cmd.OperationTemplateID,
		PriceListRuleJSON:        cmd.PriceListRuleJSON,
		UnitRuleJSON:             cmd.UnitRuleJSON,
	})
	if err != nil {
		return SaveCustomerProductRuleOverrideCommand{}, err
	}
	cmd.ProductSubtypeCategoryID = item.ProductSubtypeCategoryID
	cmd.GradientTemplateID = item.GradientTemplateID
	cmd.OperationTemplateID = item.OperationTemplateID
	cmd.PriceListRuleJSON = item.PriceListRuleJSON
	cmd.UnitRuleJSON = item.UnitRuleJSON
	return cmd, nil
}

func normalizeProductPriceRecordCommand(cmd SaveProductPriceRecordCommand) (SaveProductPriceRecordCommand, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	cmd.PriceUnit = strings.TrimSpace(cmd.PriceUnit)
	cmd.Currency = normalizeCurrency(cmd.Currency)
	cmd.PriceGroupName = strings.TrimSpace(cmd.PriceGroupName)
	cmd.InventoryUnit = strings.TrimSpace(cmd.InventoryUnit)
	cmd.Status = normalizeProductPriceStatus(cmd.Status)
	cmd.Remark = strings.TrimSpace(cmd.Remark)
	if cmd.ID < 0 || cmd.ProductID < 0 || cmd.CustomerProductAliasID < 0 || cmd.PriceGroupID < 0 {
		return SaveProductPriceRecordCommand{}, ValidationError{Message: "invalid product price record"}
	}
	if cmd.ProductID <= 0 && cmd.CustomerProductAliasID <= 0 {
		return SaveProductPriceRecordCommand{}, ValidationError{Message: "product or customer product required"}
	}
	if cmd.FinalUnitPrice <= 0 {
		return SaveProductPriceRecordCommand{}, ValidationError{Message: "final_unit_price must be > 0"}
	}
	if cmd.PriceUnit == "" {
		return SaveProductPriceRecordCommand{}, ValidationError{Message: "price_unit required"}
	}
	if cmd.InventoryUnit == "" {
		cmd.InventoryUnit = "kg"
	}
	conversionJSON, err := normalizeJSONObjectText(cmd.InventoryConversionJSON)
	if err != nil {
		return SaveProductPriceRecordCommand{}, ValidationError{Message: "invalid inventory_conversion_json"}
	}
	cmd.InventoryConversionJSON = conversionJSON
	return cmd, nil
}

func normalizeCurrency(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return "CNY"
	}
	return value
}

func normalizeProductPriceStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "published", "active", "启用", "已发布":
		return "published"
	case "inactive", "disabled", "停用":
		return "inactive"
	case "draft", "草稿", "":
		return "draft"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func normalizeGradientTemplateCommand(cmd SaveGradientTemplateCommand) (SaveGradientTemplateCommand, error) {
	cmd.Name = strings.TrimSpace(cmd.Name)
	if cmd.Name == "" {
		return cmd, fmt.Errorf("name required")
	}
	cmd.DisplayUnit = normalizeGradientDisplayUnit(cmd.DisplayUnit)
	if len(cmd.Tiers) == 0 {
		return cmd, fmt.Errorf("tiers required")
	}
	normalized := make([]GradientTemplateTier, 0, len(cmd.Tiers))
	for i, tier := range cmd.Tiers {
		tier.Label = strings.TrimSpace(tier.Label)
		if tier.Label == "" {
			return cmd, fmt.Errorf("tier label required")
		}
		tier = normalizeGradientTemplateTierWeights(cmd.DisplayUnit, tier)
		if tier.MinWeightG <= 0 {
			return cmd, fmt.Errorf("tier min_weight_g must be > 0")
		}
		if tier.MaxWeightG != nil && *tier.MaxWeightG <= tier.MinWeightG {
			return cmd, fmt.Errorf("tier max_weight_g must be greater than min_weight_g")
		}
		if tier.MarginRate < 0 {
			return cmd, fmt.Errorf("tier margin_rate must be >= 0")
		}
		if tier.Position <= 0 {
			tier.Position = i + 1
		}
		normalized = append(normalized, tier)
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		if normalized[i].Position != normalized[j].Position {
			return normalized[i].Position < normalized[j].Position
		}
		return normalized[i].MinWeightG < normalized[j].MinWeightG
	})
	cmd.Tiers = normalized
	return cmd, nil
}

func normalizeProductConfigTemplateCommand(cmd SaveProductConfigTemplateCommand) (SaveProductConfigTemplateCommand, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	cmd.Name = strings.TrimSpace(cmd.Name)
	if cmd.ID < 0 {
		return SaveProductConfigTemplateCommand{}, ValidationError{Message: "invalid id"}
	}
	if cmd.CustomerID < 0 {
		return SaveProductConfigTemplateCommand{}, ValidationError{Message: "invalid customer_id"}
	}
	if cmd.Name == "" {
		return SaveProductConfigTemplateCommand{}, ValidationError{Message: "name required"}
	}
	if cmd.GradientTemplateID < 0 || cmd.OperationTemplateID < 0 || cmd.UnitTemplateID < 0 {
		return SaveProductConfigTemplateCommand{}, ValidationError{Message: "invalid template id"}
	}
	priceRuleJSON, err := normalizeJSONText(cmd.PriceListRuleJSON)
	if err != nil {
		return SaveProductConfigTemplateCommand{}, ValidationError{Message: "invalid price_list_rule_json"}
	}
	if err := validateProductConfigPriceRule(priceRuleJSON); err != nil {
		return SaveProductConfigTemplateCommand{}, err
	}
	specialAttrsSchemaJSON, err := normalizeJSONArrayText(cmd.SpecialAttrsSchemaJSON)
	if err != nil {
		return SaveProductConfigTemplateCommand{}, ValidationError{Message: "invalid special_attrs_schema_json"}
	}
	unitRule := catalogdomain.NormalizeProductUnitRule(catalogdomain.ProductUnitRule{
		InventoryUnit:  cmd.InventoryUnit,
		QuoteUnit:      cmd.QuoteUnit,
		OrderUnit:      cmd.OrderUnit,
		ConversionJSON: cmd.UnitConversionJSON,
		IntegerUnit:    cmd.IntegerUnit,
	})
	unitConversionJSON, err := normalizeJSONText(unitRule.ConversionJSON)
	if err != nil {
		return SaveProductConfigTemplateCommand{}, ValidationError{Message: "invalid unit_conversion_json"}
	}
	cmd.PriceListRuleJSON = priceRuleJSON
	cmd.SpecialAttrsSchemaJSON = specialAttrsSchemaJSON
	cmd.InventoryUnit = unitRule.InventoryUnit
	cmd.QuoteUnit = unitRule.QuoteUnit
	cmd.OrderUnit = unitRule.OrderUnit
	cmd.UnitConversionJSON = unitConversionJSON
	cmd.IntegerUnit = unitRule.IntegerUnit
	return cmd, nil
}

func normalizeProductUnitDefinitionCommand(cmd SaveProductUnitDefinitionCommand) (SaveProductUnitDefinitionCommand, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	cmd.Code = strings.TrimSpace(cmd.Code)
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.UnitType = strings.TrimSpace(cmd.UnitType)
	if cmd.Code == "" {
		return SaveProductUnitDefinitionCommand{}, ValidationError{Message: "unit code required"}
	}
	if cmd.Name == "" {
		cmd.Name = cmd.Code
	}
	if cmd.UnitType == "" {
		cmd.UnitType = "other"
	}
	return cmd, nil
}

func normalizeProductUnitTemplateCommand(cmd SaveProductUnitTemplateCommand) (SaveProductUnitTemplateCommand, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	cmd.Name = strings.TrimSpace(cmd.Name)
	if cmd.ID < 0 {
		return SaveProductUnitTemplateCommand{}, ValidationError{Message: "invalid id"}
	}
	if cmd.Name == "" {
		return SaveProductUnitTemplateCommand{}, ValidationError{Message: "name required"}
	}
	if len(cmd.SalesSpecs) > 0 {
		specs, err := normalizeProductSalesSpecs(cmd.SalesSpecs)
		if err != nil {
			return SaveProductUnitTemplateCommand{}, err
		}
		defaultSalesUnit := defaultSalesUnitFromSpecs(specs)
		unitConversionJSON, err := normalizeJSONObjectText(cmd.UnitConversionJSON)
		if err != nil {
			return SaveProductUnitTemplateCommand{}, ValidationError{Message: "invalid unit_conversion_json"}
		}
		if strings.TrimSpace(unitConversionJSON) == "" {
			unitConversionJSON = "{}"
		}
		cmd.InventoryUnit = normalizeUnitTemplateUnit(cmd.InventoryUnit, "kg")
		cmd.DefaultSalesUnit = defaultSalesUnit
		cmd.SalesUnit = defaultSalesUnit
		cmd.SalesUnits = salesUnitsFromSpecs(specs)
		cmd.SalesSpecs = specs
		cmd.QuoteUnit = defaultSalesUnit
		cmd.OrderUnit = defaultSalesUnit
		cmd.UnitConversionJSON = unitConversionJSON
		return cmd, nil
	}
	inventoryUnit := normalizeUnitTemplateUnit(cmd.InventoryUnit, "kg")
	defaultSalesUnit := normalizeUnitTemplateUnit(firstNonEmptyUnitTemplateUnit(cmd.DefaultSalesUnit, cmd.SalesUnit, cmd.OrderUnit, cmd.QuoteUnit), inventoryUnit)
	unitConversionJSON, salesUnits, err := normalizeUnitTemplateConversionJSON(cmd.UnitConversionJSON, inventoryUnit, defaultSalesUnit, cmd.SalesUnits, cmd.SalesUnit, cmd.OrderUnit, cmd.QuoteUnit)
	if err != nil {
		return SaveProductUnitTemplateCommand{}, err
	}
	cmd.InventoryUnit = inventoryUnit
	cmd.DefaultSalesUnit = defaultSalesUnit
	cmd.SalesUnit = defaultSalesUnit
	cmd.SalesUnits = salesUnits
	cmd.QuoteUnit = defaultSalesUnit
	cmd.OrderUnit = defaultSalesUnit
	cmd.UnitConversionJSON = unitConversionJSON
	return cmd, nil
}

func decorateProductUnitTemplates(rows []ProductUnitTemplate) []ProductUnitTemplate {
	if len(rows) == 0 {
		return rows
	}
	out := make([]ProductUnitTemplate, 0, len(rows))
	for _, row := range rows {
		out = append(out, decorateProductUnitTemplate(row))
	}
	return out
}

func decorateProductUnitTemplate(row ProductUnitTemplate) ProductUnitTemplate {
	if len(row.SalesSpecs) > 0 {
		specs, err := normalizeProductSalesSpecs(row.SalesSpecs)
		if err == nil {
			row.SalesSpecs = specs
			row.InventoryUnit = normalizeUnitTemplateUnit(row.InventoryUnit, "kg")
			row.DefaultSalesUnit = defaultSalesUnitFromSpecs(specs)
			row.SalesUnit = row.DefaultSalesUnit
			row.SalesUnits = salesUnitsFromSpecs(specs)
			row.QuoteUnit = row.DefaultSalesUnit
			row.OrderUnit = row.DefaultSalesUnit
			if strings.TrimSpace(row.UnitConversionJSON) == "" {
				row.UnitConversionJSON = "{}"
			}
			return row
		}
	}
	inventoryUnit := normalizeUnitTemplateUnit(row.InventoryUnit, "kg")
	defaultSalesUnit := normalizeUnitTemplateUnit(firstNonEmptyUnitTemplateUnit(row.DefaultSalesUnit, row.OrderUnit, row.QuoteUnit, row.SalesUnit), inventoryUnit)
	unitConversionJSON, salesUnits, err := normalizeUnitTemplateConversionJSON(row.UnitConversionJSON, inventoryUnit, defaultSalesUnit, row.SalesUnits, row.SalesUnit, row.OrderUnit, row.QuoteUnit)
	if err == nil {
		row.UnitConversionJSON = unitConversionJSON
		row.SalesUnits = salesUnits
	} else {
		row.SalesUnits = uniqueUnitTemplateUnits(append([]string{inventoryUnit, defaultSalesUnit}, row.SalesUnits...))
		if strings.TrimSpace(row.UnitConversionJSON) == "" {
			row.UnitConversionJSON = "{}"
		}
	}
	row.InventoryUnit = inventoryUnit
	row.DefaultSalesUnit = defaultSalesUnit
	row.SalesUnit = defaultSalesUnit
	row.QuoteUnit = defaultSalesUnit
	row.OrderUnit = defaultSalesUnit
	return row
}

func normalizeProductSalesSpecs(rows []ProductSalesSpec) ([]ProductSalesSpec, error) {
	out := make([]ProductSalesSpec, 0, len(rows))
	seen := map[string]int{}
	defaultIdx := -1
	activeCount := 0
	for _, row := range rows {
		spec := ProductSalesSpec{
			SpecKey:           strings.TrimSpace(row.SpecKey),
			SpecName:          strings.TrimSpace(row.SpecName),
			SalesUnit:         strings.TrimSpace(row.SalesUnit),
			NetContentQty:     row.NetContentQty,
			NetContentUnit:    strings.TrimSpace(row.NetContentUnit),
			Default:           row.Default,
			Active:            row.Active,
			DerivedSKUID:      row.DerivedSKUID,
			DerivedSKUCode:    strings.TrimSpace(row.DerivedSKUCode),
			DerivedSpecStatus: strings.TrimSpace(row.DerivedSpecStatus),
		}
		if spec.SpecName == "" {
			return nil, ValidationError{Message: "spec_name required"}
		}
		spec.SalesUnit = spec.SpecName
		if spec.NetContentQty < 0 || math.IsNaN(spec.NetContentQty) || math.IsInf(spec.NetContentQty, 0) {
			return nil, ValidationError{Message: "invalid net_content_qty"}
		}
		if spec.SpecKey == "" {
			spec.SpecKey = generatedSalesSpecKey(spec.SpecName, spec.SalesUnit, spec.NetContentQty, spec.NetContentUnit)
		}
		baseKey := spec.SpecKey
		if baseKey == "" {
			baseKey = fmt.Sprintf("spec-%d", len(out)+1)
		}
		if seen[baseKey] > 0 {
			seen[baseKey]++
			spec.SpecKey = fmt.Sprintf("%s-%d", baseKey, seen[baseKey])
		} else {
			seen[baseKey] = 1
			spec.SpecKey = baseKey
		}
		if spec.Active && spec.DerivedSpecStatus == "" {
			spec.DerivedSpecStatus = "active"
		}
		if spec.Active {
			activeCount++
		}
		if spec.Default && !spec.Active {
			return nil, ValidationError{Message: "default sales spec must be active"}
		}
		if spec.Default && defaultIdx < 0 {
			defaultIdx = len(out)
		} else {
			spec.Default = false
		}
		out = append(out, spec)
	}
	if len(out) == 0 {
		return nil, ValidationError{Message: "sales_specs required"}
	}
	if activeCount == 0 {
		return nil, ValidationError{Message: "active sales_specs required"}
	}
	if defaultIdx < 0 {
		for i := range out {
			if out[i].Active {
				out[i].Default = true
				defaultIdx = i
				break
			}
		}
	}
	return out, nil
}

func generatedSalesSpecKey(specName, salesUnit string, netContentQty float64, netContentUnit string) string {
	sum := sha1.Sum([]byte(fmt.Sprintf("%s|%s|%.6f|%s", strings.TrimSpace(specName), strings.TrimSpace(salesUnit), netContentQty, strings.TrimSpace(netContentUnit))))
	key := fmt.Sprintf("%x", sum)
	if len(key) > 12 {
		key = key[:12]
	}
	return "spec-" + key
}

func defaultSalesUnitFromSpecs(specs []ProductSalesSpec) string {
	for _, spec := range specs {
		if spec.Default && strings.TrimSpace(spec.SalesUnit) != "" {
			return strings.TrimSpace(spec.SalesUnit)
		}
	}
	for _, spec := range specs {
		if spec.Active && strings.TrimSpace(spec.SalesUnit) != "" {
			return strings.TrimSpace(spec.SalesUnit)
		}
	}
	if len(specs) > 0 && strings.TrimSpace(specs[0].SalesUnit) != "" {
		return strings.TrimSpace(specs[0].SalesUnit)
	}
	return "kg"
}

func salesUnitsFromSpecs(specs []ProductSalesSpec) []string {
	units := make([]string, 0, len(specs))
	for _, spec := range specs {
		units = append(units, spec.SalesUnit)
	}
	return uniqueUnitTemplateUnits(units)
}

func normalizeUnitTemplateConversionJSON(raw string, inventoryUnit string, defaultSalesUnit string, explicitSalesUnits []string, legacyUnits ...any) (string, []string, error) {
	inventoryUnit = normalizeUnitTemplateUnit(inventoryUnit, "kg")
	defaultSalesUnit = normalizeUnitTemplateUnit(defaultSalesUnit, inventoryUnit)
	graph, conversionKeys, err := parseUnitTemplateConversionGraph(raw, inventoryUnit)
	if err != nil {
		return "", nil, ValidationError{Message: "invalid unit_conversion_json"}
	}
	direct := map[string]map[string]float64{
		inventoryUnit: {inventoryUnit: 1},
	}
	orderedUnits := []string{inventoryUnit, defaultSalesUnit}
	orderedUnits = append(orderedUnits, explicitSalesUnits...)
	for _, legacy := range legacyUnits {
		switch value := legacy.(type) {
		case string:
			orderedUnits = append(orderedUnits, value)
		case []string:
			orderedUnits = append(orderedUnits, value...)
		}
	}
	orderedUnits = append(orderedUnits, conversionKeys...)
	orderedUnits = uniqueUnitTemplateUnits(orderedUnits)
	for _, unit := range orderedUnits {
		if unit == "" {
			continue
		}
		if unit == inventoryUnit {
			direct[unit] = map[string]float64{inventoryUnit: 1}
			continue
		}
		factor, ok := resolveUnitTemplateConversionToInventory(unit, inventoryUnit, graph, map[string]bool{})
		if !ok || factor <= 0 {
			if unit == defaultSalesUnit {
				return "", nil, ValidationError{Message: "default sales unit conversion required"}
			}
			return "", nil, ValidationError{Message: "sales unit conversion required"}
		}
		direct[unit] = map[string]float64{inventoryUnit: roundUnitTemplateFactor(factor)}
	}
	encoded, err := json.Marshal(direct)
	if err != nil {
		return "", nil, err
	}
	return string(encoded), orderedUnits, nil
}

func parseUnitTemplateConversionGraph(raw string, inventoryUnit string) (map[string]map[string]float64, []string, error) {
	graph := map[string]map[string]float64{}
	keys := []string{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "{}"
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, nil, err
	}
	for fromUnit, rawTargets := range parsed {
		fromUnit = strings.TrimSpace(fromUnit)
		if fromUnit == "" {
			continue
		}
		keys = append(keys, fromUnit)
		if graph[fromUnit] == nil {
			graph[fromUnit] = map[string]float64{}
		}
		if factor := unitTemplatePositiveFloat(rawTargets); factor > 0 {
			graph[fromUnit][inventoryUnit] = roundUnitTemplateFactor(factor)
			continue
		}
		targets, ok := rawTargets.(map[string]any)
		if !ok {
			continue
		}
		for toUnit, rawFactor := range targets {
			toUnit = strings.TrimSpace(toUnit)
			factor := unitTemplatePositiveFloat(rawFactor)
			if toUnit == "" || factor <= 0 {
				continue
			}
			graph[fromUnit][toUnit] = roundUnitTemplateFactor(factor)
		}
	}
	return graph, keys, nil
}

func resolveUnitTemplateConversionToInventory(unit string, inventoryUnit string, graph map[string]map[string]float64, seen map[string]bool) (float64, bool) {
	unit = strings.TrimSpace(unit)
	if unit == "" {
		return 0, false
	}
	if unit == inventoryUnit {
		return 1, true
	}
	if seen[unit] {
		return 0, false
	}
	seen[unit] = true
	targets := graph[unit]
	if factor := targets[inventoryUnit]; factor > 0 {
		return factor, true
	}
	for targetUnit, factor := range targets {
		if factor <= 0 {
			continue
		}
		targetFactor, ok := resolveUnitTemplateConversionToInventory(targetUnit, inventoryUnit, graph, seen)
		if ok && targetFactor > 0 {
			return roundUnitTemplateFactor(factor * targetFactor), true
		}
	}
	return 0, false
}

func roundUnitTemplateFactor(value float64) float64 {
	if value <= 0 {
		return 0
	}
	return math.Round(value*1e12) / 1e12
}

func uniqueUnitTemplateUnits(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		unit := strings.TrimSpace(value)
		if unit == "" || seen[unit] {
			continue
		}
		seen[unit] = true
		out = append(out, unit)
	}
	return out
}

func firstNonEmptyUnitTemplateUnit(values ...string) string {
	for _, value := range values {
		if unit := strings.TrimSpace(value); unit != "" {
			return unit
		}
	}
	return ""
}

func normalizeUnitTemplateUnit(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	fallback = strings.TrimSpace(fallback)
	if fallback != "" {
		return fallback
	}
	return "kg"
}

func unitTemplatePositiveFloat(value any) float64 {
	switch v := value.(type) {
	case float64:
		if v > 0 {
			return v
		}
	case float32:
		if v > 0 {
			return float64(v)
		}
	case int:
		if v > 0 {
			return float64(v)
		}
	case int64:
		if v > 0 {
			return float64(v)
		}
	case json.Number:
		parsed, err := v.Float64()
		if err == nil && parsed > 0 {
			return parsed
		}
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err == nil && parsed > 0 {
			return parsed
		}
	}
	return 0
}

func normalizeDeleteProductUnitDefinitionCommand(cmd DeleteProductUnitDefinitionCommand) (DeleteProductUnitDefinitionCommand, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	cmd.Code = strings.TrimSpace(cmd.Code)
	if cmd.Code == "" {
		return DeleteProductUnitDefinitionCommand{}, ValidationError{Message: "unit code required"}
	}
	return cmd, nil
}

func normalizeDeleteProductUnitTemplateCommand(cmd DeleteProductUnitTemplateCommand) (DeleteProductUnitTemplateCommand, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.ID <= 0 {
		return DeleteProductUnitTemplateCommand{}, ValidationError{Message: "invalid id"}
	}
	return cmd, nil
}

func normalizeGradientDisplayUnit(unit string) string {
	trimmed := strings.TrimSpace(unit)
	switch trimmed {
	case "kg":
		return "kg"
	case "g227":
		return "g227"
	case "g100":
		return "g100"
	case "g250":
		return "g250"
	case "lb":
		return "lb"
	default:
		if trimmed != "" {
			return trimmed
		}
		return "lb"
	}
}

func normalizeJSONText(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "{}"
	}
	if !json.Valid([]byte(raw)) {
		return "", fmt.Errorf("invalid json")
	}
	return raw, nil
}

func normalizeJSONObjectText(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "{}"
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return "", err
	}
	encoded, err := json.Marshal(parsed)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func normalizeJSONArrayText(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "[]"
	}
	var parsed []any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return "", err
	}
	encoded, err := json.Marshal(parsed)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func normalizePricingRuleCalculationJSON(raw map[string]any) (map[string]any, error) {
	if pricingRuleCalculationContainsQuantityTierField(raw) {
		return nil, fmt.Errorf("pricing rule must not contain quantity tiers")
	}
	profitMethod := pricingRuleProfitMethod(raw)
	switch profitMethod {
	case "", "gross_margin", "markup":
	default:
		return nil, fmt.Errorf("only markup rate is supported")
	}
	normalized, err := clonePricingRuleCalculationJSON(raw)
	if err != nil {
		return nil, err
	}
	normalized["profit_method"] = "markup"
	stripPricingRuleRemovedCostFields(normalized)
	otherCosts, err := normalizePricingRuleOtherCosts(normalized["other_costs"])
	if err != nil {
		return nil, err
	}
	if otherCosts == nil {
		otherCosts, err = normalizePricingRuleOtherCosts(normalized["otherCosts"])
		if err != nil {
			return nil, err
		}
	}
	delete(normalized, "otherCosts")
	if otherCosts != nil {
		normalized["other_costs"] = otherCosts
	}
	return normalized, nil
}

func clonePricingRuleCalculationJSON(raw map[string]any) (map[string]any, error) {
	if raw == nil {
		return map[string]any{}, nil
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid calculation_json")
	}
	var normalized map[string]any
	if err := json.Unmarshal(encoded, &normalized); err != nil {
		return nil, fmt.Errorf("invalid calculation_json")
	}
	if normalized == nil {
		normalized = map[string]any{}
	}
	return normalized, nil
}

func pricingRuleProfitMethod(calculationJSON map[string]any) string {
	if calculationJSON == nil {
		return ""
	}
	value, ok := calculationJSON["profit_method"]
	if !ok || value == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(fmt.Sprint(value)))
}

func pricingRuleCalculationHasLegacyQuarantine(calculationJSON map[string]any) bool {
	for _, key := range []string{"legacy_profit_method", "migration_warning"} {
		value := strings.TrimSpace(fmt.Sprint(calculationJSON[key]))
		if value != "" && value != "<nil>" {
			return true
		}
	}
	return false
}

func pricingRuleCalculationNeedsQuarantine(calculationJSON map[string]any) bool {
	if pricingRuleCalculationHasLegacyQuarantine(calculationJSON) {
		return true
	}
	switch pricingRuleProfitMethod(calculationJSON) {
	case "", "gross_margin", "markup":
		return false
	default:
		return true
	}
}

func normalizeLegacyPricingRuleMarkupRate(rate float64, profitMethod string) float64 {
	if (profitMethod == "" || profitMethod == "gross_margin") && rate > 1 {
		return rate / 100
	}
	return rate
}

func normalizePricingRuleCostSourceMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "product_cost_context", "bom_current_cost", "inventory_cost", "manual_cost", "last_purchase_cost":
		return "bom_current_cost"
	default:
		return "bom_current_cost"
	}
}

func stripPricingRuleRemovedCostFields(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			switch strings.ToLower(strings.TrimSpace(key)) {
			case "cost_components", "costcomponents":
				delete(typed, key)
				continue
			}
			stripPricingRuleRemovedCostFields(child)
		}
	case []any:
		for _, child := range typed {
			stripPricingRuleRemovedCostFields(child)
		}
	}
}

func normalizePricingRuleOtherCosts(value any) (map[string]any, error) {
	if value == nil {
		return nil, nil
	}
	typed, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("other costs must be key-value pairs")
	}
	out := map[string]any{}
	for rawKey, rawValue := range typed {
		key := strings.TrimSpace(rawKey)
		if key == "" {
			continue
		}
		amount, ok := pricingRuleCostAmount(rawValue)
		if !ok {
			return nil, fmt.Errorf("other cost must be numeric")
		}
		if amount < 0 {
			return nil, fmt.Errorf("other cost must not be negative")
		}
		out[key] = amount
	}
	if len(out) == 0 {
		return map[string]any{}, nil
	}
	return out, nil
}

func pricingRuleCostAmount(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case json.Number:
		amount, err := typed.Float64()
		return amount, err == nil
	case string:
		amount, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return amount, err == nil
	default:
		return 0, false
	}
}

func pricingRuleCalculationContainsQuantityTierField(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			switch strings.ToLower(strings.TrimSpace(key)) {
			case "min_qty", "minqty", "max_qty", "maxqty", "tier_label", "tierlabel", "tier_name", "tiername", "tiers", "quantity_unit", "quantityunit", "position", "final_unit_price", "finalunitprice", "customer_tiers", "customertiers":
				return true
			}
			if pricingRuleCalculationContainsQuantityTierField(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if pricingRuleCalculationContainsQuantityTierField(child) {
				return true
			}
		}
	}
	return false
}

func validateProductConfigPriceRule(raw string) error {
	var rule map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &rule); err != nil {
		return ValidationError{Message: "invalid price_list_rule_json"}
	}
	mode := strings.TrimSpace(fmt.Sprint(rule["pricing_mode"]))
	switch mode {
	case "fixed_unit_price":
		if numberFromJSON(rule["fixed_unit_price"], rule["unit_price"], rule["price_per_unit"], rule["fixed_price"]) <= 0 {
			return ValidationError{Message: "fixed_unit_price required"}
		}
	case "cost_plus":
		if !hasJSONNumber(rule, "cost_plus_rate", "markup_rate", "margin_rate") {
			return ValidationError{Message: "cost_plus_rate required"}
		}
	}
	return nil
}

func hasJSONNumber(rule map[string]any, keys ...string) bool {
	for _, key := range keys {
		if _, ok := jsonNumber(rule[key]); ok {
			return true
		}
	}
	return false
}

func numberFromJSON(values ...any) float64 {
	for _, value := range values {
		if n, ok := jsonNumber(value); ok {
			return n
		}
	}
	return 0
}

func jsonNumber(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		n, err := v.Float64()
		return n, err == nil
	case string:
		var n float64
		if _, err := fmt.Sscanf(strings.TrimSpace(v), "%f", &n); err == nil {
			return n, true
		}
	}
	return 0, false
}

func normalizeGradientTemplateTierWeights(displayUnit string, tier GradientTemplateTier) GradientTemplateTier {
	specG := gradientDisplayUnitSpecG(displayUnit)
	if tier.MinDisplayQty != nil {
		tier.MinWeightG = roundTemplateWeightG(*tier.MinDisplayQty * float64(specG))
	}
	if tier.MaxDisplayQty != nil {
		v := roundTemplateWeightG(*tier.MaxDisplayQty * float64(specG))
		tier.MaxWeightG = &v
	}
	return tier
}

func gradientDisplayUnitSpecG(unit string) int {
	switch normalizeGradientDisplayUnit(unit) {
	case "kg":
		return 1000
	case "g227":
		return 227
	case "g100":
		return 100
	case "g250":
		return 250
	default:
		if normalizeGradientDisplayUnit(unit) != "lb" {
			return 1
		}
		return 454
	}
}

func roundTemplateWeightG(value float64) float64 {
	return math.Round(value*1000) / 1000
}

func BuildProductSettings(categories []ProductCategory, products []Product) ProductSettingsData {
	roots := make([]ProductCategory, 0)
	children := map[int64][]ProductCategory{}
	for _, category := range categories {
		if category.ParentID == 0 {
			roots = append(roots, category)
			continue
		}
		children[category.ParentID] = append(children[category.ParentID], category)
	}
	sortCategories(roots)
	for parentID := range children {
		sortCategories(children[parentID])
	}

	productsByCategory := map[int64][]ProductSettingsProduct{}
	allProducts := make([]ProductSettingsProduct, 0, len(products))
	for _, product := range products {
		row := productSettingsProduct(product)
		allProducts = append(allProducts, row)
		productsByCategory[product.ProductCategoryID] = append(productsByCategory[product.ProductCategoryID], row)
	}
	for categoryID := range productsByCategory {
		sortProducts(productsByCategory[categoryID])
		for i := range productsByCategory[categoryID] {
			productsByCategory[categoryID][i].Number = i + 1
		}
	}
	sortProducts(allProducts)

	out := ProductSettingsData{Products: allProducts, Categories: make([]ProductCategoryNode, 0, len(roots))}
	for i, root := range roots {
		root.Number = i + 1
		node := ProductCategoryNode{
			ProductCategory: root,
			Children:        make([]ProductCategoryNode, 0),
			Products:        productsByCategory[root.ID],
		}
		if node.Products == nil {
			node.Products = make([]ProductSettingsProduct, 0)
		}
		for j, child := range children[root.ID] {
			child.Number = j + 1
			childProducts := productsByCategory[child.ID]
			if childProducts == nil {
				childProducts = make([]ProductSettingsProduct, 0)
			}
			childNode := ProductCategoryNode{
				ProductCategory: child,
				Children:        make([]ProductCategoryNode, 0),
				Products:        childProducts,
			}
			node.Children = append(node.Children, childNode)
		}
		out.Categories = append(out.Categories, node)
	}
	return out
}

func productSettingsProduct(p Product) ProductSettingsProduct {
	productKind, dripBagGrams, dripBoxBagCount, salesUnits, _ := normalizeProductKindSettings(p.ProductKind, p.DripBagGrams, p.DripBoxBagCount)
	inventoryUnit, integerInventoryUnit := productInventoryUnitFields(p)
	defaultSalesUnit, unitConversionJSON, salesUnitRulesJSON := productSalesUnitRuleFields(p, inventoryUnit)
	if !catalogdomain.ProductKindSupportsBomParams(productKind) {
		p.RoastLevel = ""
		p.YieldRate = 0
	}
	return ProductSettingsProduct{
		ID:                          p.ID,
		SKUID:                       p.SKUID,
		ParentProductID:             p.ParentProductID,
		EffectiveParentProductID:    p.EffectiveParentProductID,
		SKUName:                     p.SKUName,
		SKUCode:                     p.SKUCode,
		Barcode:                     p.Barcode,
		SpecLabel:                   p.SpecLabel,
		NetContentQty:               p.NetContentQty,
		NetContentUnit:              p.NetContentUnit,
		IsDefaultSKU:                p.IsDefaultSKU,
		AutoDerivedSKU:              p.AutoDerivedSKU,
		DerivedUnitTemplateID:       p.DerivedUnitTemplateID,
		DerivedSpecKey:              p.DerivedSpecKey,
		DerivedSpecName:             p.DerivedSpecName,
		DerivedSalesUnit:            p.DerivedSalesUnit,
		DerivedSpecStatus:           p.DerivedSpecStatus,
		Name:                        p.Name,
		ProductCode:                 productCodeForID(p.ID),
		Remark:                      p.Remark,
		GreenBeanType:               p.GreenBeanType,
		GreenBeanBomProductID:       p.GreenBeanBomProductID,
		RoastLevel:                  p.RoastLevel,
		SpecialAttrsJSON:            productJSONOrDefault(p.SpecialAttrsJSON),
		ProductKind:                 productKind,
		DripBagGrams:                dripBagGrams,
		DripBoxBagCount:             dripBoxBagCount,
		AllowFulfillmentOrder:       p.AllowFulfillmentOrder,
		AllowMallOrder:              p.AllowMallOrder,
		SalesUnits:                  salesUnits,
		DefaultPrice:                p.DefaultPrice,
		RetailPrice100G:             p.RetailPrice100G,
		RetailPrice200G:             p.RetailPrice200G,
		RetailPrice227G:             p.RetailPrice227G,
		RetailPrice250G:             p.RetailPrice250G,
		YieldRate:                   p.YieldRate,
		ExpectedLossRate:            p.ExpectedLossRate,
		ProcessRouteID:              p.ProcessRouteID,
		ProductionConfigNote:        p.ProductionConfigNote,
		ProductCategoryID:           p.ProductCategoryID,
		ProductCategoryPosition:     p.ProductCategoryPosition,
		ClassificationTemplateID:    p.ClassificationTemplateID,
		CustomerID:                  p.CustomerID,
		BaseProductID:               p.BaseProductID,
		Visibility:                  productVisibility(p.Visibility, p.CustomerID),
		CustomType:                  p.CustomType,
		MarginRateOverride:          p.MarginRateOverride,
		GradientTemplateIDOverride:  p.GradientTemplateIDOverride,
		OperationTemplateIDOverride: p.OperationTemplateIDOverride,
		UnitRuleOverrideJSON:        productJSONOrDefault(p.UnitRuleOverrideJSON),
		InventoryUnit:               inventoryUnit,
		IntegerInventoryUnit:        integerInventoryUnit,
		DefaultSalesUnit:            defaultSalesUnit,
		UnitConversionJSON:          unitConversionJSON,
		SalesUnitRulesJSON:          salesUnitRulesJSON,
		UnitTemplateID:              p.UnitTemplateID,
		UnitTemplateName:            p.UnitTemplateName,
		UnitRuleSource:              productUnitRuleSource(p),
		ProductConfigTemplateID:     p.ProductConfigTemplateID,
		Active:                      p.Active,
		BomItemCount:                p.BomItemCount,
		BomStatus:                   productBomStatus(p.BomStatus, p.BomItemCount),
		BomSourceType:               p.BomSourceType,
		EffectiveProductID:          p.EffectiveProductID,
		EffectiveBomVersionID:       p.EffectiveBomVersionID,
		SourceProductID:             p.SourceProductID,
		SourceProductCode:           p.SourceProductCode,
		SourceProductName:           p.SourceProductName,
		SourceBomVersionID:          p.SourceBomVersionID,
		SourceBomVersionNo:          p.SourceBomVersionNo,
		DerivedFromLabel:            p.DerivedFromLabel,
		CanEditBOM:                  p.CanEditBOM,
		ProductionBomID:             p.ProductionBomID,
		ProductionBomCode:           p.ProductionBomCode,
		ProductionBomName:           p.ProductionBomName,
		ProductionBomVersionID:      p.ProductionBomVersionID,
		ProductionBomVersionNo:      p.ProductionBomVersionNo,
		LatestBomVersionID:          p.LatestBomVersionID,
		LatestBomVersionNo:          p.LatestBomVersionNo,
		IsLatestBomVersion:          p.IsLatestBomVersion,
		ProductionBomGroupID:        p.ProductionBomGroupID,
		ProductionBomGroupName:      p.ProductionBomGroupName,
		GroupID:                     p.GroupID,
		GroupName:                   p.GroupName,
		GroupItemID:                 p.GroupItemID,
		GroupItemName:               p.GroupItemName,
		ParentGroupItemID:           p.ParentGroupItemID,
		ParentGroupItemName:         p.ParentGroupItemName,
		GroupSource:                 p.GroupSource,
		OrderUsageCount:             p.OrderUsageCount,
		PriceSummary:                p.PriceSummary,
	}
}

func productSalesUnitRuleFields(p Product, inventoryUnit string) (string, string, string) {
	rule := map[string]any{}
	if raw := strings.TrimSpace(p.UnitRuleOverrideJSON); raw != "" {
		_ = json.Unmarshal([]byte(raw), &rule)
	}
	defaultSalesUnit := strings.TrimSpace(p.DefaultSalesUnit)
	if p.AutoDerivedSKU {
		if derivedSalesUnit := strings.TrimSpace(p.DerivedSalesUnit); derivedSalesUnit != "" {
			defaultSalesUnit = derivedSalesUnit
		}
	}
	for _, key := range []string{"default_sales_unit", "quote_unit", "order_unit"} {
		if defaultSalesUnit != "" {
			break
		}
		if value, ok := rule[key].(string); ok {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				defaultSalesUnit = trimmed
				break
			}
		}
	}
	if defaultSalesUnit == "" {
		defaultSalesUnit = strings.TrimSpace(inventoryUnit)
	}
	if defaultSalesUnit == "" {
		defaultSalesUnit = "kg"
	}
	conversion := map[string]any{}
	if raw := strings.TrimSpace(p.UnitConversionJSON); raw != "" {
		conversion = jsonObjectFromAny(raw)
	} else if value, ok := rule["unit_conversion_json"]; ok {
		conversion = jsonObjectFromAny(value)
	} else if value, ok := rule["conversion_json"]; ok {
		conversion = jsonObjectFromAny(value)
	}
	if len(conversion) == 0 && p.AutoDerivedSKU {
		conversion = salesSpecUnitConversionJSON(p, inventoryUnit)
	}
	if len(conversion) == 0 && defaultSalesUnit == inventoryUnit {
		conversion = map[string]any{defaultSalesUnit: map[string]any{inventoryUnit: float64(1)}}
	}
	salesRules := map[string]any{}
	if raw := strings.TrimSpace(p.SalesUnitRulesJSON); raw != "" {
		salesRules = jsonObjectFromAny(raw)
	} else if value, ok := rule["sales_unit_rules"]; ok {
		salesRules = jsonObjectFromAny(value)
	}
	return defaultSalesUnit, productJSONFromMap(conversion), productJSONFromMap(salesRules)
}

func salesSpecUnitConversionJSON(p Product, inventoryUnit string) map[string]any {
	salesUnit := strings.TrimSpace(p.DerivedSalesUnit)
	if salesUnit == "" {
		salesUnit = strings.TrimSpace(p.DefaultSalesUnit)
	}
	netContentUnit := strings.TrimSpace(p.NetContentUnit)
	targetUnit := strings.TrimSpace(inventoryUnit)
	if targetUnit == "" {
		targetUnit = strings.TrimSpace(p.InventoryUnit)
	}
	if targetUnit == "" {
		targetUnit = netContentUnit
	}
	if salesUnit == "" || netContentUnit == "" || targetUnit == "" || p.NetContentQty <= 0 || math.IsNaN(p.NetContentQty) || math.IsInf(p.NetContentQty, 0) {
		return map[string]any{}
	}
	quantity := p.NetContentQty
	if netContentUnit != targetUnit {
		sourceGram, sourceOK := salesSpecWeightUnitGrams(netContentUnit)
		targetGram, targetOK := salesSpecWeightUnitGrams(targetUnit)
		if !sourceOK || !targetOK || targetGram <= 0 {
			return map[string]any{}
		}
		quantity = (p.NetContentQty * sourceGram) / targetGram
	}
	return map[string]any{salesUnit: map[string]any{targetUnit: normalizeSalesSpecConversionQuantity(quantity)}}
}

func salesSpecWeightUnitGrams(unit string) (float64, bool) {
	switch strings.TrimSpace(unit) {
	case "g":
		return 1, true
	case "kg":
		return 1000, true
	case "lb", "lbs", "磅":
		return 453.59237, true
	default:
		return 0, false
	}
}

func normalizeSalesSpecConversionQuantity(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	const scale = 1_000_000_000
	return math.Round(value*scale) / scale
}

func productUnitRuleSource(p Product) string {
	if source := strings.TrimSpace(p.UnitRuleSource); source != "" {
		return source
	}
	if hasProductUnitRuleOverride(p.UnitRuleOverrideJSON) {
		return "product_override"
	}
	if p.UnitTemplateID > 0 {
		return "product_unit_template"
	}
	if strings.TrimSpace(p.InventoryUnit) != "" && strings.TrimSpace(p.InventoryUnit) != "kg" {
		return "legacy_template"
	}
	return "default"
}

func hasProductUnitRuleOverride(raw string) bool {
	rule := map[string]any{}
	if strings.TrimSpace(raw) == "" {
		return false
	}
	if err := json.Unmarshal([]byte(raw), &rule); err != nil {
		return false
	}
	for _, key := range []string{"inventory_unit", "integer_inventory_unit", "integer_unit", "default_sales_unit", "quote_unit", "order_unit", "unit_conversion_json", "conversion_json", "sales_unit_rules"} {
		if _, ok := rule[key]; ok {
			return true
		}
	}
	return false
}

func jsonObjectFromAny(value any) map[string]any {
	switch v := value.(type) {
	case map[string]any:
		if v != nil {
			return v
		}
	case string:
		var out map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(v)), &out); err == nil && out != nil {
			return out
		}
	}
	return map[string]any{}
}

func productJSONFromMap(value map[string]any) string {
	if len(value) == 0 {
		return "{}"
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func productInventoryUnitFields(p Product) (string, bool) {
	inventoryUnit := strings.TrimSpace(p.InventoryUnit)
	integerInventoryUnit := p.IntegerInventoryUnit
	rule := map[string]any{}
	if raw := strings.TrimSpace(p.UnitRuleOverrideJSON); raw != "" {
		_ = json.Unmarshal([]byte(raw), &rule)
	}
	if inventoryUnit == "" {
		if value, ok := rule["inventory_unit"].(string); ok {
			inventoryUnit = strings.TrimSpace(value)
		}
	}
	if !integerInventoryUnit {
		if value, ok := rule["integer_inventory_unit"]; ok {
			integerInventoryUnit = boolFromJSONValue(value)
		} else if value, ok := rule["integer_unit"]; ok {
			integerInventoryUnit = boolFromJSONValue(value)
		}
	}
	if inventoryUnit == "" {
		inventoryUnit = "kg"
	}
	return inventoryUnit, integerInventoryUnit
}

func boolFromJSONValue(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "yes", "y":
			return true
		}
	case float64:
		return v != 0
	}
	return false
}

func productCodeForID(id int64) string {
	if id <= 0 {
		return ""
	}
	return fmt.Sprintf("SKU-%06d", id)
}

func productJSONOrDefault(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "{}"
	}
	return raw
}

func normalizeProductKindSettings(productKind string, dripBagGrams float64, dripBoxBagCount int) (string, float64, int, []string, error) {
	productKind = catalogdomain.NormalizeProductKind(productKind)
	if productKind != catalogdomain.ProductKindDripBag {
		return productKind, dripBagGrams, dripBoxBagCount, []string{}, nil
	}
	if dripBagGrams == 0 {
		dripBagGrams = 10
	}
	if dripBoxBagCount == 0 {
		dripBoxBagCount = 10
	}
	if dripBagGrams <= 0 {
		return productKind, dripBagGrams, dripBoxBagCount, nil, ValidationError{Message: "drip_bag_grams must be > 0"}
	}
	if dripBoxBagCount <= 0 {
		return productKind, dripBagGrams, dripBoxBagCount, nil, ValidationError{Message: "drip_box_bag_count must be > 0"}
	}
	return productKind, dripBagGrams, dripBoxBagCount, []string{"bag", "box"}, nil
}

func productBomStatus(status string, itemCount int) string {
	status = strings.TrimSpace(status)
	if status != "" {
		return status
	}
	if itemCount > 0 {
		return "active"
	}
	return "missing"
}

func productVisibility(visibility string, customerID int64) string {
	visibility = strings.TrimSpace(visibility)
	if visibility != "" {
		return visibility
	}
	if customerID > 0 {
		return "customer_only"
	}
	return "public"
}

func sortCategories(categories []ProductCategory) {
	sort.SliceStable(categories, func(i, j int) bool {
		if categories[i].Position != categories[j].Position {
			return categories[i].Position < categories[j].Position
		}
		if categories[i].Name != categories[j].Name {
			return categories[i].Name < categories[j].Name
		}
		return categories[i].ID < categories[j].ID
	})
}

func sortProducts(products []ProductSettingsProduct) {
	sort.SliceStable(products, func(i, j int) bool {
		if products[i].ProductCategoryPosition != products[j].ProductCategoryPosition {
			return products[i].ProductCategoryPosition < products[j].ProductCategoryPosition
		}
		if products[i].Name != products[j].Name {
			return products[i].Name < products[j].Name
		}
		return products[i].ID < products[j].ID
	})
}
