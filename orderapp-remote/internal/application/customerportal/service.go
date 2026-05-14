package customerportal

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	CapabilityMall             = "mall"
	CapabilityBeanList         = "bean_list"
	CapabilityProductOrder     = "product_order"
	CapabilityDirectShip       = "direct_ship"
	CapabilityProcessing       = "processing"
	CapabilityInventoryCustody = "inventory_custody"
	CapabilityShippingQuery    = "shipping_query"
	CapabilitySettlement       = "settlement"
)

const (
	CapabilityTemplateProcessingFulfillment = "processing_fulfillment"
	CapabilityTemplatePublicSKUDirectShip   = "public_sku_direct_ship"
	CapabilityTemplateRetailMall            = "retail_mall"
)

const (
	ServiceKeyBeanList     = "beanList"
	ServiceKeyOrders       = "orders"
	ServiceKeyProductOrder = "productOrder"
	ServiceKeyDirectShip   = "directShip"
	ServiceKeyProcessing   = "processing"
	ServiceKeyInventory    = "inventory"
	ServiceKeyShipping     = "shipping"
	ServiceKeySettlement   = "settlement"
)

const (
	PortalServiceProductOrder       = "product_order"
	PortalServiceDirectShip         = "direct_ship"
	PortalServiceProcessingShipment = "processing_ship"
	PortalServiceMall               = "mall"
)

const (
	PortalThemeCoffeeFactory  = "coffee_factory"
	PortalThemeCleanOps       = "clean_ops"
	PortalThemePremiumPartner = "premium_partner"
)

const (
	MiniappEntryModeServices = "services"
	MiniappEntryModeMall     = "mall"
)

const (
	MallProductStatusDraft     = "draft"
	MallProductStatusPublished = "published"
)

const (
	MallTemplateHero    = "hero"
	MallTemplateCompact = "compact"
	MallTemplateWide    = "wide"
)

var (
	ErrCustomerBindingNotFound                   = errors.New("customer binding not found")
	ErrMiniSessionNotFound                       = errors.New("mini session not found")
	ErrMiniLoginDisabled                         = errors.New("mini login disabled")
	ErrMiniUserDisabled                          = errors.New("mini user disabled")
	ErrMiniInvalidLogin                          = errors.New("mini invalid login")
	ErrMiniAccountLoginDisabled                  = errors.New("mini account login disabled")
	ErrCapabilityNotEnabled                      = errors.New("capability not enabled")
	ErrPortalCustomerNotFound                    = errors.New("portal customer not found")
	ErrBeanListPublicationNotFound               = errors.New("bean list publication not found")
	ErrCapabilityTemplateInvalid                 = errors.New("capability template invalid")
	ErrCapabilityTemplateERPWorkbenchUnavailable = errors.New("ERP workbench unavailable for capability template")
)

type LoginCommand struct {
	Code     string
	Phone    string
	Nickname string
}

type PasswordLoginCommand struct {
	Login    string
	Password string
}

type MiniIdentity struct {
	OpenID  string
	UnionID string
}

type CreateLoginSessionCommand struct {
	OpenID   string
	UnionID  string
	Phone    string
	Nickname string
}

type CreatePasswordLoginSessionCommand struct {
	Login    string
	Password string
}

type LoginResult struct {
	Token             string            `json:"token"`
	MiniUserID        int64             `json:"mini_user_id"`
	CurrentCustomerID int64             `json:"current_customer_id"`
	ThemeKey          string            `json:"theme_key"`
	MiniappEntryMode  string            `json:"miniapp_entry_mode"`
	Bindings          []CustomerBinding `json:"bindings"`
	Capabilities      []Capability      `json:"capabilities"`
}

type CustomerBinding struct {
	CustomerID   int64  `json:"customer_id"`
	CustomerName string `json:"customer_name"`
	Role         string `json:"role"`
	Status       string `json:"status"`
}

type Capability struct {
	Code    string         `json:"code"`
	Enabled bool           `json:"enabled"`
	Config  map[string]any `json:"config,omitempty"`
}

type CurrentContext struct {
	MiniUserID          int64             `json:"mini_user_id"`
	CurrentCustomerID   int64             `json:"current_customer_id"`
	CurrentCustomerName string            `json:"current_customer_name"`
	ThemeKey            string            `json:"theme_key"`
	MiniappEntryMode    string            `json:"miniapp_entry_mode"`
	Bindings            []CustomerBinding `json:"bindings"`
	Capabilities        []Capability      `json:"capabilities"`
}

type ServiceMetric struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type BeanListSummary struct {
	ID                  int64                  `json:"id"`
	ListType            string                 `json:"list_type"`
	VersionNo           string                 `json:"version_no"`
	Status              string                 `json:"status"`
	PublishedAt         string                 `json:"published_at"`
	Changelog           string                 `json:"changelog"`
	PDFURL              string                 `json:"pdf_url,omitempty"`
	CacheKey            string                 `json:"cache_key,omitempty"`
	Title               string                 `json:"title,omitempty"`
	Subtitle            string                 `json:"subtitle,omitempty"`
	ListTypeLabel       string                 `json:"list_type_label,omitempty"`
	BrandName           string                 `json:"brand_name,omitempty"`
	BrandIntro          string                 `json:"brand_intro,omitempty"`
	LayoutStyle         string                 `json:"layout_style,omitempty"`
	CardsPerRow         int                    `json:"cards_per_row,omitempty"`
	ShowVersion         bool                   `json:"show_version"`
	ShowChangelog       bool                   `json:"show_changelog"`
	ShowCategoryNumbers bool                   `json:"show_category_numbers"`
	BackgroundColor     string                 `json:"background_color,omitempty"`
	FontColor           string                 `json:"font_color,omitempty"`
	BackgroundImage     string                 `json:"background_image,omitempty"`
	LogoImage           string                 `json:"logo_image,omitempty"`
	Groups              []BeanListGroupSummary `json:"groups,omitempty"`
}

type BeanListGroupSummary struct {
	Category     string                   `json:"category"`
	ShowCategory bool                     `json:"show_category"`
	Items        []BeanListProductSummary `json:"items"`
}

type BeanListProductSummary struct {
	Code           string                 `json:"code,omitempty"`
	Name           string                 `json:"name"`
	Badge          string                 `json:"badge,omitempty"`
	BadgeLabel     string                 `json:"badge_label,omitempty"`
	RecommendedUse string                 `json:"recommended_use,omitempty"`
	Flavor         string                 `json:"flavor,omitempty"`
	Description    string                 `json:"description,omitempty"`
	HighlightTerms []string               `json:"highlight_terms,omitempty"`
	Prices         []BeanListPriceSummary `json:"prices,omitempty"`
}

type BeanListPriceSummary struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Red   bool   `json:"red,omitempty"`
}

type ProductSummary struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	RoastLevel     string `json:"roast_level"`
	DefaultPrice   string `json:"default_price"`
	RetailPrice100 string `json:"retail_price_100g"`
	RetailPrice200 string `json:"retail_price_200g"`
	RetailPrice227 string `json:"retail_price_227g"`
	RetailPrice250 string `json:"retail_price_250g"`
}

type MallProduct struct {
	ID          int64   `json:"id"`
	ProductID   int64   `json:"product_id"`
	ProductName string  `json:"product_name"`
	Title       string  `json:"title"`
	Subtitle    string  `json:"subtitle"`
	Description string  `json:"description"`
	ImageURL    string  `json:"image_url"`
	SpecG       int64   `json:"spec_g"`
	UnitPrice   float64 `json:"unit_price"`
	TemplateKey string  `json:"template_key"`
	Status      string  `json:"status"`
	SortOrder   int     `json:"sort_order"`
	UpdatedAt   string  `json:"updated_at"`
}

type MallProductOption struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	DefaultPrice float64 `json:"default_price"`
}

type MallPage struct {
	ThemeKey            string        `json:"theme_key"`
	MiniappEntryMode    string        `json:"miniapp_entry_mode"`
	CurrentCustomerID   int64         `json:"current_customer_id"`
	CurrentCustomerName string        `json:"current_customer_name"`
	Products            []MallProduct `json:"products"`
}

type SaveMallProductCommand struct {
	ID          int64
	ProductID   int64
	Title       string
	Subtitle    string
	Description string
	ImageURL    string
	SpecG       int64
	UnitPrice   float64
	TemplateKey string
	Status      string
	SortOrder   int
	Actor       string
}

type UpdateMallProductImageCommand struct {
	ID       int64
	ImageURL string
	Actor    string
}

type MallOrderItemCommand struct {
	MallProductID int64 `json:"mall_product_id"`
	Qty           int64 `json:"qty"`
}

type CreateMallOrderCommand struct {
	CustomerID          int64
	CreatedByMiniUserID int64
	RecipientName       string
	RecipientPhone      string
	RecipientAddress    string
	RecipientCompany    string
	ShippingAmount      float64
	Note                string
	Items               []MallOrderItemCommand
}

type CustomerOrderSummary struct {
	ID              int64                      `json:"id"`
	OrderNo         string                     `json:"order_no"`
	OrderDate       string                     `json:"order_date"`
	ReceiverName    string                     `json:"receiver_name"`
	ReceiverPhone   string                     `json:"receiver_phone"`
	ReceiverAddress string                     `json:"receiver_address"`
	ProcessStatus   string                     `json:"process_status"`
	PayStatus       string                     `json:"pay_status"`
	ShipStatus      string                     `json:"ship_status"`
	ShipTrackingNo  string                     `json:"ship_tracking_no"`
	GrandTotal      string                     `json:"grand_total"`
	ShippingAmount  string                     `json:"shipping_amount"`
	SalesOrderURL   string                     `json:"sales_order_url,omitempty"`
	DeliveryNoteURL string                     `json:"delivery_note_url,omitempty"`
	Items           []CustomerOrderItemSummary `json:"items,omitempty"`
}

type CustomerOrderItemSummary struct {
	ID        int64  `json:"id"`
	ItemName  string `json:"item_name"`
	Spec      string `json:"spec"`
	Qty       string `json:"qty"`
	Unit      string `json:"unit"`
	UnitPrice string `json:"unit_price"`
	LineTotal string `json:"line_total"`
}

type DirectShipBatch struct {
	ID          int64  `json:"id"`
	BatchNo     string `json:"batch_no"`
	SourceName  string `json:"source_name"`
	Status      string `json:"status"`
	TotalRows   int    `json:"total_rows"`
	ValidRows   int    `json:"valid_rows"`
	InvalidRows int    `json:"invalid_rows"`
	Note        string `json:"note"`
	CreatedAt   string `json:"created_at"`
}

type InventoryItem struct {
	ID        int64  `json:"id"`
	ItemType  string `json:"item_type"`
	ItemID    int64  `json:"item_id"`
	ItemName  string `json:"item_name"`
	SpecG     int64  `json:"spec_g"`
	Warehouse string `json:"warehouse"`
	QtyG      int64  `json:"qty_g"`
	QtyUnits  int64  `json:"qty_units"`
	Status    string `json:"status"`
	Note      string `json:"note"`
	UpdatedAt string `json:"updated_at"`
}

type ProcessingRequest struct {
	ID                int64  `json:"id"`
	RequestNo         string `json:"request_no"`
	InputMaterialID   int64  `json:"input_material_id"`
	InputMaterialName string `json:"input_material_name"`
	InputQtyG         int64  `json:"input_qty_g"`
	TargetProductID   int64  `json:"target_product_id"`
	TargetProductName string `json:"target_product_name"`
	TargetSpecG       int64  `json:"target_spec_g"`
	TargetQty         int    `json:"target_qty"`
	Status            string `json:"status"`
	Note              string `json:"note"`
	CreatedAt         string `json:"created_at"`
	AcceptedAt        string `json:"accepted_at"`
	LinkedWorkOrderID int64  `json:"linked_work_order_id"`
}

type FulfillmentOrder struct {
	OrderID           int64  `json:"order_id"`
	OrderNo           string `json:"order_no"`
	PortalServiceCode string `json:"portal_service_code"`
	SourceWarehouse   string `json:"source_warehouse"`
}

type FeeItem struct {
	ID                int64  `json:"id"`
	SourceType        string `json:"source_type"`
	SourceID          int64  `json:"source_id"`
	FeeType           string `json:"fee_type"`
	Amount            string `json:"amount"`
	Currency          string `json:"currency"`
	OccurredAt        string `json:"occurred_at"`
	SettlementBatchID int64  `json:"settlement_batch_id"`
	Status            string `json:"status"`
	Note              string `json:"note"`
}

type SettlementBatch struct {
	ID           int64  `json:"id"`
	SettlementNo string `json:"settlement_no"`
	PeriodFrom   string `json:"period_from"`
	PeriodTo     string `json:"period_to"`
	Status       string `json:"status"`
	TotalAmount  string `json:"total_amount"`
	ConfirmedAt  string `json:"confirmed_at"`
	PaidAt       string `json:"paid_at"`
	CreatedAt    string `json:"created_at"`
}

type ServicePage struct {
	Key                 string                 `json:"key"`
	Title               string                 `json:"title"`
	Capability          string                 `json:"capability"`
	ThemeKey            string                 `json:"theme_key"`
	MiniappEntryMode    string                 `json:"miniapp_entry_mode"`
	CurrentCustomerID   int64                  `json:"current_customer_id"`
	CurrentCustomerName string                 `json:"current_customer_name"`
	Summary             []ServiceMetric        `json:"summary"`
	BeanLists           []BeanListSummary      `json:"bean_lists,omitempty"`
	Products            []ProductSummary       `json:"products,omitempty"`
	Orders              []CustomerOrderSummary `json:"orders,omitempty"`
	DirectShipBatches   []DirectShipBatch      `json:"direct_ship_batches,omitempty"`
	Inventory           []InventoryItem        `json:"inventory,omitempty"`
	ProcessingRequests  []ProcessingRequest    `json:"processing_requests,omitempty"`
	FeeItems            []FeeItem              `json:"fee_items,omitempty"`
	SettlementBatches   []SettlementBatch      `json:"settlement_batches,omitempty"`
}

type ServicePageQuery struct {
	CustomerID    int64
	Key           string
	Limit         int
	Query         string
	DateFrom      string
	DateTo        string
	ProcessStatus string
	PayStatus     string
	ShipStatus    string
}

type ServicePageFilter struct {
	Query         string
	DateFrom      string
	DateTo        string
	ProcessStatus string
	PayStatus     string
	ShipStatus    string
}

type CreateDirectShipBatchCommand struct {
	CustomerID          int64
	CreatedByMiniUserID int64
	SourceName          string
	TotalRows           int
	Note                string
}

type CreateProcessingRequestCommand struct {
	CustomerID          int64
	CreatedByMiniUserID int64
	InputMaterialID     int64
	InputQtyG           int64
	TargetProductID     int64
	TargetSpecG         int64
	TargetQty           int
	Note                string
}

type CreateFulfillmentOrderCommand struct {
	CustomerID          int64
	CreatedByMiniUserID int64
	PortalServiceCode   string
	RecipientName       string
	RecipientPhone      string
	RecipientAddress    string
	RecipientCompany    string
	ProductID           int64
	ProductName         string
	SpecG               int64
	Qty                 int64
	UnitPrice           float64
	ShippingAmount      float64
	Note                string
}

type CapabilityOption struct {
	Code        string         `json:"code"`
	Label       string         `json:"label"`
	Description string         `json:"description,omitempty"`
	Enabled     bool           `json:"enabled"`
	Config      map[string]any `json:"config,omitempty"`
}

type SmallBatchPriceRule struct {
	Enabled     bool    `json:"enabled"`
	ThresholdLB float64 `json:"threshold_lb"`
	TierMinLB   float64 `json:"tier_min_lb"`
	TierMaxLB   float64 `json:"tier_max_lb"`
}

type CapabilityTemplate struct {
	Key               string             `json:"key"`
	ParentTemplateKey string             `json:"parent_template_key,omitempty"`
	Label             string             `json:"label"`
	Description       string             `json:"description,omitempty"`
	ThemeKey          string             `json:"theme_key"`
	MiniappEntryMode  string             `json:"miniapp_entry_mode"`
	ERPRoleCodes      []string           `json:"erp_role_codes"`
	ERPPermissions    []string           `json:"erp_permissions"`
	ERPViewKeys       []string           `json:"erp_view_keys"`
	Capabilities      []CapabilityOption `json:"capabilities"`
	Active            bool               `json:"active"`
	SortOrder         int                `json:"sort_order,omitempty"`
	UpdatedAt         string             `json:"updated_at,omitempty"`
	UpdatedBy         string             `json:"updated_by,omitempty"`
}

type PortalAdminCustomerQuery struct {
	Query string
	Limit int
}

type PortalAdminCustomer struct {
	ID                      int64             `json:"id"`
	Name                    string            `json:"name"`
	CustomerType            string            `json:"customer_type"`
	Phone                   string            `json:"phone"`
	CompanyName             string            `json:"company_name"`
	DisplayName             string            `json:"display_name"`
	ProcessingWarehouseCode string            `json:"processing_warehouse_code"`
	DefaultSenderID         int64             `json:"default_sender_id"`
	PortalEnabled           bool              `json:"portal_enabled"`
	PortalStatus            string            `json:"portal_status"`
	ThemeKey                string            `json:"theme_key"`
	MiniappEntryMode        string            `json:"miniapp_entry_mode"`
	CapabilityTemplateKey   string            `json:"capability_template_key"`
	BindingCount            int               `json:"binding_count"`
	ERPBinding              *PortalERPBinding `json:"erp_binding,omitempty"`
}

type PortalERPBinding struct {
	CustomerID   int64  `json:"customer_id"`
	EmployeeID   int64  `json:"employee_id"`
	EmployeeName string `json:"employee_name"`
	Phone        string `json:"phone"`
	Role         string `json:"role"`
	Status       string `json:"status"`
	UpdatedBy    string `json:"updated_by,omitempty"`
	UpdatedAt    string `json:"updated_at,omitempty"`
}

type PortalUserBinding struct {
	MiniUserID int64  `json:"mini_user_id"`
	OpenID     string `json:"openid,omitempty"`
	Phone      string `json:"phone"`
	Nickname   string `json:"nickname"`
	Role       string `json:"role"`
	Status     string `json:"status"`
	CreatedAt  string `json:"created_at"`
}

type PortalAdminDetail struct {
	Customer     PortalAdminCustomer `json:"customer"`
	Bindings     []PortalUserBinding `json:"bindings"`
	Capabilities []CapabilityOption  `json:"capabilities"`
}

type UpdatePortalVisibilityCommand struct {
	CustomerID              int64
	DisplayName             string
	ProcessingWarehouseCode string
	DefaultSenderID         int64
	Enabled                 bool
	ThemeKey                string
	MiniappEntryMode        string
	CapabilityTemplateKey   string
	Template                CapabilityTemplate
	Capabilities            []CapabilityOption
	UpdatedBy               string
}

type ApplyCapabilityTemplateCommand struct {
	CustomerID  int64
	TemplateKey string
	Template    CapabilityTemplate
	UpdatedBy   string
}

type SaveCapabilityTemplateCommand struct {
	Template  CapabilityTemplate
	UpdatedBy string
	ActiveSet bool
}

type CopyCapabilityTemplateCommand struct {
	SourceKey string
	NewKey    string
	Label     string
	UpdatedBy string
}

type UpsertPortalERPBindingCommand struct {
	CustomerID int64
	EmployeeID int64
	Status     string
	UpdatedBy  string
}

func (d PortalAdminDetail) HasCapabilityOption(code string) bool {
	code = strings.TrimSpace(code)
	for _, capability := range d.Capabilities {
		if capability.Code == code {
			return true
		}
	}
	return false
}

func (c CurrentContext) HasCapability(code string) bool {
	code = strings.TrimSpace(code)
	for _, capability := range c.Capabilities {
		if capability.Enabled && capability.Code == code {
			return true
		}
	}
	return false
}

func (c CurrentContext) HasAnyCapability(codes []string) bool {
	for _, code := range codes {
		if c.HasCapability(code) {
			return true
		}
	}
	return false
}

type IdentityProvider interface {
	Resolve(ctx context.Context, code string) (MiniIdentity, error)
}

type Repository interface {
	CreateLoginSession(ctx context.Context, cmd CreateLoginSessionCommand) (LoginResult, error)
	CreatePasswordLoginSession(ctx context.Context, cmd CreatePasswordLoginSessionCommand) (LoginResult, error)
	CurrentContextByToken(ctx context.Context, token string) (CurrentContext, error)
	SwitchCurrentCustomer(ctx context.Context, token string, customerID int64) (CurrentContext, error)
	LoadServicePage(ctx context.Context, query ServicePageQuery) (ServicePage, error)
	LoadBeanListPublication(ctx context.Context, customerID, publicationID int64) (BeanListSummary, error)
	ListPortalAdminCustomers(ctx context.Context, query PortalAdminCustomerQuery) ([]PortalAdminCustomer, error)
	PortalAdminDetail(ctx context.Context, customerID int64) (PortalAdminDetail, error)
	UpdatePortalVisibility(ctx context.Context, cmd UpdatePortalVisibilityCommand) (PortalAdminDetail, error)
	ListCapabilityTemplates(ctx context.Context) ([]CapabilityTemplate, error)
	SaveCapabilityTemplate(ctx context.Context, cmd SaveCapabilityTemplateCommand) (CapabilityTemplate, error)
	ApplyCapabilityTemplate(ctx context.Context, cmd ApplyCapabilityTemplateCommand) (PortalAdminDetail, error)
	UpsertPortalERPBinding(ctx context.Context, cmd UpsertPortalERPBindingCommand) (PortalAdminDetail, error)
	ListMallProducts(ctx context.Context) ([]MallProduct, []MallProductOption, error)
	SaveMallProduct(ctx context.Context, cmd SaveMallProductCommand) (MallProduct, error)
	UpdateMallProductImage(ctx context.Context, cmd UpdateMallProductImageCommand) (MallProduct, error)
	LoadMallPage(ctx context.Context, customerID int64) (MallPage, error)
	CustomerOwnsOrder(ctx context.Context, customerID, orderID int64) (bool, error)
	CreateMallOrder(ctx context.Context, cmd CreateMallOrderCommand) (FulfillmentOrder, error)
	CreateDirectShipBatch(ctx context.Context, cmd CreateDirectShipBatchCommand) (DirectShipBatch, error)
	CreateProcessingRequest(ctx context.Context, cmd CreateProcessingRequestCommand) (ProcessingRequest, error)
	CreateFulfillmentOrder(ctx context.Context, cmd CreateFulfillmentOrderCommand) (FulfillmentOrder, error)
}

type Service struct {
	repo     Repository
	identity IdentityProvider
}

func NewService(repo Repository, identity IdentityProvider) *Service {
	return &Service{repo: repo, identity: identity}
}

func (s *Service) Login(ctx context.Context, cmd LoginCommand) (LoginResult, error) {
	code := strings.TrimSpace(cmd.Code)
	if code == "" {
		return LoginResult{}, fmt.Errorf("code required")
	}
	if s.repo == nil {
		return LoginResult{}, fmt.Errorf("repository required")
	}
	if s.identity == nil {
		return LoginResult{}, fmt.Errorf("identity provider required")
	}
	identity, err := s.identity.Resolve(ctx, code)
	if err != nil {
		return LoginResult{}, err
	}
	identity.OpenID = strings.TrimSpace(identity.OpenID)
	if identity.OpenID == "" {
		return LoginResult{}, fmt.Errorf("openid required")
	}
	result, err := s.repo.CreateLoginSession(ctx, CreateLoginSessionCommand{
		OpenID:   identity.OpenID,
		UnionID:  strings.TrimSpace(identity.UnionID),
		Phone:    strings.TrimSpace(cmd.Phone),
		Nickname: strings.TrimSpace(cmd.Nickname),
	})
	if err != nil {
		return LoginResult{}, err
	}
	return normalizeLoginResult(result), nil
}

func (s *Service) LoginWithPassword(ctx context.Context, cmd PasswordLoginCommand) (LoginResult, error) {
	login := strings.TrimSpace(cmd.Login)
	if login == "" {
		return LoginResult{}, fmt.Errorf("login required")
	}
	password := strings.TrimSpace(cmd.Password)
	if password == "" {
		return LoginResult{}, fmt.Errorf("password required")
	}
	if s.repo == nil {
		return LoginResult{}, fmt.Errorf("repository required")
	}
	result, err := s.repo.CreatePasswordLoginSession(ctx, CreatePasswordLoginSessionCommand{
		Login:    login,
		Password: password,
	})
	if err != nil {
		return LoginResult{}, err
	}
	return normalizeLoginResult(result), nil
}

func (s *Service) Me(ctx context.Context, token string) (CurrentContext, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return CurrentContext{}, fmt.Errorf("mini token required")
	}
	if s.repo == nil {
		return CurrentContext{}, fmt.Errorf("repository required")
	}
	current, err := s.repo.CurrentContextByToken(ctx, token)
	if err != nil {
		return CurrentContext{}, err
	}
	return normalizeCurrentContext(current), nil
}

func (s *Service) SwitchCurrentCustomer(ctx context.Context, token string, customerID int64) (CurrentContext, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return CurrentContext{}, fmt.Errorf("mini token required")
	}
	if customerID <= 0 {
		return CurrentContext{}, fmt.Errorf("customer required")
	}
	if s.repo == nil {
		return CurrentContext{}, fmt.Errorf("repository required")
	}
	current, err := s.repo.SwitchCurrentCustomer(ctx, token, customerID)
	if err != nil {
		return CurrentContext{}, err
	}
	return normalizeCurrentContext(current), nil
}

func (s *Service) EnsureOrderAccess(ctx context.Context, token string, orderID int64) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("mini token required")
	}
	if orderID <= 0 {
		return fmt.Errorf("order required")
	}
	if s.repo == nil {
		return fmt.Errorf("repository required")
	}
	current, err := s.Me(ctx, token)
	if err != nil {
		return err
	}
	if current.CurrentCustomerID <= 0 {
		return ErrCustomerBindingNotFound
	}
	ok, err := s.repo.CustomerOwnsOrder(ctx, current.CurrentCustomerID, orderID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrCustomerBindingNotFound
	}
	return nil
}

func (s *Service) GetServicePage(ctx context.Context, token, key string, filter ServicePageFilter) (ServicePage, error) {
	def, err := serviceDefinition(key)
	if err != nil {
		return ServicePage{}, err
	}
	current, err := s.Me(ctx, token)
	if err != nil {
		return ServicePage{}, err
	}
	if current.CurrentCustomerID <= 0 {
		return ServicePage{}, ErrCustomerBindingNotFound
	}
	if !current.HasAnyCapability(def.capabilities) {
		return ServicePage{}, ErrCapabilityNotEnabled
	}
	filter = normalizeServicePageFilter(filter)
	limit := 20
	if serviceKeyContainsOrders(def.key) {
		limit = 50
	}
	page, err := s.repo.LoadServicePage(ctx, ServicePageQuery{
		CustomerID:    current.CurrentCustomerID,
		Key:           def.key,
		Limit:         limit,
		Query:         filter.Query,
		DateFrom:      filter.DateFrom,
		DateTo:        filter.DateTo,
		ProcessStatus: filter.ProcessStatus,
		PayStatus:     filter.PayStatus,
		ShipStatus:    filter.ShipStatus,
	})
	if err != nil {
		return ServicePage{}, err
	}
	page.Key = def.key
	page.Title = def.title
	page.Capability = def.capability
	page.ThemeKey = NormalizePortalThemeKey(current.ThemeKey)
	page.MiniappEntryMode = NormalizeMiniappEntryMode(current.MiniappEntryMode)
	page.CurrentCustomerID = current.CurrentCustomerID
	page.CurrentCustomerName = current.CurrentCustomerName
	page.Summary = serviceSummary(page)
	return page, nil
}

func (s *Service) GetBeanListPublication(ctx context.Context, token string, publicationID int64) (BeanListSummary, error) {
	if publicationID <= 0 {
		return BeanListSummary{}, fmt.Errorf("bean_list required")
	}
	current, err := s.requireCustomerCapability(ctx, token, CapabilityBeanList)
	if err != nil {
		return BeanListSummary{}, err
	}
	return s.repo.LoadBeanListPublication(ctx, current.CurrentCustomerID, publicationID)
}

func (s *Service) ListPortalAdminCustomers(ctx context.Context, query PortalAdminCustomerQuery) ([]PortalAdminCustomer, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("repository required")
	}
	query.Query = strings.TrimSpace(query.Query)
	if query.Limit <= 0 {
		query.Limit = 20
	}
	if query.Limit > 100 {
		query.Limit = 100
	}
	rows, err := s.repo.ListPortalAdminCustomers(ctx, query)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i] = normalizePortalAdminCustomer(rows[i])
	}
	return rows, nil
}

func (s *Service) PortalAdminDetail(ctx context.Context, customerID int64) (PortalAdminDetail, error) {
	if customerID <= 0 {
		return PortalAdminDetail{}, fmt.Errorf("customer required")
	}
	if s.repo == nil {
		return PortalAdminDetail{}, fmt.Errorf("repository required")
	}
	detail, err := s.repo.PortalAdminDetail(ctx, customerID)
	if err != nil {
		return PortalAdminDetail{}, err
	}
	detail.Customer = normalizePortalAdminCustomer(detail.Customer)
	if template, ok := s.capabilityTemplateByKey(ctx, detail.Customer.CapabilityTemplateKey); ok {
		detail.Customer.CapabilityTemplateKey = template.Key
		detail.Customer.ThemeKey = template.ThemeKey
		detail.Customer.MiniappEntryMode = template.MiniappEntryMode
		detail.Capabilities = cloneCapabilityOptions(template.Capabilities)
	}
	detail.Capabilities = completeCapabilityOptions(detail.Capabilities)
	return detail, nil
}

func (s *Service) UpdatePortalVisibility(ctx context.Context, cmd UpdatePortalVisibilityCommand) (PortalAdminDetail, error) {
	if cmd.CustomerID <= 0 {
		return PortalAdminDetail{}, fmt.Errorf("customer required")
	}
	if s.repo == nil {
		return PortalAdminDetail{}, fmt.Errorf("repository required")
	}
	cmd.DisplayName = strings.TrimSpace(cmd.DisplayName)
	cmd.ProcessingWarehouseCode = strings.TrimSpace(cmd.ProcessingWarehouseCode)
	cmd.UpdatedBy = strings.TrimSpace(cmd.UpdatedBy)
	rawTemplateKey := strings.TrimSpace(cmd.CapabilityTemplateKey)
	cmd.CapabilityTemplateKey = NormalizeCapabilityTemplateKey(cmd.CapabilityTemplateKey)
	if rawTemplateKey != "" && cmd.CapabilityTemplateKey == "" {
		return PortalAdminDetail{}, ErrCapabilityTemplateInvalid
	}
	if cmd.CapabilityTemplateKey != "" {
		template, ok := s.capabilityTemplateByKey(ctx, cmd.CapabilityTemplateKey)
		if !ok {
			return PortalAdminDetail{}, ErrCapabilityTemplateInvalid
		}
		cmd.Template = template
		cmd.ThemeKey = template.ThemeKey
		cmd.MiniappEntryMode = template.MiniappEntryMode
		cmd.Capabilities = cloneCapabilityOptions(template.Capabilities)
	} else {
		cmd.ThemeKey = NormalizePortalThemeKey(cmd.ThemeKey)
		cmd.MiniappEntryMode = NormalizeMiniappEntryMode(cmd.MiniappEntryMode)
		cmd.Capabilities = normalizeCapabilityOptions(cmd.Capabilities)
	}
	detail, err := s.repo.UpdatePortalVisibility(ctx, cmd)
	if err != nil {
		return PortalAdminDetail{}, err
	}
	detail.Customer = normalizePortalAdminCustomer(detail.Customer)
	detail.Capabilities = completeCapabilityOptions(detail.Capabilities)
	return detail, nil
}

func (s *Service) ListCapabilityTemplates(ctx context.Context) ([]CapabilityTemplate, error) {
	if s.repo == nil {
		return DefaultCapabilityTemplates(), nil
	}
	rows, err := s.repo.ListCapabilityTemplates(ctx)
	if err != nil {
		return nil, err
	}
	return mergeCapabilityTemplates(rows), nil
}

func (s *Service) SaveCapabilityTemplate(ctx context.Context, cmd SaveCapabilityTemplateCommand) (CapabilityTemplate, error) {
	if s.repo == nil {
		return CapabilityTemplate{}, fmt.Errorf("repository required")
	}
	if !cmd.ActiveSet && !cmd.Template.Active {
		cmd.Template.Active = true
	}
	if err := s.validateCapabilityTemplateParent(ctx, cmd.Template); err != nil {
		return CapabilityTemplate{}, err
	}
	if templateERPWorkbenchDisallowed(cmd.Template) && (hasNonEmptyString(cmd.Template.ERPPermissions) || hasNonEmptyString(cmd.Template.ERPViewKeys)) {
		return CapabilityTemplate{}, ErrCapabilityTemplateERPWorkbenchUnavailable
	}
	template, ok := normalizeCapabilityTemplate(cmd.Template)
	if !ok {
		return CapabilityTemplate{}, ErrCapabilityTemplateInvalid
	}
	cmd.Template = template
	cmd.UpdatedBy = strings.TrimSpace(cmd.UpdatedBy)
	row, err := s.repo.SaveCapabilityTemplate(ctx, cmd)
	if err != nil {
		return CapabilityTemplate{}, err
	}
	normalized, ok := normalizeCapabilityTemplate(row)
	if !ok {
		return CapabilityTemplate{}, ErrCapabilityTemplateInvalid
	}
	return normalized, nil
}

func (s *Service) CopyCapabilityTemplate(ctx context.Context, cmd CopyCapabilityTemplateCommand) (CapabilityTemplate, error) {
	if s.repo == nil {
		return CapabilityTemplate{}, fmt.Errorf("repository required")
	}
	source, ok := s.capabilityTemplateByKey(ctx, cmd.SourceKey)
	if !ok {
		return CapabilityTemplate{}, ErrCapabilityTemplateInvalid
	}
	newKey := NormalizeCapabilityTemplateKey(cmd.NewKey)
	if strings.TrimSpace(cmd.NewKey) != "" && newKey == "" {
		return CapabilityTemplate{}, ErrCapabilityTemplateInvalid
	}
	if newKey == "" {
		newKey = s.nextCapabilityTemplateCopyKey(ctx, source.Key)
	}
	if newKey == "" || newKey == source.Key || s.capabilityTemplateExists(ctx, newKey) {
		return CapabilityTemplate{}, ErrCapabilityTemplateInvalid
	}
	sourceKey := source.Key
	source.Key = newKey
	source.ParentTemplateKey = sourceKey
	source.Label = firstNonEmpty(strings.TrimSpace(cmd.Label), source.Label+" 副本")
	source.Active = true
	source.SortOrder = source.SortOrder + 1
	source.UpdatedAt = ""
	source.UpdatedBy = ""
	return s.SaveCapabilityTemplate(ctx, SaveCapabilityTemplateCommand{
		Template:  source,
		UpdatedBy: cmd.UpdatedBy,
		ActiveSet: true,
	})
}

func (s *Service) nextCapabilityTemplateCopyKey(ctx context.Context, sourceKey string) string {
	sourceKey = NormalizeCapabilityTemplateKey(sourceKey)
	if sourceKey == "" {
		return ""
	}
	rows, err := s.ListCapabilityTemplates(ctx)
	if err != nil {
		return ""
	}
	exists := make(map[string]bool, len(rows))
	for _, row := range rows {
		if key := NormalizeCapabilityTemplateKey(row.Key); key != "" {
			exists[key] = true
		}
	}
	for index := 1; index <= 999; index++ {
		suffix := "_copy"
		if index > 1 {
			suffix = fmt.Sprintf("_copy_%d", index)
		}
		base := sourceKey
		if maxBaseLength := 64 - len(suffix); len(base) > maxBaseLength {
			base = strings.TrimRight(base[:maxBaseLength], "_")
		}
		candidate := NormalizeCapabilityTemplateKey(base + suffix)
		if candidate != "" && !exists[candidate] {
			return candidate
		}
	}
	return ""
}

func (s *Service) ApplyCapabilityTemplate(ctx context.Context, cmd ApplyCapabilityTemplateCommand) (PortalAdminDetail, error) {
	if cmd.CustomerID <= 0 {
		return PortalAdminDetail{}, fmt.Errorf("customer required")
	}
	if s.repo == nil {
		return PortalAdminDetail{}, fmt.Errorf("repository required")
	}
	template, ok := s.capabilityTemplateByKey(ctx, cmd.TemplateKey)
	if !ok {
		return PortalAdminDetail{}, ErrCapabilityTemplateInvalid
	}
	cmd.TemplateKey = template.Key
	cmd.Template = template
	cmd.UpdatedBy = strings.TrimSpace(cmd.UpdatedBy)
	detail, err := s.repo.ApplyCapabilityTemplate(ctx, cmd)
	if err != nil {
		return PortalAdminDetail{}, err
	}
	detail.Customer = normalizePortalAdminCustomer(detail.Customer)
	detail.Capabilities = completeCapabilityOptions(detail.Capabilities)
	return detail, nil
}

func (s *Service) UpsertPortalERPBinding(ctx context.Context, cmd UpsertPortalERPBindingCommand) (PortalAdminDetail, error) {
	if cmd.CustomerID <= 0 {
		return PortalAdminDetail{}, fmt.Errorf("customer required")
	}
	if cmd.EmployeeID <= 0 {
		return PortalAdminDetail{}, fmt.Errorf("employee required")
	}
	if s.repo == nil {
		return PortalAdminDetail{}, fmt.Errorf("repository required")
	}
	cmd.Status = normalizePortalERPBindingStatus(cmd.Status)
	if cmd.Status == "active" {
		detail, err := s.repo.PortalAdminDetail(ctx, cmd.CustomerID)
		if err != nil {
			return PortalAdminDetail{}, err
		}
		template, ok, err := s.capabilityTemplateByKeyStrict(ctx, detail.Customer.CapabilityTemplateKey)
		if err != nil {
			return PortalAdminDetail{}, err
		}
		if strings.TrimSpace(detail.Customer.CapabilityTemplateKey) != "" && !ok {
			return PortalAdminDetail{}, ErrCapabilityTemplateERPWorkbenchUnavailable
		}
		if ok && !template.ExposesERPWorkbench() {
			return PortalAdminDetail{}, ErrCapabilityTemplateERPWorkbenchUnavailable
		}
	}
	cmd.UpdatedBy = strings.TrimSpace(cmd.UpdatedBy)
	detail, err := s.repo.UpsertPortalERPBinding(ctx, cmd)
	if err != nil {
		return PortalAdminDetail{}, err
	}
	detail.Customer = normalizePortalAdminCustomer(detail.Customer)
	detail.Capabilities = completeCapabilityOptions(detail.Capabilities)
	return detail, nil
}

func (s *Service) capabilityTemplateByKey(ctx context.Context, key string) (CapabilityTemplate, bool) {
	key = NormalizeCapabilityTemplateKey(key)
	if key == "" {
		return CapabilityTemplate{}, false
	}
	rows, err := s.ListCapabilityTemplates(ctx)
	if err != nil {
		return CapabilityTemplate{}, false
	}
	for _, template := range rows {
		if template.Key == key && template.Active {
			return template, true
		}
	}
	return CapabilityTemplate{}, false
}

func (s *Service) capabilityTemplateByKeyStrict(ctx context.Context, key string) (CapabilityTemplate, bool, error) {
	key = NormalizeCapabilityTemplateKey(key)
	if key == "" {
		return CapabilityTemplate{}, false, nil
	}
	rows, err := s.ListCapabilityTemplates(ctx)
	if err != nil {
		return CapabilityTemplate{}, false, err
	}
	for _, template := range rows {
		if template.Key == key && template.Active {
			return template, true, nil
		}
	}
	return CapabilityTemplate{}, false, nil
}

func (s *Service) capabilityTemplateExists(ctx context.Context, key string) bool {
	key = NormalizeCapabilityTemplateKey(key)
	if key == "" {
		return false
	}
	rows, err := s.ListCapabilityTemplates(ctx)
	if err != nil {
		return false
	}
	for _, template := range rows {
		if template.Key == key {
			return true
		}
	}
	return false
}

func (s *Service) validateCapabilityTemplateParent(ctx context.Context, template CapabilityTemplate) error {
	key := NormalizeCapabilityTemplateKey(template.Key)
	if key == "" {
		return ErrCapabilityTemplateInvalid
	}
	parentKey := NormalizeCapabilityTemplateKey(template.ParentTemplateKey)
	if strings.TrimSpace(template.ParentTemplateKey) != "" && parentKey == "" {
		return ErrCapabilityTemplateInvalid
	}
	if parentKey == "" {
		return nil
	}
	if parentKey == key {
		return ErrCapabilityTemplateInvalid
	}
	if !s.capabilityTemplateExists(ctx, parentKey) {
		return ErrCapabilityTemplateInvalid
	}
	return nil
}

func (s *Service) ListMallProducts(ctx context.Context) ([]MallProduct, []MallProductOption, error) {
	if s.repo == nil {
		return nil, nil, fmt.Errorf("repository required")
	}
	rows, options, err := s.repo.ListMallProducts(ctx)
	if err != nil {
		return nil, nil, err
	}
	for i := range rows {
		rows[i] = normalizeMallProduct(rows[i])
	}
	if options == nil {
		options = []MallProductOption{}
	}
	return rows, options, nil
}

func (s *Service) SaveMallProduct(ctx context.Context, cmd SaveMallProductCommand) (MallProduct, error) {
	if s.repo == nil {
		return MallProduct{}, fmt.Errorf("repository required")
	}
	cmd = normalizeMallProductCommand(cmd)
	if cmd.ProductID <= 0 {
		return MallProduct{}, fmt.Errorf("product required")
	}
	if cmd.Title == "" {
		return MallProduct{}, fmt.Errorf("title required")
	}
	if cmd.SpecG <= 0 {
		return MallProduct{}, fmt.Errorf("spec required")
	}
	if cmd.UnitPrice < 0 {
		return MallProduct{}, fmt.Errorf("unit_price invalid")
	}
	row, err := s.repo.SaveMallProduct(ctx, cmd)
	if err != nil {
		return MallProduct{}, err
	}
	return normalizeMallProduct(row), nil
}

func (s *Service) UpdateMallProductImage(ctx context.Context, cmd UpdateMallProductImageCommand) (MallProduct, error) {
	if s.repo == nil {
		return MallProduct{}, fmt.Errorf("repository required")
	}
	cmd.ImageURL = strings.TrimSpace(cmd.ImageURL)
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.ID <= 0 {
		return MallProduct{}, fmt.Errorf("mall_product required")
	}
	if cmd.ImageURL == "" {
		return MallProduct{}, fmt.Errorf("image_url required")
	}
	row, err := s.repo.UpdateMallProductImage(ctx, cmd)
	if err != nil {
		return MallProduct{}, err
	}
	return normalizeMallProduct(row), nil
}

func (s *Service) GetMallPage(ctx context.Context, token string) (MallPage, error) {
	current, err := s.requireCustomerCapability(ctx, token, CapabilityMall)
	if err != nil {
		return MallPage{}, err
	}
	page, err := s.repo.LoadMallPage(ctx, current.CurrentCustomerID)
	if err != nil {
		return MallPage{}, err
	}
	page.ThemeKey = NormalizePortalThemeKey(firstNonEmpty(page.ThemeKey, current.ThemeKey))
	page.MiniappEntryMode = NormalizeMiniappEntryMode(firstNonEmpty(page.MiniappEntryMode, current.MiniappEntryMode))
	page.CurrentCustomerID = current.CurrentCustomerID
	page.CurrentCustomerName = firstNonEmpty(page.CurrentCustomerName, current.CurrentCustomerName)
	if page.Products == nil {
		page.Products = []MallProduct{}
	}
	for i := range page.Products {
		page.Products[i] = normalizeMallProduct(page.Products[i])
	}
	return page, nil
}

func (s *Service) CreateMallOrder(ctx context.Context, token string, cmd CreateMallOrderCommand) (FulfillmentOrder, error) {
	current, err := s.requireCustomerCapability(ctx, token, CapabilityMall)
	if err != nil {
		return FulfillmentOrder{}, err
	}
	cmd.CustomerID = current.CurrentCustomerID
	cmd.CreatedByMiniUserID = current.MiniUserID
	cmd.RecipientName = strings.TrimSpace(cmd.RecipientName)
	cmd.RecipientPhone = strings.TrimSpace(cmd.RecipientPhone)
	cmd.RecipientAddress = strings.TrimSpace(cmd.RecipientAddress)
	cmd.RecipientCompany = strings.TrimSpace(cmd.RecipientCompany)
	cmd.Note = strings.TrimSpace(cmd.Note)
	if cmd.RecipientName == "" {
		return FulfillmentOrder{}, fmt.Errorf("recipient_name required")
	}
	if cmd.RecipientPhone == "" {
		return FulfillmentOrder{}, fmt.Errorf("recipient_phone required")
	}
	if cmd.RecipientAddress == "" {
		return FulfillmentOrder{}, fmt.Errorf("recipient_address required")
	}
	if len(cmd.Items) == 0 {
		return FulfillmentOrder{}, fmt.Errorf("items required")
	}
	for i := range cmd.Items {
		if cmd.Items[i].MallProductID <= 0 {
			return FulfillmentOrder{}, fmt.Errorf("mall_product required")
		}
		if cmd.Items[i].Qty <= 0 {
			return FulfillmentOrder{}, fmt.Errorf("qty required")
		}
	}
	return s.repo.CreateMallOrder(ctx, cmd)
}

func (s *Service) CreateDirectShipBatch(ctx context.Context, token string, cmd CreateDirectShipBatchCommand) (DirectShipBatch, error) {
	current, err := s.requireCustomerCapability(ctx, token, CapabilityDirectShip)
	if err != nil {
		return DirectShipBatch{}, err
	}
	cmd.CustomerID = current.CurrentCustomerID
	cmd.CreatedByMiniUserID = current.MiniUserID
	cmd.SourceName = strings.TrimSpace(cmd.SourceName)
	cmd.Note = strings.TrimSpace(cmd.Note)
	if cmd.SourceName == "" {
		return DirectShipBatch{}, fmt.Errorf("source_name required")
	}
	if cmd.TotalRows <= 0 {
		return DirectShipBatch{}, fmt.Errorf("total_rows invalid")
	}
	return s.repo.CreateDirectShipBatch(ctx, cmd)
}

func (s *Service) CreateProcessingRequest(ctx context.Context, token string, cmd CreateProcessingRequestCommand) (ProcessingRequest, error) {
	current, err := s.requireCustomerCapability(ctx, token, CapabilityProcessing)
	if err != nil {
		return ProcessingRequest{}, err
	}
	cmd.CustomerID = current.CurrentCustomerID
	cmd.CreatedByMiniUserID = current.MiniUserID
	cmd.Note = strings.TrimSpace(cmd.Note)
	if cmd.InputMaterialID <= 0 {
		return ProcessingRequest{}, fmt.Errorf("input_material required")
	}
	if cmd.InputQtyG <= 0 {
		return ProcessingRequest{}, fmt.Errorf("input_qty required")
	}
	if cmd.TargetProductID <= 0 {
		return ProcessingRequest{}, fmt.Errorf("target_product required")
	}
	if cmd.TargetSpecG <= 0 {
		return ProcessingRequest{}, fmt.Errorf("target_spec required")
	}
	if cmd.TargetQty <= 0 {
		return ProcessingRequest{}, fmt.Errorf("target_qty required")
	}
	return s.repo.CreateProcessingRequest(ctx, cmd)
}

func (s *Service) CreateFulfillmentOrder(ctx context.Context, token string, cmd CreateFulfillmentOrderCommand) (FulfillmentOrder, error) {
	capability, serviceCode, err := fulfillmentOrderCapability(cmd.PortalServiceCode)
	if err != nil {
		return FulfillmentOrder{}, err
	}
	current, err := s.requireCustomerCapability(ctx, token, capability)
	if err != nil {
		return FulfillmentOrder{}, err
	}
	cmd.CustomerID = current.CurrentCustomerID
	cmd.CreatedByMiniUserID = current.MiniUserID
	cmd.PortalServiceCode = serviceCode
	cmd.RecipientName = strings.TrimSpace(cmd.RecipientName)
	cmd.RecipientPhone = strings.TrimSpace(cmd.RecipientPhone)
	cmd.RecipientAddress = strings.TrimSpace(cmd.RecipientAddress)
	cmd.RecipientCompany = strings.TrimSpace(cmd.RecipientCompany)
	cmd.ProductName = strings.TrimSpace(cmd.ProductName)
	cmd.UnitPrice = 0
	cmd.Note = strings.TrimSpace(cmd.Note)
	if cmd.RecipientName == "" {
		return FulfillmentOrder{}, fmt.Errorf("recipient_name required")
	}
	if cmd.RecipientPhone == "" {
		return FulfillmentOrder{}, fmt.Errorf("recipient_phone required")
	}
	if cmd.RecipientAddress == "" {
		return FulfillmentOrder{}, fmt.Errorf("recipient_address required")
	}
	if cmd.ProductID <= 0 {
		return FulfillmentOrder{}, fmt.Errorf("product required")
	}
	if cmd.SpecG <= 0 {
		return FulfillmentOrder{}, fmt.Errorf("spec required")
	}
	if cmd.Qty <= 0 {
		return FulfillmentOrder{}, fmt.Errorf("qty required")
	}
	return s.repo.CreateFulfillmentOrder(ctx, cmd)
}

func fulfillmentOrderCapability(raw string) (string, string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "directship", PortalServiceDirectShip:
		return CapabilityDirectShip, PortalServiceDirectShip, nil
	case "processing", "processingshipment", PortalServiceProcessingShipment:
		return CapabilityProcessing, PortalServiceProcessingShipment, nil
	case "productorder", PortalServiceProductOrder:
		return CapabilityProductOrder, PortalServiceProductOrder, nil
	default:
		return "", "", fmt.Errorf("service_code invalid")
	}
}

func (s *Service) requireCustomerCapability(ctx context.Context, token, capability string) (CurrentContext, error) {
	current, err := s.Me(ctx, token)
	if err != nil {
		return CurrentContext{}, err
	}
	if current.CurrentCustomerID <= 0 {
		return CurrentContext{}, ErrCustomerBindingNotFound
	}
	if !current.HasCapability(capability) {
		return CurrentContext{}, ErrCapabilityNotEnabled
	}
	return current, nil
}

type serviceDef struct {
	key          string
	title        string
	capability   string
	capabilities []string
}

func serviceDefinition(key string) (serviceDef, error) {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "beanlist", "bean_list":
		return singleCapabilityServiceDef(ServiceKeyBeanList, "我的豆单", CapabilityBeanList), nil
	case "orders", "order", "myorders", "my_orders":
		return serviceDef{
			key:          ServiceKeyOrders,
			title:        "我的订单",
			capability:   CapabilityProductOrder,
			capabilities: []string{CapabilityProductOrder, CapabilityDirectShip, CapabilityShippingQuery, CapabilityMall},
		}, nil
	case "productorder", "product_order":
		return singleCapabilityServiceDef(ServiceKeyProductOrder, "现货下单", CapabilityProductOrder), nil
	case "directship", "direct_ship":
		return singleCapabilityServiceDef(ServiceKeyDirectShip, "一件代发", CapabilityDirectShip), nil
	case "processing":
		return singleCapabilityServiceDef(ServiceKeyProcessing, "代加工", CapabilityProcessing), nil
	case "inventory", "inventory_custody":
		return singleCapabilityServiceDef(ServiceKeyInventory, "我的库存", CapabilityInventoryCustody), nil
	case "shipping", "shipping_query":
		return singleCapabilityServiceDef(ServiceKeyShipping, "物流查询", CapabilityShippingQuery), nil
	case "settlement":
		return singleCapabilityServiceDef(ServiceKeySettlement, "结算中心", CapabilitySettlement), nil
	default:
		return serviceDef{}, fmt.Errorf("service key invalid")
	}
}

func singleCapabilityServiceDef(key, title, capability string) serviceDef {
	return serviceDef{key: key, title: title, capability: capability, capabilities: []string{capability}}
}

func serviceKeyContainsOrders(key string) bool {
	return key == ServiceKeyOrders
}

func normalizeLoginResult(result LoginResult) LoginResult {
	result.ThemeKey = NormalizePortalThemeKey(result.ThemeKey)
	result.MiniappEntryMode = NormalizeMiniappEntryMode(result.MiniappEntryMode)
	return result
}

func normalizeCurrentContext(current CurrentContext) CurrentContext {
	current.ThemeKey = NormalizePortalThemeKey(current.ThemeKey)
	current.MiniappEntryMode = NormalizeMiniappEntryMode(current.MiniappEntryMode)
	return current
}

func normalizePortalAdminCustomer(customer PortalAdminCustomer) PortalAdminCustomer {
	if strings.TrimSpace(customer.CustomerType) == "" {
		customer.CustomerType = "retail"
	}
	customer.ThemeKey = NormalizePortalThemeKey(customer.ThemeKey)
	customer.MiniappEntryMode = NormalizeMiniappEntryMode(customer.MiniappEntryMode)
	customer.CapabilityTemplateKey = normalizePortalAdminCapabilityTemplateKey(customer.CapabilityTemplateKey)
	if customer.ERPBinding != nil {
		customer.ERPBinding.Role = firstNonEmpty(customer.ERPBinding.Role, "customer")
		customer.ERPBinding.Status = normalizePortalERPBindingStatus(customer.ERPBinding.Status)
	}
	return customer
}

func normalizePortalAdminCapabilityTemplateKey(value string) string {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return ""
	}
	if normalized := NormalizeCapabilityTemplateKey(raw); normalized != "" {
		return normalized
	}
	return raw
}

func normalizePortalERPBindingStatus(value string) string {
	switch strings.TrimSpace(value) {
	case "inactive":
		return "inactive"
	default:
		return "active"
	}
}

func NormalizePortalThemeKey(value string) string {
	switch strings.TrimSpace(value) {
	case PortalThemeCoffeeFactory:
		return PortalThemeCoffeeFactory
	case PortalThemeCleanOps:
		return PortalThemeCleanOps
	case PortalThemePremiumPartner:
		return PortalThemePremiumPartner
	default:
		return PortalThemeCoffeeFactory
	}
}

func NormalizeMiniappEntryMode(value string) string {
	switch strings.TrimSpace(value) {
	case MiniappEntryModeMall:
		return MiniappEntryModeMall
	default:
		return MiniappEntryModeServices
	}
}

func normalizeMallProductCommand(cmd SaveMallProductCommand) SaveMallProductCommand {
	cmd.Title = strings.Join(strings.Fields(strings.TrimSpace(cmd.Title)), " ")
	cmd.Subtitle = strings.Join(strings.Fields(strings.TrimSpace(cmd.Subtitle)), " ")
	cmd.Description = strings.TrimSpace(cmd.Description)
	cmd.ImageURL = strings.TrimSpace(cmd.ImageURL)
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.SpecG <= 0 {
		cmd.SpecG = 454
	}
	cmd.TemplateKey = NormalizeMallTemplateKey(cmd.TemplateKey)
	cmd.Status = NormalizeMallProductStatus(cmd.Status)
	return cmd
}

func normalizeMallProduct(row MallProduct) MallProduct {
	row.Title = strings.Join(strings.Fields(strings.TrimSpace(row.Title)), " ")
	row.Subtitle = strings.Join(strings.Fields(strings.TrimSpace(row.Subtitle)), " ")
	row.Description = strings.TrimSpace(row.Description)
	row.ImageURL = strings.TrimSpace(row.ImageURL)
	if row.SpecG <= 0 {
		row.SpecG = 454
	}
	row.TemplateKey = NormalizeMallTemplateKey(row.TemplateKey)
	row.Status = NormalizeMallProductStatus(row.Status)
	return row
}

func NormalizeMallProductStatus(value string) string {
	switch strings.TrimSpace(value) {
	case MallProductStatusPublished:
		return MallProductStatusPublished
	default:
		return MallProductStatusDraft
	}
}

func NormalizeMallTemplateKey(value string) string {
	switch strings.TrimSpace(value) {
	case MallTemplateHero:
		return MallTemplateHero
	case MallTemplateCompact:
		return MallTemplateCompact
	case MallTemplateWide:
		return MallTemplateWide
	default:
		return MallTemplateHero
	}
}

func normalizeServicePageFilter(filter ServicePageFilter) ServicePageFilter {
	out := ServicePageFilter{
		Query:         strings.Join(strings.Fields(strings.TrimSpace(filter.Query)), " "),
		DateFrom:      normalizeDateString(filter.DateFrom),
		DateTo:        normalizeDateString(filter.DateTo),
		ProcessStatus: normalizeStatusFilter(filter.ProcessStatus),
		PayStatus:     normalizeStatusFilter(filter.PayStatus),
		ShipStatus:    normalizeStatusFilter(filter.ShipStatus),
	}
	if out.DateFrom != "" && out.DateTo != "" {
		from, _ := time.Parse("2006-01-02", out.DateFrom)
		to, _ := time.Parse("2006-01-02", out.DateTo)
		if from.After(to) {
			out.DateFrom, out.DateTo = out.DateTo, out.DateFrom
		}
	}
	return out
}

func normalizeStatusFilter(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func normalizeDateString(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if _, err := time.Parse("2006-01-02", value); err != nil {
		return ""
	}
	return value
}

func DefaultCapabilityOptions() []CapabilityOption {
	return []CapabilityOption{
		{Code: CapabilityBeanList, Label: "我的豆单", Description: "查看客户专属豆单；没有专属豆单时默认查看系统最新已发布豆单"},
		{Code: CapabilityMall, Label: "商城下单", Description: "面向 C 端客户展示商城、浏览上架商品并提交订单"},
		{Code: CapabilityProductOrder, Label: "现货下单", Description: "查看现货商品和自己的历史订单"},
		{Code: CapabilityDirectShip, Label: "一件代发", Description: "查看代发批次、订单生产和发货状态"},
		{Code: CapabilityProcessing, Label: "代加工", Description: "查看托管库存并提交加工申请"},
		{Code: CapabilityInventoryCustody, Label: "我的库存", Description: "查看客户托管的生豆、成品和包材库存"},
		{Code: CapabilitySettlement, Label: "结算中心", Description: "查看费用明细和结算单"},
	}
}

func DefaultCapabilityTemplates() []CapabilityTemplate {
	return []CapabilityTemplate{
		{
			Key:              CapabilityTemplateProcessingFulfillment,
			Active:           true,
			SortOrder:        10,
			Label:            "客户代加工履约",
			Description:      "客户登录后查看原料库存、生产进度、成品库存、代发订单、费用和结算，并提交代加工和代发需求",
			ThemeKey:         PortalThemeCoffeeFactory,
			MiniappEntryMode: MiniappEntryModeServices,
			ERPRoleCodes:     []string{},
			ERPPermissions:   []string{"customer_processing.read", "customer_processing.submit"},
			ERPViewKeys:      []string{"customerProcessingPortal"},
			Capabilities: capabilityTemplateOptions(map[string]map[string]any{
				CapabilityDirectShip:       {"customer_sender": true, "external_recipients": true},
				CapabilityProcessing:       {},
				CapabilityInventoryCustody: {},
				CapabilitySettlement:       {},
			}),
		},
		{
			Key:              CapabilityTemplatePublicSKUDirectShip,
			Active:           true,
			SortOrder:        20,
			Label:            "公共 SKU 小批量代发",
			Description:      "客户使用公共 SKU，可维护客户专属 SKU 名称，默认寄件人为客户自己，收件人为客户的终端客户",
			ThemeKey:         PortalThemeCleanOps,
			MiniappEntryMode: MiniappEntryModeServices,
			ERPRoleCodes:     []string{},
			ERPPermissions:   []string{"customer_processing.read", "customer_processing.submit"},
			ERPViewKeys:      []string{"customerProcessingPortal"},
			Capabilities: capabilityTemplateOptions(map[string]map[string]any{
				CapabilityProductOrder: {
					"public_sku_aliases": true,
				},
				CapabilityDirectShip: {
					"public_sku_aliases":  true,
					"customer_sender":     true,
					"external_recipients": true,
					"small_batch_price_rule": map[string]any{
						"enabled":      true,
						"threshold_lb": 14,
						"tier_min_lb":  15,
						"tier_max_lb":  28,
					},
				},
				CapabilitySettlement: {},
			}),
		},
		{
			Key:              CapabilityTemplateRetailMall,
			Active:           true,
			SortOrder:        30,
			Label:            "零售商城客户",
			Description:      "零售和电商客户默认进入商城，可在商城下单并查看商城订单记录",
			ThemeKey:         PortalThemeCleanOps,
			MiniappEntryMode: MiniappEntryModeMall,
			ERPRoleCodes:     []string{},
			ERPPermissions:   []string{},
			ERPViewKeys:      []string{},
			Capabilities: capabilityTemplateOptions(map[string]map[string]any{
				CapabilityMall: {},
			}),
		},
	}
}

func CustomerCapabilityTemplateByKey(key string) (CapabilityTemplate, bool) {
	key = NormalizeCapabilityTemplateKey(key)
	if key == "" {
		return CapabilityTemplate{}, false
	}
	for _, template := range DefaultCapabilityTemplates() {
		if template.Key == key {
			return template, true
		}
	}
	return CapabilityTemplate{}, false
}

func mergeCapabilityTemplates(rows []CapabilityTemplate) []CapabilityTemplate {
	out := DefaultCapabilityTemplates()
	positions := make(map[string]int, len(out))
	for i := range out {
		positions[out[i].Key] = i
	}
	for _, row := range rows {
		template, ok := normalizeCapabilityTemplate(row)
		if !ok {
			continue
		}
		if idx, ok := positions[template.Key]; ok {
			out[idx] = template
			continue
		}
		positions[template.Key] = len(out)
		out = append(out, template)
	}
	sortCapabilityTemplatesForTree(out)
	return out
}

func sortCapabilityTemplatesForTree(rows []CapabilityTemplate) {
	indexByKey := make(map[string]int, len(rows))
	for i, row := range rows {
		indexByKey[row.Key] = i
	}
	sort.SliceStable(rows, func(i, j int) bool {
		left := rows[i]
		right := rows[j]
		leftRoot := templateTreeRoot(left, rows, indexByKey)
		rightRoot := templateTreeRoot(right, rows, indexByKey)
		if leftRoot != rightRoot {
			return templateSortValue(leftRoot, rows, indexByKey) < templateSortValue(rightRoot, rows, indexByKey)
		}
		if left.ParentTemplateKey != right.ParentTemplateKey {
			if left.Key == right.ParentTemplateKey {
				return true
			}
			if right.Key == left.ParentTemplateKey {
				return false
			}
			return left.ParentTemplateKey < right.ParentTemplateKey
		}
		if left.SortOrder != right.SortOrder {
			return left.SortOrder < right.SortOrder
		}
		return left.Key < right.Key
	})
}

func templateTreeRoot(row CapabilityTemplate, rows []CapabilityTemplate, indexByKey map[string]int) string {
	seen := map[string]bool{}
	current := row
	for current.ParentTemplateKey != "" && !seen[current.Key] {
		seen[current.Key] = true
		idx, ok := indexByKey[current.ParentTemplateKey]
		if !ok {
			break
		}
		current = rows[idx]
	}
	return current.Key
}

func templateSortValue(key string, rows []CapabilityTemplate, indexByKey map[string]int) int {
	if idx, ok := indexByKey[key]; ok {
		return rows[idx].SortOrder
	}
	return 0
}

func normalizeCapabilityTemplate(input CapabilityTemplate) (CapabilityTemplate, bool) {
	key := NormalizeCapabilityTemplateKey(input.Key)
	if key == "" {
		return CapabilityTemplate{}, false
	}
	parentKey := NormalizeCapabilityTemplateKey(input.ParentTemplateKey)
	if parentKey == key {
		return CapabilityTemplate{}, false
	}
	defaultTemplate, ok := CustomerCapabilityTemplateByKey(key)
	if !ok && parentKey != "" {
		defaultTemplate, ok = CustomerCapabilityTemplateByKey(parentKey)
	}
	if !ok {
		defaultTemplate = CapabilityTemplate{
			Key:              key,
			Label:            key,
			ThemeKey:         PortalThemeCoffeeFactory,
			MiniappEntryMode: MiniappEntryModeServices,
			ERPRoleCodes:     []string{},
			ERPPermissions:   []string{},
			ERPViewKeys:      []string{},
			Capabilities:     capabilityTemplateOptions(map[string]map[string]any{}),
			Active:           true,
		}
	}
	input.Key = key
	input.ParentTemplateKey = parentKey
	input.Label = firstNonEmpty(input.Label, defaultTemplate.Label)
	input.Description = firstNonEmpty(input.Description, defaultTemplate.Description)
	input.ThemeKey = NormalizePortalThemeKey(firstNonEmpty(input.ThemeKey, defaultTemplate.ThemeKey))
	input.MiniappEntryMode = NormalizeMiniappEntryMode(firstNonEmpty(input.MiniappEntryMode, defaultTemplate.MiniappEntryMode))
	input.ERPRoleCodes = []string{}
	if !templateERPWorkbenchDisallowed(input) {
		input.ERPPermissions = normalizedStringListOrDefault(input.ERPPermissions, defaultTemplate.ERPPermissions)
		input.ERPViewKeys = normalizedStringListOrDefault(input.ERPViewKeys, defaultTemplate.ERPViewKeys)
	} else {
		input.ERPPermissions = []string{}
		input.ERPViewKeys = []string{}
	}
	if len(input.Capabilities) == 0 {
		input.Capabilities = cloneCapabilityOptions(defaultTemplate.Capabilities)
	} else {
		input.Capabilities = normalizeTemplateCapabilityOptions(input.Capabilities)
	}
	input.UpdatedAt = strings.TrimSpace(input.UpdatedAt)
	input.UpdatedBy = strings.TrimSpace(input.UpdatedBy)
	return input, true
}

func templateERPWorkbenchDisallowed(template CapabilityTemplate) bool {
	key := NormalizeCapabilityTemplateKey(template.Key)
	if key == "" {
		return false
	}
	defaultTemplate, ok := CustomerCapabilityTemplateByKey(key)
	if ok {
		return !defaultTemplate.ExposesERPWorkbench()
	}
	parentKey := NormalizeCapabilityTemplateKey(template.ParentTemplateKey)
	if parentKey == "" {
		return false
	}
	parentTemplate, ok := CustomerCapabilityTemplateByKey(parentKey)
	return ok && !parentTemplate.ExposesERPWorkbench()
}

func normalizedStringListOrDefault(values, fallback []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	if len(out) > 0 {
		return out
	}
	return cloneStringList(fallback)
}

func cloneStringList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func cloneCapabilityOptions(values []CapabilityOption) []CapabilityOption {
	out := make([]CapabilityOption, 0, len(values))
	for _, value := range values {
		value.Config = copyCapabilityConfig(value.Config)
		out = append(out, value)
	}
	return out
}

func NormalizeCapabilityTemplateKey(value string) string {
	key := strings.TrimSpace(value)
	switch key {
	case CapabilityTemplateProcessingFulfillment:
		return CapabilityTemplateProcessingFulfillment
	case CapabilityTemplatePublicSKUDirectShip:
		return CapabilityTemplatePublicSKUDirectShip
	case CapabilityTemplateRetailMall:
		return CapabilityTemplateRetailMall
	}
	if len(key) < 2 || len(key) > 64 {
		return ""
	}
	for _, ch := range key {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' {
			continue
		}
		return ""
	}
	return key
}

func normalizeTemplateCapabilityOptions(input []CapabilityOption) []CapabilityOption {
	out := completeCapabilityOptions(normalizeCapabilityOptions(input))
	for i := range out {
		out[i].Config = normalizeTemplateCapabilityConfig(out[i].Code, out[i].Config)
	}
	return out
}

func normalizeTemplateCapabilityConfig(code string, config map[string]any) map[string]any {
	out := copyCapabilityConfig(config)
	switch code {
	case CapabilityProductOrder:
		out["public_sku_aliases"] = boolFromAny(out["public_sku_aliases"])
	case CapabilityDirectShip:
		out["public_sku_aliases"] = boolFromAny(out["public_sku_aliases"])
		out["customer_sender"] = boolFromAny(out["customer_sender"])
		out["external_recipients"] = boolFromAny(out["external_recipients"])
		out["small_batch_price_rule"] = smallBatchPriceRuleConfig(out["small_batch_price_rule"])
	}
	return out
}

func smallBatchPriceRuleConfig(value any) map[string]any {
	rule := normalizeSmallBatchPriceRule(mapFromAny(value))
	return map[string]any{
		"enabled":      rule.Enabled,
		"threshold_lb": rule.ThresholdLB,
		"tier_min_lb":  rule.TierMinLB,
		"tier_max_lb":  rule.TierMaxLB,
	}
}

func mapFromAny(value any) map[string]any {
	switch got := value.(type) {
	case map[string]any:
		return got
	default:
		return map[string]any{}
	}
}

func capabilityTemplateOptions(enabled map[string]map[string]any) []CapabilityOption {
	out := DefaultCapabilityOptions()
	for i := range out {
		if config, ok := enabled[out[i].Code]; ok {
			out[i].Enabled = true
			out[i].Config = copyCapabilityConfig(config)
		} else {
			out[i].Enabled = false
			out[i].Config = map[string]any{}
		}
	}
	return out
}

func copyCapabilityConfig(config map[string]any) map[string]any {
	if config == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(config))
	for k, v := range config {
		out[k] = v
	}
	return out
}

func (t CapabilityTemplate) HasCapability(code string) bool {
	code = strings.TrimSpace(code)
	for _, capability := range t.Capabilities {
		if capability.Code == code && capability.Enabled {
			return true
		}
	}
	return false
}

func (t CapabilityTemplate) ExposesERPWorkbench() bool {
	return hasNonEmptyString(t.ERPPermissions) || hasNonEmptyString(t.ERPViewKeys)
}

func hasNonEmptyString(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func (t CapabilityTemplate) SmallBatchPriceRule() SmallBatchPriceRule {
	for _, capability := range t.Capabilities {
		if capability.Code != CapabilityDirectShip || !capability.Enabled {
			continue
		}
		raw, _ := capability.Config["small_batch_price_rule"].(map[string]any)
		return normalizeSmallBatchPriceRule(raw)
	}
	return SmallBatchPriceRule{}
}

func normalizeSmallBatchPriceRule(raw map[string]any) SmallBatchPriceRule {
	rule := SmallBatchPriceRule{
		Enabled:     boolFromAny(raw["enabled"]),
		ThresholdLB: floatFromAny(raw["threshold_lb"]),
		TierMinLB:   floatFromAny(raw["tier_min_lb"]),
		TierMaxLB:   floatFromAny(raw["tier_max_lb"]),
	}
	if rule.ThresholdLB <= 0 {
		rule.ThresholdLB = 14
	}
	if rule.TierMinLB <= 0 {
		rule.TierMinLB = 15
	}
	if rule.TierMaxLB <= 0 {
		rule.TierMaxLB = 28
	}
	return rule
}

func boolFromAny(value any) bool {
	got, _ := value.(bool)
	return got
}

func floatFromAny(value any) float64 {
	switch got := value.(type) {
	case int:
		return float64(got)
	case int64:
		return float64(got)
	case float64:
		return got
	case float32:
		return float64(got)
	default:
		return 0
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func completeCapabilityOptions(existing []CapabilityOption) []CapabilityOption {
	byCode := map[string]CapabilityOption{}
	for _, item := range existing {
		code := strings.TrimSpace(item.Code)
		if code == "" {
			continue
		}
		item.Code = code
		if item.Config == nil {
			item.Config = map[string]any{}
		}
		byCode[code] = item
	}
	out := DefaultCapabilityOptions()
	for i, item := range out {
		if got, ok := byCode[item.Code]; ok {
			out[i].Enabled = got.Enabled
			out[i].Config = got.Config
			if strings.TrimSpace(got.Label) != "" {
				out[i].Label = strings.TrimSpace(got.Label)
			}
			if strings.TrimSpace(got.Description) != "" {
				out[i].Description = strings.TrimSpace(got.Description)
			}
		}
	}
	return out
}

func normalizeCapabilityOptions(input []CapabilityOption) []CapabilityOption {
	known := map[string]bool{}
	for _, item := range DefaultCapabilityOptions() {
		known[item.Code] = true
	}
	byCode := map[string]CapabilityOption{}
	for _, item := range input {
		code := strings.TrimSpace(item.Code)
		if !known[code] {
			continue
		}
		item.Code = code
		if item.Config == nil {
			item.Config = map[string]any{}
		}
		byCode[code] = item
	}
	out := make([]CapabilityOption, 0, len(byCode))
	for _, item := range DefaultCapabilityOptions() {
		if got, ok := byCode[item.Code]; ok {
			got.Label = item.Label
			got.Description = item.Description
			out = append(out, got)
		}
	}
	return out
}

func serviceSummary(page ServicePage) []ServiceMetric {
	switch page.Key {
	case ServiceKeyBeanList:
		return []ServiceMetric{{Label: "已发布豆单", Value: fmt.Sprintf("%d", len(page.BeanLists))}}
	case ServiceKeyOrders:
		return []ServiceMetric{{Label: "近期订单", Value: fmt.Sprintf("%d", len(page.Orders))}}
	case ServiceKeyProductOrder:
		return []ServiceMetric{{Label: "可见商品", Value: fmt.Sprintf("%d", len(page.Products))}, {Label: "近期订单", Value: fmt.Sprintf("%d", len(page.Orders))}}
	case ServiceKeyDirectShip:
		return []ServiceMetric{{Label: "导入批次", Value: fmt.Sprintf("%d", len(page.DirectShipBatches))}, {Label: "近期订单", Value: fmt.Sprintf("%d", len(page.Orders))}}
	case ServiceKeyProcessing:
		return []ServiceMetric{{Label: "加工申请", Value: fmt.Sprintf("%d", len(page.ProcessingRequests))}, {Label: "托管库存", Value: fmt.Sprintf("%d", len(page.Inventory))}}
	case ServiceKeyInventory:
		return []ServiceMetric{{Label: "库存项目", Value: fmt.Sprintf("%d", len(page.Inventory))}}
	case ServiceKeyShipping:
		return []ServiceMetric{{Label: "订单 / 物流", Value: fmt.Sprintf("%d", len(page.Orders))}}
	case ServiceKeySettlement:
		return []ServiceMetric{{Label: "费用明细", Value: fmt.Sprintf("%d", len(page.FeeItems))}, {Label: "结算单", Value: fmt.Sprintf("%d", len(page.SettlementBatches))}}
	default:
		return []ServiceMetric{}
	}
}
