package catalog

import "strings"

const (
	ProductKindRoasted       = "roasted"
	ProductKindRoastedBean   = ProductKindRoasted
	ProductKindGreenBean     = "green_bean"
	ProductKindDripBag       = "drip_bag"
	ProductKindInstantCoffee = "instant_coffee"
)

func NormalizeProductKind(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ProductKindDripBag, "drip", "挂耳":
		return ProductKindDripBag
	case ProductKindInstantCoffee, "instant", "instant_coffee_powder", "速溶", "速溶咖啡":
		return ProductKindInstantCoffee
	case ProductKindGreenBean, "green", "raw", "raw_bean", "生豆":
		return ProductKindGreenBean
	case ProductKindRoasted, "roasted_bean", "熟豆":
		return ProductKindRoasted
	default:
		return ProductKindRoasted
	}
}

func ProductKindLabel(value string) string {
	switch NormalizeProductKind(value) {
	case ProductKindDripBag:
		return "挂耳"
	case ProductKindInstantCoffee:
		return "速溶咖啡"
	case ProductKindGreenBean:
		return "生豆"
	default:
		return "熟豆"
	}
}

func ProductKindRequiresRoast(value string) bool {
	switch NormalizeProductKind(value) {
	case ProductKindRoasted, ProductKindDripBag:
		return true
	default:
		return false
	}
}
