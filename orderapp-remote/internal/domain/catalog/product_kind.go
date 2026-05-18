package catalog

import "strings"

const (
	ProductKindRoastedBean = "roasted_bean"
	ProductKindGreenBean   = "green_bean"
	ProductKindDripBag     = "drip_bag"
)

func NormalizeProductKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case ProductKindDripBag, "drip", "挂耳":
		return ProductKindDripBag
	case ProductKindGreenBean, "green", "raw", "raw_bean", "生豆":
		return ProductKindGreenBean
	case ProductKindRoastedBean, "roasted", "熟豆":
		return ProductKindRoastedBean
	default:
		return ProductKindRoastedBean
	}
}

func ProductKindLabel(kind string) string {
	switch NormalizeProductKind(kind) {
	case ProductKindDripBag:
		return "挂耳"
	case ProductKindGreenBean:
		return "生豆"
	default:
		return "熟豆"
	}
}
