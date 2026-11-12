package catalog

import (
	"context"
	"strings"
)

type SpecPricingGroupItem struct {
	SpecKey       string `json:"spec_key"`
	InventoryUnit string `json:"inventory_unit"`
	PricingRuleID int64  `json:"pricing_rule_id"`
	SortOrder     int    `json:"sort_order"`
}
type SpecPricingGroup struct {
	ID                    int64                  `json:"id"`
	Name                  string                 `json:"name"`
	SpecTemplateID        int64                  `json:"spec_template_id"`
	SpecTemplateVersionID int64                  `json:"spec_template_version_id"`
	Revision              int64                  `json:"revision"`
	Active                bool                   `json:"active"`
	Items                 []SpecPricingGroupItem `json:"items"`
	Actor                 string                 `json:"-"`
}
type SpecPricingOptionVariant struct {
	SpecKey       string `json:"spec_key"`
	Name          string `json:"name"`
	InventoryUnit string `json:"inventory_unit"`
}
type SpecPricingOptionVersion struct {
	ID        int64                      `json:"id"`
	VersionNo string                     `json:"version_no"`
	Variants  []SpecPricingOptionVariant `json:"variants"`
}
type SpecPricingOptionTemplate struct {
	ID       int64                      `json:"id"`
	Name     string                     `json:"name"`
	Versions []SpecPricingOptionVersion `json:"versions"`
}
type SpecPricingOptionRule struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}
type SpecPricingOptions struct {
	Templates    []SpecPricingOptionTemplate `json:"templates"`
	PricingRules []SpecPricingOptionRule     `json:"pricing_rules"`
}
type SpecPricingGroupRepository interface {
	ListSpecPricingGroups(context.Context) ([]SpecPricingGroup, error)
	SaveSpecPricingGroup(context.Context, SpecPricingGroup) (SpecPricingGroup, error)
	SpecPricingGroupOptions(context.Context) (SpecPricingOptions, error)
}

func validateSpecPricingGroup(g SpecPricingGroup) error {
	if strings.TrimSpace(g.Name) == "" || g.SpecTemplateID <= 0 || g.SpecTemplateVersionID <= 0 || len(g.Items) == 0 {
		return ValidationError{Message: "请填写计价组名称、已发布规格模板并选择规格"}
	}
	seen := map[string]bool{}
	for _, item := range g.Items {
		key := strings.ToLower(strings.TrimSpace(item.SpecKey)) + "\x00" + strings.ToLower(strings.TrimSpace(item.InventoryUnit))
		if strings.TrimSpace(item.SpecKey) == "" || strings.TrimSpace(item.InventoryUnit) == "" || item.PricingRuleID <= 0 || seen[key] {
			return ValidationError{Message: "规格重复或缺少规格、单位、价格计算模板"}
		}
		seen[key] = true
	}
	return nil
}
func (s *Service) ListSpecPricingGroups(ctx context.Context) ([]SpecPricingGroup, error) {
	r, ok := s.repo.(SpecPricingGroupRepository)
	if !ok {
		return nil, ValidationError{Message: "规格计价组不可用"}
	}
	return r.ListSpecPricingGroups(ctx)
}
func (s *Service) SaveSpecPricingGroup(ctx context.Context, g SpecPricingGroup) (SpecPricingGroup, error) {
	if err := validateSpecPricingGroup(g); err != nil {
		return SpecPricingGroup{}, err
	}
	g.Name = strings.TrimSpace(g.Name)
	r, ok := s.repo.(SpecPricingGroupRepository)
	if !ok {
		return SpecPricingGroup{}, ValidationError{Message: "规格计价组不可用"}
	}
	return r.SaveSpecPricingGroup(ctx, g)
}
func (s *Service) SpecPricingGroupOptions(ctx context.Context) (SpecPricingOptions, error) {
	r, ok := s.repo.(SpecPricingGroupRepository)
	if !ok {
		return SpecPricingOptions{}, ValidationError{Message: "规格计价组选项不可用"}
	}
	return r.SpecPricingGroupOptions(ctx)
}
