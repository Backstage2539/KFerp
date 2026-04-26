package appmain

import inventorydomain "orderapp/internal/domain/inventory"

// InvQty is kept as a compatibility alias while inventory rules live in the
// domain layer.
type InvQty = inventorydomain.Quantity

func invTotalG(specG int64, q InvQty) (int64, error) {
	return inventorydomain.TotalGrams(specG, q)
}

func invNormalize(specG int64, q InvQty) (InvQty, error) {
	return inventorydomain.Normalize(specG, q)
}

// invDeduct deducts needG grams from inventory and returns (remain, deductedG, gapG).
// DEV-046 rule: when inventory is insufficient, deduction is forbidden.
// - Allows using loose grams to fulfill unit-based needs only when enough inventory exists.
// - Never returns negative remain.
func invDeduct(specG int64, q InvQty, needG int64) (remain InvQty, deductedG int64, gapG int64, err error) {
	return inventorydomain.Deduct(specG, q, needG)
}
