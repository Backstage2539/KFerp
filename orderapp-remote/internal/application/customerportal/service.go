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
	CapabilityBeanList         = "bean_list"
	CapabilityProductOrder     = "product_order"
	CapabilityDirectShip       = "direct_ship"
	CapabilityProcessing       = "processing"
	CapabilityInventoryCustody = "inventory_custody"
	CapabilityShippingQuery    = "shipping_query"
	CapabilitySettlement       = "settlement"
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
	PortalThemeCoffeeFactory  = "coffee_factory"
	PortalThemeCleanOps       = "clean_ops"
	PortalThemePremiumPartner = "premium_partner"
)

var (
	ErrCustomerBindingNotFound     = errors.New("customer binding not found")
	ErrMiniSessionNotFound         = errors.New("mini session not found")
	ErrMiniLoginDisabled           = errors.New("mini login disabled")
	ErrMiniUserDisabled            = errors.New("mini user disabled")
	ErrCapabilityNotEnabled        = errors.New("capability not enabled")
	ErrPortalCustomerNotFound      = errors.New("portal customer not found")
	ErrBeanListPublicationNotFound = errors.New("bean list publication not found")
)

type LoginCommand struct {
	Code     string
	Phone    string
	Nickname string
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

type LoginResult struct {
	Token             string            `json:"token"`
	MiniUserID        int64             `json:"mini_user_id"`
	CurrentCustomerID int64             `json:"current_customer_id"`
	ThemeKey          string            `json:"theme_key"`
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

type CapabilityOption struct {
	Code        string         `json:"code"`
	Label       string         `json:"label"`
	Description string         `json:"description,omitempty"`
	Enabled     bool           `json:"enabled"`
	Config      map[string]any `json:"config,omitempty"`
}

type PortalAdminCustomerQuery struct {
	Query string
	Limit int
}

type PortalAdminCustomer struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Phone         string `json:"phone"`
	CompanyName   string `json:"company_name"`
	DisplayName   string `json:"display_name"`
	PortalEnabled bool   `json:"portal_enabled"`
	PortalStatus  string `json:"portal_status"`
	ThemeKey      string `json:"theme_key"`
	BindingCount  int    `json:"binding_count"`
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
	CustomerID   int64
	DisplayName  string
	Enabled      bool
	ThemeKey     string
	Capabilities []CapabilityOption
	UpdatedBy    string
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
	CurrentContextByToken(ctx context.Context, token string) (CurrentContext, error)
	SwitchCurrentCustomer(ctx context.Context, token string, customerID int64) (CurrentContext, error)
	LoadServicePage(ctx context.Context, query ServicePageQuery) (ServicePage, error)
	LoadBeanListPublication(ctx context.Context, customerID, publicationID int64) (BeanListSummary, error)
	ListPortalAdminCustomers(ctx context.Context, query PortalAdminCustomerQuery) ([]PortalAdminCustomer, error)
	PortalAdminDetail(ctx context.Context, customerID int64) (PortalAdminDetail, error)
	UpdatePortalVisibility(ctx context.Context, cmd UpdatePortalVisibilityCommand) (PortalAdminDetail, error)
	CreateDirectShipBatch(ctx context.Context, cmd CreateDirectShipBatchCommand) (DirectShipBatch, error)
	CreateProcessingRequest(ctx context.Context, cmd CreateProcessingRequestCommand) (ProcessingRequest, error)
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
	cmd.UpdatedBy = strings.TrimSpace(cmd.UpdatedBy)
	cmd.ThemeKey = NormalizePortalThemeKey(cmd.ThemeKey)
	cmd.Capabilities = normalizeCapabilityOptions(cmd.Capabilities)
	detail, err := s.repo.UpdatePortalVisibility(ctx, cmd)
	if err != nil {
		return PortalAdminDetail{}, err
	}
	detail.Customer = normalizePortalAdminCustomer(detail.Customer)
	detail.Capabilities = completeCapabilityOptions(detail.Capabilities)
	return detail, nil
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
	if cmd.TotalRows < 0 {
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
			capabilities: []string{CapabilityProductOrder, CapabilityDirectShip, CapabilityShippingQuery},
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
	return result
}

func normalizeCurrentContext(current CurrentContext) CurrentContext {
	current.ThemeKey = NormalizePortalThemeKey(current.ThemeKey)
	return current
}

func normalizePortalAdminCustomer(customer PortalAdminCustomer) PortalAdminCustomer {
	customer.ThemeKey = NormalizePortalThemeKey(customer.ThemeKey)
	return customer
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
		{Code: CapabilityProductOrder, Label: "现货下单", Description: "查看现货商品和自己的历史订单"},
		{Code: CapabilityDirectShip, Label: "一件代发", Description: "查看代发批次、订单生产和发货状态"},
		{Code: CapabilityProcessing, Label: "代加工", Description: "查看托管库存并提交加工申请"},
		{Code: CapabilityInventoryCustody, Label: "我的库存", Description: "查看客户托管的生豆、成品和包材库存"},
		{Code: CapabilityShippingQuery, Label: "物流查询", Description: "查看订单发货和物流单号"},
		{Code: CapabilitySettlement, Label: "结算中心", Description: "查看费用明细和结算单"},
	}
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
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Code < out[j].Code
	})
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
