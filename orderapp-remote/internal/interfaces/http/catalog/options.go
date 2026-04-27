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
	RoastLevel              string              `json:"roast_level"`
	DefaultPrice            float64             `json:"default_price"`
	RetailPrice100G         float64             `json:"retail_price_100g"`
	RetailPrice200G         float64             `json:"retail_price_200g"`
	RetailPrice227G         float64             `json:"retail_price_227g"`
	RetailPrice250G         float64             `json:"retail_price_250g"`
	ProductCategoryID       int64               `json:"product_category_id"`
	ProductCategoryPosition int                 `json:"product_category_position"`
	RetailSpecs             []int64             `json:"retail_specs"`
	Tiers                   []ProductTierOption `json:"tiers"`
}

type APIOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
