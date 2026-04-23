package catalog

import "context"

type PriceTier struct {
	ID      int64
	MinLb   float64
	MaxLb   *float64
	PriceLb float64
}

type Product struct {
	ID           int64
	Name         string
	DefaultPrice float64
	Tiers        []PriceTier
}

type ReplacePriceTiersCommand struct {
	Actor     string
	ProductID int64
	Tiers     []PriceTier
}

type Repository interface {
	ListProducts(ctx context.Context) ([]Product, error)
	GetProduct(ctx context.Context, id int64) (*Product, error)
	ReplacePriceTiers(ctx context.Context, cmd ReplacePriceTiersCommand) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListProducts(ctx context.Context) ([]Product, error) {
	return s.repo.ListProducts(ctx)
}

func (s *Service) GetProduct(ctx context.Context, id int64) (*Product, error) {
	return s.repo.GetProduct(ctx, id)
}

func (s *Service) ReplacePriceTiers(ctx context.Context, cmd ReplacePriceTiersCommand) error {
	return s.repo.ReplacePriceTiers(ctx, cmd)
}
