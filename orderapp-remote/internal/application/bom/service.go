package bom

import (
	"context"
	"encoding/json"
	"fmt"
	productiondomain "orderapp/internal/domain/production"
	"strings"
)

type ListItem struct {
	ProductID              int64   `json:"product_id"`
	CustomerID             int64   `json:"customer_id"`
	Product                string  `json:"product"`
	RoastLevel             string  `json:"roast_level"`
	ProductKind            string  `json:"product_kind,omitempty"`
	YieldRate              float64 `json:"yield_rate"`
	ExpectedYieldRate      float64 `json:"expected_yield_rate"`
	ExpectedLossRate       float64 `json:"expected_loss_rate"`
	Status                 string  `json:"status"`
	ItemCount              int     `json:"item_count"`
	OrderUsageCount        int     `json:"order_usage_count"`
	UpdatedAt              string  `json:"updated_at"`
	BomSourceType          string  `json:"bom_source_type"`
	EffectiveProductID     int64   `json:"effective_product_id"`
	EffectiveBomVersionID  int64   `json:"effective_bom_version_id"`
	SourceProductID        int64   `json:"source_product_id"`
	SourceProductCode      string  `json:"source_product_code"`
	SourceProductName      string  `json:"source_product_name"`
	SourceBomVersionID     int64   `json:"source_bom_version_id"`
	SourceBomVersionNo     string  `json:"source_bom_version_no"`
	DerivedFromLabel       string  `json:"derived_from_label"`
	CanEditBOM             bool    `json:"can_edit_bom"`
	ProductionBomID        int64   `json:"production_bom_id"`
	ProductionBomCode      string  `json:"production_bom_code"`
	ProductionBomName      string  `json:"production_bom_name"`
	ProductionBomVersionID int64   `json:"production_bom_version_id"`
	ProductionBomVersionNo string  `json:"production_bom_version_no"`
	LatestBomVersionID     int64   `json:"latest_bom_version_id"`
	LatestBomVersionNo     string  `json:"latest_bom_version_no"`
	IsLatestBomVersion     bool    `json:"is_latest_bom_version"`
	ProductionBomGroupID   int64   `json:"production_bom_group_id"`
	ProductionBomGroupName string  `json:"production_bom_group_name"`
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
	ProductID              int64   `json:"product_id"`
	ProductName            string  `json:"product_name"`
	RoastLevel             string  `json:"roast_level"`
	YieldRate              float64 `json:"yield_rate"`
	ExpectedYieldRate      float64 `json:"expected_yield_rate"`
	ExpectedLossRate       float64 `json:"expected_loss_rate"`
	Status                 string  `json:"status"`
	Items                  []Item  `json:"items"`
	TotalRatio             float64 `json:"total_ratio"`
	UpdatedAt              string  `json:"updated_at"`
	BomSourceType          string  `json:"bom_source_type"`
	EffectiveProductID     int64   `json:"effective_product_id"`
	EffectiveBomVersionID  int64   `json:"effective_bom_version_id"`
	SourceProductID        int64   `json:"source_product_id"`
	SourceProductCode      string  `json:"source_product_code"`
	SourceProductName      string  `json:"source_product_name"`
	SourceBomVersionID     int64   `json:"source_bom_version_id"`
	SourceBomVersionNo     string  `json:"source_bom_version_no"`
	DerivedFromLabel       string  `json:"derived_from_label"`
	CanEditBOM             bool    `json:"can_edit_bom"`
	ProductionBomID        int64   `json:"production_bom_id"`
	ProductionBomCode      string  `json:"production_bom_code"`
	ProductionBomName      string  `json:"production_bom_name"`
	ProductionBomVersionID int64   `json:"production_bom_version_id"`
	ProductionBomVersionNo string  `json:"production_bom_version_no"`
	LatestBomVersionID     int64   `json:"latest_bom_version_id"`
	LatestBomVersionNo     string  `json:"latest_bom_version_no"`
	IsLatestBomVersion     bool    `json:"is_latest_bom_version"`
	ProductionBomGroupID   int64   `json:"production_bom_group_id"`
	ProductionBomGroupName string  `json:"production_bom_group_name"`
}

type Option struct {
	ID              int64   `json:"id"`
	ProductCode     string  `json:"product_code,omitempty"`
	Name            string  `json:"name"`
	CustomerID      int64   `json:"customer_id"`
	InventoryUnit   string  `json:"inventory_unit,omitempty"`
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

type ProductionBomGroup struct {
	ID         int64                        `json:"id"`
	Name       string                       `json:"name"`
	SortOrder  int                          `json:"sort_order"`
	Active     bool                         `json:"active"`
	Categories []ProductionBomGroupCategory `json:"categories,omitempty"`
}

type ProductionBomGroupCategory struct {
	ID        int64  `json:"id"`
	GroupID   int64  `json:"group_id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

type ProductionBomSummary struct {
	ID                    int64   `json:"id"`
	Code                  string  `json:"code"`
	Name                  string  `json:"name"`
	OutputProductID       int64   `json:"output_product_id"`
	OutputProductName     string  `json:"output_product_name"`
	OutputProductCode     string  `json:"output_product_code"`
	BusinessGroupID       int64   `json:"business_group_id"`
	BusinessGroupName     string  `json:"business_group_name"`
	GroupItemID           int64   `json:"group_item_id"`
	GroupItemName         string  `json:"group_item_name"`
	GroupID               int64   `json:"group_id"`
	GroupName             string  `json:"group_name"`
	GroupCategoryID       int64   `json:"group_category_id"`
	GroupCategoryName     string  `json:"group_category_name"`
	Status                string  `json:"status"`
	LatestVersionID       int64   `json:"latest_version_id"`
	LatestVersionNo       string  `json:"latest_version_no"`
	LatestVersionStatus   string  `json:"latest_version_status"`
	ProcessRouteID        int64   `json:"process_route_id"`
	ProcessRouteName      string  `json:"process_route_name"`
	IsLatestUsable        bool    `json:"is_latest_usable"`
	ExpectedYieldRate     float64 `json:"expected_yield_rate"`
	ExpectedLossRate      float64 `json:"expected_loss_rate"`
	ReferenceProductCount int     `json:"reference_product_count"`
	UpdatedAt             string  `json:"updated_at"`
}

type ProductionBomDetail struct {
	ProductionBomSummary
	Versions           []ProductionBomVersion           `json:"versions"`
	Items              []Item                           `json:"items"`
	ReferencedProducts []ProductionBomReferencedProduct `json:"referenced_products"`
	UsedByBoms         []ProductionBomUsedByBom         `json:"used_by_boms"`
}

type ProductionBomReferencedProduct struct {
	ProductID    int64  `json:"product_id"`
	ProductName  string `json:"product_name"`
	ProductCode  string `json:"product_code"`
	Active       bool   `json:"active"`
	BomVersionID int64  `json:"bom_version_id"`
	BomVersionNo string `json:"bom_version_no"`
}

type ProductionBomUsedByBom struct {
	BomID                     int64   `json:"bom_id"`
	BomCode                   string  `json:"bom_code"`
	BomName                   string  `json:"bom_name"`
	BomVersionID              int64   `json:"bom_version_id"`
	BomVersionNo              string  `json:"bom_version_no"`
	OutputProductID           int64   `json:"output_product_id"`
	OutputProductName         string  `json:"output_product_name"`
	BomStatus                 string  `json:"bom_status"`
	IsDefault                 bool    `json:"is_default"`
	CanSetDefault             bool    `json:"can_set_default"`
	CurrentPublishedVersionID int64   `json:"current_published_version_id"`
	CurrentPublishedVersionNo string  `json:"current_published_version_no"`
	RelationType              string  `json:"relation_type"`
	ConsumeUnit               string  `json:"consume_unit"`
	QtyPerUnit                float64 `json:"qty_per_unit"`
}

type ProductionBomVersion struct {
	ID                     int64   `json:"id"`
	BomID                  int64   `json:"bom_id"`
	VersionNo              string  `json:"version_no"`
	Status                 string  `json:"status"`
	YieldRate              float64 `json:"yield_rate"`
	ExpectedYieldRate      float64 `json:"expected_yield_rate"`
	ExpectedLossRate       float64 `json:"expected_loss_rate"`
	OutputQty              float64 `json:"output_qty"`
	OutputUnit             string  `json:"output_unit"`
	ItemCount              int     `json:"item_count"`
	Note                   string  `json:"note"`
	SpecialAttrsSchemaJSON string  `json:"special_attrs_schema_json"`
	SpecialAttrsJSON       string  `json:"special_attrs_json"`
	ProcessRouteID         int64   `json:"process_route_id"`
	ProcessRouteName       string  `json:"process_route_name"`
	CreatedAt              string  `json:"created_at"`
	PublishedAt            string  `json:"published_at"`
	IsLatest               bool    `json:"is_latest"`
	IsLatestUsable         bool    `json:"is_latest_usable"`
}

type ProductProductionBomBinding struct {
	ProductID            int64  `json:"product_id"`
	BomID                int64  `json:"production_bom_id"`
	BomCode              string `json:"production_bom_code"`
	BomName              string `json:"production_bom_name"`
	BomVersionID         int64  `json:"production_bom_version_id"`
	BomVersionNo         string `json:"production_bom_version_no"`
	LatestBomVersionID   int64  `json:"latest_bom_version_id"`
	LatestBomVersionNo   string `json:"latest_bom_version_no"`
	IsLatestBomVersion   bool   `json:"is_latest_bom_version"`
	ProductionBomGroupID int64  `json:"production_bom_group_id"`
	ProductionBomGroup   string `json:"production_bom_group_name"`
}

type CreateProductionBomGroupCommand struct {
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
	Actor     string `json:"actor"`
}

type UpdateProductionBomGroupCommand struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
	Actor     string `json:"actor"`
}

type DeleteProductionBomGroupCommand struct {
	ID    int64  `json:"id"`
	Actor string `json:"actor"`
}

type MoveProductionBomGroupCommand struct {
	ID        int64  `json:"id"`
	SortOrder int    `json:"sort_order"`
	Actor     string `json:"actor"`
}

type CreateProductionBomGroupCategoryCommand struct {
	GroupID   int64  `json:"group_id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
	Actor     string `json:"actor"`
}

type UpdateProductionBomGroupCategoryCommand struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
	Actor     string `json:"actor"`
}

type DeleteProductionBomGroupCategoryCommand struct {
	ID    int64  `json:"id"`
	Actor string `json:"actor"`
}

type CreateProductionBomCommand struct {
	Name             string   `json:"name"`
	OutputProductID  int64    `json:"output_product_id"`
	OutputQty        float64  `json:"output_qty"`
	OutputUnit       string   `json:"output_unit"`
	GroupID          int64    `json:"group_id"`
	GroupCategoryID  int64    `json:"group_category_id"`
	ExpectedLossRate *float64 `json:"expected_loss_rate,omitempty"`
	Actor            string   `json:"actor"`
}

type UpdateProductionBomCommand struct {
	ID                    int64  `json:"id"`
	Name                  string `json:"name"`
	OutputProductID       int64  `json:"output_product_id"`
	GroupID               int64  `json:"group_id"`
	GroupCategoryID       int64  `json:"group_category_id"`
	UpdateGroupAssignment bool   `json:"-"`
	Status                string `json:"status"`
	Actor                 string `json:"actor"`
}

type CopyProductionBomCommand struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	OutputProductID int64  `json:"output_product_id"`
	GroupID         int64  `json:"group_id"`
	GroupCategoryID int64  `json:"group_category_id"`
	Actor           string `json:"actor"`
}

type CreateProductionBomVersionCommand struct {
	BomID           int64  `json:"bom_id"`
	SourceVersionID int64  `json:"source_version_id"`
	Note            string `json:"note"`
	Actor           string `json:"actor"`
}

type ProductionBomDraftItem struct {
	MaterialID         int64   `json:"material_id"`
	ComponentType      string  `json:"component_type"`
	ComponentProductID int64   `json:"component_product_id"`
	ComponentSpecG     int64   `json:"component_spec_g"`
	ConsumeUnit        string  `json:"consume_unit"`
	QtyPerUnit         float64 `json:"qty_per_unit"`
	RatioPct           float64 `json:"ratio_pct"`
}

type UpdateProductionBomVersionDraftCommand struct {
	VersionID              int64                    `json:"version_id"`
	ExpectedLossRate       *float64                 `json:"expected_loss_rate,omitempty"`
	OutputQty              float64                  `json:"output_qty"`
	OutputUnit             string                   `json:"output_unit"`
	ProcessRouteID         int64                    `json:"process_route_id"`
	Items                  []ProductionBomDraftItem `json:"items"`
	SpecialAttrsSchemaJSON string                   `json:"special_attrs_schema_json"`
	SpecialAttrsJSON       string                   `json:"special_attrs_json"`
	Actor                  string                   `json:"actor"`
}

type PublishProductionBomVersionCommand struct {
	VersionID int64  `json:"version_id"`
	Actor     string `json:"actor"`
}

type BindProductProductionBomCommand struct {
	ProductID              int64  `json:"product_id"`
	BomID                  int64  `json:"bom_id"`
	BomVersionID           int64  `json:"bom_version_id"`
	DefaultProductionBomID int64  `json:"default_production_bom_id"`
	Actor                  string `json:"actor"`
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
	SetBomSource(ctx context.Context, cmd SetBomSourceCommand) (Detail, error)
	ListProductionBomGroups(ctx context.Context, includeInactive bool) ([]ProductionBomGroup, error)
	CreateProductionBomGroup(ctx context.Context, cmd CreateProductionBomGroupCommand) (ProductionBomGroup, error)
	UpdateProductionBomGroup(ctx context.Context, cmd UpdateProductionBomGroupCommand) (ProductionBomGroup, error)
	DeleteProductionBomGroup(ctx context.Context, cmd DeleteProductionBomGroupCommand) error
	MoveProductionBomGroup(ctx context.Context, cmd MoveProductionBomGroupCommand) error
	CreateProductionBomGroupCategory(ctx context.Context, cmd CreateProductionBomGroupCategoryCommand) (ProductionBomGroupCategory, error)
	UpdateProductionBomGroupCategory(ctx context.Context, cmd UpdateProductionBomGroupCategoryCommand) (ProductionBomGroupCategory, error)
	DeleteProductionBomGroupCategory(ctx context.Context, cmd DeleteProductionBomGroupCategoryCommand) error
	ListProductionBoms(ctx context.Context) ([]ProductionBomSummary, error)
	GetProductionBomDetail(ctx context.Context, id int64, versionID int64) (ProductionBomDetail, error)
	ListProductionBomUsageByProduct(ctx context.Context, productID int64) ([]ProductionBomUsedByBom, error)
	CreateProductionBom(ctx context.Context, cmd CreateProductionBomCommand) (ProductionBomSummary, error)
	UpdateProductionBom(ctx context.Context, cmd UpdateProductionBomCommand) (ProductionBomSummary, error)
	CopyProductionBom(ctx context.Context, cmd CopyProductionBomCommand) (ProductionBomSummary, error)
	CreateProductionBomVersion(ctx context.Context, cmd CreateProductionBomVersionCommand) (ProductionBomVersion, error)
	UpdateProductionBomVersionDraft(ctx context.Context, cmd UpdateProductionBomVersionDraftCommand) (ProductionBomVersion, error)
	ValidateProductionBomVersionForPublish(ctx context.Context, cmd PublishProductionBomVersionCommand) error
	PublishProductionBomVersion(ctx context.Context, cmd PublishProductionBomVersionCommand) error
	BindProductProductionBom(ctx context.Context, cmd BindProductProductionBomCommand) (ProductProductionBomBinding, error)
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

func (s *Service) SetBomSource(ctx context.Context, cmd SetBomSourceCommand) (Detail, error) {
	if cmd.ProductID <= 0 {
		return Detail{}, fmt.Errorf("product_id required")
	}
	if cmd.SourceType != "inherit_current" && cmd.SourceType != "inherit_version" && cmd.SourceType != "owned" {
		return Detail{}, fmt.Errorf("source_type must be inherit_current, inherit_version, or owned")
	}
	if cmd.SourceType == "inherit_version" && cmd.SourceBomVersionID <= 0 {
		return Detail{}, fmt.Errorf("source_bom_version_id required for inherit_version")
	}
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	detail, err := s.repo.SetBomSource(ctx, cmd)
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

func enrichProductionBomSummaryYield(row *ProductionBomSummary) {
	row.ExpectedYieldRate = productiondomain.NormalizeYieldRate(row.ExpectedYieldRate)
	if row.ExpectedYieldRate <= 0 {
		row.ExpectedYieldRate = productiondomain.NormalizeYieldRate(1 - row.ExpectedLossRate)
	}
	row.ExpectedLossRate = productiondomain.ExpectedLossRate(row.ExpectedYieldRate)
}

func normalizeProductionBomSummaryGroups(row *ProductionBomSummary) {
	if row.BusinessGroupID <= 0 {
		row.BusinessGroupID = row.GroupID
	}
	if row.BusinessGroupName == "" {
		row.BusinessGroupName = row.GroupName
	}
	if row.GroupItemID <= 0 {
		row.GroupItemID = row.GroupCategoryID
	}
	if row.GroupItemName == "" {
		row.GroupItemName = row.GroupCategoryName
	}
	if row.GroupID <= 0 {
		row.GroupID = row.BusinessGroupID
	}
	if row.GroupName == "" {
		row.GroupName = row.BusinessGroupName
	}
	if row.GroupCategoryID <= 0 {
		row.GroupCategoryID = row.GroupItemID
	}
	if row.GroupCategoryName == "" {
		row.GroupCategoryName = row.GroupItemName
	}
}

func enrichProductionBomVersionYield(row *ProductionBomVersion) {
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

func (s *Service) ListProductionBomGroups(ctx context.Context, includeInactive bool) ([]ProductionBomGroup, error) {
	return s.repo.ListProductionBomGroups(ctx, includeInactive)
}

func (s *Service) CreateProductionBomGroup(ctx context.Context, cmd CreateProductionBomGroupCommand) (ProductionBomGroup, error) {
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.Name == "" {
		return ProductionBomGroup{}, fmt.Errorf("name required")
	}
	return s.repo.CreateProductionBomGroup(ctx, cmd)
}

func (s *Service) UpdateProductionBomGroup(ctx context.Context, cmd UpdateProductionBomGroupCommand) (ProductionBomGroup, error) {
	if cmd.ID <= 0 {
		return ProductionBomGroup{}, fmt.Errorf("group_id required")
	}
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.Name == "" {
		return ProductionBomGroup{}, fmt.Errorf("name required")
	}
	return s.repo.UpdateProductionBomGroup(ctx, cmd)
}

func (s *Service) DeleteProductionBomGroup(ctx context.Context, cmd DeleteProductionBomGroupCommand) error {
	if cmd.ID <= 0 {
		return fmt.Errorf("group_id required")
	}
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	return s.repo.DeleteProductionBomGroup(ctx, cmd)
}

func (s *Service) MoveProductionBomGroup(ctx context.Context, cmd MoveProductionBomGroupCommand) error {
	if cmd.ID <= 0 {
		return fmt.Errorf("group_id required")
	}
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	return s.repo.MoveProductionBomGroup(ctx, cmd)
}

func (s *Service) CreateProductionBomGroupCategory(ctx context.Context, cmd CreateProductionBomGroupCategoryCommand) (ProductionBomGroupCategory, error) {
	if cmd.GroupID <= 0 {
		return ProductionBomGroupCategory{}, fmt.Errorf("group_id required")
	}
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.Name == "" {
		return ProductionBomGroupCategory{}, fmt.Errorf("name required")
	}
	return s.repo.CreateProductionBomGroupCategory(ctx, cmd)
}

func (s *Service) UpdateProductionBomGroupCategory(ctx context.Context, cmd UpdateProductionBomGroupCategoryCommand) (ProductionBomGroupCategory, error) {
	if cmd.ID <= 0 {
		return ProductionBomGroupCategory{}, fmt.Errorf("category_id required")
	}
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.Name == "" {
		return ProductionBomGroupCategory{}, fmt.Errorf("name required")
	}
	return s.repo.UpdateProductionBomGroupCategory(ctx, cmd)
}

func (s *Service) DeleteProductionBomGroupCategory(ctx context.Context, cmd DeleteProductionBomGroupCategoryCommand) error {
	if cmd.ID <= 0 {
		return fmt.Errorf("category_id required")
	}
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	return s.repo.DeleteProductionBomGroupCategory(ctx, cmd)
}

func (s *Service) ListProductionBoms(ctx context.Context) ([]ProductionBomSummary, error) {
	rows, err := s.repo.ListProductionBoms(ctx)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		normalizeProductionBomSummaryGroups(&rows[i])
		enrichProductionBomSummaryYield(&rows[i])
	}
	return rows, nil
}

func (s *Service) GetProductionBomDetail(ctx context.Context, id int64, versionID int64) (ProductionBomDetail, error) {
	if id <= 0 {
		return ProductionBomDetail{}, fmt.Errorf("bom_id required")
	}
	row, err := s.repo.GetProductionBomDetail(ctx, id, versionID)
	if err != nil {
		return ProductionBomDetail{}, err
	}
	normalizeProductionBomSummaryGroups(&row.ProductionBomSummary)
	enrichProductionBomSummaryYield(&row.ProductionBomSummary)
	for i := range row.Versions {
		enrichProductionBomVersionYield(&row.Versions[i])
	}
	return row, nil
}

func (s *Service) ListProductionBomUsageByProduct(ctx context.Context, productID int64) ([]ProductionBomUsedByBom, error) {
	if productID <= 0 {
		return nil, fmt.Errorf("product_id required")
	}
	return s.repo.ListProductionBomUsageByProduct(ctx, productID)
}

func (s *Service) CreateProductionBom(ctx context.Context, cmd CreateProductionBomCommand) (ProductionBomSummary, error) {
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.OutputUnit = strings.TrimSpace(cmd.OutputUnit)
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.Name == "" {
		return ProductionBomSummary{}, fmt.Errorf("name required")
	}
	if cmd.OutputProductID <= 0 {
		return ProductionBomSummary{}, fmt.Errorf("output_product_id required")
	}
	if cmd.OutputQty <= 0 {
		cmd.OutputQty = 1
	}
	cmd.OutputUnit = s.deriveProductionBomOutputUnit(ctx, cmd.OutputProductID, cmd.OutputUnit)
	row, err := s.repo.CreateProductionBom(ctx, cmd)
	if err != nil {
		return ProductionBomSummary{}, err
	}
	normalizeProductionBomSummaryGroups(&row)
	enrichProductionBomSummaryYield(&row)
	return row, nil
}

func (s *Service) deriveProductionBomOutputUnit(ctx context.Context, outputProductID int64, fallback string) string {
	fallback = strings.TrimSpace(fallback)
	if outputProductID <= 0 {
		if fallback != "" {
			return fallback
		}
		return "unit"
	}
	rows, err := s.repo.Products(ctx)
	if err != nil {
		if fallback != "" {
			return fallback
		}
		return "unit"
	}
	for _, row := range rows {
		if row.ID == outputProductID {
			return outputProductInventoryUnit(row, fallback)
		}
	}
	if fallback != "" {
		return fallback
	}
	return "unit"
}

func outputProductInventoryUnit(product Option, fallback string) string {
	if unit := strings.TrimSpace(product.InventoryUnit); unit != "" {
		return unit
	}
	if fallback = strings.TrimSpace(fallback); fallback != "" {
		return fallback
	}
	return "unit"
}

func (s *Service) UpdateProductionBom(ctx context.Context, cmd UpdateProductionBomCommand) (ProductionBomSummary, error) {
	if cmd.ID <= 0 {
		return ProductionBomSummary{}, fmt.Errorf("bom_id required")
	}
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Status = strings.TrimSpace(cmd.Status)
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	row, err := s.repo.UpdateProductionBom(ctx, cmd)
	if err != nil {
		return ProductionBomSummary{}, err
	}
	normalizeProductionBomSummaryGroups(&row)
	enrichProductionBomSummaryYield(&row)
	return row, nil
}

func (s *Service) CopyProductionBom(ctx context.Context, cmd CopyProductionBomCommand) (ProductionBomSummary, error) {
	if cmd.ID <= 0 {
		return ProductionBomSummary{}, fmt.Errorf("bom_id required")
	}
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	row, err := s.repo.CopyProductionBom(ctx, cmd)
	if err != nil {
		return ProductionBomSummary{}, err
	}
	normalizeProductionBomSummaryGroups(&row)
	enrichProductionBomSummaryYield(&row)
	return row, nil
}

func (s *Service) CreateProductionBomVersion(ctx context.Context, cmd CreateProductionBomVersionCommand) (ProductionBomVersion, error) {
	if cmd.BomID <= 0 {
		return ProductionBomVersion{}, fmt.Errorf("bom_id required")
	}
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	row, err := s.repo.CreateProductionBomVersion(ctx, cmd)
	if err != nil {
		return ProductionBomVersion{}, err
	}
	enrichProductionBomVersionYield(&row)
	return row, nil
}

func (s *Service) UpdateProductionBomVersionDraft(ctx context.Context, cmd UpdateProductionBomVersionDraftCommand) (ProductionBomVersion, error) {
	if cmd.VersionID <= 0 {
		return ProductionBomVersion{}, fmt.Errorf("version_id required")
	}
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	cmd.OutputUnit = strings.TrimSpace(cmd.OutputUnit)
	if cmd.OutputQty < 0 {
		return ProductionBomVersion{}, fmt.Errorf("output_qty must be positive")
	}
	if cmd.OutputQty > 0 && cmd.OutputUnit == "" {
		cmd.OutputUnit = "unit"
	}
	if cmd.Items != nil {
		for i := range cmd.Items {
			item, err := normalizeProductionBomDraftItem(cmd.Items[i])
			if err != nil {
				return ProductionBomVersion{}, err
			}
			cmd.Items[i] = item
		}
	}
	if strings.TrimSpace(cmd.SpecialAttrsSchemaJSON) != "" {
		schemaJSON, err := normalizeJSONArrayText(cmd.SpecialAttrsSchemaJSON)
		if err != nil {
			return ProductionBomVersion{}, fmt.Errorf("invalid special_attrs_schema_json")
		}
		cmd.SpecialAttrsSchemaJSON = schemaJSON
	}
	if strings.TrimSpace(cmd.SpecialAttrsJSON) != "" {
		attrsJSON, err := normalizeJSONObjectText(cmd.SpecialAttrsJSON)
		if err != nil {
			return ProductionBomVersion{}, fmt.Errorf("invalid special_attrs_json")
		}
		cmd.SpecialAttrsJSON = attrsJSON
	}
	row, err := s.repo.UpdateProductionBomVersionDraft(ctx, cmd)
	if err != nil {
		return ProductionBomVersion{}, err
	}
	enrichProductionBomVersionYield(&row)
	return row, nil
}

func normalizeJSONArrayText(input string) (string, error) {
	text := strings.TrimSpace(input)
	if text == "" {
		return "[]", nil
	}
	var v any
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		return "", err
	}
	if _, ok := v.([]any); !ok {
		return "", fmt.Errorf("json array required")
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func normalizeJSONObjectText(input string) (string, error) {
	text := strings.TrimSpace(input)
	if text == "" {
		return "{}", nil
	}
	var v any
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		return "", err
	}
	if _, ok := v.(map[string]any); !ok {
		return "", fmt.Errorf("json object required")
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (s *Service) PublishProductionBomVersion(ctx context.Context, cmd PublishProductionBomVersionCommand) error {
	if cmd.VersionID <= 0 {
		return fmt.Errorf("version_id required")
	}
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if err := s.repo.ValidateProductionBomVersionForPublish(ctx, cmd); err != nil {
		return err
	}
	return s.repo.PublishProductionBomVersion(ctx, cmd)
}

func normalizeProductionBomDraftItem(item ProductionBomDraftItem) (ProductionBomDraftItem, error) {
	componentType := strings.TrimSpace(item.ComponentType)
	if componentType == "" {
		componentType = "material"
	}
	if componentType == "finished_product" {
		componentType = "product"
	}
	if componentType != "material" && componentType != "product" {
		return item, fmt.Errorf("invalid component_type")
	}
	consumeUnit := strings.TrimSpace(item.ConsumeUnit)
	if consumeUnit == "" {
		if componentType == "product" {
			consumeUnit = "unit_per_box"
		} else {
			consumeUnit = "ratio_pct"
		}
	}
	switch consumeUnit {
	case "ratio_pct", "g_per_bag", "unit_per_bag", "unit_per_box", "fixed_qty", "unit", "g", "kg", "length", "area":
	default:
		return item, fmt.Errorf("invalid consume_unit")
	}
	switch componentType {
	case "material":
		if item.MaterialID <= 0 {
			return item, fmt.Errorf("material_id required")
		}
	case "product":
		item.MaterialID = 0
		if item.ComponentProductID <= 0 {
			return item, fmt.Errorf("component_product_id required")
		}
		if consumeUnit == "ratio_pct" {
			return item, fmt.Errorf("product consume_unit must not be ratio_pct")
		}
	}
	if consumeUnit == "ratio_pct" {
		if item.RatioPct <= 0 || item.RatioPct > 100 {
			return item, fmt.Errorf("ratio must be (0,100]")
		}
		item.QtyPerUnit = 0
	} else if item.QtyPerUnit <= 0 {
		return item, fmt.Errorf("qty_per_unit required")
	}
	item.ComponentType = componentType
	item.ConsumeUnit = consumeUnit
	return item, nil
}

func (s *Service) BindProductProductionBom(ctx context.Context, cmd BindProductProductionBomCommand) (ProductProductionBomBinding, error) {
	if cmd.ProductID <= 0 {
		return ProductProductionBomBinding{}, fmt.Errorf("product_id required")
	}
	if cmd.BomID <= 0 && cmd.DefaultProductionBomID > 0 {
		cmd.BomID = cmd.DefaultProductionBomID
	}
	if cmd.BomID <= 0 {
		return ProductProductionBomBinding{}, fmt.Errorf("bom_id required")
	}
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	return s.repo.BindProductProductionBom(ctx, cmd)
}

type SetBomSourceCommand struct {
	ProductID          int64  `json:"product_id"`
	SourceType         string `json:"source_type"`
	SourceBomVersionID int64  `json:"source_bom_version_id"`
	Actor              string `json:"actor"`
}
