package sales

import (
	"context"
	"fmt"
	salesdomain "orderapp/internal/domain/sales"
	"strings"
	"time"
)

type SaveOrderCommand struct {
	Actor                           string
	EditID                          int64
	DocumentDate                    time.Time
	OrderDate                       time.Time
	CustomerID                      int64
	SourceID                        int64
	OrderTypeID                     int64
	PayStatusID                     int64
	PaymentMethod                   string
	ShipStatusID                    int64
	ShipMethod                      string
	ShipTrackingNo                  string
	LogisticsCompanyID              int64
	LogisticsProductID              int64
	PaymentGoodsAmount              float64
	PaymentShippingAmount           float64
	PaymentVoucherAssetID           int64
	ResponsibleType                 string
	ResponsibleID                   int64
	Notes                           string
	ShippingAmount                  float64
	DiscountAmount                  float64
	RoundToInt                      bool
	ExpressFee                      string
	OutsourceMaterialFee            float64
	OutsourceRoastFee               float64
	OutsourcePackagingFee           float64
	OutsourceManualFee              float64
	OutsourceTaxFee                 float64
	OutsourceOtherFee               float64
	StockBatchDecision              string
	BeanListPublicationID           int64
	CommercialBeanListPublicationID int64
	GreenBeanListPublicationID      int64
	DripBeanListPublicationID       int64
	Items                           []OrderItemCommand
}

type OrderItemCommand struct {
	ProductID     *int64
	TierID        *int64
	ManualPrice   *float64
	DiscountType  string
	DiscountValue float64
	Name          string
	Note          string
	Units         int64
	Unit          string
	SpecG         int64
	ProductKind   string
	SalesUnit     string
	UnitBagCount  int64
	UnitBeanG     float64
}

type SaveOrderResult struct {
	OrderID        int64
	OrderNo        string
	Edited         bool
	StockBatchUsed bool
}

type OrderStockBatchPreviewCommand struct {
	EditID int64
	Items  []OrderItemCommand
}

type OrderStockBatchAllocation struct {
	BatchID    int64  `json:"batch_id"`
	BatchCode  string `json:"batch_code"`
	AvailableG int64  `json:"available_g"`
	AllocatedG int64  `json:"allocated_g"`
	CreatedAt  string `json:"created_at"`
}

type OrderStockBatchPreviewLine struct {
	ProductID   int64                       `json:"product_id"`
	ProductName string                      `json:"product_name"`
	SpecG       int64                       `json:"spec_g"`
	NeedUnits   int64                       `json:"need_units"`
	NeedG       int64                       `json:"need_g"`
	AvailableG  int64                       `json:"available_g"`
	Sufficient  bool                        `json:"sufficient"`
	Allocations []OrderStockBatchAllocation `json:"allocations"`
}

type OrderStockBatchPreview struct {
	Sufficient      bool                         `json:"sufficient"`
	HasBatchChoices bool                         `json:"has_batch_choices"`
	TotalNeedG      int64                        `json:"total_need_g"`
	TotalAvailableG int64                        `json:"total_available_g"`
	Lines           []OrderStockBatchPreviewLine `json:"lines"`
}

type OrderShippingExportData struct {
	OrderID       int64
	OrderNo       string
	OrderDate     string
	CustomerName  string
	RecvName      string
	RecvPhone     string
	RecvAddr      string
	RecvCompany   string
	SenderID      int64
	ProcessStatus string
	Items         []OrderShippingExportItem
}

type OrderShippingExportItem struct {
	LineNo    int
	Name      string
	Spec      string
	Qty       string
	Unit      string
	UnitPrice string
	LineTotal string
}

type UpdateHeaderCommand struct {
	Actor                 string
	DocumentDate          string
	OrderDate             string
	CustomerID            int64
	SourceID              int64
	OrderTypeID           int64
	PayStatusID           int64
	PaymentMethod         string
	ShipStatusID          int64
	ShipMethod            string
	ShipTrackingNo        string
	LogisticsCompanyID    int64
	LogisticsProductID    int64
	PaymentGoodsAmount    string
	PaymentShippingAmount string
	PaymentVoucherAssetID int64
	Notes                 string
	ShippingAmount        string
	DiscountAmount        string
	RoundToInt            string
	ExpressFee            string
	OutsourceMaterialFee  string
	OutsourceRoastFee     string
	OutsourcePackagingFee string
	OutsourceManualFee    string
	OutsourceTaxFee       string
	OutsourceOtherFee     string
	ItemID                []string
	Qty                   []string
	UnitPrice             []string
}

type InlineUpdateCommand struct {
	OrderTypeID     string
	PayStatusID     string
	PaymentMethod   string
	ShipStatusID    string
	ProcessStatusID string
	Notes           string
}

type OrderListQuery struct {
	OrderID               int64
	Q                     string
	From                  string
	To                    string
	Void                  string
	Scope                 string
	EmployeeID            int64
	FulfillmentEmployeeID int64
	CustomerID            int64
	PayStatusID           int64
	ShipStatusID          int64
	ProcessStatusID       int64
	UnproducedOnly        bool
	CompletedOnly         bool
	ShipReadyOnly         bool
	Limit                 int
	Offset                int
}

type OrderListResult struct {
	Rows            []OrderRow
	Summary         OrdersSummary
	OrderTypes      []Option
	PayStatuses     []Option
	ShipStatuses    []Option
	ProcessStatuses []Option
	HasNext         bool
}

type Option struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type CustomerOption struct {
	ID                      int64  `json:"id"`
	Name                    string `json:"name"`
	CustomerType            string `json:"customer_type"`
	Contact                 string `json:"contact"`
	Phone                   string `json:"phone"`
	DefaultSourceID         int64  `json:"default_source_id"`
	DefaultOrderTypeID      int64  `json:"default_order_type_id"`
	ResponsibleEmployeeID   int64  `json:"responsible_employee_id"`
	ResponsibleEmployeeName string `json:"responsible_employee_name"`
}

type EmployeeOption struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Phone        string `json:"phone"`
	DepartmentID int64  `json:"department_id"`
	Department   string `json:"department"`
}

type ProductTierOption struct {
	ID              int64    `json:"id"`
	SpecG           int64    `json:"spec_g"`
	MinQty          float64  `json:"min_qty"`
	MaxQty          *float64 `json:"max_qty"`
	UnitPrice       float64  `json:"unit_price"`
	DisplayUnit     string   `json:"display_unit,omitempty"`
	ProductKind     string   `json:"product_kind"`
	SalesUnit       string   `json:"sales_unit"`
	UnitBagCount    int64    `json:"unit_bag_count"`
	PriceSourceJSON string   `json:"price_source_json"`
}

type ProductOption struct {
	ID              int64               `json:"id"`
	Name            string              `json:"name"`
	ProductKind     string              `json:"product_kind"`
	RoastLevel      string              `json:"roast_level"`
	DefaultPrice    float64             `json:"default_price"`
	RetailPrice100G float64             `json:"retail_price_100g"`
	RetailPrice200G float64             `json:"retail_price_200g"`
	RetailPrice227G float64             `json:"retail_price_227g"`
	RetailPrice250G float64             `json:"retail_price_250g"`
	CustomerID      int64               `json:"customer_id"`
	BaseProductID   int64               `json:"base_product_id"`
	Visibility      string              `json:"visibility"`
	CustomType      string              `json:"custom_type"`
	DripBagGrams    float64             `json:"drip_bag_grams"`
	DripBoxBagCount int64               `json:"drip_box_bag_count"`
	SalesUnits      []string            `json:"sales_units"`
	RetailSpecs     []int64             `json:"retail_specs"`
	Tiers           []ProductTierOption `json:"tiers"`
}

type BeanListVersionOption struct {
	CustomerID      int64  `json:"customer_id"`
	ListType        string `json:"list_type"`
	ID              int64  `json:"id"`
	VersionNo       string `json:"version_no"`
	Label           string `json:"label"`
	PublishedAt     string `json:"published_at"`
	Changelog       string `json:"changelog"`
	IsCustomerOwned bool   `json:"is_customer_owned"`
	IsDefault       bool   `json:"is_default"`
}

type CustomerPublicUsageOption struct {
	CustomerID   int64 `json:"customer_id"`
	UsePublicSKU bool  `json:"use_public_sku"`
}

type CustomerProductUsageOption struct {
	CustomerID     int64  `json:"customer_id"`
	ProductID      int64  `json:"product_id"`
	OrderCount     int64  `json:"order_count"`
	ItemCount      int64  `json:"item_count"`
	LastOrderDate  string `json:"last_order_date"`
	LastOrderID    int64  `json:"last_order_id"`
	LastOrderNo    string `json:"last_order_no"`
	LastOrderItem  string `json:"last_order_item"`
	LastOrderSpec  string `json:"last_order_spec"`
	LastOrderUnits string `json:"last_order_units"`
}

type OrderFormData struct {
	Today                  string
	Customers              []CustomerOption
	Sources                []Option
	ShipStatuses           []Option
	PayStatuses            []Option
	OrderTypes             []Option
	Products               []ProductOption
	Employees              []EmployeeOption
	LogisticsCompanies     []LogisticsCompany
	BeanListVersionOptions []BeanListVersionOption
	CustomerPublicUsages   []CustomerPublicUsageOption
	CustomerProductUsages  []CustomerProductUsageOption
	EditData               *OrderEditData
}

type OrderEditItem struct {
	ItemID                int64
	LineNo                int
	ProductID             int64
	Product               string
	Note                  string
	Spec                  string
	Qty                   string
	Unit                  string
	UnitPrice             string
	LineTotal             string
	PriceTierID           int64
	BeanListPublicationID int64
	BeanListVersionNo     string
	DiscountType          string
	DiscountValue         string
	DiscountAmount        string
	ProductKind           string
	SalesUnit             string
	UnitBagCount          int64
	UnitBeanG             string
	MatchedPriceQty       string
	UnitConversionLabel   string
	PriceSourceJSON       string
}

type OrderEditData struct {
	ID                    int64
	OrderNo               string
	DocumentDate          string
	OrderDate             string
	CustomerID            int64
	SourceID              int64
	OrderTypeID           int64
	PayStatusID           int64
	PaymentMethod         string
	ShipStatusID          int64
	ShipMethod            string
	ShipTrackingNo        string
	LogisticsCompanyID    int64
	LogisticsProductID    int64
	PaymentGoodsAmount    string
	PaymentShippingAmount string
	PaymentVoucherAssetID int64
	PaymentVoucher        *SalesOrderAsset
	ResponsibleType       string
	ResponsibleID         int64
	ResponsibleName       string
	ReceiverName          string
	ReceiverPhone         string
	ReceiverAddress       string
	ReceiverCompany       string
	PortalServiceCode     string
	SourceWarehouse       string
	BeanListPublicationID int64
	BeanListVersionNo     string
	Notes                 string

	TotalAmount           string
	ShippingAmount        string
	DiscountAmount        string
	RoundToInt            bool
	RoundingAmount        string
	GrandTotal            string
	ExpressFee            string
	OutsourceMaterialFee  string
	OutsourceRoastFee     string
	OutsourcePackagingFee string
	OutsourceManualFee    string
	OutsourceTaxFee       string
	OutsourceOtherFee     string
	OutsourceTotalFee     string

	IsVoid     bool
	VoidedAt   *string
	VoidReason *string

	Items []OrderEditItem
	Error string
}

type OrdersSummary struct {
	Orders    int `json:"orders"`
	Customers int `json:"customers"`
}

type OrderRow struct {
	ID                    int64  `json:"id"`
	OrderNo               string `json:"order_no"`
	DocumentDate          string `json:"document_date"`
	OrderDate             string `json:"order_date"`
	CustomerID            int64  `json:"customer_id"`
	Customer              string `json:"customer"`
	ResponsibleType       string `json:"responsible_type"`
	ResponsibleID         int64  `json:"responsible_id"`
	ResponsibleName       string `json:"responsible_name"`
	TotalAmount           string `json:"total_amount"`
	ShippingAmount        string `json:"shipping_amount"`
	DiscountAmount        string `json:"discount_amount"`
	GrandTotal            string `json:"grand_total"`
	ExpressFee            string `json:"express_fee"`
	OutsourceMaterialFee  string `json:"outsource_material_fee"`
	OutsourceRoastFee     string `json:"outsource_roast_fee"`
	OutsourcePackagingFee string `json:"outsource_packaging_fee"`
	OutsourceManualFee    string `json:"outsource_manual_fee"`
	OutsourceTaxFee       string `json:"outsource_tax_fee"`
	OutsourceOtherFee     string `json:"outsource_other_fee"`
	OutsourceTotalFee     string `json:"outsource_total_fee"`
	OrderType             string `json:"order_type"`
	PayStatus             string `json:"pay_status"`
	PaymentMethod         string `json:"payment_method"`
	ShipStatus            string `json:"ship_status"`
	ShipTrackingNo        string `json:"ship_tracking_no"`
	ReceiverName          string `json:"receiver_name"`
	ReceiverPhone         string `json:"receiver_phone"`
	ReceiverAddress       string `json:"receiver_address"`
	ReceiverCompany       string `json:"receiver_company"`
	PortalServiceCode     string `json:"portal_service_code"`
	SourceWarehouse       string `json:"source_warehouse"`
	SenderID              int64  `json:"sender_id"`
	SenderLabel           string `json:"sender_label"`
	SenderName            string `json:"sender_name"`
	OrderTypeID           int64  `json:"order_type_id"`
	PayStatusID           int64  `json:"pay_status_id"`
	ShipStatusID          int64  `json:"ship_status_id"`
	ProcessStatusID       int64  `json:"process_status_id"`
	ProcessStatus         string `json:"process_status"`
	ProductKindSummary    string `json:"product_kind_summary"`
	CreatedByEmployee     string `json:"created_by_employee"`
	Notes                 string `json:"notes"`
	IsVoid                bool   `json:"is_void"`
	InvoiceStatus         string `json:"invoice_status"`
	InvoiceFilename       string `json:"invoice_filename"`
	InvoiceFileURL        string `json:"invoice_file_url"`
}

type AuditRow struct {
	ChangedAt string  `json:"changed_at"`
	Actor     string  `json:"actor"`
	Field     string  `json:"field"`
	OldValue  *string `json:"old_value"`
	NewValue  *string `json:"new_value"`
}

type OutsourceTemplate struct {
	ID                int64   `json:"id"`
	Name              string  `json:"name"`
	IsDefault         bool    `json:"is_default"`
	RoastUnitPrice    float64 `json:"roast_unit_price"`
	BeanPackUnitPrice float64 `json:"bean_pack_unit_price"`
	DripPackUnitPrice float64 `json:"drip_pack_unit_price"`
	SCUnitPrice       float64 `json:"sc_unit_price"`
}

type SaveOutsourceTemplateCommand struct {
	Name              string  `json:"name"`
	IsDefault         bool    `json:"is_default"`
	RoastUnitPrice    float64 `json:"roast_unit_price"`
	BeanPackUnitPrice float64 `json:"bean_pack_unit_price"`
	DripPackUnitPrice float64 `json:"drip_pack_unit_price"`
	SCUnitPrice       float64 `json:"sc_unit_price"`
}

type TrackingPair struct {
	Phone    string
	Tracking string
}

type FillTrackingPairsCommand struct {
	Actor string
	Pairs []TrackingPair
}

type FillTrackingResult struct {
	Updated int `json:"updated"`
	Total   int `json:"total"`
}

type SetShipMethodCommand struct {
	Actor    string
	OrderIDs []int64
	Method   string
}

type OrderShipmentOrderCommand struct {
	OrderID  int64
	SenderID int64
}

type CreateOrderShipmentCommand struct {
	Actor    string
	SenderID int64
	FileURL  string
	Orders   []OrderShipmentOrderCommand
}

type OrderShipmentResult struct {
	ShipmentID int64  `json:"shipment_id"`
	ShipmentNo string `json:"shipment_no"`
}

type ShipmentTrackingItemCommand struct {
	OrderID    int64  `json:"order_id"`
	TrackingNo string `json:"tracking_no"`
}

type ShipmentTrackingByOrderNoItemCommand struct {
	OrderNo    string `json:"order_no"`
	TrackingNo string `json:"tracking_no"`
}

type FillShipmentTrackingCommand struct {
	Actor      string
	ShipmentID int64
	Items      []ShipmentTrackingItemCommand
}

type FillShipmentTrackingByOrderNoCommand struct {
	Actor string
	Items []ShipmentTrackingByOrderNoItemCommand
}

type FillOrderTrackingCommand struct {
	Actor      string
	OrderID    int64
	TrackingNo string
}

type FillShipmentTrackingResult struct {
	Updated  int      `json:"updated"`
	Total    int      `json:"total"`
	OrderIDs []int64  `json:"order_ids,omitempty"`
	OrderNos []string `json:"order_nos,omitempty"`
}

type SenderProfile struct {
	ID        int64  `json:"sender_id"`
	Label     string `json:"sender_label"`
	Name      string `json:"sender_name"`
	Phone     string `json:"sender_phone"`
	Addr      string `json:"sender_addr"`
	Company   string `json:"sender_company"`
	Goods     string `json:"sender_goods"`
	BizType   string `json:"sf_biz_type"`
	IsDefault bool   `json:"is_default"`
	Active    bool   `json:"active"`
}

type ShippingExportQuery struct {
	Q               string
	From            string
	To              string
	Void            string
	CustomerID      int64
	PayStatusID     int64
	ShipStatusID    int64
	ProcessStatusID int64
	CompletedOnly   bool
	OneClick        bool
}

type ShippingExportRow struct {
	OrderID     int64
	OrderNo     string
	CustomerID  int64
	RecvName    string
	RecvPhone   string
	RecvAddr    string
	RecvCompany string
	TrackingNo  string
	WeightKg    float64
}

type SalesOrderSettings struct {
	CompanyName           string                  `json:"company_name"`
	Note                  string                  `json:"note"`
	PaymentText           string                  `json:"payment_text"`
	BankAccountName       string                  `json:"bank_account_name"`
	BankName              string                  `json:"bank_name"`
	BankAccountNo         string                  `json:"bank_account_no"`
	SealXMM               float64                 `json:"seal_x_mm"`
	SealYMM               float64                 `json:"seal_y_mm"`
	SealWidthMM           float64                 `json:"seal_width_mm"`
	PaymentTextXMM        float64                 `json:"payment_text_x_mm"`
	PaymentTextYMM        float64                 `json:"payment_text_y_mm"`
	PaymentTextWidthMM    float64                 `json:"payment_text_width_mm"`
	PaymentTextHeightMM   float64                 `json:"payment_text_height_mm"`
	PaymentTextPageNumber int                     `json:"payment_text_page_number"`
	PaymentCodeXMM        float64                 `json:"payment_code_x_mm"`
	PaymentCodeYMM        float64                 `json:"payment_code_y_mm"`
	PaymentCodeWidthMM    float64                 `json:"payment_code_width_mm"`
	PaymentCodeHeightMM   float64                 `json:"payment_code_height_mm"`
	PaymentCodePageNumber int                     `json:"payment_code_page_number"`
	Seal                  *SalesOrderAsset        `json:"seal,omitempty"`
	PaymentCodes          []SalesOrderPaymentCode `json:"payment_codes"`
}

const (
	DefaultSalesOrderPaymentTextXMM      = 16
	DefaultSalesOrderPaymentTextYMM      = 118
	DefaultSalesOrderPaymentTextWidthMM  = 104
	DefaultSalesOrderPaymentTextHeightMM = 78
	DefaultSalesOrderPaymentCodeXMM      = 126
	DefaultSalesOrderPaymentCodeYMM      = 106
	DefaultSalesOrderPaymentCodeWidthMM  = 72
	DefaultSalesOrderPaymentCodeHeightMM = 122
)

type SalesOrderAsset struct {
	ID          int64  `json:"id"`
	Kind        string `json:"kind"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Bytes       int64  `json:"bytes"`
	SHA256      string `json:"sha256"`
	ObjectKey   string `json:"object_key"`
	CreatedAt   string `json:"created_at"`
	CreatedBy   string `json:"created_by"`
	URL         string `json:"url"`
}

type SalesOrderPaymentCode struct {
	ID          int64           `json:"id"`
	Label       string          `json:"label"`
	Description string          `json:"description"`
	AssetID     int64           `json:"asset_id"`
	Sort        int             `json:"sort"`
	Active      bool            `json:"active"`
	Asset       SalesOrderAsset `json:"asset"`
}

type LogisticsProduct struct {
	ID        int64  `json:"id"`
	CompanyID int64  `json:"company_id"`
	Name      string `json:"name"`
	Sort      int    `json:"sort"`
	Active    bool   `json:"active"`
}

type LogisticsCompany struct {
	ID       int64              `json:"id"`
	Name     string             `json:"name"`
	Sort     int                `json:"sort"`
	Active   bool               `json:"active"`
	Products []LogisticsProduct `json:"products"`
}

type SaveSalesOrderSettingsCommand struct {
	Actor                 string  `json:"actor"`
	CompanyName           string  `json:"company_name"`
	Note                  string  `json:"note"`
	PaymentText           string  `json:"payment_text"`
	BankAccountName       string  `json:"bank_account_name"`
	BankName              string  `json:"bank_name"`
	BankAccountNo         string  `json:"bank_account_no"`
	SealXMM               float64 `json:"seal_x_mm"`
	SealYMM               float64 `json:"seal_y_mm"`
	SealWidthMM           float64 `json:"seal_width_mm"`
	PaymentTextXMM        float64 `json:"payment_text_x_mm"`
	PaymentTextYMM        float64 `json:"payment_text_y_mm"`
	PaymentTextWidthMM    float64 `json:"payment_text_width_mm"`
	PaymentTextHeightMM   float64 `json:"payment_text_height_mm"`
	PaymentTextPageNumber int     `json:"payment_text_page_number"`
	PaymentCodeXMM        float64 `json:"payment_code_x_mm"`
	PaymentCodeYMM        float64 `json:"payment_code_y_mm"`
	PaymentCodeWidthMM    float64 `json:"payment_code_width_mm"`
	PaymentCodeHeightMM   float64 `json:"payment_code_height_mm"`
	PaymentCodePageNumber int     `json:"payment_code_page_number"`
}

type SaveSalesOrderAssetCommand struct {
	Actor       string
	Kind        string
	Filename    string
	ContentType string
	Bytes       int64
	SHA256      string
	ObjectKey   string
}

type SaveSalesOrderPaymentCodeCommand struct {
	Actor       string `json:"actor"`
	ID          int64  `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	AssetID     int64  `json:"asset_id"`
	Sort        int    `json:"sort"`
	Active      bool   `json:"active"`
}

type SaveLogisticsCompanyCommand struct {
	Actor  string `json:"actor"`
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Sort   int    `json:"sort"`
	Active bool   `json:"active"`
}

type SaveLogisticsProductCommand struct {
	Actor     string `json:"actor"`
	ID        int64  `json:"id"`
	CompanyID int64  `json:"company_id"`
	Name      string `json:"name"`
	Sort      int    `json:"sort"`
	Active    bool   `json:"active"`
}

type GenerateSalesOrderDocumentCommand struct {
	Actor   string
	OrderID int64
}

type CombinedDocumentCommand struct {
	Actor    string  `json:"actor"`
	OrderIDs []int64 `json:"order_ids"`
}

type GenerateSalesOrderImageCommand struct {
	Actor   string
	OrderID int64
}

type SaveSalesOrderNoteCommand struct {
	Actor   string `json:"actor"`
	OrderID int64  `json:"order_id"`
	Note    string `json:"note"`
}

type GenerateSalesOrderDocumentResult struct {
	Document SalesOrderDocument             `json:"document"`
	Snapshot salesdomain.SalesOrderSnapshot `json:"snapshot"`
}

type GenerateSalesOrderImageResult struct {
	Document SalesOrderImageDocument        `json:"document"`
	Snapshot salesdomain.SalesOrderSnapshot `json:"snapshot"`
}

type SalesOrderPreview struct {
	OrderID       int64                          `json:"order_id"`
	OrderNo       string                         `json:"order_no"`
	NextVersionNo int                            `json:"next_version_no"`
	Snapshot      salesdomain.SalesOrderSnapshot `json:"snapshot"`
}

type SalesOrderPreviewPDF struct {
	Preview  SalesOrderPreview `json:"preview"`
	Data     []byte            `json:"-"`
	Filename string            `json:"filename"`
}

type CombinedSalesOrderPreview struct {
	OrderIDs      []int64                                `json:"order_ids"`
	OrderNos      []string                               `json:"order_nos"`
	NextVersionNo int                                    `json:"next_version_no"`
	Snapshot      salesdomain.CombinedSalesOrderSnapshot `json:"snapshot"`
}

type CombinedSalesOrderPreviewPDF struct {
	Preview  CombinedSalesOrderPreview `json:"preview"`
	Data     []byte                    `json:"-"`
	Filename string                    `json:"filename"`
}

type SalesOrderDocument struct {
	ID          int64                          `json:"id"`
	OrderID     int64                          `json:"order_id"`
	OrderNo     string                         `json:"order_no"`
	VersionNo   int                            `json:"version_no"`
	Snapshot    salesdomain.SalesOrderSnapshot `json:"snapshot"`
	PDFAssetID  int64                          `json:"pdf_asset_id"`
	IsLatest    bool                           `json:"is_latest"`
	CreatedAt   string                         `json:"created_at"`
	CreatedBy   string                         `json:"created_by"`
	DownloadURL string                         `json:"download_url"`
}

type SalesOrderImageDocument struct {
	ID           int64                          `json:"id"`
	OrderID      int64                          `json:"order_id"`
	OrderNo      string                         `json:"order_no"`
	VersionNo    int                            `json:"version_no"`
	Snapshot     salesdomain.SalesOrderSnapshot `json:"snapshot"`
	ImageAssetID int64                          `json:"image_asset_id"`
	IsLatest     bool                           `json:"is_latest"`
	CreatedAt    string                         `json:"created_at"`
	CreatedBy    string                         `json:"created_by"`
	DownloadURL  string                         `json:"download_url"`
}

type SalesOrderCustomerInfo struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	CompanyName    string `json:"company_name"`
	CompanyAddress string `json:"company_address"`
	CompanyPhone   string `json:"company_phone"`
	Contact        string `json:"contact"`
	Phone          string `json:"phone"`
	Address        string `json:"address"`
}

type SalesOrderContext struct {
	OrderID  int64                  `json:"order_id"`
	OrderNo  string                 `json:"order_no"`
	Customer SalesOrderCustomerInfo `json:"customer"`
}

type SalesOrderDocumentFile struct {
	Document SalesOrderDocument
	Path     string
	Filename string
}

type CombinedSalesOrderDocument struct {
	ID             int64                                  `json:"id"`
	CombinationKey string                                 `json:"combination_key"`
	CustomerID     int64                                  `json:"customer_id"`
	OrderIDs       []int64                                `json:"order_ids"`
	OrderNos       []string                               `json:"order_nos"`
	VersionNo      int                                    `json:"version_no"`
	Snapshot       salesdomain.CombinedSalesOrderSnapshot `json:"snapshot"`
	PDFAssetID     int64                                  `json:"pdf_asset_id"`
	IsLatest       bool                                   `json:"is_latest"`
	CreatedAt      string                                 `json:"created_at"`
	CreatedBy      string                                 `json:"created_by"`
	DownloadURL    string                                 `json:"download_url"`
}

type GenerateCombinedSalesOrderDocumentResult struct {
	Document CombinedSalesOrderDocument             `json:"document"`
	Snapshot salesdomain.CombinedSalesOrderSnapshot `json:"snapshot"`
}

type CombinedSalesOrderDocumentFile struct {
	Document CombinedSalesOrderDocument
	Path     string
	Filename string
}

type CombinedSalesOrderImageDocument struct {
	ID             int64                                  `json:"id"`
	CombinationKey string                                 `json:"combination_key"`
	CustomerID     int64                                  `json:"customer_id"`
	OrderIDs       []int64                                `json:"order_ids"`
	OrderNos       []string                               `json:"order_nos"`
	VersionNo      int                                    `json:"version_no"`
	Snapshot       salesdomain.CombinedSalesOrderSnapshot `json:"snapshot"`
	ImageAssetID   int64                                  `json:"image_asset_id"`
	IsLatest       bool                                   `json:"is_latest"`
	CreatedAt      string                                 `json:"created_at"`
	CreatedBy      string                                 `json:"created_by"`
	DownloadURL    string                                 `json:"download_url"`
}

type GenerateCombinedSalesOrderImageResult struct {
	Document CombinedSalesOrderImageDocument        `json:"document"`
	Snapshot salesdomain.CombinedSalesOrderSnapshot `json:"snapshot"`
}

type CombinedSalesOrderImageFile struct {
	Document CombinedSalesOrderImageDocument
	Path     string
	Filename string
}

type DeliveryNoteForm struct {
	OrderID             int64  `json:"order_id"`
	OrderNo             string `json:"order_no"`
	PostingDate         string `json:"posting_date"`
	SourceWarehouse     string `json:"source_warehouse"`
	SourceWarehouseName string `json:"source_warehouse_name"`
	DeliveryMethod      string `json:"delivery_method"`
	TrackingNo          string `json:"tracking_no"`
	Note                string `json:"note"`
	UpdatedAt           string `json:"updated_at"`
	UpdatedBy           string `json:"updated_by"`
}

type SaveDeliveryNoteFormCommand struct {
	Actor           string `json:"actor"`
	OrderID         int64  `json:"order_id"`
	PostingDate     string `json:"posting_date"`
	SourceWarehouse string `json:"source_warehouse"`
	DeliveryMethod  string `json:"delivery_method"`
	TrackingNo      string `json:"tracking_no"`
	Note            string `json:"note"`
}

type GenerateDeliveryNoteDocumentCommand struct {
	Actor   string
	OrderID int64
}

type GenerateDeliveryNoteDocumentResult struct {
	Document DeliveryNoteDocument             `json:"document"`
	Snapshot salesdomain.DeliveryNoteSnapshot `json:"snapshot"`
}

type DeliveryNotePreview struct {
	OrderID       int64                            `json:"order_id"`
	OrderNo       string                           `json:"order_no"`
	NextVersionNo int                              `json:"next_version_no"`
	Form          DeliveryNoteForm                 `json:"form"`
	Snapshot      salesdomain.DeliveryNoteSnapshot `json:"snapshot"`
}

type DeliveryNotePreviewPDF struct {
	Preview  DeliveryNotePreview `json:"preview"`
	Data     []byte              `json:"-"`
	Filename string              `json:"filename"`
}

type CombinedDeliveryNotePreview struct {
	OrderIDs      []int64                                  `json:"order_ids"`
	OrderNos      []string                                 `json:"order_nos"`
	NextVersionNo int                                      `json:"next_version_no"`
	Snapshot      salesdomain.CombinedDeliveryNoteSnapshot `json:"snapshot"`
}

type CombinedDeliveryNotePreviewPDF struct {
	Preview  CombinedDeliveryNotePreview `json:"preview"`
	Data     []byte                      `json:"-"`
	Filename string                      `json:"filename"`
}

type DeliveryNoteDocument struct {
	ID          int64                            `json:"id"`
	OrderID     int64                            `json:"order_id"`
	OrderNo     string                           `json:"order_no"`
	VersionNo   int                              `json:"version_no"`
	Snapshot    salesdomain.DeliveryNoteSnapshot `json:"snapshot"`
	PDFAssetID  int64                            `json:"pdf_asset_id"`
	IsLatest    bool                             `json:"is_latest"`
	CreatedAt   string                           `json:"created_at"`
	CreatedBy   string                           `json:"created_by"`
	DownloadURL string                           `json:"download_url"`
}

type DeliveryNoteContext struct {
	OrderID    int64                  `json:"order_id"`
	OrderNo    string                 `json:"order_no"`
	ShipStatus string                 `json:"ship_status"`
	Customer   SalesOrderCustomerInfo `json:"customer"`
}

type DeliveryNoteDocumentFile struct {
	Document DeliveryNoteDocument
	Path     string
	Filename string
}

type CombinedDeliveryNoteDocument struct {
	ID             int64                                    `json:"id"`
	CombinationKey string                                   `json:"combination_key"`
	CustomerID     int64                                    `json:"customer_id"`
	OrderIDs       []int64                                  `json:"order_ids"`
	OrderNos       []string                                 `json:"order_nos"`
	VersionNo      int                                      `json:"version_no"`
	Snapshot       salesdomain.CombinedDeliveryNoteSnapshot `json:"snapshot"`
	PDFAssetID     int64                                    `json:"pdf_asset_id"`
	IsLatest       bool                                     `json:"is_latest"`
	CreatedAt      string                                   `json:"created_at"`
	CreatedBy      string                                   `json:"created_by"`
	DownloadURL    string                                   `json:"download_url"`
}

type GenerateCombinedDeliveryNoteDocumentResult struct {
	Document CombinedDeliveryNoteDocument             `json:"document"`
	Snapshot salesdomain.CombinedDeliveryNoteSnapshot `json:"snapshot"`
}

type CombinedDeliveryNoteDocumentFile struct {
	Document CombinedDeliveryNoteDocument
	Path     string
	Filename string
}

type SalesOrderImageFile struct {
	Document SalesOrderImageDocument
	Path     string
	Filename string
}

const (
	ExternalShareSalesOrderPDF   = "sales_order_pdf"
	ExternalShareSalesOrderImage = "sales_order_image"
	ExternalShareDeliveryNotePDF = "delivery_note_pdf"
)

type CreateExternalShareResourceCommand struct {
	Actor        string `json:"actor"`
	ResourceType string `json:"resource_type"`
	OrderID      int64  `json:"order_id"`
	DocumentID   int64  `json:"document_id"`
	Latest       bool   `json:"latest"`
}

type ExternalShareResource struct {
	Token        string `json:"token"`
	ResourceType string `json:"resource_type"`
	OrderID      int64  `json:"order_id"`
	ResourceID   int64  `json:"resource_id"`
	Title        string `json:"title"`
	Filename     string `json:"filename"`
	ContentType  string `json:"content_type"`
	ShareURL     string `json:"share_url"`
	FileURL      string `json:"file_url"`
	ShareText    string `json:"share_text"`
	CreatedAt    string `json:"created_at"`
	CreatedBy    string `json:"created_by"`
}

type ExternalShareResourceFile struct {
	Resource ExternalShareResource
	Path     string
}

type OrderInvoice struct {
	OrderID     int64            `json:"order_id"`
	OrderNo     string           `json:"order_no"`
	Status      string           `json:"status"`
	RequestedAt string           `json:"requested_at"`
	RequestedBy string           `json:"requested_by"`
	UploadedAt  string           `json:"uploaded_at"`
	UploadedBy  string           `json:"uploaded_by"`
	Asset       *SalesOrderAsset `json:"asset,omitempty"`
}

type RequestOrderInvoiceCommand struct {
	Actor   string
	OrderID int64
}

type SaveOrderInvoiceFileCommand struct {
	Actor       string
	OrderID     int64
	Filename    string
	ContentType string
	Bytes       int64
	SHA256      string
	ObjectKey   string
}

type Repository interface {
	SaveOrder(ctx context.Context, cmd SaveOrderCommand) (SaveOrderResult, error)
	PreviewOrderStockBatches(ctx context.Context, cmd OrderStockBatchPreviewCommand) (OrderStockBatchPreview, error)
	UpdateHeader(ctx context.Context, id int64, cmd UpdateHeaderCommand) error
	InlineUpdate(ctx context.Context, id int64, actor string, cmd InlineUpdateCommand) error
	Void(ctx context.Context, id int64, actor, reason string) error
	VoidMany(ctx context.Context, ids []int64, actor, reason string) (int, error)
	ListOrders(ctx context.Context, query OrderListQuery) (OrderListResult, error)
	ListOrderAuditLogs(ctx context.Context, orderID int64, limit int) ([]AuditRow, error)
	OrderForm(ctx context.Context, editID int64) (OrderFormData, error)
	ListOutsourceTemplates(ctx context.Context) ([]OutsourceTemplate, error)
	SaveOutsourceTemplate(ctx context.Context, cmd SaveOutsourceTemplateCommand) error
	FillTrackingPairs(ctx context.Context, cmd FillTrackingPairsCommand) (FillTrackingResult, error)
	SetShipMethod(ctx context.Context, cmd SetShipMethodCommand) error
	LoadSenderProfile(ctx context.Context) (SenderProfile, error)
	LoadSenderProfileByID(ctx context.Context, id int64) (SenderProfile, error)
	ListSenderProfiles(ctx context.Context) ([]SenderProfile, error)
	SaveSenderProfile(ctx context.Context, profile SenderProfile) error
	ListSFSmallShippingRows(ctx context.Context, query ShippingExportQuery) ([]ShippingExportRow, error)
	LoadOrderShippingExportData(ctx context.Context, orderID int64) (OrderShippingExportData, error)
	CreateOrderShipment(ctx context.Context, cmd CreateOrderShipmentCommand) (OrderShipmentResult, error)
	FillShipmentTracking(ctx context.Context, cmd FillShipmentTrackingCommand) (FillShipmentTrackingResult, error)
	FillShipmentTrackingByOrderNo(ctx context.Context, cmd FillShipmentTrackingByOrderNoCommand) (FillShipmentTrackingResult, error)
	FillOrderTracking(ctx context.Context, cmd FillOrderTrackingCommand) (FillShipmentTrackingResult, error)
	LoadSalesOrderSettings(ctx context.Context) (SalesOrderSettings, error)
	SaveSalesOrderSettings(ctx context.Context, cmd SaveSalesOrderSettingsCommand) error
	SaveSalesOrderAsset(ctx context.Context, cmd SaveSalesOrderAssetCommand) (SalesOrderAsset, error)
	DeleteSalesOrderAsset(ctx context.Context, id int64, actor string) error
	ListSalesOrderSealAssets(ctx context.Context) ([]SalesOrderAsset, error)
	SaveSalesOrderPaymentCode(ctx context.Context, cmd SaveSalesOrderPaymentCodeCommand) (SalesOrderPaymentCode, error)
	DeactivateSalesOrderPaymentCode(ctx context.Context, id int64, actor string) error
	ActivateSalesOrderPaymentCode(ctx context.Context, id int64, actor string) error
	DeleteSalesOrderPaymentCode(ctx context.Context, id int64, actor string) error
	ListLogisticsCompanies(ctx context.Context, includeInactive bool) ([]LogisticsCompany, error)
	SaveLogisticsCompany(ctx context.Context, cmd SaveLogisticsCompanyCommand) (LogisticsCompany, error)
	SaveLogisticsProduct(ctx context.Context, cmd SaveLogisticsProductCommand) (LogisticsProduct, error)
	SetSalesOrderSealAsset(ctx context.Context, assetID int64, actor string) error
	LoadSalesOrderContext(ctx context.Context, orderID int64) (SalesOrderContext, error)
	SaveSalesOrderNote(ctx context.Context, cmd SaveSalesOrderNoteCommand) error
	ListSalesOrderDocuments(ctx context.Context, orderID int64) ([]SalesOrderDocument, error)
	ListSalesOrderImageDocuments(ctx context.Context, orderID int64) ([]SalesOrderImageDocument, error)
	PreviewSalesOrderDocument(ctx context.Context, orderID int64) (SalesOrderPreview, error)
	PreviewSalesOrderPDF(ctx context.Context, orderID int64) (SalesOrderPreviewPDF, error)
	GenerateSalesOrderDocument(ctx context.Context, cmd GenerateSalesOrderDocumentCommand) (GenerateSalesOrderDocumentResult, error)
	GenerateSalesOrderImage(ctx context.Context, cmd GenerateSalesOrderImageCommand) (GenerateSalesOrderImageResult, error)
	LoadSalesOrderDocumentFile(ctx context.Context, orderID, documentID int64, latest bool) (SalesOrderDocumentFile, error)
	LoadSalesOrderImageFile(ctx context.Context, orderID, imageID int64, latest bool) (SalesOrderImageFile, error)
	ListCombinedSalesOrderDocuments(ctx context.Context, orderIDs []int64) ([]CombinedSalesOrderDocument, error)
	ListCombinedSalesOrderImageDocuments(ctx context.Context, orderIDs []int64) ([]CombinedSalesOrderImageDocument, error)
	PreviewCombinedSalesOrderDocument(ctx context.Context, orderIDs []int64) (CombinedSalesOrderPreview, error)
	PreviewCombinedSalesOrderPDF(ctx context.Context, orderIDs []int64) (CombinedSalesOrderPreviewPDF, error)
	GenerateCombinedSalesOrderDocument(ctx context.Context, cmd CombinedDocumentCommand) (GenerateCombinedSalesOrderDocumentResult, error)
	GenerateCombinedSalesOrderImage(ctx context.Context, cmd CombinedDocumentCommand) (GenerateCombinedSalesOrderImageResult, error)
	LoadCombinedSalesOrderDocumentFile(ctx context.Context, documentID int64) (CombinedSalesOrderDocumentFile, error)
	LoadCombinedSalesOrderImageFile(ctx context.Context, imageID int64) (CombinedSalesOrderImageFile, error)
	LoadDeliveryNoteContext(ctx context.Context, orderID int64) (DeliveryNoteContext, error)
	LoadDeliveryNoteForm(ctx context.Context, orderID int64) (DeliveryNoteForm, error)
	SaveDeliveryNoteForm(ctx context.Context, cmd SaveDeliveryNoteFormCommand) (DeliveryNoteForm, error)
	ListDeliveryNoteDocuments(ctx context.Context, orderID int64) ([]DeliveryNoteDocument, error)
	PreviewDeliveryNoteDocument(ctx context.Context, orderID int64) (DeliveryNotePreview, error)
	PreviewDeliveryNotePDF(ctx context.Context, orderID int64) (DeliveryNotePreviewPDF, error)
	GenerateDeliveryNoteDocument(ctx context.Context, cmd GenerateDeliveryNoteDocumentCommand) (GenerateDeliveryNoteDocumentResult, error)
	LoadDeliveryNoteDocumentFile(ctx context.Context, orderID, documentID int64, latest bool) (DeliveryNoteDocumentFile, error)
	ListCombinedDeliveryNoteDocuments(ctx context.Context, orderIDs []int64) ([]CombinedDeliveryNoteDocument, error)
	PreviewCombinedDeliveryNoteDocument(ctx context.Context, orderIDs []int64) (CombinedDeliveryNotePreview, error)
	PreviewCombinedDeliveryNotePDF(ctx context.Context, orderIDs []int64) (CombinedDeliveryNotePreviewPDF, error)
	GenerateCombinedDeliveryNoteDocument(ctx context.Context, cmd CombinedDocumentCommand) (GenerateCombinedDeliveryNoteDocumentResult, error)
	LoadCombinedDeliveryNoteDocumentFile(ctx context.Context, documentID int64) (CombinedDeliveryNoteDocumentFile, error)
	CreateExternalShareResource(ctx context.Context, cmd CreateExternalShareResourceCommand) (ExternalShareResource, error)
	LoadExternalShareResourceFile(ctx context.Context, token string) (ExternalShareResourceFile, error)
	LoadOrderInvoice(ctx context.Context, orderID int64) (OrderInvoice, error)
	RequestOrderInvoice(ctx context.Context, cmd RequestOrderInvoiceCommand) (OrderInvoice, error)
	SaveOrderInvoiceFile(ctx context.Context, cmd SaveOrderInvoiceFileCommand) (OrderInvoice, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) SaveOrder(ctx context.Context, cmd SaveOrderCommand) (SaveOrderResult, error) {
	if err := validateSaveOrderCommand(cmd); err != nil {
		return SaveOrderResult{}, err
	}
	if decision := strings.TrimSpace(cmd.StockBatchDecision); decision != "" && decision != "use_batch" && decision != "produce" {
		return SaveOrderResult{}, fmt.Errorf("invalid stock_batch_decision")
	}
	cmd.ShipTrackingNo = TrackingNumbersSummary(NormalizeTrackingNumbers(cmd.ShipTrackingNo))
	return s.repo.SaveOrder(ctx, cmd)
}

func (s *Service) PreviewOrderStockBatches(ctx context.Context, cmd OrderStockBatchPreviewCommand) (OrderStockBatchPreview, error) {
	if len(cmd.Items) == 0 {
		return OrderStockBatchPreview{}, fmt.Errorf("at least one item required")
	}
	hasValid := false
	for _, item := range cmd.Items {
		if item.ProductID == nil && strings.TrimSpace(item.Name) == "" {
			continue
		}
		if item.ProductID == nil {
			return OrderStockBatchPreview{}, fmt.Errorf("product required")
		}
		if item.SpecG <= 0 {
			return OrderStockBatchPreview{}, fmt.Errorf("spec required")
		}
		if item.Units <= 0 {
			return OrderStockBatchPreview{}, fmt.Errorf("qty required")
		}
		hasValid = true
	}
	if !hasValid {
		return OrderStockBatchPreview{}, fmt.Errorf("at least one item required")
	}
	return s.repo.PreviewOrderStockBatches(ctx, cmd)
}

func validateSaveOrderCommand(cmd SaveOrderCommand) error {
	if cmd.OrderDate.IsZero() {
		return fmt.Errorf("invalid order_date")
	}
	if cmd.CustomerID <= 0 {
		return fmt.Errorf("customer required")
	}
	valid := false
	for _, item := range cmd.Items {
		if item.ProductID == nil && strings.TrimSpace(item.Name) == "" {
			continue
		}
		if item.ProductID == nil {
			return fmt.Errorf("product required")
		}
		if isDripBagOrderItemKind(item.ProductKind) {
			if item.UnitBeanG <= 0 && item.SpecG <= 0 {
				return fmt.Errorf("unit_bean_g required")
			}
			if strings.TrimSpace(item.SalesUnit) == "box" && item.UnitBagCount <= 0 {
				return fmt.Errorf("unit_bag_count required")
			}
		} else if item.SpecG <= 0 {
			return fmt.Errorf("spec required")
		}
		if item.Units <= 0 {
			return fmt.Errorf("qty required")
		}
		valid = true
	}
	if !valid {
		return fmt.Errorf("at least one item required")
	}
	return nil
}

func isDripBagOrderItemKind(kind string) bool {
	return strings.TrimSpace(kind) == "drip_bag"
}

func (s *Service) UpdateHeader(ctx context.Context, id int64, cmd UpdateHeaderCommand) error {
	return s.repo.UpdateHeader(ctx, id, cmd)
}

func (s *Service) InlineUpdate(ctx context.Context, id int64, actor string, cmd InlineUpdateCommand) error {
	return s.repo.InlineUpdate(ctx, id, actor, cmd)
}

func (s *Service) Void(ctx context.Context, id int64, actor, reason string) error {
	if id <= 0 {
		return fmt.Errorf("invalid order id")
	}
	return s.repo.Void(ctx, id, actor, reason)
}

func (s *Service) VoidMany(ctx context.Context, ids []int64, actor, reason string) (int, error) {
	normalized, err := normalizeOrderIDs(ids)
	if err != nil {
		return 0, err
	}
	return s.repo.VoidMany(ctx, normalized, actor, reason)
}

func normalizeOrderIDs(ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("order_ids required")
	}
	seen := map[int64]bool{}
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, fmt.Errorf("invalid order id")
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("order_ids required")
	}
	return out, nil
}

func (s *Service) ListOrders(ctx context.Context, query OrderListQuery) (OrderListResult, error) {
	if query.Limit <= 0 {
		query.Limit = 10
	}
	if query.Limit > 200 {
		query.Limit = 200
	}
	if query.Offset < 0 {
		query.Offset = 0
	}
	if strings.TrimSpace(query.Void) == "" {
		query.Void = "normal"
	}
	return s.repo.ListOrders(ctx, query)
}

func (s *Service) ListOrderAuditLogs(ctx context.Context, orderID int64, limit int) ([]AuditRow, error) {
	if orderID <= 0 {
		return nil, fmt.Errorf("invalid order id")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	return s.repo.ListOrderAuditLogs(ctx, orderID, limit)
}

func (s *Service) OrderForm(ctx context.Context, editID int64) (OrderFormData, error) {
	if editID < 0 {
		return OrderFormData{}, fmt.Errorf("invalid edit_id")
	}
	data, err := s.repo.OrderForm(ctx, editID)
	if err != nil {
		return OrderFormData{}, err
	}
	if strings.TrimSpace(data.Today) == "" {
		data.Today = time.Now().Format("2006-01-02")
	}
	return data, nil
}

func (s *Service) ListOutsourceTemplates(ctx context.Context) ([]OutsourceTemplate, error) {
	return s.repo.ListOutsourceTemplates(ctx)
}

func (s *Service) SaveOutsourceTemplate(ctx context.Context, cmd SaveOutsourceTemplateCommand) error {
	cmd.Name = strings.TrimSpace(cmd.Name)
	if cmd.Name == "" {
		return fmt.Errorf("name required")
	}
	if cmd.RoastUnitPrice < 0 || cmd.BeanPackUnitPrice < 0 || cmd.DripPackUnitPrice < 0 || cmd.SCUnitPrice < 0 {
		return fmt.Errorf("prices must be non-negative")
	}
	return s.repo.SaveOutsourceTemplate(ctx, cmd)
}

func (s *Service) FillTrackingPairs(ctx context.Context, cmd FillTrackingPairsCommand) (FillTrackingResult, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.Actor == "" {
		cmd.Actor = "shipping"
	}
	pairs := make([]TrackingPair, 0, len(cmd.Pairs))
	for _, pair := range cmd.Pairs {
		phone := digitsOnly(pair.Phone)
		tracking := TrackingNumbersSummary(NormalizeTrackingNumbers(pair.Tracking))
		if phone == "" || tracking == "" {
			continue
		}
		pairs = append(pairs, TrackingPair{Phone: phone, Tracking: tracking})
	}
	if len(pairs) == 0 {
		return FillTrackingResult{}, nil
	}
	cmd.Pairs = pairs
	return s.repo.FillTrackingPairs(ctx, cmd)
}

func (s *Service) SetShipMethod(ctx context.Context, cmd SetShipMethodCommand) error {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.Actor == "" {
		cmd.Actor = "shipping"
	}
	cmd.Method = strings.TrimSpace(cmd.Method)
	if cmd.Method == "" {
		return fmt.Errorf("ship method required")
	}
	seen := map[int64]bool{}
	ids := make([]int64, 0, len(cmd.OrderIDs))
	for _, id := range cmd.OrderIDs {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil
	}
	cmd.OrderIDs = ids
	return s.repo.SetShipMethod(ctx, cmd)
}

func (s *Service) CreateOrderShipment(ctx context.Context, cmd CreateOrderShipmentCommand) (OrderShipmentResult, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.Actor == "" {
		cmd.Actor = "shipping"
	}
	cmd.FileURL = strings.TrimSpace(cmd.FileURL)
	if cmd.FileURL == "" {
		return OrderShipmentResult{}, fmt.Errorf("file_url required")
	}
	seen := map[int64]bool{}
	orders := make([]OrderShipmentOrderCommand, 0, len(cmd.Orders))
	for _, order := range cmd.Orders {
		if order.OrderID <= 0 || seen[order.OrderID] {
			continue
		}
		seen[order.OrderID] = true
		if order.SenderID <= 0 {
			order.SenderID = cmd.SenderID
		}
		orders = append(orders, order)
	}
	if len(orders) == 0 {
		return OrderShipmentResult{}, fmt.Errorf("order required")
	}
	cmd.Orders = orders
	return s.repo.CreateOrderShipment(ctx, cmd)
}

func (s *Service) FillShipmentTracking(ctx context.Context, cmd FillShipmentTrackingCommand) (FillShipmentTrackingResult, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.Actor == "" {
		cmd.Actor = "shipping"
	}
	if cmd.ShipmentID <= 0 {
		return FillShipmentTrackingResult{}, fmt.Errorf("shipment required")
	}
	byOrder := make(map[int64][]string, len(cmd.Items))
	orderSeq := make([]int64, 0, len(cmd.Items))
	for _, item := range cmd.Items {
		numbers := NormalizeTrackingNumbers(item.TrackingNo)
		if item.OrderID <= 0 || len(numbers) == 0 {
			continue
		}
		if _, ok := byOrder[item.OrderID]; !ok {
			orderSeq = append(orderSeq, item.OrderID)
		}
		byOrder[item.OrderID] = appendUniqueTrackingNumbers(byOrder[item.OrderID], numbers)
	}
	items := make([]ShipmentTrackingItemCommand, 0, len(orderSeq))
	for _, orderID := range orderSeq {
		items = append(items, ShipmentTrackingItemCommand{
			OrderID:    orderID,
			TrackingNo: TrackingNumbersSummary(byOrder[orderID]),
		})
	}
	if len(items) == 0 {
		return FillShipmentTrackingResult{}, nil
	}
	cmd.Items = items
	return s.repo.FillShipmentTracking(ctx, cmd)
}

func (s *Service) FillShipmentTrackingByOrderNo(ctx context.Context, cmd FillShipmentTrackingByOrderNoCommand) (FillShipmentTrackingResult, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.Actor == "" {
		cmd.Actor = "shipping"
	}
	byOrderNo := make(map[string][]string, len(cmd.Items))
	orderNoSeq := make([]string, 0, len(cmd.Items))
	for _, item := range cmd.Items {
		item.OrderNo = strings.TrimSpace(item.OrderNo)
		numbers := NormalizeTrackingNumbers(item.TrackingNo)
		if item.OrderNo == "" || len(numbers) == 0 {
			continue
		}
		if _, ok := byOrderNo[item.OrderNo]; !ok {
			orderNoSeq = append(orderNoSeq, item.OrderNo)
		}
		byOrderNo[item.OrderNo] = appendUniqueTrackingNumbers(byOrderNo[item.OrderNo], numbers)
	}
	items := make([]ShipmentTrackingByOrderNoItemCommand, 0, len(orderNoSeq))
	for _, orderNo := range orderNoSeq {
		items = append(items, ShipmentTrackingByOrderNoItemCommand{
			OrderNo:    orderNo,
			TrackingNo: TrackingNumbersSummary(byOrderNo[orderNo]),
		})
	}
	if len(items) == 0 {
		return FillShipmentTrackingResult{}, nil
	}
	cmd.Items = items
	return s.repo.FillShipmentTrackingByOrderNo(ctx, cmd)
}

func (s *Service) FillOrderTracking(ctx context.Context, cmd FillOrderTrackingCommand) (FillShipmentTrackingResult, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.Actor == "" {
		cmd.Actor = "shipping"
	}
	if cmd.OrderID <= 0 {
		return FillShipmentTrackingResult{}, fmt.Errorf("order required")
	}
	cmd.TrackingNo = TrackingNumbersSummary(NormalizeTrackingNumbers(cmd.TrackingNo))
	if cmd.TrackingNo == "" {
		return FillShipmentTrackingResult{}, fmt.Errorf("tracking_no required")
	}
	return s.repo.FillOrderTracking(ctx, cmd)
}

func appendUniqueTrackingNumbers(existing []string, next []string) []string {
	seen := make(map[string]bool, len(existing)+len(next))
	out := make([]string, 0, len(existing)+len(next))
	for _, no := range existing {
		if no == "" || seen[no] {
			continue
		}
		seen[no] = true
		out = append(out, no)
	}
	for _, no := range next {
		if no == "" || seen[no] {
			continue
		}
		seen[no] = true
		out = append(out, no)
	}
	return out
}

func (s *Service) LoadSenderProfile(ctx context.Context) (SenderProfile, error) {
	profile, err := s.repo.LoadSenderProfile(ctx)
	if err != nil {
		return SenderProfile{}, err
	}
	if strings.TrimSpace(profile.Goods) == "" {
		profile.Goods = "茶叶"
	}
	return profile, nil
}

func (s *Service) LoadSenderProfileByID(ctx context.Context, id int64) (SenderProfile, error) {
	var (
		profile SenderProfile
		err     error
	)
	if id > 0 {
		profile, err = s.repo.LoadSenderProfileByID(ctx, id)
	} else {
		profile, err = s.repo.LoadSenderProfile(ctx)
	}
	if err != nil {
		return SenderProfile{}, err
	}
	if strings.TrimSpace(profile.Goods) == "" {
		profile.Goods = "茶叶"
	}
	return profile, nil
}

func (s *Service) ListSenderProfiles(ctx context.Context) ([]SenderProfile, error) {
	profiles, err := s.repo.ListSenderProfiles(ctx)
	if err != nil {
		return nil, err
	}
	for i := range profiles {
		if strings.TrimSpace(profiles[i].Goods) == "" {
			profiles[i].Goods = "茶叶"
		}
	}
	return profiles, nil
}

func (s *Service) SaveSenderProfile(ctx context.Context, profile SenderProfile) error {
	profile.Label = strings.TrimSpace(profile.Label)
	profile.Name = strings.TrimSpace(profile.Name)
	profile.Phone = strings.TrimSpace(profile.Phone)
	profile.Addr = strings.TrimSpace(profile.Addr)
	profile.Company = strings.TrimSpace(profile.Company)
	profile.Goods = strings.TrimSpace(profile.Goods)
	if profile.Goods == "" {
		profile.Goods = "茶叶"
	}
	profile.BizType = strings.TrimSpace(profile.BizType)
	if profile.Label == "" {
		profile.Label = firstNonEmptyText(profile.Name, profile.Company, "寄件人")
	}
	if !profile.Active && profile.ID == 0 {
		profile.Active = true
	}
	return s.repo.SaveSenderProfile(ctx, profile)
}

func firstNonEmptyText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (s *Service) ListSFSmallShippingRows(ctx context.Context, query ShippingExportQuery) ([]ShippingExportRow, error) {
	query.Q = strings.TrimSpace(query.Q)
	query.From = strings.TrimSpace(query.From)
	query.To = strings.TrimSpace(query.To)
	query.Void = strings.TrimSpace(query.Void)
	if query.Void == "" {
		query.Void = "normal"
	}
	return s.repo.ListSFSmallShippingRows(ctx, query)
}

func (s *Service) LoadOrderShippingExportData(ctx context.Context, orderID int64) (OrderShippingExportData, error) {
	if orderID <= 0 {
		return OrderShippingExportData{}, fmt.Errorf("invalid order id")
	}
	return s.repo.LoadOrderShippingExportData(ctx, orderID)
}

func (s *Service) LoadSalesOrderSettings(ctx context.Context) (SalesOrderSettings, error) {
	return s.repo.LoadSalesOrderSettings(ctx)
}

func (s *Service) SaveSalesOrderSettings(ctx context.Context, cmd SaveSalesOrderSettingsCommand) error {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.Actor == "" {
		cmd.Actor = "sales"
	}
	cmd.CompanyName = strings.TrimSpace(cmd.CompanyName)
	cmd.Note = strings.TrimSpace(cmd.Note)
	cmd.PaymentText = strings.TrimSpace(cmd.PaymentText)
	cmd.BankAccountName = strings.TrimSpace(cmd.BankAccountName)
	cmd.BankName = strings.TrimSpace(cmd.BankName)
	cmd.BankAccountNo = strings.TrimSpace(cmd.BankAccountNo)
	if cmd.SealXMM <= 0 {
		cmd.SealXMM = 32
	}
	if cmd.SealYMM <= 0 {
		cmd.SealYMM = 5
	}
	if cmd.SealWidthMM <= 0 {
		cmd.SealWidthMM = 36
	}
	if cmd.PaymentTextXMM <= 0 {
		cmd.PaymentTextXMM = DefaultSalesOrderPaymentTextXMM
	}
	if cmd.PaymentTextYMM <= 0 {
		cmd.PaymentTextYMM = DefaultSalesOrderPaymentTextYMM
	}
	if cmd.PaymentTextWidthMM <= 0 {
		cmd.PaymentTextWidthMM = DefaultSalesOrderPaymentTextWidthMM
	}
	if cmd.PaymentTextHeightMM <= 0 {
		cmd.PaymentTextHeightMM = DefaultSalesOrderPaymentTextHeightMM
	}
	if cmd.PaymentTextPageNumber < 0 {
		cmd.PaymentTextPageNumber = 0
	}
	if cmd.PaymentCodeXMM <= 0 {
		cmd.PaymentCodeXMM = DefaultSalesOrderPaymentCodeXMM
	}
	if cmd.PaymentCodeYMM <= 0 {
		cmd.PaymentCodeYMM = DefaultSalesOrderPaymentCodeYMM
	}
	if cmd.PaymentCodeWidthMM <= 0 {
		cmd.PaymentCodeWidthMM = DefaultSalesOrderPaymentCodeWidthMM
	}
	if cmd.PaymentCodeHeightMM <= 0 {
		cmd.PaymentCodeHeightMM = DefaultSalesOrderPaymentCodeHeightMM
	}
	if cmd.PaymentCodePageNumber < 0 {
		cmd.PaymentCodePageNumber = 0
	}
	return s.repo.SaveSalesOrderSettings(ctx, cmd)
}

func (s *Service) SaveSalesOrderAsset(ctx context.Context, cmd SaveSalesOrderAssetCommand) (SalesOrderAsset, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.Actor == "" {
		cmd.Actor = "sales"
	}
	cmd.Kind = strings.TrimSpace(cmd.Kind)
	cmd.Filename = strings.TrimSpace(cmd.Filename)
	cmd.ContentType = strings.TrimSpace(cmd.ContentType)
	cmd.SHA256 = strings.TrimSpace(cmd.SHA256)
	cmd.ObjectKey = strings.TrimSpace(cmd.ObjectKey)
	if cmd.Kind == "" {
		return SalesOrderAsset{}, fmt.Errorf("kind required")
	}
	if cmd.ObjectKey == "" {
		return SalesOrderAsset{}, fmt.Errorf("object_key required")
	}
	return s.repo.SaveSalesOrderAsset(ctx, cmd)
}

func (s *Service) DeleteSalesOrderAsset(ctx context.Context, id int64, actor string) error {
	if id <= 0 {
		return fmt.Errorf("asset required")
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "sales"
	}
	return s.repo.DeleteSalesOrderAsset(ctx, id, actor)
}

func (s *Service) ListSalesOrderSealAssets(ctx context.Context) ([]SalesOrderAsset, error) {
	assets, err := s.repo.ListSalesOrderSealAssets(ctx)
	if err != nil {
		return nil, err
	}
	for i := range assets {
		if strings.TrimSpace(assets[i].URL) == "" && strings.TrimSpace(assets[i].ObjectKey) != "" {
			assets[i].URL = "/assets/" + strings.TrimSpace(assets[i].ObjectKey)
		}
	}
	return assets, nil
}

func (s *Service) SaveSalesOrderPaymentCode(ctx context.Context, cmd SaveSalesOrderPaymentCodeCommand) (SalesOrderPaymentCode, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.Actor == "" {
		cmd.Actor = "sales"
	}
	cmd.Label = strings.TrimSpace(cmd.Label)
	cmd.Description = strings.TrimSpace(cmd.Description)
	if cmd.Label == "" {
		return SalesOrderPaymentCode{}, fmt.Errorf("label required")
	}
	if cmd.AssetID <= 0 {
		return SalesOrderPaymentCode{}, fmt.Errorf("asset required")
	}
	return s.repo.SaveSalesOrderPaymentCode(ctx, cmd)
}

func (s *Service) DeactivateSalesOrderPaymentCode(ctx context.Context, id int64, actor string) error {
	if id <= 0 {
		return fmt.Errorf("payment code required")
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "sales"
	}
	return s.repo.DeactivateSalesOrderPaymentCode(ctx, id, actor)
}

func (s *Service) ActivateSalesOrderPaymentCode(ctx context.Context, id int64, actor string) error {
	if id <= 0 {
		return fmt.Errorf("payment code required")
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "sales"
	}
	return s.repo.ActivateSalesOrderPaymentCode(ctx, id, actor)
}

func (s *Service) DeleteSalesOrderPaymentCode(ctx context.Context, id int64, actor string) error {
	if id <= 0 {
		return fmt.Errorf("payment code required")
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "sales"
	}
	return s.repo.DeleteSalesOrderPaymentCode(ctx, id, actor)
}

func (s *Service) ListLogisticsCompanies(ctx context.Context, includeInactive bool) ([]LogisticsCompany, error) {
	return s.repo.ListLogisticsCompanies(ctx, includeInactive)
}

func (s *Service) SaveLogisticsCompany(ctx context.Context, cmd SaveLogisticsCompanyCommand) (LogisticsCompany, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.Actor == "" {
		cmd.Actor = "sales"
	}
	cmd.Name = strings.TrimSpace(cmd.Name)
	if cmd.Name == "" {
		return LogisticsCompany{}, fmt.Errorf("logistics company name required")
	}
	return s.repo.SaveLogisticsCompany(ctx, cmd)
}

func (s *Service) SaveLogisticsProduct(ctx context.Context, cmd SaveLogisticsProductCommand) (LogisticsProduct, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.Actor == "" {
		cmd.Actor = "sales"
	}
	cmd.Name = strings.TrimSpace(cmd.Name)
	if cmd.CompanyID <= 0 {
		return LogisticsProduct{}, fmt.Errorf("logistics company required")
	}
	if cmd.Name == "" {
		return LogisticsProduct{}, fmt.Errorf("logistics product name required")
	}
	return s.repo.SaveLogisticsProduct(ctx, cmd)
}

func (s *Service) SetSalesOrderSealAsset(ctx context.Context, assetID int64, actor string) error {
	if assetID <= 0 {
		return fmt.Errorf("asset required")
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "sales"
	}
	return s.repo.SetSalesOrderSealAsset(ctx, assetID, actor)
}

func (s *Service) ListSalesOrderDocuments(ctx context.Context, orderID int64) ([]SalesOrderDocument, error) {
	if orderID <= 0 {
		return nil, fmt.Errorf("invalid order id")
	}
	return s.repo.ListSalesOrderDocuments(ctx, orderID)
}

func (s *Service) ListSalesOrderImageDocuments(ctx context.Context, orderID int64) ([]SalesOrderImageDocument, error) {
	if orderID <= 0 {
		return nil, fmt.Errorf("invalid order id")
	}
	return s.repo.ListSalesOrderImageDocuments(ctx, orderID)
}

func (s *Service) LoadSalesOrderContext(ctx context.Context, orderID int64) (SalesOrderContext, error) {
	if orderID <= 0 {
		return SalesOrderContext{}, fmt.Errorf("invalid order id")
	}
	return s.repo.LoadSalesOrderContext(ctx, orderID)
}

func (s *Service) SaveSalesOrderNote(ctx context.Context, cmd SaveSalesOrderNoteCommand) error {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.Actor == "" {
		cmd.Actor = "sales"
	}
	cmd.Note = strings.TrimSpace(cmd.Note)
	if cmd.OrderID <= 0 {
		return fmt.Errorf("invalid order id")
	}
	return s.repo.SaveSalesOrderNote(ctx, cmd)
}

func (s *Service) GenerateSalesOrderDocument(ctx context.Context, cmd GenerateSalesOrderDocumentCommand) (GenerateSalesOrderDocumentResult, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.Actor == "" {
		cmd.Actor = "sales"
	}
	if cmd.OrderID <= 0 {
		return GenerateSalesOrderDocumentResult{}, fmt.Errorf("invalid order id")
	}
	return s.repo.GenerateSalesOrderDocument(ctx, cmd)
}

func (s *Service) GenerateSalesOrderImage(ctx context.Context, cmd GenerateSalesOrderImageCommand) (GenerateSalesOrderImageResult, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.Actor == "" {
		cmd.Actor = "sales"
	}
	if cmd.OrderID <= 0 {
		return GenerateSalesOrderImageResult{}, fmt.Errorf("invalid order id")
	}
	return s.repo.GenerateSalesOrderImage(ctx, cmd)
}

func (s *Service) PreviewSalesOrderDocument(ctx context.Context, orderID int64) (SalesOrderPreview, error) {
	if orderID <= 0 {
		return SalesOrderPreview{}, fmt.Errorf("invalid order id")
	}
	return s.repo.PreviewSalesOrderDocument(ctx, orderID)
}

func (s *Service) PreviewSalesOrderPDF(ctx context.Context, orderID int64) (SalesOrderPreviewPDF, error) {
	if orderID <= 0 {
		return SalesOrderPreviewPDF{}, fmt.Errorf("invalid order id")
	}
	return s.repo.PreviewSalesOrderPDF(ctx, orderID)
}

func (s *Service) LoadSalesOrderDocumentFile(ctx context.Context, orderID, documentID int64, latest bool) (SalesOrderDocumentFile, error) {
	if orderID <= 0 {
		return SalesOrderDocumentFile{}, fmt.Errorf("invalid order id")
	}
	if !latest && documentID <= 0 {
		return SalesOrderDocumentFile{}, fmt.Errorf("invalid document id")
	}
	return s.repo.LoadSalesOrderDocumentFile(ctx, orderID, documentID, latest)
}

func (s *Service) LoadSalesOrderImageFile(ctx context.Context, orderID, imageID int64, latest bool) (SalesOrderImageFile, error) {
	if orderID <= 0 {
		return SalesOrderImageFile{}, fmt.Errorf("invalid order id")
	}
	if !latest && imageID <= 0 {
		return SalesOrderImageFile{}, fmt.Errorf("invalid image id")
	}
	return s.repo.LoadSalesOrderImageFile(ctx, orderID, imageID, latest)
}

func (s *Service) ListCombinedSalesOrderDocuments(ctx context.Context, orderIDs []int64) ([]CombinedSalesOrderDocument, error) {
	ids, err := normalizeCombinedDocumentOrderIDs(orderIDs)
	if err != nil {
		return nil, err
	}
	return s.repo.ListCombinedSalesOrderDocuments(ctx, ids)
}

func (s *Service) ListCombinedSalesOrderImageDocuments(ctx context.Context, orderIDs []int64) ([]CombinedSalesOrderImageDocument, error) {
	ids, err := normalizeCombinedDocumentOrderIDs(orderIDs)
	if err != nil {
		return nil, err
	}
	return s.repo.ListCombinedSalesOrderImageDocuments(ctx, ids)
}

func (s *Service) PreviewCombinedSalesOrderDocument(ctx context.Context, orderIDs []int64) (CombinedSalesOrderPreview, error) {
	ids, err := normalizeCombinedDocumentOrderIDs(orderIDs)
	if err != nil {
		return CombinedSalesOrderPreview{}, err
	}
	return s.repo.PreviewCombinedSalesOrderDocument(ctx, ids)
}

func (s *Service) PreviewCombinedSalesOrderPDF(ctx context.Context, orderIDs []int64) (CombinedSalesOrderPreviewPDF, error) {
	ids, err := normalizeCombinedDocumentOrderIDs(orderIDs)
	if err != nil {
		return CombinedSalesOrderPreviewPDF{}, err
	}
	return s.repo.PreviewCombinedSalesOrderPDF(ctx, ids)
}

func (s *Service) GenerateCombinedSalesOrderDocument(ctx context.Context, cmd CombinedDocumentCommand) (GenerateCombinedSalesOrderDocumentResult, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.Actor == "" {
		cmd.Actor = "sales"
	}
	ids, err := normalizeCombinedDocumentOrderIDs(cmd.OrderIDs)
	if err != nil {
		return GenerateCombinedSalesOrderDocumentResult{}, err
	}
	cmd.OrderIDs = ids
	return s.repo.GenerateCombinedSalesOrderDocument(ctx, cmd)
}

func (s *Service) GenerateCombinedSalesOrderImage(ctx context.Context, cmd CombinedDocumentCommand) (GenerateCombinedSalesOrderImageResult, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.Actor == "" {
		cmd.Actor = "sales"
	}
	ids, err := normalizeCombinedDocumentOrderIDs(cmd.OrderIDs)
	if err != nil {
		return GenerateCombinedSalesOrderImageResult{}, err
	}
	cmd.OrderIDs = ids
	return s.repo.GenerateCombinedSalesOrderImage(ctx, cmd)
}

func (s *Service) LoadCombinedSalesOrderDocumentFile(ctx context.Context, documentID int64) (CombinedSalesOrderDocumentFile, error) {
	if documentID <= 0 {
		return CombinedSalesOrderDocumentFile{}, fmt.Errorf("invalid document id")
	}
	return s.repo.LoadCombinedSalesOrderDocumentFile(ctx, documentID)
}

func (s *Service) LoadCombinedSalesOrderImageFile(ctx context.Context, imageID int64) (CombinedSalesOrderImageFile, error) {
	if imageID <= 0 {
		return CombinedSalesOrderImageFile{}, fmt.Errorf("invalid image id")
	}
	return s.repo.LoadCombinedSalesOrderImageFile(ctx, imageID)
}

func (s *Service) LoadDeliveryNoteContext(ctx context.Context, orderID int64) (DeliveryNoteContext, error) {
	if orderID <= 0 {
		return DeliveryNoteContext{}, fmt.Errorf("invalid order id")
	}
	return s.repo.LoadDeliveryNoteContext(ctx, orderID)
}

func (s *Service) LoadDeliveryNoteForm(ctx context.Context, orderID int64) (DeliveryNoteForm, error) {
	if orderID <= 0 {
		return DeliveryNoteForm{}, fmt.Errorf("invalid order id")
	}
	return s.repo.LoadDeliveryNoteForm(ctx, orderID)
}

func (s *Service) SaveDeliveryNoteForm(ctx context.Context, cmd SaveDeliveryNoteFormCommand) error {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.Actor == "" {
		cmd.Actor = "warehouse"
	}
	if cmd.OrderID <= 0 {
		return fmt.Errorf("invalid order id")
	}
	cmd.PostingDate = strings.TrimSpace(cmd.PostingDate)
	cmd.SourceWarehouse = strings.TrimSpace(cmd.SourceWarehouse)
	if cmd.SourceWarehouse == "" {
		cmd.SourceWarehouse = "finished_goods"
	}
	cmd.DeliveryMethod = strings.TrimSpace(cmd.DeliveryMethod)
	cmd.TrackingNo = strings.TrimSpace(cmd.TrackingNo)
	cmd.Note = strings.TrimSpace(cmd.Note)
	_, err := s.repo.SaveDeliveryNoteForm(ctx, cmd)
	return err
}

func (s *Service) ListDeliveryNoteDocuments(ctx context.Context, orderID int64) ([]DeliveryNoteDocument, error) {
	if orderID <= 0 {
		return nil, fmt.Errorf("invalid order id")
	}
	return s.repo.ListDeliveryNoteDocuments(ctx, orderID)
}

func (s *Service) PreviewDeliveryNoteDocument(ctx context.Context, orderID int64) (DeliveryNotePreview, error) {
	if orderID <= 0 {
		return DeliveryNotePreview{}, fmt.Errorf("invalid order id")
	}
	return s.repo.PreviewDeliveryNoteDocument(ctx, orderID)
}

func (s *Service) PreviewDeliveryNotePDF(ctx context.Context, orderID int64) (DeliveryNotePreviewPDF, error) {
	if orderID <= 0 {
		return DeliveryNotePreviewPDF{}, fmt.Errorf("invalid order id")
	}
	return s.repo.PreviewDeliveryNotePDF(ctx, orderID)
}

func (s *Service) GenerateDeliveryNoteDocument(ctx context.Context, cmd GenerateDeliveryNoteDocumentCommand) (GenerateDeliveryNoteDocumentResult, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.Actor == "" {
		cmd.Actor = "warehouse"
	}
	if cmd.OrderID <= 0 {
		return GenerateDeliveryNoteDocumentResult{}, fmt.Errorf("invalid order id")
	}
	return s.repo.GenerateDeliveryNoteDocument(ctx, cmd)
}

func (s *Service) LoadDeliveryNoteDocumentFile(ctx context.Context, orderID, documentID int64, latest bool) (DeliveryNoteDocumentFile, error) {
	if orderID <= 0 {
		return DeliveryNoteDocumentFile{}, fmt.Errorf("invalid order id")
	}
	if !latest && documentID <= 0 {
		return DeliveryNoteDocumentFile{}, fmt.Errorf("invalid document id")
	}
	return s.repo.LoadDeliveryNoteDocumentFile(ctx, orderID, documentID, latest)
}

func (s *Service) ListCombinedDeliveryNoteDocuments(ctx context.Context, orderIDs []int64) ([]CombinedDeliveryNoteDocument, error) {
	ids, err := normalizeCombinedDocumentOrderIDs(orderIDs)
	if err != nil {
		return nil, err
	}
	return s.repo.ListCombinedDeliveryNoteDocuments(ctx, ids)
}

func (s *Service) PreviewCombinedDeliveryNoteDocument(ctx context.Context, orderIDs []int64) (CombinedDeliveryNotePreview, error) {
	ids, err := normalizeCombinedDocumentOrderIDs(orderIDs)
	if err != nil {
		return CombinedDeliveryNotePreview{}, err
	}
	return s.repo.PreviewCombinedDeliveryNoteDocument(ctx, ids)
}

func (s *Service) PreviewCombinedDeliveryNotePDF(ctx context.Context, orderIDs []int64) (CombinedDeliveryNotePreviewPDF, error) {
	ids, err := normalizeCombinedDocumentOrderIDs(orderIDs)
	if err != nil {
		return CombinedDeliveryNotePreviewPDF{}, err
	}
	return s.repo.PreviewCombinedDeliveryNotePDF(ctx, ids)
}

func (s *Service) GenerateCombinedDeliveryNoteDocument(ctx context.Context, cmd CombinedDocumentCommand) (GenerateCombinedDeliveryNoteDocumentResult, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.Actor == "" {
		cmd.Actor = "warehouse"
	}
	ids, err := normalizeCombinedDocumentOrderIDs(cmd.OrderIDs)
	if err != nil {
		return GenerateCombinedDeliveryNoteDocumentResult{}, err
	}
	cmd.OrderIDs = ids
	return s.repo.GenerateCombinedDeliveryNoteDocument(ctx, cmd)
}

func (s *Service) LoadCombinedDeliveryNoteDocumentFile(ctx context.Context, documentID int64) (CombinedDeliveryNoteDocumentFile, error) {
	if documentID <= 0 {
		return CombinedDeliveryNoteDocumentFile{}, fmt.Errorf("invalid document id")
	}
	return s.repo.LoadCombinedDeliveryNoteDocumentFile(ctx, documentID)
}

func normalizeCombinedDocumentOrderIDs(orderIDs []int64) ([]int64, error) {
	seen := map[int64]bool{}
	ids := make([]int64, 0, len(orderIDs))
	for _, id := range orderIDs {
		if id <= 0 {
			return nil, fmt.Errorf("invalid order id")
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) < 2 {
		return nil, fmt.Errorf("at least two orders required")
	}
	return ids, nil
}

func (s *Service) CreateExternalShareResource(ctx context.Context, cmd CreateExternalShareResourceCommand) (ExternalShareResource, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.Actor == "" {
		cmd.Actor = "sales"
	}
	cmd.ResourceType = strings.TrimSpace(cmd.ResourceType)
	if !isExternalShareResourceType(cmd.ResourceType) {
		return ExternalShareResource{}, fmt.Errorf("invalid share resource type")
	}
	if cmd.OrderID <= 0 {
		return ExternalShareResource{}, fmt.Errorf("invalid order id")
	}
	if !cmd.Latest && cmd.DocumentID <= 0 {
		return ExternalShareResource{}, fmt.Errorf("invalid document id")
	}
	return s.repo.CreateExternalShareResource(ctx, cmd)
}

func (s *Service) LoadExternalShareResourceFile(ctx context.Context, token string) (ExternalShareResourceFile, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return ExternalShareResourceFile{}, fmt.Errorf("invalid share token")
	}
	return s.repo.LoadExternalShareResourceFile(ctx, token)
}

func isExternalShareResourceType(resourceType string) bool {
	switch resourceType {
	case ExternalShareSalesOrderPDF, ExternalShareSalesOrderImage, ExternalShareDeliveryNotePDF:
		return true
	default:
		return false
	}
}

func (s *Service) LoadOrderInvoice(ctx context.Context, orderID int64) (OrderInvoice, error) {
	if orderID <= 0 {
		return OrderInvoice{}, fmt.Errorf("invalid order id")
	}
	return s.repo.LoadOrderInvoice(ctx, orderID)
}

func (s *Service) RequestOrderInvoice(ctx context.Context, cmd RequestOrderInvoiceCommand) (OrderInvoice, error) {
	if cmd.OrderID <= 0 {
		return OrderInvoice{}, fmt.Errorf("invalid order id")
	}
	return s.repo.RequestOrderInvoice(ctx, cmd)
}

func (s *Service) SaveOrderInvoiceFile(ctx context.Context, cmd SaveOrderInvoiceFileCommand) (OrderInvoice, error) {
	if cmd.OrderID <= 0 {
		return OrderInvoice{}, fmt.Errorf("invalid order id")
	}
	if strings.TrimSpace(cmd.Filename) == "" {
		return OrderInvoice{}, fmt.Errorf("filename required")
	}
	if !IsOrderInvoiceContentTypeAllowed(cmd.ContentType) {
		return OrderInvoice{}, fmt.Errorf("only PDF and image files are allowed")
	}
	if cmd.Bytes <= 0 {
		return OrderInvoice{}, fmt.Errorf("empty file")
	}
	if strings.TrimSpace(cmd.SHA256) == "" {
		return OrderInvoice{}, fmt.Errorf("sha256 required")
	}
	if strings.TrimSpace(cmd.ObjectKey) == "" {
		return OrderInvoice{}, fmt.Errorf("object_key required")
	}
	return s.repo.SaveOrderInvoiceFile(ctx, cmd)
}

func IsOrderInvoiceContentTypeAllowed(contentType string) bool {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "application/pdf", "image/png", "image/jpeg", "image/gif", "image/webp":
		return true
	default:
		return false
	}
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
