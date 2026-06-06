package costing

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

const (
	WholesaleTierScheme454GFour = "bag_454_four"
	WholesaleTierSchemeKgThree  = "kg_three"
	WholesaleTierScheme227GTwo  = "bag_227_two"

	MissingPricingMethodWarning = "未设置计价方式"

	GradientDisplayUnitLb   = "lb"
	GradientDisplayUnitKg   = "kg"
	GradientDisplayUnit227G = "g227"
	GradientDisplayUnit100G = "g100"
	GradientDisplayUnit250G = "g250"
)

type Parameters struct {
	RoastYieldRate                float64   `json:"roast_yield_rate"`
	KgToLbFactor                  float64   `json:"kg_to_lb_factor"`
	SmallBatchProductionCostPerKg float64   `json:"small_batch_production_cost_per_kg"`
	LargeBatchProductionCostPerKg float64   `json:"large_batch_production_cost_per_kg"`
	WholesalePackageCostPerKg     float64   `json:"wholesale_package_cost_per_kg"`
	ProductLossPerKg              float64   `json:"product_loss_per_kg"`
	RetailBeanMarginRate          float64   `json:"retail_bean_margin_rate"`
	RetailTaxRate                 float64   `json:"retail_tax_rate"`
	RetailLogisticsPerKg          float64   `json:"retail_logistics_per_kg"`
	RetailDripLogisticsPer10Bags  float64   `json:"retail_drip_logistics_per_10_bags"`
	DripGreenRatioKgPerBag        float64   `json:"drip_green_ratio_kg_per_bag"`
	DripProcessCostPerBag         float64   `json:"drip_process_cost_per_bag"`
	DripExtraCostPerBag           float64   `json:"drip_extra_cost_per_bag"`
	DripPackingMaterialPerBag     float64   `json:"drip_packing_material_per_bag"`
	RetailDripMultiplier          float64   `json:"retail_drip_multiplier"`
	WholesaleKgMarginRates        []float64 `json:"wholesale_kg_margin_rates"`
	WholesaleDripMultipliers      []float64 `json:"wholesale_drip_multipliers"`
}

type ProductInput struct {
	ProductID                  int64                     `json:"product_id"`
	ProductCode                string                    `json:"product_code,omitempty"`
	ProductName                string                    `json:"product_name,omitempty"`
	Name                       string                    `json:"name"`
	CustomerProductAliasID     int64                     `json:"customer_product_alias_id,omitempty"`
	CustomerProductDisplayName string                    `json:"customer_product_display_name,omitempty"`
	CustomerItemCode           string                    `json:"customer_item_code,omitempty"`
	BrandName                  string                    `json:"brand_name,omitempty"`
	DisplayCategoryID          int64                     `json:"display_category_id,omitempty"`
	DisplayCategoryName        string                    `json:"display_category_name,omitempty"`
	ClassificationTemplateID   int64                     `json:"classification_template_id,omitempty"`
	ClassificationTemplateName string                    `json:"classification_template_name,omitempty"`
	ClassificationCategoryID   int64                     `json:"classification_category_id,omitempty"`
	ClassificationCategoryName string                    `json:"classification_category_name,omitempty"`
	BomVersionID               int64                     `json:"bom_version_id,omitempty"`
	BomVersionNo               string                    `json:"bom_version_no,omitempty"`
	BomUsageMode               string                    `json:"bom_usage_mode,omitempty"`
	ProductKind                string                    `json:"product_kind,omitempty"`
	DripBagGrams               float64                   `json:"drip_bag_grams,omitempty"`
	DripBoxBagCount            int                       `json:"drip_box_bag_count,omitempty"`
	CustomerID                 int64                     `json:"customer_id,omitempty"`
	BaseProductID              int64                     `json:"base_product_id,omitempty"`
	Visibility                 string                    `json:"visibility,omitempty"`
	CustomType                 string                    `json:"custom_type,omitempty"`
	ProductCategoryID          int64                     `json:"product_category_id,omitempty"`
	ProductCategoryPosition    int                       `json:"product_category_position,omitempty"`
	ProductTypeCategoryID      int64                     `json:"product_type_category_id,omitempty"`
	ProductSubtypeCategoryID   int64                     `json:"product_subtype_category_id,omitempty"`
	ProductTypeName            string                    `json:"product_type_name,omitempty"`
	ProductSubtypeName         string                    `json:"product_subtype_name,omitempty"`
	CategoryPrimaryName        string                    `json:"category_primary_name,omitempty"`
	CategoryPrimaryPosition    int                       `json:"category_primary_position,omitempty"`
	CategorySecondaryName      string                    `json:"category_secondary_name,omitempty"`
	CategorySecondaryPosition  int                       `json:"category_secondary_position,omitempty"`
	OperationTemplateID        int64                     `json:"operation_template_id,omitempty"`
	InventoryUnit              string                    `json:"inventory_unit,omitempty"`
	QuoteUnit                  string                    `json:"quote_unit,omitempty"`
	OrderUnit                  string                    `json:"order_unit,omitempty"`
	UnitConversionJSON         string                    `json:"unit_conversion_json,omitempty"`
	IntegerUnit                bool                      `json:"integer_unit,omitempty"`
	PriceListRuleJSON          string                    `json:"price_list_rule_json,omitempty"`
	SpecialAttrsJSON           string                    `json:"special_attrs_json,omitempty"`
	SpecialAttrsSchemaJSON     string                    `json:"special_attrs_schema_json,omitempty"`
	BeanListTemplateName       string                    `json:"bean_list_template_name,omitempty"`
	Flavor                     string                    `json:"flavor,omitempty"`
	Origin                     string                    `json:"origin,omitempty"`
	ProcessingStation          string                    `json:"processing_station,omitempty"`
	Variety                    string                    `json:"variety,omitempty"`
	ProcessMethod              string                    `json:"process_method,omitempty"`
	Grade                      string                    `json:"grade,omitempty"`
	Altitude                   string                    `json:"altitude,omitempty"`
	BeanListNote               string                    `json:"bean_list_note,omitempty"`
	BomStatus                  string                    `json:"bom_status,omitempty"`
	Warnings                   []string                  `json:"warnings,omitempty"`
	GreenBeanCostPerKg         float64                   `json:"green_bean_cost_per_kg"`
	BomCostPerUnit             float64                   `json:"bom_cost_per_unit,omitempty"`
	OperationCostPerUnit       float64                   `json:"operation_cost_per_unit,omitempty"`
	OperationCostPerKg         float64                   `json:"operation_cost_per_kg,omitempty"`
	YieldRate                  float64                   `json:"yield_rate"`
	ExpectedLossRate           float64                   `json:"expected_loss_rate,omitempty"`
	WholesaleTaxAddPerKg       float64                   `json:"wholesale_tax_add_per_kg"`
	WholesaleTaxAddPerKgTiers  []float64                 `json:"wholesale_tax_add_per_kg_tiers"`
	DripTaxAddPerBag100        float64                   `json:"drip_tax_add_per_bag_100"`
	DripTaxAddPerBagRetail     float64                   `json:"drip_tax_add_per_bag_retail"`
	WholesaleKgMarginRates     []float64                 `json:"wholesale_kg_margin_rates"`
	WholesaleDripMultipliers   []float64                 `json:"wholesale_drip_multipliers"`
	WholesaleTierScheme        string                    `json:"wholesale_tier_scheme,omitempty"`
	MarginRateOverride         *float64                  `json:"margin_rate_override,omitempty"`
	GradientTemplate           *GradientTemplate         `json:"gradient_template,omitempty"`
	DripPriceTemplate          *DripPriceTemplate        `json:"drip_price_template,omitempty"`
	ProductPriceSnapshots      []ProductPriceSnapshot    `json:"product_price_snapshots,omitempty"`
	GreenBeanSaleTiers         []CommercialWholesaleTier `json:"green_bean_sale_tiers,omitempty"`
	BeanListQuality            BeanListQuality           `json:"bean_list_quality,omitempty"`
}

type ProductPriceSnapshot struct {
	SourcePriceRecordID     int64           `json:"source_price_record_id"`
	FinalUnitPrice          float64         `json:"final_unit_price"`
	PriceUnit               string          `json:"price_unit"`
	Currency                string          `json:"currency"`
	PriceGroupID            int64           `json:"price_group_id,omitempty"`
	PriceGroupName          string          `json:"price_group_name,omitempty"`
	InventoryUnit           string          `json:"inventory_unit"`
	InventoryConversionJSON json.RawMessage `json:"inventory_conversion_json"`
	ProductID               int64           `json:"product_id,omitempty"`
	CustomerProductAliasID  int64           `json:"customer_product_alias_id,omitempty"`
}

type CommercialWholesaleTier struct {
	Label          string   `json:"label"`
	Scheme         string   `json:"scheme,omitempty"`
	SpecG          int64    `json:"spec_g,omitempty"`
	MinQty         float64  `json:"min_qty,omitempty"`
	MaxQty         *float64 `json:"max_qty,omitempty"`
	PricePerUnit   float64  `json:"price_per_unit"`
	MinLb          float64  `json:"min_lb"`
	MaxLb          *float64 `json:"max_lb,omitempty"`
	PricePerKg     float64  `json:"price_per_kg"`
	PricePerLb     float64  `json:"price_per_lb"`
	TemplateID     int64    `json:"template_id,omitempty"`
	TemplateTierID int64    `json:"template_tier_id,omitempty"`
	DisplayUnit    string   `json:"display_unit,omitempty"`
	PriceUnit      string   `json:"price_unit,omitempty"`
	MinWeightG     float64  `json:"min_weight_g,omitempty"`
	MaxWeightG     *float64 `json:"max_weight_g,omitempty"`
	MarginRate     float64  `json:"margin_rate,omitempty"`
}

type DripWholesaleTier struct {
	Label             string   `json:"label,omitempty"`
	MinBags           int64    `json:"min_bags"`
	MaxBags           *float64 `json:"max_bags,omitempty"`
	Multiplier        float64  `json:"multiplier"`
	LoosePricePerBag  float64  `json:"loose_price_per_bag"`
	PackedPricePerBag float64  `json:"packed_price_per_bag"`
	TemplateID        int64    `json:"template_id,omitempty"`
	TemplateTierID    int64    `json:"template_tier_id,omitempty"`
	BagGrams          float64  `json:"bag_grams,omitempty"`
	BoxBagCount       int      `json:"box_bag_count,omitempty"`
	TaxRate           float64  `json:"tax_rate,omitempty"`
}

type DripPriceTemplate struct {
	ID               int64                   `json:"id,omitempty"`
	Name             string                  `json:"name"`
	Active           bool                    `json:"active"`
	BagGrams         float64                 `json:"bag_grams"`
	BoxBagCount      int                     `json:"box_bag_count"`
	IncludePackaging bool                    `json:"include_packaging"`
	Tiers            []DripPriceTemplateTier `json:"tiers"`
}

type DripPriceTemplateTier struct {
	ID         int64    `json:"id,omitempty"`
	Label      string   `json:"label"`
	MinBags    float64  `json:"min_bags"`
	MaxBags    *float64 `json:"max_bags,omitempty"`
	Multiplier float64  `json:"multiplier"`
	Position   int      `json:"position"`
	Active     bool     `json:"active"`
}

type GradientTemplate struct {
	ID          int64                  `json:"id,omitempty"`
	Name        string                 `json:"name"`
	DisplayUnit string                 `json:"display_unit"`
	Active      bool                   `json:"active"`
	Tiers       []GradientTemplateTier `json:"tiers"`
}

type GradientTemplateTier struct {
	ID         int64    `json:"id,omitempty"`
	Label      string   `json:"label"`
	MinWeightG float64  `json:"min_weight_g"`
	MaxWeightG *float64 `json:"max_weight_g,omitempty"`
	MarginRate float64  `json:"margin_rate"`
	Position   int      `json:"position"`
}

type PriceExplanationRequest struct {
	TierLabel string                    `json:"tier_label"`
	Overrides PriceExplanationOverrides `json:"overrides,omitempty"`
}

type PriceExplanationOverrides struct {
	GreenBeanCostPerKg *float64 `json:"green_bean_cost_per_kg,omitempty"`
	YieldRate          *float64 `json:"yield_rate,omitempty"`
	MarginRate         *float64 `json:"margin_rate,omitempty"`
}

type PriceExplanationStep struct {
	Key     string  `json:"key"`
	Label   string  `json:"label"`
	Source  string  `json:"source"`
	Value   float64 `json:"value"`
	Unit    string  `json:"unit,omitempty"`
	Changed bool    `json:"changed,omitempty"`
}

type PriceExplanation struct {
	ProductID         int64                  `json:"product_id"`
	ProductName       string                 `json:"product_name"`
	TemplateID        int64                  `json:"template_id,omitempty"`
	TemplateName      string                 `json:"template_name,omitempty"`
	TierLabel         string                 `json:"tier_label"`
	DisplayUnit       string                 `json:"display_unit"`
	MinWeightG        float64                `json:"min_weight_g"`
	MaxWeightG        *float64               `json:"max_weight_g,omitempty"`
	SavedFinalPrice   float64                `json:"saved_final_price"`
	PreviewFinalPrice float64                `json:"preview_final_price"`
	Steps             []PriceExplanationStep `json:"steps"`
}

type DripPriceExplanation struct {
	ProductID         int64                  `json:"product_id"`
	ProductName       string                 `json:"product_name"`
	TemplateID        int64                  `json:"template_id,omitempty"`
	TemplateTierID    int64                  `json:"template_tier_id,omitempty"`
	TemplateName      string                 `json:"template_name,omitempty"`
	TierLabel         string                 `json:"tier_label"`
	BagGrams          float64                `json:"bag_grams"`
	BoxBagCount       int                    `json:"box_bag_count"`
	MinBags           int64                  `json:"min_bags"`
	MinBoxes          int64                  `json:"min_boxes"`
	LoosePricePerBag  float64                `json:"loose_price_per_bag"`
	PackedPricePerBag float64                `json:"packed_price_per_bag"`
	PackedPricePerBox float64                `json:"packed_price_per_box"`
	Steps             []PriceExplanationStep `json:"steps"`
}

func (e PriceExplanation) HasStep(key string) bool {
	for _, step := range e.Steps {
		if step.Key == key {
			return true
		}
	}
	return false
}

type RetailBeanTier struct {
	Label        string  `json:"label"`
	SpecG        int64   `json:"spec_g"`
	PricePerUnit float64 `json:"price_per_unit"`
}

type BeanListDisplay struct {
	Code           string `json:"code,omitempty"`
	Category       string `json:"category,omitempty"`
	DisplayName    string `json:"display_name,omitempty"`
	RecommendedUse string `json:"recommended_use,omitempty"`
	Flavor         string `json:"flavor,omitempty"`
	Description    string `json:"description,omitempty"`
}

type BeanListQuality struct {
	FactoryFlavorDescription string `json:"factory_flavor_description,omitempty"`
	Moisture                 string `json:"moisture,omitempty"`
	Density                  string `json:"density,omitempty"`
	InspectionCreatedAt      string `json:"inspection_created_at,omitempty"`
	InspectionReferenceNo    string `json:"inspection_reference_no,omitempty"`
}

type ProductAttribute struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Value    string `json:"value"`
	Position int    `json:"position,omitempty"`
}

type ProductResult struct {
	ProductID                      int64                     `json:"product_id"`
	ProductCode                    string                    `json:"product_code,omitempty"`
	ProductName                    string                    `json:"product_name,omitempty"`
	Name                           string                    `json:"name"`
	CustomerProductAliasID         int64                     `json:"customer_product_alias_id,omitempty"`
	CustomerProductDisplayName     string                    `json:"customer_product_display_name,omitempty"`
	CustomerItemCode               string                    `json:"customer_item_code,omitempty"`
	BrandName                      string                    `json:"brand_name,omitempty"`
	DisplayCategoryID              int64                     `json:"display_category_id,omitempty"`
	DisplayCategoryName            string                    `json:"display_category_name,omitempty"`
	ClassificationTemplateID       int64                     `json:"classification_template_id,omitempty"`
	ClassificationTemplateName     string                    `json:"classification_template_name,omitempty"`
	ClassificationCategoryID       int64                     `json:"classification_category_id,omitempty"`
	ClassificationCategoryName     string                    `json:"classification_category_name,omitempty"`
	BomVersionID                   int64                     `json:"bom_version_id,omitempty"`
	BomVersionNo                   string                    `json:"bom_version_no,omitempty"`
	BomUsageMode                   string                    `json:"bom_usage_mode,omitempty"`
	ProductKind                    string                    `json:"product_kind,omitempty"`
	DripBagGrams                   float64                   `json:"drip_bag_grams,omitempty"`
	DripBoxBagCount                int                       `json:"drip_box_bag_count,omitempty"`
	CustomerID                     int64                     `json:"customer_id,omitempty"`
	BaseProductID                  int64                     `json:"base_product_id,omitempty"`
	Visibility                     string                    `json:"visibility,omitempty"`
	CustomType                     string                    `json:"custom_type,omitempty"`
	ProductCategoryID              int64                     `json:"product_category_id,omitempty"`
	ProductCategoryPosition        int                       `json:"product_category_position,omitempty"`
	ProductTypeCategoryID          int64                     `json:"product_type_category_id,omitempty"`
	ProductSubtypeCategoryID       int64                     `json:"product_subtype_category_id,omitempty"`
	ProductTypeName                string                    `json:"product_type_name,omitempty"`
	ProductSubtypeName             string                    `json:"product_subtype_name,omitempty"`
	CategoryPrimaryName            string                    `json:"category_primary_name,omitempty"`
	CategoryPrimaryPosition        int                       `json:"category_primary_position,omitempty"`
	CategorySecondaryName          string                    `json:"category_secondary_name,omitempty"`
	CategorySecondaryPosition      int                       `json:"category_secondary_position,omitempty"`
	OperationTemplateID            int64                     `json:"operation_template_id,omitempty"`
	InventoryUnit                  string                    `json:"inventory_unit,omitempty"`
	QuoteUnit                      string                    `json:"quote_unit,omitempty"`
	OrderUnit                      string                    `json:"order_unit,omitempty"`
	UnitConversionJSON             string                    `json:"unit_conversion_json,omitempty"`
	IntegerUnit                    bool                      `json:"integer_unit,omitempty"`
	PriceListRuleJSON              string                    `json:"price_list_rule_json,omitempty"`
	ProductAttributes              []ProductAttribute        `json:"product_attributes,omitempty"`
	MarginRateOverride             *float64                  `json:"margin_rate_override,omitempty"`
	GradientTemplate               *GradientTemplate         `json:"gradient_template,omitempty"`
	DripPriceTemplate              *DripPriceTemplate        `json:"drip_price_template,omitempty"`
	ProductPriceSnapshots          []ProductPriceSnapshot    `json:"product_price_snapshots,omitempty"`
	CommercialBeanList             BeanListDisplay           `json:"commercial_bean_list"`
	DripBeanList                   BeanListDisplay           `json:"drip_bean_list"`
	RetailBeanList                 BeanListDisplay           `json:"retail_bean_list"`
	GreenBeanList                  BeanListDisplay           `json:"green_bean_list"`
	BeanListQuality                BeanListQuality           `json:"bean_list_quality,omitempty"`
	Flavor                         string                    `json:"flavor,omitempty"`
	Origin                         string                    `json:"origin,omitempty"`
	ProcessingStation              string                    `json:"processing_station,omitempty"`
	Variety                        string                    `json:"variety,omitempty"`
	ProcessMethod                  string                    `json:"process_method,omitempty"`
	Grade                          string                    `json:"grade,omitempty"`
	Altitude                       string                    `json:"altitude,omitempty"`
	BeanListNote                   string                    `json:"bean_list_note,omitempty"`
	BomStatus                      string                    `json:"bom_status,omitempty"`
	Warnings                       []string                  `json:"warnings,omitempty"`
	YieldRate                      float64                   `json:"yield_rate"`
	ExpectedLossRate               float64                   `json:"expected_loss_rate,omitempty"`
	GreenBeanCostPerKg             float64                   `json:"green_bean_cost_per_kg"`
	BomCostPerUnit                 float64                   `json:"bom_cost_per_unit,omitempty"`
	OperationCostPerUnit           float64                   `json:"operation_cost_per_unit,omitempty"`
	OperationCostPerKg             float64                   `json:"operation_cost_per_kg,omitempty"`
	RoastedBeanCostPerKg           float64                   `json:"roasted_bean_cost_per_kg"`
	SmallBatchCostPerKg            float64                   `json:"small_batch_cost_per_kg"`
	LargeBatchCostPerKg            float64                   `json:"large_batch_cost_per_kg"`
	DripBaseCostPerBag             float64                   `json:"drip_base_cost_per_bag"`
	RetailTaxPerKg                 float64                   `json:"retail_tax_per_kg"`
	WholesaleKgPrices              []float64                 `json:"wholesale_kg_prices"`
	WholesaleLbPrices              []float64                 `json:"wholesale_lb_prices"`
	CommercialWholesaleTiers       []CommercialWholesaleTier `json:"commercial_wholesale_tiers"`
	GreenBeanSaleTiers             []CommercialWholesaleTier `json:"green_bean_sale_tiers,omitempty"`
	DripWholesaleTiers             []DripWholesaleTier       `json:"drip_wholesale_tiers"`
	WholesaleDripBagPrices         []float64                 `json:"wholesale_drip_bag_prices"`
	WholesaleDripBagWithPackPrices []float64                 `json:"wholesale_drip_bag_with_pack_prices"`
	RetailKgPrice                  float64                   `json:"retail_kg_price"`
	Retail454gPrice                float64                   `json:"retail_454g_price"`
	Retail227gPrice                float64                   `json:"retail_227g_price"`
	Retail250gPrice                float64                   `json:"retail_250g_price"`
	Retail200gPrice                float64                   `json:"retail_200g_price"`
	Retail100gPrice                float64                   `json:"retail_100g_price"`
	RetailBeanTiers                []RetailBeanTier          `json:"retail_bean_tiers"`
	RetailDrip10BagPrice           float64                   `json:"retail_drip_10_bag_price"`
}

func DefaultParameters() Parameters {
	return Parameters{
		RoastYieldRate:                0.8,
		KgToLbFactor:                  0.454,
		SmallBatchProductionCostPerKg: 6.2625,
		LargeBatchProductionCostPerKg: 3.1625,
		WholesalePackageCostPerKg:     1.7,
		ProductLossPerKg:              0.06,
		RetailBeanMarginRate:          1.45,
		RetailTaxRate:                 0.03,
		RetailLogisticsPerKg:          8,
		RetailDripLogisticsPer10Bags:  8,
		DripGreenRatioKgPerBag:        0.01,
		DripProcessCostPerBag:         0.44,
		DripExtraCostPerBag:           0.1,
		DripPackingMaterialPerBag:     0.2,
		RetailDripMultiplier:          2.5,
		WholesaleKgMarginRates:        []float64{0.5421052631578949, 0.3842105263157895, 0.27894736842105267, 0.2, 0.12, 0.045},
		WholesaleDripMultipliers:      []float64{2.2, 1.8, 1.6, 1.35},
	}
}

func ValidateProductInput(params Parameters, in ProductInput) (ProductInput, error) {
	if in.GreenBeanCostPerKg < 0 {
		return in, fmt.Errorf("green_bean_cost_per_kg must be >= 0")
	}
	if in.BomCostPerUnit < 0 {
		return in, fmt.Errorf("bom_cost_per_unit must be >= 0")
	}
	if in.OperationCostPerUnit < 0 || in.OperationCostPerKg < 0 {
		return in, fmt.Errorf("operation cost must be >= 0")
	}
	if in.ExpectedLossRate < 0 || in.ExpectedLossRate >= 1 {
		return in, fmt.Errorf("expected_loss_rate must be [0,1)")
	}
	if in.ExpectedLossRate > 0 {
		in.YieldRate = 1 - in.ExpectedLossRate
	}
	if in.YieldRate == 0 {
		in.YieldRate = params.RoastYieldRate
	}
	if in.YieldRate <= 0 || in.YieldRate > 1 {
		return in, fmt.Errorf("yield_rate must be (0,1]")
	}
	in.ExpectedLossRate = 1 - in.YieldRate
	if in.MarginRateOverride != nil && *in.MarginRateOverride < 0 {
		return in, fmt.Errorf("margin_rate_override must be >= 0")
	}
	in.ProductKind = normalizeProductKind(in.ProductKind)
	if in.DripBagGrams <= 0 {
		in.DripBagGrams = 10
	}
	if in.DripBoxBagCount <= 0 {
		in.DripBoxBagCount = 10
	}
	in.InventoryUnit = normalizeProductUnit(in.InventoryUnit, "kg")
	in.QuoteUnit = normalizeProductUnit(in.QuoteUnit, in.InventoryUnit)
	in.OrderUnit = normalizeProductUnit(in.OrderUnit, in.QuoteUnit)
	in.UnitConversionJSON = strings.TrimSpace(in.UnitConversionJSON)
	if in.UnitConversionJSON == "" {
		in.UnitConversionJSON = "{}"
	}
	in.PriceListRuleJSON = strings.TrimSpace(in.PriceListRuleJSON)
	if in.PriceListRuleJSON == "" {
		in.PriceListRuleJSON = "{}"
	}
	in.SpecialAttrsJSON = strings.TrimSpace(in.SpecialAttrsJSON)
	if in.SpecialAttrsJSON == "" {
		in.SpecialAttrsJSON = "{}"
	}
	in.SpecialAttrsSchemaJSON = strings.TrimSpace(in.SpecialAttrsSchemaJSON)
	if in.SpecialAttrsSchemaJSON == "" {
		in.SpecialAttrsSchemaJSON = "[]"
	}
	if strings.TrimSpace(in.ProductTypeName) == "" {
		in.ProductTypeName = strings.TrimSpace(in.CategoryPrimaryName)
	}
	if strings.TrimSpace(in.ProductSubtypeName) == "" {
		in.ProductSubtypeName = strings.TrimSpace(in.CategorySecondaryName)
	}
	in = ApplyExcelCommercialPricingProfile(params, in)
	if len(in.WholesaleKgMarginRates) == 0 {
		in.WholesaleKgMarginRates = params.WholesaleKgMarginRates
	}
	in.WholesaleKgMarginRates = normalizeWholesaleMarginRates(params, in.WholesaleKgMarginRates)
	if len(in.WholesaleTaxAddPerKgTiers) == 0 {
		in.WholesaleTaxAddPerKgTiers = defaultWholesaleTaxAddPerKgTiers(params, in)
	}
	if len(in.WholesaleDripMultipliers) == 0 {
		in.WholesaleDripMultipliers = params.WholesaleDripMultipliers
	}
	in.BomStatus = normalizeBomStatus(in.BomStatus)
	in.Warnings = normalizeWarnings(in.Warnings)
	return in, nil
}

func ApplyExcelCommercialPricingProfile(params Parameters, in ProductInput) ProductInput {
	profileName := costingProfileName(in)
	if strings.TrimSpace(in.WholesaleTierScheme) == "" {
		in.WholesaleTierScheme = inferWholesaleTierScheme(profileName)
	} else {
		in.WholesaleTierScheme = normalizeWholesaleTierScheme(in.WholesaleTierScheme)
	}
	if len(in.WholesaleKgMarginRates) == 0 {
		switch {
		case isCookieBlend(profileName):
			in.WholesaleKgMarginRates = cookieWholesaleMarginRates(params)
		case isWineSunBean(profileName):
			in.WholesaleKgMarginRates = wineSunWholesaleMarginRates(params)
		case isYirgacheffeG2(profileName):
			in.WholesaleKgMarginRates = premiumFirstThreeWholesaleMarginRates(params)
		case isPremiumCommercialBean(profileName):
			in.WholesaleKgMarginRates = premiumWholesaleMarginRates(params)
		default:
			in.WholesaleKgMarginRates = normalizeWholesaleMarginRates(params, nil)
		}
	}
	return in
}

func normalizeProductKind(kind string) string {
	value := strings.TrimSpace(kind)
	switch value {
	case "drip_bag":
		return "drip_bag"
	case "green_bean":
		return "green_bean"
	case "instant_coffee":
		return "instant_coffee"
	case "roasted":
		return "roasted"
	default:
		if value != "" {
			return value
		}
		return "roasted"
	}
}

func CalculateProduct(params Parameters, in ProductInput) ProductResult {
	if strings.TrimSpace(in.ProductKind) == "green_bean" {
		return calculateGreenBeanProduct(params, in)
	}
	in, _ = ValidateProductInput(params, in)
	profileName := costingProfileName(in)
	roasted := in.GreenBeanCostPerKg / in.YieldRate
	small := roasted + params.SmallBatchProductionCostPerKg
	large := roasted + params.LargeBatchProductionCostPerKg
	dripRatioKgPerBag := in.DripBagGrams / 1000.0
	if dripRatioKgPerBag <= 0 {
		dripRatioKgPerBag = params.DripGreenRatioKgPerBag
	}
	dripBase := small*dripRatioKgPerBag + params.DripProcessCostPerBag + params.DripExtraCostPerBag
	retailTax := small * params.RetailBeanMarginRate * params.RetailTaxRate
	retailSmall := small
	if retailGreenCost, ok := excelRetailGreenCostOverride(profileName); ok {
		retailSmall = retailGreenCost/in.YieldRate + params.SmallBatchProductionCostPerKg
		retailTax = retailSmall * params.RetailBeanMarginRate * params.RetailTaxRate
	}
	commercialDisplay := commercialBeanListDisplay(profileName)
	retailDisplay := retailBeanListDisplay(profileName)
	dripDisplay := BeanListDisplay{}
	if in.CustomerID > 0 {
		commercialDisplay = customerCategoryBeanListDisplay(in, commercialDisplay, true)
		retailDisplay = customerCategoryBeanListDisplay(in, retailDisplay, false)
	} else {
		if commercialDisplay.Code == "" {
			commercialDisplay = customerCategoryBeanListDisplay(in, commercialDisplay, true)
		}
		if retailDisplay.Code == "" {
			retailDisplay = customerCategoryBeanListDisplay(in, retailDisplay, false)
		}
	}

	greenDisplay := BeanListDisplay{}
	// 产品分类为生豆时，即使 product_kind 非 green_bean，也生成 green_bean_list meta
	if isGreenBeanCategory(in) {
		greenDisplay = buildCategoryGreenBeanListDisplay(in, commercialDisplay)
	}

	out := ProductResult{
		ProductID:                  in.ProductID,
		ProductCode:                in.ProductCode,
		ProductName:                in.ProductName,
		Name:                       in.Name,
		CustomerProductAliasID:     in.CustomerProductAliasID,
		CustomerProductDisplayName: in.CustomerProductDisplayName,
		CustomerItemCode:           in.CustomerItemCode,
		BrandName:                  in.BrandName,
		DisplayCategoryID:          in.DisplayCategoryID,
		DisplayCategoryName:        in.DisplayCategoryName,
		ClassificationTemplateID:   in.ClassificationTemplateID,
		ClassificationTemplateName: in.ClassificationTemplateName,
		ClassificationCategoryID:   in.ClassificationCategoryID,
		ClassificationCategoryName: in.ClassificationCategoryName,
		BomVersionID:               in.BomVersionID,
		BomVersionNo:               in.BomVersionNo,
		BomUsageMode:               in.BomUsageMode,
		ProductKind:                in.ProductKind,
		DripBagGrams:               in.DripBagGrams,
		DripBoxBagCount:            in.DripBoxBagCount,
		CustomerID:                 in.CustomerID,
		BaseProductID:              in.BaseProductID,
		Visibility:                 in.Visibility,
		CustomType:                 in.CustomType,
		ProductCategoryID:          in.ProductCategoryID,
		ProductCategoryPosition:    in.ProductCategoryPosition,
		ProductTypeCategoryID:      in.ProductTypeCategoryID,
		ProductSubtypeCategoryID:   in.ProductSubtypeCategoryID,
		ProductTypeName:            in.ProductTypeName,
		ProductSubtypeName:         in.ProductSubtypeName,
		CategoryPrimaryName:        in.CategoryPrimaryName,
		CategoryPrimaryPosition:    in.CategoryPrimaryPosition,
		CategorySecondaryName:      in.CategorySecondaryName,
		CategorySecondaryPosition:  in.CategorySecondaryPosition,
		OperationTemplateID:        in.OperationTemplateID,
		InventoryUnit:              in.InventoryUnit,
		QuoteUnit:                  in.QuoteUnit,
		OrderUnit:                  in.OrderUnit,
		UnitConversionJSON:         in.UnitConversionJSON,
		IntegerUnit:                in.IntegerUnit,
		PriceListRuleJSON:          in.PriceListRuleJSON,
		ProductAttributes:          productAttributesFromSpecialAttrs(in.SpecialAttrsSchemaJSON, in.SpecialAttrsJSON),
		MarginRateOverride:         in.MarginRateOverride,
		GradientTemplate:           in.GradientTemplate,
		DripPriceTemplate:          in.DripPriceTemplate,
		ProductPriceSnapshots:      append([]ProductPriceSnapshot(nil), in.ProductPriceSnapshots...),
		CommercialBeanList:         commercialDisplay,
		DripBeanList:               dripDisplay,
		RetailBeanList:             retailDisplay,
		GreenBeanList:              greenDisplay,
		BeanListQuality:            in.BeanListQuality,
		Flavor:                     in.Flavor,
		Origin:                     in.Origin,
		ProcessingStation:          in.ProcessingStation,
		Variety:                    in.Variety,
		ProcessMethod:              in.ProcessMethod,
		Grade:                      in.Grade,
		Altitude:                   in.Altitude,
		BeanListNote:               in.BeanListNote,
		BomStatus:                  in.BomStatus,
		Warnings:                   append([]string(nil), in.Warnings...),
		YieldRate:                  in.YieldRate,
		ExpectedLossRate:           in.ExpectedLossRate,
		GreenBeanCostPerKg:         in.GreenBeanCostPerKg,
		BomCostPerUnit:             in.BomCostPerUnit,
		OperationCostPerUnit:       in.OperationCostPerUnit,
		OperationCostPerKg:         in.OperationCostPerKg,
		RoastedBeanCostPerKg:       roasted,
		SmallBatchCostPerKg:        small,
		LargeBatchCostPerKg:        large,
		DripBaseCostPerBag:         dripBase,
		RetailTaxPerKg:             retailTax,
	}

	for i, margin := range in.WholesaleKgMarginRates {
		base := small
		if i >= 2 {
			base = large
		}
		taxAdd := base * margin * params.RetailTaxRate
		if in.WholesaleTaxAddPerKg != 0 {
			taxAdd = in.WholesaleTaxAddPerKg
		}
		if i < len(in.WholesaleTaxAddPerKgTiers) {
			taxAdd = in.WholesaleTaxAddPerKgTiers[i]
		}
		kg := base*(1+margin) + params.WholesalePackageCostPerKg + params.ProductLossPerKg + taxAdd
		out.WholesaleKgPrices = append(out.WholesaleKgPrices, kg)
		out.WholesaleLbPrices = append(out.WholesaleLbPrices, kg*params.KgToLbFactor+1)
	}
	out.CommercialWholesaleTiers = buildCommercialWholesaleTiers(params, in, out.WholesaleKgPrices, out.WholesaleLbPrices)
	if shouldWarnMissingPricingMethod(in) {
		out.Warnings = normalizeWarnings(append(out.Warnings, MissingPricingMethodWarning))
	}

	if in.DripPriceTemplate != nil {
		out.DripWholesaleTiers = buildDripWholesaleTiers(params, in)
		for _, tier := range out.DripWholesaleTiers {
			out.WholesaleDripBagPrices = append(out.WholesaleDripBagPrices, tier.LoosePricePerBag)
			out.WholesaleDripBagWithPackPrices = append(out.WholesaleDripBagWithPackPrices, tier.PackedPricePerBag)
		}
	}

	out.RetailKgPrice = retailSmall*(1+params.RetailBeanMarginRate) + params.WholesalePackageCostPerKg + params.ProductLossPerKg + retailTax + params.RetailLogisticsPerKg
	out.Retail454gPrice = out.RetailKgPrice * params.KgToLbFactor
	out.Retail227gPrice = out.Retail454gPrice / 2
	out.Retail250gPrice = out.RetailKgPrice * 0.25
	out.Retail200gPrice = out.RetailKgPrice * 0.2
	out.Retail100gPrice = out.RetailKgPrice * 0.1
	out.RetailDrip10BagPrice = dripBase*10*params.RetailDripMultiplier + in.DripTaxAddPerBagRetail*10 + params.DripPackingMaterialPerBag + params.RetailDripLogisticsPer10Bags
	out.RetailBeanTiers = buildRetailBeanTiers(profileName, out)
	roundProductPrices(&out)
	return out
}

func calculateGreenBeanProduct(params Parameters, in ProductInput) ProductResult {
	in.InventoryUnit = normalizeProductUnit(in.InventoryUnit, "kg")
	in.QuoteUnit = normalizeProductUnit(in.QuoteUnit, in.InventoryUnit)
	in.OrderUnit = normalizeProductUnit(in.OrderUnit, in.QuoteUnit)
	in.UnitConversionJSON = strings.TrimSpace(in.UnitConversionJSON)
	if in.UnitConversionJSON == "" {
		in.UnitConversionJSON = "{}"
	}
	in.SpecialAttrsJSON = strings.TrimSpace(in.SpecialAttrsJSON)
	if in.SpecialAttrsJSON == "" {
		in.SpecialAttrsJSON = "{}"
	}
	in.SpecialAttrsSchemaJSON = strings.TrimSpace(in.SpecialAttrsSchemaJSON)
	if in.SpecialAttrsSchemaJSON == "" {
		in.SpecialAttrsSchemaJSON = "[]"
	}
	if strings.TrimSpace(in.ProductTypeName) == "" {
		in.ProductTypeName = strings.TrimSpace(in.CategoryPrimaryName)
	}
	if strings.TrimSpace(in.ProductSubtypeName) == "" {
		in.ProductSubtypeName = strings.TrimSpace(in.CategorySecondaryName)
	}
	displayName := strings.TrimSpace(in.BeanListTemplateName)
	if displayName == "" {
		displayName = strings.TrimSpace(in.Name)
	}
	code := fmt.Sprintf("G.%d", in.ProductID)
	if in.ProductID <= 0 {
		code = "G.0"
	}
	tiers := buildGreenBeanTemplateSaleTiers(params, in)
	bomStatus := "bom_cost_template_price"
	if len(tiers) == 0 {
		tiers = normalizeLegacyGreenBeanSaleTiers(in.GreenBeanSaleTiers)
		bomStatus = "missing_green_bean_template"
		if len(tiers) > 0 {
			bomStatus = "direct_sale_price"
		}
	}
	out := ProductResult{
		ProductID:                  in.ProductID,
		ProductCode:                in.ProductCode,
		ProductName:                in.ProductName,
		Name:                       in.Name,
		CustomerProductAliasID:     in.CustomerProductAliasID,
		CustomerProductDisplayName: in.CustomerProductDisplayName,
		CustomerItemCode:           in.CustomerItemCode,
		BrandName:                  in.BrandName,
		DisplayCategoryID:          in.DisplayCategoryID,
		DisplayCategoryName:        in.DisplayCategoryName,
		ClassificationTemplateID:   in.ClassificationTemplateID,
		ClassificationTemplateName: in.ClassificationTemplateName,
		ClassificationCategoryID:   in.ClassificationCategoryID,
		ClassificationCategoryName: in.ClassificationCategoryName,
		BomVersionID:               in.BomVersionID,
		BomVersionNo:               in.BomVersionNo,
		BomUsageMode:               in.BomUsageMode,
		ProductKind:                "green_bean",
		CustomerID:                 in.CustomerID,
		BaseProductID:              in.BaseProductID,
		Visibility:                 in.Visibility,
		CustomType:                 in.CustomType,
		ProductCategoryID:          in.ProductCategoryID,
		ProductCategoryPosition:    in.ProductCategoryPosition,
		ProductTypeCategoryID:      in.ProductTypeCategoryID,
		ProductSubtypeCategoryID:   in.ProductSubtypeCategoryID,
		ProductTypeName:            in.ProductTypeName,
		ProductSubtypeName:         in.ProductSubtypeName,
		CategoryPrimaryName:        in.CategoryPrimaryName,
		CategoryPrimaryPosition:    in.CategoryPrimaryPosition,
		CategorySecondaryName:      in.CategorySecondaryName,
		CategorySecondaryPosition:  in.CategorySecondaryPosition,
		OperationTemplateID:        in.OperationTemplateID,
		InventoryUnit:              in.InventoryUnit,
		QuoteUnit:                  in.QuoteUnit,
		OrderUnit:                  in.OrderUnit,
		UnitConversionJSON:         in.UnitConversionJSON,
		IntegerUnit:                in.IntegerUnit,
		PriceListRuleJSON:          in.PriceListRuleJSON,
		ProductAttributes:          productAttributesFromSpecialAttrs(in.SpecialAttrsSchemaJSON, in.SpecialAttrsJSON),
		ProductPriceSnapshots:      append([]ProductPriceSnapshot(nil), in.ProductPriceSnapshots...),
		BeanListQuality:            in.BeanListQuality,
		GreenBeanList: BeanListDisplay{
			Code:           code,
			Category:       firstNonEmptyString(in.CategorySecondaryName, in.CategoryPrimaryName, "生豆销售"),
			DisplayName:    displayName,
			RecommendedUse: "生豆销售",
			Flavor:         in.Flavor,
			Description:    firstNonEmptyString(in.BeanListNote, in.Origin),
		},
		Flavor:               in.Flavor,
		Origin:               in.Origin,
		ProcessingStation:    in.ProcessingStation,
		Variety:              in.Variety,
		ProcessMethod:        in.ProcessMethod,
		Grade:                in.Grade,
		Altitude:             in.Altitude,
		BeanListNote:         in.BeanListNote,
		GreenBeanCostPerKg:   in.GreenBeanCostPerKg,
		BomCostPerUnit:       in.BomCostPerUnit,
		OperationCostPerUnit: in.OperationCostPerUnit,
		OperationCostPerKg:   in.OperationCostPerKg,
		BomStatus:            bomStatus,
		Warnings:             append([]string(nil), in.Warnings...),
		GreenBeanSaleTiers:   tiers,
	}
	if shouldWarnMissingPricingMethod(in) {
		out.Warnings = normalizeWarnings(append(out.Warnings, MissingPricingMethodWarning))
	}
	return out
}

func normalizeProductUnit(value string, fallback string) string {
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

func buildGreenBeanTemplateSaleTiers(params Parameters, in ProductInput) []CommercialWholesaleTier {
	template := normalizeGradientTemplate(in.GradientTemplate)
	if template == nil {
		return nil
	}
	out := make([]CommercialWholesaleTier, 0, len(template.Tiers))
	for _, tier := range template.Tiers {
		displayUnit := normalizeGradientDisplayUnit(template.DisplayUnit)
		specG := specGForGradientDisplayUnit(displayUnit)
		pricePerKg := roundPriceTo(in.GreenBeanCostPerKg, 2)
		pricePerLb := roundPriceTo(pricePerKg*params.KgToLbFactor, 2)
		pricePerUnit := pricePerKg
		switch displayUnit {
		case GradientDisplayUnitLb:
			pricePerUnit = pricePerLb
		case GradientDisplayUnitKg:
			pricePerUnit = pricePerKg
		default:
			pricePerUnit = roundPriceTo(pricePerKg*float64(specG)/1000.0, 2)
		}
		minQty := roundQuantity(tier.MinWeightG / float64(specG))
		var maxQty *float64
		if tier.MaxWeightG != nil {
			v := roundQuantity(*tier.MaxWeightG / float64(specG))
			maxQty = &v
		}
		minLb := roundQuantity(tier.MinWeightG / 454.0)
		var maxLb *float64
		if tier.MaxWeightG != nil {
			v := roundQuantity(*tier.MaxWeightG / 454.0)
			maxLb = &v
		}
		out = append(out, CommercialWholesaleTier{
			Label:          tier.Label,
			Scheme:         "green_bean_template",
			SpecG:          int64(specG),
			MinQty:         minQty,
			MaxQty:         maxQty,
			PricePerUnit:   pricePerUnit,
			MinLb:          minLb,
			MaxLb:          maxLb,
			PricePerKg:     pricePerKg,
			PricePerLb:     pricePerLb,
			TemplateID:     template.ID,
			TemplateTierID: tier.ID,
			DisplayUnit:    displayUnit,
			MinWeightG:     tier.MinWeightG,
			MaxWeightG:     tier.MaxWeightG,
			MarginRate:     0,
		})
	}
	return out
}

func normalizeLegacyGreenBeanSaleTiers(source []CommercialWholesaleTier) []CommercialWholesaleTier {
	tiers := make([]CommercialWholesaleTier, 0, len(source))
	for i, tier := range source {
		next := tier
		if strings.TrimSpace(next.Label) == "" {
			next.Label = greenBeanTierLabel(next)
		}
		if next.SpecG <= 0 {
			next.SpecG = 1000
		}
		if strings.TrimSpace(next.DisplayUnit) == "" {
			next.DisplayUnit = GradientDisplayUnitKg
		}
		if next.MinQty <= 0 {
			next.MinQty = float64(i + 1)
		}
		if next.PricePerUnit > 0 && next.PricePerLb == 0 {
			next.PricePerLb = next.PricePerUnit * 454.0 / float64(next.SpecG)
		}
		if next.PricePerUnit > 0 && next.PricePerKg == 0 {
			next.PricePerKg = next.PricePerUnit * 1000.0 / float64(next.SpecG)
		}
		next.Scheme = "green_bean_direct"
		tiers = append(tiers, next)
	}
	return tiers
}

func greenBeanTierLabel(tier CommercialWholesaleTier) string {
	min := tier.MinQty
	if min <= 0 {
		min = tier.MinLb
	}
	unit := "kg"
	switch tier.DisplayUnit {
	case GradientDisplayUnitLb:
		unit = "磅"
	case GradientDisplayUnit100G:
		unit = "100g"
	case GradientDisplayUnit227G:
		unit = "227g"
	case GradientDisplayUnit250G:
		unit = "250g"
	}
	if min <= 0 {
		return "全部数量"
	}
	return fmt.Sprintf("%s%s+", trimFloatZero(min), unit)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if s := strings.TrimSpace(value); s != "" {
			return s
		}
	}
	return ""
}

func customerCategoryBeanListDisplay(in ProductInput, display BeanListDisplay, allowFallback bool) BeanListDisplay {
	if display.Code == "" && !allowFallback {
		return display
	}
	categoryName := customerBeanListCategoryName(in)
	if categoryName == "" {
		if display.Code == "" {
			display.RecommendedUse = "客户定制"
		}
		display.Code = customerUnclassifiedBeanListCode(display.Code, in.ProductCategoryPosition, in.ProductID)
		display.Category = "未分类"
		display.DisplayName = in.Name
		if strings.TrimSpace(display.Flavor) == "" {
			display.Flavor = in.Flavor
		}
		if strings.TrimSpace(display.Description) == "" {
			display.Description = firstNonEmptyString(in.BeanListNote, in.Origin)
		}
		return display
	}
	categoryPosition := customerBeanListCategoryPosition(in)
	productPosition := in.ProductCategoryPosition
	if productPosition <= 0 && in.ProductID > 0 {
		productPosition = int(in.ProductID)
	}
	if categoryPosition <= 0 {
		categoryPosition = 1
	}
	if productPosition <= 0 {
		productPosition = 1
	}
	if display.Code == "" {
		display.RecommendedUse = "客户定制"
	}
	display.Code = fmt.Sprintf("%d.%d", categoryPosition, productPosition)
	display.Category = fmt.Sprintf("%d、%s", categoryPosition, categoryName)
	display.DisplayName = in.Name
	if strings.TrimSpace(display.Flavor) == "" {
		display.Flavor = in.Flavor
	}
	if strings.TrimSpace(display.Description) == "" {
		display.Description = firstNonEmptyString(in.BeanListNote, in.Origin)
	}
	return display
}

func hasSkuCategoryBeanListMetadata(in ProductInput) bool {
	_ = in // kept for backward reference; fallback now always applies in CalculateProduct
	return strings.TrimSpace(in.CategoryPrimaryName) != "" || strings.TrimSpace(in.CategorySecondaryName) != ""
}

func customerBeanListCategoryName(in ProductInput) string {
	primary := strings.TrimSpace(in.CategoryPrimaryName)
	secondary := strings.TrimSpace(in.CategorySecondaryName)
	return firstNonEmptyString(secondary, primary)
}

func customerBeanListCategoryPosition(in ProductInput) int {
	if strings.TrimSpace(in.CategorySecondaryName) != "" && in.CategorySecondaryPosition > 0 {
		return in.CategorySecondaryPosition
	}
	if in.CategoryPrimaryPosition > 0 {
		return in.CategoryPrimaryPosition
	}
	return 0
}

func customerUnclassifiedBeanListCode(sourceCode string, productCategoryPosition int, productID int64) string {
	itemPosition := secondBeanListCodePart(sourceCode)
	if itemPosition <= 0 {
		itemPosition = productCategoryPosition
	}
	if itemPosition <= 0 && productID > 0 {
		itemPosition = int(productID)
	}
	if itemPosition <= 0 {
		itemPosition = 1
	}
	return fmt.Sprintf("999.%d", itemPosition)
}

func secondBeanListCodePart(code string) int {
	parts := strings.Split(strings.TrimSpace(code), ".")
	if len(parts) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
	return n
}

func trimFloatZero(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func buildDripWholesaleTiers(params Parameters, in ProductInput) []DripWholesaleTier {
	template := normalizeDripPriceTemplate(in.DripPriceTemplate)
	bagGrams := in.DripBagGrams
	if bagGrams <= 0 {
		bagGrams = 10
	}
	boxBagCount := in.DripBoxBagCount
	if boxBagCount <= 0 {
		boxBagCount = 10
	}
	type tierDef struct {
		id         int64
		label      string
		minBags    float64
		maxBags    *float64
		multiplier float64
		position   int
	}
	defs := make([]tierDef, 0)
	var templateID int64
	includePackaging := true
	if template != nil {
		templateID = template.ID
		includePackaging = template.IncludePackaging
		for _, tier := range template.Tiers {
			defs = append(defs, tierDef{
				id:         tier.ID,
				label:      tier.Label,
				minBags:    tier.MinBags,
				maxBags:    tier.MaxBags,
				multiplier: tier.Multiplier,
				position:   tier.Position,
			})
		}
	}
	if len(defs) == 0 {
		for i, multiplier := range in.WholesaleDripMultipliers {
			defs = append(defs, tierDef{
				label:      fmt.Sprintf("%d袋", defaultDripWholesaleMinBags(i)),
				minBags:    float64(defaultDripWholesaleMinBags(i)),
				multiplier: multiplier,
				position:   i + 1,
			})
		}
	}
	sort.SliceStable(defs, func(i, j int) bool {
		if defs[i].position != defs[j].position {
			return defs[i].position < defs[j].position
		}
		return defs[i].minBags < defs[j].minBags
	})
	out := make([]DripWholesaleTier, 0, len(defs))
	for _, def := range defs {
		if def.minBags <= 0 || def.multiplier <= 0 {
			continue
		}
		base := dripBaseCostPerBag(params, in, bagGrams)
		loose := base*def.multiplier + base*(def.multiplier-1)*params.RetailTaxRate
		packed := loose
		if includePackaging {
			packed += params.DripPackingMaterialPerBag
		}
		label := strings.TrimSpace(def.label)
		if label == "" {
			label = fmt.Sprintf("%.0f袋", def.minBags)
		}
		out = append(out, DripWholesaleTier{
			Label:             label,
			MinBags:           int64(math.Ceil(def.minBags)),
			MaxBags:           def.maxBags,
			Multiplier:        def.multiplier,
			LoosePricePerBag:  loose,
			PackedPricePerBag: packed,
			TemplateID:        templateID,
			TemplateTierID:    def.id,
			BagGrams:          bagGrams,
			BoxBagCount:       boxBagCount,
			TaxRate:           params.RetailTaxRate,
		})
	}
	return out
}

func dripBaseCostPerBag(params Parameters, in ProductInput, bagGrams float64) float64 {
	yield := in.YieldRate
	if yield <= 0 {
		yield = params.RoastYieldRate
	}
	ratio := bagGrams / 1000.0
	if ratio <= 0 {
		ratio = params.DripGreenRatioKgPerBag
	}
	roasted := in.GreenBeanCostPerKg / yield
	small := roasted + params.SmallBatchProductionCostPerKg
	return small*ratio + params.DripProcessCostPerBag + params.DripExtraCostPerBag
}

func normalizeBomStatus(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return "active"
	}
	return status
}

func normalizeWarnings(warnings []string) []string {
	out := make([]string, 0, len(warnings))
	seen := map[string]bool{}
	for _, warning := range warnings {
		warning = strings.TrimSpace(warning)
		if warning == "" || seen[warning] {
			continue
		}
		out = append(out, warning)
		seen[warning] = true
	}
	return out
}

func buildCommercialWholesaleTiers(params Parameters, in ProductInput, kgPrices, lbPrices []float64) []CommercialWholesaleTier {
	if template := normalizeGradientTemplate(in.GradientTemplate); template != nil {
		return buildGradientTemplateCommercialTiers(params, in, *template)
	}
	return nil
}

func shouldWarnMissingPricingMethod(in ProductInput) bool {
	return !hasEffectivePricingMethod(in)
}

func hasEffectivePricingMethod(in ProductInput) bool {
	rule := parseProductPriceRuleJSON(in.PriceListRuleJSON)
	switch rule.PricingMode {
	case "fixed_unit_price":
		return rule.UnitPrice > 0 || len(rule.TierPrices) > 0
	case "cost_plus":
		return rule.HasCostPlusRate
	default:
		return normalizeGradientTemplate(in.GradientTemplate) != nil
	}
}

type commercialPriceParts struct {
	RoastedCostPerKg    float64
	BaseCostPerKg       float64
	ProductionCostPerKg float64
	ProductionKey       string
	MarginRate          float64
	TaxAddPerKg         float64
	RawPricePerKg       float64
	RawPricePerLb       float64
	FinalPricePerKg     float64
	FinalPricePerLb     float64
	FinalPricePerUnit   float64
	DisplayUnit         string
}

type productPriceRule struct {
	PricingMode     string
	DisplayUnit     string
	Rounding        string
	TaxIncluded     bool
	UnitPrice       float64
	CostPlusRate    float64
	HasCostPlusRate bool
	TierPrices      map[string]float64
	RawRuleJSON     string
}

func parseProductPriceRuleJSON(value string) productPriceRule {
	rule := productPriceRule{
		PricingMode: "inherit_gradient_template",
		DisplayUnit: "",
		Rounding:    "none",
		RawRuleJSON: strings.TrimSpace(value),
	}
	raw := map[string]any{}
	if err := json.Unmarshal([]byte(rule.RawRuleJSON), &raw); err != nil {
		return rule
	}
	rule.PricingMode = normalizeProductPriceRuleMode(stringValue(raw["pricing_mode"]), rule.PricingMode)
	rule.DisplayUnit = strings.TrimSpace(firstNonEmptyString(stringValue(raw["display_unit"]), stringValue(raw["display_mode"]), rule.DisplayUnit))
	rule.Rounding = normalizeProductPriceRuleRounding(stringValue(raw["rounding"]))
	rule.TaxIncluded = boolValue(raw["tax_included"])
	rule.UnitPrice = firstPositiveFloat(raw, "unit_price", "price_per_unit", "fixed_unit_price", "fixed_price")
	if rate, ok := firstNonNegativeFloat(raw, "cost_plus_rate", "markup_rate", "margin_rate"); ok {
		rule.CostPlusRate = rate
		rule.HasCostPlusRate = true
	}
	rule.TierPrices = map[string]float64{}
	for _, key := range []string{"tier_prices", "fixed_prices", "prices"} {
		if rows, ok := raw[key].(map[string]any); ok {
			for tierKey, value := range rows {
				if price := floatValue(value); price > 0 {
					rule.TierPrices[strings.TrimSpace(tierKey)] = price
				}
			}
		}
	}
	return rule
}

func normalizeProductPriceRuleMode(value string, fallback string) string {
	switch strings.TrimSpace(value) {
	case "fixed_unit_price", "cost_plus", "inherit_gradient_template":
		return strings.TrimSpace(value)
	default:
		if fallback != "" {
			return fallback
		}
		return "inherit_gradient_template"
	}
}

func normalizeProductPriceRuleRounding(value string) string {
	switch strings.TrimSpace(value) {
	case "yuan", "jiao":
		return strings.TrimSpace(value)
	default:
		return "none"
	}
}

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return ""
	}
}

func boolValue(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "yes":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

type specialAttrSchemaRow struct {
	Key             string   `json:"key"`
	Label           string   `json:"label"`
	ValueType       string   `json:"value_type"`
	Options         []string `json:"options"`
	Required        bool     `json:"required"`
	ShowInPriceList bool     `json:"show_in_price_list"`
	Position        int      `json:"position"`
}

func productAttributesFromSpecialAttrs(schemaJSON string, valuesJSON string) []ProductAttribute {
	var schema []specialAttrSchemaRow
	if err := json.Unmarshal([]byte(strings.TrimSpace(schemaJSON)), &schema); err != nil {
		return nil
	}
	values := map[string]any{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(valuesJSON)), &values); err != nil {
		return nil
	}
	out := make([]ProductAttribute, 0, len(schema))
	for index, row := range schema {
		if !row.ShowInPriceList {
			continue
		}
		key := strings.TrimSpace(row.Key)
		if key == "" {
			continue
		}
		value := specialAttrDisplayValue(values[key])
		if value == "" {
			continue
		}
		label := strings.TrimSpace(row.Label)
		if label == "" {
			label = key
		}
		position := row.Position
		if position <= 0 {
			position = index + 1
		}
		out = append(out, ProductAttribute{
			Key:      key,
			Label:    label,
			Value:    value,
			Position: position,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Position != out[j].Position {
			return out[i].Position < out[j].Position
		}
		if out[i].Label != out[j].Label {
			return out[i].Label < out[j].Label
		}
		return out[i].Key < out[j].Key
	})
	return out
}

func specialAttrDisplayValue(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(v, 'f', 6, 64), "0"), ".")
	case bool:
		if v {
			return "true"
		}
		return "false"
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if s := specialAttrDisplayValue(item); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "、")
	default:
		return ""
	}
}

func floatValue(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		n, _ := v.Float64()
		return n
	case string:
		n, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return n
	default:
		return 0
	}
}

func firstPositiveFloat(raw map[string]any, keys ...string) float64 {
	for _, key := range keys {
		if value := floatValue(raw[key]); value > 0 {
			return value
		}
	}
	return 0
}

func firstNonNegativeFloat(raw map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		value, ok := raw[key]
		if !ok {
			continue
		}
		numberValue := floatValue(value)
		if numberValue >= 0 {
			return numberValue, true
		}
	}
	return 0, false
}

func productPriceRuleTierPrice(rule productPriceRule, tier GradientTemplateTier) (float64, bool) {
	if rule.UnitPrice > 0 {
		return rule.UnitPrice, true
	}
	for _, key := range []string{
		strconv.FormatInt(tier.ID, 10),
		strings.TrimSpace(tier.Label),
	} {
		if key == "" || key == "0" {
			continue
		}
		if price := rule.TierPrices[key]; price > 0 {
			return price, true
		}
	}
	return 0, false
}

func shouldUseComposableProductPricing(in ProductInput, rule productPriceRule) bool {
	if in.BomCostPerUnit > 0 || in.OperationCostPerUnit > 0 || in.OperationCostPerKg > 0 {
		return true
	}
	switch rule.PricingMode {
	case "cost_plus", "fixed_unit_price":
		return true
	default:
		return false
	}
}

func effectivePriceRuleDisplayUnit(in ProductInput, template GradientTemplate, rule productPriceRule) string {
	displayUnit := strings.TrimSpace(rule.DisplayUnit)
	if displayUnit == "" || displayUnit == "inherit_quote_unit" || displayUnit == "inherit_gradient_template" {
		displayUnit = strings.TrimSpace(template.DisplayUnit)
	}
	if displayUnit == "" {
		displayUnit = strings.TrimSpace(in.QuoteUnit)
	}
	return normalizeGradientDisplayUnit(displayUnit)
}

func isLegacyGradientDisplayUnit(unit string) bool {
	switch strings.TrimSpace(unit) {
	case "":
		return true
	case GradientDisplayUnitKg, GradientDisplayUnitLb, GradientDisplayUnit227G, GradientDisplayUnit100G, GradientDisplayUnit250G:
		return true
	default:
		return false
	}
}

func quantityScaleForGradientDisplayUnit(unit string) float64 {
	if isLegacyGradientDisplayUnit(unit) {
		return float64(specGForGradientDisplayUnit(unit))
	}
	return 1
}

func physicalSpecGForDisplayUnit(unit string, conversionJSON string) int {
	if isLegacyGradientDisplayUnit(unit) {
		return specGForGradientDisplayUnit(unit)
	}
	grams := gramsForUnitFromConversion(unit, conversionJSON)
	if grams > 0 {
		return int64ToInt(math.Round(grams))
	}
	return 1
}

func composableCostPerDisplayUnit(in ProductInput, displayUnit string) float64 {
	cost := in.BomCostPerUnit + in.OperationCostPerUnit
	if in.OperationCostPerKg > 0 {
		specG := physicalSpecGForDisplayUnit(displayUnit, in.UnitConversionJSON)
		if specG > 0 {
			cost += in.OperationCostPerKg * float64(specG) / 1000.0
		}
	}
	return cost
}

func int64ToInt(v float64) int {
	if v <= 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return 1
	}
	maxInt := int(^uint(0) >> 1)
	if v > float64(maxInt) {
		return maxInt
	}
	return int(v)
}

func gramsForUnitFromConversion(unit string, conversionJSON string) float64 {
	unit = strings.TrimSpace(unit)
	if unit == "" {
		return 0
	}
	raw := map[string]map[string]any{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(conversionJSON)), &raw); err != nil {
		return 0
	}
	return gramsForUnitFromConversionMap(unit, raw, map[string]bool{})
}

func gramsForUnitFromConversionMap(unit string, conversion map[string]map[string]any, seen map[string]bool) float64 {
	unit = strings.TrimSpace(unit)
	if unit == "" || seen[unit] {
		return 0
	}
	seen[unit] = true
	targets := conversion[unit]
	for target, ratioValue := range targets {
		target = strings.TrimSpace(target)
		ratio := floatValue(ratioValue)
		if ratio <= 0 {
			continue
		}
		switch target {
		case "g", "克":
			return ratio
		case "kg", "公斤", "千克":
			return ratio * 1000
		case "lb", "磅":
			return ratio * 454
		default:
			if targetGrams := gramsForUnitFromConversionMap(target, conversion, seen); targetGrams > 0 {
				return ratio * targetGrams
			}
		}
	}
	return 0
}

func roundProductPriceByRule(value float64, rounding string) float64 {
	switch normalizeProductPriceRuleRounding(rounding) {
	case "yuan":
		return math.Round(value)
	case "jiao":
		return math.Round(value*10) / 10
	default:
		return roundPrice(value)
	}
}

func buildGradientTemplateCommercialTiers(params Parameters, in ProductInput, template GradientTemplate) []CommercialWholesaleTier {
	rule := parseProductPriceRuleJSON(in.PriceListRuleJSON)
	if shouldUseComposableProductPricing(in, rule) {
		return buildComposableProductCommercialTiers(params, in, template, rule)
	}
	out := make([]CommercialWholesaleTier, 0, len(template.Tiers))
	displayUnit := normalizeGradientDisplayUnit(template.DisplayUnit)
	for _, tier := range template.Tiers {
		parts := commercialPriceForGradientTier(params, in, displayUnit, tier, in.MarginRateOverride)
		specG := specGForGradientDisplayUnit(displayUnit)
		minQty := roundQuantity(tier.MinWeightG / float64(specG))
		var maxQty *float64
		if tier.MaxWeightG != nil {
			v := roundQuantity(*tier.MaxWeightG / float64(specG))
			maxQty = &v
		}
		minLb := roundQuantity(tier.MinWeightG / 454.0)
		var maxLb *float64
		if tier.MaxWeightG != nil {
			v := roundQuantity(*tier.MaxWeightG / 454.0)
			maxLb = &v
		}
		out = append(out, CommercialWholesaleTier{
			Label:          tier.Label,
			Scheme:         "gradient_template",
			SpecG:          int64(specG),
			MinQty:         minQty,
			MaxQty:         maxQty,
			PricePerUnit:   parts.FinalPricePerUnit,
			MinLb:          minLb,
			MaxLb:          maxLb,
			PricePerKg:     parts.FinalPricePerKg,
			PricePerLb:     parts.FinalPricePerLb,
			TemplateID:     template.ID,
			TemplateTierID: tier.ID,
			DisplayUnit:    parts.DisplayUnit,
			PriceUnit:      parts.DisplayUnit,
			MinWeightG:     tier.MinWeightG,
			MaxWeightG:     tier.MaxWeightG,
			MarginRate:     parts.MarginRate,
		})
	}
	return out
}

func buildComposableProductCommercialTiers(params Parameters, in ProductInput, template GradientTemplate, rule productPriceRule) []CommercialWholesaleTier {
	out := make([]CommercialWholesaleTier, 0, len(template.Tiers))
	displayUnit := effectivePriceRuleDisplayUnit(in, template, rule)
	specG := physicalSpecGForDisplayUnit(displayUnit, in.UnitConversionJSON)
	hasPhysicalSpec := isLegacyGradientDisplayUnit(displayUnit) || gramsForUnitFromConversion(displayUnit, in.UnitConversionJSON) > 0
	quantityScale := quantityScaleForGradientDisplayUnit(displayUnit)
	if quantityScale <= 0 {
		quantityScale = 1
	}
	for _, tier := range template.Tiers {
		margin := tier.MarginRate
		if in.MarginRateOverride != nil {
			margin = *in.MarginRateOverride
		}
		if rule.PricingMode == "cost_plus" && rule.HasCostPlusRate {
			margin = rule.CostPlusRate
		}
		pricePerUnit := composableCostPerDisplayUnit(in, displayUnit) * (1 + margin)
		if rule.PricingMode == "fixed_unit_price" {
			if fixedPrice, ok := productPriceRuleTierPrice(rule, tier); ok {
				pricePerUnit = fixedPrice
			}
		}
		pricePerUnit = roundProductPriceByRule(pricePerUnit, rule.Rounding)
		pricePerKg := 0.0
		pricePerLb := 0.0
		if hasPhysicalSpec && specG > 0 {
			pricePerKg = roundPrice(pricePerUnit * 1000.0 / float64(specG))
			pricePerLb = roundPrice(pricePerKg * params.KgToLbFactor)
		}
		minQty := roundQuantity(tier.MinWeightG / quantityScale)
		var maxQty *float64
		if tier.MaxWeightG != nil {
			v := roundQuantity(*tier.MaxWeightG / quantityScale)
			maxQty = &v
		}
		minWeightG := tier.MinWeightG
		if !isLegacyGradientDisplayUnit(displayUnit) {
			minWeightG = minQty * float64(specG)
		}
		maxWeightG := tier.MaxWeightG
		if !isLegacyGradientDisplayUnit(displayUnit) && maxQty != nil {
			v := *maxQty * float64(specG)
			maxWeightG = &v
		}
		minLb := roundQuantity(minWeightG / 454.0)
		var maxLb *float64
		if maxWeightG != nil {
			v := roundQuantity(*maxWeightG / 454.0)
			maxLb = &v
		}
		out = append(out, CommercialWholesaleTier{
			Label:          tier.Label,
			Scheme:         "gradient_template",
			SpecG:          int64(specG),
			MinQty:         minQty,
			MaxQty:         maxQty,
			PricePerUnit:   pricePerUnit,
			MinLb:          minLb,
			MaxLb:          maxLb,
			PricePerKg:     pricePerKg,
			PricePerLb:     pricePerLb,
			TemplateID:     template.ID,
			TemplateTierID: tier.ID,
			DisplayUnit:    displayUnit,
			PriceUnit:      displayUnit,
			MinWeightG:     minWeightG,
			MaxWeightG:     maxWeightG,
			MarginRate:     margin,
		})
	}
	return out
}

func commercialPriceForGradientTier(params Parameters, in ProductInput, displayUnit string, tier GradientTemplateTier, marginOverride *float64) commercialPriceParts {
	greenCost := in.GreenBeanCostPerKg
	yieldRate := in.YieldRate
	if yieldRate <= 0 {
		yieldRate = params.RoastYieldRate
	}
	roasted := greenCost / yieldRate
	productionCost := params.SmallBatchProductionCostPerKg
	productionKey := "small_batch_production_cost_per_kg"
	if tier.MinWeightG >= 10000 {
		productionCost = params.LargeBatchProductionCostPerKg
		productionKey = "large_batch_production_cost_per_kg"
	}
	base := roasted + productionCost
	margin := tier.MarginRate
	if marginOverride != nil {
		margin = *marginOverride
	}
	taxAdd := base * margin * params.RetailTaxRate
	rawKg := base*(1+margin) + params.WholesalePackageCostPerKg + params.ProductLossPerKg + taxAdd
	rawLb := rawKg*params.KgToLbFactor + 1
	finalKg := roundPrice(rawKg)
	finalLb := roundPrice(rawLb)
	normalizedDisplayUnit := normalizeGradientDisplayUnit(displayUnit)
	finalUnit := finalLb
	if normalizedDisplayUnit == GradientDisplayUnitKg {
		finalUnit = finalKg
	} else if normalizedDisplayUnit != GradientDisplayUnitLb {
		finalUnit = roundPrice(rawKg * float64(specGForGradientDisplayUnit(normalizedDisplayUnit)) / 1000.0)
	}
	return commercialPriceParts{
		RoastedCostPerKg:    roasted,
		BaseCostPerKg:       base,
		ProductionCostPerKg: productionCost,
		ProductionKey:       productionKey,
		MarginRate:          margin,
		TaxAddPerKg:         taxAdd,
		RawPricePerKg:       rawKg,
		RawPricePerLb:       rawLb,
		FinalPricePerKg:     finalKg,
		FinalPricePerLb:     finalLb,
		FinalPricePerUnit:   finalUnit,
		DisplayUnit:         normalizedDisplayUnit,
	}
}

func ExplainCommercialPrice(params Parameters, in ProductInput, req PriceExplanationRequest) (PriceExplanation, error) {
	validated, err := ValidateProductInput(params, in)
	if err != nil {
		return PriceExplanation{}, err
	}
	template := normalizeGradientTemplate(validated.GradientTemplate)
	if template == nil {
		return PriceExplanation{}, fmt.Errorf("gradient template required")
	}
	label := strings.TrimSpace(req.TierLabel)
	var tier GradientTemplateTier
	found := false
	for _, row := range template.Tiers {
		if label == "" || row.Label == label {
			tier = row
			found = true
			break
		}
	}
	if !found {
		return PriceExplanation{}, fmt.Errorf("tier not found")
	}
	saved := commercialPriceForGradientTier(params, validated, template.DisplayUnit, tier, validated.MarginRateOverride)
	previewInput := validated
	if req.Overrides.GreenBeanCostPerKg != nil {
		previewInput.GreenBeanCostPerKg = *req.Overrides.GreenBeanCostPerKg
	}
	if req.Overrides.YieldRate != nil {
		previewInput.YieldRate = *req.Overrides.YieldRate
	}
	previewMarginOverride := validated.MarginRateOverride
	if req.Overrides.MarginRate != nil {
		previewMarginOverride = req.Overrides.MarginRate
	}
	preview := commercialPriceForGradientTier(params, previewInput, template.DisplayUnit, tier, previewMarginOverride)
	marginSource := "gradient_template"
	if validated.MarginRateOverride != nil {
		marginSource = "product_margin_override"
	}
	if req.Overrides.MarginRate != nil {
		marginSource = "temporary_override"
	}
	return PriceExplanation{
		ProductID:         validated.ProductID,
		ProductName:       validated.Name,
		TemplateID:        template.ID,
		TemplateName:      template.Name,
		TierLabel:         tier.Label,
		DisplayUnit:       template.DisplayUnit,
		MinWeightG:        tier.MinWeightG,
		MaxWeightG:        tier.MaxWeightG,
		SavedFinalPrice:   saved.FinalPricePerUnit,
		PreviewFinalPrice: preview.FinalPricePerUnit,
		Steps: []PriceExplanationStep{
			{Key: "green_bean_cost_per_kg", Label: "生豆成本", Source: "product", Value: previewInput.GreenBeanCostPerKg, Unit: "元/kg", Changed: req.Overrides.GreenBeanCostPerKg != nil},
			{Key: "expected_loss_rate", Label: "预期损耗率", Source: "product_production_config", Value: 1 - previewInput.YieldRate, Unit: "ratio", Changed: req.Overrides.YieldRate != nil},
			{Key: "roasted_bean_cost_per_kg", Label: "熟豆成本", Source: "formula", Value: preview.RoastedCostPerKg, Unit: "元/kg", Changed: req.Overrides.GreenBeanCostPerKg != nil || req.Overrides.YieldRate != nil},
			{Key: preview.ProductionKey, Label: "生产成本", Source: "cost_parameter", Value: preview.ProductionCostPerKg, Unit: "元/kg"},
			{Key: "wholesale_package_cost_per_kg", Label: "批发包装成本", Source: "cost_parameter", Value: params.WholesalePackageCostPerKg, Unit: "元/kg"},
			{Key: "product_loss_per_kg", Label: "产品损耗", Source: "cost_parameter", Value: params.ProductLossPerKg, Unit: "元/kg"},
			{Key: "retail_tax_rate", Label: "税费比例", Source: "cost_parameter", Value: params.RetailTaxRate, Unit: "ratio"},
			{Key: "template_margin_rate", Label: "利润率", Source: marginSource, Value: preview.MarginRate, Unit: "ratio", Changed: req.Overrides.MarginRate != nil},
			{Key: "display_unit_conversion", Label: "展示单位克重", Source: "gradient_template", Value: float64(specGForGradientDisplayUnit(template.DisplayUnit)), Unit: "g"},
			{Key: "final_price", Label: "最终价格", Source: "formula", Value: preview.FinalPricePerUnit, Unit: template.DisplayUnit, Changed: preview.FinalPricePerUnit != saved.FinalPricePerUnit},
		},
	}, nil
}

func ExplainDripPrice(params Parameters, in ProductInput, req PriceExplanationRequest) (DripPriceExplanation, error) {
	validated, err := ValidateProductInput(params, in)
	if err != nil {
		return DripPriceExplanation{}, err
	}
	if normalizeProductKind(validated.ProductKind) != "drip_bag" {
		return DripPriceExplanation{}, fmt.Errorf("drip_bag product required")
	}
	tierLabel := strings.TrimSpace(req.TierLabel)
	tiers := buildDripWholesaleTiers(params, validated)
	if len(tiers) == 0 {
		return DripPriceExplanation{}, fmt.Errorf("drip price tier required")
	}
	tier := tiers[0]
	if tierLabel != "" {
		found := false
		for _, row := range tiers {
			if row.Label == tierLabel {
				tier = row
				found = true
				break
			}
		}
		if !found {
			return DripPriceExplanation{}, fmt.Errorf("tier not found")
		}
	}
	boxBagCount := tier.BoxBagCount
	if boxBagCount <= 0 {
		boxBagCount = validated.DripBoxBagCount
	}
	if boxBagCount <= 0 {
		boxBagCount = 10
	}
	roasted := validated.GreenBeanCostPerKg / validated.YieldRate
	small := roasted + params.SmallBatchProductionCostPerKg
	base := dripBaseCostPerBag(params, validated, tier.BagGrams)
	minBoxes := int64(math.Ceil(float64(tier.MinBags) / float64(boxBagCount)))
	packedBox := tier.PackedPricePerBag * float64(boxBagCount)
	return DripPriceExplanation{
		ProductID:         validated.ProductID,
		ProductName:       validated.Name,
		TemplateID:        tier.TemplateID,
		TemplateTierID:    tier.TemplateTierID,
		TemplateName:      dripTemplateName(validated.DripPriceTemplate),
		TierLabel:         tier.Label,
		BagGrams:          tier.BagGrams,
		BoxBagCount:       boxBagCount,
		MinBags:           tier.MinBags,
		MinBoxes:          minBoxes,
		LoosePricePerBag:  tier.LoosePricePerBag,
		PackedPricePerBag: tier.PackedPricePerBag,
		PackedPricePerBox: packedBox,
		Steps: []PriceExplanationStep{
			{Key: "green_bean_cost_per_kg", Label: "生豆成本", Source: "product", Value: validated.GreenBeanCostPerKg, Unit: "元/kg"},
			{Key: "expected_loss_rate", Label: "预期损耗率", Source: "product_production_config", Value: 1 - validated.YieldRate, Unit: "ratio"},
			{Key: "roasted_bean_cost_per_kg", Label: "熟豆成本", Source: "formula", Value: roasted, Unit: "元/kg"},
			{Key: "small_batch_production_cost_per_kg", Label: "小批量生产成本", Source: "cost_parameter", Value: params.SmallBatchProductionCostPerKg, Unit: "元/kg"},
			{Key: "bag_grams", Label: "单袋熟豆克重", Source: "product_or_template", Value: tier.BagGrams, Unit: "g"},
			{Key: "drip_roasted_cost_per_bag", Label: "单袋熟豆成本", Source: "formula", Value: small * tier.BagGrams / 1000.0, Unit: "元/袋"},
			{Key: "drip_process_cost_per_bag", Label: "挂耳加工成本", Source: "cost_parameter", Value: params.DripProcessCostPerBag, Unit: "元/袋"},
			{Key: "drip_extra_cost_per_bag", Label: "挂耳额外成本", Source: "cost_parameter", Value: params.DripExtraCostPerBag, Unit: "元/袋"},
			{Key: "drip_base_cost_per_bag", Label: "单袋基础成本", Source: "formula", Value: base, Unit: "元/袋"},
			{Key: "template_multiplier", Label: "供应价倍率", Source: "drip_price_template", Value: tier.Multiplier, Unit: "ratio"},
			{Key: "retail_tax_rate", Label: "税费比例", Source: "cost_parameter", Value: params.RetailTaxRate, Unit: "ratio"},
			{Key: "drip_packing_material_per_bag", Label: "挂耳包装材料", Source: "cost_parameter", Value: params.DripPackingMaterialPerBag, Unit: "元/袋"},
			{Key: "packed_price_per_bag", Label: "含包装单袋价", Source: "formula", Value: tier.PackedPricePerBag, Unit: "元/袋"},
			{Key: "box_conversion", Label: "盒装换算", Source: "product", Value: packedBox, Unit: "元/盒"},
		},
	}, nil
}

func normalizeGradientTemplate(template *GradientTemplate) *GradientTemplate {
	if template == nil || len(template.Tiers) == 0 {
		return nil
	}
	out := *template
	out.Name = strings.TrimSpace(out.Name)
	out.DisplayUnit = normalizeGradientDisplayUnit(out.DisplayUnit)
	out.Tiers = append([]GradientTemplateTier(nil), template.Tiers...)
	sort.SliceStable(out.Tiers, func(i, j int) bool {
		if out.Tiers[i].Position != out.Tiers[j].Position {
			return out.Tiers[i].Position < out.Tiers[j].Position
		}
		if out.Tiers[i].MinWeightG != out.Tiers[j].MinWeightG {
			return out.Tiers[i].MinWeightG < out.Tiers[j].MinWeightG
		}
		return out.Tiers[i].ID < out.Tiers[j].ID
	})
	filtered := make([]GradientTemplateTier, 0, len(out.Tiers))
	for _, tier := range out.Tiers {
		tier.Label = strings.TrimSpace(tier.Label)
		if tier.Label == "" {
			tier.Label = fmt.Sprintf("%.0fg+", tier.MinWeightG)
		}
		if tier.MinWeightG <= 0 || tier.MarginRate < 0 {
			continue
		}
		if tier.MaxWeightG != nil && *tier.MaxWeightG <= tier.MinWeightG {
			continue
		}
		filtered = append(filtered, tier)
	}
	if len(filtered) == 0 {
		return nil
	}
	out.Tiers = filtered
	return &out
}

func normalizeDripPriceTemplate(template *DripPriceTemplate) *DripPriceTemplate {
	if template == nil || len(template.Tiers) == 0 {
		return nil
	}
	out := *template
	out.Name = strings.TrimSpace(out.Name)
	if out.BagGrams <= 0 {
		out.BagGrams = 10
	}
	if out.BoxBagCount <= 0 {
		out.BoxBagCount = 10
	}
	out.Tiers = append([]DripPriceTemplateTier(nil), template.Tiers...)
	sort.SliceStable(out.Tiers, func(i, j int) bool {
		if out.Tiers[i].Position != out.Tiers[j].Position {
			return out.Tiers[i].Position < out.Tiers[j].Position
		}
		if out.Tiers[i].MinBags != out.Tiers[j].MinBags {
			return out.Tiers[i].MinBags < out.Tiers[j].MinBags
		}
		return out.Tiers[i].ID < out.Tiers[j].ID
	})
	filtered := make([]DripPriceTemplateTier, 0, len(out.Tiers))
	for _, tier := range out.Tiers {
		tier.Label = strings.TrimSpace(tier.Label)
		if tier.Label == "" {
			tier.Label = fmt.Sprintf("%.0f袋", tier.MinBags)
		}
		if tier.MinBags <= 0 || tier.Multiplier <= 0 {
			continue
		}
		if tier.MaxBags != nil && *tier.MaxBags <= tier.MinBags {
			continue
		}
		filtered = append(filtered, tier)
	}
	if len(filtered) == 0 {
		return nil
	}
	out.Tiers = filtered
	return &out
}

func dripTemplateName(template *DripPriceTemplate) string {
	if template == nil {
		return ""
	}
	return strings.TrimSpace(template.Name)
}

func normalizeGradientDisplayUnit(unit string) string {
	value := strings.TrimSpace(unit)
	switch value {
	case GradientDisplayUnitKg:
		return GradientDisplayUnitKg
	case GradientDisplayUnit227G:
		return GradientDisplayUnit227G
	case GradientDisplayUnit100G:
		return GradientDisplayUnit100G
	case GradientDisplayUnit250G:
		return GradientDisplayUnit250G
	case GradientDisplayUnitLb:
		return GradientDisplayUnitLb
	default:
		if value != "" {
			return value
		}
		return GradientDisplayUnitLb
	}
}

func specGForGradientDisplayUnit(unit string) int {
	switch normalizeGradientDisplayUnit(unit) {
	case GradientDisplayUnitKg:
		return 1000
	case GradientDisplayUnitLb:
		return 454
	case GradientDisplayUnit227G:
		return 227
	case GradientDisplayUnit100G:
		return 100
	case GradientDisplayUnit250G:
		return 250
	default:
		return 1
	}
}

func roundQuantity(v float64) float64 {
	return math.Round(v*100) / 100
}

func costingProfileName(in ProductInput) string {
	name := strings.TrimSpace(in.BeanListTemplateName)
	if name != "" {
		return name
	}
	return in.Name
}

func buildRetailBeanTiers(name string, out ProductResult) []RetailBeanTier {
	if isRetailSmallPackBean(name) {
		return []RetailBeanTier{
			{Label: "100g", SpecG: 100, PricePerUnit: out.Retail100gPrice},
			{Label: "200g", SpecG: 200, PricePerUnit: out.Retail200gPrice},
		}
	}
	return []RetailBeanTier{
		{Label: "227g", SpecG: 227, PricePerUnit: out.Retail227gPrice},
		{Label: "250g", SpecG: 250, PricePerUnit: out.Retail250gPrice},
	}
}

func roundProductPrices(out *ProductResult) {
	for i := range out.WholesaleKgPrices {
		out.WholesaleKgPrices[i] = roundPrice(out.WholesaleKgPrices[i])
	}
	for i := range out.WholesaleLbPrices {
		out.WholesaleLbPrices[i] = roundPrice(out.WholesaleLbPrices[i])
	}
	for i := range out.CommercialWholesaleTiers {
		if isLegacyGradientDisplayUnit(out.CommercialWholesaleTiers[i].DisplayUnit) {
			out.CommercialWholesaleTiers[i].PricePerKg = roundPrice(out.CommercialWholesaleTiers[i].PricePerKg)
			out.CommercialWholesaleTiers[i].PricePerLb = roundPrice(out.CommercialWholesaleTiers[i].PricePerLb)
			out.CommercialWholesaleTiers[i].PricePerUnit = roundPrice(out.CommercialWholesaleTiers[i].PricePerUnit)
		} else {
			out.CommercialWholesaleTiers[i].PricePerKg = roundPriceTo(out.CommercialWholesaleTiers[i].PricePerKg, 2)
			out.CommercialWholesaleTiers[i].PricePerLb = roundPriceTo(out.CommercialWholesaleTiers[i].PricePerLb, 2)
			out.CommercialWholesaleTiers[i].PricePerUnit = roundPriceTo(out.CommercialWholesaleTiers[i].PricePerUnit, 2)
		}
	}
	for i := range out.WholesaleDripBagPrices {
		out.WholesaleDripBagPrices[i] = roundPrice(out.WholesaleDripBagPrices[i])
	}
	for i := range out.WholesaleDripBagWithPackPrices {
		out.WholesaleDripBagWithPackPrices[i] = roundPrice(out.WholesaleDripBagWithPackPrices[i])
	}
	for i := range out.DripWholesaleTiers {
		out.DripWholesaleTiers[i].LoosePricePerBag = roundPrice(out.DripWholesaleTiers[i].LoosePricePerBag)
		out.DripWholesaleTiers[i].PackedPricePerBag = roundPrice(out.DripWholesaleTiers[i].PackedPricePerBag)
	}
	out.RetailKgPrice = roundPrice(out.RetailKgPrice)
	out.Retail454gPrice = roundPrice(out.Retail454gPrice)
	out.Retail227gPrice = roundPrice(out.Retail227gPrice)
	out.Retail250gPrice = roundPrice(out.Retail250gPrice)
	out.Retail200gPrice = roundPrice(out.Retail200gPrice)
	out.Retail100gPrice = roundPrice(out.Retail100gPrice)
	out.RetailDrip10BagPrice = roundPrice(out.RetailDrip10BagPrice)
	for i := range out.RetailBeanTiers {
		out.RetailBeanTiers[i].PricePerUnit = roundPrice(out.RetailBeanTiers[i].PricePerUnit)
	}
}

func roundPrice(v float64) float64 {
	return math.Round(v)
}

func roundPriceTo(v float64, precision int) float64 {
	if precision <= 0 {
		return roundPrice(v)
	}
	pow := math.Pow10(precision)
	return math.Round(v*pow) / pow
}

func ptrFloat64(v float64) *float64 {
	return &v
}

func defaultDripWholesaleMinBags(index int) int64 {
	tiers := []int64{100, 1000, 5000, 10000}
	if index >= 0 && index < len(tiers) {
		return tiers[index]
	}
	return tiers[len(tiers)-1]
}

func normalizeWholesaleTierScheme(s string) string {
	switch strings.TrimSpace(s) {
	case WholesaleTierSchemeKgThree:
		return WholesaleTierSchemeKgThree
	case WholesaleTierScheme227GTwo:
		return WholesaleTierScheme227GTwo
	default:
		return WholesaleTierScheme454GFour
	}
}

func inferWholesaleTierScheme(name string) string {
	switch {
	case isCookieBlend(name):
		return WholesaleTierSchemeKgThree
	case containsAnyNormalized(name, []string{"白月光", "芸上莓梦", "晨曦", "晚香玉"}):
		return WholesaleTierScheme227GTwo
	default:
		return WholesaleTierScheme454GFour
	}
}

func normalizeWholesaleMarginRates(params Parameters, rates []float64) []float64 {
	fallback := []float64{0.5421052631578949, 0.3842105263157895, 0.27894736842105267, 0.2, 0.12, 0.045}
	for i := range fallback {
		if i < len(params.WholesaleKgMarginRates) && params.WholesaleKgMarginRates[i] > 0 {
			fallback[i] = params.WholesaleKgMarginRates[i]
		}
	}
	out := append([]float64(nil), fallback...)
	for i := range rates {
		if i < len(out) {
			out[i] = rates[i]
		} else {
			out = append(out, rates[i])
		}
	}
	return out
}

func premiumWholesaleMarginRates(params Parameters) []float64 {
	base := normalizeWholesaleMarginRates(params, nil)
	out := append([]float64(nil), base...)
	for i := 0; i < 4 && i < len(out); i++ {
		out[i] = out[i] * 1.5
	}
	return out
}

func premiumFirstThreeWholesaleMarginRates(params Parameters) []float64 {
	base := normalizeWholesaleMarginRates(params, nil)
	out := append([]float64(nil), base...)
	for i := 0; i < 3 && i < len(out); i++ {
		out[i] = out[i] * 1.5
	}
	return out
}

func wineSunWholesaleMarginRates(params Parameters) []float64 {
	out := normalizeWholesaleMarginRates(params, nil)
	if len(out) > 3 {
		out[3] = 0.17
	}
	return out
}

func cookieWholesaleMarginRates(params Parameters) []float64 {
	out := normalizeWholesaleMarginRates(params, nil)
	out[3] = 0.175
	out[4] = 0.12
	out[5] = 0.045
	return out
}

func defaultWholesaleTaxAddPerKgTiers(params Parameters, in ProductInput) []float64 {
	rates := defaultWholesaleTaxMarginRates(params)
	roasted := in.GreenBeanCostPerKg / in.YieldRate
	small := roasted + params.SmallBatchProductionCostPerKg
	out := make([]float64, len(in.WholesaleKgMarginRates))
	for i := range out {
		rate := rates[0]
		if i < len(rates) {
			rate = rates[i]
		}
		out[i] = small * rate * params.RetailTaxRate
	}
	return out
}

func defaultWholesaleTaxMarginRates(params Parameters) []float64 {
	rates := normalizeWholesaleMarginRates(params, nil)
	if len(rates) > 3 {
		rates[3] = 0.15
	}
	return rates
}

func isCookieBlend(name string) bool {
	return containsAnyNormalized(name, []string{"曲奇"})
}

func isWineSunBean(name string) bool {
	return containsAnyNormalized(name, []string{"酒心巧克力"})
}

func isYirgacheffeG2(name string) bool {
	return containsAnyNormalized(name, []string{"耶加雪菲G2", "耶加雪菲g2"})
}

func isPremiumCommercialBean(name string) bool {
	return containsAnyNormalized(name, []string{
		"Nenka", "嫩咖",
		"TOPAA", "肯尼亚",
		"浣纱", "果园",
		"Uraga", "乌拉嘎",
		"曼特宁", "森林瑰夏", "白月光", "芸上莓梦", "晨曦", "晚香玉",
		"萨奇姆", "芒霜", "小菠萝", "菠萝意式",
	})
}

func isMorningNayi(name string) bool {
	return containsAnyNormalized(name, []string{"晨曦", "娜伊", "娜依"})
}

func isRetailSmallPackBean(name string) bool {
	return containsAnyNormalized(name, []string{"白月光", "芸上莓梦", "晨曦", "晚香玉"})
}

func excelRetailGreenCostOverride(name string) (float64, bool) {
	if containsAnyNormalized(name, []string{"红岩"}) {
		return 57.9, true
	}
	return 0, false
}

func applyCommercialTierOverrides(name string, tiers []CommercialWholesaleTier) []CommercialWholesaleTier {
	keep := map[string]bool{}
	switch {
	case containsAnyNormalized(name, []string{"浣纱", "肯尼亚", "萨奇姆"}):
		keep["2包-13包"] = true
		keep["14包-23包"] = true
		keep["48包+"] = true
	}
	if len(keep) > 0 {
		filtered := make([]CommercialWholesaleTier, 0, len(keep))
		for _, tier := range tiers {
			if keep[tier.Label] {
				filtered = append(filtered, tier)
			}
		}
		tiers = filtered
	}
	if containsAnyNormalized(name, []string{"浣纱"}) {
		for i := range tiers {
			if tiers[i].Label == "48包+" {
				tiers[i].PricePerUnit = 100
				tiers[i].PricePerLb = 100
				tiers[i].PricePerKg = (100 - 1) / 0.454
			}
		}
	}
	return tiers
}

func isGreenBeanCategory(in ProductInput) bool {
	return strings.Contains(strings.ToLower(in.CategoryPrimaryName), "生豆") ||
		strings.Contains(strings.ToLower(in.CategorySecondaryName), "生豆") ||
		strings.Contains(strings.ToLower(in.ProductTypeName), "生豆") ||
		strings.Contains(strings.ToLower(in.ProductSubtypeName), "生豆")
}

func buildCategoryGreenBeanListDisplay(in ProductInput, fallback BeanListDisplay) BeanListDisplay {
	code := fallback.Code
	// 为生豆列表生成 G.xxx 格式编号，使用产品ID保证唯一性
	if !strings.HasPrefix(code, "G.") {
		if in.ProductID > 0 {
			code = fmt.Sprintf("G.%d", in.ProductID)
		}
	}
	category := firstNonEmptyString(in.CategorySecondaryName, in.CategoryPrimaryName, "生豆销售")
	displayName := fallback.DisplayName
	if displayName == "" {
		displayName = in.Name
	}
	flavor := fallback.Flavor
	if flavor == "" {
		flavor = in.Flavor
	}
	description := fallback.Description
	if description == "" {
		description = firstNonEmptyString(in.BeanListNote, in.Origin)
	}
	return BeanListDisplay{
		Code:           code,
		Category:       category,
		DisplayName:    displayName,
		RecommendedUse: firstNonEmptyString(fallback.RecommendedUse, "生豆销售"),
		Flavor:         flavor,
		Description:    description,
	}
}

func containsAnyNormalized(s string, needles []string) bool {
	n := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), " ", ""))
	for _, needle := range needles {
		if strings.Contains(n, strings.ToLower(strings.ReplaceAll(needle, " ", ""))) {
			return true
		}
	}
	return false
}
