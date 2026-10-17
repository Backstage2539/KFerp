package sales

import (
	"context"
	"encoding/json"
	"fmt"
	conversion "orderapp/internal/domain/specconversion"
)

type SpecConversionSelection struct {
	PricingMethod   string `json:"pricing_method,omitempty"`
	QuoteRevision   string `json:"quote_revision,omitempty"`
	GroupID         int64  `json:"group_id"`
	Revision        int64  `json:"revision"`
	SourceSpecID    int64  `json:"source_bom_spec_id"`
	SourceVariantID int64  `json:"source_bom_variant_id"`
	Reprice         bool   `json:"reprice,omitempty"`
}
type SpecConversionSpec struct {
	IsDefault     bool   `json:"is_default"`
	SortOrder     int    `json:"sort_order"`
	ProductID     int64  `json:"product_id"`
	ProductName   string `json:"product_name"`
	SpecID        int64  `json:"bom_spec_id"`
	VariantID     int64  `json:"bom_variant_id"`
	Name          string `json:"spec_name"`
	InventoryUnit string `json:"inventory_unit"`
	TemplateID    int64  `json:"template_id"`
	TemplateKey   string `json:"template_key"`
}

func (s SpecConversionSpec) Identity() conversion.Identity {
	return conversion.Identity{SpecID: s.SpecID, TemplateID: s.TemplateID, TemplateKey: s.TemplateKey, InventoryUnit: s.InventoryUnit}
}

type SpecConversionUnit struct {
	Code string `json:"code"`
	Name string `json:"name"`
}
type SpecConversionSourceTier struct {
	MinQty        *float64 `json:"min_qty"`
	MaxQty        *float64 `json:"max_qty"`
	QuantityBasis string   `json:"quantity_basis"`
	SpecConversionSpec
	PriceRowKey string `json:"price_row_key"`
	TierLabel   string `json:"tier_label"`
	UnitPrice   string `json:"unit_price"`
}
type SpecConversionCatalog struct {
	SourceTiers []SpecConversionSourceTier `json:"source_tiers,omitempty"`
	Groups      []conversion.Group         `json:"groups"`
	Specs       []SpecConversionSpec       `json:"specs"`
	Units       []SpecConversionUnit       `json:"units,omitempty"`
}
type SaveSpecConversionGroupCommand struct {
	Group conversion.Group `json:"group"`
	Actor string           `json:"-"`
}
type SpecConversionQuoteRequest struct {
	SpecConversionSelection
	CustomerID      int64  `json:"customer_id"`
	ProductID       int64  `json:"product_id"`
	PublicationID   int64  `json:"publication_id"`
	PriceRowKey     string `json:"price_row_key"`
	TargetSpecID    int64  `json:"target_bom_spec_id"`
	TargetVariantID int64  `json:"target_bom_variant_id"`
	Quantity        int64  `json:"quantity"`
}
type SpecConversionQuote struct {
	SpecConversionQuoteRequest
	PricingRuleName string          `json:"pricing_rule_name,omitempty"`
	Calculation     json.RawMessage `json:"calculation,omitempty"`
	GroupName       string          `json:"group_name"`
	BaseUnit        string          `json:"base_unit"`
	SourceSpecName  string          `json:"source_spec_name"`
	TargetSpecName  string          `json:"target_spec_name"`
	SourceUnit      string          `json:"source_unit"`
	TargetUnit      string          `json:"target_unit"`
	SourceQuantity  string          `json:"source_quantity"`
	TargetQuantity  string          `json:"target_quantity"`
	SourceUnitPrice string          `json:"source_unit_price"`
	UnitPrice       string          `json:"unit_price"`
	LineTotal       string          `json:"line_total"`
	Rounding        string          `json:"rounding"`
	VersionNo       string          `json:"version_no"`
	TierLabel       string          `json:"tier_label"`
	PriceSourceJSON string          `json:"price_source_json,omitempty"`
}
type SpecConversionRepository interface {
	SpecConversionSettings(context.Context) (SpecConversionCatalog, error)
	SaveSpecConversionGroup(context.Context, SaveSpecConversionGroupCommand) (conversion.Group, error)
	OrderSpecConversions(context.Context, int64, int64, int64) (SpecConversionCatalog, error)
	QuoteSpecConversion(context.Context, SpecConversionQuoteRequest) (SpecConversionQuote, error)
}

func (s *Service) conversionRepository() (SpecConversionRepository, error) {
	r, ok := s.repo.(SpecConversionRepository)
	if !ok {
		return nil, fmt.Errorf("规格换算服务不可用")
	}
	return r, nil
}
func (s *Service) SpecConversionSettings(ctx context.Context) (SpecConversionCatalog, error) {
	r, e := s.conversionRepository()
	if e != nil {
		return SpecConversionCatalog{}, e
	}
	return r.SpecConversionSettings(ctx)
}
func (s *Service) SaveSpecConversionGroup(ctx context.Context, c SaveSpecConversionGroupCommand) (conversion.Group, error) {
	if e := c.Group.Validate(); e != nil {
		return conversion.Group{}, e
	}
	r, e := s.conversionRepository()
	if e != nil {
		return conversion.Group{}, e
	}
	return r.SaveSpecConversionGroup(ctx, c)
}
func (s *Service) OrderSpecConversions(ctx context.Context, customerID, productID, publicationID int64) (SpecConversionCatalog, error) {
	r, e := s.conversionRepository()
	if e != nil {
		return SpecConversionCatalog{}, e
	}
	return r.OrderSpecConversions(ctx, customerID, productID, publicationID)
}
func (s *Service) QuoteSpecConversion(ctx context.Context, c SpecConversionQuoteRequest) (SpecConversionQuote, error) {
	r, e := s.conversionRepository()
	if e != nil {
		return SpecConversionQuote{}, e
	}
	return r.QuoteSpecConversion(ctx, c)
}
func SpecConversionFromSnapshot(raw string) *SpecConversionSelection {
	var s struct {
		Conversion *SpecConversionQuote `json:"spec_conversion"`
	}
	if json.Unmarshal([]byte(raw), &s) != nil || s.Conversion == nil {
		return nil
	}
	copy := s.Conversion.SpecConversionSelection
	copy.Reprice = false
	return &copy
}
