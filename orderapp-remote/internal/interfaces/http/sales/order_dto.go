package sales

type Option struct {
	ID   int64
	Name string
}

type ProductTierOption struct {
	ID        int64    `json:"id"`
	SpecG     int64    `json:"spec_g"`
	MinQty    float64  `json:"min_qty"`
	MaxQty    *float64 `json:"max_qty"`
	UnitPrice float64  `json:"unit_price"`
}

type ProductOption struct {
	ID              int64               `json:"id"`
	Name            string              `json:"name"`
	RoastLevel      string              `json:"roast_level"`
	DefaultPrice    float64             `json:"default_price"`
	RetailPrice100G float64             `json:"retail_price_100g"`
	RetailPrice200G float64             `json:"retail_price_200g"`
	RetailPrice227G float64             `json:"retail_price_227g"`
	RetailPrice250G float64             `json:"retail_price_250g"`
	RetailSpecs     []int64             `json:"retail_specs"`
	Tiers           []ProductTierOption `json:"tiers"`
}

type PageData struct {
	Today        string
	Customers    []Option
	Sources      []Option
	ShipStatuses []Option
	PayStatuses  []Option
	OrderTypes   []Option
	Products     []ProductOption
}

type CreateOrderRequest struct {
	OrderDate             string `form:"order_date"`
	CustomerID            int64  `form:"customer_id"`
	SourceID              int64  `form:"source_id"`
	OrderTypeID           int64  `form:"order_type_id"`
	PayStatusID           int64  `form:"pay_status_id"`
	ShipStatusID          int64  `form:"ship_status_id"`
	ShipMethod            string `form:"ship_method"`
	ShipTrackingNo        string `form:"ship_tracking_no"`
	Notes                 string `form:"notes"`
	ShippingAmount        string `form:"shipping_amount"`
	DiscountAmount        string `form:"discount_amount"`
	RoundToInt            string `form:"round_to_int"`
	ExpressFee            string `form:"express_fee"`
	OutsourceMaterialFee  string `form:"outsource_material_fee"`
	OutsourceRoastFee     string `form:"outsource_roast_fee"`
	OutsourcePackagingFee string `form:"outsource_packaging_fee"`
	OutsourceManualFee    string `form:"outsource_manual_fee"`
	OutsourceTaxFee       string `form:"outsource_tax_fee"`
	OutsourceOtherFee     string `form:"outsource_other_fee"`

	ProductID []string `form:"product_id[]"`
	TierID    []string `form:"tier_id[]"`
	UnitPrice []string `form:"unit_price[]"`
	ItemName  []string `form:"item_name[]"`
	Qty       []string `form:"qty[]"`
	Unit      []string `form:"unit[]"`
	Spec      []string `form:"spec[]"`
}

type UpdateOrderRequest struct {
	OrderDate             string `form:"order_date"`
	CustomerID            int64  `form:"customer_id"`
	SourceID              int64  `form:"source_id"`
	OrderTypeID           int64  `form:"order_type_id"`
	PayStatusID           int64  `form:"pay_status_id"`
	ShipStatusID          int64  `form:"ship_status_id"`
	ShipMethod            string `form:"ship_method"`
	ShipTrackingNo        string `form:"ship_tracking_no"`
	Notes                 string `form:"notes"`
	ShippingAmount        string `form:"shipping_amount"`
	DiscountAmount        string `form:"discount_amount"`
	RoundToInt            string `form:"round_to_int"`
	ExpressFee            string `form:"express_fee"`
	OutsourceMaterialFee  string `form:"outsource_material_fee"`
	OutsourceRoastFee     string `form:"outsource_roast_fee"`
	OutsourcePackagingFee string `form:"outsource_packaging_fee"`
	OutsourceManualFee    string `form:"outsource_manual_fee"`
	OutsourceTaxFee       string `form:"outsource_tax_fee"`
	OutsourceOtherFee     string `form:"outsource_other_fee"`

	ItemID    []string `form:"item_id[]"`
	Qty       []string `form:"qty[]"`
	UnitPrice []string `form:"unit_price[]"`
}

func (r *CreateOrderRequest) GetMaterial() string  { return r.OutsourceMaterialFee }
func (r *CreateOrderRequest) GetRoast() string     { return r.OutsourceRoastFee }
func (r *CreateOrderRequest) GetPackaging() string { return r.OutsourcePackagingFee }
func (r *CreateOrderRequest) GetManual() string    { return r.OutsourceManualFee }
func (r *CreateOrderRequest) GetTax() string       { return r.OutsourceTaxFee }
func (r *CreateOrderRequest) GetOther() string     { return r.OutsourceOtherFee }

func (r *UpdateOrderRequest) GetMaterial() string  { return r.OutsourceMaterialFee }
func (r *UpdateOrderRequest) GetRoast() string     { return r.OutsourceRoastFee }
func (r *UpdateOrderRequest) GetPackaging() string { return r.OutsourcePackagingFee }
func (r *UpdateOrderRequest) GetManual() string    { return r.OutsourceManualFee }
func (r *UpdateOrderRequest) GetTax() string       { return r.OutsourceTaxFee }
func (r *UpdateOrderRequest) GetOther() string     { return r.OutsourceOtherFee }
