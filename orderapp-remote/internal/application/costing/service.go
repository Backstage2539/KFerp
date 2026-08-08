package costing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	domain "orderapp/internal/domain/costing"
)

var (
	ErrBeanListPublicationNotFound       = errors.New("bean list publication not found")
	ErrProductPricingRuleNotFound        = errors.New("product pricing rule not found")
	ErrProductSalesUnitRuleNotFound      = errors.New("product sales unit rule not found")
	ErrProductSpecIdentityNotFound       = errors.New("product spec identity not found")
	ErrPriceTierTemplateUnitRuleNotFound = errors.New("price tier template unit rule not found")
	priceTierMassUnitPattern             = regexp.MustCompile(`^(?:\d+(?:\.\d+)?)?(kg|kgs|公斤|千克|g|克|lb|lbs|磅)(?:袋装)?$`)
)

const (
	BeanListPublicationPurposeFactorySupply  = "factory_supply"
	BeanListPublicationPurposeCustomerResale = "customer_resale"
)

const (
	pricingRuleTrialBomOperationSnapshotMissingSource  = "bom_operation_snapshot_missing"
	pricingRuleTrialBomOperationSnapshotMissingWarning = "请先发布包含标准成本产能档快照的 BOM"
	pricingRuleTrialMissingPublishedBomError           = "该商品未配置可用于试算的已发布生产 BOM，无法计算标准制造成本；请到 生产管理 → 生产 BOM 新增或发布 BOM 后再试算"
)

type CalculateRequest struct {
	Products []domain.ProductInput `json:"products"`
}

type PriceExplanationCommand struct {
	Product   domain.ProductInput              `json:"product"`
	TierLabel string                           `json:"tier_label"`
	Overrides domain.PriceExplanationOverrides `json:"overrides,omitempty"`
}

type ProductPricingRule struct {
	ID              int64          `json:"id"`
	Name            string         `json:"name"`
	Code            string         `json:"code,omitempty"`
	CostSourceMode  string         `json:"cost_source_mode"`
	MarginRate      float64        `json:"margin_rate"`
	TaxRate         float64        `json:"tax_rate"`
	RoundingMode    string         `json:"rounding_mode"`
	CalculationJSON map[string]any `json:"calculation_json"`
	FormulaVersion  string         `json:"formula_version"`
	Active          bool           `json:"active"`
	Remark          string         `json:"remark,omitempty"`
}

type ProductSalesUnitRule struct {
	ProductID          int64                         `json:"product_id"`
	SKUName            string                        `json:"sku_name,omitempty"`
	SKUCode            string                        `json:"sku_code,omitempty"`
	Barcode            string                        `json:"barcode,omitempty"`
	DefaultSalesUnit   string                        `json:"default_sales_unit"`
	InventoryUnit      string                        `json:"inventory_unit"`
	Conversion         map[string]map[string]float64 `json:"unit_conversion_json"`
	EffectiveSalesSpec *domain.EffectiveSalesSpec    `json:"effective_sales_spec,omitempty"`
}

// ProductSpecIdentity is the current product-master identity used to validate
// a price-list selection. EffectiveParentProductID equals ProductID for a
// historical parent-only product and otherwise points at the owning parent.
type ProductSpecIdentity struct {
	ProductID                int64
	EffectiveParentProductID int64
	ParentProductName        string
	Active                   bool
	SpecValid                bool
}

type PriceTierTemplateUnitRule struct {
	TemplateID   int64
	TemplateName string
	TierUnits    map[int64]string
}

type PricingRuleTrialCommand struct {
	PricingRuleID       int64                     `json:"pricing_rule_id"`
	ProductID           int64                     `json:"product_id"`
	CustomerID          int64                     `json:"customer_id,omitempty"`
	BomVersionID        int64                     `json:"bom_version_id,omitempty"`
	ProcessRouteID      int64                     `json:"process_route_id,omitempty"`
	OperationTemplateID int64                     `json:"operation_template_id,omitempty"`
	QuoteUnit           string                    `json:"quote_unit,omitempty"`
	Overrides           PricingRuleTrialOverrides `json:"overrides,omitempty"`
}

type PricingRuleTrialBatchRow struct {
	Index  int                     `json:"index"`
	Result *PricingRuleTrialResult `json:"result,omitempty"`
	Error  string                  `json:"error,omitempty"`
}

type PricingRuleTrialOverrides struct {
	ExpectedLossRate *float64           `json:"expected_loss_rate,omitempty"`
	BaseCost         *float64           `json:"base_cost,omitempty"`
	MarginRate       *float64           `json:"margin_rate,omitempty"`
	TaxRate          *float64           `json:"tax_rate,omitempty"`
	OtherCosts       map[string]float64 `json:"other_costs,omitempty"`
	PostMarkupCosts  map[string]float64 `json:"post_markup_costs,omitempty"`
}

type PricingRuleTrialProductionOptions struct {
	BomVersions        []PricingRuleTrialBomVersionOption        `json:"bom_versions,omitempty"`
	ProcessRoutes      []PricingRuleTrialProcessRouteOption      `json:"process_routes,omitempty"`
	OperationTemplates []PricingRuleTrialOperationTemplateOption `json:"operation_templates,omitempty"`
	loaded             bool
}

type PricingRuleTrialBomVersionOption struct {
	BomID                        int64  `json:"bom_id"`
	BomCode                      string `json:"bom_code,omitempty"`
	BomName                      string `json:"bom_name"`
	VersionID                    int64  `json:"version_id"`
	VersionNo                    string `json:"version_no"`
	Status                       string `json:"status"`
	IsDefault                    bool   `json:"is_default"`
	ProcessRouteID               int64  `json:"process_route_id,omitempty"`
	ProcessRouteName             string `json:"process_route_name,omitempty"`
	ComponentCount               int    `json:"component_count"`
	LatestNonEmptyDraftVersionID int64  `json:"latest_nonempty_draft_version_id,omitempty"`
	LatestNonEmptyDraftVersionNo string `json:"latest_nonempty_draft_version_no,omitempty"`
}

type PricingRuleTrialProcessRouteOption struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
}

type PricingRuleTrialOperationTemplateOption struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
}

const pricingRuleTrialStandardManufacturingCostSource = "standard_manufacturing_cost" // cost_source = standard_manufacturing_cost

type PricingRuleTrialBomSnapshot struct {
	VersionID int64  `json:"version_id,omitempty"`
	VersionNo string `json:"version_no,omitempty"`
	UsageMode string `json:"usage_mode,omitempty"`
	Status    string `json:"status,omitempty"`
}

type PricingRuleTrialProcessRouteSnapshot struct {
	ID   int64  `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type PricingRuleTrialWorkstationCostSnapshot struct {
	MaterialUnitCost              float64                                      `json:"material_unit_cost"`
	OperationUnitCost             float64                                      `json:"operation_unit_cost"`
	StandardManufacturingUnitCost float64                                      `json:"standard_manufacturing_unit_cost"`
	OperationRows                 []PricingRuleTrialWorkstationCostSnapshotRow `json:"operation_rows,omitempty"`
}

type PricingRuleTrialWorkstationCostSnapshotRow struct {
	OperationName      string  `json:"operation_name,omitempty"`
	WorkstationName    string  `json:"workstation_name,omitempty"`
	CapacityName       string  `json:"capacity_name,omitempty"`
	CostMethod         string  `json:"cost_method,omitempty"`
	PieceRate          float64 `json:"piece_rate,omitempty"`
	RateUnit           string  `json:"rate_unit,omitempty"`
	HourlyRate         float64 `json:"hourly_rate,omitempty"`
	StandardMinutes    float64 `json:"standard_minutes,omitempty"`
	StandardOutputQty  float64 `json:"standard_output_qty,omitempty"`
	StandardOutputUnit string  `json:"standard_output_unit,omitempty"`
	UnitCost           float64 `json:"unit_cost,omitempty"`
	Unit               string  `json:"unit,omitempty"`
}

type PricingRuleTrialResult struct {
	PricingRuleID                 int64                                     `json:"pricing_rule_id"`
	PricingRuleName               string                                    `json:"pricing_rule_name"`
	FormulaVersion                string                                    `json:"formula_version"`
	ProductID                     int64                                     `json:"product_id"`
	ProductName                   string                                    `json:"product_name"`
	QuoteUnit                     string                                    `json:"quote_unit"`
	InventoryUnit                 string                                    `json:"inventory_unit"`
	BomVersionID                  int64                                     `json:"bom_version_id,omitempty"`
	BomVersionNo                  string                                    `json:"bom_version_no,omitempty"`
	BomVersionOptions             []PricingRuleTrialBomVersionOption        `json:"bom_version_options,omitempty"`
	ProcessRouteID                int64                                     `json:"process_route_id,omitempty"`
	ProcessRouteName              string                                    `json:"process_route_name,omitempty"`
	ProcessRouteOptions           []PricingRuleTrialProcessRouteOption      `json:"process_route_options,omitempty"`
	OperationTemplateID           int64                                     `json:"operation_template_id,omitempty"`
	OperationTemplateName         string                                    `json:"operation_template_name,omitempty"`
	OperationTemplateOptions      []PricingRuleTrialOperationTemplateOption `json:"operation_template_options,omitempty"`
	BomUsageMode                  string                                    `json:"bom_usage_mode,omitempty"`
	BomStatus                     string                                    `json:"bom_status,omitempty"`
	BaseCost                      float64                                   `json:"base_cost"`
	BomCostTotal                  float64                                   `json:"bom_cost_total"`
	OperationCostTotal            float64                                   `json:"operation_cost_total"`
	MaterialUnitCost              float64                                   `json:"material_unit_cost"`
	OperationUnitCost             float64                                   `json:"operation_unit_cost"`
	StandardManufacturingUnitCost float64                                   `json:"standard_manufacturing_unit_cost"`
	CostSource                    string                                    `json:"cost_source,omitempty"`
	BomSnapshot                   PricingRuleTrialBomSnapshot               `json:"bom_snapshot,omitempty"`
	ProcessRouteSnapshot          PricingRuleTrialProcessRouteSnapshot      `json:"process_route_snapshot,omitempty"`
	WorkstationCostSnapshot       PricingRuleTrialWorkstationCostSnapshot   `json:"workstation_cost_snapshot,omitempty"`
	BaseCostDetails               []PricingRuleTrialBaseCostDetail          `json:"base_cost_details,omitempty"`
	OtherCostTotal                float64                                   `json:"other_cost_total"`
	OtherCostDetails              []PricingRuleTrialOtherCostDetail         `json:"other_cost_details,omitempty"`
	CostBaseTotal                 float64                                   `json:"cost_base_total"`
	CostAfterYield                float64                                   `json:"cost_after_yield"`
	YieldLossAmount               float64                                   `json:"yield_loss_amount"`
	PriceAfterMarkup              float64                                   `json:"price_after_markup,omitempty"`
	ProfitMarkupAmount            float64                                   `json:"profit_markup_amount"`
	ProfitExplanation             PricingRuleTrialProfitExplanation         `json:"profit_explanation"`
	PostMarkupCostTotal           float64                                   `json:"post_markup_cost_total,omitempty"`
	PreTaxPrice                   float64                                   `json:"pre_tax_price"`
	TaxAmount                     float64                                   `json:"tax_amount"`
	TaxInPriceAmount              float64                                   `json:"tax_in_price_amount"`
	TaxRateSource                 string                                    `json:"tax_rate_source,omitempty"`
	FinalBeforeRounding           float64                                   `json:"final_before_rounding"`
	RoundingAdjustment            float64                                   `json:"rounding_adjustment"`
	RoundingRuleSource            string                                    `json:"rounding_rule_source,omitempty"`
	FinalUnitPrice                float64                                   `json:"final_unit_price"`
	GrossMarginRate               float64                                   `json:"gross_margin_rate"`
	MinimumMarginRate             float64                                   `json:"minimum_margin_rate"`
	FormulaExpression             string                                    `json:"formula_expression,omitempty"`
	FormulaExpressionLines        []string                                  `json:"formula_expression_lines,omitempty"`
	Steps                         []domain.PriceExplanationStep             `json:"steps"`
	Warnings                      []string                                  `json:"warnings,omitempty"`
}

type PricingRuleTrialBaseCostDetail struct {
	Key                     string  `json:"key,omitempty"`
	Type                    string  `json:"type"`
	TypeLabel               string  `json:"type_label"`
	Name                    string  `json:"name"`
	ConsumeUnit             string  `json:"consume_unit,omitempty"`
	Quantity                float64 `json:"quantity,omitempty"`
	RatioPct                float64 `json:"ratio_pct,omitempty"`
	RecipeRatioPct          float64 `json:"recipe_ratio_pct,omitempty"`
	EffectiveRatioPct       float64 `json:"effective_ratio_pct,omitempty"`
	MaterialLossRate        float64 `json:"material_loss_rate,omitempty"`
	UnitCost                float64 `json:"unit_cost,omitempty"`
	CostUnitCost            float64 `json:"cost_unit_cost,omitempty"`
	CostUnit                string  `json:"cost_unit,omitempty"`
	Amount                  float64 `json:"amount"`
	Unit                    string  `json:"unit"`
	Description             string  `json:"description,omitempty"`
	WorkstationName         string  `json:"workstation_name,omitempty"`
	CapacityName            string  `json:"capacity_name,omitempty"`
	CapacitySelectionSource string  `json:"capacity_selection_source,omitempty"`
	Warning                 string  `json:"warning,omitempty"`
	CostMethod              string  `json:"cost_method,omitempty"`
	PieceRate               float64 `json:"piece_rate,omitempty"`
	RateUnit                string  `json:"rate_unit,omitempty"`
	HourlyRate              float64 `json:"hourly_rate,omitempty"`
	StandardMinutes         float64 `json:"standard_minutes,omitempty"`
	StandardOutputQty       float64 `json:"standard_output_qty,omitempty"`
	StandardOutputUnit      string  `json:"standard_output_unit,omitempty"`
	AmountPerKg             float64 `json:"-"`
	AmountPerUnit           float64 `json:"-"`
}

type PricingRuleTrialOtherCostDetail struct {
	Name            string  `json:"name"`
	Amount          float64 `json:"amount"`
	Unit            string  `json:"unit"`
	Source          string  `json:"source"`
	SettingLocation string  `json:"setting_location"`
}

type PricingRuleTrialProfitExplanation struct {
	Method         string  `json:"method"`
	MethodLabel    string  `json:"method_label"`
	Rate           float64 `json:"rate"`
	Source         string  `json:"source"`
	CostAfterYield float64 `json:"cost_after_yield"`
	MarkupAmount   float64 `json:"markup_amount"`
	PreTaxPrice    float64 `json:"pre_tax_price"`
	Formula        string  `json:"formula"`
}

type PricingRuleTrialDefaultTaxRate struct {
	Rate   float64 `json:"rate"`
	Source string  `json:"source"`
}

type DripPriceExplanationCommand struct {
	Product   domain.ProductInput `json:"product"`
	TierLabel string              `json:"tier_label"`
}

type SaveDripPriceTemplateCommand struct {
	ID               int64                          `json:"id,omitempty"`
	Name             string                         `json:"name"`
	Active           *bool                          `json:"active,omitempty"`
	BagGrams         float64                        `json:"bag_grams"`
	BoxBagCount      int                            `json:"box_bag_count"`
	IncludePackaging *bool                          `json:"include_packaging,omitempty"`
	Tiers            []SaveDripPriceTemplateTierRow `json:"tiers"`
	Actor            string                         `json:"actor,omitempty"`
}

type SaveDripPriceTemplateTierRow struct {
	ID         int64    `json:"id,omitempty"`
	Label      string   `json:"label"`
	MinBags    float64  `json:"min_bags"`
	MaxBags    *float64 `json:"max_bags,omitempty"`
	Multiplier float64  `json:"multiplier"`
	Position   int      `json:"position"`
	Active     bool     `json:"active"`
}

type DeactivateDripPriceTemplateCommand struct {
	ID    int64  `json:"id"`
	Actor string `json:"actor,omitempty"`
}

type CalculateResponse struct {
	Parameters domain.Parameters      `json:"parameters"`
	Items      []domain.ProductResult `json:"items"`
}

type BeanListQuery struct {
	CustomerID int64 `json:"customer_id,omitempty"`
}

type Run struct {
	ID           int64                  `json:"id"`
	Status       string                 `json:"status"`
	ProductCount int                    `json:"product_count"`
	Items        []domain.ProductResult `json:"items,omitempty"`
}

type ParameterSetting struct {
	Key       string  `json:"key"`
	Label     string  `json:"label"`
	Value     float64 `json:"value"`
	Unit      string  `json:"unit"`
	UpdatedAt string  `json:"updated_at,omitempty"`
}

type UpdateParameterCommand struct {
	Key   string  `json:"key"`
	Value float64 `json:"value"`
	Actor string  `json:"actor,omitempty"`
}

type BeanListPublication struct {
	ID                         int64          `json:"id"`
	PublicationPurpose         string         `json:"publication_purpose"`
	ListType                   string         `json:"list_type"`
	ProductTypeCategoryID      int64          `json:"product_type_category_id,omitempty"`
	ProductTypeName            string         `json:"product_type_name,omitempty"`
	ClassificationTemplateID   int64          `json:"classification_template_id,omitempty"`
	ClassificationTemplateName string         `json:"classification_template_name,omitempty"`
	ClassificationCategoryID   int64          `json:"classification_category_id,omitempty"`
	ClassificationCategoryName string         `json:"classification_category_name,omitempty"`
	Version                    string         `json:"version"`
	Status                     string         `json:"status"`
	OwnerType                  string         `json:"owner_type"`
	OwnerKey                   string         `json:"owner_key,omitempty"`
	PriceSourcePublicationID   int64          `json:"price_source_publication_id,omitempty"`
	StyleSourcePublicationID   int64          `json:"style_source_publication_id,omitempty"`
	SourceVersion              string         `json:"source_version,omitempty"`
	Config                     map[string]any `json:"config"`
	Content                    map[string]any `json:"content"`
	Changelog                  string         `json:"changelog"`
	PublishedAt                string         `json:"published_at,omitempty"`
	WithdrawnAt                string         `json:"withdrawn_at,omitempty"`
	CreatedAt                  string         `json:"created_at,omitempty"`
}

type BeanListPublicationQuery struct {
	ListType                 string `json:"list_type"`
	PublicationPurpose       string `json:"publication_purpose,omitempty"`
	ProductTypeCategoryID    int64  `json:"product_type_category_id,omitempty"`
	ClassificationTemplateID int64  `json:"classification_template_id,omitempty"`
	Scope                    string `json:"scope,omitempty"`
	CustomerID               int64  `json:"customer_id,omitempty"`
	OwnerType                string `json:"owner_type,omitempty"`
	OwnerKey                 string `json:"owner_key,omitempty"`
}

type BeanListPublicationAsset struct {
	PublicationID int64  `json:"publication_id"`
	AssetType     string `json:"asset_type"`
	ContentType   string `json:"content_type"`
	CacheKey      string `json:"cache_key"`
	Payload       []byte `json:"-"`
}

type BeanListPublicationPDFCommand struct {
	PublicationID int64
	Query         BeanListPublicationQuery
	Actor         string
}

type BeanListPublicationPDFFile struct {
	PublicationID int64  `json:"publication_id"`
	ListType      string `json:"list_type"`
	Version       string `json:"version"`
	ContentType   string `json:"content_type"`
	CacheKey      string `json:"cache_key"`
	Filename      string `json:"filename"`
	DownloadURL   string `json:"download_url,omitempty"`
	Bytes         int    `json:"bytes"`
	Payload       []byte `json:"-"`
}

const (
	PriceListGroupSourceProductCatalog = "product_catalog"
	PriceListGroupSourcePriceList      = "price_list"
)

type PublishBeanListCommand struct {
	ListType                   string         `json:"list_type"`
	PublicationPurpose         string         `json:"publication_purpose,omitempty"`
	ProductTypeCategoryID      int64          `json:"product_type_category_id,omitempty"`
	ProductTypeName            string         `json:"product_type_name,omitempty"`
	ClassificationTemplateID   int64          `json:"classification_template_id,omitempty"`
	ClassificationTemplateName string         `json:"classification_template_name,omitempty"`
	ClassificationCategoryID   int64          `json:"classification_category_id,omitempty"`
	ClassificationCategoryName string         `json:"classification_category_name,omitempty"`
	Version                    string         `json:"version"`
	Scope                      string         `json:"scope,omitempty"`
	CustomerID                 int64          `json:"customer_id,omitempty"`
	OwnerType                  string         `json:"owner_type,omitempty"`
	OwnerKey                   string         `json:"owner_key,omitempty"`
	PriceSourcePublicationID   int64          `json:"price_source_publication_id,omitempty"`
	StyleSourcePublicationID   int64          `json:"style_source_publication_id,omitempty"`
	SourceVersion              string         `json:"source_version,omitempty"`
	Config                     map[string]any `json:"config"`
	Content                    map[string]any `json:"content"`
	Changelog                  string         `json:"changelog"`
	Actor                      string         `json:"actor,omitempty"`
}

type WithdrawBeanListCommand struct {
	ID                 int64  `json:"id"`
	PublicationPurpose string `json:"publication_purpose,omitempty"`
	Scope              string `json:"scope,omitempty"`
	OwnerType          string `json:"owner_type,omitempty"`
	OwnerKey           string `json:"owner_key,omitempty"`
	Actor              string `json:"actor,omitempty"`
}

type ArchiveBeanListPublicationsCommand struct {
	IDs                []int64 `json:"ids"`
	PublicationPurpose string  `json:"publication_purpose,omitempty"`
	Scope              string  `json:"scope,omitempty"`
	OwnerType          string  `json:"owner_type,omitempty"`
	OwnerKey           string  `json:"owner_key,omitempty"`
	Actor              string  `json:"actor,omitempty"`
}

type Repository interface {
	LoadParameters(ctx context.Context) (domain.Parameters, error)
	LoadProductInputs(ctx context.Context, params domain.Parameters) ([]domain.ProductInput, error)
	LoadProductPricingRule(ctx context.Context, id int64) (ProductPricingRule, error)
	CreateRun(ctx context.Context, actor string, items []domain.ProductResult) (*Run, error)
	PublishRun(ctx context.Context, actor string, runID int64) error
	ListParameterSettings(ctx context.Context) ([]ParameterSetting, error)
	UpdateParameterSetting(ctx context.Context, cmd UpdateParameterCommand) (ParameterSetting, error)
	ListDripPriceTemplates(ctx context.Context) ([]domain.DripPriceTemplate, error)
	SaveDripPriceTemplate(ctx context.Context, cmd SaveDripPriceTemplateCommand) (*domain.DripPriceTemplate, error)
	DeactivateDripPriceTemplate(ctx context.Context, cmd DeactivateDripPriceTemplateCommand) error
	ListBeanListPublications(ctx context.Context, query BeanListPublicationQuery) ([]BeanListPublication, error)
	PublishedBeanList(ctx context.Context, query BeanListPublicationQuery) (*BeanListPublication, error)
	LoadBeanListPublication(ctx context.Context, query BeanListPublicationQuery, publicationID int64) (*BeanListPublication, error)
	LoadBeanListPublicationAsset(ctx context.Context, publicationID int64, assetType string) (BeanListPublicationAsset, error)
	SaveBeanListPublicationAsset(ctx context.Context, asset BeanListPublicationAsset, actor string) (BeanListPublicationAsset, error)
	PublishBeanList(ctx context.Context, cmd PublishBeanListCommand) (*BeanListPublication, error)
	SaveBeanListDraft(ctx context.Context, cmd PublishBeanListCommand) (*BeanListPublication, error)
	WithdrawBeanList(ctx context.Context, cmd WithdrawBeanListCommand) error
	ArchiveBeanListPublications(ctx context.Context, cmd ArchiveBeanListPublicationsCommand) error
	UnarchiveBeanListPublications(ctx context.Context, cmd ArchiveBeanListPublicationsCommand) error
}

type productSalesUnitRuleRepository interface {
	ResolveProductSalesUnitRule(ctx context.Context, productID int64, priceUnit string) (ProductSalesUnitRule, error)
}

type productSpecIdentityRepository interface {
	ResolveProductSpecIdentity(ctx context.Context, productID int64) (ProductSpecIdentity, error)
}

type productDefaultSalesUnitRepository interface {
	ResolveProductDefaultSalesUnit(ctx context.Context, productID int64) (string, error)
}

type customerProductSalesUnitRuleRepository interface {
	ResolveCustomerProductSalesUnitRule(ctx context.Context, productID int64, customerProductAliasID int64, priceUnit string) (ProductSalesUnitRule, error)
}

type priceTierTemplateUnitRuleRepository interface {
	ResolvePriceTierTemplateUnitRule(ctx context.Context, templateID int64) (PriceTierTemplateUnitRule, error)
}

type customerScopedProductInputRepository interface {
	LoadProductInputsForCustomer(ctx context.Context, params domain.Parameters, customerID int64) ([]domain.ProductInput, error)
}

type pricingRuleTrialBaseCostDetailRepository interface {
	LoadPricingRuleTrialBaseCostDetails(ctx context.Context, input domain.ProductInput) ([]PricingRuleTrialBaseCostDetail, error)
}

type pricingRuleTrialBatchBaseCostDetailRepository interface {
	LoadPricingRuleTrialBaseCostDetailsBatch(ctx context.Context, inputs []domain.ProductInput) ([][]PricingRuleTrialBaseCostDetail, []error, error)
}

type pricingRuleTrialProductionOptionRepository interface {
	LoadPricingRuleTrialProductionOptions(ctx context.Context, input domain.ProductInput) (PricingRuleTrialProductionOptions, error)
}

type pricingRuleTrialDefaultTaxRateRepository interface {
	LoadPricingRuleTrialDefaultTaxRate(ctx context.Context) (PricingRuleTrialDefaultTaxRate, error)
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

func (s *Service) Settings(ctx context.Context) ([]ParameterSetting, error) {
	if s.repo == nil {
		return filterEditableQuickSettings(defaultParameterSettings()), nil
	}
	rows, err := s.repo.ListParameterSettings(ctx)
	if err != nil {
		return nil, err
	}
	return filterEditableQuickSettings(rows), nil
}

func (s *Service) UpdateSetting(ctx context.Context, cmd UpdateParameterCommand) (ParameterSetting, error) {
	cmd.Key = strings.TrimSpace(cmd.Key)
	if cmd.Key == "" {
		return ParameterSetting{}, fmt.Errorf("key required")
	}
	if isHiddenQuickSetting(cmd.Key) {
		return ParameterSetting{}, fmt.Errorf("setting %s is managed by BOM, gradient templates, or drip templates", cmd.Key)
	}
	if math.IsNaN(cmd.Value) || math.IsInf(cmd.Value, 0) || cmd.Value < 0 {
		return ParameterSetting{}, fmt.Errorf("value must be >= 0")
	}
	if s.repo == nil {
		return ParameterSetting{}, fmt.Errorf("repository required")
	}
	return s.repo.UpdateParameterSetting(ctx, cmd)
}

func filterEditableQuickSettings(rows []ParameterSetting) []ParameterSetting {
	out := make([]ParameterSetting, 0, len(rows))
	for _, row := range rows {
		if isHiddenQuickSetting(row.Key) {
			continue
		}
		out = append(out, row)
	}
	return out
}

func isHiddenQuickSetting(key string) bool {
	switch strings.TrimSpace(key) {
	case "roast_yield_rate",
		"retail_bean_margin_rate",
		"retail_drip_multiplier",
		"wholesale_kg_margin_rate_1",
		"wholesale_kg_margin_rate_2",
		"wholesale_kg_margin_rate_3",
		"wholesale_kg_margin_rate_4",
		"wholesale_kg_margin_rate_5",
		"wholesale_kg_margin_rate_6",
		"wholesale_drip_multiplier_1",
		"wholesale_drip_multiplier_2",
		"wholesale_drip_multiplier_3",
		"wholesale_drip_multiplier_4":
		return true
	default:
		return false
	}
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

func (s *Service) ExplainPrice(ctx context.Context, req PriceExplanationCommand) (*domain.PriceExplanation, error) {
	params, err := s.Parameters(ctx)
	if err != nil {
		return nil, err
	}
	out, err := domain.ExplainCommercialPrice(params, req.Product, domain.PriceExplanationRequest{
		TierLabel: req.TierLabel,
		Overrides: req.Overrides,
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Service) PricingRuleTrial(ctx context.Context, cmd PricingRuleTrialCommand) (*PricingRuleTrialResult, error) {
	if cmd.PricingRuleID <= 0 {
		return nil, fmt.Errorf("pricing_rule_id required")
	}
	if cmd.ProductID <= 0 {
		return nil, fmt.Errorf("product_id required")
	}
	if s.repo == nil {
		return nil, fmt.Errorf("repository required")
	}
	params, err := s.Parameters(ctx)
	if err != nil {
		return nil, err
	}
	rule, err := s.repo.LoadProductPricingRule(ctx, cmd.PricingRuleID)
	if err != nil {
		if errors.Is(err, ErrProductPricingRuleNotFound) {
			return nil, ErrProductPricingRuleNotFound
		}
		return nil, err
	}
	inputs, err := s.pricingRuleTrialProductInputs(ctx, params, cmd.CustomerID)
	if err != nil {
		return nil, err
	}
	var input domain.ProductInput
	found := false
	for _, row := range inputs {
		if row.ProductID == cmd.ProductID {
			input = row
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("product not found")
	}
	productionOptions := PricingRuleTrialProductionOptions{}
	if optionRepo, ok := s.repo.(pricingRuleTrialProductionOptionRepository); ok {
		productionOptions, err = optionRepo.LoadPricingRuleTrialProductionOptions(ctx, input)
		if err != nil {
			return nil, err
		}
		productionOptions.loaded = true
	}
	input, productionOptions, err = pricingRuleTrialApplyProductionSelection(input, cmd, productionOptions)
	if err != nil {
		return nil, err
	}
	quoteUnit := pricingRuleTrialResolvedQuoteUnit(input, cmd.QuoteUnit)
	if !pricingRuleTrialQuoteUnitResolvable(input, quoteUnit) {
		return nil, fmt.Errorf("销售单位%s缺少可解析的单位换算，请先在商品档案维护销售规格或单位换算", pricingRuleTrialQuotedUnit(quoteUnit))
	}
	cmd.QuoteUnit = quoteUnit
	var baseCostDetails []PricingRuleTrialBaseCostDetail
	if detailRepo, ok := s.repo.(pricingRuleTrialBaseCostDetailRepository); ok {
		baseCostDetails, err = detailRepo.LoadPricingRuleTrialBaseCostDetails(ctx, input)
		if err != nil {
			return nil, err
		}
	}
	defaultTaxRate := PricingRuleTrialDefaultTaxRate{}
	if taxRepo, ok := s.repo.(pricingRuleTrialDefaultTaxRateRepository); ok {
		defaultTaxRate, err = taxRepo.LoadPricingRuleTrialDefaultTaxRate(ctx)
		if err != nil {
			return nil, err
		}
	}
	return calculatePricingRuleTrial(rule, input, cmd, baseCostDetails, productionOptions, defaultTaxRate)
}

type preparedPricingRuleTrial struct {
	index             int
	command           PricingRuleTrialCommand
	rule              ProductPricingRule
	input             domain.ProductInput
	productionOptions PricingRuleTrialProductionOptions
}

func (s *Service) PricingRuleTrialBatch(ctx context.Context, commands []PricingRuleTrialCommand) ([]PricingRuleTrialBatchRow, error) {
	rows := make([]PricingRuleTrialBatchRow, len(commands))
	for i := range rows {
		rows[i].Index = i
	}
	if len(commands) == 0 {
		return rows, nil
	}
	if s.repo == nil {
		return nil, fmt.Errorf("repository required")
	}
	params, err := s.Parameters(ctx)
	if err != nil {
		return nil, err
	}

	rules := make(map[int64]ProductPricingRule)
	ruleErrors := make(map[int64]error)
	inputsByCustomer := make(map[int64]map[int64]domain.ProductInput)
	prepared := make([]preparedPricingRuleTrial, 0, len(commands))
	for index, original := range commands {
		cmd := original
		if cmd.PricingRuleID <= 0 {
			rows[index].Error = "pricing_rule_id required"
			continue
		}
		if cmd.ProductID <= 0 {
			rows[index].Error = "product_id required"
			continue
		}

		if ruleErr, failed := ruleErrors[cmd.PricingRuleID]; failed {
			rows[index].Error = ruleErr.Error()
			continue
		}
		rule, ok := rules[cmd.PricingRuleID]
		if !ok {
			rule, err = s.repo.LoadProductPricingRule(ctx, cmd.PricingRuleID)
			if err != nil {
				ruleErrors[cmd.PricingRuleID] = err
				rows[index].Error = err.Error()
				continue
			}
			rules[cmd.PricingRuleID] = rule
		}

		customerInputs, ok := inputsByCustomer[cmd.CustomerID]
		if !ok {
			inputs, loadErr := s.pricingRuleTrialProductInputs(ctx, params, cmd.CustomerID)
			if loadErr != nil {
				return nil, loadErr
			}
			customerInputs = make(map[int64]domain.ProductInput, len(inputs))
			for _, input := range inputs {
				customerInputs[input.ProductID] = input
			}
			inputsByCustomer[cmd.CustomerID] = customerInputs
		}
		input, ok := customerInputs[cmd.ProductID]
		if !ok {
			rows[index].Error = "product not found"
			continue
		}

		prepared = append(prepared, preparedPricingRuleTrial{
			index:   index,
			command: cmd,
			rule:    rule,
			input:   input,
		})
	}
	if len(prepared) == 0 {
		return rows, nil
	}
	productionOptions := make([]PricingRuleTrialProductionOptions, len(prepared))
	productionOptionErrors := make([]error, len(prepared))
	if optionRepo, ok := s.repo.(pricingRuleTrialProductionOptionRepository); ok {
		productionOptions, productionOptionErrors = loadPricingRuleTrialProductionOptionsBatch(ctx, optionRepo, prepared)
	}
	ready := make([]preparedPricingRuleTrial, 0, len(prepared))
	for i, item := range prepared {
		if productionOptionErrors[i] != nil {
			rows[item.index].Error = productionOptionErrors[i].Error()
			continue
		}
		input, options, applyErr := pricingRuleTrialApplyProductionSelection(item.input, item.command, productionOptions[i])
		if applyErr != nil {
			rows[item.index].Error = applyErr.Error()
			continue
		}
		quoteUnit := pricingRuleTrialResolvedQuoteUnit(input, item.command.QuoteUnit)
		if !pricingRuleTrialQuoteUnitResolvable(input, quoteUnit) {
			rows[item.index].Error = fmt.Sprintf("销售单位%s缺少可解析的单位换算，请先在商品档案维护销售规格或单位换算", pricingRuleTrialQuotedUnit(quoteUnit))
			continue
		}
		item.command.QuoteUnit = quoteUnit
		item.input = input
		item.productionOptions = options
		ready = append(ready, item)
	}
	prepared = ready
	if len(prepared) == 0 {
		return rows, nil
	}

	defaultTaxRate := PricingRuleTrialDefaultTaxRate{}
	if taxRepo, ok := s.repo.(pricingRuleTrialDefaultTaxRateRepository); ok {
		defaultTaxRate, err = taxRepo.LoadPricingRuleTrialDefaultTaxRate(ctx)
		if err != nil {
			return nil, err
		}
	}

	inputs := make([]domain.ProductInput, len(prepared))
	for i := range prepared {
		inputs[i] = prepared[i].input
	}
	details := make([][]PricingRuleTrialBaseCostDetail, len(prepared))
	detailErrors := make([]error, len(prepared))
	if batchRepo, ok := s.repo.(pricingRuleTrialBatchBaseCostDetailRepository); ok {
		details, detailErrors, err = batchRepo.LoadPricingRuleTrialBaseCostDetailsBatch(ctx, inputs)
		if err != nil {
			return nil, err
		}
		if len(details) != len(prepared) || len(detailErrors) != len(prepared) {
			return nil, fmt.Errorf("pricing rule trial batch details count mismatch")
		}
	} else if _, ok := s.repo.(pricingRuleTrialBaseCostDetailRepository); ok {
		return nil, fmt.Errorf("repository batch pricing rule trial details required")
	}

	for i, item := range prepared {
		if detailErrors[i] != nil {
			rows[item.index].Error = detailErrors[i].Error()
			continue
		}
		result, calculateErr := calculatePricingRuleTrial(item.rule, item.input, item.command, details[i], item.productionOptions, defaultTaxRate)
		if calculateErr != nil {
			rows[item.index].Error = calculateErr.Error()
			continue
		}
		rows[item.index].Result = result
	}
	return rows, nil
}

func loadPricingRuleTrialProductionOptionsBatch(ctx context.Context, repo pricingRuleTrialProductionOptionRepository, items []preparedPricingRuleTrial) ([]PricingRuleTrialProductionOptions, []error) {
	options := make([]PricingRuleTrialProductionOptions, len(items))
	errs := make([]error, len(items))
	workerCount := len(items)
	if workerCount > 6 {
		workerCount = 6
	}
	jobs := make(chan int)
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for worker := 0; worker < workerCount; worker++ {
		go func() {
			defer workers.Done()
			for index := range jobs {
				options[index], errs[index] = repo.LoadPricingRuleTrialProductionOptions(ctx, items[index].input)
				options[index].loaded = errs[index] == nil
			}
		}()
	}
	for index := range items {
		jobs <- index
	}
	close(jobs)
	workers.Wait()
	return options, errs
}

func (s *Service) pricingRuleTrialProductInputs(ctx context.Context, params domain.Parameters, customerID int64) ([]domain.ProductInput, error) {
	if customerID > 0 {
		if scoped, ok := s.repo.(customerScopedProductInputRepository); ok {
			return scoped.LoadProductInputsForCustomer(ctx, params, customerID)
		}
	}
	return s.repo.LoadProductInputs(ctx, params)
}

func pricingRuleTrialApplyProductionSelection(input domain.ProductInput, cmd PricingRuleTrialCommand, options PricingRuleTrialProductionOptions) (domain.ProductInput, PricingRuleTrialProductionOptions, error) {
	options = pricingRuleTrialNormalizeProductionOptions(input, options)
	if len(options.BomVersions) > 0 {
		var selected *PricingRuleTrialBomVersionOption
		if cmd.BomVersionID > 0 {
			selected = pricingRuleTrialFindBomVersionOption(options.BomVersions, cmd.BomVersionID)
			if selected == nil {
				return input, options, fmt.Errorf("production BOM version not found for product")
			}
		} else {
			selected = pricingRuleTrialDefaultBomVersionOption(options.BomVersions, input.BomVersionID)
		}
		if selected != nil {
			input.BomVersionID = selected.VersionID
			input.BomVersionNo = selected.VersionNo
			if input.ProcessRouteID <= 0 && selected.ProcessRouteID > 0 {
				input.ProcessRouteID = selected.ProcessRouteID
				input.ProcessRouteName = strings.TrimSpace(selected.ProcessRouteName)
			}
			input.BomUsageMode = "production_bom_output"
			switch strings.TrimSpace(selected.Status) {
			case "disabled", "inactive":
				input.BomStatus = "disabled"
			default:
				input.BomStatus = "active"
			}
		}
	} else if options.loaded {
		if cmd.BomVersionID > 0 {
			return input, options, fmt.Errorf("production BOM version not found for product")
		}
		input.BomVersionID = 0
		input.BomVersionNo = ""
		input.BomUsageMode = ""
		input.BomStatus = "missing"
	} else if cmd.BomVersionID > 0 {
		input.BomVersionID = cmd.BomVersionID
		input.BomVersionNo = ""
		input.BomUsageMode = "production_bom_output"
	}

	if len(options.ProcessRoutes) > 0 {
		var selected *PricingRuleTrialProcessRouteOption
		if cmd.ProcessRouteID > 0 {
			selected = pricingRuleTrialFindProcessRouteOption(options.ProcessRoutes, cmd.ProcessRouteID)
			if selected == nil {
				return input, options, fmt.Errorf("process route not found")
			}
		} else if input.ProcessRouteID > 0 {
			selected = pricingRuleTrialFindProcessRouteOption(options.ProcessRoutes, input.ProcessRouteID)
		}
		if selected != nil {
			input.ProcessRouteID = selected.ID
			input.ProcessRouteName = selected.Name
			input.OperationTemplateID = 0
		} else {
			input.ProcessRouteID = 0
			input.ProcessRouteName = ""
		}
	} else if cmd.ProcessRouteID > 0 {
		input.ProcessRouteID = cmd.ProcessRouteID
		input.OperationTemplateID = 0
	}

	options = pricingRuleTrialNormalizeProductionOptions(input, options)

	if len(options.OperationTemplates) > 0 {
		var selected *PricingRuleTrialOperationTemplateOption
		if input.ProcessRouteID > 0 {
			input.OperationTemplateID = 0
			return input, options, nil
		}
		if cmd.OperationTemplateID > 0 {
			selected = pricingRuleTrialFindOperationTemplateOption(options.OperationTemplates, cmd.OperationTemplateID)
			if selected == nil {
				return input, options, fmt.Errorf("operation template not found")
			}
		} else if input.OperationTemplateID > 0 {
			selected = pricingRuleTrialFindOperationTemplateOption(options.OperationTemplates, input.OperationTemplateID)
		}
		if selected != nil {
			input.OperationTemplateID = selected.ID
		} else {
			input.OperationTemplateID = 0
		}
	} else if cmd.OperationTemplateID > 0 {
		input.OperationTemplateID = cmd.OperationTemplateID
	}
	return input, options, nil
}

func pricingRuleTrialNormalizeProductionOptions(input domain.ProductInput, options PricingRuleTrialProductionOptions) PricingRuleTrialProductionOptions {
	for i := range options.BomVersions {
		options.BomVersions[i].BomCode = strings.TrimSpace(options.BomVersions[i].BomCode)
		options.BomVersions[i].BomName = strings.TrimSpace(options.BomVersions[i].BomName)
		options.BomVersions[i].VersionNo = strings.TrimSpace(options.BomVersions[i].VersionNo)
		options.BomVersions[i].Status = strings.TrimSpace(options.BomVersions[i].Status)
		options.BomVersions[i].ProcessRouteName = strings.TrimSpace(options.BomVersions[i].ProcessRouteName)
		options.BomVersions[i].LatestNonEmptyDraftVersionNo = strings.TrimSpace(options.BomVersions[i].LatestNonEmptyDraftVersionNo)
		if options.BomVersions[i].VersionID == input.BomVersionID && !pricingRuleTrialHasDefaultBomVersion(options.BomVersions) {
			options.BomVersions[i].IsDefault = true
		}
	}
	for i := range options.ProcessRoutes {
		options.ProcessRoutes[i].Name = strings.TrimSpace(options.ProcessRoutes[i].Name)
		if options.ProcessRoutes[i].ID == input.ProcessRouteID && !pricingRuleTrialHasDefaultProcessRoute(options.ProcessRoutes) {
			options.ProcessRoutes[i].IsDefault = true
		}
	}
	for i := range options.OperationTemplates {
		options.OperationTemplates[i].Name = strings.TrimSpace(options.OperationTemplates[i].Name)
		if options.OperationTemplates[i].ID == input.OperationTemplateID && !pricingRuleTrialHasDefaultOperationTemplate(options.OperationTemplates) {
			options.OperationTemplates[i].IsDefault = true
		}
	}
	return options
}

func pricingRuleTrialRejectEmptyPublishedBom(input domain.ProductInput, cmd PricingRuleTrialCommand, operationCostTotal float64, options PricingRuleTrialProductionOptions) error {
	selected := pricingRuleTrialFindBomVersionOption(options.BomVersions, input.BomVersionID)
	if selected == nil || selected.ComponentCount > 0 || selected.LatestNonEmptyDraftVersionID <= 0 {
		return nil
	}
	if cmd.Overrides.BaseCost != nil && *cmd.Overrides.BaseCost > 0 {
		return nil
	}
	if operationCostTotal > 0 {
		return nil
	}
	publishedVersion := firstNonEmptyString(strings.TrimSpace(selected.VersionNo), fmt.Sprintf("#%d", selected.VersionID))
	draftVersion := firstNonEmptyString(strings.TrimSpace(selected.LatestNonEmptyDraftVersionNo), fmt.Sprintf("#%d", selected.LatestNonEmptyDraftVersionID))
	return fmt.Errorf("当前已发布生产 BOM %s 没有组件，无法计算标准制造成本；检测到 %s 草稿未发布，请先发布 %s（入口：生产管理 → 生产 BOM）后再试算", publishedVersion, draftVersion, draftVersion)
}

func pricingRuleTrialHasIndependentPositiveOperationCost(details []PricingRuleTrialBaseCostDetail) bool {
	for _, detail := range details {
		if strings.TrimSpace(detail.Type) != "operation" || detail.Amount <= 0 {
			continue
		}
		switch strings.TrimSpace(detail.CapacitySelectionSource) {
		case "operation_template", "operation_master", "process_route":
			return true
		}
	}
	return false
}

func pricingRuleTrialRejectMissingPublishedBom(input domain.ProductInput, cmd PricingRuleTrialCommand, bomCostTotal float64, details []PricingRuleTrialBaseCostDetail, options PricingRuleTrialProductionOptions) error {
	missingPublishedBom := options.loaded && len(options.BomVersions) == 0
	if !options.loaded {
		missingPublishedBom = strings.TrimSpace(input.BomStatus) == "missing" && input.BomVersionID <= 0
	}
	if !missingPublishedBom {
		return nil
	}
	if cmd.Overrides.BaseCost != nil && *cmd.Overrides.BaseCost > 0 {
		return nil
	}
	if pricingRuleTrialHasIndependentPositiveOperationCost(details) {
		return nil
	}
	if !options.loaded && bomCostTotal > 0 {
		return nil
	}
	return errors.New(pricingRuleTrialMissingPublishedBomError)
}

func pricingRuleTrialHasDefaultBomVersion(options []PricingRuleTrialBomVersionOption) bool {
	for _, option := range options {
		if option.IsDefault {
			return true
		}
	}
	return false
}

func pricingRuleTrialHasDefaultOperationTemplate(options []PricingRuleTrialOperationTemplateOption) bool {
	for _, option := range options {
		if option.IsDefault {
			return true
		}
	}
	return false
}

func pricingRuleTrialHasDefaultProcessRoute(options []PricingRuleTrialProcessRouteOption) bool {
	for _, option := range options {
		if option.IsDefault {
			return true
		}
	}
	return false
}

func pricingRuleTrialDefaultBomVersionOption(options []PricingRuleTrialBomVersionOption, currentVersionID int64) *PricingRuleTrialBomVersionOption {
	if currentVersionID > 0 {
		if option := pricingRuleTrialFindBomVersionOption(options, currentVersionID); option != nil {
			return option
		}
	}
	for i := range options {
		if options[i].IsDefault {
			return &options[i]
		}
	}
	if len(options) == 0 {
		return nil
	}
	return &options[0]
}

func pricingRuleTrialFindBomVersionOption(options []PricingRuleTrialBomVersionOption, versionID int64) *PricingRuleTrialBomVersionOption {
	for i := range options {
		if options[i].VersionID == versionID {
			return &options[i]
		}
	}
	return nil
}

func pricingRuleTrialFindProcessRouteOption(options []PricingRuleTrialProcessRouteOption, id int64) *PricingRuleTrialProcessRouteOption {
	for i := range options {
		if options[i].ID == id {
			return &options[i]
		}
	}
	return nil
}

func pricingRuleTrialFindOperationTemplateOption(options []PricingRuleTrialOperationTemplateOption, id int64) *PricingRuleTrialOperationTemplateOption {
	for i := range options {
		if options[i].ID == id {
			return &options[i]
		}
	}
	return nil
}

func pricingRuleTrialProcessRouteName(options []PricingRuleTrialProcessRouteOption, id int64, fallback string) string {
	if option := pricingRuleTrialFindProcessRouteOption(options, id); option != nil {
		return option.Name
	}
	return strings.TrimSpace(fallback)
}

func pricingRuleTrialOperationTemplateName(options []PricingRuleTrialOperationTemplateOption, id int64) string {
	if option := pricingRuleTrialFindOperationTemplateOption(options, id); option != nil {
		return option.Name
	}
	return ""
}

func pricingRuleTrialResolvedQuoteUnit(input domain.ProductInput, requested string) string {
	quoteUnit := strings.TrimSpace(requested)
	if quoteUnit == "" {
		quoteUnit = strings.TrimSpace(input.QuoteUnit)
	}
	if quoteUnit == "" {
		quoteUnit = strings.TrimSpace(input.InventoryUnit)
	}
	if quoteUnit == "" {
		quoteUnit = "kg"
	}
	return quoteUnit
}

func pricingRuleTrialQuotedUnit(unit string) string {
	unit = strings.TrimSpace(unit)
	if unit == "" {
		return ""
	}
	return fmt.Sprintf("“%s”", unit)
}

func pricingRuleTrialQuoteUnitResolvable(input domain.ProductInput, quoteUnit string) bool {
	unit := strings.TrimSpace(quoteUnit)
	if unit == "" {
		return false
	}
	inventoryUnit := strings.TrimSpace(input.InventoryUnit)
	if inventoryUnit == "" {
		inventoryUnit = "kg"
	}
	if pricingRuleTrialUnitTargetFactor(unit, inventoryUnit, input.UnitConversionJSON) > 0 {
		return true
	}
	if pricingRuleTrialUnitKgFactor(unit, input.UnitConversionJSON) > 0 {
		return true
	}
	return false
}

func calculatePricingRuleTrial(rule ProductPricingRule, input domain.ProductInput, cmd PricingRuleTrialCommand, rawBaseCostDetails []PricingRuleTrialBaseCostDetail, productionOptions PricingRuleTrialProductionOptions, defaultTaxRate PricingRuleTrialDefaultTaxRate) (*PricingRuleTrialResult, error) {
	calc := rule.CalculationJSON
	if calc == nil {
		calc = map[string]any{}
	}
	quoteUnit := pricingRuleTrialResolvedQuoteUnit(input, cmd.QuoteUnit)
	warnings := append([]string{}, input.Warnings...)
	bomStatus := strings.TrimSpace(input.BomStatus)
	switch bomStatus {
	case "inactive", "disabled":
		warnings = appendUniqueString(warnings, "BOM已失效：试算结果仅供参考")
	case "missing":
		warnings = appendUniqueString(warnings, "该商品未配置可用于试算的已发布生产 BOM")
	}
	if !rule.Active {
		warnings = appendUniqueString(warnings, "停用模板：试算仅供查看，不能作为新发布价格来源")
	}

	baseCost := 0.0
	baseSource := pricingRuleTrialStandardManufacturingCostSource
	if cmd.Overrides.BaseCost != nil {
		if *cmd.Overrides.BaseCost < 0 {
			return nil, fmt.Errorf("base_cost must be >= 0")
		}
		baseCost = *cmd.Overrides.BaseCost
		baseSource = "temporary_override"
	}
	otherCosts := pricingRuleTrialOtherCostMap(calc)
	if cmd.Overrides.OtherCosts != nil {
		otherCosts = cmd.Overrides.OtherCosts
	}
	otherCostTotal := 0.0
	for _, value := range otherCosts {
		if value < 0 {
			return nil, fmt.Errorf("other_costs must be >= 0")
		}
		otherCostTotal += value
	}
	yieldMode := pricingRuleTrialString(calc, "yield_loss_mode", "bom_or_product")
	expectedLossRate, lossChanged, err := pricingRuleTrialExpectedLossRate(input, cmd.Overrides.ExpectedLossRate, yieldMode)
	if err != nil {
		return nil, err
	}
	postMarkupCosts := pricingRuleTrialPostMarkupCostMap(calc)
	if cmd.Overrides.PostMarkupCosts != nil {
		postMarkupCosts = cmd.Overrides.PostMarkupCosts
	}
	formulaMode := pricingRuleTrialString(calc, "formula_mode", "standard")
	if formulaMode == "standard" && len(postMarkupCosts) > 0 {
		formulaMode = "supplier_tier_markup"
	}
	if pricingRuleTrialString(calc, "legacy_profit_method", "") != "" || pricingRuleTrialString(calc, "migration_warning", "") != "" {
		return nil, fmt.Errorf("quarantined legacy pricing rule must be replaced with a new markup template")
	}
	legacyProfitMethod := strings.ToLower(strings.TrimSpace(pricingRuleTrialString(calc, "profit_method", "")))
	switch legacyProfitMethod {
	case "", "gross_margin", "markup":
	default:
		return nil, fmt.Errorf("only markup rate is supported; update this pricing rule before trial")
	}
	profitMethod := "markup"
	marginRate := rule.MarginRate
	if cmd.Overrides.MarginRate != nil {
		marginRate = *cmd.Overrides.MarginRate
	} else if (legacyProfitMethod == "" || legacyProfitMethod == "gross_margin") && marginRate > 1 {
		marginRate /= 100
	}
	if marginRate < 0 {
		return nil, fmt.Errorf("margin_rate must be >= 0")
	}

	taxMode := pricingRuleTrialString(calc, "tax_mode", "tax_included")
	taxRate, taxRateSource, taxRateChanged, err := pricingRuleTrialResolvedTaxRate(rule, cmd, taxMode, defaultTaxRate)
	if err != nil {
		return nil, err
	}
	postMarkupCostTotal := 0.0
	if formulaMode == "supplier_tier_markup" {
		for _, value := range postMarkupCosts {
			if value < 0 {
				return nil, fmt.Errorf("post_markup_costs must be >= 0")
			}
			postMarkupCostTotal += value
		}
	}
	baseCostDetails, bomCostTotal, operationCostTotal := pricingRuleTrialNormalizeBaseCostDetails(input, quoteUnit, formulaMode, baseCost, cmd.Overrides.BaseCost != nil, rawBaseCostDetails)
	if err := pricingRuleTrialRejectEmptyPublishedBom(input, cmd, operationCostTotal, productionOptions); err != nil {
		return nil, err
	}
	if err := pricingRuleTrialRejectMissingPublishedBom(input, cmd, bomCostTotal, baseCostDetails, productionOptions); err != nil {
		return nil, err
	}
	for _, detail := range baseCostDetails {
		if warning := strings.TrimSpace(detail.Warning); warning != "" {
			warnings = appendUniqueString(warnings, warning)
		}
	}
	warnings = pricingRuleTrialCombinedLossWarnings(warnings, input, baseCostDetails)
	if cmd.Overrides.BaseCost == nil {
		if detailBaseCost := bomCostTotal + operationCostTotal; detailBaseCost > 0 {
			baseCost = detailBaseCost
			baseSource = pricingRuleTrialStandardManufacturingCostSource
		} else {
			baseCost = 0
		}
	}
	if pricingRuleTrialSuppressDefaultLossForActualBomCost(input, cmd, baseCostDetails, bomCostTotal, operationCostTotal) {
		expectedLossRate = 0
		lossChanged = false
	}
	if baseCost <= 0 {
		warnings = appendUniqueString(warnings, "该商品暂无可试算的标准制造成本")
	}
	costBeforeYield := baseCost + otherCostTotal
	costAfterYield := costBeforeYield
	if yieldMode != "none" && expectedLossRate > 0 {
		costAfterYield = costBeforeYield / (1 - expectedLossRate)
	}
	priceAfterMarkup := 0.0
	preTaxPrice := 0.0
	taxAmount := 0.0
	finalBeforeRounding := 0.0
	profitParameterRate := 0.0
	if formulaMode == "supplier_tier_markup" {
		profitParameterRate = pricingRuleTrialNumber(calc, "profit_parameter_rate", 0)
		markupRate := marginRate + profitParameterRate
		if markupRate < 0 {
			return nil, fmt.Errorf("markup_rate must be >= 0")
		}
		priceAfterMarkup = costAfterYield * (1 + markupRate)
		preTaxPrice = priceAfterMarkup + postMarkupCostTotal
		finalBeforeRounding = preTaxPrice
		if taxMode != "none" {
			finalBeforeRounding, taxAmount = pricingRuleTrialTax(preTaxPrice, taxRate, taxMode)
		}
	} else {
		preTaxPrice, err = pricingRuleTrialPreTaxPrice(costAfterYield, marginRate, profitMethod)
		if err != nil {
			return nil, err
		}
		priceAfterMarkup = preTaxPrice
		finalBeforeRounding, taxAmount = pricingRuleTrialTax(preTaxPrice, taxRate, taxMode)
	}
	finalUnitPrice := pricingRuleTrialRoundedPrice(finalBeforeRounding, rule.RoundingMode)
	if formulaMode == "supplier_tier_markup" && strings.TrimSpace(rule.RoundingMode) == "none" {
		finalUnitPrice = roundPricingRuleTrialDetail(finalBeforeRounding)
	}
	costBaseTotal := costBeforeYield
	yieldLossAmount := costAfterYield - costBeforeYield
	profitMarkupAmount := preTaxPrice - costAfterYield
	taxInPriceAmount := taxAmount
	if strings.TrimSpace(taxMode) == "tax_excluded" {
		taxInPriceAmount = 0
	}
	finalBeforeRoundingRounded := roundBeanListPrice(finalBeforeRounding)
	if formulaMode == "supplier_tier_markup" {
		finalBeforeRoundingRounded = roundPricingRuleTrialDetail(finalBeforeRounding)
	}
	displayedWaterfallBeforeRounding := pricingRuleTrialResultAmount(formulaMode, costBaseTotal) +
		pricingRuleTrialResultAmount(formulaMode, yieldLossAmount) +
		pricingRuleTrialResultAmount(formulaMode, profitMarkupAmount) +
		pricingRuleTrialResultAmount(formulaMode, taxInPriceAmount)
	roundingAdjustment := finalUnitPrice - displayedWaterfallBeforeRounding
	grossMarginRate := 0.0
	if preTaxPrice > 0 {
		grossMarginRate = (preTaxPrice - costAfterYield) / preTaxPrice
	}
	minimumMarginRate := pricingRuleTrialNumber(calc, "minimum_margin_rate", 0)
	if minimumMarginRate > 0 && grossMarginRate < minimumMarginRate {
		warnings = appendUniqueString(warnings, "试算毛利率低于最低毛利")
	}

	productName := strings.TrimSpace(input.Name)
	if productName == "" {
		productName = strings.TrimSpace(input.ProductName)
	}
	materialUnitCost := pricingRuleTrialResultAmount(formulaMode, bomCostTotal)
	operationUnitCost := pricingRuleTrialResultAmount(formulaMode, operationCostTotal)
	standardManufacturingUnitCost := pricingRuleTrialResultAmount(formulaMode, baseCost)
	processRouteName := pricingRuleTrialProcessRouteName(productionOptions.ProcessRoutes, input.ProcessRouteID, input.ProcessRouteName)
	result := &PricingRuleTrialResult{
		PricingRuleID:                 rule.ID,
		PricingRuleName:               firstNonEmptyString(rule.Name, rule.Code),
		FormulaVersion:                firstNonEmptyString(rule.FormulaVersion, "v1"),
		ProductID:                     input.ProductID,
		ProductName:                   productName,
		QuoteUnit:                     quoteUnit,
		InventoryUnit:                 strings.TrimSpace(input.InventoryUnit),
		BomVersionID:                  input.BomVersionID,
		BomVersionNo:                  input.BomVersionNo,
		BomVersionOptions:             productionOptions.BomVersions,
		ProcessRouteID:                input.ProcessRouteID,
		ProcessRouteName:              processRouteName,
		ProcessRouteOptions:           productionOptions.ProcessRoutes,
		OperationTemplateID:           input.OperationTemplateID,
		OperationTemplateName:         pricingRuleTrialOperationTemplateName(productionOptions.OperationTemplates, input.OperationTemplateID),
		OperationTemplateOptions:      productionOptions.OperationTemplates,
		BomUsageMode:                  input.BomUsageMode,
		BomStatus:                     input.BomStatus,
		BaseCost:                      pricingRuleTrialResultAmount(formulaMode, baseCost),
		BomCostTotal:                  pricingRuleTrialResultAmount(formulaMode, bomCostTotal),
		OperationCostTotal:            pricingRuleTrialResultAmount(formulaMode, operationCostTotal),
		MaterialUnitCost:              materialUnitCost,
		OperationUnitCost:             operationUnitCost,
		StandardManufacturingUnitCost: standardManufacturingUnitCost,
		CostSource:                    baseSource,
		BomSnapshot:                   PricingRuleTrialBomSnapshot{VersionID: input.BomVersionID, VersionNo: input.BomVersionNo, UsageMode: input.BomUsageMode, Status: input.BomStatus},
		ProcessRouteSnapshot:          PricingRuleTrialProcessRouteSnapshot{ID: input.ProcessRouteID, Name: processRouteName},
		WorkstationCostSnapshot:       pricingRuleTrialWorkstationCostSnapshot(baseCostDetails, materialUnitCost, operationUnitCost, standardManufacturingUnitCost, quoteUnit),
		BaseCostDetails:               baseCostDetails,
		OtherCostTotal:                pricingRuleTrialResultAmount(formulaMode, otherCostTotal),
		OtherCostDetails:              pricingRuleTrialOtherCostDetails(otherCosts, formulaMode, quoteUnit, pricingRuleTrialOtherCostSource(cmd.Overrides.OtherCosts)),
		CostBaseTotal:                 pricingRuleTrialResultAmount(formulaMode, costBaseTotal),
		CostAfterYield:                pricingRuleTrialResultAmount(formulaMode, costAfterYield),
		YieldLossAmount:               pricingRuleTrialResultAmount(formulaMode, yieldLossAmount),
		PriceAfterMarkup:              pricingRuleTrialResultAmount(formulaMode, priceAfterMarkup),
		ProfitMarkupAmount:            pricingRuleTrialResultAmount(formulaMode, profitMarkupAmount),
		ProfitExplanation:             pricingRuleTrialProfitExplanation(formulaMode, profitMethod, quoteUnit, marginRate, profitParameterRate, pricingRuleTrialOverrideSource(cmd.Overrides.MarginRate), pricingRuleTrialResultAmount(formulaMode, costAfterYield), pricingRuleTrialResultAmount(formulaMode, profitMarkupAmount), pricingRuleTrialResultAmount(formulaMode, preTaxPrice), pricingRuleTrialResultAmount(formulaMode, priceAfterMarkup), pricingRuleTrialResultAmount(formulaMode, postMarkupCostTotal)),
		PostMarkupCostTotal:           pricingRuleTrialResultAmount(formulaMode, postMarkupCostTotal),
		PreTaxPrice:                   pricingRuleTrialResultAmount(formulaMode, preTaxPrice),
		TaxAmount:                     pricingRuleTrialResultAmount(formulaMode, taxAmount),
		TaxInPriceAmount:              pricingRuleTrialResultAmount(formulaMode, taxInPriceAmount),
		TaxRateSource:                 taxRateSource,
		FinalBeforeRounding:           finalBeforeRoundingRounded,
		RoundingAdjustment:            pricingRuleTrialResultAmount(formulaMode, roundingAdjustment),
		RoundingRuleSource:            "pricing_rule",
		FinalUnitPrice:                finalUnitPrice,
		GrossMarginRate:               roundRatio(grossMarginRate),
		MinimumMarginRate:             roundRatio(minimumMarginRate),
		Warnings:                      warnings,
	}
	if formulaMode == "supplier_tier_markup" {
		result.Steps = pricingRuleTrialSupplierSteps(result, cmd, quoteUnit, baseSource, otherCosts, postMarkupCosts, expectedLossRate, yieldMode, lossChanged, marginRate, profitParameterRate, taxMode, taxRate, taxRateSource, taxRateChanged, finalBeforeRounding)
	} else {
		result.Steps = []domain.PriceExplanationStep{
			{Key: "standard_manufacturing_cost", Label: "标准制造成本", Source: baseSource, Value: result.BaseCost, Unit: quoteUnit, Changed: cmd.Overrides.BaseCost != nil},
			{Key: "other_cost_total", Label: "其他成本", Source: pricingRuleTrialOtherCostSource(cmd.Overrides.OtherCosts), Value: result.OtherCostTotal, Unit: quoteUnit, Changed: cmd.Overrides.OtherCosts != nil},
			{Key: "expected_loss_rate", Label: "预期损耗率", Source: pricingRuleTrialLossSource(cmd.Overrides.ExpectedLossRate, yieldMode), Value: roundRatio(expectedLossRate), Unit: "ratio", Changed: lossChanged},
			{Key: "cost_after_yield", Label: "损耗后成本", Source: "formula", Value: result.CostAfterYield, Unit: quoteUnit, Changed: expectedLossRate > 0 && yieldMode != "none"},
			{Key: "profit_method", Label: pricingRuleTrialProfitLabel(profitMethod), Source: pricingRuleTrialOverrideSource(cmd.Overrides.MarginRate), Value: roundBeanListPrice(preTaxPrice), Unit: quoteUnit, Changed: cmd.Overrides.MarginRate != nil},
			{Key: "tax_rate", Label: pricingRuleTrialTaxLabel(taxMode), Source: taxRateSource, Value: roundRatio(taxRate), Unit: "ratio", Changed: taxRateChanged},
			{Key: "rounding_rule", Label: "取整规则", Source: "pricing_rule", Value: finalBeforeRounding, Unit: quoteUnit, Changed: finalUnitPrice != roundBeanListPrice(finalBeforeRounding)},
			{Key: "final_unit_price", Label: "试算单价", Source: "formula", Value: finalUnitPrice, Unit: quoteUnit, Changed: true},
		}
	}
	result.FormulaExpression, result.FormulaExpressionLines = pricingRuleTrialFormulaExpression(
		result,
		formulaMode,
		quoteUnit,
		profitMethod,
		taxMode,
		taxRate,
		rule.RoundingMode,
		expectedLossRate,
		yieldMode,
		marginRate,
		pricingRuleTrialNumber(calc, "profit_parameter_rate", 0),
		finalBeforeRounding,
	)
	return result, nil
}

func pricingRuleTrialBaseCost(input domain.ProductInput, quoteUnit string) (float64, string, string) {
	perKgCost := input.GreenBeanCostPerKg + input.OperationCostPerKg
	perUnitCost := input.BomCostPerUnit + input.OperationCostPerUnit
	factor := pricingRuleTrialUnitKgFactor(quoteUnit, input.UnitConversionJSON)
	if perKgCost > 0 && factor > 0 {
		return perKgCost * factor, pricingRuleTrialStandardManufacturingCostSource, ""
	}
	if perUnitCost > 0 {
		return perUnitCost, pricingRuleTrialStandardManufacturingCostSource, ""
	}
	if perKgCost > 0 {
		return perKgCost, pricingRuleTrialStandardManufacturingCostSource, "报价单位无法换算，已按 kg 试算"
	}
	return 0, pricingRuleTrialStandardManufacturingCostSource, ""
}

func pricingRuleTrialNormalizeBaseCostDetails(input domain.ProductInput, quoteUnit string, formulaMode string, baseCost float64, overridden bool, rawDetails []PricingRuleTrialBaseCostDetail) ([]PricingRuleTrialBaseCostDetail, float64, float64) {
	unit := strings.TrimSpace(quoteUnit)
	if unit == "" {
		unit = firstNonEmptyString(strings.TrimSpace(input.QuoteUnit), strings.TrimSpace(input.InventoryUnit))
	}
	if unit == "" {
		unit = "kg"
	}
	if overridden {
		amount := pricingRuleTrialResultAmount(formulaMode, baseCost)
		return []PricingRuleTrialBaseCostDetail{{
			Key:         "temporary_override:base_cost",
			Type:        "temporary_override",
			TypeLabel:   "临时覆盖",
			Name:        "临时录入标准制造成本",
			Amount:      amount,
			Unit:        unit,
			Description: fmt.Sprintf("本次试算临时覆盖标准制造成本 %s", pricingRuleTrialMoneyExpression(amount, unit)),
		}}, amount, 0
	}

	factor := pricingRuleTrialUnitKgFactor(unit, input.UnitConversionJSON)
	usePerKg := input.GreenBeanCostPerKg+input.OperationCostPerKg > 0 && factor > 0
	perUnitSourceUnit := pricingRuleTrialBaseCostPerUnitSourceUnit(input)
	out := make([]PricingRuleTrialBaseCostDetail, 0, len(rawDetails))
	bomTotal := 0.0
	operationTotal := 0.0
	for i, row := range rawDetails {
		row.Type = strings.TrimSpace(row.Type)
		if row.Type == "" {
			row.Type = "material"
		}
		pricingRuleTrialPreparePieceCostDetail(input, &row)
		if strings.TrimSpace(row.TypeLabel) == "" {
			row.TypeLabel = pricingRuleTrialBaseCostTypeLabel(row.Type)
		}
		pricingRuleTrialBaseCostDetailPreserveCostUnit(&row)
		rawAmount, amountUnit, unitScale := pricingRuleTrialBaseCostDetailQuoteAmount(row, unit, input.UnitConversionJSON, factor, usePerKg, perUnitSourceUnit)
		amount := pricingRuleTrialResultAmount(formulaMode, rawAmount)
		if amount == 0 && strings.TrimSpace(row.Name) == "" {
			continue
		}
		row.Amount = amount
		if unitScale > 0 && unitScale != 1 && pricingRuleTrialBaseCostDetailUnitCostFollowsOutput(row) {
			row.UnitCost = row.UnitCost * unitScale
		}
		row.UnitCost = pricingRuleTrialResultAmount(formulaMode, row.UnitCost)
		if strings.TrimSpace(amountUnit) != "" {
			row.Unit = amountUnit
		} else if strings.TrimSpace(row.Unit) == "" {
			row.Unit = unit
		}
		if strings.TrimSpace(row.Name) == "" {
			row.Name = row.TypeLabel
		}
		pricingRuleTrialBaseCostDetailPreserveComposition(&row)
		if strings.TrimSpace(row.Key) == "" {
			row.Key = fmt.Sprintf("%s:%d", row.Type, i+1)
		}
		if strings.TrimSpace(row.Description) == "" {
			row.Description = pricingRuleTrialBaseCostDetailDescription(row)
		}
		out = append(out, row)
		if row.Type == "operation" {
			operationTotal += amount
		} else {
			bomTotal += amount
		}
	}
	if len(out) > 0 {
		return out, pricingRuleTrialResultAmount(formulaMode, bomTotal), pricingRuleTrialResultAmount(formulaMode, operationTotal)
	}
	return nil, 0, 0
}

func pricingRuleTrialWorkstationCostSnapshot(details []PricingRuleTrialBaseCostDetail, materialUnitCost float64, operationUnitCost float64, standardManufacturingUnitCost float64, quoteUnit string) PricingRuleTrialWorkstationCostSnapshot {
	snapshot := PricingRuleTrialWorkstationCostSnapshot{
		MaterialUnitCost:              materialUnitCost,
		OperationUnitCost:             operationUnitCost,
		StandardManufacturingUnitCost: standardManufacturingUnitCost,
	}
	for _, row := range details {
		if strings.TrimSpace(row.Type) != "operation" {
			continue
		}
		snapshot.OperationRows = append(snapshot.OperationRows, PricingRuleTrialWorkstationCostSnapshotRow{
			OperationName:      row.Name,
			WorkstationName:    row.WorkstationName,
			CapacityName:       row.CapacityName,
			CostMethod:         row.CostMethod,
			PieceRate:          row.PieceRate,
			RateUnit:           row.RateUnit,
			HourlyRate:         row.HourlyRate,
			StandardMinutes:    row.StandardMinutes,
			StandardOutputQty:  row.StandardOutputQty,
			StandardOutputUnit: row.StandardOutputUnit,
			UnitCost:           row.Amount,
			Unit:               firstNonEmptyString(row.Unit, quoteUnit),
		})
	}
	return snapshot
}

func pricingRuleTrialPreparePieceCostDetail(input domain.ProductInput, row *PricingRuleTrialBaseCostDetail) {
	if row == nil || !strings.EqualFold(strings.TrimSpace(row.CostMethod), "piece") {
		return
	}
	row.CostMethod = "piece"
	if row.PieceRate <= 0 {
		row.AmountPerUnit = 0
		row.UnitCost = 0
		row.Warning = "BOM 计件工序成本快照缺少有效计件单价"
		return
	}
	salesUnit := firstNonEmptyString(strings.TrimSpace(input.QuoteUnit), strings.TrimSpace(input.OrderUnit))
	if salesUnit == "" {
		row.AmountPerUnit = 0
		row.UnitCost = 0
		row.Warning = "具体 SKU 缺少权威销售规格单位，无法折算计件工序成本"
		return
	}
	inventoryUnit := strings.TrimSpace(input.InventoryUnit)
	if inventoryUnit != "" && pricingRuleTrialUnitKey(salesUnit) != pricingRuleTrialUnitKey(inventoryUnit) &&
		pricingRuleTrialUnitTargetFactor(salesUnit, inventoryUnit, input.UnitConversionJSON) <= 0 {
		row.AmountPerUnit = 0
		row.UnitCost = 0
		row.Warning = "具体 SKU 缺少权威 inventory_qty_per_sales_unit，无法折算计件工序成本"
		return
	}
	row.RateUnit = firstNonEmptyString(strings.TrimSpace(row.RateUnit), "sales_spec_count")
	row.ConsumeUnit = "per_sales_unit"
	row.AmountPerUnit = row.PieceRate
	row.UnitCost = row.PieceRate
	row.Unit = salesUnit
	row.CostUnit = salesUnit
	row.CostUnitCost = row.PieceRate
}

func pricingRuleTrialBaseCostDetailQuoteAmount(row PricingRuleTrialBaseCostDetail, quoteUnit string, conversionJSON string, quoteKgFactor float64, usePerKg bool, perUnitSourceUnit string) (float64, string, float64) {
	quoteUnit = strings.TrimSpace(quoteUnit)
	rowUnit := strings.TrimSpace(row.Unit)
	if row.Amount != 0 {
		if amount, unit, scale, ok := pricingRuleTrialConvertCostAmount(row.Amount, rowUnit, quoteUnit, conversionJSON); ok {
			return amount, unit, scale
		}
		return row.Amount, rowUnit, 1
	}
	if usePerKg && row.AmountPerKg > 0 {
		return row.AmountPerKg * quoteKgFactor, quoteUnit, quoteKgFactor
	}
	if row.AmountPerUnit > 0 {
		sourceUnit := firstNonEmptyString(rowUnit, perUnitSourceUnit)
		if amount, unit, scale, ok := pricingRuleTrialConvertCostAmount(row.AmountPerUnit, sourceUnit, quoteUnit, conversionJSON); ok {
			return amount, unit, scale
		}
		return row.AmountPerUnit, firstNonEmptyString(sourceUnit, quoteUnit), 1
	}
	if row.AmountPerKg > 0 && quoteKgFactor > 0 {
		return row.AmountPerKg * quoteKgFactor, quoteUnit, quoteKgFactor
	}
	if row.AmountPerKg > 0 {
		return row.AmountPerKg, firstNonEmptyString(rowUnit, "kg"), 1
	}
	return 0, firstNonEmptyString(rowUnit, quoteUnit), 1
}

func pricingRuleTrialBaseCostPerUnitSourceUnit(input domain.ProductInput) string {
	for _, unit := range []string{input.QuoteUnit, input.OrderUnit, input.InventoryUnit} {
		unit = strings.TrimSpace(unit)
		if unit == "" {
			continue
		}
		if pricingRuleTrialUnitKgFactor(unit, input.UnitConversionJSON) > 0 {
			return unit
		}
	}
	return firstNonEmptyString(input.QuoteUnit, input.OrderUnit, input.InventoryUnit)
}

func pricingRuleTrialConvertCostAmount(amount float64, sourceUnit string, targetUnit string, conversionJSON string) (float64, string, float64, bool) {
	sourceUnit = strings.TrimSpace(sourceUnit)
	targetUnit = strings.TrimSpace(targetUnit)
	if amount == 0 || sourceUnit == "" || targetUnit == "" {
		return amount, "", 1, false
	}
	if pricingRuleTrialUnitKey(sourceUnit) == pricingRuleTrialUnitKey(targetUnit) {
		return amount, targetUnit, 1, true
	}
	if scale := pricingRuleTrialUnitTargetFactor(targetUnit, sourceUnit, conversionJSON); scale > 0 {
		return amount * scale, targetUnit, scale, true
	}
	sourceKgFactor := pricingRuleTrialUnitKgFactor(sourceUnit, conversionJSON)
	targetKgFactor := pricingRuleTrialUnitKgFactor(targetUnit, conversionJSON)
	if sourceKgFactor <= 0 || targetKgFactor <= 0 {
		return amount, "", 1, false
	}
	scale := targetKgFactor / sourceKgFactor
	return amount * scale, targetUnit, scale, true
}

func pricingRuleTrialBaseCostDetailUnitCostFollowsOutput(row PricingRuleTrialBaseCostDetail) bool {
	switch strings.TrimSpace(row.ConsumeUnit) {
	case "ratio_pct", "per_kg", "per_kg_output", "per_finished_kg", "per_inventory_unit", "per_sales_unit":
		return true
	default:
		return false
	}
}

func pricingRuleTrialBaseCostDetailPreserveCostUnit(row *PricingRuleTrialBaseCostDetail) {
	if row == nil || row.UnitCost == 0 {
		return
	}
	if row.CostUnitCost == 0 {
		row.CostUnitCost = row.UnitCost
	}
	if strings.TrimSpace(row.CostUnit) == "" {
		row.CostUnit = pricingRuleTrialBaseCostDetailDefaultCostUnit(*row)
	}
}

func pricingRuleTrialBaseCostDetailPreserveComposition(row *PricingRuleTrialBaseCostDetail) {
	if row == nil || strings.TrimSpace(row.ConsumeUnit) != "ratio_pct" {
		return
	}
	lossRate := row.MaterialLossRate
	if row.EffectiveRatioPct == 0 {
		row.EffectiveRatioPct = row.RatioPct
	}
	if row.RecipeRatioPct == 0 {
		if lossRate > 0 && lossRate < 1 && row.EffectiveRatioPct > 0 {
			row.RecipeRatioPct = row.EffectiveRatioPct * (1 - lossRate)
		} else {
			row.RecipeRatioPct = row.EffectiveRatioPct
		}
	}
	if row.EffectiveRatioPct == 0 && row.RecipeRatioPct > 0 {
		if lossRate > 0 && lossRate < 1 {
			row.EffectiveRatioPct = row.RecipeRatioPct / (1 - lossRate)
		} else {
			row.EffectiveRatioPct = row.RecipeRatioPct
		}
	}
}

func pricingRuleTrialBaseCostDetailDefaultCostUnit(row PricingRuleTrialBaseCostDetail) string {
	if unit := strings.TrimSpace(row.Unit); unit != "" {
		return unit
	}
	switch strings.TrimSpace(row.Type) {
	case "operation":
		if row.AmountPerKg > 0 {
			return "kg"
		}
		switch strings.TrimSpace(row.ConsumeUnit) {
		case "per_kg", "per_kg_output", "per_finished_kg":
			return "kg"
		default:
			return ""
		}
	default:
		return "kg"
	}
}

func pricingRuleTrialBaseCostTypeLabel(value string) string {
	switch strings.TrimSpace(value) {
	case "operation":
		return "工序"
	case "component_product", "product", "finished_product":
		return "成品组件"
	case "temporary_override":
		return "临时覆盖"
	default:
		return "物料"
	}
}

func pricingRuleTrialBaseCostDetailDescription(row PricingRuleTrialBaseCostDetail) string {
	unit := strings.TrimSpace(row.Unit)
	if unit == "" {
		unit = "单位"
	}
	costUnit := strings.TrimSpace(row.CostUnit)
	if costUnit == "" {
		costUnit = unit
	}
	unitCost := row.UnitCost
	if row.CostUnitCost != 0 {
		unitCost = row.CostUnitCost
	}
	amountLabel := "金额"
	if strings.TrimSpace(row.CostUnit) != "" && pricingRuleTrialUnitKey(costUnit) != pricingRuleTrialUnitKey(unit) {
		amountLabel = "折算金额"
	}
	unitCostText := pricingRuleTrialMoneyExpression(unitCost, costUnit)
	amountText := pricingRuleTrialMoneyExpression(row.Amount, unit)
	switch strings.TrimSpace(row.ConsumeUnit) {
	case "ratio_pct":
		if row.MaterialLossRate > 0 && row.MaterialLossRate < 1 {
			recipeRatio := row.RecipeRatioPct
			if recipeRatio == 0 {
				recipeRatio = row.RatioPct * (1 - row.MaterialLossRate)
			}
			effectiveRatio := row.EffectiveRatioPct
			if effectiveRatio == 0 {
				effectiveRatio = row.RatioPct
			}
			return fmt.Sprintf("%s：%s，原比例 %s%%，原料损耗 %s%%，有效比例 %s%%，单位成本 %s，%s %s", row.TypeLabel, row.Name, pricingRuleTrialNumberExpression(recipeRatio), pricingRuleTrialNumberExpression(row.MaterialLossRate*100), pricingRuleTrialNumberExpression(effectiveRatio), unitCostText, amountLabel, amountText)
		}
		recipeRatio := row.RecipeRatioPct
		if recipeRatio == 0 {
			recipeRatio = row.RatioPct
		}
		return fmt.Sprintf("%s：%s，比例 %s%%，单位成本 %s，%s %s", row.TypeLabel, row.Name, pricingRuleTrialNumberExpression(recipeRatio), unitCostText, amountLabel, amountText)
	case "g_per_bag", "unit_per_bag", "unit_per_box", "per_kg", "per_unit", "per_quote_unit":
		return fmt.Sprintf("%s：%s，用量 %s，单位成本 %s，%s %s", row.TypeLabel, row.Name, pricingRuleTrialNumberExpression(row.Quantity), unitCostText, amountLabel, amountText)
	default:
		return fmt.Sprintf("%s：%s，%s %s", row.TypeLabel, row.Name, amountLabel, amountText)
	}
}

func pricingRuleTrialFormulaExpression(result *PricingRuleTrialResult, formulaMode string, quoteUnit string, _ string, taxMode string, taxRate float64, roundingMode string, expectedLossRate float64, yieldMode string, marginRate float64, profitParameterRate float64, finalBeforeRounding float64) (string, []string) {
	if result == nil {
		return "", nil
	}
	unit := firstNonEmptyString(quoteUnit, result.QuoteUnit)
	baseTerm := fmt.Sprintf("标准制造成本 %s", pricingRuleTrialMoneyExpression(result.BaseCost, unit))
	otherTerm := fmt.Sprintf("其他成本 %s", pricingRuleTrialMoneyExpression(result.OtherCostTotal, unit))
	if strings.TrimSpace(formulaMode) == "supplier_tier_markup" {
		otherTerm = fmt.Sprintf("生产项目成本 %s", pricingRuleTrialMoneyExpression(result.OtherCostTotal, unit))
	}
	costBaseExpr := fmt.Sprintf("(%s + %s)", baseTerm, otherTerm)
	lines := []string{
		fmt.Sprintf("成本基数 = %s = %s", strings.Trim(costBaseExpr, "()"), pricingRuleTrialMoneyExpression(result.BaseCost+result.OtherCostTotal, unit)),
	}
	currentExpr := costBaseExpr
	if strings.TrimSpace(yieldMode) != "none" && expectedLossRate > 0 {
		currentExpr = fmt.Sprintf("%s / (1 - 损耗率 %s)", currentExpr, pricingRuleTrialPercentExpression(expectedLossRate))
		lines = append(lines, fmt.Sprintf("损耗后成本 = %s = %s", currentExpr, pricingRuleTrialMoneyExpression(result.CostAfterYield, unit)))
	} else {
		lines = append(lines, fmt.Sprintf("损耗后成本 = %s", pricingRuleTrialMoneyExpression(result.CostAfterYield, unit)))
	}

	if strings.TrimSpace(formulaMode) == "supplier_tier_markup" {
		currentExpr = fmt.Sprintf("%s * (1 + 档位加价率 %s", currentExpr, pricingRuleTrialPercentExpression(marginRate))
		if profitParameterRate != 0 {
			currentExpr = fmt.Sprintf("%s + 加价参数 %s", currentExpr, pricingRuleTrialPercentExpression(profitParameterRate))
		}
		currentExpr += ")"
		lines = append(lines, fmt.Sprintf("加价后价格 = %s = %s", currentExpr, pricingRuleTrialMoneyExpression(result.PriceAfterMarkup, unit)))
		if result.PostMarkupCostTotal > 0 {
			currentExpr = fmt.Sprintf("(%s + 加价附加成本 %s)", currentExpr, pricingRuleTrialMoneyExpression(result.PostMarkupCostTotal, unit))
			lines = append(lines, fmt.Sprintf("税前价 = 加价后价格 %s + 加价附加成本 %s = %s", pricingRuleTrialMoneyExpression(result.PriceAfterMarkup, unit), pricingRuleTrialMoneyExpression(result.PostMarkupCostTotal, unit), pricingRuleTrialMoneyExpression(result.PreTaxPrice, unit)))
		} else {
			lines = append(lines, fmt.Sprintf("税前价 = %s", pricingRuleTrialMoneyExpression(result.PreTaxPrice, unit)))
		}
	} else {
		currentExpr = fmt.Sprintf("%s * (1 + 加价率 %s)", currentExpr, pricingRuleTrialPercentExpression(marginRate))
		lines = append(lines, fmt.Sprintf("税前价 = %s = %s", currentExpr, pricingRuleTrialMoneyExpression(result.PreTaxPrice, unit)))
	}

	switch strings.TrimSpace(taxMode) {
	case "tax_included":
		currentExpr = fmt.Sprintf("(%s) * (1 + 税率 %s)", currentExpr, pricingRuleTrialPercentExpression(taxRate))
		lines = append(lines, fmt.Sprintf("含税价 = 税前价 %s + 税额 %s = %s", pricingRuleTrialMoneyExpression(result.PreTaxPrice, unit), pricingRuleTrialMoneyExpression(result.TaxAmount, unit), pricingRuleTrialMoneyExpression(finalBeforeRounding, unit)))
	case "tax_excluded":
		lines = append(lines, fmt.Sprintf("未税价 = %s；税额单独提示 %s", pricingRuleTrialMoneyExpression(result.PreTaxPrice, unit), pricingRuleTrialMoneyExpression(result.TaxAmount, unit)))
	default:
		lines = append(lines, fmt.Sprintf("不计税价格 = %s", pricingRuleTrialMoneyExpression(finalBeforeRounding, unit)))
	}
	if pricingRuleTrialRoundingApplied(roundingMode) && result.FinalUnitPrice != roundBeanListPrice(finalBeforeRounding) {
		lines = append(lines, fmt.Sprintf("取整规则 = %s：%s -> %s", pricingRuleTrialRoundingExpression(roundingMode), pricingRuleTrialMoneyExpression(finalBeforeRounding, unit), pricingRuleTrialMoneyExpression(result.FinalUnitPrice, unit)))
	}
	lines = append(lines, fmt.Sprintf("最终售价 = %s", pricingRuleTrialMoneyExpression(result.FinalUnitPrice, unit)))
	formulaExpr := currentExpr
	if pricingRuleTrialRoundingApplied(roundingMode) {
		formulaExpr = fmt.Sprintf("%s -> %s", formulaExpr, pricingRuleTrialRoundingExpression(roundingMode))
	}
	expression := fmt.Sprintf("最终售价 = %s；公式：%s", pricingRuleTrialMoneyExpression(result.FinalUnitPrice, unit), formulaExpr)
	return expression, lines
}

func pricingRuleTrialMoneyExpression(value float64, unit string) string {
	formatted := pricingRuleTrialNumberExpression(roundBeanListPrice(value))
	if strings.TrimSpace(unit) == "" {
		return formatted
	}
	return fmt.Sprintf("%s/%s", formatted, strings.TrimSpace(unit))
}

func pricingRuleTrialPercentExpression(value float64) string {
	return pricingRuleTrialNumberExpression(roundPricingRuleTrialDetail(value*100)) + "%"
}

func pricingRuleTrialNumberExpression(value float64) string {
	formatted := strconv.FormatFloat(value, 'f', 4, 64)
	formatted = strings.TrimRight(formatted, "0")
	formatted = strings.TrimRight(formatted, ".")
	if formatted == "-0" {
		return "0"
	}
	return formatted
}

func pricingRuleTrialRoundingApplied(mode string) bool {
	switch strings.TrimSpace(mode) {
	case "yuan", "jiao":
		return true
	default:
		return false
	}
}

func pricingRuleTrialRoundingExpression(mode string) string {
	switch strings.TrimSpace(mode) {
	case "yuan":
		return "保留到元"
	case "jiao":
		return "保留到角"
	default:
		return "不取整"
	}
}

func pricingRuleTrialUnitKey(unit string) string {
	return strings.ToLower(strings.TrimSpace(unit))
}

func pricingRuleTrialMassKgFactor(unit string) float64 {
	switch pricingRuleTrialUnitKey(unit) {
	case "kg", "公斤", "千克":
		return 1
	case "g", "克":
		return 0.001
	case "lb", "lbs", "磅":
		return 0.45359237
	default:
		return 0
	}
}

func pricingRuleTrialMassConversionFactor(fromUnit string, toUnit string) float64 {
	source := pricingRuleTrialMassKgFactor(fromUnit)
	target := pricingRuleTrialMassKgFactor(toUnit)
	if source <= 0 || target <= 0 {
		return 0
	}
	return source / target
}

func pricingRuleTrialUnitTargetFactor(unit string, targetUnit string, conversionJSON string) float64 {
	sourceKey := pricingRuleTrialUnitKey(unit)
	targetKey := pricingRuleTrialUnitKey(targetUnit)
	if sourceKey == "" {
		if targetKey == "" || targetKey == "kg" || targetKey == "公斤" || targetKey == "千克" {
			return 1
		}
		return 0
	}
	if targetKey == "" {
		targetKey = "kg"
	}
	if sourceKey == targetKey {
		return 1
	}
	if factor := pricingRuleTrialMassConversionFactor(unit, targetUnit); factor > 0 {
		return factor
	}
	conversionJSON = strings.TrimSpace(conversionJSON)
	if conversionJSON == "" {
		return 0
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(conversionJSON), &raw); err != nil {
		return 0
	}
	graph := map[string]map[string]float64{}
	addEdge := func(from string, to string, factor float64) {
		fromKey := pricingRuleTrialUnitKey(from)
		toKey := pricingRuleTrialUnitKey(to)
		if fromKey == "" || toKey == "" || factor <= 0 {
			return
		}
		if graph[fromKey] == nil {
			graph[fromKey] = map[string]float64{}
		}
		graph[fromKey][toKey] = factor
	}
	for from, value := range raw {
		switch typed := value.(type) {
		case float64:
			addEdge(from, targetKey, typed)
		case int:
			addEdge(from, targetKey, float64(typed))
		case map[string]any:
			for to, rawFactor := range typed {
				addEdge(from, to, numberFromAny(rawFactor))
			}
		}
	}
	return pricingRuleTrialResolveUnitTargetFactor(sourceKey, targetKey, graph, map[string]bool{})
}

func pricingRuleTrialResolveUnitTargetFactor(sourceKey string, targetKey string, graph map[string]map[string]float64, seen map[string]bool) float64 {
	sourceKey = pricingRuleTrialUnitKey(sourceKey)
	targetKey = pricingRuleTrialUnitKey(targetKey)
	if sourceKey == "" || targetKey == "" {
		return 0
	}
	if sourceKey == targetKey {
		return 1
	}
	if factor := pricingRuleTrialMassConversionFactor(sourceKey, targetKey); factor > 0 {
		return factor
	}
	if seen[sourceKey] {
		return 0
	}
	seen[sourceKey] = true
	for nextKey, factor := range graph[sourceKey] {
		if factor <= 0 {
			continue
		}
		if targetFactor := pricingRuleTrialResolveUnitTargetFactor(nextKey, targetKey, graph, seen); targetFactor > 0 {
			return factor * targetFactor
		}
	}
	return 0
}

func pricingRuleTrialUnitKgFactor(unit string, conversionJSON string) float64 {
	return pricingRuleTrialUnitTargetFactor(unit, "kg", conversionJSON)
}

func pricingRuleTrialOtherCostMap(calc map[string]any) map[string]float64 {
	out := map[string]float64{}
	raw, ok := calc["other_costs"]
	if !ok {
		raw = calc["otherCosts"]
	}
	switch typed := raw.(type) {
	case map[string]any:
		for key, value := range typed {
			name := strings.TrimSpace(key)
			if name == "" {
				continue
			}
			out[name] = numberFromAny(value)
		}
	case map[string]float64:
		for key, value := range typed {
			name := strings.TrimSpace(key)
			if name == "" {
				continue
			}
			out[name] = value
		}
	}
	return out
}

func pricingRuleTrialPostMarkupCostMap(calc map[string]any) map[string]float64 {
	out := map[string]float64{}
	raw, ok := calc["post_markup_costs"]
	if !ok {
		raw = calc["postMarkupCosts"]
	}
	switch typed := raw.(type) {
	case map[string]any:
		for key, value := range typed {
			name := strings.TrimSpace(key)
			if name == "" {
				continue
			}
			out[name] = numberFromAny(value)
		}
	case map[string]float64:
		for key, value := range typed {
			name := strings.TrimSpace(key)
			if name == "" {
				continue
			}
			out[name] = value
		}
	}
	return out
}

func pricingRuleTrialOtherCostDetails(costs map[string]float64, formulaMode string, unit string, source string) []PricingRuleTrialOtherCostDetail {
	if len(costs) == 0 {
		return nil
	}
	names := make([]string, 0, len(costs))
	for name := range costs {
		name = strings.TrimSpace(name)
		if name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	location := "价格计算模板编辑区「其他成本」"
	if strings.TrimSpace(source) == "temporary_override" {
		location = "本次试算抽屉「其他成本」"
	}
	out := make([]PricingRuleTrialOtherCostDetail, 0, len(names))
	for _, name := range names {
		out = append(out, PricingRuleTrialOtherCostDetail{
			Name:            name,
			Amount:          pricingRuleTrialResultAmount(formulaMode, costs[name]),
			Unit:            unit,
			Source:          source,
			SettingLocation: location,
		})
	}
	return out
}

func pricingRuleTrialProfitExplanation(formulaMode string, _ string, unit string, marginRate float64, profitParameterRate float64, source string, costAfterYield float64, markupAmount float64, preTaxPrice float64, priceAfterMarkup float64, postMarkupCostTotal float64) PricingRuleTrialProfitExplanation {
	mode := strings.TrimSpace(formulaMode)
	if mode == "supplier_tier_markup" {
		rate := marginRate + profitParameterRate
		formula := fmt.Sprintf("加价后价格 = 损耗后成本 * (1 + 档位加价率 %s) = %s", pricingRuleTrialPercentExpression(rate), pricingRuleTrialMoneyExpression(priceAfterMarkup, unit))
		if postMarkupCostTotal > 0 {
			formula += fmt.Sprintf("；税前价 = 加价后价格 + 加价附加成本 %s = %s", pricingRuleTrialMoneyExpression(postMarkupCostTotal, unit), pricingRuleTrialMoneyExpression(preTaxPrice, unit))
		}
		return PricingRuleTrialProfitExplanation{
			Method:         "supplier_tier_markup",
			MethodLabel:    "档位加价率",
			Rate:           roundRatio(rate),
			Source:         source,
			CostAfterYield: costAfterYield,
			MarkupAmount:   markupAmount,
			PreTaxPrice:    preTaxPrice,
			Formula:        formula,
		}
	}
	formula := fmt.Sprintf("税前价 = 损耗后成本 * (1 + 加价率 %s) = %s", pricingRuleTrialPercentExpression(marginRate), pricingRuleTrialMoneyExpression(preTaxPrice, unit))
	return PricingRuleTrialProfitExplanation{
		Method:         "markup",
		MethodLabel:    "加价率",
		Rate:           roundRatio(marginRate),
		Source:         source,
		CostAfterYield: costAfterYield,
		MarkupAmount:   markupAmount,
		PreTaxPrice:    preTaxPrice,
		Formula:        formula,
	}
}

func pricingRuleTrialSupplierSteps(result *PricingRuleTrialResult, cmd PricingRuleTrialCommand, quoteUnit string, baseSource string, preMarkupCosts map[string]float64, postMarkupCosts map[string]float64, expectedLossRate float64, yieldMode string, lossChanged bool, marginRate float64, profitParameterRate float64, taxMode string, taxRate float64, taxRateSource string, taxRateChanged bool, finalBeforeRounding float64) []domain.PriceExplanationStep {
	steps := []domain.PriceExplanationStep{
		{Key: "material_cost", Label: "物料/BOM成本", Source: baseSource, Value: result.BaseCost, Unit: quoteUnit, Changed: cmd.Overrides.BaseCost != nil},
		{Key: "other_cost_total", Label: "生产项目成本", Source: pricingRuleTrialOtherCostSource(cmd.Overrides.OtherCosts), Value: result.OtherCostTotal, Unit: quoteUnit, Changed: cmd.Overrides.OtherCosts != nil},
	}
	steps = appendPricingRuleTrialCostSteps(steps, "other_cost", "生产项目", pricingRuleTrialOtherCostSource(cmd.Overrides.OtherCosts), preMarkupCosts, quoteUnit, cmd.Overrides.OtherCosts != nil)
	steps = append(steps,
		domain.PriceExplanationStep{Key: "expected_loss_rate", Label: "预期损耗率", Source: pricingRuleTrialLossSource(cmd.Overrides.ExpectedLossRate, yieldMode), Value: roundRatio(expectedLossRate), Unit: "ratio", Changed: lossChanged},
		domain.PriceExplanationStep{Key: "cost_after_yield", Label: "成本基数", Source: "formula", Value: result.CostAfterYield, Unit: quoteUnit, Changed: expectedLossRate > 0 && yieldMode != "none"},
		domain.PriceExplanationStep{Key: "tier_markup_rate", Label: "档位加价率", Source: pricingRuleTrialOverrideSource(cmd.Overrides.MarginRate), Value: roundRatio(marginRate), Unit: "ratio", Changed: cmd.Overrides.MarginRate != nil},
	)
	if profitParameterRate != 0 {
		steps = append(steps, domain.PriceExplanationStep{Key: "profit_parameter_rate", Label: "加价参数", Source: "pricing_rule", Value: roundRatio(profitParameterRate), Unit: "ratio"})
	}
	steps = append(steps,
		domain.PriceExplanationStep{Key: "price_after_markup", Label: "加价后价格", Source: "formula", Value: result.PriceAfterMarkup, Unit: quoteUnit, Changed: true},
		domain.PriceExplanationStep{Key: "post_markup_cost_total", Label: "加价附加成本", Source: pricingRuleTrialPostMarkupCostSource(cmd.Overrides.PostMarkupCosts), Value: result.PostMarkupCostTotal, Unit: quoteUnit, Changed: cmd.Overrides.PostMarkupCosts != nil},
	)
	steps = appendPricingRuleTrialCostSteps(steps, "post_markup_cost", "加价附加", pricingRuleTrialPostMarkupCostSource(cmd.Overrides.PostMarkupCosts), postMarkupCosts, quoteUnit, cmd.Overrides.PostMarkupCosts != nil)
	if strings.TrimSpace(taxMode) != "none" {
		steps = append(steps, domain.PriceExplanationStep{Key: "tax_rate", Label: pricingRuleTrialTaxLabel(taxMode), Source: taxRateSource, Value: roundRatio(taxRate), Unit: "ratio", Changed: taxRateChanged})
	}
	return append(steps,
		domain.PriceExplanationStep{Key: "rounding_rule", Label: "取整规则", Source: "pricing_rule", Value: finalBeforeRounding, Unit: quoteUnit, Changed: result.FinalUnitPrice != roundBeanListPrice(finalBeforeRounding)},
		domain.PriceExplanationStep{Key: "final_unit_price", Label: "试算单价", Source: "formula", Value: result.FinalUnitPrice, Unit: quoteUnit, Changed: true},
	)
}

func appendPricingRuleTrialCostSteps(steps []domain.PriceExplanationStep, keyPrefix string, labelPrefix string, source string, costs map[string]float64, unit string, changed bool) []domain.PriceExplanationStep {
	if len(costs) == 0 {
		return steps
	}
	names := make([]string, 0, len(costs))
	for name := range costs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		steps = append(steps, domain.PriceExplanationStep{
			Key:     pricingRuleTrialCostStepKey(keyPrefix, name),
			Label:   strings.TrimSpace(labelPrefix + "：" + name),
			Source:  source,
			Value:   roundPricingRuleTrialDetail(costs[name]),
			Unit:    unit,
			Changed: changed,
		})
	}
	return steps
}

func pricingRuleTrialCostStepKey(prefix string, name string) string {
	key := strings.TrimSpace(name)
	key = strings.NewReplacer(" ", "_", "\t", "_", "/", "_", "（", "_", "）", "_", "(", "_", ")", "_").Replace(key)
	key = strings.Trim(key, "_")
	if key == "" {
		key = "cost"
	}
	return strings.TrimSpace(prefix) + "_" + key
}

func pricingRuleTrialExpectedLossRate(input domain.ProductInput, override *float64, mode string) (float64, bool, error) {
	normalizedMode := strings.TrimSpace(mode)
	if normalizedMode == "" {
		normalizedMode = "bom_or_product"
	}
	if normalizedMode == "none" {
		return 0, false, nil
	}
	loss := 0.0
	changed := false
	if override != nil {
		loss = *override
		changed = true
	} else if input.ExpectedLossRate > 0 {
		loss = input.ExpectedLossRate
	} else if input.YieldRate > 0 && input.YieldRate < 1 {
		loss = 1 - input.YieldRate
	}
	if loss < 0 || loss >= 1 {
		return 0, changed, fmt.Errorf("expected_loss_rate must be >= 0 and < 1")
	}
	return loss, changed, nil
}

func pricingRuleTrialCombinedLossWarnings(warnings []string, input domain.ProductInput, details []PricingRuleTrialBaseCostDetail) []string {
	overallLossRate := input.ExpectedLossRate
	if overallLossRate <= 0 && input.YieldRate > 0 && input.YieldRate < 1 {
		overallLossRate = 1 - input.YieldRate
	}
	if overallLossRate <= 0 || overallLossRate >= 1 {
		return warnings
	}

	materialLossRate := 0.0
	for _, detail := range details {
		if detail.MaterialLossRate > materialLossRate && detail.MaterialLossRate < 1 {
			materialLossRate = detail.MaterialLossRate
		}
	}
	if materialLossRate <= 0 {
		return warnings
	}

	warning := fmt.Sprintf(
		"生产 BOM 同时设置了整体预期损耗 %s 和原料损耗 %s，两项会连续放大标准制造成本；如只需计算原料损耗，请在商品档案生产配置将整体预期损耗率设为 0，并使用整体损耗为 0 的已发布 BOM 版本。",
		pricingRuleTrialPercentExpression(overallLossRate),
		pricingRuleTrialPercentExpression(materialLossRate),
	)
	return appendUniqueString(warnings, warning)
}

func pricingRuleTrialResolvedTaxRate(rule ProductPricingRule, cmd PricingRuleTrialCommand, taxMode string, defaultTaxRate PricingRuleTrialDefaultTaxRate) (float64, string, bool, error) {
	if strings.TrimSpace(taxMode) == "none" {
		return 0, "tax_disabled", false, nil
	}
	if cmd.Overrides.TaxRate != nil {
		if *cmd.Overrides.TaxRate < 0 {
			return 0, "", false, fmt.Errorf("tax_rate must be >= 0")
		}
		return *cmd.Overrides.TaxRate, "trial_override", true, nil
	}
	if rule.TaxRate < 0 {
		return 0, "", false, fmt.Errorf("tax_rate must be >= 0")
	}
	if rule.TaxRate > 0 {
		return rule.TaxRate, "pricing_rule", false, nil
	}
	if defaultTaxRate.Rate < 0 {
		return 0, "", false, fmt.Errorf("tax_rate must be >= 0")
	}
	source := strings.TrimSpace(defaultTaxRate.Source)
	if source == "" {
		source = "finance_settings"
	}
	if defaultTaxRate.Rate > 0 {
		return defaultTaxRate.Rate, source, false, nil
	}
	return 0, "default", false, nil
}

func pricingRuleTrialSuppressDefaultLossForActualBomCost(input domain.ProductInput, cmd PricingRuleTrialCommand, baseCostDetails []PricingRuleTrialBaseCostDetail, bomCostTotal float64, operationCostTotal float64) bool {
	if cmd.Overrides.ExpectedLossRate != nil || cmd.Overrides.BaseCost != nil {
		return false
	}
	if len(baseCostDetails) == 0 || bomCostTotal+operationCostTotal <= 0 {
		return false
	}
	if input.BomVersionID <= 0 && strings.TrimSpace(input.BomUsageMode) == "" {
		return false
	}
	return true
}

func pricingRuleTrialPreTaxPrice(cost float64, marginRate float64, method string) (float64, error) {
	switch strings.TrimSpace(method) {
	case "markup":
		return cost * (1 + marginRate), nil
	default:
		return 0, fmt.Errorf("only markup rate is supported")
	}
}

func pricingRuleTrialTax(preTaxPrice float64, taxRate float64, mode string) (float64, float64) {
	switch strings.TrimSpace(mode) {
	case "tax_included":
		final := preTaxPrice * (1 + taxRate)
		return final, final - preTaxPrice
	case "tax_excluded":
		return preTaxPrice, preTaxPrice * taxRate
	default:
		return preTaxPrice, 0
	}
}

func pricingRuleTrialRoundedPrice(value float64, rounding string) float64 {
	switch strings.TrimSpace(rounding) {
	case "yuan":
		return math.Round(value)
	case "jiao":
		return math.Round(value*10) / 10
	default:
		return roundBeanListPrice(value)
	}
}

func pricingRuleTrialString(calc map[string]any, key string, fallback string) string {
	if calc == nil {
		return fallback
	}
	value := strings.TrimSpace(fmt.Sprint(calc[key]))
	if value == "" || value == "<nil>" {
		return fallback
	}
	return value
}

func pricingRuleTrialNumber(calc map[string]any, key string, fallback float64) float64 {
	if calc == nil {
		return fallback
	}
	if value, ok := calc[key]; ok {
		return numberFromAny(value)
	}
	return fallback
}

func numberFromAny(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		n, _ := typed.Float64()
		return n
	case string:
		n, _ := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return n
	default:
		return 0
	}
}

func pricingRuleTrialOtherCostSource(overrides map[string]float64) string {
	if overrides != nil {
		return "temporary_override"
	}
	return "pricing_rule"
}

func pricingRuleTrialPostMarkupCostSource(overrides map[string]float64) string {
	if overrides != nil {
		return "temporary_override"
	}
	return "pricing_rule"
}

func pricingRuleTrialLossSource(override *float64, mode string) string {
	if override != nil {
		return "temporary_override"
	}
	if strings.TrimSpace(mode) == "none" {
		return "pricing_rule"
	}
	return "product_bom"
}

func pricingRuleTrialOverrideSource(value *float64) string {
	if value != nil {
		return "temporary_override"
	}
	return "pricing_rule"
}

func pricingRuleTrialProfitLabel(_ string) string {
	return "加价率"
}

func pricingRuleTrialTaxLabel(mode string) string {
	switch strings.TrimSpace(mode) {
	case "tax_excluded":
		return "未税税额"
	case "none":
		return "不计税"
	default:
		return "含税税率"
	}
}

func appendUniqueString(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func roundRatio(value float64) float64 {
	return math.Round((value+1e-12)*10000) / 10000
}

func pricingRuleTrialResultAmount(formulaMode string, value float64) float64 {
	if strings.TrimSpace(formulaMode) == "supplier_tier_markup" {
		return roundPricingRuleTrialDetail(value)
	}
	return roundBeanListPrice(value)
}

func roundPricingRuleTrialDetail(value float64) float64 {
	return math.Round((value+1e-12)*10000) / 10000
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (s *Service) ExplainDripPrice(ctx context.Context, req DripPriceExplanationCommand) (*domain.DripPriceExplanation, error) {
	params, err := s.Parameters(ctx)
	if err != nil {
		return nil, err
	}
	out, err := domain.ExplainDripPrice(params, req.Product, domain.PriceExplanationRequest{TierLabel: req.TierLabel})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Service) ListDripPriceTemplates(ctx context.Context) ([]domain.DripPriceTemplate, error) {
	if s.repo == nil {
		return []domain.DripPriceTemplate{}, nil
	}
	return s.repo.ListDripPriceTemplates(ctx)
}

func (s *Service) SaveDripPriceTemplate(ctx context.Context, cmd SaveDripPriceTemplateCommand) (*domain.DripPriceTemplate, error) {
	normalized, err := normalizeDripPriceTemplateCommand(cmd)
	if err != nil {
		return nil, err
	}
	if s.repo == nil {
		return nil, fmt.Errorf("repository required")
	}
	return s.repo.SaveDripPriceTemplate(ctx, normalized)
}

func (s *Service) DeactivateDripPriceTemplate(ctx context.Context, cmd DeactivateDripPriceTemplateCommand) error {
	if cmd.ID <= 0 {
		return fmt.Errorf("invalid id")
	}
	if s.repo == nil {
		return fmt.Errorf("repository required")
	}
	return s.repo.DeactivateDripPriceTemplate(ctx, cmd)
}

func (s *Service) BeanList(ctx context.Context, query BeanListQuery) (*CalculateResponse, error) {
	if query.CustomerID < 0 {
		return nil, fmt.Errorf("customer_id must be >= 0")
	}
	params, err := s.Parameters(ctx)
	if err != nil {
		return nil, err
	}
	if s.repo == nil {
		return &CalculateResponse{Parameters: params}, nil
	}
	var inputs []domain.ProductInput
	if query.CustomerID > 0 {
		if scoped, ok := s.repo.(customerScopedProductInputRepository); ok {
			inputs, err = scoped.LoadProductInputsForCustomer(ctx, params, query.CustomerID)
		} else {
			inputs, err = s.repo.LoadProductInputs(ctx, params)
		}
	} else {
		inputs, err = s.repo.LoadProductInputs(ctx, params)
	}
	if err != nil {
		return nil, err
	}
	if len(inputs) == 0 {
		return &CalculateResponse{Parameters: params, Items: []domain.ProductResult{}}, nil
	}
	items, err := calculate(CalculateRequest{Products: inputs}, params)
	if err != nil {
		return nil, err
	}
	sortBeanListResults(items)
	return &CalculateResponse{Parameters: params, Items: items}, nil
}

func normalizeDripPriceTemplateCommand(cmd SaveDripPriceTemplateCommand) (SaveDripPriceTemplateCommand, error) {
	cmd.Name = strings.TrimSpace(cmd.Name)
	if cmd.Name == "" {
		return SaveDripPriceTemplateCommand{}, fmt.Errorf("name required")
	}
	if math.IsNaN(cmd.BagGrams) || math.IsInf(cmd.BagGrams, 0) || cmd.BagGrams <= 0 {
		return SaveDripPriceTemplateCommand{}, fmt.Errorf("bag_grams must be > 0")
	}
	if cmd.BoxBagCount <= 0 {
		return SaveDripPriceTemplateCommand{}, fmt.Errorf("box_bag_count must be > 0")
	}
	if len(cmd.Tiers) == 0 {
		return SaveDripPriceTemplateCommand{}, fmt.Errorf("tiers required")
	}
	for i := range cmd.Tiers {
		cmd.Tiers[i].Label = strings.TrimSpace(cmd.Tiers[i].Label)
		if cmd.Tiers[i].Label == "" {
			return SaveDripPriceTemplateCommand{}, fmt.Errorf("tier label required")
		}
		if cmd.Tiers[i].MinBags <= 0 {
			return SaveDripPriceTemplateCommand{}, fmt.Errorf("tier min_bags must be > 0")
		}
		if cmd.Tiers[i].MaxBags != nil && *cmd.Tiers[i].MaxBags <= cmd.Tiers[i].MinBags {
			return SaveDripPriceTemplateCommand{}, fmt.Errorf("tier max_bags must be greater than min_bags")
		}
		if cmd.Tiers[i].Multiplier <= 0 || math.IsNaN(cmd.Tiers[i].Multiplier) || math.IsInf(cmd.Tiers[i].Multiplier, 0) {
			return SaveDripPriceTemplateCommand{}, fmt.Errorf("tier multiplier must be > 0")
		}
		if cmd.Tiers[i].Position <= 0 {
			cmd.Tiers[i].Position = i + 1
		}
		cmd.Tiers[i].Active = true
	}
	return cmd, nil
}

func (s *Service) CreateRun(ctx context.Context, actor string) (*Run, error) {
	resp, err := s.BeanList(ctx, BeanListQuery{})
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

func (s *Service) ListBeanListPublications(ctx context.Context, query BeanListPublicationQuery) ([]BeanListPublication, error) {
	normalized, err := normalizeBeanListPublicationQuery(query)
	if err != nil {
		return nil, err
	}
	if s.repo == nil {
		return []BeanListPublication{}, nil
	}
	return s.repo.ListBeanListPublications(ctx, normalized)
}

func (s *Service) PublishedBeanList(ctx context.Context, query BeanListPublicationQuery) (*BeanListPublication, error) {
	normalized, err := normalizeBeanListPublicationQuery(query)
	if err != nil {
		return nil, err
	}
	if s.repo == nil {
		return nil, nil
	}
	return s.repo.PublishedBeanList(ctx, normalized)
}

func (s *Service) GenerateBeanListPublicationPDF(ctx context.Context, cmd BeanListPublicationPDFCommand, render func(BeanListPublication) ([]byte, error)) (BeanListPublicationPDFFile, error) {
	if render == nil {
		return BeanListPublicationPDFFile{}, fmt.Errorf("bean list renderer required")
	}
	normalized, err := normalizeBeanListPublicationPDFCommand(cmd)
	if err != nil {
		return BeanListPublicationPDFFile{}, err
	}
	if s.repo == nil {
		return BeanListPublicationPDFFile{}, fmt.Errorf("repository required")
	}
	row, err := s.repo.LoadBeanListPublication(ctx, normalized.Query, normalized.PublicationID)
	if err != nil {
		return BeanListPublicationPDFFile{}, err
	}
	if row == nil {
		return BeanListPublicationPDFFile{}, ErrBeanListPublicationNotFound
	}
	cacheKey := beanListPublicationPDFCacheKey(*row)
	if asset, err := s.repo.LoadBeanListPublicationAsset(ctx, row.ID, "pdf"); err == nil && len(asset.Payload) > 0 && strings.TrimSpace(asset.CacheKey) == cacheKey {
		return beanListPublicationPDFFile(*row, asset), nil
	} else if err != nil && !errors.Is(err, ErrBeanListPublicationNotFound) {
		return BeanListPublicationPDFFile{}, err
	}
	body, err := render(*row)
	if err != nil {
		return BeanListPublicationPDFFile{}, err
	}
	if len(body) == 0 {
		return BeanListPublicationPDFFile{}, fmt.Errorf("bean list PDF is empty")
	}
	asset, err := s.repo.SaveBeanListPublicationAsset(ctx, BeanListPublicationAsset{
		PublicationID: row.ID,
		AssetType:     "pdf",
		ContentType:   "application/pdf",
		CacheKey:      cacheKey,
		Payload:       body,
	}, normalized.Actor)
	if err != nil {
		return BeanListPublicationPDFFile{}, err
	}
	return beanListPublicationPDFFile(*row, asset), nil
}

func (s *Service) LoadBeanListPublicationPDF(ctx context.Context, cmd BeanListPublicationPDFCommand) (BeanListPublicationPDFFile, error) {
	normalized, err := normalizeBeanListPublicationPDFCommand(cmd)
	if err != nil {
		return BeanListPublicationPDFFile{}, err
	}
	if s.repo == nil {
		return BeanListPublicationPDFFile{}, fmt.Errorf("repository required")
	}
	row, err := s.repo.LoadBeanListPublication(ctx, normalized.Query, normalized.PublicationID)
	if err != nil {
		return BeanListPublicationPDFFile{}, err
	}
	if row == nil {
		return BeanListPublicationPDFFile{}, ErrBeanListPublicationNotFound
	}
	asset, err := s.repo.LoadBeanListPublicationAsset(ctx, row.ID, "pdf")
	if err != nil {
		return BeanListPublicationPDFFile{}, err
	}
	if len(asset.Payload) == 0 || strings.TrimSpace(asset.CacheKey) != beanListPublicationPDFCacheKey(*row) {
		return BeanListPublicationPDFFile{}, ErrBeanListPublicationNotFound
	}
	return beanListPublicationPDFFile(*row, asset), nil
}

func (s *Service) PublishBeanList(ctx context.Context, cmd PublishBeanListCommand) (*BeanListPublication, error) {
	normalized, err := normalizeBeanListCommand(cmd)
	if err != nil {
		return nil, err
	}
	if err := s.validateProductSpecSelections(ctx, &normalized); err != nil {
		return nil, err
	}
	if !beanListUsesConcreteProductSpecSelections(&normalized) {
		if err := s.validatePriceTierTemplateUnitCompatibility(ctx, &normalized); err != nil {
			return nil, err
		}
	}
	if s.repo != nil {
		if err := s.applyNextBeanListPublicationVersion(ctx, &normalized); err != nil {
			return nil, err
		}
		if err := s.applyProductSalesUnitSnapshots(ctx, &normalized); err != nil {
			return nil, err
		}
	}
	if err := validateBeanListFinalPriceSnapshots(normalized); err != nil {
		return nil, err
	}
	if s.repo == nil {
		return nil, fmt.Errorf("repository required")
	}
	return s.repo.PublishBeanList(ctx, normalized)
}

func (s *Service) applyNextBeanListPublicationVersion(ctx context.Context, cmd *PublishBeanListCommand) error {
	if s.repo == nil || cmd == nil {
		return nil
	}
	rows, err := s.repo.ListBeanListPublications(ctx, BeanListPublicationQuery{
		ListType:                 cmd.ListType,
		PublicationPurpose:       cmd.PublicationPurpose,
		ProductTypeCategoryID:    cmd.ProductTypeCategoryID,
		ClassificationTemplateID: cmd.ClassificationTemplateID,
		OwnerType:                cmd.OwnerType,
		OwnerKey:                 cmd.OwnerKey,
	})
	if err != nil {
		return err
	}
	cmd.Version = nextBeanListPublicationVersion(cmd.Version, rows)
	return nil
}

func nextBeanListPublicationVersion(requested string, rows []BeanListPublication) string {
	requested = strings.TrimSpace(requested)
	maxExisting := ""
	duplicate := false
	for _, row := range rows {
		version := strings.TrimSpace(row.Version)
		if version == "" {
			continue
		}
		if strings.EqualFold(version, requested) {
			duplicate = true
		}
		if maxExisting == "" || compareBeanListPublicationVersion(version, maxExisting) > 0 {
			maxExisting = version
		}
	}
	if maxExisting == "" {
		return requested
	}
	if requested == "" || duplicate || compareBeanListPublicationVersion(requested, maxExisting) <= 0 {
		return incrementBeanListPublicationVersion(maxExisting)
	}
	return requested
}

func incrementBeanListPublicationVersion(version string) string {
	source := strings.TrimSpace(version)
	if source == "" {
		return "V3.0.5"
	}
	if idx := strings.LastIndex(source, "."); idx >= 0 && idx < len(source)-1 {
		segment := source[idx+1:]
		if beanListPublicationVersionDigitsOnly(segment) {
			next, err := strconv.Atoi(segment)
			if err == nil {
				return source[:idx+1] + fmt.Sprintf("%0*d", len(segment), next+1)
			}
		}
	}
	if beanListPublicationVersionNumberPattern.MatchString(source) {
		return source + ".01"
	}
	return source + ".01"
}

func compareBeanListPublicationVersion(left, right string) int {
	leftStandard := beanListPublicationStandardVersionPattern.MatchString(strings.TrimSpace(left))
	rightStandard := beanListPublicationStandardVersionPattern.MatchString(strings.TrimSpace(right))
	if leftStandard && !rightStandard {
		return 1
	}
	if !leftStandard && rightStandard {
		return -1
	}
	leftNumbers := beanListPublicationVersionNumbers(left)
	rightNumbers := beanListPublicationVersionNumbers(right)
	if len(leftNumbers) > 0 && len(rightNumbers) > 0 {
		length := len(leftNumbers)
		if len(rightNumbers) > length {
			length = len(rightNumbers)
		}
		for i := 0; i < length; i++ {
			leftValue, rightValue := 0, 0
			if i < len(leftNumbers) {
				leftValue = leftNumbers[i]
			}
			if i < len(rightNumbers) {
				rightValue = rightNumbers[i]
			}
			if leftValue > rightValue {
				return 1
			}
			if leftValue < rightValue {
				return -1
			}
		}
	}
	return strings.Compare(strings.TrimSpace(left), strings.TrimSpace(right))
}

func beanListPublicationVersionNumbers(version string) []int {
	matches := beanListPublicationVersionNumberPattern.FindAllString(strings.TrimSpace(version), -1)
	out := make([]int, 0, len(matches))
	for _, match := range matches {
		value, err := strconv.Atoi(match)
		if err != nil {
			continue
		}
		out = append(out, value)
	}
	return out
}

func beanListPublicationVersionDigitsOnly(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func (s *Service) validatePriceTierTemplateUnitCompatibility(ctx context.Context, cmd *PublishBeanListCommand) error {
	templateResolver, hasTemplateResolver := s.repo.(priceTierTemplateUnitRuleRepository)
	productResolver, hasProductResolver := s.repo.(productDefaultSalesUnitRepository)
	if !hasTemplateResolver || cmd == nil || cmd.Content == nil {
		return nil
	}
	rows := priceTierTemplateCompatibilityRows(cmd.Content["price_rows"])
	if len(rows) == 0 {
		return nil
	}
	if !hasProductResolver {
		return fmt.Errorf("阶梯模板不可用：服务缺少当前商品销售规格校验能力")
	}

	templateCache := map[int64]PriceTierTemplateUnitRule{}
	productCache := map[int64]string{}
	for idx, rawRow := range rows {
		row, ok := rawRow.(map[string]any)
		if !ok || normalizePriceRowPricingMode(row) != "tier_template" {
			continue
		}
		templateID := int64(numberValue(row["tier_template_id"]))
		templateTierID := int64(numberValue(row["template_tier_id"]))
		productID := int64(numberValue(row["product_id"]))
		skuID := int64(numberValue(row["sku_id"]))
		if templateID <= 0 || templateTierID <= 0 {
			return fmt.Errorf("阶梯模板不可用：第%d行缺少有效模板或档位", idx+1)
		}
		unitProductID := skuID
		if unitProductID <= 0 {
			unitProductID = productID
		}
		if unitProductID <= 0 {
			return fmt.Errorf("阶梯模板不可用：第%d行缺少有效商品或子 SKU", idx+1)
		}

		templateRule, cached := templateCache[templateID]
		if !cached {
			resolved, err := templateResolver.ResolvePriceTierTemplateUnitRule(ctx, templateID)
			if err != nil {
				if errors.Is(err, ErrPriceTierTemplateUnitRuleNotFound) {
					return fmt.Errorf("阶梯模板不可用：第%d行引用的阶梯模板不存在、已停用或没有有效档位", idx+1)
				}
				return err
			}
			templateRule = resolved
			templateCache[templateID] = resolved
		}
		tierUnit := strings.TrimSpace(templateRule.TierUnits[templateTierID])
		if tierUnit == "" {
			return fmt.Errorf("阶梯模板不可用：第%d行引用的阶梯模板“%s”不包含当前有效档位", idx+1, priceTierTemplateUnitRuleName(templateRule, templateID))
		}

		productUnit, cached := productCache[unitProductID]
		if !cached {
			resolved, err := productResolver.ResolveProductDefaultSalesUnit(ctx, unitProductID)
			if err != nil {
				if errors.Is(err, ErrProductSalesUnitRuleNotFound) {
					return fmt.Errorf("阶梯模板不可用：商品“%s”缺少有效默认销售规格，无法校验阶梯模板“%s”", beanListFlatRowProductName(row, idx+1), priceTierTemplateUnitRuleName(templateRule, templateID))
				}
				return err
			}
			productUnit = strings.TrimSpace(resolved)
			productCache[unitProductID] = productUnit
		}

		if productUnit == "" {
			return fmt.Errorf("阶梯模板不可用：商品“%s”缺少有效默认销售规格，无法校验阶梯模板“%s”", beanListFlatRowProductName(row, idx+1), priceTierTemplateUnitRuleName(templateRule, templateID))
		}
		productUnitIdentity := normalizePriceTierCompatibilityUnit(productUnit)
		tierIDs := make([]int64, 0, len(templateRule.TierUnits))
		for tierID := range templateRule.TierUnits {
			tierIDs = append(tierIDs, tierID)
		}
		sort.Slice(tierIDs, func(i, j int) bool { return tierIDs[i] < tierIDs[j] })
		if len(tierIDs) == 0 {
			return fmt.Errorf("阶梯模板不可用：阶梯模板“%s”没有有效档位", priceTierTemplateUnitRuleName(templateRule, templateID))
		}
		for _, tierID := range tierIDs {
			activeTierUnit := strings.TrimSpace(templateRule.TierUnits[tierID])
			if activeTierUnit == "" {
				return fmt.Errorf("阶梯模板不可用：阶梯模板“%s”存在缺少数量单位的有效档位", priceTierTemplateUnitRuleName(templateRule, templateID))
			}
			if productUnitIdentity != normalizePriceTierCompatibilityUnit(activeTierUnit) {
				return fmt.Errorf("阶梯模板不可用：商品“%s”的默认销售规格“%s”与阶梯模板“%s”的档位数量单位“%s”不匹配", beanListFlatRowProductName(row, idx+1), productUnit, priceTierTemplateUnitRuleName(templateRule, templateID), activeTierUnit)
			}
		}
		row["tier_template_name"] = priceTierTemplateUnitRuleName(templateRule, templateID)
		row["tier_quantity_unit"] = tierUnit
		row["product_sales_unit"] = productUnit
	}
	return nil
}

func priceTierTemplateCompatibilityRows(value any) []any {
	switch rows := value.(type) {
	case []any:
		return rows
	case []map[string]any:
		out := make([]any, 0, len(rows))
		for _, row := range rows {
			out = append(out, row)
		}
		return out
	default:
		return nil
	}
}

func normalizePriceTierCompatibilityUnit(unit string) string {
	compact := strings.ToLower(strings.Join(strings.Fields(unit), ""))
	if strings.HasPrefix(compact, "盒(") || strings.HasPrefix(compact, "盒（") {
		return "盒"
	}
	if strings.HasPrefix(compact, "袋(") || strings.HasPrefix(compact, "袋（") {
		return "袋"
	}
	if strings.HasPrefix(compact, "条(") || strings.HasPrefix(compact, "条（") {
		return "条"
	}
	if matches := priceTierMassUnitPattern.FindStringSubmatch(compact); len(matches) == 2 {
		compact = matches[1]
	}
	switch compact {
	case "kg", "公斤", "千克":
		return "kg"
	case "kgs":
		return "kg"
	case "lb", "lbs", "磅":
		return "lb"
	case "g", "克":
		return "g"
	default:
		return compact
	}
}

func priceTierTemplateUnitRuleName(rule PriceTierTemplateUnitRule, templateID int64) string {
	if name := strings.TrimSpace(rule.TemplateName); name != "" {
		return name
	}
	return fmt.Sprintf("#%d", templateID)
}

func beanListFlatRowProductName(row map[string]any, position int) string {
	if name := beanListFlatRowString(row, "product_name", "display_name_snapshot", "product_name_snapshot", "name"); name != "" {
		return name
	}
	return fmt.Sprintf("第%d行商品", position)
}

func (s *Service) applyProductSalesUnitSnapshots(ctx context.Context, cmd *PublishBeanListCommand) error {
	resolver, ok := s.repo.(productSalesUnitRuleRepository)
	if !ok || cmd == nil || cmd.Content == nil {
		return nil
	}
	rows := priceTierTemplateCompatibilityRows(cmd.Content["price_rows"])
	if len(rows) == 0 {
		return nil
	}
	strictSnapshots := beanListUsesConcreteProductSpecSelections(cmd)
	for idx, rawRow := range rows {
		row, ok := rawRow.(map[string]any)
		if !ok || !beanListFlatPriceRowHasPrice(row) {
			continue
		}
		productID := int64(numberValue(row["product_id"]))
		skuID := int64(numberValue(row["sku_id"]))
		concreteProductID := skuID
		if concreteProductID <= 0 {
			concreteProductID = productID
		}
		priceUnit := strings.TrimSpace(stringValue(row["price_unit"]))
		if concreteProductID <= 0 || priceUnit == "" {
			continue
		}
		customerAliasID := beanListFlatPriceRowCustomerAliasID(row)
		rule, err := ProductSalesUnitRule{}, error(nil)
		specRule, specErr := resolver.ResolveProductSalesUnitRule(ctx, concreteProductID, "")
		if specErr != nil || specRule.ProductID <= 0 {
			if strictSnapshots {
				if specErr != nil && !errors.Is(specErr, ErrProductSalesUnitRuleNotFound) {
					return specErr
				}
				return beanListFlatRowUnitConversionError("商品档案缺少权威销售规格", idx+1, row, priceUnit)
			}
			specRule = ProductSalesUnitRule{}
		}
		if customerAliasID > 0 {
			if customerResolver, ok := s.repo.(customerProductSalesUnitRuleRepository); ok {
				aliasProductID := productID
				if aliasProductID <= 0 {
					aliasProductID = concreteProductID
				}
				rule, err = customerResolver.ResolveCustomerProductSalesUnitRule(ctx, aliasProductID, customerAliasID, priceUnit)
			} else {
				rule, err = resolver.ResolveProductSalesUnitRule(ctx, concreteProductID, priceUnit)
			}
		} else {
			rule, err = resolver.ResolveProductSalesUnitRule(ctx, concreteProductID, priceUnit)
		}
		if errors.Is(err, ErrProductSalesUnitRuleNotFound) && productID > 0 && productID != concreteProductID {
			// Historical flat rows could carry a display-only sku_id without a
			// parent_product_id. Keep their conversion fallback, while the new
			// sales-spec snapshot remains anchored to the concrete SKU identity.
			rule, err = resolver.ResolveProductSalesUnitRule(ctx, productID, priceUnit)
		}
		if err != nil {
			if errors.Is(err, ErrProductSalesUnitRuleNotFound) {
				return beanListFlatRowUnitConversionError("商品档案缺少价格单位到库存单位换算", idx+1, row, priceUnit)
			}
			return err
		}
		if strings.TrimSpace(rule.InventoryUnit) == "" || len(rule.Conversion) == 0 {
			continue
		}
		targets, ok := rule.Conversion[priceUnit]
		if !ok || len(targets) == 0 {
			return beanListFlatRowUnitConversionError("商品档案缺少价格单位到库存单位换算", idx+1, row, priceUnit)
		}
		row["inventory_unit"] = strings.TrimSpace(rule.InventoryUnit)
		row["inventory_conversion_json"] = productSalesUnitConversionSnapshot(priceUnit, targets)
		if specRule.ProductID <= 0 {
			specRule = rule
		}
		spec, err := applyFlatRowEffectiveSalesSpec(row, concreteProductID, priceUnit, specRule, strictSnapshots)
		if err != nil {
			return beanListFlatRowUnitConversionError(err.Error(), idx+1, row, priceUnit)
		}
		applyFlatRowSKUSnapshot(row, concreteProductID, specRule, spec, strictSnapshots)
	}
	return nil
}

func applyFlatRowSKUSnapshot(row map[string]any, productID int64, rule ProductSalesUnitRule, spec domain.EffectiveSalesSpec, strict bool) {
	if row == nil {
		return
	}
	skuID := int64(numberValue(row["sku_id"]))
	if skuID <= 0 {
		skuID = productID
	}
	if skuID > 0 {
		row["sku_id"] = float64(skuID)
	}
	if parentID := int64(numberValue(row["parent_product_id"])); parentID > 0 {
		row["parent_product_id"] = float64(parentID)
	}
	snapshot := map[string]any{}
	if !strict {
		if existing, ok := row["sku_snapshot"].(map[string]any); ok {
			for key, value := range existing {
				snapshot[key] = value
			}
		}
	}
	if skuID > 0 {
		snapshot["sku_id"] = float64(skuID)
	}
	stringFields := map[string]string{
		"sku_name":         strings.TrimSpace(rule.SKUName),
		"sku_code":         strings.TrimSpace(rule.SKUCode),
		"barcode":          strings.TrimSpace(rule.Barcode),
		"spec_key":         strings.TrimSpace(spec.SpecKey),
		"spec_label":       strings.TrimSpace(spec.SpecLabel),
		"net_content_unit": strings.TrimSpace(spec.NetContentUnit),
		"sales_unit":       strings.TrimSpace(spec.SalesUnit),
	}
	if stringFields["sku_name"] == "" {
		if !strict {
			stringFields["sku_name"] = firstNonEmptyBeanListString(beanListFlatRowString(row, "sku_name"), beanListFlatRowString(snapshot, "sku_name"))
		}
		if stringFields["sku_name"] == "" {
			stringFields["sku_name"] = strings.TrimSpace(spec.SpecName)
		}
	}
	for field, value := range stringFields {
		if !strict && value == "" {
			value = strings.TrimSpace(stringValue(row[field]))
			if value == "" {
				value = strings.TrimSpace(stringValue(snapshot[field]))
			}
		}
		if value == "" {
			if strict {
				delete(row, field)
			}
			continue
		}
		row[field] = value
		snapshot[field] = value
	}
	if spec.NetContentQty > 0 {
		row["net_content_qty"] = spec.NetContentQty
		snapshot["net_content_qty"] = spec.NetContentQty
	} else {
		delete(row, "net_content_qty")
	}
	row["sku_snapshot"] = snapshot
}

func applyFlatRowEffectiveSalesSpec(row map[string]any, skuID int64, priceUnit string, rule ProductSalesUnitRule, strict bool) (domain.EffectiveSalesSpec, error) {
	if row == nil || skuID <= 0 {
		return domain.EffectiveSalesSpec{}, fmt.Errorf("商品档案缺少权威销售规格")
	}
	spec := domain.EffectiveSalesSpec{}
	if rule.EffectiveSalesSpec != nil {
		spec = *rule.EffectiveSalesSpec
	}
	if !strict {
		existing, _ := objectSnapshotMap(row["effective_sales_spec"])
		if spec.SpecKey == "" {
			spec.SpecKey = beanListFlatRowString(existing, "spec_key")
		}
		if spec.SpecName == "" {
			spec.SpecName = beanListFlatRowString(existing, "spec_name")
		}
		if spec.SpecLabel == "" {
			spec.SpecLabel = firstNonEmptyBeanListString(beanListFlatRowString(row, "spec_label"), beanListFlatRowString(existing, "spec_label"))
		}
		if spec.SalesUnit == "" {
			spec.SalesUnit = firstNonEmptyBeanListString(beanListFlatRowString(existing, "sales_unit"), beanListFlatRowString(row, "sales_unit"))
		}
		if spec.NetContentQty <= 0 {
			spec.NetContentQty = numberValue(row["net_content_qty"])
			if spec.NetContentQty <= 0 {
				spec.NetContentQty = numberValue(existing["net_content_qty"])
			}
		}
		if spec.NetContentUnit == "" {
			spec.NetContentUnit = firstNonEmptyBeanListString(beanListFlatRowString(row, "net_content_unit"), beanListFlatRowString(existing, "net_content_unit"))
		}
	}
	if spec.SKUID > 0 && spec.SKUID != skuID {
		return domain.EffectiveSalesSpec{}, fmt.Errorf("商品档案销售规格身份与价格行 SKU 不一致")
	}
	spec.SKUID = skuID
	spec.SpecKey = strings.TrimSpace(spec.SpecKey)
	spec.SpecName = strings.TrimSpace(spec.SpecName)
	spec.SpecLabel = strings.TrimSpace(spec.SpecLabel)
	spec.SalesUnit = strings.TrimSpace(spec.SalesUnit)
	spec.NetContentUnit = strings.TrimSpace(spec.NetContentUnit)
	if spec.SpecName == "" {
		spec.SpecName = strings.TrimSpace(rule.DefaultSalesUnit)
	}
	if !strict && spec.SpecName == "" {
		spec.SpecName = firstNonEmptyBeanListString(spec.SpecLabel, beanListFlatRowString(row, "sku_name"), strings.TrimSpace(priceUnit))
	}
	if spec.SpecLabel == "" {
		spec.SpecLabel = spec.SpecName
	}
	if spec.SalesUnit == "" {
		spec.SalesUnit = strings.TrimSpace(rule.DefaultSalesUnit)
	}
	if spec.SalesUnit == "" {
		spec.SalesUnit = spec.SpecName
	}
	if spec.SpecName == "" {
		spec.SpecName = spec.SalesUnit
	}
	if spec.SpecName == "" || spec.SalesUnit == "" {
		return domain.EffectiveSalesSpec{}, fmt.Errorf("商品档案缺少权威销售规格名称或销售单位")
	}
	if strings.TrimSpace(rule.InventoryUnit) != "" {
		spec.InventoryUnit = strings.TrimSpace(rule.InventoryUnit)
	}
	if len(rule.Conversion) > 0 {
		spec.InventoryConversionJSON = rule.Conversion
	}
	payload, _ := json.Marshal(spec)
	snapshot := map[string]any{}
	_ = json.Unmarshal(payload, &snapshot)
	row["quantity_basis"] = "sales_spec_count"
	row["effective_sales_spec"] = snapshot
	row["tier_quantity_unit"] = spec.SpecName
	return spec, nil
}

func firstNonEmptyBeanListString(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func beanListFlatPriceRowCustomerAliasID(row map[string]any) int64 {
	if row == nil {
		return 0
	}
	if id := int64(numberValue(row["customer_product_alias_id"])); id > 0 {
		return id
	}
	snapshot, _ := row["customer_reference_snapshot"].(map[string]any)
	if id := int64(numberValue(snapshot["customer_product_alias_id"])); id > 0 {
		return id
	}
	if id := int64(numberValue(snapshot["customerProductAliasID"])); id > 0 {
		return id
	}
	return 0
}

func productSalesUnitConversionSnapshot(priceUnit string, targets map[string]float64) map[string]any {
	outTargets := map[string]any{}
	for unit, factor := range targets {
		if strings.TrimSpace(unit) == "" || factor <= 0 {
			continue
		}
		outTargets[strings.TrimSpace(unit)] = factor
	}
	if len(outTargets) == 0 {
		return map[string]any{}
	}
	return map[string]any{strings.TrimSpace(priceUnit): outTargets}
}

func (s *Service) SaveBeanListDraft(ctx context.Context, cmd PublishBeanListCommand) (*BeanListPublication, error) {
	normalized, err := normalizeBeanListCommand(cmd)
	if err != nil {
		return nil, err
	}
	if s.repo == nil {
		return nil, fmt.Errorf("repository required")
	}
	if err := s.validateProductSpecSelections(ctx, &normalized); err != nil {
		return nil, err
	}
	if !beanListUsesConcreteProductSpecSelections(&normalized) {
		if err := s.validatePriceTierTemplateUnitCompatibility(ctx, &normalized); err != nil {
			return nil, err
		}
	}
	if err := s.applyProductSalesUnitSnapshots(ctx, &normalized); err != nil {
		return nil, err
	}
	return s.repo.SaveBeanListDraft(ctx, normalized)
}

type beanListProductSpecSelection struct {
	ParentProductID         int64
	SKUID                   int64
	SelectionSource         string
	DefaultSKUIDAtSelection int64
}

func beanListUsesConcreteProductSpecSelections(cmd *PublishBeanListCommand) bool {
	if cmd == nil || cmd.Config == nil {
		return false
	}
	_, present := cmd.Config["product_spec_selections"]
	return present
}

func beanListUsesSharedParentProductPricing(cmd *PublishBeanListCommand) bool {
	if cmd == nil || cmd.Config == nil {
		return false
	}
	selection, ok := objectSnapshotMap(cmd.Config["price_list_template_selection"])
	if !ok {
		return false
	}
	return strings.TrimSpace(stringValue(selection["product_pricing_scope"])) == "parent_product_shared"
}

type beanListParentPricingSelection struct {
	pricingMode    string
	tierTemplateID int64
	pricingRuleID  int64
}

func validateSharedParentProductPricing(cmd *PublishBeanListCommand) error {
	if !beanListUsesSharedParentProductPricing(cmd) {
		// Drafts and publications created before the shared-parent pricing
		// contract retain their historical per-SKU interpretation.
		return nil
	}

	selection, _ := objectSnapshotMap(cmd.Config["price_list_template_selection"])
	for idx, rawOverride := range beanListAnySlice(selection["product_overrides"]) {
		override, ok := objectSnapshotMap(rawOverride)
		if !ok {
			continue
		}
		if strings.TrimSpace(stringValue(override["scope"])) != "sku" {
			continue
		}
		if strings.TrimSpace(stringValue(override["pricing_mode"])) != "" ||
			numberValue(override["tier_template_id"]) > 0 ||
			numberValue(override["pricing_rule_id"]) > 0 {
			return fmt.Errorf("商品计价设置无效：第%d个规格不能单独设置计价类型或模板", idx+1)
		}
	}

	byParent := map[int64]beanListParentPricingSelection{}
	for idx, rawRow := range beanListAnySlice(cmd.Content["price_rows"]) {
		row, ok := objectSnapshotMap(rawRow)
		if !ok {
			continue
		}
		parentProductID := int64(numberValue(row["parent_product_id"]))
		if parentProductID <= 0 {
			continue
		}
		current := beanListParentPricingSelection{
			pricingMode:    normalizePriceRowPricingMode(row),
			tierTemplateID: int64(numberValue(row["tier_template_id"])),
			pricingRuleID:  int64(numberValue(row["pricing_rule_id"])),
		}
		previous, exists := byParent[parentProductID]
		if !exists {
			byParent[parentProductID] = current
			continue
		}
		if previous.pricingMode != current.pricingMode {
			return fmt.Errorf("商品规格价格行无效：第%d行同一父商品只能选择一种计价类型", idx+1)
		}
		switch current.pricingMode {
		case "tier_template":
			if previous.tierTemplateID != current.tierTemplateID {
				return fmt.Errorf("商品规格价格行无效：第%d行同一父商品只能选择一个阶梯模板", idx+1)
			}
		case "pricing_rule":
			if previous.pricingRuleID != current.pricingRuleID {
				return fmt.Errorf("商品规格价格行无效：第%d行同一父商品只能选择一个价格计算模板", idx+1)
			}
		}
	}
	return nil
}

type beanListConcreteSpecSnapshot struct {
	productName string
	aliasID     int64
	tierLabels  []beanListTierLabelSnapshot
}

type beanListTierLabelSnapshot struct {
	oldLabel       string
	newLabel       string
	templateTierID int64
	minQty         float64
	maxQty         *float64
}

func normalizeConcreteProductSpecPublicationSnapshots(cmd *PublishBeanListCommand, parentProductNamesBySKU map[int64]string) {
	if !beanListUsesConcreteProductSpecSelections(cmd) || cmd.Content == nil {
		return
	}

	groups := beanListAnySlice(cmd.Content["groups"])
	bySKU := map[int64]beanListConcreteSpecSnapshot{}
	for groupIdx, rawGroup := range groups {
		group, ok := objectSnapshotMap(rawGroup)
		if !ok {
			continue
		}
		groups[groupIdx] = group
		items := beanListAnySlice(group["items"])
		for itemIdx, rawItem := range items {
			item, ok := objectSnapshotMap(rawItem)
			if !ok {
				continue
			}
			items[itemIdx] = item
			skuID := beanListConcreteSpecSKUID(item)
			if skuID <= 0 {
				continue
			}
			snapshot := bySKU[skuID]
			snapshot.productName = firstNonEmptyBeanListString(
				snapshot.productName,
				beanListFlatRowString(item, "product_name_snapshot", "product_name"),
			)
			if snapshot.aliasID <= 0 {
				snapshot.aliasID = int64(numberValue(item["customer_product_alias_id"]))
			}
			bySKU[skuID] = snapshot
		}
		group["items"] = items
	}
	cmd.Content["groups"] = groups

	rows := beanListAnySlice(cmd.Content["price_rows"])
	for rowIdx, rawRow := range rows {
		row, ok := objectSnapshotMap(rawRow)
		if !ok {
			continue
		}
		rows[rowIdx] = row
		skuID := beanListConcreteSpecSKUID(row)
		if skuID <= 0 {
			continue
		}
		snapshot := bySKU[skuID]
		productName := firstNonEmptyBeanListString(
			parentProductNamesBySKU[skuID],
			snapshot.productName,
			beanListFlatRowString(row, "product_name_snapshot", "product_name"),
		)
		if productName != "" {
			row["product_name"] = productName
			row["product_name_snapshot"] = productName
			snapshot.productName = productName
		}
		if snapshot.aliasID <= 0 {
			snapshot.aliasID = beanListFlatPriceRowCustomerAliasID(row)
		}
		if tierLabel, ok := normalizeBeanListSalesSpecCountTierLabel(row); ok {
			oldLabel := beanListFlatRowString(row, "tier_label", "label")
			row["tier_label"] = tierLabel
			if costSnapshot, exists := objectSnapshotMap(row["cost_source_snapshot"]); exists {
				costSnapshot["tier_label"] = tierLabel
				row["cost_source_snapshot"] = costSnapshot
			}
			snapshot.tierLabels = append(snapshot.tierLabels, beanListTierLabelSnapshot{
				oldLabel:       oldLabel,
				newLabel:       tierLabel,
				templateTierID: int64(numberValue(row["template_tier_id"])),
				minQty:         numberValue(row["min_qty"]),
				maxQty:         beanListOptionalNumber(row["max_qty"]),
			})
		}
		bySKU[skuID] = snapshot
	}
	cmd.Content["price_rows"] = rows

	for _, rawGroup := range groups {
		group, ok := objectSnapshotMap(rawGroup)
		if !ok {
			continue
		}
		items := beanListAnySlice(group["items"])
		for itemIdx, rawItem := range items {
			item, ok := objectSnapshotMap(rawItem)
			if !ok {
				continue
			}
			items[itemIdx] = item
			skuID := beanListConcreteSpecSKUID(item)
			snapshot := bySKU[skuID]
			normalizeBeanListItemSalesSpecCountTierLabels(item, snapshot.tierLabels)
			productName := firstNonEmptyBeanListString(
				parentProductNamesBySKU[skuID],
				snapshot.productName,
				beanListFlatRowString(item, "product_name_snapshot", "product_name"),
			)
			if productName == "" {
				continue
			}
			item["product_name"] = productName
			item["product_name_snapshot"] = productName
			aliasID := int64(numberValue(item["customer_product_alias_id"]))
			if aliasID <= 0 {
				aliasID = snapshot.aliasID
			}
			if aliasID > 0 {
				displayName := beanListFlatRowString(item,
					"customer_product_display_name_snapshot",
					"customer_product_display_name",
					"display_name_snapshot",
					"name",
				)
				if displayName != "" {
					item["name"] = displayName
					item["display_name_snapshot"] = displayName
				}
				continue
			}
			item["name"] = productName
			item["display_name_snapshot"] = productName
		}
		group["items"] = items
	}
	cmd.Content["groups"] = groups
}

func beanListConcreteSpecSKUID(row map[string]any) int64 {
	skuID := int64(numberValue(row["sku_id"]))
	if skuID <= 0 {
		skuID = int64(numberValue(row["product_id"]))
	}
	return skuID
}

func normalizeBeanListSalesSpecCountTierLabel(row map[string]any) (string, bool) {
	if strings.TrimSpace(stringValue(row["quantity_basis"])) != "sales_spec_count" || normalizePriceRowPricingMode(row) != "tier_template" {
		return "", false
	}
	minQty := numberValue(row["min_qty"])
	maxQty := beanListOptionalNumber(row["max_qty"])
	if minQty < 0 {
		return "", false
	}
	formatQty := func(value float64) string {
		return strconv.FormatFloat(math.Round((value+1e-12)*10000)/10000, 'f', -1, 64)
	}
	if maxQty == nil {
		return formatQty(minQty) + "件+", true
	}
	if *maxQty > minQty {
		return formatQty(minQty) + "-" + formatQty(*maxQty) + "件", true
	}
	return formatQty(minQty) + "件", true
}

func beanListOptionalNumber(value any) *float64 {
	if value == nil || strings.TrimSpace(stringValue(value)) == "" {
		return nil
	}
	number := numberValue(value)
	return &number
}

func normalizeBeanListItemSalesSpecCountTierLabels(item map[string]any, labels []beanListTierLabelSnapshot) {
	if len(labels) == 0 {
		return
	}
	oldToNew := make(map[string]string, len(labels))
	for _, label := range labels {
		if label.oldLabel != "" && label.newLabel != "" {
			oldToNew[label.oldLabel] = label.newLabel
		}
	}
	tierKeys := []string{"commercial_wholesale_tiers", "green_bean_sale_tiers", "retail_bean_tiers", "drip_wholesale_tiers", "tiers_snapshot"}
	for _, key := range tierKeys {
		tiers := beanListAnySlice(item[key])
		if len(tiers) == 0 {
			continue
		}
		for idx, rawTier := range tiers {
			tier, ok := objectSnapshotMap(rawTier)
			if !ok {
				continue
			}
			tiers[idx] = tier
			if label, ok := normalizeBeanListSalesSpecCountTierLabel(tier); ok {
				tier["label"] = label
				tier["tier_label"] = label
				continue
			}
			if replacement := beanListTierLabelReplacement(tier, labels); replacement != "" {
				tier["label"] = replacement
				tier["tier_label"] = replacement
			}
		}
		item[key] = tiers
	}

	prices := beanListAnySlice(item["prices"])
	for idx, rawPrice := range prices {
		price, ok := objectSnapshotMap(rawPrice)
		if !ok {
			continue
		}
		prices[idx] = price
		oldLabel := beanListFlatRowString(price, "label", "tier_label")
		newLabel := oldToNew[oldLabel]
		if newLabel == "" && len(prices) == len(labels) {
			newLabel = labels[idx].newLabel
		}
		if newLabel != "" {
			price["label"] = newLabel
			if _, exists := price["tier_label"]; exists {
				price["tier_label"] = newLabel
			}
		}
	}
	if len(prices) > 0 {
		item["prices"] = prices
	}
}

func beanListTierLabelReplacement(tier map[string]any, labels []beanListTierLabelSnapshot) string {
	templateTierID := int64(numberValue(tier["template_tier_id"]))
	oldLabel := beanListFlatRowString(tier, "label", "tier_label")
	minQty := numberValue(tier["min_qty"])
	maxQty := beanListOptionalNumber(tier["max_qty"])
	for _, candidate := range labels {
		if templateTierID > 0 && candidate.templateTierID == templateTierID {
			return candidate.newLabel
		}
		if oldLabel != "" && candidate.oldLabel == oldLabel {
			return candidate.newLabel
		}
		if minQty == candidate.minQty && beanListOptionalNumbersEqual(maxQty, candidate.maxQty) {
			return candidate.newLabel
		}
	}
	return ""
}

func beanListOptionalNumbersEqual(left, right *float64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return math.Abs(*left-*right) < 1e-9
}

func (s *Service) validateProductSpecSelections(ctx context.Context, cmd *PublishBeanListCommand) error {
	if cmd == nil || cmd.Config == nil {
		return nil
	}
	rawSelections, present := cmd.Config["product_spec_selections"]
	if !present {
		if beanListUsesSharedParentProductPricing(cmd) {
			return fmt.Errorf("商品规格选择无效：父商品共享计价必须提供 product_spec_selections 数组")
		}
		// Historical drafts and publications predate SKU-bound selections and
		// must keep their original compatibility path.
		return nil
	}
	selectionRows := beanListAnySlice(rawSelections)
	if rawSelections == nil || selectionRows == nil {
		return fmt.Errorf("商品规格选择无效：product_spec_selections 必须是数组")
	}
	if err := validateSharedParentProductPricing(cmd); err != nil {
		return err
	}
	resolver, ok := s.repo.(productSpecIdentityRepository)
	if !ok {
		return fmt.Errorf("商品规格选择校验不可用：服务缺少当前商品规格身份校验能力")
	}

	identityCache := map[int64]ProductSpecIdentity{}
	resolveIdentity := func(productID int64) (ProductSpecIdentity, error) {
		if identity, exists := identityCache[productID]; exists {
			return identity, nil
		}
		identity, err := resolver.ResolveProductSpecIdentity(ctx, productID)
		if err != nil {
			if errors.Is(err, ErrProductSpecIdentityNotFound) {
				return ProductSpecIdentity{}, fmt.Errorf("商品规格不存在：%d", productID)
			}
			return ProductSpecIdentity{}, err
		}
		identityCache[productID] = identity
		return identity, nil
	}

	selections := make(map[string]beanListProductSpecSelection, len(selectionRows))
	selectionBySKU := make(map[int64]beanListProductSpecSelection, len(selectionRows))
	parentProductNamesBySKU := make(map[int64]string, len(selectionRows))
	for idx, rawSelection := range selectionRows {
		selectionMap, ok := objectSnapshotMap(rawSelection)
		if !ok {
			return fmt.Errorf("商品规格选择无效：第%d项必须是对象", idx+1)
		}
		selection := beanListProductSpecSelection{
			ParentProductID:         int64(numberValue(selectionMap["parent_product_id"])),
			SKUID:                   int64(numberValue(selectionMap["sku_id"])),
			SelectionSource:         strings.TrimSpace(stringValue(selectionMap["selection_source"])),
			DefaultSKUIDAtSelection: int64(numberValue(selectionMap["default_sku_id_at_selection"])),
		}
		if selection.ParentProductID <= 0 || selection.SKUID <= 0 || selection.DefaultSKUIDAtSelection <= 0 {
			return fmt.Errorf("商品规格选择无效：第%d项缺少父商品、具体 SKU 或选择时默认规格", idx+1)
		}
		if selection.SelectionSource != "product_default" && selection.SelectionSource != "explicit" {
			return fmt.Errorf("商品规格选择无效：第%d项选择来源必须是 product_default 或 explicit", idx+1)
		}
		parent, err := resolveIdentity(selection.ParentProductID)
		if err != nil {
			return fmt.Errorf("商品规格选择无效：第%d项父商品校验失败：%w", idx+1, err)
		}
		if parent.ProductID != selection.ParentProductID || parent.EffectiveParentProductID != selection.ParentProductID {
			return fmt.Errorf("商品规格选择无效：第%d项 parent_product_id 不是父商品", idx+1)
		}
		if !parent.Active {
			return fmt.Errorf("商品规格选择无效：第%d项父商品已停用", idx+1)
		}
		sku, err := resolveIdentity(selection.SKUID)
		if err != nil {
			return fmt.Errorf("商品规格选择无效：第%d项 SKU 校验失败：%w", idx+1, err)
		}
		if sku.EffectiveParentProductID != selection.ParentProductID {
			return fmt.Errorf("商品规格选择无效：第%d项 SKU %d 不属于父商品 %d", idx+1, selection.SKUID, selection.ParentProductID)
		}
		if !sku.Active {
			return fmt.Errorf("商品规格选择无效：第%d项 SKU %d 已停用", idx+1, selection.SKUID)
		}
		if !sku.SpecValid {
			return fmt.Errorf("商品规格选择无效：第%d项 SKU %d 规格已失效", idx+1, selection.SKUID)
		}
		defaultSKU, err := resolveIdentity(selection.DefaultSKUIDAtSelection)
		if err != nil {
			return fmt.Errorf("商品规格选择无效：第%d项选择时默认规格校验失败：%w", idx+1, err)
		}
		if defaultSKU.EffectiveParentProductID != selection.ParentProductID {
			return fmt.Errorf("商品规格选择无效：第%d项选择时默认规格不属于父商品 %d", idx+1, selection.ParentProductID)
		}
		key := beanListProductSpecSelectionKey(selection.ParentProductID, selection.SKUID)
		if _, duplicate := selections[key]; duplicate {
			return fmt.Errorf("商品规格选择无效：第%d项重复选择 SKU %d", idx+1, selection.SKUID)
		}
		selections[key] = selection
		selectionBySKU[selection.SKUID] = selection
		parentProductNamesBySKU[selection.SKUID] = strings.TrimSpace(parent.ParentProductName)
	}
	for groupIdx, rawGroup := range beanListAnySlice(cmd.Content["groups"]) {
		group, ok := objectSnapshotMap(rawGroup)
		if !ok {
			return fmt.Errorf("商品规格分组无效：第%d组必须是对象", groupIdx+1)
		}
		for itemIdx, rawItem := range beanListAnySlice(group["items"]) {
			item, ok := objectSnapshotMap(rawItem)
			if !ok {
				return fmt.Errorf("商品规格分组无效：第%d组第%d个商品项必须是对象", groupIdx+1, itemIdx+1)
			}
			skuID := int64(numberValue(item["sku_id"]))
			if skuID <= 0 {
				skuID = int64(numberValue(item["product_id"]))
			}
			selection, selected := selectionBySKU[skuID]
			if !selected {
				return fmt.Errorf("分组商品项无效：第%d组第%d项 SKU %d 未在规格选择中", groupIdx+1, itemIdx+1, skuID)
			}
			parentProductID := int64(numberValue(item["parent_product_id"]))
			if parentProductID <= 0 {
				parentProductID = int64(numberValue(item["effective_parent_product_id"]))
			}
			if parentProductID > 0 && parentProductID != selection.ParentProductID {
				return fmt.Errorf("分组商品项无效：第%d组第%d项父商品与规格选择不一致", groupIdx+1, itemIdx+1)
			}
		}
	}

	covered := make(map[string]bool, len(selections))
	priceRows := beanListAnySlice(cmd.Content["price_rows"])
	for idx, rawRow := range priceRows {
		row, ok := objectSnapshotMap(rawRow)
		if !ok {
			return fmt.Errorf("商品规格价格行无效：第%d行必须是对象", idx+1)
		}
		parentProductID := int64(numberValue(row["parent_product_id"]))
		skuID := int64(numberValue(row["sku_id"]))
		if parentProductID <= 0 || skuID <= 0 {
			return fmt.Errorf("商品规格价格行无效：第%d行缺少 parent_product_id 或 sku_id", idx+1)
		}
		selection, skuSelected := selectionBySKU[skuID]
		if skuSelected && selection.ParentProductID != parentProductID {
			return fmt.Errorf("商品规格价格行无效：第%d行父商品与规格选择不一致", idx+1)
		}
		key := beanListProductSpecSelectionKey(parentProductID, skuID)
		if _, selected := selections[key]; !selected {
			return fmt.Errorf("商品规格价格行无效：第%d行 SKU %d 未在规格选择中", idx+1, skuID)
		}
		if snapshot, exists := objectSnapshotMap(row["sku_snapshot"]); exists {
			if snapshotSKUID := int64(numberValue(snapshot["sku_id"])); snapshotSKUID > 0 && snapshotSKUID != skuID {
				return fmt.Errorf("商品规格价格行无效：第%d行 SKU 快照身份不一致", idx+1)
			}
		}
		if snapshot, exists := objectSnapshotMap(row["effective_sales_spec"]); exists {
			if snapshotSKUID := int64(numberValue(snapshot["sku_id"])); snapshotSKUID > 0 && snapshotSKUID != skuID {
				return fmt.Errorf("商品规格价格行无效：第%d行有效销售规格快照身份不一致", idx+1)
			}
		}
		if beanListFlatPriceRowHasPrice(row) {
			covered[key] = true
		}
	}
	for key, selection := range selections {
		if !covered[key] {
			return fmt.Errorf("商品规格选择无效：父商品 %d 的 SKU %d 缺少对应有效价格行", selection.ParentProductID, selection.SKUID)
		}
	}
	normalizeConcreteProductSpecPublicationSnapshots(cmd, parentProductNamesBySKU)
	return nil
}

func beanListProductSpecSelectionKey(parentProductID, skuID int64) string {
	return fmt.Sprintf("%d:%d", parentProductID, skuID)
}

func normalizeBeanListCommand(cmd PublishBeanListCommand) (PublishBeanListCommand, error) {
	listType, err := normalizeBeanListType(cmd.ListType)
	if err != nil {
		return PublishBeanListCommand{}, err
	}
	cmd.ListType = listType
	if cmd.ProductTypeCategoryID < 0 {
		return PublishBeanListCommand{}, fmt.Errorf("product_type_category_id must be >= 0")
	}
	if cmd.ClassificationTemplateID < 0 || cmd.ClassificationCategoryID < 0 {
		return PublishBeanListCommand{}, fmt.Errorf("classification ids must be >= 0")
	}
	cmd.ProductTypeName = strings.TrimSpace(cmd.ProductTypeName)
	cmd.ClassificationTemplateName = strings.TrimSpace(cmd.ClassificationTemplateName)
	cmd.ClassificationCategoryName = strings.TrimSpace(cmd.ClassificationCategoryName)
	if cmd.ProductTypeName == "" {
		if cmd.ClassificationTemplateName != "" {
			cmd.ProductTypeName = cmd.ClassificationTemplateName
		} else {
			cmd.ProductTypeName = LegacyBeanListTypeProductTypeName(cmd.ListType)
		}
	}
	if cmd.ClassificationTemplateID == 0 {
		cmd.ClassificationTemplateID = cmd.ProductTypeCategoryID
	}
	if cmd.ClassificationTemplateName == "" {
		cmd.ClassificationTemplateName = cmd.ProductTypeName
	}
	cmd.Version = strings.TrimSpace(cmd.Version)
	cmd.Changelog = strings.TrimSpace(cmd.Changelog)
	cmd.SourceVersion = strings.TrimSpace(cmd.SourceVersion)
	purpose, err := NormalizeBeanListPublicationPurpose(cmd.PublicationPurpose)
	if err != nil {
		return PublishBeanListCommand{}, err
	}
	cmd.PublicationPurpose = purpose
	if cmd.Version == "" {
		return PublishBeanListCommand{}, fmt.Errorf("version required")
	}
	ownerType, ownerKey, err := normalizeBeanListOwner(cmd.OwnerType, cmd.OwnerKey)
	if err != nil {
		return PublishBeanListCommand{}, err
	}
	cmd.OwnerType = ownerType
	cmd.OwnerKey = ownerKey
	if cmd.PriceSourcePublicationID < 0 || cmd.StyleSourcePublicationID < 0 {
		return PublishBeanListCommand{}, fmt.Errorf("source publication id must be >= 0")
	}
	if cmd.Config == nil {
		cmd.Config = map[string]any{}
	}
	if cmd.Content == nil {
		cmd.Content = map[string]any{}
	}
	if cmd.ListType == "green" {
		applyGreenBeanListManualPriceOverrides(cmd.Config, cmd.Content)
	}
	normalizeBeanListGroupSourceSnapshots(cmd.Content)
	return cmd, nil
}

func normalizeBeanListGroupSourceSnapshots(content map[string]any) {
	rows, ok := content["price_rows"].([]any)
	if !ok {
		return
	}
	for _, rawRow := range rows {
		row, ok := rawRow.(map[string]any)
		if !ok {
			continue
		}
		source := strings.TrimSpace(stringValue(row["group_source"]))
		switch source {
		case PriceListGroupSourcePriceList:
			row["group_source"] = PriceListGroupSourcePriceList
		case PriceListGroupSourceProductCatalog:
			row["group_source"] = PriceListGroupSourceProductCatalog
		default:
			if hasNonEmptyObjectSnapshot(row["group_snapshot"]) {
				row["group_source"] = PriceListGroupSourceProductCatalog
			}
		}
	}
}

func validateBeanListFinalPriceSnapshots(cmd PublishBeanListCommand) error {
	if err := validateBeanListFlatPriceRows(cmd); err != nil {
		return err
	}
	flatPriceRowProductKeys := beanListFlatPriceRowProductKeys(cmd.Content)
	groups, ok := cmd.Content["groups"].([]any)
	if !ok || len(groups) == 0 {
		return nil
	}
	tierKeys := []string{"commercial_wholesale_tiers", "green_bean_sale_tiers", "retail_bean_tiers", "drip_wholesale_tiers"}
	for groupIdx, rawGroup := range groups {
		group, ok := rawGroup.(map[string]any)
		if !ok {
			continue
		}
		items, ok := group["items"].([]any)
		if !ok {
			continue
		}
		for itemIdx, rawItem := range items {
			item, ok := rawItem.(map[string]any)
			if !ok {
				continue
			}
			itemHasFlatPriceRows := beanListItemHasFlatPriceRows(item, flatPriceRowProductKeys)
			for _, tierKey := range tierKeys {
				tiers, ok := item[tierKey].([]any)
				if !ok {
					continue
				}
				for tierIdx, rawTier := range tiers {
					tier, ok := rawTier.(map[string]any)
					if !ok || !beanListTierHasPrice(tier) {
						continue
					}
					if numberValue(tier["source_price_record_id"]) <= 0 {
						if !itemHasFlatPriceRows {
							return fmt.Errorf("价格表旧价格档不完整：第%d组第%d个商品第%d档。请重新生成价格表预览；如果该商品仍使用旧价格记录或阶梯方案，请先在商品价格管理中补齐已发布价格记录", groupIdx+1, itemIdx+1, tierIdx+1)
						}
					}
					if numberValue(tier["final_unit_price"]) <= 0 {
						return fmt.Errorf("价格表快照缺少最终价：第%d组第%d个商品第%d档", groupIdx+1, itemIdx+1, tierIdx+1)
					}
					if stringValue(tier["price_unit"]) == "" {
						return fmt.Errorf("价格表快照缺少价格单位：第%d组第%d个商品第%d档", groupIdx+1, itemIdx+1, tierIdx+1)
					}
					if stringValue(tier["inventory_unit"]) == "" {
						return fmt.Errorf("价格表快照缺少库存单位：第%d组第%d个商品第%d档", groupIdx+1, itemIdx+1, tierIdx+1)
					}
					if !hasBeanListInventoryConversion(tier["inventory_conversion_json"]) {
						return fmt.Errorf("价格表快照缺少价格单位到库存单位换算：第%d组第%d个商品第%d档", groupIdx+1, itemIdx+1, tierIdx+1)
					}
				}
			}
		}
	}
	return nil
}

func validateBeanListFlatPriceRows(cmd PublishBeanListCommand) error {
	rows, ok := cmd.Content["price_rows"].([]any)
	if !ok || len(rows) == 0 {
		return nil
	}
	for idx, rawRow := range rows {
		row, ok := rawRow.(map[string]any)
		if !ok {
			continue
		}
		if !beanListFlatPriceRowHasPrice(row) && strings.TrimSpace(stringValue(row["pricing_mode"])) == "" {
			continue
		}
		position := idx + 1
		if numberValue(row["final_unit_price"]) <= 0 {
			return fmt.Errorf("价格表平铺行缺少最终价：第%d行", position)
		}
		if stringValue(row["price_unit"]) == "" {
			return fmt.Errorf("价格表平铺行缺少价格单位：第%d行", position)
		}
		if stringValue(row["inventory_unit"]) == "" {
			return fmt.Errorf("价格表平铺行缺少库存单位：第%d行", position)
		}
		if !hasBeanListInventoryConversion(row["inventory_conversion_json"]) {
			return beanListFlatRowUnitConversionError("价格表平铺行缺少价格单位到库存单位换算", position, row, "")
		}
		if !hasNonEmptyObjectSnapshot(row["group_snapshot"]) {
			return fmt.Errorf("价格表平铺行缺少分组快照：第%d行", position)
		}
		groupSource := strings.TrimSpace(stringValue(row["group_source"]))
		if groupSource != PriceListGroupSourceProductCatalog && groupSource != PriceListGroupSourcePriceList {
			return fmt.Errorf("价格表平铺行缺少分组来源：第%d行", position)
		}
		mode := normalizePriceRowPricingMode(row)
		if mode == "" || stringValue(row["pricing_mode_source"]) == "" {
			return fmt.Errorf("价格表平铺行缺少计价模式来源：第%d行", position)
		}
		switch mode {
		case "tier_template":
			if numberValue(row["tier_template_id"]) <= 0 || stringValue(row["tier_template_source"]) == "" {
				return fmt.Errorf("价格表平铺行缺少阶梯模板来源：第%d行", position)
			}
			if numberValue(row["template_tier_id"]) <= 0 {
				return fmt.Errorf("价格表平铺行缺少阶梯模板档位：第%d行", position)
			}
			if numberValue(row["pricing_rule_id"]) <= 0 || stringValue(row["pricing_rule_source"]) == "" || stringValue(row["pricing_rule_version"]) == "" {
				return fmt.Errorf("价格表平铺行缺少 Pricing Rule 来源：第%d行", position)
			}
			if numberValue(row["tier_pricing_rule_id"]) <= 0 || stringValue(row["tier_pricing_rule_version"]) == "" {
				return fmt.Errorf("价格表平铺行缺少档位 Pricing Rule 来源：第%d行", position)
			}
		case "pricing_rule":
			if numberValue(row["pricing_rule_id"]) <= 0 || stringValue(row["pricing_rule_source"]) == "" || stringValue(row["pricing_rule_version"]) == "" {
				return fmt.Errorf("价格表平铺行缺少 Pricing Rule 来源：第%d行", position)
			}
		case "fixed_price":
			if numberValue(row["fixed_unit_price"]) <= 0 {
				return fmt.Errorf("价格表平铺行缺少固定价：第%d行", position)
			}
		default:
			return fmt.Errorf("价格表平铺行计价模式无效：第%d行", position)
		}
		if !hasNonEmptyObjectSnapshot(row["cost_source_snapshot"]) {
			return fmt.Errorf("价格表平铺行缺少成本来源快照：第%d行", position)
		}
		if err := validateBeanListFlatRowCostSourceSnapshot(row["cost_source_snapshot"], position); err != nil {
			return err
		}
		if _, exists := row["customer_reference_snapshot"]; !exists || !hasObjectSnapshot(row["customer_reference_snapshot"]) {
			return fmt.Errorf("价格表平铺行缺少客户引用展示快照：第%d行", position)
		}
		if _, exists := row["manual_adjusted"]; !exists {
			return fmt.Errorf("价格表平铺行缺少人工调整标记：第%d行", position)
		}
	}
	return nil
}

func validateBeanListFlatRowCostSourceSnapshot(value any, position int) error {
	snapshot, ok := objectSnapshotMap(value)
	if !ok {
		return nil
	}
	if beanListCostSourceSnapshotMissingBomOperationCost(snapshot) {
		return fmt.Errorf("价格表平铺行缺少 BOM 工序成本快照：第%d行。%s", position, pricingRuleTrialBomOperationSnapshotMissingWarning)
	}
	return nil
}

func beanListFlatRowUnitConversionError(prefix string, position int, row map[string]any, priceUnit string) error {
	if strings.TrimSpace(prefix) == "" {
		prefix = "价格表平铺行缺少价格单位到库存单位换算"
	}
	details := make([]string, 0, 4)
	if productName := beanListFlatRowString(row, "product_name", "display_name_snapshot", "product_name_snapshot", "name"); productName != "" {
		details = append(details, "商品："+productName)
	}
	if skuLabel := beanListFlatRowSKULabel(row); skuLabel != "" {
		details = append(details, "SKU："+skuLabel)
	}
	priceUnit = strings.TrimSpace(priceUnit)
	if priceUnit == "" {
		priceUnit = beanListFlatRowString(row, "price_unit", "priceUnit")
	}
	if priceUnit != "" {
		details = append(details, "价格单位："+priceUnit)
	}
	if inventoryUnit := beanListFlatRowString(row, "inventory_unit", "inventoryUnit"); inventoryUnit != "" {
		details = append(details, "库存单位："+inventoryUnit)
	}
	message := fmt.Sprintf("%s：第%d行", prefix, position)
	if len(details) > 0 {
		message = fmt.Sprintf("%s（%s）", message, strings.Join(details, "，"))
	}
	return fmt.Errorf("%s。请到 商品档案 → 销售规格模板 检查该规格的“1 规格 = 库存数量 库存单位”，或重新生成价格表预览", message)
}

func beanListFlatRowSKULabel(row map[string]any) string {
	name := beanListFlatRowString(row, "sku_name", "skuName", "spec_label", "specLabel", "derived_spec_name", "derivedSpecName")
	code := beanListFlatRowString(row, "sku_code", "skuCode")
	if snapshot, ok := objectSnapshotMap(row["sku_snapshot"]); ok {
		if name == "" {
			name = beanListFlatRowString(snapshot, "sku_name", "skuName", "spec_label", "specLabel")
		}
		if code == "" {
			code = beanListFlatRowString(snapshot, "sku_code", "skuCode")
		}
	}
	switch {
	case name != "" && code != "":
		return fmt.Sprintf("%s（%s）", name, code)
	case name != "":
		return name
	case code != "":
		return code
	default:
		return ""
	}
}

func beanListFlatRowString(row map[string]any, keys ...string) string {
	for _, key := range keys {
		value := strings.TrimSpace(stringValue(row[key]))
		if value != "" {
			return value
		}
	}
	return ""
}

func beanListCostSourceSnapshotMissingBomOperationCost(snapshot map[string]any) bool {
	if beanListStringSliceContains(snapshot["pricing_rule_trial_warnings"], pricingRuleTrialBomOperationSnapshotMissingWarning) {
		return true
	}
	for _, rawDetail := range beanListAnySlice(snapshot["pricing_rule_trial_base_cost_details"]) {
		detail, ok := objectSnapshotMap(rawDetail)
		if !ok {
			continue
		}
		if strings.TrimSpace(stringValue(detail["capacity_selection_source"])) == pricingRuleTrialBomOperationSnapshotMissingSource {
			return true
		}
		if strings.TrimSpace(stringValue(detail["warning"])) == pricingRuleTrialBomOperationSnapshotMissingWarning {
			return true
		}
	}
	return false
}

func beanListStringSliceContains(value any, want string) bool {
	want = strings.TrimSpace(want)
	if want == "" {
		return false
	}
	for _, raw := range beanListAnySlice(value) {
		if strings.TrimSpace(stringValue(raw)) == want {
			return true
		}
	}
	if strings.TrimSpace(stringValue(value)) == want {
		return true
	}
	return false
}

func beanListAnySlice(value any) []any {
	switch v := value.(type) {
	case []any:
		return v
	case []map[string]any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, item)
		}
		return out
	case json.RawMessage:
		return beanListAnySlice(string(v))
	case []byte:
		return beanListAnySlice(string(v))
	case string:
		v = strings.TrimSpace(v)
		if v == "" || v == "null" {
			return nil
		}
		var decoded []any
		if err := json.Unmarshal([]byte(v), &decoded); err != nil {
			return nil
		}
		return decoded
	default:
		return nil
	}
}

func objectSnapshotMap(value any) (map[string]any, bool) {
	switch v := value.(type) {
	case map[string]any:
		return v, true
	case json.RawMessage:
		return objectSnapshotMap(string(v))
	case []byte:
		return objectSnapshotMap(string(v))
	case string:
		v = strings.TrimSpace(v)
		if v == "" || v == "{}" || v == "null" {
			return nil, false
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(v), &decoded); err != nil {
			return nil, false
		}
		return decoded, decoded != nil
	default:
		return nil, false
	}
}

func normalizePriceRowPricingMode(row map[string]any) string {
	switch strings.TrimSpace(stringValue(row["pricing_mode"])) {
	case "tier_template", "inherit_gradient_template":
		return "tier_template"
	case "pricing_rule", "cost_plus":
		return "pricing_rule"
	case "fixed_price", "fixed_unit_price":
		return "fixed_price"
	}
	switch {
	case numberValue(row["tier_template_id"]) > 0:
		return "tier_template"
	case numberValue(row["pricing_rule_id"]) > 0:
		return "pricing_rule"
	case numberValue(row["fixed_unit_price"]) > 0:
		return "fixed_price"
	default:
		return ""
	}
}

func beanListFlatPriceRowHasPrice(row map[string]any) bool {
	return numberValue(row["final_unit_price"]) > 0 || numberValue(row["original_final_unit_price"]) > 0
}

func beanListFlatPriceRowProductKeys(content map[string]any) map[string]bool {
	out := map[string]bool{}
	rows, ok := content["price_rows"].([]any)
	if !ok {
		return out
	}
	for _, rawRow := range rows {
		row, ok := rawRow.(map[string]any)
		if !ok || !beanListFlatPriceRowHasPrice(row) {
			continue
		}
		for _, key := range []string{"product_id", "productID", "productId", "product_key", "productKey", "product_name", "productName", "name"} {
			value := stringValue(row[key])
			if value != "" {
				out[value] = true
			}
		}
	}
	return out
}

func beanListItemHasFlatPriceRows(item map[string]any, flatRowProductKeys map[string]bool) bool {
	if len(flatRowProductKeys) == 0 {
		return false
	}
	for _, key := range []string{"product_id", "productID", "productId", "id", "product_key", "productKey", "product_name", "productName", "name"} {
		value := stringValue(item[key])
		if value != "" && flatRowProductKeys[value] {
			return true
		}
	}
	return false
}

func beanListTierHasPrice(tier map[string]any) bool {
	for _, key := range []string{"final_unit_price", "price_per_unit", "price_per_kg", "price_per_lb", "packed_price_per_bag", "packed_price_per_box"} {
		if numberValue(tier[key]) > 0 {
			return true
		}
	}
	return false
}

func hasBeanListInventoryConversion(value any) bool {
	switch v := value.(type) {
	case map[string]any:
		return len(v) > 0
	case map[string]map[string]any:
		return len(v) > 0
	case json.RawMessage:
		return hasBeanListInventoryConversion(string(v))
	case []byte:
		return hasBeanListInventoryConversion(string(v))
	case string:
		v = strings.TrimSpace(v)
		if v == "" || v == "{}" || v == "null" {
			return false
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(v), &decoded); err != nil {
			return false
		}
		return len(decoded) > 0
	default:
		return false
	}
}

func hasObjectSnapshot(value any) bool {
	switch v := value.(type) {
	case map[string]any:
		return true
	case json.RawMessage:
		return hasObjectSnapshot(string(v))
	case []byte:
		return hasObjectSnapshot(string(v))
	case string:
		v = strings.TrimSpace(v)
		if v == "" || v == "null" {
			return false
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(v), &decoded); err != nil {
			return false
		}
		return decoded != nil
	default:
		return false
	}
}

func hasNonEmptyObjectSnapshot(value any) bool {
	switch v := value.(type) {
	case map[string]any:
		return len(v) > 0
	case json.RawMessage:
		return hasNonEmptyObjectSnapshot(string(v))
	case []byte:
		return hasNonEmptyObjectSnapshot(string(v))
	case string:
		v = strings.TrimSpace(v)
		if v == "" || v == "{}" || v == "null" {
			return false
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(v), &decoded); err != nil {
			return false
		}
		return len(decoded) > 0
	default:
		return false
	}
}

func applyGreenBeanListManualPriceOverrides(config map[string]any, content map[string]any) {
	overridesByProduct := greenPriceOverridesByProduct(config)
	if len(overridesByProduct) == 0 {
		return
	}
	groups, ok := content["groups"].([]any)
	if !ok {
		return
	}
	for _, rawGroup := range groups {
		group, ok := rawGroup.(map[string]any)
		if !ok {
			continue
		}
		items, ok := group["items"].([]any)
		if !ok {
			continue
		}
		for _, rawItem := range items {
			item, ok := rawItem.(map[string]any)
			if !ok {
				continue
			}
			overrides := overridesByProduct[beanListItemProductKey(item)]
			if len(overrides) == 0 {
				overrides = overridesByProduct[stringValue(item["name"])]
			}
			if len(overrides) == 0 {
				continue
			}
			tiers, ok := item["green_bean_sale_tiers"].([]any)
			if !ok {
				continue
			}
			changed := false
			for _, rawTier := range tiers {
				tier, ok := rawTier.(map[string]any)
				if !ok {
					continue
				}
				price, ok := greenTierManualOverride(tier, overrides)
				if !ok {
					continue
				}
				applyGreenTierManualPrice(tier, price)
				changed = true
			}
			if changed {
				item["prices"] = greenBeanPriceRowsFromTiers(tiers)
			}
		}
	}
}

func greenPriceOverridesByProduct(config map[string]any) map[string]map[string]float64 {
	customizers, ok := config["customizers"].(map[string]any)
	if !ok {
		return nil
	}
	out := map[string]map[string]float64{}
	for productKey, rawCustomizer := range customizers {
		customizer, ok := rawCustomizer.(map[string]any)
		if !ok {
			continue
		}
		rawOverrides, ok := customizer["greenPriceOverrides"].(map[string]any)
		if !ok {
			continue
		}
		for tierKey, rawPrice := range rawOverrides {
			price := numberValue(rawPrice)
			if price <= 0 || strings.TrimSpace(tierKey) == "" {
				continue
			}
			key := strings.TrimSpace(productKey)
			if key == "" {
				continue
			}
			if out[key] == nil {
				out[key] = map[string]float64{}
			}
			out[key][strings.TrimSpace(tierKey)] = price
		}
	}
	return out
}

func greenTierManualOverride(tier map[string]any, overrides map[string]float64) (float64, bool) {
	for _, key := range []string{
		stringValue(tier["template_tier_id"]),
		stringValue(tier["templateTierID"]),
		stringValue(tier["label"]),
	} {
		if key == "" {
			continue
		}
		price, ok := overrides[key]
		if ok && price > 0 {
			return price, true
		}
	}
	return 0, false
}

func applyGreenTierManualPrice(tier map[string]any, price float64) {
	unitPrice := roundBeanListPrice(price)
	priceUnit := greenBeanTierPriceUnit(tier, true)
	unitG := greenBeanPriceUnitG(priceUnit, tier)
	pricePerKg := unitPrice
	if unitG > 0 && unitG != 1000 {
		pricePerKg = unitPrice * 1000.0 / unitG
	}
	tier["price_unit"] = priceUnit
	tier["final_unit_price"] = unitPrice
	tier["price_per_unit"] = unitPrice
	tier["price_per_kg"] = roundBeanListPrice(pricePerKg)
	tier["price_per_lb"] = roundBeanListPrice(pricePerKg * domain.DefaultParameters().KgToLbFactor)
}

func greenBeanPriceRowsFromTiers(tiers []any) []any {
	rows := make([]any, 0, len(tiers))
	for _, rawTier := range tiers {
		tier, ok := rawTier.(map[string]any)
		if !ok {
			continue
		}
		rows = append(rows, map[string]any{
			"label": stringValue(tier["label"]),
			"price": greenBeanDisplayPrice(tier),
			"unit":  greenBeanPriceUnitLabel(tier),
			"red":   false,
		})
	}
	return rows
}

func greenBeanPriceUnitLabel(tier map[string]any) string {
	switch greenBeanTierPriceUnit(tier, false) {
	case "kg":
		return "kg"
	case "g100":
		return "100g"
	case "g227":
		return "227g"
	case "g250":
		return "250g"
	case "lb":
		return "磅"
	}
	switch normalizeGreenBeanPriceUnit(stringValue(tier["display_unit"])) {
	case "kg":
		return "kg"
	case "g100":
		return "100g"
	case "g227":
		return "227g"
	case "g250":
		return "250g"
	default:
		return "磅"
	}
}

func greenBeanDisplayPrice(tier map[string]any) float64 {
	priceUnit := greenBeanTierPriceUnit(tier, false)
	pricePerKg := greenBeanPricePerKg(tier)
	switch priceUnit {
	case "kg":
		return roundBeanListPrice(firstPositiveNumber(pricePerKg, numberValue(tier["price_per_unit"])))
	case "lb":
		return roundBeanListPrice(firstPositiveNumber(numberValue(tier["price_per_lb"]), pricePerKg*domain.DefaultParameters().KgToLbFactor, numberValue(tier["price_per_unit"])))
	default:
		unitG := greenBeanPriceUnitG(priceUnit, tier)
		if unitG > 0 && pricePerKg > 0 {
			return roundBeanListPrice(pricePerKg * unitG / 1000.0)
		}
		return roundBeanListPrice(numberValue(tier["price_per_unit"]))
	}
}

func greenBeanPricePerKg(tier map[string]any) float64 {
	if price := numberValue(tier["price_per_kg"]); price > 0 {
		return price
	}
	if normalizeGreenBeanPriceUnit(stringValue(tier["display_unit"])) == "kg" {
		if price := numberValue(tier["price_per_unit"]); price > 0 {
			return price
		}
	}
	if price := numberValue(tier["price_per_lb"]); price > 0 {
		return price / domain.DefaultParameters().KgToLbFactor
	}
	return 0
}

func greenBeanTierPriceUnit(tier map[string]any, preferDisplay bool) string {
	displayUnit := normalizeGreenBeanPriceUnit(stringValue(tier["display_unit"]))
	explicitUnit := normalizeGreenBeanPriceUnit(stringValue(tier["price_unit"]))
	if preferDisplay {
		return firstNonEmpty(displayUnit, explicitUnit, "lb")
	}
	return firstNonEmpty(explicitUnit, displayUnit, "lb")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func normalizeGreenBeanPriceUnit(unit string) string {
	switch strings.TrimSpace(strings.ToLower(unit)) {
	case "kg", "lb", "g100", "g227", "g250":
		return strings.TrimSpace(strings.ToLower(unit))
	default:
		return ""
	}
}

func greenBeanPriceUnitG(unit string, tier map[string]any) float64 {
	switch normalizeGreenBeanPriceUnit(unit) {
	case "kg":
		return 1000
	case "lb":
		return 454
	case "g100":
		return 100
	case "g227":
		return 227
	case "g250":
		return 250
	default:
		specG := numberValue(tier["spec_g"])
		if specG > 0 {
			return specG
		}
		return 454
	}
}

func beanListItemProductKey(item map[string]any) string {
	for _, key := range []string{"product_id", "productID", "productId", "id", "name"} {
		value := stringValue(item[key])
		if value != "" {
			return value
		}
	}
	return ""
}

func stringValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case json.Number:
		return strings.TrimSpace(v.String())
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func numberValue(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		n, _ := v.Float64()
		return n
	case string:
		n, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return n
	default:
		return 0
	}
}

func firstPositiveNumber(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func roundBeanListPrice(value float64) float64 {
	return math.Round((value+1e-9)*100) / 100
}

func (s *Service) WithdrawBeanList(ctx context.Context, cmd WithdrawBeanListCommand) error {
	if cmd.ID <= 0 {
		return fmt.Errorf("invalid id")
	}
	ownerType, ownerKey, err := normalizeBeanListOwner(cmd.OwnerType, cmd.OwnerKey)
	if err != nil {
		return err
	}
	cmd.OwnerType = ownerType
	cmd.OwnerKey = ownerKey
	purpose, err := NormalizeBeanListPublicationPurpose(cmd.PublicationPurpose)
	if err != nil {
		return err
	}
	cmd.PublicationPurpose = purpose
	if s.repo == nil {
		return fmt.Errorf("repository required")
	}
	return s.repo.WithdrawBeanList(ctx, cmd)
}

func (s *Service) ArchiveBeanListPublications(ctx context.Context, cmd ArchiveBeanListPublicationsCommand) error {
	normalized, err := normalizeArchiveBeanListPublicationsCommand(cmd)
	if err != nil {
		return err
	}
	if s.repo == nil {
		return fmt.Errorf("repository required")
	}
	return s.repo.ArchiveBeanListPublications(ctx, normalized)
}

func (s *Service) UnarchiveBeanListPublications(ctx context.Context, cmd ArchiveBeanListPublicationsCommand) error {
	normalized, err := normalizeArchiveBeanListPublicationsCommand(cmd)
	if err != nil {
		return err
	}
	if s.repo == nil {
		return fmt.Errorf("repository required")
	}
	return s.repo.UnarchiveBeanListPublications(ctx, normalized)
}

func normalizeArchiveBeanListPublicationsCommand(cmd ArchiveBeanListPublicationsCommand) (ArchiveBeanListPublicationsCommand, error) {
	ids := make([]int64, 0, len(cmd.IDs))
	seen := map[int64]bool{}
	for _, id := range cmd.IDs {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return ArchiveBeanListPublicationsCommand{}, fmt.Errorf("ids required")
	}
	cmd.IDs = ids
	ownerType, ownerKey, err := normalizeBeanListOwner(cmd.OwnerType, cmd.OwnerKey)
	if err != nil {
		return ArchiveBeanListPublicationsCommand{}, err
	}
	cmd.OwnerType = ownerType
	cmd.OwnerKey = ownerKey
	purpose, err := NormalizeBeanListPublicationPurpose(cmd.PublicationPurpose)
	if err != nil {
		return ArchiveBeanListPublicationsCommand{}, err
	}
	cmd.PublicationPurpose = purpose
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	return cmd, nil
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

func normalizeBeanListType(listType string) (string, error) {
	switch strings.TrimSpace(listType) {
	case "", "commercial":
		return "commercial", nil
	case "drip":
		return "drip", nil
	case "retail":
		return "retail", nil
	case "green", "green_bean":
		return "green", nil
	default:
		return "", fmt.Errorf("invalid list_type")
	}
}

func LegacyBeanListTypeProductTypeName(listType string) string {
	switch strings.TrimSpace(listType) {
	case "green", "green_bean":
		return "生豆"
	case "drip":
		return "挂耳"
	default:
		return "熟豆"
	}
}

func normalizeBeanListPublicationQuery(query BeanListPublicationQuery) (BeanListPublicationQuery, error) {
	listType, err := normalizeBeanListType(query.ListType)
	if err != nil {
		return BeanListPublicationQuery{}, err
	}
	if query.ProductTypeCategoryID < 0 {
		return BeanListPublicationQuery{}, fmt.Errorf("product_type_category_id must be >= 0")
	}
	if query.ClassificationTemplateID < 0 {
		return BeanListPublicationQuery{}, fmt.Errorf("classification_template_id must be >= 0")
	}
	purpose, err := NormalizeBeanListPublicationPurpose(query.PublicationPurpose)
	if err != nil {
		return BeanListPublicationQuery{}, err
	}
	ownerType, ownerKey, err := normalizeBeanListOwner(query.OwnerType, query.OwnerKey)
	if err != nil {
		return BeanListPublicationQuery{}, err
	}
	query.ListType = listType
	query.PublicationPurpose = purpose
	query.OwnerType = ownerType
	query.OwnerKey = ownerKey
	return query, nil
}

func NormalizeBeanListPublicationPurpose(value string) (string, error) {
	switch strings.TrimSpace(value) {
	case "", BeanListPublicationPurposeFactorySupply:
		return BeanListPublicationPurposeFactorySupply, nil
	case BeanListPublicationPurposeCustomerResale:
		return BeanListPublicationPurposeCustomerResale, nil
	default:
		return "", fmt.Errorf("invalid publication_purpose")
	}
}

func normalizeBeanListPublicationPDFCommand(cmd BeanListPublicationPDFCommand) (BeanListPublicationPDFCommand, error) {
	if cmd.PublicationID <= 0 {
		return BeanListPublicationPDFCommand{}, fmt.Errorf("invalid id")
	}
	query, err := normalizeBeanListPublicationQuery(cmd.Query)
	if err != nil {
		return BeanListPublicationPDFCommand{}, err
	}
	cmd.Query = query
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	return cmd, nil
}

var (
	beanListPublicationPDFFilenameUnsafeChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)
	beanListPublicationVersionNumberPattern   = regexp.MustCompile(`\d+`)
	beanListPublicationStandardVersionPattern = regexp.MustCompile(`(?i)^v\d+(?:\.\d+)*$`)
)

func beanListPublicationPDFFile(row BeanListPublication, asset BeanListPublicationAsset) BeanListPublicationPDFFile {
	contentType := strings.TrimSpace(asset.ContentType)
	if contentType == "" {
		contentType = "application/pdf"
	}
	cacheKey := strings.TrimSpace(asset.CacheKey)
	if cacheKey == "" {
		cacheKey = beanListPublicationPDFCacheKey(row)
	}
	return BeanListPublicationPDFFile{
		PublicationID: row.ID,
		ListType:      row.ListType,
		Version:       row.Version,
		ContentType:   contentType,
		CacheKey:      cacheKey,
		Filename:      beanListPublicationPDFFilename(row),
		Bytes:         len(asset.Payload),
		Payload:       asset.Payload,
	}
}

func beanListPublicationPDFCacheKey(row BeanListPublication) string {
	version := strings.TrimSpace(row.Version)
	if version == "" {
		version = "published"
	}
	return fmt.Sprintf("bean-list-preview-style-v4:%d:%s", row.ID, version)
}

func beanListPublicationPDFFilename(row BeanListPublication) string {
	listType := strings.TrimSpace(row.ListType)
	if listType == "" {
		listType = "bean-list"
	}
	version := strings.TrimSpace(row.Version)
	if version == "" {
		version = fmt.Sprintf("%d", row.ID)
	}
	return "bean-list-" + beanListPublicationPDFFilenameUnsafeChars.ReplaceAllString(listType+"-"+version, "-") + ".pdf"
}

func normalizeBeanListOwner(ownerType, ownerKey string) (string, string, error) {
	typ := strings.TrimSpace(ownerType)
	key := strings.TrimSpace(ownerKey)
	switch typ {
	case "", "official":
		return "official", "", nil
	case "actor", "customer":
		if key == "" {
			return "", "", fmt.Errorf("owner_key required")
		}
		return typ, key, nil
	default:
		return "", "", fmt.Errorf("invalid owner_type")
	}
}

func sortBeanListResults(items []domain.ProductResult) {
	sort.SliceStable(items, func(i, j int) bool {
		return compareBeanListCodes(beanListSortCode(items[i]), beanListSortCode(items[j])) < 0
	})
}

func beanListSortCode(item domain.ProductResult) string {
	if item.CommercialBeanList.Code != "" {
		return item.CommercialBeanList.Code
	}
	if item.RetailBeanList.Code != "" {
		return item.RetailBeanList.Code
	}
	if item.GreenBeanList.Code != "" {
		return item.GreenBeanList.Code
	}
	return "9999"
}

func compareBeanListCodes(a, b string) int {
	aa := parseBeanListCode(a)
	bb := parseBeanListCode(b)
	max := len(aa)
	if len(bb) > max {
		max = len(bb)
	}
	for i := 0; i < max; i++ {
		var av, bv int
		if i < len(aa) {
			av = aa[i]
		}
		if i < len(bb) {
			bv = bb[i]
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return strings.Compare(a, b)
}

func parseBeanListCode(code string) []int {
	parts := strings.Split(code, ".")
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			n = 0
		}
		out = append(out, n)
	}
	return out
}

func defaultParameterSettings() []ParameterSetting {
	params := domain.DefaultParameters()
	return []ParameterSetting{
		{Key: "roast_yield_rate", Label: "生豆到熟豆转化率", Value: params.RoastYieldRate, Unit: "ratio"},
		{Key: "kg_to_lb_factor", Label: "kg 到 lb 换算", Value: params.KgToLbFactor, Unit: "lb/kg"},
		{Key: "small_batch_production_cost_per_kg", Label: "小批量生产成本", Value: params.SmallBatchProductionCostPerKg, Unit: "元/kg"},
		{Key: "large_batch_production_cost_per_kg", Label: "大批量生产成本", Value: params.LargeBatchProductionCostPerKg, Unit: "元/kg"},
		{Key: "wholesale_package_cost_per_kg", Label: "批发包装成本", Value: params.WholesalePackageCostPerKg, Unit: "元/kg"},
		{Key: "product_loss_per_kg", Label: "产品损耗", Value: params.ProductLossPerKg, Unit: "元/kg"},
		{Key: "retail_bean_margin_rate", Label: "零售熟豆利润系数", Value: params.RetailBeanMarginRate, Unit: "ratio"},
		{Key: "retail_tax_rate", Label: "零售税率", Value: params.RetailTaxRate, Unit: "ratio"},
		{Key: "retail_logistics_per_kg", Label: "零售熟豆物流", Value: params.RetailLogisticsPerKg, Unit: "元/kg"},
		{Key: "retail_drip_logistics_per_10_bags", Label: "零售挂耳物流", Value: params.RetailDripLogisticsPer10Bags, Unit: "元/10袋"},
		{Key: "drip_green_ratio_kg_per_bag", Label: "挂耳单袋咖啡消耗", Value: params.DripGreenRatioKgPerBag, Unit: "kg/袋"},
		{Key: "drip_process_cost_per_bag", Label: "挂耳加工成本", Value: params.DripProcessCostPerBag, Unit: "元/袋"},
		{Key: "drip_extra_cost_per_bag", Label: "挂耳额外成本", Value: params.DripExtraCostPerBag, Unit: "元/袋"},
		{Key: "drip_packing_material_per_bag", Label: "挂耳外包装材料", Value: params.DripPackingMaterialPerBag, Unit: "元/袋"},
		{Key: "retail_drip_multiplier", Label: "零售挂耳利润系数", Value: params.RetailDripMultiplier, Unit: "ratio"},
		{Key: "wholesale_kg_margin_rate_1", Label: "商用熟豆 2包-13包 利润系数", Value: params.WholesaleKgMarginRates[0], Unit: "ratio"},
		{Key: "wholesale_kg_margin_rate_2", Label: "商用熟豆 14包-23包 利润系数", Value: params.WholesaleKgMarginRates[1], Unit: "ratio"},
		{Key: "wholesale_kg_margin_rate_3", Label: "商用熟豆 24包-47包 利润系数", Value: params.WholesaleKgMarginRates[2], Unit: "ratio"},
		{Key: "wholesale_kg_margin_rate_4", Label: "商用熟豆 48包+ / 24-49kg 利润系数", Value: params.WholesaleKgMarginRates[3], Unit: "ratio"},
		{Key: "wholesale_kg_margin_rate_5", Label: "商用熟豆 50-99kg 利润系数", Value: params.WholesaleKgMarginRates[4], Unit: "ratio"},
		{Key: "wholesale_kg_margin_rate_6", Label: "商用熟豆 100-199kg 利润系数", Value: params.WholesaleKgMarginRates[5], Unit: "ratio"},
		{Key: "wholesale_drip_multiplier_1", Label: "商用挂耳 100包 利润系数", Value: params.WholesaleDripMultipliers[0], Unit: "ratio"},
		{Key: "wholesale_drip_multiplier_2", Label: "商用挂耳 200包 利润系数", Value: params.WholesaleDripMultipliers[1], Unit: "ratio"},
		{Key: "wholesale_drip_multiplier_3", Label: "商用挂耳 300包 利润系数", Value: params.WholesaleDripMultipliers[2], Unit: "ratio"},
		{Key: "wholesale_drip_multiplier_4", Label: "商用挂耳 500包 利润系数", Value: params.WholesaleDripMultipliers[3], Unit: "ratio"},
	}
}
