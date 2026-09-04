package stock

import (
	"context"
	"fmt"
	"math"
	stockdomain "orderapp/internal/domain/stock"
	"strings"
)

type LedgerQuery struct {
	Q             string
	ItemType      string
	Warehouse     string
	SourceDocType string
	SourceBatch   string
	From          string
	To            string
	Limit         int
	Offset        int
}

type LedgerRow struct {
	ID              int64  `json:"id"`
	ItemType        string `json:"item_type"`
	ItemID          int64  `json:"item_id"`
	ItemName        string `json:"item_name"`
	SpecG           int64  `json:"spec_g"`
	BomSpecID       int64  `json:"bom_spec_id,omitempty"`
	BomVariantID    int64  `json:"bom_variant_id,omitempty"`
	Warehouse       string `json:"warehouse"`
	SourceDocType   string `json:"source_doc_type"`
	SourceDocID     int64  `json:"source_doc_id"`
	SourceBatchCode string `json:"source_batch_code"`
	SourceBatchID   string `json:"source_batch_id"`
	QtyBeforeG      int64  `json:"qty_before_g"`
	QtyChangeG      int64  `json:"qty_change_g"`
	QtyAfterG       int64  `json:"qty_after_g"`
	QtyBeforeUnits  int64  `json:"qty_before_units"`
	QtyChangeUnits  int64  `json:"qty_change_units"`
	QtyAfterUnits   int64  `json:"qty_after_units"`
	Operator        string `json:"operator"`
	CreatedAt       string `json:"created_at"`
}

type LedgerResult struct {
	Rows       []LedgerRow `json:"rows"`
	HasNext    bool        `json:"has_next"`
	Total      int         `json:"total"`
	Page       int         `json:"page"`
	Limit      int         `json:"limit"`
	Offset     int         `json:"offset"`
	TotalPages int         `json:"total_pages"`
}

type BatchQuery struct {
	Q        string
	ItemType string
	Limit    int
	Offset   int
}

type BatchRow struct {
	ID             int64   `json:"id"`
	BatchCode      string  `json:"batch_code"`
	ItemType       string  `json:"item_type"`
	ItemID         int64   `json:"item_id"`
	ItemName       string  `json:"item_name"`
	SpecG          int64   `json:"spec_g"`
	BomSpecID      int64   `json:"bom_spec_id,omitempty"`
	BomVariantID   int64   `json:"bom_variant_id,omitempty"`
	SourceDocType  string  `json:"source_doc_type"`
	SourceDocID    int64   `json:"source_doc_id"`
	SourceBatchID  string  `json:"source_batch_id"`
	QtyG           int64   `json:"qty_g"`
	QtyUnits       int64   `json:"qty_units"`
	RemainingG     int64   `json:"remaining_g"`
	RemainingUnits int64   `json:"remaining_units"`
	UnitCost       float64 `json:"unit_cost"`
	QualityStatus  string  `json:"quality_status"`
	Operator       string  `json:"operator"`
	CreatedAt      string  `json:"created_at"`
}

type BatchResult struct {
	Rows       []BatchRow `json:"rows"`
	HasNext    bool       `json:"has_next"`
	Total      int        `json:"total"`
	Page       int        `json:"page"`
	Limit      int        `json:"limit"`
	Offset     int        `json:"offset"`
	TotalPages int        `json:"total_pages"`
}

type MaterialBatchQuery struct {
	Q          string
	MaterialID int64
	ActiveOnly bool
	Limit      int
	Offset     int
}

type MaterialBatchRow struct {
	ID                        int64   `json:"id"`
	BatchCode                 string  `json:"batch_code"`
	MaterialID                int64   `json:"material_id"`
	MaterialName              string  `json:"material_name"`
	Supplier                  string  `json:"supplier"`
	ReceiptID                 int64   `json:"receipt_id"`
	QtyG                      int64   `json:"qty_g"`
	QtyUnits                  int64   `json:"qty_units"`
	RemainingG                int64   `json:"remaining_g"`
	RemainingUnits            int64   `json:"remaining_units"`
	UnitCost                  float64 `json:"unit_cost"`
	CropSeason                string  `json:"crop_season"`
	Origin                    string  `json:"origin"`
	ProducerFlavorDescription string  `json:"producer_flavor_description"`
	ReceivedAt                string  `json:"received_at"`
	Status                    string  `json:"status"`
	QualityStatus             string  `json:"quality_status"`
	Note                      string  `json:"note"`
}

type MaterialBatchResult struct {
	Rows       []MaterialBatchRow `json:"rows"`
	HasNext    bool               `json:"has_next"`
	Total      int                `json:"total"`
	Page       int                `json:"page"`
	Limit      int                `json:"limit"`
	Offset     int                `json:"offset"`
	TotalPages int                `json:"total_pages"`
}

type WarehouseRow struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	Kind          string `json:"kind"`
	ParentCode    string `json:"parent_code"`
	SortOrder     int    `json:"sort_order"`
	IsDefault     bool   `json:"is_default"`
	Active        bool   `json:"active"`
	Description   string `json:"description"`
	CustomerID    int64  `json:"customer_id"`
	CustomerName  string `json:"customer_name"`
	GroupID       int64  `json:"group_id"`
	GroupName     string `json:"group_name"`
	GroupItemID   int64  `json:"group_item_id"`
	GroupItemName string `json:"group_item_name"`
	GroupSource   string `json:"group_source"`
}

type WarehouseListQuery struct {
	CustomerID int64
}

type MaterialBatchLocationQuery struct {
	Q          string
	MaterialID int64
	Warehouse  string
	ActiveOnly bool
	Limit      int
	Offset     int
}

type MaterialBatchLocationRow struct {
	MaterialBatchID int64  `json:"material_batch_id"`
	BatchCode       string `json:"batch_code"`
	MaterialID      int64  `json:"material_id"`
	MaterialName    string `json:"material_name"`
	Warehouse       string `json:"warehouse"`
	WarehouseName   string `json:"warehouse_name"`
	QtyG            int64  `json:"qty_g"`
	QtyUnits        int64  `json:"qty_units"`
	QualityStatus   string `json:"quality_status"`
	ReceivedAt      string `json:"received_at"`
	UpdatedAt       string `json:"updated_at"`
}

type MaterialBatchLocationResult struct {
	Rows       []MaterialBatchLocationRow `json:"rows"`
	HasNext    bool                       `json:"has_next"`
	Total      int                        `json:"total"`
	Page       int                        `json:"page"`
	Limit      int                        `json:"limit"`
	Offset     int                        `json:"offset"`
	TotalPages int                        `json:"total_pages"`
}

type MaterialBalanceQuery struct {
	Warehouse   string
	MaterialIDs []int64
}

type MaterialBalanceRow struct {
	MaterialID     int64   `json:"material_id"`
	MaterialName   string  `json:"material_name"`
	Warehouse      string  `json:"warehouse"`
	WarehouseName  string  `json:"warehouse_name"`
	UnitCode       string  `json:"unit_code"`
	BookQty        float64 `json:"book_qty"`
	AvailableQty   float64 `json:"available_qty"`
	FrozenQty      float64 `json:"frozen_qty"`
	BookG          int64   `json:"book_g"`
	AvailableG     int64   `json:"available_g"`
	FrozenG        int64   `json:"frozen_g"`
	BookUnits      int64   `json:"book_units"`
	AvailableUnits int64   `json:"available_units"`
	FrozenUnits    int64   `json:"frozen_units"`
}

type MaterialBalanceRepository interface {
	ListMaterialBalances(ctx context.Context, query MaterialBalanceQuery) ([]MaterialBalanceRow, error)
}

type WarehouseInventoryQuery struct {
	Q           string
	Warehouse   string
	ItemType    string
	CustomerID  int64
	GroupID     int64
	GroupItemID int64
	Limit       int
	Offset      int
}

type WarehouseInventoryRow struct {
	Warehouse     string  `json:"warehouse"`
	WarehouseName string  `json:"warehouse_name"`
	WarehouseKind string  `json:"warehouse_kind"`
	GroupID       int64   `json:"group_id"`
	GroupName     string  `json:"group_name"`
	GroupItemID   int64   `json:"group_item_id"`
	GroupItemName string  `json:"group_item_name"`
	GroupSource   string  `json:"group_source"`
	ItemType      string  `json:"item_type"`
	ItemID        int64   `json:"item_id"`
	ItemName      string  `json:"item_name"`
	SpecG         int64   `json:"spec_g"`
	BomSpecID     int64   `json:"bom_spec_id,omitempty"`
	BomVariantID  int64   `json:"bom_variant_id,omitempty"`
	BomSpecName   string  `json:"bom_spec_name,omitempty"`
	InventoryUnit string  `json:"inventory_unit,omitempty"`
	BatchID       int64   `json:"batch_id"`
	BatchCode     string  `json:"batch_code"`
	QtyG          int64   `json:"qty_g"`
	QtyUnits      int64   `json:"qty_units"`
	UnitCost      float64 `json:"unit_cost"`
	QualityStatus string  `json:"quality_status"`
	UpdatedAt     string  `json:"updated_at"`
}

type WarehouseInventoryResult struct {
	Rows       []WarehouseInventoryRow `json:"rows"`
	HasNext    bool                    `json:"has_next"`
	Total      int                     `json:"total"`
	Page       int                     `json:"page"`
	Limit      int                     `json:"limit"`
	Offset     int                     `json:"offset"`
	TotalPages int                     `json:"total_pages"`
}

type OutboundLogQuery struct {
	Q      string
	From   string
	To     string
	Limit  int
	Offset int
}

type OutboundLogRow struct {
	DocumentID      int64  `json:"document_id"`
	OrderID         int64  `json:"order_id"`
	OrderNo         string `json:"order_no"`
	CustomerName    string `json:"customer_name"`
	PostingDate     string `json:"posting_date"`
	SourceWarehouse string `json:"source_warehouse"`
	WarehouseName   string `json:"warehouse_name"`
	DeliveryMethod  string `json:"delivery_method"`
	TrackingNo      string `json:"tracking_no"`
	VersionNo       int    `json:"version_no"`
	IsLatest        bool   `json:"is_latest"`
	CreatedAt       string `json:"created_at"`
	CreatedBy       string `json:"created_by"`
	PayStatus       string `json:"pay_status"`
	ShipStatus      string `json:"ship_status"`
	ProcessStatus   string `json:"process_status"`
	InvoiceStatus   string `json:"invoice_status"`
	DownloadURL     string `json:"download_url"`
	LatestURL       string `json:"latest_url"`
}

type OutboundLogResult struct {
	Rows       []OutboundLogRow `json:"rows"`
	HasNext    bool             `json:"has_next"`
	Total      int              `json:"total"`
	Page       int              `json:"page"`
	Limit      int              `json:"limit"`
	Offset     int              `json:"offset"`
	TotalPages int              `json:"total_pages"`
}

type StockTraceQuery struct {
	BatchCode string
}

type TraceFinishedBatch struct {
	BatchCode      string `json:"batch_code"`
	ProductID      int64  `json:"product_id"`
	ProductName    string `json:"product_name"`
	SpecG          int64  `json:"spec_g"`
	BomSpecID      int64  `json:"bom_spec_id,omitempty"`
	BomVariantID   int64  `json:"bom_variant_id,omitempty"`
	Warehouse      string `json:"warehouse"`
	QtyG           int64  `json:"qty_g"`
	QtyUnits       int64  `json:"qty_units"`
	RemainingG     int64  `json:"remaining_g"`
	RemainingUnits int64  `json:"remaining_units"`
	QualityStatus  string `json:"quality_status"`
	CreatedAt      string `json:"created_at"`
}

type TraceProduction struct {
	RunningItemID   int64   `json:"running_item_id"`
	WorkOrderNo     string  `json:"work_order_no"`
	BatchID         string  `json:"batch_id"`
	OrderNos        string  `json:"order_nos"`
	InputG          int64   `json:"input_g"`
	FinishedTotalG  int64   `json:"finished_total_g"`
	ActualYieldRate float64 `json:"actual_yield_rate"`
	StartedBy       string  `json:"started_by"`
	FinishedBy      string  `json:"finished_by"`
	FinishedAt      string  `json:"finished_at"`
}

type TraceMaterial struct {
	MaterialID        int64  `json:"material_id"`
	MaterialName      string `json:"material_name"`
	Unit              string `json:"unit"`
	DeductG           int64  `json:"deduct_g"`
	DeductUnits       int64  `json:"deduct_units"`
	MaterialBatchID   int64  `json:"material_batch_id"`
	MaterialBatchCode string `json:"material_batch_code"`
}

type TraceMaterialBatch struct {
	ID                        int64   `json:"id"`
	BatchCode                 string  `json:"batch_code"`
	MaterialID                int64   `json:"material_id"`
	MaterialName              string  `json:"material_name"`
	Supplier                  string  `json:"supplier"`
	ReceiptID                 int64   `json:"receipt_id"`
	QtyG                      int64   `json:"qty_g"`
	QtyUnits                  int64   `json:"qty_units"`
	RemainingG                int64   `json:"remaining_g"`
	RemainingUnits            int64   `json:"remaining_units"`
	UnitCost                  float64 `json:"unit_cost"`
	CropSeason                string  `json:"crop_season"`
	Origin                    string  `json:"origin"`
	ProducerFlavorDescription string  `json:"producer_flavor_description"`
	ReceivedAt                string  `json:"received_at"`
	Status                    string  `json:"status"`
	QualityStatus             string  `json:"quality_status"`
	Note                      string  `json:"note"`
}

type StockTraceResult struct {
	TraceType         string                     `json:"trace_type"`
	FinishedBatch     TraceFinishedBatch         `json:"finished_batch"`
	Production        TraceProduction            `json:"production"`
	Materials         []TraceMaterial            `json:"materials"`
	MaterialBatch     TraceMaterialBatch         `json:"material_batch"`
	MaterialLocations []MaterialBatchLocationRow `json:"material_locations,omitempty"`
}

type MaterialTransferCommand struct {
	MaterialID     int64
	FromWarehouse  string
	ToWarehouse    string
	QtyG           int64
	Note           string
	Operator       string
	IdempotencyKey string
}

type MaterialTransferAllocation struct {
	MaterialBatchID int64  `json:"material_batch_id"`
	BatchCode       string `json:"batch_code"`
	QtyG            int64  `json:"qty_g"`
}

type MaterialTransferResult struct {
	TransferID  int64                        `json:"transfer_id"`
	TransferNo  string                       `json:"transfer_no"`
	EntryID     int64                        `json:"entry_id,omitempty"`
	EntryNo     string                       `json:"entry_no,omitempty"`
	Allocations []MaterialTransferAllocation `json:"allocations"`
}

type FinishedProductTransferCommand struct {
	ProductID      int64
	SpecG          int64
	BomSpecID      int64
	BomVariantID   int64
	UnitCode       string
	FromWarehouse  string
	ToWarehouse    string
	QtyUnits       int64
	QtyLooseG      int64
	Note           string
	Operator       string
	IdempotencyKey string
}

type FinishedProductTransferResult struct {
	TransferID   int64  `json:"transfer_id"`
	TransferNo   string `json:"transfer_no"`
	EntryID      int64  `json:"entry_id,omitempty"`
	EntryNo      string `json:"entry_no,omitempty"`
	ProductID    int64  `json:"product_id"`
	SpecG        int64  `json:"spec_g,omitempty"`
	BomSpecID    int64  `json:"bom_spec_id,omitempty"`
	BomVariantID int64  `json:"bom_variant_id,omitempty"`
}

type MaterialReceiptCommand struct {
	MaterialID                int64
	Supplier                  string
	Qty                       float64
	UnitCode                  string
	QtyG                      int64
	QtyUnits                  int64
	UnitCost                  float64
	CropSeason                string
	Origin                    string
	ProducerFlavorDescription string
	Note                      string
	Operator                  string
	TargetWarehouse           string
}

type MaterialReceiptResult struct {
	ReceiptID int64  `json:"receipt_id"`
	BatchID   int64  `json:"batch_id"`
	BatchCode string `json:"batch_code"`
	EntryID   int64  `json:"entry_id,omitempty"`
	EntryNo   string `json:"entry_no,omitempty"`
}

const (
	itemTypeMaterial                      = "material"
	itemTypeFinishedProduct               = "finished_product"
	PurposeMaterialReceipt                = "material_receipt"
	PurposeCustomerReceipt                = "customer_receipt"
	PurposeMaterialIssue                  = "material_issue"
	PurposeMaterialTransfer               = "material_transfer"
	PurposeMaterialTransferForManufacture = "material_transfer_for_manufacture"
	PurposeMaterialConsumption            = "material_consumption_for_manufacture"
	PurposeManufacture                    = "manufacture"
	legacyPurposeMaterialReturn           = "material_return_from_manufacture"
)

type StockDocumentItemCommand struct {
	MaterialID                int64   `json:"material_id"`
	ProductID                 int64   `json:"product_id"`
	ItemType                  string  `json:"item_type"`
	ItemName                  string  `json:"item_name"`
	SpecG                     int64   `json:"spec_g"`
	BomSpecID                 int64   `json:"bom_spec_id,omitempty"`
	BomVariantID              int64   `json:"bom_variant_id,omitempty"`
	InventoryUnit             string  `json:"inventory_unit"`
	FromWarehouse             string  `json:"from_warehouse"`
	ToWarehouse               string  `json:"to_warehouse"`
	QtyG                      int64   `json:"qty_g"`
	QtyUnits                  int64   `json:"qty_units"`
	BatchCode                 string  `json:"batch_code"`
	UnitCost                  float64 `json:"unit_cost"`
	Supplier                  string  `json:"supplier"`
	OwnerCustomerID           int64   `json:"owner_customer_id,omitempty"`
	CropSeason                string  `json:"crop_season"`
	Origin                    string  `json:"origin"`
	ProducerFlavorDescription string  `json:"producer_flavor_description"`
}

type StockDocumentCommand struct {
	EntryType      string                     `json:"entry_type"`
	Purpose        string                     `json:"purpose"`
	IsReturn       bool                       `json:"is_return"`
	WorkOrderID    int64                      `json:"work_order_id"`
	JobCardID      int64                      `json:"job_card_id"`
	RunningItemID  int64                      `json:"running_item_id"`
	SourceType     string                     `json:"source_type"`
	SourceID       int64                      `json:"source_id"`
	ReturnSource   string                     `json:"return_source"`
	Operator       string                     `json:"operator"`
	CustomerID     int64                      `json:"customer_id,omitempty"`
	Note           string                     `json:"note"`
	IdempotencyKey string                     `json:"idempotency_key"`
	Items          []StockDocumentItemCommand `json:"items"`
}

type StockDocumentQuery struct {
	Q           string
	Purpose     string
	Status      string
	WorkOrderID int64
	JobCardID   int64
	Limit       int
	Offset      int
}

type StockDocumentBatchAllocation struct {
	MaterialBatchID int64   `json:"material_batch_id"`
	BatchCode       string  `json:"batch_code"`
	QtyG            int64   `json:"qty_g"`
	QtyUnits        int64   `json:"qty_units"`
	UnitCost        float64 `json:"unit_cost"`
}

type StockDocumentItemRow struct {
	ID                        int64                          `json:"id"`
	StockEntryID              int64                          `json:"stock_entry_id"`
	MaterialID                int64                          `json:"material_id"`
	ProductID                 int64                          `json:"product_id"`
	ItemType                  string                         `json:"item_type"`
	ItemName                  string                         `json:"item_name"`
	OwnerCustomerID           int64                          `json:"owner_customer_id,omitempty"`
	SpecG                     int64                          `json:"spec_g"`
	BomSpecID                 int64                          `json:"bom_spec_id,omitempty"`
	BomVariantID              int64                          `json:"bom_variant_id,omitempty"`
	InventoryUnit             string                         `json:"inventory_unit"`
	FromWarehouse             string                         `json:"from_warehouse"`
	ToWarehouse               string                         `json:"to_warehouse"`
	QtyG                      int64                          `json:"qty_g"`
	QtyUnits                  int64                          `json:"qty_units"`
	BatchCode                 string                         `json:"batch_code"`
	UnitCost                  float64                        `json:"unit_cost"`
	TotalCost                 float64                        `json:"total_cost"`
	Supplier                  string                         `json:"supplier"`
	CropSeason                string                         `json:"crop_season"`
	Origin                    string                         `json:"origin"`
	ProducerFlavorDescription string                         `json:"producer_flavor_description"`
	Allocations               []StockDocumentBatchAllocation `json:"allocations"`
}

type StockDocumentRow struct {
	ID            int64   `json:"id"`
	EntryNo       string  `json:"entry_no"`
	EntryType     string  `json:"entry_type"`
	Purpose       string  `json:"purpose"`
	IsReturn      bool    `json:"is_return"`
	Status        string  `json:"status"`
	WorkOrderID   int64   `json:"work_order_id"`
	WorkOrderNo   string  `json:"work_order_no"`
	JobCardID     int64   `json:"job_card_id"`
	RunningItemID int64   `json:"running_item_id"`
	SourceType    string  `json:"source_type"`
	SourceID      int64   `json:"source_id"`
	ReturnSource  string  `json:"return_source"`
	ItemCount     int64   `json:"item_count"`
	TotalQtyG     int64   `json:"total_qty_g"`
	TotalQtyUnits int64   `json:"total_qty_units"`
	TotalCost     float64 `json:"total_cost"`
	Operator      string  `json:"operator"`
	Note          string  `json:"note"`
	Legacy        bool    `json:"legacy"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

type StockDocumentDetail struct {
	StockDocumentRow
	Items []StockDocumentItemRow `json:"items"`
}

type StockDocumentResult struct {
	Rows       []StockDocumentRow `json:"rows"`
	HasNext    bool               `json:"has_next"`
	Total      int                `json:"total"`
	Page       int                `json:"page"`
	Limit      int                `json:"limit"`
	Offset     int                `json:"offset"`
	TotalPages int                `json:"total_pages"`
}

type StockAdjustmentCommand struct {
	AdjustmentType  string
	ItemType        string
	ItemID          int64
	SpecG           int64
	BomSpecID       int64
	BomVariantID    int64
	Warehouse       string
	TargetG         int64
	TargetUnits     int64
	TargetQty       float64
	HasTargetQty    bool
	UnitCode        string
	MaterialBatchID int64
	TargetUnitCost  float64
	Reason          string
	Operator        string
}

type StockAdjustmentResult struct {
	AdjustmentID int64 `json:"adjustment_id"`
	ProductID    int64 `json:"product_id,omitempty"`
	SpecG        int64 `json:"spec_g,omitempty"`
	BomSpecID    int64 `json:"bom_spec_id,omitempty"`
	BomVariantID int64 `json:"bom_variant_id,omitempty"`
}

type BindWarehouseCustomerCommand struct {
	WarehouseCode string
	CustomerID    int64
	Actor         string
}

type EnsureCustomerWarehouseCommand struct {
	CustomerID int64
	Kind       string
	Actor      string
}

type Repository interface {
	ListLedger(ctx context.Context, query LedgerQuery) (LedgerResult, error)
	ListBatches(ctx context.Context, query BatchQuery) (BatchResult, error)
	ListMaterialBatches(ctx context.Context, query MaterialBatchQuery) (MaterialBatchResult, error)
	ListWarehouses(ctx context.Context, query WarehouseListQuery) ([]WarehouseRow, error)
	ListMaterialBatchLocations(ctx context.Context, query MaterialBatchLocationQuery) (MaterialBatchLocationResult, error)
	ListWarehouseInventory(ctx context.Context, query WarehouseInventoryQuery) (WarehouseInventoryResult, error)
	ListOutboundLogs(ctx context.Context, query OutboundLogQuery) (OutboundLogResult, error)
	GetStockTrace(ctx context.Context, query StockTraceQuery) (StockTraceResult, error)
	ReceiveMaterial(ctx context.Context, cmd MaterialReceiptCommand) (MaterialReceiptResult, error)
	CreateAdjustment(ctx context.Context, cmd StockAdjustmentCommand) (StockAdjustmentResult, error)
	TransferMaterial(ctx context.Context, cmd MaterialTransferCommand) (MaterialTransferResult, error)
	TransferFinishedProduct(ctx context.Context, cmd FinishedProductTransferCommand) (FinishedProductTransferResult, error)
	BindWarehouseCustomer(ctx context.Context, cmd BindWarehouseCustomerCommand) (WarehouseRow, error)
}

type CustomerWarehouseCreator interface {
	EnsureCustomerWarehouse(ctx context.Context, cmd EnsureCustomerWarehouseCommand) (WarehouseRow, error)
}

type StockDocumentRepository interface {
	CreateStockDocumentDraft(ctx context.Context, cmd StockDocumentCommand) (StockDocumentDetail, error)
	UpdateStockDocumentDraft(ctx context.Context, id int64, cmd StockDocumentCommand) (StockDocumentDetail, error)
	SubmitStockDocument(ctx context.Context, id int64, actor string) (StockDocumentDetail, error)
	CancelStockDocument(ctx context.Context, id int64, actor string) (StockDocumentDetail, error)
	ListStockDocuments(ctx context.Context, query StockDocumentQuery) (StockDocumentResult, error)
	GetStockDocument(ctx context.Context, id int64) (StockDocumentDetail, error)
	CreateAndSubmitStockDocument(ctx context.Context, cmd StockDocumentCommand) (StockDocumentDetail, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) stockDocumentRepo() (StockDocumentRepository, error) {
	repo, ok := s.repo.(StockDocumentRepository)
	if !ok {
		return nil, fmt.Errorf("stock document repository unavailable")
	}
	return repo, nil
}

func (s *Service) CreateStockDocumentDraft(ctx context.Context, cmd StockDocumentCommand) (StockDocumentDetail, error) {
	repo, err := s.stockDocumentRepo()
	if err != nil {
		return StockDocumentDetail{}, err
	}
	cmd, err = normalizeStockDocumentCommand(cmd)
	if err != nil {
		return StockDocumentDetail{}, err
	}
	return repo.CreateStockDocumentDraft(ctx, cmd)
}

func (s *Service) UpdateStockDocumentDraft(ctx context.Context, id int64, cmd StockDocumentCommand) (StockDocumentDetail, error) {
	if id <= 0 {
		return StockDocumentDetail{}, fmt.Errorf("stock_document_id required")
	}
	repo, err := s.stockDocumentRepo()
	if err != nil {
		return StockDocumentDetail{}, err
	}
	cmd, err = normalizeStockDocumentCommand(cmd)
	if err != nil {
		return StockDocumentDetail{}, err
	}
	return repo.UpdateStockDocumentDraft(ctx, id, cmd)
}

func (s *Service) SubmitStockDocument(ctx context.Context, id int64, actor string) (StockDocumentDetail, error) {
	if id <= 0 {
		return StockDocumentDetail{}, fmt.Errorf("stock_document_id required")
	}
	repo, err := s.stockDocumentRepo()
	if err != nil {
		return StockDocumentDetail{}, err
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "stock"
	}
	return repo.SubmitStockDocument(ctx, id, actor)
}

func (s *Service) CancelStockDocument(ctx context.Context, id int64, actor string) (StockDocumentDetail, error) {
	if id <= 0 {
		return StockDocumentDetail{}, fmt.Errorf("stock_document_id required")
	}
	repo, err := s.stockDocumentRepo()
	if err != nil {
		return StockDocumentDetail{}, err
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "stock"
	}
	return repo.CancelStockDocument(ctx, id, actor)
}

func (s *Service) ListStockDocuments(ctx context.Context, query StockDocumentQuery) (StockDocumentResult, error) {
	repo, err := s.stockDocumentRepo()
	if err != nil {
		return StockDocumentResult{}, err
	}
	query.Q = strings.TrimSpace(query.Q)
	query.Purpose = strings.TrimSpace(query.Purpose)
	query.Status = strings.TrimSpace(query.Status)
	query.Limit, query.Offset = normalizePage(query.Limit, query.Offset, 100, 500)
	return repo.ListStockDocuments(ctx, query)
}

func (s *Service) GetStockDocument(ctx context.Context, id int64) (StockDocumentDetail, error) {
	if id <= 0 {
		return StockDocumentDetail{}, fmt.Errorf("stock_document_id required")
	}
	repo, err := s.stockDocumentRepo()
	if err != nil {
		return StockDocumentDetail{}, err
	}
	return repo.GetStockDocument(ctx, id)
}

func (s *Service) CreateAndSubmitStockDocument(ctx context.Context, cmd StockDocumentCommand) (StockDocumentDetail, error) {
	repo, err := s.stockDocumentRepo()
	if err != nil {
		return StockDocumentDetail{}, err
	}
	cmd, err = normalizeStockDocumentCommand(cmd)
	if err != nil {
		return StockDocumentDetail{}, err
	}
	return repo.CreateAndSubmitStockDocument(ctx, cmd)
}

func normalizeStockDocumentCommand(cmd StockDocumentCommand) (StockDocumentCommand, error) {
	cmd.Purpose = strings.TrimSpace(cmd.Purpose)
	cmd.EntryType = strings.TrimSpace(cmd.EntryType)
	if cmd.Purpose == "" {
		cmd.Purpose = purposeForLegacyEntryType(cmd.EntryType)
	} else {
		cmd.Purpose = purposeForLegacyEntryType(cmd.Purpose)
	}
	if cmd.Purpose == legacyPurposeMaterialReturn {
		cmd.Purpose = PurposeMaterialTransferForManufacture
		cmd.IsReturn = true
	}
	switch cmd.Purpose {
	case PurposeMaterialReceipt, PurposeCustomerReceipt, PurposeMaterialIssue, PurposeMaterialTransfer,
		PurposeMaterialTransferForManufacture, PurposeMaterialConsumption, PurposeManufacture:
	default:
		return StockDocumentCommand{}, fmt.Errorf("invalid stock document purpose")
	}
	if cmd.Purpose == PurposeCustomerReceipt && cmd.CustomerID <= 0 {
		return StockDocumentCommand{}, fmt.Errorf("customer_id required for customer receipt")
	}
	if (cmd.Purpose == PurposeMaterialTransferForManufacture || cmd.Purpose == PurposeMaterialConsumption || cmd.Purpose == PurposeManufacture) && cmd.WorkOrderID <= 0 {
		return StockDocumentCommand{}, fmt.Errorf("work_order_id required")
	}
	if cmd.WorkOrderID > 0 &&
		cmd.Purpose != PurposeMaterialTransferForManufacture &&
		cmd.Purpose != PurposeMaterialConsumption &&
		cmd.Purpose != PurposeManufacture &&
		cmd.Purpose != PurposeMaterialIssue {
		return StockDocumentCommand{}, fmt.Errorf("stock document purpose cannot be linked to work order")
	}
	cmd.EntryType = entryTypeForPurpose(cmd.Purpose, cmd.IsReturn)
	cmd.SourceType = strings.TrimSpace(cmd.SourceType)
	cmd.ReturnSource = strings.TrimSpace(cmd.ReturnSource)
	cmd.Note = strings.TrimSpace(cmd.Note)
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	cmd.IdempotencyKey = strings.TrimSpace(cmd.IdempotencyKey)
	if cmd.Operator == "" {
		cmd.Operator = "stock"
	}
	if len(cmd.Items) == 0 {
		return StockDocumentCommand{}, fmt.Errorf("stock document items required")
	}
	for i := range cmd.Items {
		item := &cmd.Items[i]
		item.ItemType = normalizeStockItemType(item.ItemType)
		if item.ItemType == "" {
			if item.MaterialID > 0 {
				item.ItemType = itemTypeMaterial
			} else if item.ProductID > 0 {
				item.ItemType = itemTypeFinishedProduct
			}
		}
		if item.ItemType == itemTypeMaterial && item.MaterialID <= 0 {
			return StockDocumentCommand{}, fmt.Errorf("item %d material_id required", i+1)
		}
		if item.ItemType == itemTypeFinishedProduct && item.ProductID <= 0 {
			return StockDocumentCommand{}, fmt.Errorf("item %d product_id required", i+1)
		}
		if item.ItemType == itemTypeFinishedProduct {
			if item.BomSpecID > 0 {
				if item.SpecG != 0 {
					return StockDocumentCommand{}, fmt.Errorf("item %d spec_g must be zero for BOM specification", i+1)
				}
			} else if item.SpecG <= 0 {
				return StockDocumentCommand{}, fmt.Errorf("item %d spec_g or bom_spec_id required", i+1)
			}
		}
		if item.ItemType != itemTypeMaterial && item.ItemType != itemTypeFinishedProduct {
			return StockDocumentCommand{}, fmt.Errorf("item %d invalid item_type", i+1)
		}
		if item.QtyG < 0 || item.QtyUnits < 0 {
			return StockDocumentCommand{}, fmt.Errorf("第 %d 行数量不能为负数", i+1)
		}
		if item.QtyG == 0 && item.QtyUnits == 0 {
			return StockDocumentCommand{}, fmt.Errorf("第 %d 行数量必须大于 0", i+1)
		}
		if item.UnitCost < 0 {
			return StockDocumentCommand{}, fmt.Errorf("item %d unit_cost must be >= 0", i+1)
		}
		item.ItemName = strings.TrimSpace(item.ItemName)
		item.InventoryUnit = strings.TrimSpace(item.InventoryUnit)
		item.BatchCode = strings.TrimSpace(item.BatchCode)
		item.Supplier = strings.TrimSpace(item.Supplier)
		item.CropSeason = strings.TrimSpace(item.CropSeason)
		item.Origin = strings.TrimSpace(item.Origin)
		item.ProducerFlavorDescription = strings.TrimSpace(item.ProducerFlavorDescription)
		item.FromWarehouse = normalizeWarehouse(item.FromWarehouse)
		item.ToWarehouse = normalizeWarehouse(item.ToWarehouse)
		if cmd.Purpose == PurposeCustomerReceipt {
			item.OwnerCustomerID = cmd.CustomerID
		}
		normalizeStockDocumentWarehouses(cmd.Purpose, cmd.IsReturn, item)
		if item.FromWarehouse != "" && item.FromWarehouse == item.ToWarehouse {
			return StockDocumentCommand{}, fmt.Errorf("item %d from/to warehouse must differ", i+1)
		}
	}
	return cmd, nil
}

func normalizeStockDocumentWarehouses(purpose string, isReturn bool, item *StockDocumentItemCommand) {
	switch purpose {
	case PurposeMaterialReceipt, PurposeCustomerReceipt:
		item.FromWarehouse = ""
		if item.ToWarehouse == "" {
			item.ToWarehouse = stockdomain.WarehouseRawMaterials
		}
	case PurposeMaterialIssue:
		if item.FromWarehouse == "" {
			item.FromWarehouse = stockdomain.WarehouseRawMaterials
		}
		item.ToWarehouse = ""
	case PurposeMaterialTransferForManufacture:
		if isReturn {
			item.FromWarehouse = stockdomain.WarehouseWIP
			item.ToWarehouse = stockdomain.WarehouseRawMaterials
		} else {
			item.FromWarehouse = stockdomain.WarehouseRawMaterials
			item.ToWarehouse = stockdomain.WarehouseWIP
		}
	case PurposeMaterialConsumption:
		if item.FromWarehouse == "" {
			item.FromWarehouse = stockdomain.WarehouseWIP
		}
		item.ToWarehouse = ""
	case PurposeManufacture:
		item.FromWarehouse = ""
		if item.ToWarehouse == "" {
			item.ToWarehouse = stockdomain.WarehouseFinishedGoods
		}
	}
}

func purposeForLegacyEntryType(entryType string) string {
	switch strings.TrimSpace(entryType) {
	case "material_issue_to_wip":
		return PurposeMaterialTransferForManufacture
	case "wip_return":
		return legacyPurposeMaterialReturn
	case "material_consume":
		return PurposeMaterialConsumption
	case "finished_receipt":
		return PurposeManufacture
	case "finished_transfer":
		return PurposeMaterialTransfer
	case "scrap_loss":
		return PurposeMaterialIssue
	default:
		return strings.TrimSpace(entryType)
	}
}

func entryTypeForPurpose(purpose string, isReturn bool) string {
	switch purpose {
	case PurposeMaterialTransferForManufacture:
		if isReturn {
			return "wip_return"
		}
		return "material_issue_to_wip"
	case PurposeMaterialConsumption:
		return "material_consume"
	case PurposeManufacture:
		return "finished_receipt"
	default:
		return purpose
	}
}

func (s *Service) ListLedger(ctx context.Context, query LedgerQuery) (LedgerResult, error) {
	query.Q = strings.TrimSpace(query.Q)
	query.ItemType = strings.TrimSpace(query.ItemType)
	query.Warehouse = strings.TrimSpace(query.Warehouse)
	query.SourceDocType = strings.TrimSpace(query.SourceDocType)
	query.SourceBatch = strings.TrimSpace(query.SourceBatch)
	query.From = strings.TrimSpace(query.From)
	query.To = strings.TrimSpace(query.To)
	query.Limit, query.Offset = normalizePage(query.Limit, query.Offset, 100, 500)
	return s.repo.ListLedger(ctx, query)
}

func (s *Service) ListBatches(ctx context.Context, query BatchQuery) (BatchResult, error) {
	query.Q = strings.TrimSpace(query.Q)
	query.ItemType = strings.TrimSpace(query.ItemType)
	query.Limit, query.Offset = normalizePage(query.Limit, query.Offset, 100, 500)
	return s.repo.ListBatches(ctx, query)
}

func (s *Service) ListMaterialBatches(ctx context.Context, query MaterialBatchQuery) (MaterialBatchResult, error) {
	query.Q = strings.TrimSpace(query.Q)
	query.Limit, query.Offset = normalizePage(query.Limit, query.Offset, 100, 500)
	return s.repo.ListMaterialBatches(ctx, query)
}

func (s *Service) ListWarehouses(ctx context.Context, query WarehouseListQuery) ([]WarehouseRow, error) {
	if query.CustomerID < 0 {
		query.CustomerID = 0
	}
	return s.repo.ListWarehouses(ctx, query)
}

func (s *Service) ListMaterialBatchLocations(ctx context.Context, query MaterialBatchLocationQuery) (MaterialBatchLocationResult, error) {
	query.Q = strings.TrimSpace(query.Q)
	query.Warehouse = normalizeWarehouse(query.Warehouse)
	query.Limit, query.Offset = normalizePage(query.Limit, query.Offset, 100, 500)
	return s.repo.ListMaterialBatchLocations(ctx, query)
}

func (s *Service) ListWarehouseInventory(ctx context.Context, query WarehouseInventoryQuery) (WarehouseInventoryResult, error) {
	query.Q = strings.TrimSpace(query.Q)
	query.Warehouse = normalizeWarehouse(query.Warehouse)
	query.ItemType = strings.TrimSpace(query.ItemType)
	if query.CustomerID < 0 {
		query.CustomerID = 0
	}
	if query.GroupID < 0 {
		query.GroupID = 0
	}
	if query.GroupItemID < 0 {
		query.GroupItemID = 0
	}
	query.Limit, query.Offset = normalizePage(query.Limit, query.Offset, 100, 500)
	return s.repo.ListWarehouseInventory(ctx, query)
}

func (s *Service) ListMaterialBalances(ctx context.Context, query MaterialBalanceQuery) ([]MaterialBalanceRow, error) {
	query.Warehouse = normalizeWarehouse(query.Warehouse)
	if query.Warehouse == "" {
		return nil, fmt.Errorf("warehouse required")
	}
	ids := make([]int64, 0, len(query.MaterialIDs))
	seen := map[int64]bool{}
	for _, id := range query.MaterialIDs {
		if id > 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return []MaterialBalanceRow{}, nil
	}
	repo, ok := s.repo.(MaterialBalanceRepository)
	if !ok {
		return nil, fmt.Errorf("material balance repository unavailable")
	}
	rows, err := repo.ListMaterialBalances(ctx, MaterialBalanceQuery{Warehouse: query.Warehouse, MaterialIDs: ids})
	if err != nil {
		return nil, err
	}
	for index := range rows {
		factor := 0.0
		if isReceiptWeightUnit(rows[index].UnitCode) {
			factor = receiptWeightQtyToGrams(1, rows[index].UnitCode)
		}
		if factor > 0 {
			rows[index].BookQty = float64(rows[index].BookG) / factor
			rows[index].AvailableQty = float64(rows[index].AvailableG) / factor
			rows[index].FrozenQty = float64(rows[index].FrozenG) / factor
		} else {
			rows[index].BookQty = float64(rows[index].BookUnits)
			rows[index].AvailableQty = float64(rows[index].AvailableUnits)
			rows[index].FrozenQty = float64(rows[index].FrozenUnits)
		}
	}
	return rows, nil
}

func (s *Service) ListOutboundLogs(ctx context.Context, query OutboundLogQuery) (OutboundLogResult, error) {
	query.Q = strings.TrimSpace(query.Q)
	query.From = strings.TrimSpace(query.From)
	query.To = strings.TrimSpace(query.To)
	query.Limit, query.Offset = normalizePage(query.Limit, query.Offset, 100, 500)
	return s.repo.ListOutboundLogs(ctx, query)
}

func (s *Service) GetStockTrace(ctx context.Context, query StockTraceQuery) (StockTraceResult, error) {
	query.BatchCode = strings.TrimSpace(query.BatchCode)
	if query.BatchCode == "" {
		return StockTraceResult{}, fmt.Errorf("batch required")
	}
	return s.repo.GetStockTrace(ctx, query)
}

func (s *Service) BindWarehouseCustomer(ctx context.Context, cmd BindWarehouseCustomerCommand) (WarehouseRow, error) {
	cmd.WarehouseCode = normalizeWarehouse(cmd.WarehouseCode)
	if cmd.WarehouseCode == "" {
		return WarehouseRow{}, fmt.Errorf("warehouse required")
	}
	if cmd.CustomerID < 0 {
		return WarehouseRow{}, fmt.Errorf("customer_id must be >= 0")
	}
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.Actor == "" {
		cmd.Actor = "stock"
	}
	return s.repo.BindWarehouseCustomer(ctx, cmd)
}

func (s *Service) EnsureCustomerWarehouse(ctx context.Context, cmd EnsureCustomerWarehouseCommand) (WarehouseRow, error) {
	if cmd.CustomerID <= 0 {
		return WarehouseRow{}, fmt.Errorf("customer_id required")
	}
	cmd.Kind = strings.ToLower(strings.TrimSpace(cmd.Kind))
	switch cmd.Kind {
	case "raw", "packaging", "finished":
	default:
		return WarehouseRow{}, fmt.Errorf("invalid warehouse kind")
	}
	creator, ok := s.repo.(CustomerWarehouseCreator)
	if !ok {
		return WarehouseRow{}, fmt.Errorf("customer warehouse creation unavailable")
	}
	return creator.EnsureCustomerWarehouse(ctx, cmd)
}

func (s *Service) ReceiveMaterial(ctx context.Context, cmd MaterialReceiptCommand) (MaterialReceiptResult, error) {
	if cmd.MaterialID <= 0 {
		return MaterialReceiptResult{}, fmt.Errorf("material required")
	}
	cmd.UnitCode = strings.TrimSpace(cmd.UnitCode)
	if cmd.QtyG <= 0 && cmd.QtyUnits <= 0 && cmd.Qty > 0 {
		qtyG, qtyUnits, ok := receiptQtyToLegacy(cmd.Qty, cmd.UnitCode)
		if !ok {
			return MaterialReceiptResult{}, fmt.Errorf("unit_code required for material receipt")
		}
		cmd.QtyG = qtyG
		cmd.QtyUnits = qtyUnits
	}
	if cmd.QtyG <= 0 && cmd.QtyUnits <= 0 {
		return MaterialReceiptResult{}, fmt.Errorf("qty required")
	}
	if cmd.UnitCost < 0 {
		return MaterialReceiptResult{}, fmt.Errorf("unit_cost must be >= 0")
	}
	cmd.Supplier = strings.TrimSpace(cmd.Supplier)
	cmd.CropSeason = strings.TrimSpace(cmd.CropSeason)
	cmd.Origin = strings.TrimSpace(cmd.Origin)
	cmd.ProducerFlavorDescription = strings.TrimSpace(cmd.ProducerFlavorDescription)
	cmd.Note = strings.TrimSpace(cmd.Note)
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	cmd.TargetWarehouse = normalizeWarehouse(cmd.TargetWarehouse)
	if cmd.TargetWarehouse == "" {
		cmd.TargetWarehouse = stockdomain.WarehouseRawMaterials
	}
	if cmd.Operator == "" {
		cmd.Operator = "stock"
	}
	if _, ok := s.repo.(StockDocumentRepository); ok {
		detail, err := s.CreateAndSubmitStockDocument(ctx, StockDocumentCommand{
			Purpose:  PurposeMaterialReceipt,
			Operator: cmd.Operator,
			Note:     cmd.Note,
			Items: []StockDocumentItemCommand{{
				MaterialID:                cmd.MaterialID,
				ItemType:                  itemTypeMaterial,
				InventoryUnit:             cmd.UnitCode,
				ToWarehouse:               cmd.TargetWarehouse,
				QtyG:                      cmd.QtyG,
				QtyUnits:                  cmd.QtyUnits,
				UnitCost:                  cmd.UnitCost,
				Supplier:                  cmd.Supplier,
				CropSeason:                cmd.CropSeason,
				Origin:                    cmd.Origin,
				ProducerFlavorDescription: cmd.ProducerFlavorDescription,
			}},
		})
		if err != nil {
			return MaterialReceiptResult{}, err
		}
		result := MaterialReceiptResult{ReceiptID: detail.ID, EntryID: detail.ID, EntryNo: detail.EntryNo}
		if len(detail.Items) > 0 {
			result.BatchCode = detail.Items[0].BatchCode
			if len(detail.Items[0].Allocations) > 0 {
				result.BatchID = detail.Items[0].Allocations[0].MaterialBatchID
				result.BatchCode = detail.Items[0].Allocations[0].BatchCode
			}
		}
		return result, nil
	}
	return s.repo.ReceiveMaterial(ctx, cmd)
}

func receiptQtyToLegacy(qty float64, unitCode string) (int64, int64, bool) {
	unitCode = strings.TrimSpace(unitCode)
	if unitCode == "" {
		return 0, 0, false
	}
	if isReceiptWeightUnit(unitCode) {
		return int64(math.Round(receiptWeightQtyToGrams(qty, unitCode))), 0, true
	}
	return 0, int64(math.Round(qty)), true
}

func isReceiptWeightUnit(unitCode string) bool {
	switch strings.ToLower(strings.TrimSpace(unitCode)) {
	case "g", "kg", "lb", "oz", "克", "千克":
		return true
	default:
		return false
	}
}

func receiptWeightQtyToGrams(qty float64, unitCode string) float64 {
	switch strings.ToLower(strings.TrimSpace(unitCode)) {
	case "kg", "千克":
		return qty * 1000
	case "lb":
		return qty * 453.59237
	case "oz":
		return qty * 28.349523125
	default:
		return qty
	}
}

func (s *Service) TransferMaterial(ctx context.Context, cmd MaterialTransferCommand) (MaterialTransferResult, error) {
	if cmd.MaterialID <= 0 {
		return MaterialTransferResult{}, fmt.Errorf("material required")
	}
	if cmd.QtyG <= 0 {
		return MaterialTransferResult{}, fmt.Errorf("qty_g required")
	}
	cmd.FromWarehouse = normalizeWarehouse(cmd.FromWarehouse)
	cmd.ToWarehouse = normalizeWarehouse(cmd.ToWarehouse)
	if cmd.FromWarehouse == "" {
		cmd.FromWarehouse = stockdomain.WarehouseRawMaterials
	}
	if cmd.ToWarehouse == "" {
		cmd.ToWarehouse = stockdomain.WarehouseWIP
	}
	if cmd.FromWarehouse == cmd.ToWarehouse {
		return MaterialTransferResult{}, fmt.Errorf("from/to warehouse must differ")
	}
	cmd.Note = strings.TrimSpace(cmd.Note)
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	cmd.IdempotencyKey = strings.TrimSpace(cmd.IdempotencyKey)
	if cmd.Operator == "" {
		cmd.Operator = "stock"
	}
	if _, ok := s.repo.(StockDocumentRepository); ok {
		detail, err := s.CreateAndSubmitStockDocument(ctx, StockDocumentCommand{
			Purpose:        PurposeMaterialTransfer,
			Operator:       cmd.Operator,
			Note:           cmd.Note,
			IdempotencyKey: cmd.IdempotencyKey,
			Items: []StockDocumentItemCommand{{
				MaterialID:    cmd.MaterialID,
				ItemType:      itemTypeMaterial,
				FromWarehouse: cmd.FromWarehouse,
				ToWarehouse:   cmd.ToWarehouse,
				QtyG:          cmd.QtyG,
			}},
		})
		if err != nil {
			return MaterialTransferResult{}, err
		}
		result := MaterialTransferResult{TransferID: detail.ID, TransferNo: detail.EntryNo, EntryID: detail.ID, EntryNo: detail.EntryNo}
		if len(detail.Items) > 0 {
			for _, alloc := range detail.Items[0].Allocations {
				result.Allocations = append(result.Allocations, MaterialTransferAllocation{
					MaterialBatchID: alloc.MaterialBatchID,
					BatchCode:       alloc.BatchCode,
					QtyG:            alloc.QtyG,
				})
			}
		}
		return result, nil
	}
	return s.repo.TransferMaterial(ctx, cmd)
}

func (s *Service) TransferFinishedProduct(ctx context.Context, cmd FinishedProductTransferCommand) (FinishedProductTransferResult, error) {
	if cmd.ProductID <= 0 {
		return FinishedProductTransferResult{}, fmt.Errorf("product required")
	}
	if cmd.BomSpecID > 0 {
		if cmd.SpecG != 0 {
			return FinishedProductTransferResult{}, fmt.Errorf("spec_g must be zero for BOM specification")
		}
		if cmd.QtyLooseG != 0 {
			return FinishedProductTransferResult{}, fmt.Errorf("qty_loose_g is not supported for BOM specification")
		}
	} else if cmd.SpecG <= 0 {
		return FinishedProductTransferResult{}, fmt.Errorf("spec_g or bom_spec_id required")
	}
	if cmd.QtyUnits < 0 || cmd.QtyLooseG < 0 {
		return FinishedProductTransferResult{}, fmt.Errorf("negative qty")
	}
	if cmd.BomSpecID == 0 && cmd.QtyLooseG >= cmd.SpecG {
		cmd.QtyUnits += cmd.QtyLooseG / cmd.SpecG
		cmd.QtyLooseG = cmd.QtyLooseG % cmd.SpecG
	}
	if cmd.QtyUnits <= 0 && cmd.QtyLooseG <= 0 {
		return FinishedProductTransferResult{}, fmt.Errorf("qty required")
	}
	cmd.FromWarehouse = normalizeWarehouse(cmd.FromWarehouse)
	cmd.ToWarehouse = normalizeWarehouse(cmd.ToWarehouse)
	if cmd.FromWarehouse == "" {
		cmd.FromWarehouse = stockdomain.WarehouseFinishedGoods
	}
	if cmd.ToWarehouse == "" {
		return FinishedProductTransferResult{}, fmt.Errorf("to warehouse required")
	}
	if cmd.FromWarehouse == cmd.ToWarehouse {
		return FinishedProductTransferResult{}, fmt.Errorf("from/to warehouse must differ")
	}
	cmd.Note = strings.TrimSpace(cmd.Note)
	cmd.UnitCode = strings.TrimSpace(cmd.UnitCode)
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	cmd.IdempotencyKey = strings.TrimSpace(cmd.IdempotencyKey)
	if cmd.Operator == "" {
		cmd.Operator = "stock"
	}
	if _, ok := s.repo.(StockDocumentRepository); ok {
		detail, err := s.CreateAndSubmitStockDocument(ctx, StockDocumentCommand{
			Purpose:        PurposeMaterialTransfer,
			Operator:       cmd.Operator,
			Note:           cmd.Note,
			IdempotencyKey: cmd.IdempotencyKey,
			Items: []StockDocumentItemCommand{{
				ProductID:     cmd.ProductID,
				ItemType:      itemTypeFinishedProduct,
				SpecG:         cmd.SpecG,
				BomSpecID:     cmd.BomSpecID,
				BomVariantID:  cmd.BomVariantID,
				InventoryUnit: cmd.UnitCode,
				FromWarehouse: cmd.FromWarehouse,
				ToWarehouse:   cmd.ToWarehouse,
				QtyG:          cmd.QtyLooseG,
				QtyUnits:      cmd.QtyUnits,
			}},
		})
		if err != nil {
			return FinishedProductTransferResult{}, err
		}
		result := FinishedProductTransferResult{TransferID: detail.ID, TransferNo: detail.EntryNo, EntryID: detail.ID, EntryNo: detail.EntryNo, ProductID: cmd.ProductID, SpecG: cmd.SpecG, BomSpecID: cmd.BomSpecID, BomVariantID: cmd.BomVariantID}
		if len(detail.Items) > 0 {
			result.BomSpecID = detail.Items[0].BomSpecID
			result.BomVariantID = detail.Items[0].BomVariantID
		}
		return result, nil
	}
	result, err := s.repo.TransferFinishedProduct(ctx, cmd)
	if err != nil {
		return FinishedProductTransferResult{}, err
	}
	result.ProductID = cmd.ProductID
	result.SpecG = cmd.SpecG
	if result.BomSpecID == 0 {
		result.BomSpecID = cmd.BomSpecID
	}
	if result.BomVariantID == 0 {
		result.BomVariantID = cmd.BomVariantID
	}
	return result, nil
}

func (s *Service) CreateAdjustment(ctx context.Context, cmd StockAdjustmentCommand) (StockAdjustmentResult, error) {
	cmd.AdjustmentType = strings.TrimSpace(cmd.AdjustmentType)
	if cmd.AdjustmentType == "" {
		cmd.AdjustmentType = "quantity"
	}
	cmd.ItemType = normalizeStockItemType(cmd.ItemType)
	cmd.Warehouse = normalizeWarehouse(cmd.Warehouse)
	cmd.UnitCode = strings.TrimSpace(cmd.UnitCode)
	cmd.Reason = strings.TrimSpace(cmd.Reason)
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	if cmd.Operator == "" {
		cmd.Operator = "stock"
	}
	if cmd.AdjustmentType != "quantity" && cmd.AdjustmentType != "material_cost" {
		return StockAdjustmentResult{}, fmt.Errorf("invalid adjustment_type")
	}
	if cmd.ItemType != "material" && cmd.ItemType != "finished_product" {
		return StockAdjustmentResult{}, fmt.Errorf("invalid item_type")
	}
	if cmd.ItemID <= 0 {
		return StockAdjustmentResult{}, fmt.Errorf("item required")
	}
	if cmd.ItemType == "material" && cmd.Warehouse == "" {
		cmd.Warehouse = stockdomain.WarehouseRawMaterials
	}
	if cmd.ItemType == "finished_product" && cmd.Warehouse == "" {
		cmd.Warehouse = stockdomain.WarehouseFinishedGoods
	}
	if cmd.ItemType == "finished_product" {
		if cmd.BomSpecID > 0 {
			if cmd.SpecG != 0 {
				return StockAdjustmentResult{}, fmt.Errorf("spec_g must be zero for BOM specification")
			}
			if cmd.TargetG != 0 {
				return StockAdjustmentResult{}, fmt.Errorf("target_g is not supported for BOM specification")
			}
		} else if cmd.SpecG <= 0 {
			return StockAdjustmentResult{}, fmt.Errorf("spec_g or bom_spec_id required")
		}
	}
	if cmd.TargetG < 0 || cmd.TargetUnits < 0 || (cmd.HasTargetQty && cmd.TargetQty < 0) {
		return StockAdjustmentResult{}, fmt.Errorf("negative qty")
	}
	if cmd.TargetUnitCost < 0 {
		return StockAdjustmentResult{}, fmt.Errorf("target_unit_cost must be >= 0")
	}
	if cmd.Reason == "" {
		return StockAdjustmentResult{}, fmt.Errorf("reason required")
	}
	if cmd.AdjustmentType == "material_cost" {
		if cmd.ItemType != "material" {
			return StockAdjustmentResult{}, fmt.Errorf("material cost adjustment requires material")
		}
		if cmd.MaterialBatchID <= 0 {
			return StockAdjustmentResult{}, fmt.Errorf("material_batch_id required")
		}
	}
	result, err := s.repo.CreateAdjustment(ctx, cmd)
	if err != nil {
		return StockAdjustmentResult{}, err
	}
	if cmd.ItemType == "finished_product" {
		result.ProductID = cmd.ItemID
		result.SpecG = cmd.SpecG
		if result.BomSpecID == 0 {
			result.BomSpecID = cmd.BomSpecID
		}
		if result.BomVariantID == 0 {
			result.BomVariantID = cmd.BomVariantID
		}
	}
	return result, nil
}

func normalizeStockItemType(v string) string {
	v = strings.TrimSpace(v)
	switch v {
	case "product", "finished", "finishedProduct":
		return "finished_product"
	default:
		return v
	}
}

func normalizeWarehouse(v string) string {
	v = strings.TrimSpace(v)
	switch v {
	case "materials", "raw", "material":
		return stockdomain.WarehouseRawMaterials
	case "wip_materials", "production_wip":
		return stockdomain.WarehouseWIP
	default:
		return v
	}
}

func normalizePage(limit, offset, def, max int) (int, int) {
	if limit <= 0 {
		limit = def
	}
	if limit > max {
		limit = def
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}
