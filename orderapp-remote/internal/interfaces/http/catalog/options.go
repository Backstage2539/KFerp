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
}

type APIOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
