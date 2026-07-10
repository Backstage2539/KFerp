package orderliststaging

import "time"

const (
	ReviewAutoReady   = "auto_ready"
	ReviewNeedsReview = "needs_review"
	ReviewApproved    = "approved"
	ReviewExcluded    = "excluded"
)

type ImportRun struct {
	RunID              string    `json:"run_id"`
	SourcePath         string    `json:"source_path"`
	SourceSHA256       string    `json:"source_sha256"`
	SourceBytes        int64     `json:"source_bytes"`
	StartPeriod        string    `json:"start_period"`
	EndPeriod          string    `json:"end_period"`
	CreatedAt          time.Time `json:"created_at"`
	WorkbookSheetCount int       `json:"workbook_sheet_count"`
	IncludedSheetCount int       `json:"included_sheet_count"`
	RawOrderCount      int       `json:"raw_order_count"`
	RawProductLines    int       `json:"raw_product_lines"`
}

type SheetInventory struct {
	SheetName      string `json:"sheet_name"`
	Period         string `json:"period"`
	Included       bool   `json:"included"`
	ExcludedReason string `json:"excluded_reason"`
	UsedRowCount   int    `json:"used_row_count"`
	OrderRowCount  int    `json:"order_row_count"`
}

type RawOrder struct {
	SheetName             string            `json:"source_sheet_name"`
	SheetPeriod           string            `json:"sheet_period"`
	SourceRowNumber       int               `json:"source_row_number"`
	SequenceOriginal      string            `json:"source_sequence_original"`
	DuplicateSuffix       int               `json:"duplicate_suffix"`
	SequenceEffective     string            `json:"source_sequence_effective"`
	SourceOrderKey        string            `json:"source_order_key"`
	Fingerprint           string            `json:"source_fingerprint"`
	OrderDateRaw          string            `json:"order_date_raw"`
	OrderDate             string            `json:"order_date"`
	CustomerRaw           string            `json:"customer_raw"`
	CustomerKey           string            `json:"customer_key"`
	ProductRaw            string            `json:"product_raw"`
	RemarkRaw             string            `json:"remark_raw"`
	GrindRaw              string            `json:"grind_raw"`
	RoastRaw              string            `json:"roast_raw"`
	ShipmentStatusRaw     string            `json:"shipment_status_raw"`
	PaymentStatusRaw      string            `json:"payment_status_raw"`
	AmountRaw             string            `json:"amount_raw"`
	AmountValue           *float64          `json:"amount_value"`
	AmountDerived         bool              `json:"amount_derived"`
	ShippingAmountRaw     string            `json:"shipping_amount_raw"`
	ShippingAmountValue   *float64          `json:"shipping_amount_value"`
	ReceiptDateRaw        string            `json:"receipt_date_raw"`
	OrderSourceRaw        string            `json:"order_source_raw"`
	ExpressFeeRaw         string            `json:"express_fee_raw"`
	OrderTypeRaw          string            `json:"order_type_raw"`
	TrackingNoRaw         string            `json:"tracking_no_raw"`
	ShipmentDateRaw       string            `json:"shipment_date_raw"`
	CreditTermRaw         string            `json:"credit_term_raw"`
	QuantityRaw           string            `json:"quantity_raw"`
	LatestShipmentDateRaw string            `json:"latest_shipment_date_raw"`
	RawFields             map[string]string `json:"raw_fields"`
	ReviewStatus          string            `json:"review_status"`
}

type SourceKeyAssignment struct {
	SheetName         string `json:"sheet_name"`
	OriginalSequence  string `json:"original_sequence"`
	EffectiveSequence string `json:"effective_sequence"`
	DuplicateSuffix   int    `json:"duplicate_suffix"`
}

type ERPReferenceCustomer struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Phone  string `json:"phone"`
	Active bool   `json:"active"`
}

type ERPReferenceProduct struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	ProductKind string `json:"product_kind"`
	Active      bool   `json:"active"`
}

type Customer struct {
	CustomerKey     string `json:"customer_key"`
	CanonicalName   string `json:"canonical_name"`
	NormalizedPhone string `json:"normalized_phone"`
	CurrentContact  string `json:"current_contact"`
	CurrentAddress  string `json:"current_address"`
	ERPMatchID      int64  `json:"erp_match_id"`
	ERPMatchName    string `json:"erp_match_name"`
	MatchMethod     string `json:"match_method"`
	ReviewStatus    string `json:"review_status"`
}

type CustomerAlias struct {
	CustomerKey     string `json:"customer_key"`
	Alias           string `json:"alias"`
	AliasNormalized string `json:"alias_normalized"`
	SourceOrderKey  string `json:"source_order_key"`
	ObservedDate    string `json:"observed_date"`
}

type CustomerPhone struct {
	CustomerKey     string `json:"customer_key"`
	PhoneRaw        string `json:"phone_raw"`
	PhoneNormalized string `json:"phone_normalized"`
	IsPrimary       bool   `json:"is_primary"`
	SourceOrderKey  string `json:"source_order_key"`
}

type Product struct {
	ProductKey    string  `json:"product_key"`
	CanonicalName string  `json:"canonical_name"`
	ProductKind   string  `json:"product_kind"`
	RoastLevel    string  `json:"roast_level"`
	ERPMatchID    int64   `json:"erp_match_id"`
	ERPMatchName  string  `json:"erp_match_name"`
	MatchMethod   string  `json:"match_method"`
	MatchScore    float64 `json:"match_score"`
	ReviewStatus  string  `json:"review_status"`
}

type SKU struct {
	SKUKey            string  `json:"sku_key"`
	ProductKey        string  `json:"product_key"`
	SpecName          string  `json:"spec_name"`
	SalesUnit         string  `json:"sales_unit"`
	NetContentQty     float64 `json:"net_content_qty"`
	NetContentUnit    string  `json:"net_content_unit"`
	NormalizedWeightG float64 `json:"normalized_weight_g"`
	ReviewStatus      string  `json:"review_status"`
}

type ProductAlias struct {
	ProductKey     string  `json:"product_key"`
	SKUKey         string  `json:"sku_key"`
	RawLine        string  `json:"raw_line"`
	NormalizedLine string  `json:"normalized_line"`
	SourceOrderKey string  `json:"source_order_key"`
	MatchMethod    string  `json:"match_method"`
	MatchScore     float64 `json:"match_score"`
}

type Order struct {
	SourceOrderKey      string   `json:"source_order_key"`
	SheetName           string   `json:"sheet_name"`
	SequenceOriginal    string   `json:"sequence_original"`
	SequenceEffective   string   `json:"sequence_effective"`
	SourceRowNumber     int      `json:"source_row_number"`
	SourceFingerprint   string   `json:"source_fingerprint"`
	OrderDate           string   `json:"order_date"`
	CustomerKey         string   `json:"customer_key"`
	CustomerRaw         string   `json:"customer_raw"`
	OrderSourceRaw      string   `json:"order_source_raw"`
	OrderTypeRaw        string   `json:"order_type_raw"`
	PaymentStatusRaw    string   `json:"payment_status_raw"`
	ShipmentStatusRaw   string   `json:"shipment_status_raw"`
	AmountValue         *float64 `json:"amount_value"`
	AmountRaw           string   `json:"amount_raw"`
	AmountDerived       bool     `json:"amount_derived"`
	ShippingAmountValue *float64 `json:"shipping_amount_value"`
	ShippingAmountRaw   string   `json:"shipping_amount_raw"`
	TrackingNoRaw       string   `json:"tracking_no_raw"`
	RemarkRaw           string   `json:"remark_raw"`
	ReviewStatus        string   `json:"review_status"`
}

type OrderItem struct {
	SourceItemKey     string  `json:"source_item_key"`
	SourceOrderKey    string  `json:"source_order_key"`
	LineNo            int     `json:"line_no"`
	RawLine           string  `json:"raw_line"`
	ProductKey        string  `json:"product_key"`
	SKUKey            string  `json:"sku_key"`
	ParentName        string  `json:"parent_name"`
	SpecName          string  `json:"spec_name"`
	ProductKind       string  `json:"product_kind"`
	RoastLevel        string  `json:"roast_level"`
	OrderQuantity     float64 `json:"order_quantity"`
	OrderUnit         string  `json:"order_unit"`
	NormalizedWeightG float64 `json:"normalized_weight_g"`
	ReviewStatus      string  `json:"review_status"`
}

type Issue struct {
	IssueKey        string `json:"issue_key"`
	EntityType      string `json:"entity_type"`
	EntityKey       string `json:"entity_key"`
	Code            string `json:"code"`
	Severity        string `json:"severity"`
	Message         string `json:"message"`
	SourceOrderKey  string `json:"source_order_key"`
	SheetName       string `json:"sheet_name"`
	SourceRowNumber int    `json:"source_row_number"`
	ReviewStatus    string `json:"review_status"`
}

type Dataset struct {
	Run             ImportRun              `json:"run"`
	Sheets          []SheetInventory       `json:"sheets"`
	RawOrders       []RawOrder             `json:"raw_orders"`
	ERPCustomers    []ERPReferenceCustomer `json:"erp_customers"`
	ERPProducts     []ERPReferenceProduct  `json:"erp_products"`
	Customers       []Customer             `json:"customers"`
	CustomerAliases []CustomerAlias        `json:"customer_aliases"`
	CustomerPhones  []CustomerPhone        `json:"customer_phones"`
	Products        []Product              `json:"products"`
	SKUs            []SKU                  `json:"skus"`
	ProductAliases  []ProductAlias         `json:"product_aliases"`
	Orders          []Order                `json:"orders"`
	OrderItems      []OrderItem            `json:"order_items"`
	Issues          []Issue                `json:"issues"`
}

type PrepareOptions struct {
	StartPeriod      string
	EndPeriod        string
	PreviousMappings map[string]SourceKeyAssignment
	ERPCustomers     []ERPReferenceCustomer
	ERPProducts      []ERPReferenceProduct
}

type AmountResult struct {
	Raw         string
	Value       *float64
	Derived     bool
	NeedsReview bool
}

type ProductLine struct {
	RawLine           string  `json:"raw_line"`
	NormalizedLine    string  `json:"normalized_line"`
	ParentName        string  `json:"parent_name"`
	SpecName          string  `json:"spec_name"`
	ProductKind       string  `json:"product_kind"`
	RoastLevel        string  `json:"roast_level"`
	OrderQuantity     float64 `json:"order_quantity"`
	OrderUnit         string  `json:"order_unit"`
	NetContentQty     float64 `json:"net_content_qty"`
	NetContentUnit    string  `json:"net_content_unit"`
	NormalizedWeightG float64 `json:"normalized_weight_g"`
	NeedsReview       bool    `json:"needs_review"`
	ReviewReason      string  `json:"review_reason"`
}

type ReviewContract struct {
	SheetNames []string `json:"sheet_names"`
}
