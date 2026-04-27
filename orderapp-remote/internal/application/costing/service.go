package costing

import (
	"context"
	"fmt"

	domain "orderapp/internal/domain/costing"
)

type CalculateRequest struct {
	Products []domain.ProductInput `json:"products"`
}

type CalculateResponse struct {
	Parameters domain.Parameters      `json:"parameters"`
	Items      []domain.ProductResult `json:"items"`
}

type Run struct {
	ID           int64                  `json:"id"`
	Status       string                 `json:"status"`
	ProductCount int                    `json:"product_count"`
	Items        []domain.ProductResult `json:"items,omitempty"`
}

type Repository interface {
	LoadParameters(ctx context.Context) (domain.Parameters, error)
	LoadProductInputs(ctx context.Context, params domain.Parameters) ([]domain.ProductInput, error)
	CreateRun(ctx context.Context, actor string, items []domain.ProductResult) (*Run, error)
	PublishRun(ctx context.Context, actor string, runID int64) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Parameters(ctx context.Context) (domain.Parameters, error) {
	if s.repo == nil {
		return domain.DefaultParameters(), nil
	}
	return s.repo.LoadParameters(ctx)
}

func (s *Service) Calculate(ctx context.Context, req CalculateRequest) (*CalculateResponse, error) {
	params, err := s.Parameters(ctx)
	if err != nil {
		return nil, err
	}
	items, err := calculate(req, params)
	if err != nil {
		return nil, err
	}
	return &CalculateResponse{Parameters: params, Items: items}, nil
}

func (s *Service) BeanList(ctx context.Context) (*CalculateResponse, error) {
	params, err := s.Parameters(ctx)
	if err != nil {
		return nil, err
	}
	if s.repo == nil {
		return &CalculateResponse{Parameters: params}, nil
	}
	inputs, err := s.repo.LoadProductInputs(ctx, params)
	if err != nil {
		return nil, err
	}
	items, err := calculate(CalculateRequest{Products: inputs}, params)
	if err != nil {
		return nil, err
	}
	return &CalculateResponse{Parameters: params, Items: items}, nil
}

func (s *Service) CreateRun(ctx context.Context, actor string) (*Run, error) {
	resp, err := s.BeanList(ctx)
	if err != nil {
		return nil, err
	}
	if s.repo == nil {
		return nil, fmt.Errorf("repository required")
	}
	return s.repo.CreateRun(ctx, actor, resp.Items)
}

func (s *Service) PublishRun(ctx context.Context, actor string, runID int64) error {
	if runID <= 0 {
		return fmt.Errorf("invalid id")
	}
	if s.repo == nil {
		return fmt.Errorf("repository required")
	}
	return s.repo.PublishRun(ctx, actor, runID)
}

func calculate(req CalculateRequest, params domain.Parameters) ([]domain.ProductResult, error) {
	if len(req.Products) == 0 {
		return nil, fmt.Errorf("products required")
	}
	out := make([]domain.ProductResult, 0, len(req.Products))
	for _, p := range req.Products {
		in, err := domain.ValidateProductInput(params, p)
		if err != nil {
			return nil, err
		}
		out = append(out, domain.CalculateProduct(params, in))
	}
	return out, nil
}
