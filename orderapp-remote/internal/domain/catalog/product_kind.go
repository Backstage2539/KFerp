package catalog

import "strings"

const (
	ProductKindRoastedBean = "roasted_bean"
	ProductKindDripBag     = "drip_bag"
)

func NormalizeProductKind(kind string) string {
	kind = strings.TrimSpace(kind)
	if kind == ProductKindDripBag {
		return ProductKindDripBag
	}
	return ProductKindRoastedBean
}
