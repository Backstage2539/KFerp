package sales

type jsTier struct {
	ID        int64    `json:"id"`
	SpecG     int64    `json:"spec_g"`
	Min       float64  `json:"min"`
	Max       *float64 `json:"max"`
	UnitPrice float64  `json:"unit_price"`
}

type jsProduct struct {
	ID              int64    `json:"id"`
	Name            string   `json:"name"`
	ProductKind     string   `json:"product_kind"`
	Py              string   `json:"py"`
	Pyi             string   `json:"pyi"`
	RetailPrice100G float64  `json:"retail_price_100g"`
	RetailPrice200G float64  `json:"retail_price_200g"`
	RetailPrice227G float64  `json:"retail_price_227g"`
	RetailPrice250G float64  `json:"retail_price_250g"`
	CustomerID      int64    `json:"customer_id"`
	BaseProductID   int64    `json:"base_product_id"`
	Visibility      string   `json:"visibility"`
	CustomType      string   `json:"custom_type"`
	RetailSpecs     []int64  `json:"retail_specs"`
	Tiers           []jsTier `json:"tiers"`
}
