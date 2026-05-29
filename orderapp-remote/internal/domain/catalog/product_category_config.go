package catalog

import "strings"

type ProductUnitRule struct {
	InventoryUnit  string
	QuoteUnit      string
	OrderUnit      string
	ConversionJSON string
	IntegerUnit    bool
}

// LegacyKindDefaultTypeName keeps the current product_kind data usable while
// product behavior moves to user-maintained product type categories.
func LegacyKindDefaultTypeName(kind string) string {
	switch NormalizeProductKind(kind) {
	case ProductKindGreenBean:
		return "生豆"
	case ProductKindDripBag:
		return "挂耳"
	case ProductKindInstantCoffee:
		return "速溶咖啡"
	default:
		return "熟豆"
	}
}

func ProductCategoryRoleLabel(level int) string {
	switch level {
	case 1:
		return "产品类型"
	case 2:
		return "产品子类型"
	default:
		return "产品分类"
	}
}

func NormalizeProductUnitRule(rule ProductUnitRule) ProductUnitRule {
	rule.InventoryUnit = normalizeProductUnit(rule.InventoryUnit, "kg")
	rule.QuoteUnit = normalizeProductUnit(rule.QuoteUnit, rule.InventoryUnit)
	rule.OrderUnit = normalizeProductUnit(rule.OrderUnit, rule.QuoteUnit)
	rule.ConversionJSON = strings.TrimSpace(rule.ConversionJSON)
	if rule.ConversionJSON == "" {
		rule.ConversionJSON = "{}"
	}
	return rule
}

func normalizeProductUnit(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	fallback = strings.TrimSpace(fallback)
	if fallback != "" {
		return fallback
	}
	return "kg"
}
