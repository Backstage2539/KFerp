package bom

import (
	"context"
	"fmt"
)

type ListItem struct {
	ProductID  int64   `json:"product_id"`
	CustomerID int64   `json:"customer_id"`
	Product    string  `json:"product"`
	RoastLevel string  `json:"roast_level"`
	YieldRate  float64 `json:"yield_rate"`
	Status     string  `json:"status"`
	ItemCount  int     `json:"item_count"`
	UpdatedAt  string  `json:"updated_at"`
}

type Item struct {
	ID                   int64   `json:"id"`
	MaterialID           int64   `json:"material_id"`
	MaterialName         string  `json:"material_name"`
	ComponentType        string  `json:"component_type"`
	ComponentProductID   int64   `json:"component_product_id"`
	ComponentProductName string  `json:"component_product_name"`
	ComponentSpecG       int64   `json:"component_spec_g"`
	ConsumeUnit          string  `json:"consume_unit"`
	QtyPerUnit           float64 `json:"qty_per_unit"`
	RatioPct             float64 `json:"ratio_pct"`
}

type Detail struct {
	ProductID   int64   `json:"product_id"`
	ProductName string  `json:"product_name"`
	RoastLevel  string  `json:"roast_level"`
	YieldRate   float64 `json:"yield_rate"`
	Status      string  `json:"status"`
	Items       []Item  `json:"items"`
	TotalRatio  float64 `json:"total_ratio"`
	UpdatedAt   string  `json:"updated_at"`
}

type Option struct {
	ID              int64   `json:"id"`
	Name            string  `json:"name"`
	CustomerID      int64   `json:"customer_id"`
	RoastLevel      string  `json:"roast_level,omitempty"`
	ProductKind     string  `json:"product_kind,omitempty"`
	DripBagGrams    float64 `json:"drip_bag_grams,omitempty"`
	DripBoxBagCount int     `json:"drip_box_bag_count,omitempty"`
}

type BagSpecMapping struct {
	SpecG        int64  `json:"spec_g"`
	MaterialID   int64  `json:"material_id"`
	MaterialName string `json:"material_name"`
}

type Version struct {
	ID        int64   `json:"id"`
	ProductID int64   `json:"product_id"`
	VersionNo string  `json:"version_no"`
	Status    string  `json:"status"`
	YieldRate float64 `json:"yield_rate"`
	ItemCount int     `json:"item_count"`
	Note      string  `json:"note"`
	CreatedAt string  `json:"created_at"`
}

type CreateVersionCommand struct {
	ProductID int64
	Note      string
}

type SaveItemCommand struct {
	ProductID          int64   `json:"product_id"`
	MaterialID         int64   `json:"material_id"`
	ComponentType      string  `json:"component_type"`
	ComponentProductID int64   `json:"component_product_id"`
	ComponentSpecG     int64   `json:"component_spec_g"`
	ConsumeUnit        string  `json:"consume_unit"`
	QtyPerUnit         float64 `json:"qty_per_unit"`
	RatioPct           float64 `json:"ratio_pct"`
	Actor              string  `json:"actor"`
}

type DeleteItemCommand struct {
	ProductID int64
	ID        int64
	Actor     string
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
	DeactivateBom(ctx context.Context, productID int64) error
	SaveItem(ctx context.Context, cmd SaveItemCommand) error
	DeleteItem(ctx context.Context, cmd DeleteItemCommand) error
	SaveBagSpecMapping(ctx context.Context, cmd SaveBagSpecMappingCommand) error
	DeleteBagSpecMapping(ctx context.Context, specG int64) error
	ListVersions(ctx context.Context, productID int64) ([]Version, error)
	CreateVersion(ctx context.Context, cmd CreateVersionCommand) (Version, error)
	ActivateVersion(ctx context.Context, versionID int64) error
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

func (s *Service) DeactivateBom(ctx context.Context, productID int64) error {
	if productID <= 0 {
		return fmt.Errorf("product required")
	}
	return s.repo.DeactivateBom(ctx, productID)
}

func (s *Service) SaveItem(ctx context.Context, cmd SaveItemCommand) error {
	if cmd.ProductID <= 0 {
		return fmt.Errorf("product required")
	}
	componentType := cmd.ComponentType
	if componentType == "" {
		componentType = "material"
	}
	if componentType != "material" && componentType != "finished_product" {
		return fmt.Errorf("invalid component_type")
	}
	consumeUnit := cmd.ConsumeUnit
	if consumeUnit == "" {
		consumeUnit = "ratio_pct"
	}
	switch consumeUnit {
	case "ratio_pct", "g_per_bag", "unit_per_bag", "unit_per_box":
	default:
		return fmt.Errorf("invalid consume_unit")
	}
	switch componentType {
	case "material":
		if cmd.MaterialID <= 0 {
			return fmt.Errorf("material_id required")
		}
	case "finished_product":
		if cmd.ComponentProductID <= 0 {
			return fmt.Errorf("component_product_id required")
		}
	}
	if consumeUnit == "ratio_pct" {
		if cmd.RatioPct <= 0 || cmd.RatioPct > 100 {
			return fmt.Errorf("ratio must be (0,100]")
		}
	} else if cmd.QtyPerUnit <= 0 {
		return fmt.Errorf("qty_per_unit required")
	}
	cmd.ComponentType = componentType
	cmd.ConsumeUnit = consumeUnit
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

func (s *Service) ListVersions(ctx context.Context, productID int64) ([]Version, error) {
	if productID <= 0 {
		return nil, fmt.Errorf("product_id required")
	}
	return s.repo.ListVersions(ctx, productID)
}

func (s *Service) CreateVersion(ctx context.Context, cmd CreateVersionCommand) (Version, error) {
	if cmd.ProductID <= 0 {
		return Version{}, fmt.Errorf("product_id required")
	}
	return s.repo.CreateVersion(ctx, cmd)
}

func (s *Service) ActivateVersion(ctx context.Context, versionID int64) error {
	if versionID <= 0 {
		return fmt.Errorf("version_id required")
	}
	return s.repo.ActivateVersion(ctx, versionID)
}
