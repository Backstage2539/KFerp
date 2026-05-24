package catalog

import "strings"

type ProductRuleConfig struct {
	GradientTemplateID  int64
	OperationTemplateID int64
	PriceListRuleJSON   string
	UnitRule            ProductUnitRule
}

type ProductRuleResolutionInput struct {
	SystemFallback        ProductRuleConfig
	ProductTypeDefault    *ProductRuleConfig
	ProductSubtypeDefault *ProductRuleConfig
	CustomerTemplate      *ProductRuleConfig
	CustomerOverride      *ProductRuleConfig
}

func ResolveProductRuleConfig(input ProductRuleResolutionInput) ProductRuleConfig {
	out := input.SystemFallback
	out.PriceListRuleJSON = normalizeProductRuleJSON(out.PriceListRuleJSON)
	out.UnitRule = NormalizeProductUnitRule(out.UnitRule)

	for _, cfg := range []*ProductRuleConfig{
		input.ProductTypeDefault,
		input.ProductSubtypeDefault,
		input.CustomerTemplate,
		input.CustomerOverride,
	} {
		out = mergeProductRuleConfig(out, cfg)
	}
	return out
}

func mergeProductRuleConfig(base ProductRuleConfig, override *ProductRuleConfig) ProductRuleConfig {
	if override == nil {
		return base
	}
	if override.GradientTemplateID > 0 {
		base.GradientTemplateID = override.GradientTemplateID
	}
	if override.OperationTemplateID > 0 {
		base.OperationTemplateID = override.OperationTemplateID
	}
	if hasProductRuleJSON(override.PriceListRuleJSON) {
		base.PriceListRuleJSON = normalizeProductRuleJSON(override.PriceListRuleJSON)
	}
	if hasProductUnitRule(override.UnitRule) {
		base.UnitRule = NormalizeProductUnitRule(override.UnitRule)
	}
	return base
}

func normalizeProductRuleJSON(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "{}"
	}
	return value
}

func hasProductRuleJSON(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && value != "{}" && value != "null"
}

func hasProductUnitRule(rule ProductUnitRule) bool {
	return strings.TrimSpace(rule.InventoryUnit) != "" ||
		strings.TrimSpace(rule.QuoteUnit) != "" ||
		strings.TrimSpace(rule.OrderUnit) != "" ||
		strings.TrimSpace(rule.ConversionJSON) != "" ||
		rule.IntegerUnit
}
