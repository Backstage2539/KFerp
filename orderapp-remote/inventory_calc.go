package main

import "fmt"

// Inventory is tracked by (units + loose grams) at a given spec size (g per unit).
// Example: 5 units + 10g at spec=454g.

type InvQty struct {
	Units  int64
	LooseG int64
}

func invTotalG(specG int64, q InvQty) (int64, error) {
	if specG <= 0 {
		return 0, fmt.Errorf("invalid spec_g")
	}
	if q.Units < 0 || q.LooseG < 0 {
		return 0, fmt.Errorf("negative inventory")
	}
	return q.Units*specG + q.LooseG, nil
}

func invNormalize(specG int64, q InvQty) (InvQty, error) {
	total, err := invTotalG(specG, q)
	if err != nil {
		return InvQty{}, err
	}
	return InvQty{Units: total / specG, LooseG: total % specG}, nil
}

// invDeduct deducts needG grams from inventory and returns (remain, deductedG, gapG).
// DEV-046 rule: when inventory is insufficient, deduction is forbidden.
// - Allows using loose grams to fulfill unit-based needs only when enough inventory exists.
// - Never returns negative remain.
func invDeduct(specG int64, q InvQty, needG int64) (remain InvQty, deductedG int64, gapG int64, err error) {
	if needG < 0 {
		return InvQty{}, 0, 0, fmt.Errorf("invalid need_g")
	}
	total, err := invTotalG(specG, q)
	if err != nil {
		return InvQty{}, 0, 0, err
	}
	if needG > total {
		// insufficient: prohibit deduction and keep inventory unchanged
		return q, 0, needG - total, nil
	}
	if needG == total {
		return InvQty{Units: 0, LooseG: 0}, needG, 0, nil
	}
	// have enough
	remainTotal := total - needG
	remain, err = invNormalize(specG, InvQty{Units: 0, LooseG: remainTotal})
	if err != nil {
		return InvQty{}, 0, 0, err
	}
	return remain, needG, 0, nil
}
