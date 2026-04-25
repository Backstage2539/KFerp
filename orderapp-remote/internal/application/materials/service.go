package materials

import "context"

type Material struct {
	ID            int64   `json:"id"`
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	Kind          string  `json:"kind"`
	Unit          string  `json:"unit"`
	PurchasePrice float64 `json:"purchase_price"`
	SalePrice     float64 `json:"sale_price"`
	OnhandG       int64   `json:"onhand_g"`
	OnhandUnits   int64   `json:"onhand_units"`
	MinLevelG     int64   `json:"min_level_g"`
	MinLevelUnits int64   `json:"min_level_units"`
	UpdatedAt     string  `json:"updated_at"`
}

type MaterialInput struct {
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	Kind          string  `json:"kind"`
	Unit          string  `json:"unit"`
	PurchasePrice float64 `json:"purchase_price"`
	SalePrice     float64 `json:"sale_price"`
	OnhandG       int64   `json:"onhand_g"`
	OnhandUnits   int64   `json:"onhand_units"`
	MinLevelG     int64   `json:"min_level_g"`
	MinLevelUnits int64   `json:"min_level_units"`
}

type ListCommand struct {
	Query string
	Limit int
}

type UpdateCommand struct {
	Actor string
	ID    int64
	Input MaterialInput
}

type Repository interface {
	List(ctx context.Context, cmd ListCommand) ([]Material, error)
	Update(ctx context.Context, cmd UpdateCommand) (Material, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context, cmd ListCommand) ([]Material, error) {
	return s.repo.List(ctx, cmd)
}

func (s *Service) Update(ctx context.Context, cmd UpdateCommand) (Material, error) {
	return s.repo.Update(ctx, cmd)
}

