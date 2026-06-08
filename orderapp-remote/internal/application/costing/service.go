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

	domain "orderapp/internal/domain/costing"
)

var (
	ErrBeanListPublicationNotFound = errors.New("bean list publication not found")
	ErrProductPricingRuleNotFound  = errors.New("product pricing rule not found")
)

const (
	BeanListPublicationPurposeFactorySupply  = "factory_supply"
	BeanListPublicationPurposeCustomerResale = "customer_resale"
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

type PricingRuleTrialCommand struct {
	PricingRuleID int64                     `json:"pricing_rule_id"`
	ProductID     int64                     `json:"product_id"`
	CustomerID    int64                     `json:"customer_id,omitempty"`
	QuoteUnit     string                    `json:"quote_unit,omitempty"`
	Overrides     PricingRuleTrialOverrides `json:"overrides,omitempty"`
}

type PricingRuleTrialOverrides struct {
	ExpectedLossRate *float64           `json:"expected_loss_rate,omitempty"`
	BaseCost         *float64           `json:"base_cost,omitempty"`
	MarginRate       *float64           `json:"margin_rate,omitempty"`
	TaxRate          *float64           `json:"tax_rate,omitempty"`
	OtherCosts       map[string]float64 `json:"other_costs,omitempty"`
	PostMarkupCosts  map[string]float64 `json:"post_markup_costs,omitempty"`
}

type PricingRuleTrialResult struct {
	PricingRuleID          int64                            `json:"pricing_rule_id"`
	PricingRuleName        string                           `json:"pricing_rule_name"`
	FormulaVersion         string                           `json:"formula_version"`
	ProductID              int64                            `json:"product_id"`
	ProductName            string                           `json:"product_name"`
	QuoteUnit              string                           `json:"quote_unit"`
	InventoryUnit          string                           `json:"inventory_unit"`
	BomVersionID           int64                            `json:"bom_version_id,omitempty"`
	BomVersionNo           string                           `json:"bom_version_no,omitempty"`
	BomUsageMode           string                           `json:"bom_usage_mode,omitempty"`
	BomStatus              string                           `json:"bom_status,omitempty"`
	BaseCost               float64                          `json:"base_cost"`
	BomCostTotal           float64                          `json:"bom_cost_total"`
	OperationCostTotal     float64                          `json:"operation_cost_total"`
	BaseCostDetails        []PricingRuleTrialBaseCostDetail `json:"base_cost_details,omitempty"`
	OtherCostTotal         float64                          `json:"other_cost_total"`
	CostBaseTotal          float64                          `json:"cost_base_total"`
	CostAfterYield         float64                          `json:"cost_after_yield"`
	YieldLossAmount        float64                          `json:"yield_loss_amount"`
	PriceAfterMarkup       float64                          `json:"price_after_markup,omitempty"`
	ProfitMarkupAmount     float64                          `json:"profit_markup_amount"`
	PostMarkupCostTotal    float64                          `json:"post_markup_cost_total,omitempty"`
	PreTaxPrice            float64                          `json:"pre_tax_price"`
	TaxAmount              float64                          `json:"tax_amount"`
	TaxInPriceAmount       float64                          `json:"tax_in_price_amount"`
	FinalBeforeRounding    float64                          `json:"final_before_rounding"`
	RoundingAdjustment     float64                          `json:"rounding_adjustment"`
	FinalUnitPrice         float64                          `json:"final_unit_price"`
	GrossMarginRate        float64                          `json:"gross_margin_rate"`
	MinimumMarginRate      float64                          `json:"minimum_margin_rate"`
	FormulaExpression      string                           `json:"formula_expression,omitempty"`
	FormulaExpressionLines []string                         `json:"formula_expression_lines,omitempty"`
	Steps                  []domain.PriceExplanationStep    `json:"steps"`
	Warnings               []string                         `json:"warnings,omitempty"`
}

type PricingRuleTrialBaseCostDetail struct {
	Key           string  `json:"key,omitempty"`
	Type          string  `json:"type"`
	TypeLabel     string  `json:"type_label"`
	Name          string  `json:"name"`
	ConsumeUnit   string  `json:"consume_unit,omitempty"`
	Quantity      float64 `json:"quantity,omitempty"`
	RatioPct      float64 `json:"ratio_pct,omitempty"`
	UnitCost      float64 `json:"unit_cost,omitempty"`
	Amount        float64 `json:"amount"`
	Unit          string  `json:"unit"`
	Description   string  `json:"description,omitempty"`
	AmountPerKg   float64 `json:"-"`
	AmountPerUnit float64 `json:"-"`
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
}

type customerScopedProductInputRepository interface {
	LoadProductInputsForCustomer(ctx context.Context, params domain.Parameters, customerID int64) ([]domain.ProductInput, error)
}

type pricingRuleTrialBaseCostDetailRepository interface {
	LoadPricingRuleTrialBaseCostDetails(ctx context.Context, input domain.ProductInput) ([]PricingRuleTrialBaseCostDetail, error)
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
	var baseCostDetails []PricingRuleTrialBaseCostDetail
	if detailRepo, ok := s.repo.(pricingRuleTrialBaseCostDetailRepository); ok {
		baseCostDetails, err = detailRepo.LoadPricingRuleTrialBaseCostDetails(ctx, input)
		if err != nil {
			return nil, err
		}
	}
	return calculatePricingRuleTrial(rule, input, cmd, baseCostDetails)
}

func (s *Service) pricingRuleTrialProductInputs(ctx context.Context, params domain.Parameters, customerID int64) ([]domain.ProductInput, error) {
	if customerID > 0 {
		if scoped, ok := s.repo.(customerScopedProductInputRepository); ok {
			return scoped.LoadProductInputsForCustomer(ctx, params, customerID)
		}
	}
	return s.repo.LoadProductInputs(ctx, params)
}

func calculatePricingRuleTrial(rule ProductPricingRule, input domain.ProductInput, cmd PricingRuleTrialCommand, rawBaseCostDetails []PricingRuleTrialBaseCostDetail) (*PricingRuleTrialResult, error) {
	calc := rule.CalculationJSON
	if calc == nil {
		calc = map[string]any{}
	}
	quoteUnit := strings.TrimSpace(cmd.QuoteUnit)
	if quoteUnit == "" {
		quoteUnit = strings.TrimSpace(input.QuoteUnit)
	}
	if quoteUnit == "" {
		quoteUnit = strings.TrimSpace(input.InventoryUnit)
	}
	if quoteUnit == "" {
		quoteUnit = "kg"
	}
	warnings := append([]string{}, input.Warnings...)
	bomStatus := strings.TrimSpace(input.BomStatus)
	switch bomStatus {
	case "inactive", "disabled":
		warnings = appendUniqueString(warnings, "BOM已失效：试算结果仅供参考")
	case "missing":
		warnings = appendUniqueString(warnings, "该商品暂无可试算的 BOM/工序成本")
	}
	if !rule.Active {
		warnings = appendUniqueString(warnings, "停用模板：试算仅供查看，不能作为新发布价格来源")
	}

	baseCost, baseSource, baseWarning := pricingRuleTrialBaseCost(input, quoteUnit)
	if cmd.Overrides.BaseCost != nil {
		if *cmd.Overrides.BaseCost < 0 {
			return nil, fmt.Errorf("base_cost must be >= 0")
		}
		baseCost = *cmd.Overrides.BaseCost
		baseSource = "temporary_override"
	}
	if baseWarning != "" {
		warnings = appendUniqueString(warnings, baseWarning)
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
	profitMethod := pricingRuleTrialString(calc, "profit_method", "gross_margin")
	marginRate := rule.MarginRate
	if cmd.Overrides.MarginRate != nil {
		marginRate = *cmd.Overrides.MarginRate
	}
	if marginRate < 0 {
		return nil, fmt.Errorf("margin_rate must be >= 0")
	}

	taxMode := pricingRuleTrialString(calc, "tax_mode", "tax_included")
	taxRate := rule.TaxRate
	if cmd.Overrides.TaxRate != nil {
		taxRate = *cmd.Overrides.TaxRate
	}
	if taxRate < 0 {
		return nil, fmt.Errorf("tax_rate must be >= 0")
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
	if baseCost <= 0 && cmd.Overrides.BaseCost == nil {
		detailBaseCost := bomCostTotal + operationCostTotal
		if detailBaseCost > 0 {
			baseCost = detailBaseCost
		}
	}
	if baseCost <= 0 {
		warnings = appendUniqueString(warnings, "该商品暂无可试算的 BOM/工序成本")
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
	if formulaMode == "supplier_tier_markup" {
		profitParameterRate := pricingRuleTrialNumber(calc, "profit_parameter_rate", 0)
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
	roundingAdjustment := finalUnitPrice - finalBeforeRoundingRounded
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
	result := &PricingRuleTrialResult{
		PricingRuleID:       rule.ID,
		PricingRuleName:     firstNonEmptyString(rule.Name, rule.Code),
		FormulaVersion:      firstNonEmptyString(rule.FormulaVersion, "v1"),
		ProductID:           input.ProductID,
		ProductName:         productName,
		QuoteUnit:           quoteUnit,
		InventoryUnit:       strings.TrimSpace(input.InventoryUnit),
		BomVersionID:        input.BomVersionID,
		BomVersionNo:        input.BomVersionNo,
		BomUsageMode:        input.BomUsageMode,
		BomStatus:           input.BomStatus,
		BaseCost:            pricingRuleTrialResultAmount(formulaMode, baseCost),
		BomCostTotal:        pricingRuleTrialResultAmount(formulaMode, bomCostTotal),
		OperationCostTotal:  pricingRuleTrialResultAmount(formulaMode, operationCostTotal),
		BaseCostDetails:     baseCostDetails,
		OtherCostTotal:      pricingRuleTrialResultAmount(formulaMode, otherCostTotal),
		CostBaseTotal:       pricingRuleTrialResultAmount(formulaMode, costBaseTotal),
		CostAfterYield:      pricingRuleTrialResultAmount(formulaMode, costAfterYield),
		YieldLossAmount:     pricingRuleTrialResultAmount(formulaMode, yieldLossAmount),
		PriceAfterMarkup:    pricingRuleTrialResultAmount(formulaMode, priceAfterMarkup),
		ProfitMarkupAmount:  pricingRuleTrialResultAmount(formulaMode, profitMarkupAmount),
		PostMarkupCostTotal: pricingRuleTrialResultAmount(formulaMode, postMarkupCostTotal),
		PreTaxPrice:         pricingRuleTrialResultAmount(formulaMode, preTaxPrice),
		TaxAmount:           pricingRuleTrialResultAmount(formulaMode, taxAmount),
		TaxInPriceAmount:    pricingRuleTrialResultAmount(formulaMode, taxInPriceAmount),
		FinalBeforeRounding: finalBeforeRoundingRounded,
		RoundingAdjustment:  pricingRuleTrialResultAmount(formulaMode, roundingAdjustment),
		FinalUnitPrice:      finalUnitPrice,
		GrossMarginRate:     roundRatio(grossMarginRate),
		MinimumMarginRate:   roundRatio(minimumMarginRate),
		Warnings:            warnings,
	}
	if formulaMode == "supplier_tier_markup" {
		result.Steps = pricingRuleTrialSupplierSteps(result, cmd, quoteUnit, baseSource, otherCosts, postMarkupCosts, expectedLossRate, yieldMode, lossChanged, marginRate, pricingRuleTrialNumber(calc, "profit_parameter_rate", 0), taxMode, taxRate, finalBeforeRounding)
	} else {
		result.Steps = []domain.PriceExplanationStep{
			{Key: "bom_operation_cost", Label: "BOM+工序成本", Source: baseSource, Value: result.BaseCost, Unit: quoteUnit, Changed: cmd.Overrides.BaseCost != nil},
			{Key: "other_cost_total", Label: "其他成本", Source: pricingRuleTrialOtherCostSource(cmd.Overrides.OtherCosts), Value: result.OtherCostTotal, Unit: quoteUnit, Changed: cmd.Overrides.OtherCosts != nil},
			{Key: "expected_loss_rate", Label: "预期损耗率", Source: pricingRuleTrialLossSource(cmd.Overrides.ExpectedLossRate, yieldMode), Value: roundRatio(expectedLossRate), Unit: "ratio", Changed: lossChanged},
			{Key: "cost_after_yield", Label: "损耗后成本", Source: "formula", Value: result.CostAfterYield, Unit: quoteUnit, Changed: expectedLossRate > 0 && yieldMode != "none"},
			{Key: "profit_method", Label: pricingRuleTrialProfitLabel(profitMethod), Source: pricingRuleTrialOverrideSource(cmd.Overrides.MarginRate), Value: roundBeanListPrice(preTaxPrice), Unit: quoteUnit, Changed: cmd.Overrides.MarginRate != nil},
			{Key: "tax_rate", Label: pricingRuleTrialTaxLabel(taxMode), Source: pricingRuleTrialOverrideSource(cmd.Overrides.TaxRate), Value: roundRatio(taxRate), Unit: "ratio", Changed: cmd.Overrides.TaxRate != nil},
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
		return perKgCost * factor, "product_bom_operation_cost", ""
	}
	if perUnitCost > 0 {
		return perUnitCost, "product_bom_operation_cost", ""
	}
	if perKgCost > 0 {
		return perKgCost, "product_bom_operation_cost", "报价单位无法换算，已按 kg 试算"
	}
	return 0, "product_bom_operation_cost", ""
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
			Name:        "临时录入 BOM+工序成本",
			Amount:      amount,
			Unit:        unit,
			Description: fmt.Sprintf("本次试算临时覆盖 BOM+工序成本 %s", pricingRuleTrialMoneyExpression(amount, unit)),
		}}, amount, 0
	}

	factor := pricingRuleTrialUnitKgFactor(unit, input.UnitConversionJSON)
	usePerKg := input.GreenBeanCostPerKg+input.OperationCostPerKg > 0 && factor > 0
	out := make([]PricingRuleTrialBaseCostDetail, 0, len(rawDetails))
	bomTotal := 0.0
	operationTotal := 0.0
	for i, row := range rawDetails {
		rawAmount := row.Amount
		if rawAmount == 0 {
			if usePerKg && row.AmountPerKg > 0 {
				rawAmount = row.AmountPerKg * factor
			} else if row.AmountPerUnit > 0 {
				rawAmount = row.AmountPerUnit
			} else if row.AmountPerKg > 0 && factor > 0 {
				rawAmount = row.AmountPerKg * factor
			} else if row.AmountPerKg > 0 {
				rawAmount = row.AmountPerKg
			}
		}
		amount := pricingRuleTrialResultAmount(formulaMode, rawAmount)
		if amount == 0 && strings.TrimSpace(row.Name) == "" {
			continue
		}
		row.Amount = amount
		row.UnitCost = pricingRuleTrialResultAmount(formulaMode, row.UnitCost)
		if strings.TrimSpace(row.Unit) == "" {
			row.Unit = unit
		}
		row.Type = strings.TrimSpace(row.Type)
		if row.Type == "" {
			row.Type = "material"
		}
		if strings.TrimSpace(row.TypeLabel) == "" {
			row.TypeLabel = pricingRuleTrialBaseCostTypeLabel(row.Type)
		}
		if strings.TrimSpace(row.Name) == "" {
			row.Name = row.TypeLabel
		}
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
	if baseCost <= 0 {
		return nil, 0, 0
	}

	bomAmount := 0.0
	operationAmount := 0.0
	if input.GreenBeanCostPerKg > 0 && factor > 0 {
		bomAmount = input.GreenBeanCostPerKg * factor
	}
	if input.OperationCostPerKg > 0 && factor > 0 {
		operationAmount = input.OperationCostPerKg * factor
	}
	if bomAmount == 0 && input.BomCostPerUnit > 0 {
		bomAmount = input.BomCostPerUnit
	}
	if operationAmount == 0 && input.OperationCostPerUnit > 0 {
		operationAmount = input.OperationCostPerUnit
	}
	if bomAmount == 0 && operationAmount == 0 {
		bomAmount = baseCost
	}
	if bomAmount > 0 {
		amount := pricingRuleTrialResultAmount(formulaMode, bomAmount)
		out = append(out, PricingRuleTrialBaseCostDetail{
			Key:         "bom:summary",
			Type:        "material",
			TypeLabel:   "物料",
			Name:        "BOM 成本汇总",
			Amount:      amount,
			Unit:        unit,
			Description: fmt.Sprintf("当前商品 BOM 成本汇总 %s", pricingRuleTrialMoneyExpression(amount, unit)),
		})
		bomTotal += amount
	}
	if operationAmount > 0 {
		amount := pricingRuleTrialResultAmount(formulaMode, operationAmount)
		out = append(out, PricingRuleTrialBaseCostDetail{
			Key:         "operation:summary",
			Type:        "operation",
			TypeLabel:   "工序",
			Name:        "工序成本汇总",
			Amount:      amount,
			Unit:        unit,
			Description: fmt.Sprintf("当前商品工序成本汇总 %s", pricingRuleTrialMoneyExpression(amount, unit)),
		})
		operationTotal += amount
	}
	return out, pricingRuleTrialResultAmount(formulaMode, bomTotal), pricingRuleTrialResultAmount(formulaMode, operationTotal)
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
	switch strings.TrimSpace(row.ConsumeUnit) {
	case "ratio_pct":
		return fmt.Sprintf("%s：%s，比例 %s%%，单位成本 %s，金额 %s", row.TypeLabel, row.Name, pricingRuleTrialNumberExpression(row.RatioPct), pricingRuleTrialMoneyExpression(row.UnitCost, unit), pricingRuleTrialMoneyExpression(row.Amount, unit))
	case "g_per_bag", "unit_per_bag", "unit_per_box", "per_kg", "per_unit", "per_quote_unit":
		return fmt.Sprintf("%s：%s，用量 %s，单位成本 %s，金额 %s", row.TypeLabel, row.Name, pricingRuleTrialNumberExpression(row.Quantity), pricingRuleTrialMoneyExpression(row.UnitCost, unit), pricingRuleTrialMoneyExpression(row.Amount, unit))
	default:
		return fmt.Sprintf("%s：%s，金额 %s", row.TypeLabel, row.Name, pricingRuleTrialMoneyExpression(row.Amount, unit))
	}
}

func pricingRuleTrialFormulaExpression(result *PricingRuleTrialResult, formulaMode string, quoteUnit string, profitMethod string, taxMode string, taxRate float64, roundingMode string, expectedLossRate float64, yieldMode string, marginRate float64, profitParameterRate float64, finalBeforeRounding float64) (string, []string) {
	if result == nil {
		return "", nil
	}
	unit := firstNonEmptyString(quoteUnit, result.QuoteUnit)
	baseTerm := fmt.Sprintf("BOM+工序成本 %s", pricingRuleTrialMoneyExpression(result.BaseCost, unit))
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
		currentExpr = fmt.Sprintf("%s * (1 + 档位利润率/加价率 %s", currentExpr, pricingRuleTrialPercentExpression(marginRate))
		if profitParameterRate != 0 {
			currentExpr = fmt.Sprintf("%s + 利润参数 %s", currentExpr, pricingRuleTrialPercentExpression(profitParameterRate))
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
		switch strings.TrimSpace(profitMethod) {
		case "", "gross_margin":
			currentExpr = fmt.Sprintf("%s / (1 - 毛利率 %s)", currentExpr, pricingRuleTrialPercentExpression(marginRate))
			lines = append(lines, fmt.Sprintf("税前价 = %s = %s", currentExpr, pricingRuleTrialMoneyExpression(result.PreTaxPrice, unit)))
		case "markup":
			currentExpr = fmt.Sprintf("%s * (1 + 加价率 %s)", currentExpr, pricingRuleTrialPercentExpression(marginRate))
			lines = append(lines, fmt.Sprintf("税前价 = %s = %s", currentExpr, pricingRuleTrialMoneyExpression(result.PreTaxPrice, unit)))
		case "fixed_add":
			currentExpr = fmt.Sprintf("%s + 固定加价 %s", currentExpr, pricingRuleTrialMoneyExpression(marginRate, unit))
			lines = append(lines, fmt.Sprintf("税前价 = %s = %s", currentExpr, pricingRuleTrialMoneyExpression(result.PreTaxPrice, unit)))
		default:
			lines = append(lines, fmt.Sprintf("税前价 = %s", pricingRuleTrialMoneyExpression(result.PreTaxPrice, unit)))
		}
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

func pricingRuleTrialUnitKgFactor(unit string, conversionJSON string) float64 {
	normalized := strings.ToLower(strings.TrimSpace(unit))
	switch normalized {
	case "", "kg", "公斤", "千克":
		return 1
	case "g", "克":
		return 0.001
	case "lb", "磅":
		return 0.454
	}
	conversionJSON = strings.TrimSpace(conversionJSON)
	if conversionJSON == "" {
		return 0
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(conversionJSON), &raw); err != nil {
		return 0
	}
	row, ok := raw[unit]
	if !ok {
		row, ok = raw[normalized]
	}
	if !ok {
		return 0
	}
	return pricingRuleTrialConversionToKg(row)
}

func pricingRuleTrialConversionToKg(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	case map[string]any:
		for key, raw := range typed {
			n := numberFromAny(raw)
			switch strings.ToLower(strings.TrimSpace(key)) {
			case "kg", "公斤", "千克":
				return n
			case "g", "克":
				return n / 1000
			case "lb", "磅":
				return n * 0.454
			}
		}
	}
	return 0
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

func pricingRuleTrialSupplierSteps(result *PricingRuleTrialResult, cmd PricingRuleTrialCommand, quoteUnit string, baseSource string, preMarkupCosts map[string]float64, postMarkupCosts map[string]float64, expectedLossRate float64, yieldMode string, lossChanged bool, marginRate float64, profitParameterRate float64, taxMode string, taxRate float64, finalBeforeRounding float64) []domain.PriceExplanationStep {
	steps := []domain.PriceExplanationStep{
		{Key: "material_cost", Label: "物料/BOM成本", Source: baseSource, Value: result.BaseCost, Unit: quoteUnit, Changed: cmd.Overrides.BaseCost != nil},
		{Key: "other_cost_total", Label: "生产项目成本", Source: pricingRuleTrialOtherCostSource(cmd.Overrides.OtherCosts), Value: result.OtherCostTotal, Unit: quoteUnit, Changed: cmd.Overrides.OtherCosts != nil},
	}
	steps = appendPricingRuleTrialCostSteps(steps, "other_cost", "生产项目", pricingRuleTrialOtherCostSource(cmd.Overrides.OtherCosts), preMarkupCosts, quoteUnit, cmd.Overrides.OtherCosts != nil)
	steps = append(steps,
		domain.PriceExplanationStep{Key: "expected_loss_rate", Label: "预期损耗率", Source: pricingRuleTrialLossSource(cmd.Overrides.ExpectedLossRate, yieldMode), Value: roundRatio(expectedLossRate), Unit: "ratio", Changed: lossChanged},
		domain.PriceExplanationStep{Key: "cost_after_yield", Label: "成本基数", Source: "formula", Value: result.CostAfterYield, Unit: quoteUnit, Changed: expectedLossRate > 0 && yieldMode != "none"},
		domain.PriceExplanationStep{Key: "tier_markup_rate", Label: "档位利润率/加价率", Source: pricingRuleTrialOverrideSource(cmd.Overrides.MarginRate), Value: roundRatio(marginRate), Unit: "ratio", Changed: cmd.Overrides.MarginRate != nil},
	)
	if profitParameterRate != 0 {
		steps = append(steps, domain.PriceExplanationStep{Key: "profit_parameter_rate", Label: "利润参数", Source: "pricing_rule", Value: roundRatio(profitParameterRate), Unit: "ratio"})
	}
	steps = append(steps,
		domain.PriceExplanationStep{Key: "price_after_markup", Label: "加价后价格", Source: "formula", Value: result.PriceAfterMarkup, Unit: quoteUnit, Changed: true},
		domain.PriceExplanationStep{Key: "post_markup_cost_total", Label: "加价附加成本", Source: pricingRuleTrialPostMarkupCostSource(cmd.Overrides.PostMarkupCosts), Value: result.PostMarkupCostTotal, Unit: quoteUnit, Changed: cmd.Overrides.PostMarkupCosts != nil},
	)
	steps = appendPricingRuleTrialCostSteps(steps, "post_markup_cost", "加价附加", pricingRuleTrialPostMarkupCostSource(cmd.Overrides.PostMarkupCosts), postMarkupCosts, quoteUnit, cmd.Overrides.PostMarkupCosts != nil)
	if strings.TrimSpace(taxMode) != "none" {
		steps = append(steps, domain.PriceExplanationStep{Key: "tax_rate", Label: pricingRuleTrialTaxLabel(taxMode), Source: pricingRuleTrialOverrideSource(cmd.Overrides.TaxRate), Value: roundRatio(taxRate), Unit: "ratio", Changed: cmd.Overrides.TaxRate != nil})
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

func pricingRuleTrialPreTaxPrice(cost float64, marginRate float64, method string) (float64, error) {
	switch strings.TrimSpace(method) {
	case "", "gross_margin":
		if marginRate >= 1 {
			return 0, fmt.Errorf("margin_rate must be < 1 for gross_margin")
		}
		return cost / (1 - marginRate), nil
	case "markup":
		return cost * (1 + marginRate), nil
	case "fixed_add":
		return cost + marginRate, nil
	default:
		return 0, fmt.Errorf("invalid profit_method")
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

func pricingRuleTrialProfitLabel(method string) string {
	switch strings.TrimSpace(method) {
	case "markup":
		return "加价率"
	case "fixed_add":
		return "固定加价"
	default:
		return "毛利率"
	}
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
	if err := validateBeanListFinalPriceSnapshots(normalized); err != nil {
		return nil, err
	}
	if s.repo == nil {
		return nil, fmt.Errorf("repository required")
	}
	return s.repo.PublishBeanList(ctx, normalized)
}

func (s *Service) SaveBeanListDraft(ctx context.Context, cmd PublishBeanListCommand) (*BeanListPublication, error) {
	normalized, err := normalizeBeanListCommand(cmd)
	if err != nil {
		return nil, err
	}
	if s.repo == nil {
		return nil, fmt.Errorf("repository required")
	}
	return s.repo.SaveBeanListDraft(ctx, normalized)
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
						return fmt.Errorf("价格表快照缺少来源价格记录：第%d组第%d个商品第%d档", groupIdx+1, itemIdx+1, tierIdx+1)
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
		if !ok || !beanListFlatPriceRowHasPrice(row) {
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
			return fmt.Errorf("价格表平铺行缺少价格单位到库存单位换算：第%d行", position)
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
		if _, exists := row["customer_reference_snapshot"]; !exists || !hasObjectSnapshot(row["customer_reference_snapshot"]) {
			return fmt.Errorf("价格表平铺行缺少客户引用展示快照：第%d行", position)
		}
		if _, exists := row["manual_adjusted"]; !exists {
			return fmt.Errorf("价格表平铺行缺少人工调整标记：第%d行", position)
		}
	}
	return nil
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
	if query.ProductTypeCategoryID == 0 && query.ClassificationTemplateID > 0 {
		query.ProductTypeCategoryID = query.ClassificationTemplateID
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

var beanListPublicationPDFFilenameUnsafeChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

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
