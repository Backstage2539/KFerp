package catalog

const (
	ProductKindRoastedBean = "roasted_bean"
	ProductKindDripBag     = "drip_bag"
)

func NormalizeProductKind(kind string) string {
	if kind == ProductKindDripBag {
		return ProductKindDripBag
	}
	return ProductKindRoastedBean
}
