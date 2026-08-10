package bom

const (
	BomKindProduct       = "product"
	BomKindSpecPackaging = "spec_packaging"
)

func IsValidBomKind(kind string) bool {
	switch kind {
	case BomKindProduct, BomKindSpecPackaging:
		return true
	default:
		return false
	}
}

func IsPackagingBomKind(kind string) bool {
	return kind == BomKindSpecPackaging
}

func NormalizeBomKind(kind string) string {
	if kind == "" {
		return BomKindProduct
	}
	return kind
}
