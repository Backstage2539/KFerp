package inventory

import (
	"context"
	"fmt"
	"strings"

	inventorydomain "orderapp/internal/domain/inventory"
)

type ProductOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type FinishedInventoryRow struct {
	ProductID int64  `json:"product_id"`
	Product   string `json:"product"`
	SpecG     int64  `json:"spec_g"`
	Warehouse string `json:"warehouse"`
	Units     int64  `json:"units"`
	LooseG    int64  `json:"loose_g"`
	UpdatedAt string `json:"updated_at"`
	TotalG    int64  `json:"total_g"`
}

type FinishedInventoryQuery struct {
	Q      string
	Limit  int
	Offset int
}

type FinishedInventoryResult struct {
	Rows     []FinishedInventoryRow
	Products []ProductOption
	HasNext  bool
	Total    int
}

type AllocationLogRow struct {
	BatchID      string `json:"batch_id"`
	Product      string `json:"product"`
	SpecG        int64  `json:"spec_g"`
	NeedG        int64  `json:"need_g"`
	DeductedG    int64  `json:"deducted_g"`
	GapG         int64  `json:"gap_g"`
	Operator     string `json:"operator"`
	CreatedAt    string `json:"created_at"`
	OperatorName string `json:"operator_name"`
}

type AllocationBatchRow struct {
	BatchID      string `json:"batch_id"`
	Items        int64  `json:"items"`
	Operator     string `json:"operator"`
	CreatedAt    string `json:"created_at"`
	OperatorName string `json:"operator_name"`
}

type AllocationLogQuery struct {
	BatchID string
	Limit   int
	Offset  int
}

type AllocationLogResult struct {
	BatchID string
	Batches []AllocationBatchRow
	Rows    []AllocationLogRow
	HasNext bool
	Total   int
}

type AdjustFinishedInventoryCommand struct {
	ProductID int64
	SpecG     int64
	Warehouse string
	Units     int64
	LooseG    int64
	Operator  string
}

type Repository interface {
	ListFinished(ctx context.Context, query FinishedInventoryQuery) (FinishedInventoryResult, error)
	AdjustFinished(ctx context.Context, cmd AdjustFinishedInventoryCommand) error
	ListAllocations(ctx context.Context, query AllocationLogQuery) (AllocationLogResult, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListFinished(ctx context.Context, query FinishedInventoryQuery) (FinishedInventoryResult, error) {
	query.Q = strings.TrimSpace(query.Q)
	if query.Limit <= 0 || query.Limit > 200 {
		query.Limit = 50
	}
	if query.Offset < 0 {
		query.Offset = 0
	}
	return s.repo.ListFinished(ctx, query)
}

func (s *Service) ListAllocations(ctx context.Context, query AllocationLogQuery) (AllocationLogResult, error) {
	query.BatchID = strings.TrimSpace(query.BatchID)
	if query.Limit <= 0 || query.Limit > 200 {
		query.Limit = 20
	}
	if query.Offset < 0 {
		query.Offset = 0
	}
	return s.repo.ListAllocations(ctx, query)
}

func (s *Service) AdjustFinished(ctx context.Context, cmd AdjustFinishedInventoryCommand) error {
	if cmd.ProductID <= 0 {
		return fmt.Errorf("product required")
	}
	if cmd.SpecG <= 0 {
		return fmt.Errorf("spec_g required")
	}
	if cmd.Units < 0 || cmd.LooseG < 0 {
		return fmt.Errorf("negative qty")
	}
	qty, err := inventorydomain.Normalize(cmd.SpecG, inventorydomain.Quantity{Units: cmd.Units, LooseG: cmd.LooseG})
	if err != nil {
		return err
	}
	cmd.Units = qty.Units
	cmd.LooseG = qty.LooseG
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	if cmd.Operator == "" {
		cmd.Operator = "inventory"
	}
	cmd.Warehouse = strings.TrimSpace(cmd.Warehouse)
	if cmd.Warehouse == "" {
		cmd.Warehouse = "finished_goods"
	}
	return s.repo.AdjustFinished(ctx, cmd)
}
