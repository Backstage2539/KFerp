package customerportal

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	ServiceKeyProductOrder = "productOrder"
	ServiceKeyDirectShip   = "directShip"
	ServiceKeyProcessing   = "processing"
	ServiceKeyInventory    = "inventory"
	ServiceKeyShipping     = "shipping"
	ServiceKeySettlement   = "settlement"
)

var (
	ErrCustomerBindingNotFound = errors.New("customer binding not found")
	ErrMiniSessionNotFound     = errors.New("mini session not found")
	ErrMiniLoginDisabled       = errors.New("mini login disabled")
	ErrMiniUserDisabled        = errors.New("mini user disabled")
	ErrCapabilityNotEnabled    = errors.New("capability not enabled")
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
	Bindings            []CustomerBinding `json:"bindings"`
	Capabilities        []Capability      `json:"capabilities"`
}

type ServiceMetric struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type BeanListSummary struct {
	ID          int64  `json:"id"`
	ListType    string `json:"list_type"`
	VersionNo   string `json:"version_no"`
	Status      string `json:"status"`
	PublishedAt string `json:"published_at"`
	Changelog   string `json:"changelog"`
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
	ID             int64  `json:"id"`
	OrderNo        string `json:"order_no"`
	OrderDate      string `json:"order_date"`
	ProcessStatus  string `json:"process_status"`
	PayStatus      string `json:"pay_status"`
	ShipStatus     string `json:"ship_status"`
	ShipTrackingNo string `json:"ship_tracking_no"`
	GrandTotal     string `json:"grand_total"`
	ShippingAmount string `json:"shipping_amount"`
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
	CustomerID int64
	Key        string
	Limit      int
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

func (c CurrentContext) HasCapability(code string) bool {
	code = strings.TrimSpace(code)
	for _, capability := range c.Capabilities {
		if capability.Enabled && capability.Code == code {
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
	return s.repo.CreateLoginSession(ctx, CreateLoginSessionCommand{
		OpenID:   identity.OpenID,
		UnionID:  strings.TrimSpace(identity.UnionID),
		Phone:    strings.TrimSpace(cmd.Phone),
		Nickname: strings.TrimSpace(cmd.Nickname),
	})
}

func (s *Service) Me(ctx context.Context, token string) (CurrentContext, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return CurrentContext{}, fmt.Errorf("mini token required")
	}
	if s.repo == nil {
		return CurrentContext{}, fmt.Errorf("repository required")
	}
	return s.repo.CurrentContextByToken(ctx, token)
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
	return s.repo.SwitchCurrentCustomer(ctx, token, customerID)
}

func (s *Service) GetServicePage(ctx context.Context, token, key string) (ServicePage, error) {
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
	if !current.HasCapability(def.capability) {
		return ServicePage{}, ErrCapabilityNotEnabled
	}
	page, err := s.repo.LoadServicePage(ctx, ServicePageQuery{CustomerID: current.CurrentCustomerID, Key: def.key, Limit: 20})
	if err != nil {
		return ServicePage{}, err
	}
	page.Key = def.key
	page.Title = def.title
	page.Capability = def.capability
	page.CurrentCustomerID = current.CurrentCustomerID
	page.CurrentCustomerName = current.CurrentCustomerName
	page.Summary = serviceSummary(page)
	return page, nil
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
	key        string
	title      string
	capability string
}

func serviceDefinition(key string) (serviceDef, error) {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "beanlist", "bean_list":
		return serviceDef{key: ServiceKeyBeanList, title: "我的豆单", capability: CapabilityBeanList}, nil
	case "productorder", "product_order":
		return serviceDef{key: ServiceKeyProductOrder, title: "现货下单", capability: CapabilityProductOrder}, nil
	case "directship", "direct_ship":
		return serviceDef{key: ServiceKeyDirectShip, title: "一件代发", capability: CapabilityDirectShip}, nil
	case "processing":
		return serviceDef{key: ServiceKeyProcessing, title: "代加工", capability: CapabilityProcessing}, nil
	case "inventory", "inventory_custody":
		return serviceDef{key: ServiceKeyInventory, title: "我的库存", capability: CapabilityInventoryCustody}, nil
	case "shipping", "shipping_query":
		return serviceDef{key: ServiceKeyShipping, title: "物流查询", capability: CapabilityShippingQuery}, nil
	case "settlement":
		return serviceDef{key: ServiceKeySettlement, title: "结算中心", capability: CapabilitySettlement}, nil
	default:
		return serviceDef{}, fmt.Errorf("service key invalid")
	}
}

func serviceSummary(page ServicePage) []ServiceMetric {
	switch page.Key {
	case ServiceKeyBeanList:
		return []ServiceMetric{{Label: "已发布豆单", Value: fmt.Sprintf("%d", len(page.BeanLists))}}
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
