package customerfulfillment

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrMiniDirectShipUnavailable       = errors.New("mini direct ship unavailable")
	ErrMiniDirectShipStockInsufficient = errors.New("customer finished stock insufficient")
	ErrMiniDirectShipRequestNotFound   = errors.New("direct ship request not found")
	ErrMiniDirectShipIdempotency       = errors.New("idempotency key already used with different request")
	ErrMiniDirectShipCannotCancel      = errors.New("shipped request cannot be cancelled")
)

type MiniDirectShipItemCommand struct {
	ProductID   int64  `json:"product_id"`
	ProductName string `json:"product_name,omitempty"`
	SKUCode     string `json:"sku_code,omitempty"`
	SpecLabel   string `json:"spec_label,omitempty"`
	SpecG       int64  `json:"spec_g"`
	Qty         int64  `json:"qty"`
}

type MiniDirectShipCommand struct {
	CustomerID       int64                       `json:"-"`
	EmployeeID       int64                       `json:"-"`
	MiniUserID       int64                       `json:"-"`
	IdempotencyKey   string                      `json:"idempotency_key"`
	RecipientName    string                      `json:"recipient_name"`
	RecipientPhone   string                      `json:"recipient_phone"`
	Province         string                      `json:"province,omitempty"`
	City             string                      `json:"city,omitempty"`
	District         string                      `json:"district,omitempty"`
	DetailAddress    string                      `json:"detail_address"`
	RecipientCompany string                      `json:"recipient_company,omitempty"`
	Items            []MiniDirectShipItemCommand `json:"items"`
	Note             string                      `json:"note,omitempty"`
	Actor            string                      `json:"-"`
}

type MiniDirectShipCatalogQuery struct {
	CustomerID int64
	Q          string
	Category   string
}

type MiniDirectShipCategory struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type MiniDirectShipCatalog struct {
	CurrentCustomerID int64                    `json:"current_customer_id"`
	Categories        []MiniDirectShipCategory `json:"categories,omitempty"`
	ProductFamilies   []map[string]any         `json:"product_families"`
}

type MiniDirectShipPreviewWarehouse struct {
	Warehouse string                      `json:"warehouse"`
	Items     []MiniDirectShipItemCommand `json:"items"`
}

type MiniDirectShipShortage struct {
	ProductID    int64 `json:"product_id"`
	SpecG        int64 `json:"spec_g"`
	Qty          int64 `json:"qty"`
	AvailableQty int64 `json:"available_qty"`
}

type MiniDirectShipPreview struct {
	CanSubmit  bool                             `json:"can_submit"`
	Warehouses []MiniDirectShipPreviewWarehouse `json:"warehouses"`
	Shortages  []MiniDirectShipShortage         `json:"shortages,omitempty"`
}

type MiniDirectShipPackage struct {
	ID          int64                         `json:"id"`
	OrderID     int64                         `json:"order_id"`
	OrderNo     string                        `json:"order_no"`
	Warehouse   string                        `json:"warehouse"`
	Status      string                        `json:"status"`
	CarrierName string                        `json:"carrier_name,omitempty"`
	TrackingNo  string                        `json:"tracking_no,omitempty"`
	ShippedAt   string                        `json:"shipped_at,omitempty"`
	DeliveredAt string                        `json:"delivered_at,omitempty"`
	Items       []MiniDirectShipItemCommand   `json:"items,omitempty"`
	Events      []MiniDirectShipTrackingEvent `json:"events,omitempty"`
}

type MiniDirectShipTrackingEvent struct {
	Time        string `json:"time"`
	Status      string `json:"status"`
	Description string `json:"description"`
	Location    string `json:"location,omitempty"`
}

type MiniDirectShipRequest struct {
	ID               int64                       `json:"id"`
	RequestNo        string                      `json:"request_no"`
	Status           string                      `json:"status"`
	RecipientName    string                      `json:"recipient_name"`
	RecipientPhone   string                      `json:"recipient_phone"`
	Province         string                      `json:"province,omitempty"`
	City             string                      `json:"city,omitempty"`
	District         string                      `json:"district,omitempty"`
	DetailAddress    string                      `json:"detail_address"`
	RecipientCompany string                      `json:"recipient_company,omitempty"`
	Items            []MiniDirectShipItemCommand `json:"items"`
	Packages         []MiniDirectShipPackage     `json:"packages,omitempty"`
	CreatedAt        string                      `json:"created_at"`
	Note             string                      `json:"note,omitempty"`
}

type CustomerInventorySummary struct {
	ProductID       int64    `json:"product_id"`
	ProductName     string   `json:"product_name"`
	ParentProductID int64    `json:"parent_product_id,omitempty"`
	SKUCode         string   `json:"sku_code,omitempty"`
	SpecG           int64    `json:"spec_g"`
	AvailableQty    int64    `json:"available_qty"`
	ReservedQty     int64    `json:"reserved_qty"`
	TotalQty        int64    `json:"total_qty"`
	Warehouses      []string `json:"warehouses"`
}

type CustomerInventoryBatch struct {
	BatchID                         int64  `json:"batch_id"`
	BatchNo                         string `json:"batch_no"`
	ProductID                       int64  `json:"product_id"`
	ProductName                     string `json:"product_name"`
	SKUCode                         string `json:"sku_code,omitempty"`
	SpecG                           int64  `json:"spec_g"`
	Warehouse                       string `json:"warehouse"`
	ProductionDate                  string `json:"production_date,omitempty"`
	InboundAt                       string `json:"inbound_at,omitempty"`
	AvailableQty                    int64  `json:"available_qty"`
	ReservedQty                     int64  `json:"reserved_qty"`
	QualityStatus                   string `json:"quality_status"`
	HistoricalWithoutProductionDate bool   `json:"historical_without_production_date,omitempty"`
}

// MiniDirectShipRepository is kept separate from the legacy fulfillment
// repository contract so the closed-loop endpoints can be introduced without
// widening every test double used by the import and settlement workflows.
type MiniDirectShipRepository interface {
	MiniDirectShipCatalog(context.Context, MiniDirectShipCatalogQuery) (MiniDirectShipCatalog, error)
	PreviewMiniDirectShip(context.Context, MiniDirectShipCommand) (MiniDirectShipPreview, error)
	SubmitMiniDirectShip(context.Context, MiniDirectShipCommand) (MiniDirectShipRequest, error)
	ListMiniDirectShipRequests(context.Context, int64, int) ([]MiniDirectShipRequest, error)
	GetMiniDirectShipRequest(context.Context, int64, int64) (MiniDirectShipRequest, error)
	CancelMiniDirectShipRequest(context.Context, int64, int64, string) (MiniDirectShipRequest, error)
	ListCustomerCentralInventory(context.Context, int64) ([]CustomerInventorySummary, error)
	ListCustomerCentralInventoryBatches(context.Context, int64, int64, int64) ([]CustomerInventoryBatch, error)
}

func (s *Service) miniDirectShipRepository() (MiniDirectShipRepository, error) {
	repo, ok := s.repo.(MiniDirectShipRepository)
	if !ok || repo == nil {
		return nil, ErrMiniDirectShipUnavailable
	}
	return repo, nil
}

func (s *Service) MiniDirectShipCatalog(ctx context.Context, query MiniDirectShipCatalogQuery) (MiniDirectShipCatalog, error) {
	if query.CustomerID <= 0 {
		return MiniDirectShipCatalog{}, fmt.Errorf("customer required")
	}
	query.Q = strings.TrimSpace(query.Q)
	query.Category = strings.TrimSpace(query.Category)
	repo, err := s.miniDirectShipRepository()
	if err != nil {
		return MiniDirectShipCatalog{}, err
	}
	return repo.MiniDirectShipCatalog(ctx, query)
}

func (s *Service) PreviewMiniDirectShip(ctx context.Context, cmd MiniDirectShipCommand) (MiniDirectShipPreview, error) {
	cmd, err := normalizeMiniDirectShipCommand(cmd, false)
	if err != nil {
		return MiniDirectShipPreview{}, err
	}
	repo, err := s.miniDirectShipRepository()
	if err != nil {
		return MiniDirectShipPreview{}, err
	}
	return repo.PreviewMiniDirectShip(ctx, cmd)
}

func (s *Service) SubmitMiniDirectShip(ctx context.Context, cmd MiniDirectShipCommand) (MiniDirectShipRequest, error) {
	cmd, err := normalizeMiniDirectShipCommand(cmd, true)
	if err != nil {
		return MiniDirectShipRequest{}, err
	}
	repo, err := s.miniDirectShipRepository()
	if err != nil {
		return MiniDirectShipRequest{}, err
	}
	return repo.SubmitMiniDirectShip(ctx, cmd)
}

func (s *Service) ListMiniDirectShipRequests(ctx context.Context, customerID int64, limit int) ([]MiniDirectShipRequest, error) {
	if customerID <= 0 {
		return nil, fmt.Errorf("customer required")
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	repo, err := s.miniDirectShipRepository()
	if err != nil {
		return nil, err
	}
	return repo.ListMiniDirectShipRequests(ctx, customerID, limit)
}

func (s *Service) GetMiniDirectShipRequest(ctx context.Context, customerID, requestID int64) (MiniDirectShipRequest, error) {
	if customerID <= 0 || requestID <= 0 {
		return MiniDirectShipRequest{}, fmt.Errorf("customer and request required")
	}
	repo, err := s.miniDirectShipRepository()
	if err != nil {
		return MiniDirectShipRequest{}, err
	}
	return repo.GetMiniDirectShipRequest(ctx, customerID, requestID)
}

func (s *Service) CancelMiniDirectShipRequest(ctx context.Context, customerID, requestID int64, actor string) (MiniDirectShipRequest, error) {
	if customerID <= 0 || requestID <= 0 {
		return MiniDirectShipRequest{}, fmt.Errorf("customer and request required")
	}
	repo, err := s.miniDirectShipRepository()
	if err != nil {
		return MiniDirectShipRequest{}, err
	}
	return repo.CancelMiniDirectShipRequest(ctx, customerID, requestID, strings.TrimSpace(actor))
}

func (s *Service) ListCustomerCentralInventory(ctx context.Context, customerID int64) ([]CustomerInventorySummary, error) {
	if customerID <= 0 {
		return nil, fmt.Errorf("customer required")
	}
	repo, err := s.miniDirectShipRepository()
	if err != nil {
		return nil, err
	}
	return repo.ListCustomerCentralInventory(ctx, customerID)
}

func (s *Service) ListCustomerCentralInventoryBatches(ctx context.Context, customerID, productID, specG int64) ([]CustomerInventoryBatch, error) {
	if customerID <= 0 || productID <= 0 {
		return nil, fmt.Errorf("customer and product required")
	}
	if specG < 0 {
		return nil, fmt.Errorf("spec invalid")
	}
	repo, err := s.miniDirectShipRepository()
	if err != nil {
		return nil, err
	}
	return repo.ListCustomerCentralInventoryBatches(ctx, customerID, productID, specG)
}

func normalizeMiniDirectShipCommand(cmd MiniDirectShipCommand, requireIdempotency bool) (MiniDirectShipCommand, error) {
	if cmd.CustomerID <= 0 {
		return MiniDirectShipCommand{}, fmt.Errorf("customer required")
	}
	cmd.IdempotencyKey = strings.TrimSpace(cmd.IdempotencyKey)
	if requireIdempotency && cmd.IdempotencyKey == "" {
		return MiniDirectShipCommand{}, fmt.Errorf("idempotency_key required")
	}
	if len(cmd.IdempotencyKey) > 160 {
		return MiniDirectShipCommand{}, fmt.Errorf("idempotency_key too long")
	}
	cmd.RecipientName = compactMiniDirectShipText(cmd.RecipientName)
	cmd.RecipientPhone = strings.TrimSpace(cmd.RecipientPhone)
	cmd.Province = compactMiniDirectShipText(cmd.Province)
	cmd.City = compactMiniDirectShipText(cmd.City)
	cmd.District = compactMiniDirectShipText(cmd.District)
	cmd.DetailAddress = compactMiniDirectShipText(cmd.DetailAddress)
	cmd.RecipientCompany = compactMiniDirectShipText(cmd.RecipientCompany)
	cmd.Note = strings.TrimSpace(cmd.Note)
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.RecipientName == "" {
		return MiniDirectShipCommand{}, fmt.Errorf("recipient_name required")
	}
	if cmd.RecipientPhone == "" {
		return MiniDirectShipCommand{}, fmt.Errorf("recipient_phone required")
	}
	if cmd.DetailAddress == "" {
		return MiniDirectShipCommand{}, fmt.Errorf("detail_address required")
	}
	items, err := normalizeMiniDirectShipItems(cmd.Items)
	if err != nil {
		return MiniDirectShipCommand{}, err
	}
	cmd.Items = items
	return cmd, nil
}

func normalizeMiniDirectShipItems(items []MiniDirectShipItemCommand) ([]MiniDirectShipItemCommand, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("items required")
	}
	out := make([]MiniDirectShipItemCommand, 0, len(items))
	byKey := make(map[string]int, len(items))
	for _, item := range items {
		if item.ProductID <= 0 {
			return nil, fmt.Errorf("product required")
		}
		if item.SpecG <= 0 {
			return nil, fmt.Errorf("spec required")
		}
		if item.Qty <= 0 {
			return nil, fmt.Errorf("quantity required")
		}
		// Names and codes are response snapshots, never trusted from the mini
		// client when reserving stock.
		item.ProductName = ""
		item.SKUCode = ""
		item.SpecLabel = ""
		key := fmt.Sprintf("%d:%d", item.ProductID, item.SpecG)
		if idx, ok := byKey[key]; ok {
			out[idx].Qty += item.Qty
			continue
		}
		byKey[key] = len(out)
		out = append(out, item)
	}
	return out, nil
}

func compactMiniDirectShipText(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}
