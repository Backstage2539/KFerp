package customerfulfillment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	salesapp "orderapp/internal/application/sales"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	ErrMiniDirectShipUnavailable       = errors.New("mini direct ship unavailable")
	ErrMiniDirectShipStockInsufficient = errors.New("customer finished stock insufficient")
	ErrMiniDirectShipRequestNotFound   = errors.New("direct ship request not found")
	ErrMiniDirectShipIdempotency       = errors.New("idempotency key already used with different request")
	ErrMiniDirectShipCannotCancel      = errors.New("shipped request cannot be cancelled")
	ErrMiniDirectShipPriceChanged      = errors.New("direct ship price quote changed")
)

type MiniDirectShipItemCommand struct {
	ProductID              int64   `json:"product_id"`
	BomSpecID              int64   `json:"bom_spec_id,omitempty"`
	BomVariantID           int64   `json:"bom_variant_id,omitempty"`
	BomSpecKey             string  `json:"bom_spec_key,omitempty"`
	ProductName            string  `json:"product_name,omitempty"`
	SKUCode                string  `json:"sku_code,omitempty"`
	SpecLabel              string  `json:"spec_label,omitempty"`
	InventoryUnit          string  `json:"inventory_unit,omitempty"`
	SpecG                  int64   `json:"spec_g"`
	Qty                    int64   `json:"qty"`
	SalesUnit              string  `json:"sales_unit,omitempty"`
	UnitPrice              float64 `json:"unit_price,omitempty"`
	LineAmount             float64 `json:"line_amount,omitempty"`
	IsProcessingProduct    bool    `json:"is_processing_product,omitempty"`
	StockReservedQty       int64   `json:"stock_reserved_qty,omitempty"`
	ProductionReservedQty  int64   `json:"production_reserved_qty,omitempty"`
	ProductionConvertedQty int64   `json:"production_converted_qty,omitempty"`
	ProductionShortfallQty int64   `json:"production_shortfall_qty,omitempty"`
}

type MiniDirectShipCommand struct {
	UsageCode        string                      `json:"-"`
	CustomerID       int64                       `json:"-"`
	EmployeeID       int64                       `json:"-"`
	MiniUserID       int64                       `json:"-"`
	IdempotencyKey   string                      `json:"idempotency_key"`
	PriceQuoteToken  string                      `json:"price_quote_token,omitempty"`
	OrderDate        string                      `json:"order_date,omitempty"`
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
	UsageCode  string
}

type MiniDirectShipCategory struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type MiniDirectShipCatalog struct {
	CurrentCustomerID int64                      `json:"current_customer_id"`
	Categories        []MiniDirectShipCategory   `json:"categories,omitempty"`
	ProductFamilies   []map[string]any           `json:"product_families"`
	PriceTables       []MiniDirectShipPriceTable `json:"price_tables"`
}

type MiniDirectShipPriceTable struct {
	ID        int64  `json:"id"`
	TableKey  string `json:"table_key"`
	TableName string `json:"table_name"`
	VersionNo string `json:"version_no"`
	ListType  string `json:"list_type"`
}

type MiniOrderPriceTablePreviewQuery struct {
	CustomerID int64
	UsageCode  string
}

type MiniOrderPriceTablePreviewRow struct {
	PublicationID int64    `json:"publication_id"`
	TableName     string   `json:"table_name"`
	VersionNo     string   `json:"version_no"`
	ProductID     int64    `json:"product_id"`
	ProductName   string   `json:"product_name"`
	BomSpecID     int64    `json:"bom_spec_id,omitempty"`
	BomVariantID  int64    `json:"bom_variant_id,omitempty"`
	SpecName      string   `json:"spec_name"`
	SalesUnit     string   `json:"sales_unit"`
	MinQty        float64  `json:"min_qty"`
	MaxQty        *float64 `json:"max_qty,omitempty"`
	UnitPrice     float64  `json:"unit_price"`
	SortOrder     int      `json:"sort_order,omitempty"`
	IsDefault     bool     `json:"is_default,omitempty"`
}

type MiniOrderPriceTablePreview struct {
	CurrentCustomerID int64                           `json:"current_customer_id"`
	UsageCode         string                          `json:"usage_code"`
	PriceTables       []MiniDirectShipPriceTable      `json:"price_tables"`
	Rows              []MiniOrderPriceTablePreviewRow `json:"rows"`
}

type MiniDirectShipPreviewWarehouse struct {
	Warehouse string                      `json:"warehouse"`
	Items     []MiniDirectShipItemCommand `json:"items"`
}

type MiniDirectShipShortage struct {
	ProductID              int64  `json:"product_id"`
	BomSpecID              int64  `json:"bom_spec_id,omitempty"`
	BomVariantID           int64  `json:"bom_variant_id,omitempty"`
	SpecG                  int64  `json:"spec_g"`
	Qty                    int64  `json:"qty"`
	AvailableQty           int64  `json:"available_qty"`
	StockAvailableQty      int64  `json:"stock_available_qty"`
	ProductionAvailableQty int64  `json:"production_available_qty"`
	Blocking               bool   `json:"blocking"`
	BlockingReason         string `json:"blocking_reason,omitempty"`
}

type MiniDirectShipProductionAllocation struct {
	ProcessingRequestID     int64 `json:"processing_request_id"`
	ProcessingRequestItemID int64 `json:"processing_request_item_id"`
	ProductID               int64 `json:"product_id"`
	BomSpecID               int64 `json:"bom_spec_id,omitempty"`
	BomVariantID            int64 `json:"bom_variant_id,omitempty"`
	SpecG                   int64 `json:"spec_g"`
	Qty                     int64 `json:"qty"`
}

type MiniDirectShipPreview struct {
	CanSubmit             bool                                 `json:"can_submit"`
	StockReady            bool                                 `json:"stock_ready"`
	TotalAmount           float64                              `json:"total_amount"`
	PriceQuoteToken       string                               `json:"price_quote_token"`
	PriceTables           []MiniDirectShipPriceTable           `json:"price_tables"`
	Items                 []MiniDirectShipItemCommand          `json:"items"`
	Warehouses            []MiniDirectShipPreviewWarehouse     `json:"warehouses"`
	Shortages             []MiniDirectShipShortage             `json:"shortages,omitempty"`
	ProductionAllocations []MiniDirectShipProductionAllocation `json:"production_allocations,omitempty"`
}

type MiniDirectShipPackage struct {
	ID            int64                         `json:"id"`
	OrderID       int64                         `json:"order_id"`
	OrderNo       string                        `json:"order_no"`
	Warehouse     string                        `json:"warehouse"`
	Status        string                        `json:"status"`
	ProcessStatus string                        `json:"process_status,omitempty"`
	ShipStatus    string                        `json:"ship_status,omitempty"`
	CarrierName   string                        `json:"carrier_name,omitempty"`
	TrackingNo    string                        `json:"tracking_no,omitempty"`
	ShippedAt     string                        `json:"shipped_at,omitempty"`
	DeliveredAt   string                        `json:"delivered_at,omitempty"`
	Items         []MiniDirectShipItemCommand   `json:"items,omitempty"`
	Events        []MiniDirectShipTrackingEvent `json:"events,omitempty"`
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
	OrderID          int64                       `json:"order_id,omitempty"`
	OrderNo          string                      `json:"order_no,omitempty"`
	TotalAmount      float64                     `json:"total_amount,omitempty"`
	PriceTables      []MiniDirectShipPriceTable  `json:"price_tables,omitempty"`
}

type PreparedMiniDirectShipOrder struct {
	Command               MiniDirectShipCommand
	RequestHash           string
	SelectedPriceTableIDs []int64
	PriceTables           []MiniDirectShipPriceTable
	Existing              *MiniDirectShipRequest
}

type MiniDirectShipOrderRepository interface {
	DirectShipPriceTables(context.Context, int64) ([]MiniDirectShipPriceTable, error)
	PrepareMiniDirectShipOrder(context.Context, MiniDirectShipCommand) (PreparedMiniDirectShipOrder, error)
	RecordMiniDirectShipOrder(context.Context, PreparedMiniDirectShipOrder, DirectShipOrderSummary) (MiniDirectShipRequest, error)
}

type MiniDirectShipCatalogEnricher interface {
	EnrichMiniDirectShipCatalog(context.Context, MiniDirectShipCatalogQuery, MiniDirectShipCatalog) (MiniDirectShipCatalog, error)
}

type MiniDirectShipAtomicOrderRepository interface {
	SubmitPreparedMiniDirectShipOrder(context.Context, PreparedMiniDirectShipOrder, SubmitCustomerDirectShipOrderCommand) (MiniDirectShipRequest, error)
}

type MiniCustomerOrderPriceTableRepository interface {
	CustomerOrderPriceTables(context.Context, int64, string) ([]MiniDirectShipPriceTable, error)
}

type MiniDirectShipListQuery struct {
	CustomerID  int64
	Q           string
	ShippedFrom string
	ShippedTo   string
	Page        int
	Limit       int
}

type MiniDirectShipListResult struct {
	Rows       []MiniDirectShipRequest `json:"rows"`
	Total      int                     `json:"total"`
	Page       int                     `json:"page"`
	Limit      int                     `json:"limit"`
	TotalPages int                     `json:"total_pages"`
	HasNext    bool                    `json:"has_next"`
}

type CustomerInventorySummary struct {
	ProductID       int64    `json:"product_id"`
	BomSpecID       int64    `json:"bom_spec_id,omitempty"`
	BomVariantID    int64    `json:"bom_variant_id,omitempty"`
	BomSpecKey      string   `json:"bom_spec_key,omitempty"`
	BomSpecName     string   `json:"bom_spec_name,omitempty"`
	InventoryUnit   string   `json:"inventory_unit,omitempty"`
	IsDefaultSpec   bool     `json:"is_default_spec,omitempty"`
	ProductName     string   `json:"product_name"`
	ParentProductID int64    `json:"parent_product_id,omitempty"`
	SKUCode         string   `json:"sku_code,omitempty"`
	SpecG           int64    `json:"spec_g"`
	AvailableQty    int64    `json:"available_qty"`
	ReservedQty     int64    `json:"reserved_qty"`
	TotalQty        int64    `json:"total_qty"`
	Warehouses      []string `json:"warehouses"`
}

type CustomerInventoryListQuery struct {
	CustomerID int64
	Q          string
	Page       int
	Limit      int
	LegacyAll  bool
}

type CustomerInventoryListResult struct {
	Rows       []CustomerInventorySummary `json:"rows"`
	Total      int                        `json:"total"`
	Page       int                        `json:"page"`
	Limit      int                        `json:"limit"`
	TotalPages int                        `json:"total_pages"`
	HasNext    bool                       `json:"has_next"`
}

type CustomerInventoryBatch struct {
	BatchID                         int64  `json:"batch_id"`
	BatchNo                         string `json:"batch_no"`
	ProductID                       int64  `json:"product_id"`
	BomSpecID                       int64  `json:"bom_spec_id,omitempty"`
	BomVariantID                    int64  `json:"bom_variant_id,omitempty"`
	BomSpecKey                      string `json:"bom_spec_key,omitempty"`
	BomSpecName                     string `json:"bom_spec_name,omitempty"`
	InventoryUnit                   string `json:"inventory_unit,omitempty"`
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

type CustomerInventoryBatchQuery struct {
	CustomerID   int64
	ProductID    int64
	BomSpecID    int64
	BomVariantID int64
	SpecG        int64
}

type CustomerAssetWarehouseBalance struct {
	WarehouseCode   string  `json:"warehouse_code"`
	WarehouseName   string  `json:"warehouse_name"`
	AvailableQty    float64 `json:"available_qty"`
	OccupiedQty     float64 `json:"occupied_qty"`
	InProductionQty float64 `json:"in_production_qty"`
}

type CustomerAssetInventoryBatch struct {
	BatchID         int64   `json:"batch_id"`
	BatchNo         string  `json:"batch_no"`
	WarehouseCode   string  `json:"warehouse_code"`
	WarehouseName   string  `json:"warehouse_name"`
	TotalQty        float64 `json:"total_qty"`
	AvailableQty    float64 `json:"available_qty"`
	OccupiedQty     float64 `json:"occupied_qty"`
	InProductionQty float64 `json:"in_production_qty"`
	QualityStatus   string  `json:"quality_status"`
	InboundAt       string  `json:"inbound_at,omitempty"`
}

type CustomerAssetInventory struct {
	InventoryType              string                          `json:"inventory_type"`
	ItemID                     int64                           `json:"item_id"`
	ProductID                  int64                           `json:"product_id,omitempty"`
	BomSpecID                  int64                           `json:"bom_spec_id,omitempty"`
	BomVariantID               int64                           `json:"bom_variant_id,omitempty"`
	SpecG                      int64                           `json:"spec_g,omitempty"`
	ItemCode                   string                          `json:"item_code,omitempty"`
	ItemName                   string                          `json:"item_name"`
	Spec                       string                          `json:"spec,omitempty"`
	Unit                       string                          `json:"unit"`
	TotalQty                   float64                         `json:"total_qty"`
	AvailableQty               float64                         `json:"available_qty"`
	OccupiedQty                float64                         `json:"occupied_qty"`
	InProductionQty            float64                         `json:"in_production_qty"`
	QualityStatus              string                          `json:"quality_status"`
	Legacy                     bool                            `json:"legacy,omitempty"`
	CanCreateProcessingRequest bool                            `json:"can_create_processing_request,omitempty"`
	Warehouses                 []CustomerAssetWarehouseBalance `json:"warehouses"`
	Batches                    []CustomerAssetInventoryBatch   `json:"batches"`
}

type CustomerAssetInventoryQuery struct {
	CustomerID    int64
	InventoryType string
	Q             string
}

type CustomerAssetInventoryLedgerQuery struct {
	CustomerID    int64
	InventoryType string
	ItemID        int64
	BomSpecID     int64
	SpecG         int64
	Limit         int
}

type CustomerAssetInventoryLedgerEntry struct {
	ID            int64   `json:"id"`
	OccurredAt    string  `json:"occurred_at"`
	MovementType  string  `json:"movement_type"`
	SourceType    string  `json:"source_type"`
	SourceID      int64   `json:"source_id,omitempty"`
	SourceNo      string  `json:"source_no,omitempty"`
	BatchNo       string  `json:"batch_no,omitempty"`
	WarehouseCode string  `json:"warehouse_code,omitempty"`
	WarehouseName string  `json:"warehouse_name,omitempty"`
	QuantityDelta float64 `json:"quantity_delta"`
	Unit          string  `json:"unit"`
	Note          string  `json:"note,omitempty"`
}

type CustomerAssetInventoryRepository interface {
	ListCustomerAssetInventory(context.Context, CustomerAssetInventoryQuery) ([]CustomerAssetInventory, error)
	ListCustomerAssetInventoryLedger(context.Context, CustomerAssetInventoryLedgerQuery) ([]CustomerAssetInventoryLedgerEntry, error)
}

// MiniDirectShipRepository is kept separate from the legacy fulfillment
// repository contract so the closed-loop endpoints can be introduced without
// widening every test double used by the import and settlement workflows.
type MiniDirectShipRepository interface {
	MiniDirectShipCatalog(context.Context, MiniDirectShipCatalogQuery) (MiniDirectShipCatalog, error)
	PreviewMiniDirectShip(context.Context, MiniDirectShipCommand) (MiniDirectShipPreview, error)
	SubmitMiniDirectShip(context.Context, MiniDirectShipCommand) (MiniDirectShipRequest, error)
	ListMiniDirectShipRequests(context.Context, MiniDirectShipListQuery) (MiniDirectShipListResult, error)
	GetMiniDirectShipRequest(context.Context, int64, int64) (MiniDirectShipRequest, error)
	CancelMiniDirectShipRequest(context.Context, int64, int64, string) (MiniDirectShipRequest, error)
	ListCustomerCentralInventory(context.Context, int64) ([]CustomerInventorySummary, error)
	ListCustomerCentralInventoryBatches(context.Context, CustomerInventoryBatchQuery) ([]CustomerInventoryBatch, error)
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
	query.UsageCode = strings.TrimSpace(query.UsageCode)
	if query.UsageCode == "" {
		query.UsageCode = "direct_ship"
	}
	if query.UsageCode != "direct_ship" && query.UsageCode != "product_order" {
		return MiniDirectShipCatalog{}, fmt.Errorf("invalid customer order usage")
	}
	repo, err := s.miniDirectShipRepository()
	if err != nil {
		return MiniDirectShipCatalog{}, err
	}
	if bridge, ok := repo.(MiniDirectShipOrderRepository); ok && s.sales != nil {
		tables, tableErr := bridge.DirectShipPriceTables(ctx, query.CustomerID)
		if exactRepo, exactOK := repo.(MiniCustomerOrderPriceTableRepository); exactOK {
			tables, tableErr = exactRepo.CustomerOrderPriceTables(ctx, query.CustomerID, query.UsageCode)
		}
		if tableErr != nil {
			return MiniDirectShipCatalog{}, tableErr
		}
		form, formErr := s.sales.OrderForm(ctx, 0)
		if formErr != nil {
			return MiniDirectShipCatalog{}, formErr
		}
		ids := make([]int64, 0, len(tables))
		for _, table := range tables {
			ids = append(ids, table.ID)
		}
		products, filterErr := salesapp.FilterOrderProductsForExactCustomerPublications(form.Products, query.CustomerID, form.BeanListVersionOptions, form.CustomerPublicUsages, ids, false)
		if filterErr != nil {
			return MiniDirectShipCatalog{}, filterErr
		}
		catalog := miniDirectShipCatalogFromSales(query, tables, products, form.ProductBOMSpecOptions)
		if enricher, ok := repo.(MiniDirectShipCatalogEnricher); ok {
			return enricher.EnrichMiniDirectShipCatalog(ctx, query, catalog)
		}
		return catalog, nil
	}
	if query.UsageCode == "product_order" {
		return MiniDirectShipCatalog{}, ErrMiniDirectShipUnavailable
	}
	return repo.MiniDirectShipCatalog(ctx, query)
}

func (s *Service) MiniOrderPriceTablePreview(ctx context.Context, query MiniOrderPriceTablePreviewQuery) (MiniOrderPriceTablePreview, error) {
	if query.CustomerID <= 0 {
		return MiniOrderPriceTablePreview{}, fmt.Errorf("customer required")
	}
	query.UsageCode = strings.TrimSpace(query.UsageCode)
	if query.UsageCode != "direct_ship" && query.UsageCode != "product_order" {
		return MiniOrderPriceTablePreview{}, fmt.Errorf("invalid customer order usage")
	}
	repo, err := s.miniDirectShipRepository()
	if err != nil {
		return MiniOrderPriceTablePreview{}, err
	}
	bridge, ok := repo.(MiniDirectShipOrderRepository)
	if !ok || s.sales == nil {
		return MiniOrderPriceTablePreview{}, ErrMiniDirectShipUnavailable
	}
	tables, err := bridge.DirectShipPriceTables(ctx, query.CustomerID)
	if exactRepo, exactOK := repo.(MiniCustomerOrderPriceTableRepository); exactOK {
		tables, err = exactRepo.CustomerOrderPriceTables(ctx, query.CustomerID, query.UsageCode)
	}
	if err != nil {
		return MiniOrderPriceTablePreview{}, err
	}
	form, err := s.sales.OrderForm(ctx, 0)
	if err != nil {
		return MiniOrderPriceTablePreview{}, err
	}
	ids := make([]int64, 0, len(tables))
	for _, table := range tables {
		ids = append(ids, table.ID)
	}
	products, err := salesapp.FilterOrderProductsForExactCustomerPublications(form.Products, query.CustomerID, form.BeanListVersionOptions, form.CustomerPublicUsages, ids, false)
	if err != nil {
		return MiniOrderPriceTablePreview{}, err
	}
	return miniOrderPriceTablePreviewFromSales(query, tables, products, form.ProductBOMSpecOptions), nil
}

func (s *Service) MiniProductOrderCatalog(ctx context.Context, query MiniDirectShipCatalogQuery) (MiniDirectShipCatalog, error) {
	query.UsageCode = "product_order"
	return s.MiniDirectShipCatalog(ctx, query)
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
	preview, err := repo.PreviewMiniDirectShip(ctx, cmd)
	if err != nil {
		return MiniDirectShipPreview{}, err
	}
	if bridge, ok := repo.(MiniDirectShipOrderRepository); ok && s.sales != nil {
		prepared, prepareErr := bridge.PrepareMiniDirectShipOrder(ctx, cmd)
		if prepareErr != nil {
			return MiniDirectShipPreview{}, prepareErr
		}
		preview.PriceTables = prepared.PriceTables
		preview.Items, preview.TotalAmount, err = s.pricePreparedMiniDirectShipOrder(ctx, prepared)
		if err != nil {
			return MiniDirectShipPreview{}, err
		}
		preview.PriceQuoteToken, err = miniDirectShipPriceQuoteToken(prepared, preview.Items)
		if err != nil {
			return MiniDirectShipPreview{}, err
		}
	}
	return preview, nil
}

func (s *Service) PreviewMiniProductOrder(ctx context.Context, cmd MiniDirectShipCommand) (MiniDirectShipPreview, error) {
	cmd.UsageCode = "product_order"
	return s.PreviewMiniDirectShip(ctx, cmd)
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
	if bridge, ok := repo.(MiniDirectShipOrderRepository); ok && s.sales != nil {
		prepared, prepareErr := bridge.PrepareMiniDirectShipOrder(ctx, cmd)
		if prepareErr != nil {
			return MiniDirectShipRequest{}, prepareErr
		}
		if prepared.Existing != nil {
			return *prepared.Existing, nil
		}
		pricedItems, _, priceErr := s.pricePreparedMiniDirectShipOrder(ctx, prepared)
		if priceErr != nil {
			return MiniDirectShipRequest{}, priceErr
		}
		if quoteErr := requireMiniDirectShipPriceQuote(cmd.PriceQuoteToken, prepared, pricedItems); quoteErr != nil {
			return MiniDirectShipRequest{}, quoteErr
		}
		items := make([]SubmitCustomerDirectShipOrderItem, 0, len(prepared.Command.Items))
		for _, item := range prepared.Command.Items {
			items = append(items, SubmitCustomerDirectShipOrderItem{
				ProductID: item.ProductID, BomSpecID: item.BomSpecID, BomVariantID: item.BomVariantID,
				BomSpecKey: item.BomSpecKey, InventoryUnit: item.InventoryUnit, ProductName: item.ProductName,
				Spec: item.SpecLabel, SpecG: item.SpecG, SalesUnit: item.SalesUnit, QuantityUnits: item.Qty,
			})
		}
		fullAddress := strings.TrimSpace(prepared.Command.Province + prepared.Command.City + prepared.Command.District + prepared.Command.DetailAddress)
		usageCode := prepared.Command.UsageCode
		if usageCode == "" {
			usageCode = "direct_ship"
		}
		orderCommand := SubmitCustomerDirectShipOrderCommand{
			CustomerID: prepared.Command.CustomerID, ReceiverName: prepared.Command.RecipientName,
			ReceiverPhone: prepared.Command.RecipientPhone, ReceiverAddress: fullAddress,
			ReceiverCompany: prepared.Command.RecipientCompany, Items: items, Note: prepared.Command.Note,
			OrderDate:           prepared.Command.OrderDate,
			Actor:               prepared.Command.Actor,
			CustomerRequestID:   fmt.Sprintf("mini-%s:%d:%s", usageCode, prepared.Command.CustomerID, prepared.Command.IdempotencyKey),
			CustomerRequestHash: prepared.RequestHash, SelectedPriceTableIDs: prepared.SelectedPriceTableIDs,
			PortalServiceCode: usageCode,
		}
		if atomic, ok := repo.(MiniDirectShipAtomicOrderRepository); ok {
			return atomic.SubmitPreparedMiniDirectShipOrder(ctx, prepared, orderCommand)
		}
		order, orderErr := s.SubmitCustomerDirectShipOrder(ctx, orderCommand)
		if orderErr != nil {
			return MiniDirectShipRequest{}, orderErr
		}
		return bridge.RecordMiniDirectShipOrder(ctx, prepared, order)
	}
	return repo.SubmitMiniDirectShip(ctx, cmd)
}

func (s *Service) SubmitMiniProductOrder(ctx context.Context, cmd MiniDirectShipCommand) (MiniDirectShipRequest, error) {
	cmd.UsageCode = "product_order"
	return s.SubmitMiniDirectShip(ctx, cmd)
}

func (s *Service) pricePreparedMiniDirectShipOrder(ctx context.Context, prepared PreparedMiniDirectShipOrder) ([]MiniDirectShipItemCommand, float64, error) {
	form, err := s.sales.OrderForm(ctx, 0)
	if err != nil {
		return nil, 0, err
	}
	products, err := salesapp.FilterOrderProductsForExactCustomerPublications(
		form.Products,
		prepared.Command.CustomerID,
		form.BeanListVersionOptions,
		form.CustomerPublicUsages,
		prepared.SelectedPriceTableIDs,
		false,
	)
	if err != nil {
		return nil, 0, err
	}
	return priceMiniDirectShipItems(prepared.Command.Items, products)
}

func miniDirectShipPriceQuoteToken(prepared PreparedMiniDirectShipOrder, items []MiniDirectShipItemCommand) (string, error) {
	tables := append([]MiniDirectShipPriceTable(nil), prepared.PriceTables...)
	sort.Slice(tables, func(i, j int) bool {
		if tables[i].ID != tables[j].ID {
			return tables[i].ID < tables[j].ID
		}
		return tables[i].ListType < tables[j].ListType
	})
	quotedItems := append([]MiniDirectShipItemCommand(nil), items...)
	sort.Slice(quotedItems, func(i, j int) bool {
		if quotedItems[i].ProductID != quotedItems[j].ProductID {
			return quotedItems[i].ProductID < quotedItems[j].ProductID
		}
		if quotedItems[i].BomSpecID != quotedItems[j].BomSpecID {
			return quotedItems[i].BomSpecID < quotedItems[j].BomSpecID
		}
		if quotedItems[i].SpecG != quotedItems[j].SpecG {
			return quotedItems[i].SpecG < quotedItems[j].SpecG
		}
		return quotedItems[i].SalesUnit < quotedItems[j].SalesUnit
	})
	raw, err := json.Marshal(struct {
		Tables []MiniDirectShipPriceTable  `json:"tables"`
		Items  []MiniDirectShipItemCommand `json:"items"`
	}{Tables: tables, Items: quotedItems})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func requireMiniDirectShipPriceQuote(expected string, prepared PreparedMiniDirectShipOrder, items []MiniDirectShipItemCommand) error {
	actual, err := miniDirectShipPriceQuoteToken(prepared, items)
	if err != nil {
		return err
	}
	if strings.TrimSpace(expected) == "" || strings.TrimSpace(expected) != actual {
		return fmt.Errorf("%w：价格表或商品价格已更新，请重新预览并确认", ErrMiniDirectShipPriceChanged)
	}
	return nil
}

type miniOrderBOMSpecKey struct {
	parentID, specID, variantID int64
}

func miniOrderBOMSpecOptions(customerID int64, options []salesapp.ProductBOMSpecOption) map[miniOrderBOMSpecKey]salesapp.ProductBOMSpecOption {
	out := make(map[miniOrderBOMSpecKey]salesapp.ProductBOMSpecOption)
	for _, option := range options {
		if !option.Published || option.ParentProductID <= 0 || option.BomSpecID <= 0 || (option.OwnerCustomerID != 0 && option.OwnerCustomerID != customerID) {
			continue
		}
		key := miniOrderBOMSpecKey{option.ParentProductID, option.BomSpecID, option.BomVariantID}
		current, exists := out[key]
		if !exists || current.OwnerCustomerID == 0 && option.OwnerCustomerID == customerID {
			out[key] = option
		}
		if option.BomVariantID > 0 {
			fallback := miniOrderBOMSpecKey{option.ParentProductID, option.BomSpecID, 0}
			if _, exists := out[fallback]; !exists {
				out[fallback] = option
			}
		}
	}
	return out
}

func miniOrderBOMSpecOption(options map[miniOrderBOMSpecKey]salesapp.ProductBOMSpecOption, parentID, specID, variantID int64) (salesapp.ProductBOMSpecOption, bool) {
	option, ok := options[miniOrderBOMSpecKey{parentID, specID, variantID}]
	if !ok {
		option, ok = options[miniOrderBOMSpecKey{parentID, specID, 0}]
	}
	return option, ok
}

func miniOrderTierBOMSpecIdentity(tier salesapp.ProductTierOption) (int64, int64) {
	specID, variantID := tier.BomSpecID, tier.BomVariantID
	if specID <= 0 {
		specID = miniOrderMapInt64(tier.EffectiveSalesSpec, "bom_spec_id")
	}
	if variantID <= 0 {
		variantID = miniOrderMapInt64(tier.EffectiveSalesSpec, "bom_variant_id")
	}
	return specID, variantID
}

func miniOrderMapInt64(values map[string]any, key string) int64 {
	value, ok := values[key]
	if !ok {
		return 0
	}
	switch value := value.(type) {
	case int:
		return int64(value)
	case int64:
		return value
	case float64:
		return int64(value)
	case json.Number:
		parsed, _ := value.Int64()
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		return parsed
	default:
		return 0
	}
}

func miniDirectShipCatalogFromSales(query MiniDirectShipCatalogQuery, tables []MiniDirectShipPriceTable, products []salesapp.ProductOption, specOptions []salesapp.ProductBOMSpecOption) MiniDirectShipCatalog {
	type familyState struct {
		row   map[string]any
		specs []map[string]any
	}
	byParent := map[int64]*familyState{}
	families := make([]*familyState, 0)
	categorySeen := map[string]bool{}
	categories := make([]MiniDirectShipCategory, 0)
	q := strings.ToLower(strings.TrimSpace(query.Q))
	categoryFilter := strings.TrimSpace(query.Category)
	options := miniOrderBOMSpecOptions(query.CustomerID, specOptions)
	for _, product := range products {
		categoryKey := ""
		if product.ProductTypeCategoryID > 0 {
			categoryKey = fmt.Sprint(product.ProductTypeCategoryID)
			if !categorySeen[categoryKey] {
				categorySeen[categoryKey] = true
				categories = append(categories, MiniDirectShipCategory{Key: categoryKey, Label: product.ProductTypeName})
			}
		}
		if categoryFilter != "" && categoryFilter != categoryKey && !strings.EqualFold(categoryFilter, product.ProductTypeName) {
			continue
		}
		name := strings.TrimSpace(product.CustomerProductDisplayName)
		if name == "" {
			name = strings.TrimSpace(product.ParentProductName)
		}
		if name == "" {
			name = strings.TrimSpace(product.Name)
		}
		search := strings.ToLower(strings.Join([]string{name, product.Name, product.SKUName, product.SKUCode, product.CustomerItemCode}, " "))
		if q != "" && !strings.Contains(search, q) {
			continue
		}
		parentID := product.ParentProductID
		if parentID <= 0 {
			parentID = product.ID
		}
		state := byParent[parentID]
		if state == nil {
			state = &familyState{row: map[string]any{
				"parent_product_id": parentID, "parent_product_name": product.ParentProductName,
				"name": name, "customer_product_display_name": product.CustomerProductDisplayName,
				"customer_item_code": product.CustomerItemCode, "product_code": product.ProductCode,
				"product_type_name": product.ProductTypeName, "product_kind": product.ProductKind,
			}}
			byParent[parentID] = state
			families = append(families, state)
		}
		tierGroups := map[string][]salesapp.ProductTierOption{}
		tierGroupOrder := make([]string, 0)
		for _, tier := range product.Tiers {
			specID, variantID := miniOrderTierBOMSpecIdentity(tier)
			key := fmt.Sprintf("%d:%d:%d:%s:%d", specID, variantID, tier.SpecG, tier.SalesUnit, tier.UnitBagCount)
			if _, exists := tierGroups[key]; !exists {
				tierGroupOrder = append(tierGroupOrder, key)
			}
			tierGroups[key] = append(tierGroups[key], tier)
		}
		for _, groupKey := range tierGroupOrder {
			tiers := tierGroups[groupKey]
			first := tiers[0]
			bomSpecID, bomVariantID := miniOrderTierBOMSpecIdentity(first)
			specLabel := strings.TrimSpace(product.SpecLabel)
			option, hasOption := miniOrderBOMSpecOption(options, parentID, bomSpecID, bomVariantID)
			if bomSpecID > 0 {
				if hasOption {
					specLabel = strings.TrimSpace(option.SpecName)
				} else {
					specLabel = strings.TrimSpace(product.SKUName)
				}
			}
			if specLabel == "" && first.SpecG > 0 {
				specLabel = fmt.Sprintf("%dg", first.SpecG)
			}
			priceTiers := make([]map[string]any, 0, len(tiers))
			for _, tier := range tiers {
				priceTiers = append(priceTiers, map[string]any{
					"min_qty": tier.MinQty, "max_qty": tier.MaxQty, "unit_price": tier.UnitPrice,
					"sales_unit": tier.SalesUnit, "publication_id": tier.PublicationID,
				})
			}
			specRow := map[string]any{
				"product_id": product.ID, "sku_id": product.SKUID, "bom_spec_id": bomSpecID,
				"bom_variant_id": bomVariantID, "sku_code": product.SKUCode, "sku_name": product.SKUName,
				"spec_label": specLabel, "net_content_qty": product.NetContentQty,
				"net_content_unit": product.NetContentUnit, "inventory_unit": product.InventoryUnit,
				"sales_unit": first.SalesUnit, "spec_g": first.SpecG, "unit_bag_count": first.UnitBagCount,
				"unit_price": first.UnitPrice, "price_tiers": priceTiers, "available_qty": int64(0),
			}
			if hasOption {
				specRow["product_id"] = option.ParentProductID
				specRow["sku_id"] = option.ParentProductID
				specRow["bom_spec_id"] = option.BomSpecID
				specRow["bom_variant_id"] = option.BomVariantID
				specRow["spec_code"] = option.SpecCode
				specRow["spec_key"] = option.SpecKey
				specRow["spec_name"] = option.SpecName
				specRow["spec_label"] = option.SpecName
				specRow["inventory_unit"] = option.InventoryUnit
				specRow["is_default"] = option.IsDefault
				specRow["is_default_sku"] = option.IsDefault
				specRow["sort_order"] = option.SortOrder
				specRow["migration_state"] = option.MigrationState
				if option.IsDefault {
					state.row["default_bom_spec_id"] = option.BomSpecID
				}
			}
			state.specs = append(state.specs, specRow)
		}
	}
	out := make([]map[string]any, 0, len(families))
	for _, family := range families {
		if len(family.specs) > 0 {
			sort.SliceStable(family.specs, func(i, j int) bool {
				leftDefault, _ := family.specs[i]["is_default"].(bool)
				rightDefault, _ := family.specs[j]["is_default"].(bool)
				if leftDefault != rightDefault {
					return leftDefault
				}
				leftOrder, _ := family.specs[i]["sort_order"].(int)
				rightOrder, _ := family.specs[j]["sort_order"].(int)
				if leftOrder != rightOrder {
					return leftOrder < rightOrder
				}
				return fmt.Sprint(family.specs[i]["spec_label"]) < fmt.Sprint(family.specs[j]["spec_label"])
			})
			family.row["specs"] = family.specs
			out = append(out, family.row)
		}
	}
	return MiniDirectShipCatalog{CurrentCustomerID: query.CustomerID, Categories: categories, ProductFamilies: out, PriceTables: tables}
}

func miniOrderPriceTablePreviewFromSales(query MiniOrderPriceTablePreviewQuery, tables []MiniDirectShipPriceTable, products []salesapp.ProductOption, specOptions []salesapp.ProductBOMSpecOption) MiniOrderPriceTablePreview {
	options := miniOrderBOMSpecOptions(query.CustomerID, specOptions)
	tableByID := make(map[int64]MiniDirectShipPriceTable, len(tables))
	for _, table := range tables {
		tableByID[table.ID] = table
	}
	rows := make([]MiniOrderPriceTablePreviewRow, 0)
	for _, product := range products {
		parentID := product.ParentProductID
		if parentID <= 0 {
			parentID = product.ID
		}
		productName := strings.TrimSpace(product.CustomerProductDisplayName)
		if productName == "" {
			productName = strings.TrimSpace(product.ParentProductName)
		}
		if productName == "" {
			productName = strings.TrimSpace(product.Name)
		}
		for _, tier := range product.Tiers {
			bomSpecID, bomVariantID := miniOrderTierBOMSpecIdentity(tier)
			option, hasOption := miniOrderBOMSpecOption(options, parentID, bomSpecID, bomVariantID)
			specName := strings.TrimSpace(product.SpecLabel)
			sortOrder := 0
			isDefault := product.IsDefaultSKU
			if bomSpecID > 0 {
				specName = strings.TrimSpace(product.SKUName)
			}
			if hasOption {
				specName, sortOrder, isDefault = strings.TrimSpace(option.SpecName), option.SortOrder, option.IsDefault
			}
			if specName == "" && tier.SpecG > 0 {
				specName = fmt.Sprintf("%dg", tier.SpecG)
			}
			table := tableByID[tier.PublicationID]
			rows = append(rows, MiniOrderPriceTablePreviewRow{
				PublicationID: tier.PublicationID, TableName: table.TableName, VersionNo: table.VersionNo,
				ProductID: parentID, ProductName: productName, BomSpecID: bomSpecID, BomVariantID: bomVariantID,
				SpecName: specName, SalesUnit: tier.SalesUnit, MinQty: tier.MinQty, MaxQty: tier.MaxQty,
				UnitPrice: tier.UnitPrice, SortOrder: sortOrder, IsDefault: isDefault,
			})
		}
	}
	return MiniOrderPriceTablePreview{CurrentCustomerID: query.CustomerID, UsageCode: query.UsageCode, PriceTables: tables, Rows: rows}
}

func priceMiniDirectShipItems(items []MiniDirectShipItemCommand, products []salesapp.ProductOption) ([]MiniDirectShipItemCommand, float64, error) {
	out := append([]MiniDirectShipItemCommand(nil), items...)
	total := float64(0)
	for idx := range out {
		item := &out[idx]
		found := false
		bestMin := float64(-1)
		for _, product := range products {
			if product.ID != item.ProductID && product.ParentProductID != item.ProductID {
				continue
			}
			for _, tier := range product.Tiers {
				tierBomSpecID, tierBomVariantID := miniOrderTierBOMSpecIdentity(tier)
				if item.BomSpecID > 0 {
					if tierBomSpecID != item.BomSpecID {
						continue
					}
					if item.BomVariantID > 0 && tierBomVariantID != item.BomVariantID {
						continue
					}
				} else if tier.SpecG != item.SpecG {
					continue
				}
				if item.SalesUnit != "" && tier.SalesUnit != "" && !strings.EqualFold(item.SalesUnit, tier.SalesUnit) {
					continue
				}
				qty := float64(item.Qty)
				if qty < tier.MinQty || tier.MaxQty != nil && qty > *tier.MaxQty || tier.UnitPrice <= 0 || tier.MinQty < bestMin {
					continue
				}
				bestMin = tier.MinQty
				item.UnitPrice = tier.UnitPrice
				item.SalesUnit = tier.SalesUnit
				item.LineAmount = float64(item.Qty) * tier.UnitPrice
				found = true
			}
		}
		if !found {
			return nil, 0, fmt.Errorf("商品「%s」不在指定价格表当前有效数量档位中", item.ProductName)
		}
		total += item.LineAmount
	}
	return out, total, nil
}

func (s *Service) ListMiniDirectShipRequests(ctx context.Context, query MiniDirectShipListQuery) (MiniDirectShipListResult, error) {
	query, err := normalizeMiniDirectShipListQuery(query)
	if err != nil {
		return MiniDirectShipListResult{}, err
	}
	repo, err := s.miniDirectShipRepository()
	if err != nil {
		return MiniDirectShipListResult{}, err
	}
	return repo.ListMiniDirectShipRequests(ctx, query)
}

func normalizeMiniDirectShipListQuery(query MiniDirectShipListQuery) (MiniDirectShipListQuery, error) {
	if query.CustomerID <= 0 {
		return MiniDirectShipListQuery{}, fmt.Errorf("customer required")
	}
	query.Q = strings.Join(strings.Fields(strings.TrimSpace(query.Q)), " ")
	query.ShippedFrom = strings.TrimSpace(query.ShippedFrom)
	query.ShippedTo = strings.TrimSpace(query.ShippedTo)
	var from, to time.Time
	var err error
	if query.ShippedFrom != "" {
		from, err = time.Parse("2006-01-02", query.ShippedFrom)
		if err != nil {
			return MiniDirectShipListQuery{}, fmt.Errorf("shipped_from invalid")
		}
	}
	if query.ShippedTo != "" {
		to, err = time.Parse("2006-01-02", query.ShippedTo)
		if err != nil {
			return MiniDirectShipListQuery{}, fmt.Errorf("shipped_to invalid")
		}
	}
	if !from.IsZero() && !to.IsZero() && from.After(to) {
		return MiniDirectShipListQuery{}, fmt.Errorf("shipment date range invalid")
	}
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.Limit <= 0 {
		query.Limit = 50
	} else if query.Limit > 100 {
		query.Limit = 100
	}
	return query, nil
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

func (s *Service) ListCustomerCentralInventory(ctx context.Context, query CustomerInventoryListQuery) (CustomerInventoryListResult, error) {
	query, err := normalizeCustomerInventoryListQuery(query)
	if err != nil {
		return CustomerInventoryListResult{}, err
	}
	repo, err := s.miniDirectShipRepository()
	if err != nil {
		return CustomerInventoryListResult{}, err
	}
	rows, err := repo.ListCustomerCentralInventory(ctx, query.CustomerID)
	if err != nil {
		return CustomerInventoryListResult{}, err
	}
	return customerInventoryListResult(rows, query), nil
}

func normalizeCustomerInventoryListQuery(query CustomerInventoryListQuery) (CustomerInventoryListQuery, error) {
	if query.CustomerID <= 0 {
		return CustomerInventoryListQuery{}, fmt.Errorf("customer required")
	}
	query.Q = normalizedCustomerInventorySearch(query.Q)
	if query.LegacyAll {
		query.Page = 1
		query.Limit = 0
		return query, nil
	}
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.Limit <= 0 {
		query.Limit = 20
	} else if query.Limit > 100 {
		query.Limit = 100
	}
	return query, nil
}

func customerInventoryListResult(rows []CustomerInventorySummary, query CustomerInventoryListQuery) CustomerInventoryListResult {
	filtered := make([]CustomerInventorySummary, 0, len(rows))
	for _, row := range rows {
		if query.Q != "" && !strings.Contains(normalizedCustomerInventorySearch(row.ProductName), query.Q) {
			continue
		}
		filtered = append(filtered, row)
	}
	total := len(filtered)
	if query.LegacyAll {
		limit := total
		if limit <= 0 {
			limit = 1
		}
		totalPages := 0
		if total > 0 {
			totalPages = 1
		}
		return CustomerInventoryListResult{
			Rows: filtered, Total: total, Page: 1, Limit: limit, TotalPages: totalPages,
		}
	}
	totalPages := 0
	if total > 0 {
		totalPages = (total + query.Limit - 1) / query.Limit
		if query.Page > totalPages {
			query.Page = totalPages
		}
	} else {
		query.Page = 1
	}
	offset := (query.Page - 1) * query.Limit
	end := offset + query.Limit
	if end > total {
		end = total
	}
	return CustomerInventoryListResult{
		Rows:       append([]CustomerInventorySummary(nil), filtered[offset:end]...),
		Total:      total,
		Page:       query.Page,
		Limit:      query.Limit,
		TotalPages: totalPages,
		HasNext:    query.Page < totalPages,
	}
}

func normalizedCustomerInventorySearch(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), ""))
}

func (s *Service) ListCustomerCentralInventoryBatches(ctx context.Context, query CustomerInventoryBatchQuery) ([]CustomerInventoryBatch, error) {
	if query.CustomerID <= 0 || query.ProductID <= 0 {
		return nil, fmt.Errorf("customer and product required")
	}
	if query.BomSpecID < 0 || query.BomVariantID < 0 || query.SpecG < 0 {
		return nil, fmt.Errorf("spec invalid")
	}
	canonical := query.BomSpecID > 0 || query.BomVariantID > 0
	if canonical {
		if query.BomSpecID <= 0 || query.SpecG > 0 {
			return nil, fmt.Errorf("bom spec invalid")
		}
		query.SpecG = 0
	} else if query.SpecG <= 0 {
		return nil, fmt.Errorf("spec required")
	}
	repo, err := s.miniDirectShipRepository()
	if err != nil {
		return nil, err
	}
	return repo.ListCustomerCentralInventoryBatches(ctx, query)
}

func (s *Service) ListCustomerAssetInventory(ctx context.Context, query CustomerAssetInventoryQuery) ([]CustomerAssetInventory, error) {
	if query.CustomerID <= 0 {
		return nil, fmt.Errorf("customer required")
	}
	query.InventoryType = strings.TrimSpace(query.InventoryType)
	if query.InventoryType != "" && query.InventoryType != "finished_product" && query.InventoryType != "green_bean" && query.InventoryType != "packaging" && query.InventoryType != "semi_finished" {
		return nil, fmt.Errorf("inventory type invalid")
	}
	query.Q = strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(query.Q)), ""))
	repo, err := s.miniDirectShipRepository()
	if err != nil {
		return nil, err
	}
	assets, ok := repo.(CustomerAssetInventoryRepository)
	if !ok {
		return nil, fmt.Errorf("customer asset inventory unavailable")
	}
	return assets.ListCustomerAssetInventory(ctx, query)
}

func (s *Service) ListCustomerAssetInventoryLedger(ctx context.Context, query CustomerAssetInventoryLedgerQuery) ([]CustomerAssetInventoryLedgerEntry, error) {
	if query.CustomerID <= 0 || query.ItemID <= 0 {
		return nil, fmt.Errorf("customer and item required")
	}
	query.InventoryType = strings.TrimSpace(query.InventoryType)
	if query.InventoryType != "finished_product" && query.InventoryType != "green_bean" && query.InventoryType != "packaging" && query.InventoryType != "semi_finished" {
		return nil, fmt.Errorf("inventory type invalid")
	}
	if query.Limit <= 0 {
		query.Limit = 100
	} else if query.Limit > 200 {
		query.Limit = 200
	}
	repo, err := s.miniDirectShipRepository()
	if err != nil {
		return nil, err
	}
	assets, ok := repo.(CustomerAssetInventoryRepository)
	if !ok {
		return nil, fmt.Errorf("customer asset inventory unavailable")
	}
	return assets.ListCustomerAssetInventoryLedger(ctx, query)
}

func normalizeMiniDirectShipCommand(cmd MiniDirectShipCommand, requireIdempotency bool) (MiniDirectShipCommand, error) {
	if cmd.CustomerID <= 0 {
		return MiniDirectShipCommand{}, fmt.Errorf("customer required")
	}
	cmd.IdempotencyKey = strings.TrimSpace(cmd.IdempotencyKey)
	cmd.UsageCode = strings.TrimSpace(cmd.UsageCode)
	if cmd.UsageCode == "" {
		cmd.UsageCode = "direct_ship"
	}
	if cmd.UsageCode != "direct_ship" && cmd.UsageCode != "product_order" {
		return MiniDirectShipCommand{}, fmt.Errorf("invalid customer order usage")
	}
	cmd.PriceQuoteToken = strings.TrimSpace(cmd.PriceQuoteToken)
	orderDate, err := normalizeCustomerOrderDate(cmd.OrderDate, time.Now())
	if err != nil {
		return MiniDirectShipCommand{}, err
	}
	cmd.OrderDate = orderDate.Format("2006-01-02")
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
		canonicalBOMSpec := item.BomSpecID > 0 || item.BomVariantID > 0
		if canonicalBOMSpec {
			if item.BomSpecID <= 0 {
				return nil, fmt.Errorf("bom_spec_id required")
			}
			if item.SpecG > 0 {
				return nil, fmt.Errorf("canonical BOM spec must not use spec_g")
			}
		} else if item.SpecG <= 0 {
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
		item.BomSpecKey = strings.TrimSpace(item.BomSpecKey)
		item.InventoryUnit = strings.TrimSpace(item.InventoryUnit)
		item.SalesUnit = strings.TrimSpace(item.SalesUnit)
		key := fmt.Sprintf("%d:legacy:%d", item.ProductID, item.SpecG)
		if canonicalBOMSpec {
			key = fmt.Sprintf("%d:bom_spec:%d", item.ProductID, item.BomSpecID)
		}
		if idx, ok := byKey[key]; ok {
			if canonicalBOMSpec && out[idx].BomVariantID > 0 && item.BomVariantID > 0 && out[idx].BomVariantID != item.BomVariantID {
				return nil, fmt.Errorf("bom_variant_id mismatch for BOM spec")
			}
			if canonicalBOMSpec && out[idx].BomVariantID == 0 {
				out[idx].BomVariantID = item.BomVariantID
			}
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
