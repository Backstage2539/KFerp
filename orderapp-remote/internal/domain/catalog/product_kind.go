package catalog

import "strings"

const (
	ProductKindRoasted     = "roasted"
	ProductKindRoastedBean = ProductKindRoasted
	ProductKindGreenBean   = "green_bean"
	ProductKindDripBag     = "drip_bag"
)

func NormalizeProductKind(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ProductKindDripBag, "drip", "挂耳":
		return ProductKindDripBag
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
	case ProductKindGreenBean:
		return "生豆"
	default:
		return "熟豆"
	}
}
