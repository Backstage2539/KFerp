package catalog

type Option struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type ProductTierOption struct {
	ID        int64    `json:"id"`
	SpecG     int64    `json:"spec_g"`
	MinQty    float64  `json:"min_qty"`
	MaxQty    *float64 `json:"max_qty"`
	UnitPrice float64  `json:"unit_price"`
}

type ProductOption struct {
	ID                          int64               `json:"id"`
	SKUID                       int64               `json:"sku_id"`
	ParentProductID             int64               `json:"parent_product_id"`
	EffectiveParentProductID    int64               `json:"effective_parent_product_id"`
	SKUName                     string              `json:"sku_name"`
	SKUCode                     string              `json:"sku_code"`
	Barcode                     string              `json:"barcode"`
	SpecLabel                   string              `json:"spec_label"`
	NetContentQty               float64             `json:"net_content_qty"`
	NetContentUnit              string              `json:"net_content_unit"`
	IsDefaultSKU                bool                `json:"is_default_sku"`
	DefaultSKUID                int64               `json:"default_sku_id"`
	EffectiveDefaultSKUID       int64               `json:"effective_default_sku_id"`
	DefaultSpecLabel            string              `json:"default_spec_label"`
	AutoDerivedSKU              bool                `json:"auto_derived_sku"`
	DerivedUnitTemplateID       int64               `json:"derived_unit_template_id"`
	DerivedSpecKey              string              `json:"derived_spec_key"`
	DerivedSpecName             string              `json:"derived_spec_name"`
	DerivedSalesUnit            string              `json:"derived_sales_unit"`
	DerivedSpecStatus           string              `json:"derived_spec_status"`
	Name                        string              `json:"name"`
	Remark                      string              `json:"remark"`
	ProductKind                 string              `json:"product_kind"`
	GreenBeanType               string              `json:"green_bean_type"`
	GreenBeanBomProductID       int64               `json:"green_bean_bom_product_id"`
	RoastLevel                  string              `json:"roast_level"`
	SpecialAttrsJSON            string              `json:"special_attrs_json"`
	DripBagGrams                float64             `json:"drip_bag_grams"`
	DripBoxBagCount             int                 `json:"drip_box_bag_count"`
	AllowFulfillmentOrder       bool                `json:"allow_fulfillment_order"`
	AllowMallOrder              bool                `json:"allow_mall_order"`
	SalesUnits                  []string            `json:"sales_units"`
	DefaultPrice                float64             `json:"default_price"`
	RetailPrice100G             float64             `json:"retail_price_100g"`
	RetailPrice200G             float64             `json:"retail_price_200g"`
	RetailPrice227G             float64             `json:"retail_price_227g"`
	RetailPrice250G             float64             `json:"retail_price_250g"`
	YieldRate                   float64             `json:"yield_rate"`
	ProductCategoryID           int64               `json:"product_category_id"`
	ProductCategoryPosition     int                 `json:"product_category_position"`
	ClassificationTemplateID    int64               `json:"classification_template_id"`
	CustomerID                  int64               `json:"customer_id"`
	BaseProductID               int64               `json:"base_product_id"`
	Visibility                  string              `json:"visibility"`
	CustomType                  string              `json:"custom_type"`
	MarginRateOverride          *float64            `json:"margin_rate_override"`
	GradientTemplateIDOverride  int64               `json:"gradient_template_id_override"`
	OperationTemplateIDOverride int64               `json:"operation_template_id_override"`
	UnitRuleOverrideJSON        string              `json:"unit_rule_override_json"`
	InventoryUnit               string              `json:"inventory_unit"`
	IntegerInventoryUnit        bool                `json:"integer_inventory_unit"`
	DefaultSalesUnit            string              `json:"default_sales_unit"`
	UnitConversionJSON          string              `json:"unit_conversion_json"`
	SalesUnitRulesJSON          string              `json:"sales_unit_rules"`
	UnitTemplateID              int64               `json:"unit_template_id"`
	UnitTemplateName            string              `json:"unit_template_name"`
	UnitRuleSource              string              `json:"unit_rule_source"`
	ProductConfigTemplateID     int64               `json:"product_config_template_id"`
	BomItemCount                int                 `json:"bom_item_count"`
	BomStatus                   string              `json:"bom_status"`
	BomSourceType               string              `json:"bom_source_type"`
	EffectiveProductID          int64               `json:"effective_product_id"`
	EffectiveBomVersionID       int64               `json:"effective_bom_version_id"`
	SourceProductID             int64               `json:"source_product_id"`
	SourceProductCode           string              `json:"source_product_code"`
	SourceProductName           string              `json:"source_product_name"`
	SourceBomVersionID          int64               `json:"source_bom_version_id"`
	SourceBomVersionNo          string              `json:"source_bom_version_no"`
	DerivedFromLabel            string              `json:"derived_from_label"`
	CanEditBOM                  bool                `json:"can_edit_bom"`
	ProductionBomID             int64               `json:"production_bom_id"`
	ProductionBomCode           string              `json:"production_bom_code"`
	ProductionBomName           string              `json:"production_bom_name"`
	ProductionBomVersionID      int64               `json:"production_bom_version_id"`
	ProductionBomVersionNo      string              `json:"production_bom_version_no"`
	LatestBomVersionID          int64               `json:"latest_bom_version_id"`
	LatestBomVersionNo          string              `json:"latest_bom_version_no"`
	IsLatestBomVersion          bool                `json:"is_latest_bom_version"`
	ProductionBomGroupID        int64               `json:"production_bom_group_id"`
	ProductionBomGroupName      string              `json:"production_bom_group_name"`
	OrderUsageCount             int                 `json:"order_usage_count"`
	RetailSpecs                 []int64             `json:"retail_specs"`
	Tiers                       []ProductTierOption `json:"tiers"`
	SpecIdentityMode            string              `json:"spec_identity_mode"`
	BomSpecAuthoritative        bool                `json:"bom_spec_authoritative"`
	MigrationState              string              `json:"migration_state"`
	LegacyCatalogProduct        bool                `json:"legacy_catalog_product"`
	BOMSpecs                    []BOMSpecOption     `json:"bom_specs"`
}

type BOMSpecOption struct {
	ProductID     int64  `json:"product_id"`
	BomID         int64  `json:"bom_id"`
	BomVersionID  int64  `json:"bom_version_id"`
	BomVersionNo  string `json:"bom_version_no"`
	BomSpecID     int64  `json:"bom_spec_id"`
	BomVariantID  int64  `json:"bom_variant_id"`
	SpecCode      string `json:"spec_code"`
	Barcode       string `json:"barcode"`
	SpecKey       string `json:"spec_key"`
	SpecName      string `json:"spec_name"`
	InventoryUnit string `json:"inventory_unit"`
	IsDefault     bool   `json:"is_default"`
	SortOrder     int    `json:"sort_order"`
}

type APIOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
