package sales

import (
	"context"
	"fmt"
	salesdomain "orderapp/internal/domain/sales"
	"strings"
	"time"
)

type SaveOrderCommand struct {
	Actor                 string
	EditID                int64
	OrderDate             time.Time
	CustomerID            int64
	SourceID              int64
	OrderTypeID           int64
	PayStatusID           int64
	ShipStatusID          int64
	ShipMethod            string
	ShipTrackingNo        string
	Notes                 string
	ShippingAmount        float64
	DiscountAmount        float64
	RoundToInt            bool
	ExpressFee            string
	OutsourceMaterialFee  float64
	OutsourceRoastFee     float64
	OutsourcePackagingFee float64
	OutsourceManualFee    float64
	OutsourceTaxFee       float64
	OutsourceOtherFee     float64
	Items                 []OrderItemCommand
}

type OrderItemCommand struct {
	ProductID   *int64
	TierID      *int64
	ManualPrice *float64
	Name        string
	Units       int64
	Unit        string
	SpecG       int64
}

type SaveOrderResult struct {
	OrderID int64
	OrderNo string
	Edited  bool
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
	OrderDate             string
	CustomerID            int64
	SourceID              int64
	OrderTypeID           int64
	PayStatusID           int64
	ShipStatusID          int64
	ShipMethod            string
	ShipTrackingNo        string
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
	ShipStatusID    string
	ProcessStatusID string
	Notes           string
}

type OrderListQuery struct {
	Q               string
	From            string
	To              string
	Void            string
	CustomerID      int64
	PayStatusID     int64
	ShipStatusID    int64
	ProcessStatusID int64
	UnproducedOnly  bool
	CompletedOnly   bool
	Limit           int
	Offset          int
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
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	DefaultSourceID    int64  `json:"default_source_id"`
	DefaultOrderTypeID int64  `json:"default_order_type_id"`
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

type OrderFormData struct {
	Today        string
	Customers    []CustomerOption
	Sources      []Option
	ShipStatuses []Option
	PayStatuses  []Option
	OrderTypes   []Option
	Products     []ProductOption
	EditData     *OrderEditData
}

type OrderEditItem struct {
	ItemID      int64
	LineNo      int
	ProductID   int64
	Product     string
	Spec        string
	Qty         string
	Unit        string
	UnitPrice   string
	LineTotal   string
	PriceTierID int64
}

type OrderEditData struct {
	ID             int64
	OrderNo        string
	OrderDate      string
	CustomerID     int64
	SourceID       int64
	OrderTypeID    int64
	PayStatusID    int64
	ShipStatusID   int64
	ShipMethod     string
	ShipTrackingNo string
	Notes          string

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
	ID                int64  `json:"id"`
	OrderNo           string `json:"order_no"`
	OrderDate         string `json:"order_date"`
	CustomerID        int64  `json:"customer_id"`
	Customer          string `json:"customer"`
	GrandTotal        string `json:"grand_total"`
	OrderType         string `json:"order_type"`
	PayStatus         string `json:"pay_status"`
	ShipStatus        string `json:"ship_status"`
	ShipTrackingNo    string `json:"ship_tracking_no"`
	OrderTypeID       int64  `json:"order_type_id"`
	PayStatusID       int64  `json:"pay_status_id"`
	ShipStatusID      int64  `json:"ship_status_id"`
	ProcessStatusID   int64  `json:"process_status_id"`
	ProcessStatus     string `json:"process_status"`
	CreatedByEmployee string `json:"created_by_employee"`
	Notes             string `json:"notes"`
	IsVoid            bool   `json:"is_void"`
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

type FillShipmentTrackingResult struct {
	Updated int `json:"updated"`
	Total   int `json:"total"`
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
	CompanyName  string                  `json:"company_name"`
	Note         string                  `json:"note"`
	PaymentText  string                  `json:"payment_text"`
	Seal         *SalesOrderAsset        `json:"seal,omitempty"`
	PaymentCodes []SalesOrderPaymentCode `json:"payment_codes"`
}

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

type SaveSalesOrderSettingsCommand struct {
	Actor       string `json:"actor"`
	CompanyName string `json:"company_name"`
	Note        string `json:"note"`
	PaymentText string `json:"payment_text"`
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

type GenerateSalesOrderDocumentCommand struct {
	Actor   string
	OrderID int64
}

type GenerateSalesOrderDocumentResult struct {
	Document SalesOrderDocument             `json:"document"`
	Snapshot salesdomain.SalesOrderSnapshot `json:"snapshot"`
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

type SalesOrderDocumentFile struct {
	Document SalesOrderDocument
	Path     string
	Filename string
}

type Repository interface {
	SaveOrder(ctx context.Context, cmd SaveOrderCommand) (SaveOrderResult, error)
	UpdateHeader(ctx context.Context, id int64, cmd UpdateHeaderCommand) error
	InlineUpdate(ctx context.Context, id int64, actor string, cmd InlineUpdateCommand) error
	Void(ctx context.Context, id int64, actor, reason string) error
	Unvoid(ctx context.Context, id int64, actor string) error
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
	LoadSalesOrderSettings(ctx context.Context) (SalesOrderSettings, error)
	SaveSalesOrderSettings(ctx context.Context, cmd SaveSalesOrderSettingsCommand) error
	SaveSalesOrderAsset(ctx context.Context, cmd SaveSalesOrderAssetCommand) (SalesOrderAsset, error)
	SaveSalesOrderPaymentCode(ctx context.Context, cmd SaveSalesOrderPaymentCodeCommand) (SalesOrderPaymentCode, error)
	DeleteSalesOrderPaymentCode(ctx context.Context, id int64, actor string) error
	SetSalesOrderSealAsset(ctx context.Context, assetID int64, actor string) error
	ListSalesOrderDocuments(ctx context.Context, orderID int64) ([]SalesOrderDocument, error)
	GenerateSalesOrderDocument(ctx context.Context, cmd GenerateSalesOrderDocumentCommand) (GenerateSalesOrderDocumentResult, error)
	LoadSalesOrderDocumentFile(ctx context.Context, orderID, documentID int64, latest bool) (SalesOrderDocumentFile, error)
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
	return s.repo.SaveOrder(ctx, cmd)
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
		if item.SpecG <= 0 {
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

func (s *Service) UpdateHeader(ctx context.Context, id int64, cmd UpdateHeaderCommand) error {
	return s.repo.UpdateHeader(ctx, id, cmd)
}

func (s *Service) InlineUpdate(ctx context.Context, id int64, actor string, cmd InlineUpdateCommand) error {
	return s.repo.InlineUpdate(ctx, id, actor, cmd)
}

func (s *Service) Void(ctx context.Context, id int64, actor, reason string) error {
	return s.repo.Void(ctx, id, actor, reason)
}

func (s *Service) Unvoid(ctx context.Context, id int64, actor string) error {
	return s.repo.Unvoid(ctx, id, actor)
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
		tracking := strings.TrimSpace(pair.Tracking)
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
	seen := map[int64]bool{}
	items := make([]ShipmentTrackingItemCommand, 0, len(cmd.Items))
	for _, item := range cmd.Items {
		item.TrackingNo = strings.TrimSpace(item.TrackingNo)
		if item.OrderID <= 0 || item.TrackingNo == "" || seen[item.OrderID] {
			continue
		}
		seen[item.OrderID] = true
		items = append(items, item)
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
	seen := map[string]bool{}
	items := make([]ShipmentTrackingByOrderNoItemCommand, 0, len(cmd.Items))
	for _, item := range cmd.Items {
		item.OrderNo = strings.TrimSpace(item.OrderNo)
		item.TrackingNo = strings.TrimSpace(item.TrackingNo)
		if item.OrderNo == "" || item.TrackingNo == "" || seen[item.OrderNo] {
			continue
		}
		seen[item.OrderNo] = true
		items = append(items, item)
	}
	if len(items) == 0 {
		return FillShipmentTrackingResult{}, nil
	}
	cmd.Items = items
	return s.repo.FillShipmentTrackingByOrderNo(ctx, cmd)
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
	if cmd.CompanyName == "" {
		return fmt.Errorf("company_name required")
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

func (s *Service) LoadSalesOrderDocumentFile(ctx context.Context, orderID, documentID int64, latest bool) (SalesOrderDocumentFile, error) {
	if orderID <= 0 {
		return SalesOrderDocumentFile{}, fmt.Errorf("invalid order id")
	}
	if !latest && documentID <= 0 {
		return SalesOrderDocumentFile{}, fmt.Errorf("invalid document id")
	}
	return s.repo.LoadSalesOrderDocumentFile(ctx, orderID, documentID, latest)
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
