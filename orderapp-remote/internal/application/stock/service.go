package stock

import (
	"context"
	"fmt"
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
	Rows    []LedgerRow `json:"rows"`
	HasNext bool        `json:"has_next"`
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
	Operator       string  `json:"operator"`
	CreatedAt      string  `json:"created_at"`
}

type BatchResult struct {
	Rows    []BatchRow `json:"rows"`
	HasNext bool       `json:"has_next"`
}

type MaterialBatchQuery struct {
	Q          string
	MaterialID int64
	ActiveOnly bool
	Limit      int
	Offset     int
}

type MaterialBatchRow struct {
	ID           int64   `json:"id"`
	BatchCode    string  `json:"batch_code"`
	MaterialID   int64   `json:"material_id"`
	MaterialName string  `json:"material_name"`
	Supplier     string  `json:"supplier"`
	ReceiptID    int64   `json:"receipt_id"`
	QtyG         int64   `json:"qty_g"`
	RemainingG   int64   `json:"remaining_g"`
	UnitCost     float64 `json:"unit_cost"`
	ReceivedAt   string  `json:"received_at"`
	Status       string  `json:"status"`
	Note         string  `json:"note"`
}

type MaterialBatchResult struct {
	Rows    []MaterialBatchRow `json:"rows"`
	HasNext bool               `json:"has_next"`
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
	ReceiveMaterial(ctx context.Context, cmd MaterialReceiptCommand) (MaterialReceiptResult, error)
	CreateAdjustment(ctx context.Context, cmd StockAdjustmentCommand) (StockAdjustmentResult, error)
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

func (s *Service) CreateAdjustment(ctx context.Context, cmd StockAdjustmentCommand) (StockAdjustmentResult, error) {
	cmd.ItemType = strings.TrimSpace(cmd.ItemType)
	cmd.Warehouse = strings.TrimSpace(cmd.Warehouse)
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
