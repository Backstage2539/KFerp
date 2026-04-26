package bom

import (
	"context"
	"fmt"
)

type ListItem struct {
	ProductID  int64   `json:"product_id"`
	Product    string  `json:"product"`
	RoastLevel string  `json:"roast_level"`
	YieldRate  float64 `json:"yield_rate"`
	ItemCount  int     `json:"item_count"`
	UpdatedAt  string  `json:"updated_at"`
}

type Item struct {
	ID           int64   `json:"id"`
	MaterialID   int64   `json:"material_id"`
	MaterialName string  `json:"material_name"`
	RatioPct     float64 `json:"ratio_pct"`
}

type Detail struct {
	ProductID   int64   `json:"product_id"`
	ProductName string  `json:"product_name"`
	RoastLevel  string  `json:"roast_level"`
	YieldRate   float64 `json:"yield_rate"`
	Items       []Item  `json:"items"`
	TotalRatio  float64 `json:"total_ratio"`
	UpdatedAt   string  `json:"updated_at"`
}

type Option struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type BagSpecMapping struct {
	SpecG        int64  `json:"spec_g"`
	MaterialID   int64  `json:"material_id"`
	MaterialName string `json:"material_name"`
}

type SaveItemCommand struct {
	ProductID  int64
	MaterialID int64
	RatioPct   float64
}

type DeleteItemCommand struct {
	ProductID int64
	ID        int64
}

type SaveBagSpecMappingCommand struct {
	SpecG      int64
	MaterialID int64
}

type Repository interface {
	List(ctx context.Context) ([]ListItem, error)
	Detail(ctx context.Context, productID int64) (Detail, error)
	Products(ctx context.Context) ([]Option, error)
	Materials(ctx context.Context) ([]Option, error)
	BagSpecMappings(ctx context.Context) ([]BagSpecMapping, error)
	SyncProductYield(ctx context.Context, productID int64) error
	SaveItem(ctx context.Context, cmd SaveItemCommand) error
	DeleteItem(ctx context.Context, cmd DeleteItemCommand) error
	SaveBagSpecMapping(ctx context.Context, cmd SaveBagSpecMappingCommand) error
	DeleteBagSpecMapping(ctx context.Context, specG int64) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context) ([]ListItem, error) {
	return s.repo.List(ctx)
}

func (s *Service) Detail(ctx context.Context, productID int64) (Detail, error) {
	if productID <= 0 {
		return Detail{}, fmt.Errorf("invalid product_id")
	}
	return s.repo.Detail(ctx, productID)
}

func (s *Service) Products(ctx context.Context) ([]Option, error) {
	return s.repo.Products(ctx)
}

func (s *Service) Materials(ctx context.Context) ([]Option, error) {
	return s.repo.Materials(ctx)
}

func (s *Service) BagSpecMappings(ctx context.Context) ([]BagSpecMapping, error) {
	return s.repo.BagSpecMappings(ctx)
}

func (s *Service) SyncProductYield(ctx context.Context, productID int64) error {
	if productID <= 0 {
		return fmt.Errorf("product required")
	}
	return s.repo.SyncProductYield(ctx, productID)
}

func (s *Service) SaveItem(ctx context.Context, cmd SaveItemCommand) error {
	if cmd.ProductID <= 0 || cmd.MaterialID <= 0 {
		return fmt.Errorf("product/material required")
	}
	if cmd.RatioPct <= 0 || cmd.RatioPct > 100 {
		return fmt.Errorf("ratio must be (0,100]")
	}
	return s.repo.SaveItem(ctx, cmd)
}

func (s *Service) DeleteItem(ctx context.Context, cmd DeleteItemCommand) error {
	if cmd.ID <= 0 {
		return nil
	}
	return s.repo.DeleteItem(ctx, cmd)
}

func (s *Service) SaveBagSpecMapping(ctx context.Context, cmd SaveBagSpecMappingCommand) error {
	if cmd.SpecG <= 0 {
		return fmt.Errorf("spec_g required")
	}
	if cmd.MaterialID <= 0 {
		return fmt.Errorf("material_id required")
	}
	return s.repo.SaveBagSpecMapping(ctx, cmd)
}

func (s *Service) DeleteBagSpecMapping(ctx context.Context, specG int64) error {
	if specG <= 0 {
		return fmt.Errorf("spec_g required")
	}
	return s.repo.DeleteBagSpecMapping(ctx, specG)
}
