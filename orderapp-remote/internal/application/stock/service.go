package stock

import (
	"context"
	"fmt"
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
	ID            int64   `json:"id"`
	BatchCode     string  `json:"batch_code"`
	MaterialID    int64   `json:"material_id"`
	MaterialName  string  `json:"material_name"`
	Supplier      string  `json:"supplier"`
	ReceiptID     int64   `json:"receipt_id"`
	QtyG          int64   `json:"qty_g"`
	RemainingG    int64   `json:"remaining_g"`
	UnitCost      float64 `json:"unit_cost"`
	ReceivedAt    string  `json:"received_at"`
	Status        string  `json:"status"`
	QualityStatus string  `json:"quality_status"`
	Note          string  `json:"note"`
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
	Code        string `json:"code"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	ParentCode  string `json:"parent_code"`
	SortOrder   int    `json:"sort_order"`
	IsDefault   bool   `json:"is_default"`
	Active      bool   `json:"active"`
	Description string `json:"description"`
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

type WarehouseInventoryQuery struct {
	Q         string
	Warehouse string
	ItemType  string
	Limit     int
	Offset    int
}

type WarehouseInventoryRow struct {
	Warehouse     string  `json:"warehouse"`
	WarehouseName string  `json:"warehouse_name"`
	WarehouseKind string  `json:"warehouse_kind"`
	ItemType      string  `json:"item_type"`
	ItemID        int64   `json:"item_id"`
	ItemName      string  `json:"item_name"`
	SpecG         int64   `json:"spec_g"`
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
	ID            int64   `json:"id"`
	BatchCode     string  `json:"batch_code"`
	MaterialID    int64   `json:"material_id"`
	MaterialName  string  `json:"material_name"`
	Supplier      string  `json:"supplier"`
	ReceiptID     int64   `json:"receipt_id"`
	QtyG          int64   `json:"qty_g"`
	RemainingG    int64   `json:"remaining_g"`
	UnitCost      float64 `json:"unit_cost"`
	ReceivedAt    string  `json:"received_at"`
	Status        string  `json:"status"`
	QualityStatus string  `json:"quality_status"`
	Note          string  `json:"note"`
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
	Allocations []MaterialTransferAllocation `json:"allocations"`
}

type FinishedProductTransferCommand struct {
	ProductID      int64
	SpecG          int64
	FromWarehouse  string
	ToWarehouse    string
	QtyUnits       int64
	QtyLooseG      int64
	Note           string
	Operator       string
	IdempotencyKey string
}

type FinishedProductTransferResult struct {
	TransferID int64  `json:"transfer_id"`
	TransferNo string `json:"transfer_no"`
}

type MaterialReceiptCommand struct {
	MaterialID int64
	Supplier   string
	QtyG       int64
	UnitCost   float64
	Note       string
	Operator   string
}

type MaterialReceiptResult struct {
	ReceiptID int64  `json:"receipt_id"`
	BatchID   int64  `json:"batch_id"`
	BatchCode string `json:"batch_code"`
}

type StockAdjustmentCommand struct {
	ItemType    string
	ItemID      int64
	SpecG       int64
	Warehouse   string
	TargetG     int64
	TargetUnits int64
	Reason      string
	Operator    string
}

type StockAdjustmentResult struct {
	AdjustmentID int64 `json:"adjustment_id"`
}

type Repository interface {
	ListLedger(ctx context.Context, query LedgerQuery) (LedgerResult, error)
	ListBatches(ctx context.Context, query BatchQuery) (BatchResult, error)
	ListMaterialBatches(ctx context.Context, query MaterialBatchQuery) (MaterialBatchResult, error)
	ListWarehouses(ctx context.Context) ([]WarehouseRow, error)
	ListMaterialBatchLocations(ctx context.Context, query MaterialBatchLocationQuery) (MaterialBatchLocationResult, error)
	ListWarehouseInventory(ctx context.Context, query WarehouseInventoryQuery) (WarehouseInventoryResult, error)
	ListOutboundLogs(ctx context.Context, query OutboundLogQuery) (OutboundLogResult, error)
	GetStockTrace(ctx context.Context, query StockTraceQuery) (StockTraceResult, error)
	ReceiveMaterial(ctx context.Context, cmd MaterialReceiptCommand) (MaterialReceiptResult, error)
	CreateAdjustment(ctx context.Context, cmd StockAdjustmentCommand) (StockAdjustmentResult, error)
	TransferMaterial(ctx context.Context, cmd MaterialTransferCommand) (MaterialTransferResult, error)
	TransferFinishedProduct(ctx context.Context, cmd FinishedProductTransferCommand) (FinishedProductTransferResult, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
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

func (s *Service) ListWarehouses(ctx context.Context) ([]WarehouseRow, error) {
	return s.repo.ListWarehouses(ctx)
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
	query.Limit, query.Offset = normalizePage(query.Limit, query.Offset, 100, 500)
	return s.repo.ListWarehouseInventory(ctx, query)
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

func (s *Service) ReceiveMaterial(ctx context.Context, cmd MaterialReceiptCommand) (MaterialReceiptResult, error) {
	if cmd.MaterialID <= 0 {
		return MaterialReceiptResult{}, fmt.Errorf("material required")
	}
	if cmd.QtyG <= 0 {
		return MaterialReceiptResult{}, fmt.Errorf("qty_g required")
	}
	if cmd.UnitCost < 0 {
		return MaterialReceiptResult{}, fmt.Errorf("unit_cost must be >= 0")
	}
	cmd.Supplier = strings.TrimSpace(cmd.Supplier)
	cmd.Note = strings.TrimSpace(cmd.Note)
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	if cmd.Operator == "" {
		cmd.Operator = "stock"
	}
	return s.repo.ReceiveMaterial(ctx, cmd)
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
	return s.repo.TransferMaterial(ctx, cmd)
}

func (s *Service) TransferFinishedProduct(ctx context.Context, cmd FinishedProductTransferCommand) (FinishedProductTransferResult, error) {
	if cmd.ProductID <= 0 {
		return FinishedProductTransferResult{}, fmt.Errorf("product required")
	}
	if cmd.SpecG <= 0 {
		return FinishedProductTransferResult{}, fmt.Errorf("spec_g required")
	}
	if cmd.QtyUnits < 0 || cmd.QtyLooseG < 0 {
		return FinishedProductTransferResult{}, fmt.Errorf("negative qty")
	}
	if cmd.QtyLooseG >= cmd.SpecG {
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
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	cmd.IdempotencyKey = strings.TrimSpace(cmd.IdempotencyKey)
	if cmd.Operator == "" {
		cmd.Operator = "stock"
	}
	return s.repo.TransferFinishedProduct(ctx, cmd)
}

func (s *Service) CreateAdjustment(ctx context.Context, cmd StockAdjustmentCommand) (StockAdjustmentResult, error) {
	cmd.ItemType = normalizeStockItemType(cmd.ItemType)
	cmd.Warehouse = normalizeWarehouse(cmd.Warehouse)
	cmd.Reason = strings.TrimSpace(cmd.Reason)
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	if cmd.Operator == "" {
		cmd.Operator = "stock"
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
	if cmd.ItemType == "finished_product" && cmd.SpecG <= 0 {
		return StockAdjustmentResult{}, fmt.Errorf("spec_g required")
	}
	if cmd.TargetG < 0 || cmd.TargetUnits < 0 {
		return StockAdjustmentResult{}, fmt.Errorf("negative qty")
	}
	if cmd.Reason == "" {
		return StockAdjustmentResult{}, fmt.Errorf("reason required")
	}
	return s.repo.CreateAdjustment(ctx, cmd)
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
