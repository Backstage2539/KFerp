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
	ID                      int64               `json:"id"`
	Name                    string              `json:"name"`
	ProductKind             string              `json:"product_kind"`
	GreenBeanType           string              `json:"green_bean_type"`
	GreenBeanBomProductID   int64               `json:"green_bean_bom_product_id"`
	RoastLevel              string              `json:"roast_level"`
	DripBagGrams            float64             `json:"drip_bag_grams"`
	DripBoxBagCount         int                 `json:"drip_box_bag_count"`
	AllowFulfillmentOrder   bool                `json:"allow_fulfillment_order"`
	AllowMallOrder          bool                `json:"allow_mall_order"`
	SalesUnits              []string            `json:"sales_units"`
	DefaultPrice            float64             `json:"default_price"`
	RetailPrice100G         float64             `json:"retail_price_100g"`
	RetailPrice200G         float64             `json:"retail_price_200g"`
	RetailPrice227G         float64             `json:"retail_price_227g"`
	RetailPrice250G         float64             `json:"retail_price_250g"`
	YieldRate               float64             `json:"yield_rate"`
	ProductCategoryID       int64               `json:"product_category_id"`
	ProductCategoryPosition int                 `json:"product_category_position"`
	CustomerID              int64               `json:"customer_id"`
	BaseProductID           int64               `json:"base_product_id"`
	Visibility              string              `json:"visibility"`
	CustomType              string              `json:"custom_type"`
	MarginRateOverride      *float64            `json:"margin_rate_override"`
	BomItemCount            int                 `json:"bom_item_count"`
	BomStatus               string              `json:"bom_status"`
	RetailSpecs             []int64             `json:"retail_specs"`
	Tiers                   []ProductTierOption `json:"tiers"`
}

type APIOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
