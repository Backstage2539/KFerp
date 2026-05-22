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

var ErrBeanListPublicationNotFound = errors.New("bean list publication not found")

type CalculateRequest struct {
	Products []domain.ProductInput `json:"products"`
}

type PriceExplanationCommand struct {
	Product   domain.ProductInput              `json:"product"`
	TierLabel string                           `json:"tier_label"`
	Overrides domain.PriceExplanationOverrides `json:"overrides,omitempty"`
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
	ID                       int64          `json:"id"`
	ListType                 string         `json:"list_type"`
	Version                  string         `json:"version"`
	Status                   string         `json:"status"`
	OwnerType                string         `json:"owner_type"`
	OwnerKey                 string         `json:"owner_key,omitempty"`
	PriceSourcePublicationID int64          `json:"price_source_publication_id,omitempty"`
	StyleSourcePublicationID int64          `json:"style_source_publication_id,omitempty"`
	SourceVersion            string         `json:"source_version,omitempty"`
	Config                   map[string]any `json:"config"`
	Content                  map[string]any `json:"content"`
	Changelog                string         `json:"changelog"`
	PublishedAt              string         `json:"published_at,omitempty"`
	WithdrawnAt              string         `json:"withdrawn_at,omitempty"`
	CreatedAt                string         `json:"created_at,omitempty"`
}

type BeanListPublicationQuery struct {
	ListType   string `json:"list_type"`
	Scope      string `json:"scope,omitempty"`
	CustomerID int64  `json:"customer_id,omitempty"`
	OwnerType  string `json:"owner_type,omitempty"`
	OwnerKey   string `json:"owner_key,omitempty"`
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

type PublishBeanListCommand struct {
	ListType                 string         `json:"list_type"`
	Version                  string         `json:"version"`
	Scope                    string         `json:"scope,omitempty"`
	CustomerID               int64          `json:"customer_id,omitempty"`
	OwnerType                string         `json:"owner_type,omitempty"`
	OwnerKey                 string         `json:"owner_key,omitempty"`
	PriceSourcePublicationID int64          `json:"price_source_publication_id,omitempty"`
	StyleSourcePublicationID int64          `json:"style_source_publication_id,omitempty"`
	SourceVersion            string         `json:"source_version,omitempty"`
	Config                   map[string]any `json:"config"`
	Content                  map[string]any `json:"content"`
	Changelog                string         `json:"changelog"`
	Actor                    string         `json:"actor,omitempty"`
}

type WithdrawBeanListCommand struct {
	ID        int64  `json:"id"`
	Scope     string `json:"scope,omitempty"`
	OwnerType string `json:"owner_type,omitempty"`
	OwnerKey  string `json:"owner_key,omitempty"`
	Actor     string `json:"actor,omitempty"`
}

type Repository interface {
	LoadParameters(ctx context.Context) (domain.Parameters, error)
	LoadProductInputs(ctx context.Context, params domain.Parameters) ([]domain.ProductInput, error)
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
	cmd.Version = strings.TrimSpace(cmd.Version)
	cmd.Changelog = strings.TrimSpace(cmd.Changelog)
	cmd.SourceVersion = strings.TrimSpace(cmd.SourceVersion)
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
	return cmd, nil
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

func normalizeBeanListPublicationQuery(query BeanListPublicationQuery) (BeanListPublicationQuery, error) {
	listType, err := normalizeBeanListType(query.ListType)
	if err != nil {
		return BeanListPublicationQuery{}, err
	}
	ownerType, ownerKey, err := normalizeBeanListOwner(query.OwnerType, query.OwnerKey)
	if err != nil {
		return BeanListPublicationQuery{}, err
	}
	query.ListType = listType
	query.OwnerType = ownerType
	query.OwnerKey = ownerKey
	return query, nil
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
	return fmt.Sprintf("bean-list-preview-style-v3:%d:%s", row.ID, version)
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
