package sales

import salesapp "orderapp/internal/application/sales"

type Option = salesapp.Option
type CustomerOption = salesapp.CustomerOption
type EmployeeOption = salesapp.EmployeeOption
type ProductTierOption = salesapp.ProductTierOption
type ProductOption = salesapp.ProductOption
type PageData = salesapp.OrderFormData

type CreateOrderRequest struct {
	DocumentDate                    string `form:"document_date"`
	OrderDate                       string `form:"order_date"`
	CustomerID                      int64  `form:"customer_id"`
	SourceID                        int64  `form:"source_id"`
	OrderTypeID                     int64  `form:"order_type_id"`
	PayStatusID                     int64  `form:"pay_status_id"`
	PaymentMethod                   string `form:"payment_method"`
	ShipStatusID                    int64  `form:"ship_status_id"`
	ShipMethod                      string `form:"ship_method"`
	ShipTrackingNo                  string `form:"ship_tracking_no"`
	LogisticsCompanyID              int64  `form:"logistics_company_id"`
	LogisticsProductID              int64  `form:"logistics_product_id"`
	PaymentGoodsAmount              string `form:"payment_goods_amount"`
	PaymentShippingAmount           string `form:"payment_shipping_amount"`
	PaymentVoucherAssetID           int64  `form:"payment_voucher_asset_id"`
	ResponsibleType                 string `form:"responsible_type"`
	ResponsibleID                   int64  `form:"responsible_id"`
	Notes                           string `form:"notes"`
	ShippingAmount                  string `form:"shipping_amount"`
	DiscountAmount                  string `form:"discount_amount"`
	RoundToInt                      string `form:"round_to_int"`
	ExpressFee                      string `form:"express_fee"`
	OutsourceMaterialFee            string `form:"outsource_material_fee"`
	OutsourceRoastFee               string `form:"outsource_roast_fee"`
	OutsourcePackagingFee           string `form:"outsource_packaging_fee"`
	OutsourceManualFee              string `form:"outsource_manual_fee"`
	OutsourceTaxFee                 string `form:"outsource_tax_fee"`
	OutsourceOtherFee               string `form:"outsource_other_fee"`
	StockBatchDecision              string `form:"stock_batch_decision"`
	BeanListPublicationID           int64  `form:"bean_list_publication_id"`
	CommercialBeanListPublicationID int64  `form:"commercial_bean_list_publication_id"`
	GreenBeanListPublicationID      int64  `form:"green_bean_list_publication_id"`
	DripBeanListPublicationID       int64  `form:"drip_bean_list_publication_id"`
	ReceiverName                    string `form:"receiver_name"`
	ReceiverPhone                   string `form:"receiver_phone"`
	ReceiverAddress                 string `form:"receiver_address"`
	ReceiverCompany                 string `form:"receiver_company"`
	PortalServiceCode               string `form:"portal_service_code"`
	OrdersScope                     string `form:"orders_scope"`

	ProductID                          []string `form:"product_id[]"`
	ParentProductID                    []string `form:"parent_product_id[]"`
	ItemParentProductID                []string `form:"item_parent_product_id[]"`
	BomSpecID                          []string `form:"bom_spec_id[]"`
	BomVariantID                       []string `form:"bom_variant_id[]"`
	CustomerProductAliasID             []string `form:"customer_product_alias_id[]"`
	CustomerProductReferenceID         []string `form:"customer_product_reference_id[]"`
	MaterialSourceMode                 []string `form:"material_source_mode[]"`
	CustomerProductDisplayNameSnapshot []string `form:"customer_product_display_name_snapshot[]"`
	CustomerItemCodeSnapshot           []string `form:"customer_item_code_snapshot[]"`
	BrandNameSnapshot                  []string `form:"brand_name_snapshot[]"`
	ProductCodeSnapshot                []string `form:"product_code_snapshot[]"`
	ProductNameSnapshot                []string `form:"product_name_snapshot[]"`
	ItemBeanListPublicationID          []string `form:"item_bean_list_publication_id[]"`
	ItemBeanListVersionNo              []string `form:"item_bean_list_version_no[]"`
	PriceSourceJSON                    []string `form:"price_source_json[]"`
	TierID                             []string `form:"tier_id[]"`
	UnitPrice                          []string `form:"unit_price[]"`
	ItemName                           []string `form:"item_name[]"`
	ItemNote                           []string `form:"item_note[]"`
	Qty                                []string `form:"qty[]"`
	Unit                               []string `form:"unit[]"`
	Spec                               []string `form:"spec[]"`
	ProductKind                        []string `form:"product_kind[]"`
	SalesUnit                          []string `form:"sales_unit[]"`
	UnitBagCount                       []string `form:"unit_bag_count[]"`
	UnitBeanG                          []string `form:"unit_bean_g[]"`
	DiscountType                       []string `form:"discount_type[]"`
	DiscountValue                      []string `form:"discount_value[]"`
}

type UpdateOrderRequest struct {
	DocumentDate          string `form:"document_date"`
	OrderDate             string `form:"order_date"`
	CustomerID            int64  `form:"customer_id"`
	SourceID              int64  `form:"source_id"`
	OrderTypeID           int64  `form:"order_type_id"`
	PayStatusID           int64  `form:"pay_status_id"`
	PaymentMethod         string `form:"payment_method"`
	ShipStatusID          int64  `form:"ship_status_id"`
	ShipMethod            string `form:"ship_method"`
	ShipTrackingNo        string `form:"ship_tracking_no"`
	LogisticsCompanyID    int64  `form:"logistics_company_id"`
	LogisticsProductID    int64  `form:"logistics_product_id"`
	PaymentGoodsAmount    string `form:"payment_goods_amount"`
	PaymentShippingAmount string `form:"payment_shipping_amount"`
	PaymentVoucherAssetID int64  `form:"payment_voucher_asset_id"`
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
