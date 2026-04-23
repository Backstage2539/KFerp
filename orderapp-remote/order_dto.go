package main

import "html/template"

type Option struct {
	ID   int64
	Name string
}

type ProductTierOption struct {
	ID      int64
	MinLb   float64
	MaxLb   *float64
	PriceLb float64
}

type ProductOption struct {
	ID              int64
	Name            string
	DefaultPrice    float64
	RetailPrice227G float64
	Tiers           []ProductTierOption
}

type PageData struct {
	Today        string
	Customers    []Option
	Sources      []Option
	ShipStatuses []Option
	PayStatuses  []Option
	OrderTypes   []Option
	Products     []ProductOption
	ProductsJSON template.JS
	EditMode     bool
	EditID       int64
	EditDataJSON template.JS
	Ok           bool
	OrderNo      string
	Error        string
}

type OrderRow struct {
	ID                int64
	OrderNo           string
	OrderDate         string
	CustomerID        int64
	Customer          string
	GrandTotal        string
	OrderType         string
	PayStatus         string
	ShipStatus        string
	OrderTypeID       int64
	PayStatusID       int64
	ShipStatusID      int64
	ProcessStatusID   int64
	ProcessStatus     string
	CreatedByEmployee string
	Notes             string
	IsVoid            bool
}

type OrdersPageData struct {
	Q                string
	From             string
	To               string
	Preset           string
	Void             string // normal|void|all
	CustomerID       int64
	PayStatusFilter  int64
	ShipStatusFilter int64
	ProcStatusFilter int64
	UnproducedOnly   bool
	CompletedOnly    bool
	Summary          OrdersSummary
	Rows             []OrderRow
	OrderTypeOpts    []Option
	PayOpts          []Option
	ShipOpts         []Option
	ProcessOpts      []Option
	Limit            int
	Offset           int
	Page             int
	HasPrev          bool
	HasNext          bool
	Error            string
}

type OrderItemRow struct {
	LineNo    int
	Product   string
	ItemName  string
	Qty       *float64
	Unit      *string
	Spec      *string
	UnitPrice *float64
	LineTotal *float64
}

type OrderDetailData struct {
	ID                    int64
	OrderNo               string
	OrderDate             string
	Customer              string
	Source                string
	OrderType             string
	PayStatus             string
	ShipStatus            string
	OrderTypeID           int64
	PayStatusID           int64
	ShipStatusID          int64
	ProcessStatus         string
	CreatedByEmployee     string
	IsVoid                bool
	VoidedAt              *string
	VoidReason            *string
	Notes                 *string
	TotalAmount           float64
	ShippingAmt           float64
	DiscountAmt           float64
	RoundToInt            bool
	RoundingAmt           float64
	GrandTotal            float64
	ExpressFee            *string
	OutsourceMaterialFee  float64
	OutsourceRoastFee     float64
	OutsourcePackagingFee float64
	OutsourceManualFee    float64
	OutsourceTaxFee       float64
	OutsourceOtherFee     float64
	OutsourceTotalFee     float64
	OrderTypeOpts         []Option
	PayOpts               []Option
	ShipOpts              []Option
	Items                 []OrderItemRow
	Error                 string
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
