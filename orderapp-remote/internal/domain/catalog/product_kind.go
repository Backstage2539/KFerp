package catalog

import "strings"

const (
	ProductKindRoasted   = "roasted"
	ProductKindGreenBean = "green_bean"
)

func NormalizeProductKind(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ProductKindGreenBean, "green", "raw", "raw_bean", "生豆":
		return ProductKindGreenBean
	case ProductKindRoasted, "熟豆":
		return ProductKindRoasted
	default:
		return ProductKindRoasted
	}
}

func ProductKindLabel(value string) string {
	if NormalizeProductKind(value) == ProductKindGreenBean {
		return "生豆"
	}
	return "熟豆"
}
