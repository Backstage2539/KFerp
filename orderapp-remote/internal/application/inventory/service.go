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
}

type AdjustFinishedInventoryCommand struct {
	ProductID int64
	SpecG     int64
	Units     int64
	LooseG    int64
	Operator  string
}

type Repository interface {
	ListFinished(ctx context.Context, query FinishedInventoryQuery) (FinishedInventoryResult, error)
	AdjustFinished(ctx context.Context, cmd AdjustFinishedInventoryCommand) error
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
	return s.repo.AdjustFinished(ctx, cmd)
}
