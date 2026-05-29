package bom

import (
	"context"
	"fmt"
	productiondomain "orderapp/internal/domain/production"
	"strings"
)

type ListItem struct {
	ProductID             int64   `json:"product_id"`
	CustomerID            int64   `json:"customer_id"`
	Product               string  `json:"product"`
	RoastLevel            string  `json:"roast_level"`
	ProductKind           string  `json:"product_kind,omitempty"`
	YieldRate             float64 `json:"yield_rate"`
	ExpectedYieldRate     float64 `json:"expected_yield_rate"`
	ExpectedLossRate      float64 `json:"expected_loss_rate"`
	Status                string  `json:"status"`
	ItemCount             int     `json:"item_count"`
	OrderUsageCount       int     `json:"order_usage_count"`
	UpdatedAt             string  `json:"updated_at"`
	BomSourceType         string  `json:"bom_source_type"`
	EffectiveProductID    int64   `json:"effective_product_id"`
	EffectiveBomVersionID int64   `json:"effective_bom_version_id"`
	SourceProductID       int64   `json:"source_product_id"`
	SourceProductCode     string  `json:"source_product_code"`
	SourceProductName     string  `json:"source_product_name"`
	SourceBomVersionID    int64   `json:"source_bom_version_id"`
	SourceBomVersionNo    string  `json:"source_bom_version_no"`
	DerivedFromLabel      string  `json:"derived_from_label"`
	CanEditBOM            bool    `json:"can_edit_bom"`
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
	ProductID             int64   `json:"product_id"`
	ProductName           string  `json:"product_name"`
	RoastLevel            string  `json:"roast_level"`
	YieldRate             float64 `json:"yield_rate"`
	ExpectedYieldRate     float64 `json:"expected_yield_rate"`
	ExpectedLossRate      float64 `json:"expected_loss_rate"`
	Status                string  `json:"status"`
	Items                 []Item  `json:"items"`
	TotalRatio            float64 `json:"total_ratio"`
	UpdatedAt             string  `json:"updated_at"`
	BomSourceType         string  `json:"bom_source_type"`
	EffectiveProductID    int64   `json:"effective_product_id"`
	EffectiveBomVersionID int64   `json:"effective_bom_version_id"`
	SourceProductID       int64   `json:"source_product_id"`
	SourceProductCode     string  `json:"source_product_code"`
	SourceProductName     string  `json:"source_product_name"`
	SourceBomVersionID    int64   `json:"source_bom_version_id"`
	SourceBomVersionNo    string  `json:"source_bom_version_no"`
	DerivedFromLabel      string  `json:"derived_from_label"`
	CanEditBOM            bool    `json:"can_edit_bom"`
}

type Option struct {
	ID              int64   `json:"id"`
	Name            string  `json:"name"`
	CustomerID      int64   `json:"customer_id"`
	RoastLevel      string  `json:"roast_level,omitempty"`
	ProductKind     string  `json:"product_kind,omitempty"`
	DripBagGrams    float64 `json:"drip_bag_grams,omitempty"`
	DripBoxBagCount int     `json:"drip_box_bag_count,omitempty"`
	OrderUsageCount int     `json:"order_usage_count"`
}

type BagSpecMapping struct {
	SpecG        int64  `json:"spec_g"`
	MaterialID   int64  `json:"material_id"`
	MaterialName string `json:"material_name"`
}

type Version struct {
	ID                int64   `json:"id"`
	ProductID         int64   `json:"product_id"`
	VersionNo         string  `json:"version_no"`
	Status            string  `json:"status"`
	YieldRate         float64 `json:"yield_rate"`
	ExpectedYieldRate float64 `json:"expected_yield_rate"`
	ExpectedLossRate  float64 `json:"expected_loss_rate"`
	ItemCount         int     `json:"item_count"`
	Note              string  `json:"note"`
	CreatedAt         string  `json:"created_at"`
}

type CreateVersionCommand struct {
	ProductID int64  `json:"product_id"`
	Note      string `json:"note"`
	Actor     string `json:"actor"`
}

type SyncProductYieldCommand struct {
	ProductID         int64    `json:"product_id"`
	ExpectedLossRate  *float64 `json:"expected_loss_rate,omitempty"`
	ExpectedYieldRate float64  `json:"expected_yield_rate,omitempty"`
	Actor             string   `json:"actor"`
}

type DeactivateBomCommand struct {
	ProductID int64  `json:"product_id"`
	Actor     string `json:"actor"`
}

type ActivateVersionCommand struct {
	VersionID int64  `json:"version_id"`
	Actor     string `json:"actor"`
}

type DeriveOwnedCommand struct {
	ProductID int64  `json:"product_id"`
	Actor     string `json:"actor"`
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
	ProductID int64  `json:"product_id"`
	ID        int64  `json:"id"`
	Actor     string `json:"actor"`
}

type SaveBagSpecMappingCommand struct {
	SpecG      int64  `json:"spec_g"`
	MaterialID int64  `json:"material_id"`
	Actor      string `json:"actor"`
}

type DeleteBagSpecMappingCommand struct {
	SpecG int64  `json:"spec_g"`
	Actor string `json:"actor"`
}

type Repository interface {
	List(ctx context.Context) ([]ListItem, error)
	Detail(ctx context.Context, productID int64) (Detail, error)
	Products(ctx context.Context) ([]Option, error)
	Materials(ctx context.Context) ([]Option, error)
	BagSpecMappings(ctx context.Context) ([]BagSpecMapping, error)
	SyncProductYield(ctx context.Context, cmd SyncProductYieldCommand) error
	DeactivateBom(ctx context.Context, cmd DeactivateBomCommand) error
	SaveItem(ctx context.Context, cmd SaveItemCommand) error
	DeleteItem(ctx context.Context, cmd DeleteItemCommand) error
	SaveBagSpecMapping(ctx context.Context, cmd SaveBagSpecMappingCommand) error
	DeleteBagSpecMapping(ctx context.Context, cmd DeleteBagSpecMappingCommand) error
	ListVersions(ctx context.Context, productID int64) ([]Version, error)
	CreateVersion(ctx context.Context, cmd CreateVersionCommand) (Version, error)
	ActivateVersion(ctx context.Context, cmd ActivateVersionCommand) error
	DeriveOwned(ctx context.Context, cmd DeriveOwnedCommand) (Detail, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context) ([]ListItem, error) {
	rows, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ListItem, 0, len(rows))
	for _, row := range rows {
		if isBomMaintainedProductKind(row.ProductKind) {
			enrichListItemYield(&row)
			out = append(out, row)
		}
	}
	return out, nil
}

func (s *Service) Detail(ctx context.Context, productID int64) (Detail, error) {
	if productID <= 0 {
		return Detail{}, fmt.Errorf("invalid product_id")
	}
	detail, err := s.repo.Detail(ctx, productID)
	if err != nil {
		return Detail{}, err
	}
	enrichDetailYield(&detail)
	return detail, nil
}

func (s *Service) DeriveOwned(ctx context.Context, cmd DeriveOwnedCommand) (Detail, error) {
	if cmd.ProductID <= 0 {
		return Detail{}, fmt.Errorf("product required")
	}
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	detail, err := s.repo.DeriveOwned(ctx, cmd)
	if err != nil {
		return Detail{}, err
	}
	enrichDetailYield(&detail)
	return detail, nil
}

func (s *Service) Products(ctx context.Context) ([]Option, error) {
	rows, err := s.repo.Products(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Option, 0, len(rows))
	for _, row := range rows {
		if isBomMaintainedProductKind(row.ProductKind) {
			out = append(out, row)
		}
	}
	return out, nil
}

func (s *Service) Materials(ctx context.Context) ([]Option, error) {
	return s.repo.Materials(ctx)
}

func (s *Service) BagSpecMappings(ctx context.Context) ([]BagSpecMapping, error) {
	return s.repo.BagSpecMappings(ctx)
}

func isBomMaintainedProductKind(kind string) bool {
	normalized := strings.ToLower(strings.TrimSpace(kind))
	if normalized == "" {
		normalized = "roasted_bean"
	}
	return normalized != "green_bean"
}

func (s *Service) SyncProductYield(ctx context.Context, cmd SyncProductYieldCommand) error {
	if cmd.ProductID <= 0 {
		return fmt.Errorf("product required")
	}
	if cmd.ExpectedLossRate != nil {
		yieldRate, err := productiondomain.YieldRateFromExpectedLossRate(*cmd.ExpectedLossRate)
		if err != nil {
			return err
		}
		cmd.ExpectedYieldRate = yieldRate
	}
	return s.repo.SyncProductYield(ctx, cmd)
}

func (s *Service) DeactivateBom(ctx context.Context, cmd DeactivateBomCommand) error {
	if cmd.ProductID <= 0 {
		return fmt.Errorf("product required")
	}
	return s.repo.DeactivateBom(ctx, cmd)
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
		if consumeUnit == "ratio_pct" {
			return fmt.Errorf("finished_product consume_unit must not be ratio_pct")
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
	if cmd.ProductID <= 0 {
		return fmt.Errorf("product_id required")
	}
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

func (s *Service) DeleteBagSpecMapping(ctx context.Context, cmd DeleteBagSpecMappingCommand) error {
	if cmd.SpecG <= 0 {
		return fmt.Errorf("spec_g required")
	}
	return s.repo.DeleteBagSpecMapping(ctx, cmd)
}

func (s *Service) ListVersions(ctx context.Context, productID int64) ([]Version, error) {
	if productID <= 0 {
		return nil, fmt.Errorf("product_id required")
	}
	rows, err := s.repo.ListVersions(ctx, productID)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		enrichVersionYield(&rows[i])
	}
	return rows, nil
}

func (s *Service) CreateVersion(ctx context.Context, cmd CreateVersionCommand) (Version, error) {
	if cmd.ProductID <= 0 {
		return Version{}, fmt.Errorf("product_id required")
	}
	return s.repo.CreateVersion(ctx, cmd)
}

func enrichListItemYield(row *ListItem) {
	row.ExpectedYieldRate = productiondomain.NormalizeYieldRate(row.YieldRate)
	row.ExpectedLossRate = productiondomain.ExpectedLossRate(row.ExpectedYieldRate)
	row.YieldRate = row.ExpectedYieldRate
}

func enrichDetailYield(row *Detail) {
	row.ExpectedYieldRate = productiondomain.NormalizeYieldRate(row.YieldRate)
	row.ExpectedLossRate = productiondomain.ExpectedLossRate(row.ExpectedYieldRate)
	row.YieldRate = row.ExpectedYieldRate
}

func enrichVersionYield(row *Version) {
	row.ExpectedYieldRate = productiondomain.NormalizeYieldRate(row.YieldRate)
	row.ExpectedLossRate = productiondomain.ExpectedLossRate(row.ExpectedYieldRate)
	row.YieldRate = row.ExpectedYieldRate
}

func (s *Service) ActivateVersion(ctx context.Context, cmd ActivateVersionCommand) error {
	if cmd.VersionID <= 0 {
		return fmt.Errorf("version_id required")
	}
	return s.repo.ActivateVersion(ctx, cmd)
}
