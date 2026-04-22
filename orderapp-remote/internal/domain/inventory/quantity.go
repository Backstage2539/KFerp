package inventory

import "fmt"

// Quantity is inventory tracked by whole units plus loose grams for a product
// spec size.
type Quantity struct {
	Units  int64
	LooseG int64
}

// TotalGrams converts a unit/loose quantity into grams.
func TotalGrams(specG int64, q Quantity) (int64, error) {
	if specG <= 0 {
		return 0, fmt.Errorf("invalid spec_g")
	}
	if q.Units < 0 || q.LooseG < 0 {
		return 0, fmt.Errorf("negative inventory")
	}
	return q.Units*specG + q.LooseG, nil
}

// Normalize carries loose grams into whole units.
func Normalize(specG int64, q Quantity) (Quantity, error) {
	total, err := TotalGrams(specG, q)
	if err != nil {
		return Quantity{}, err
	}
	return Quantity{Units: total / specG, LooseG: total % specG}, nil
}

// Deduct deducts needG grams from inventory and returns the remaining
// quantity, deducted grams, and any gap. Insufficient inventory is not mutated.
func Deduct(specG int64, q Quantity, needG int64) (remain Quantity, deductedG int64, gapG int64, err error) {
	if needG < 0 {
		return Quantity{}, 0, 0, fmt.Errorf("invalid need_g")
	}
	total, err := TotalGrams(specG, q)
	if err != nil {
		return Quantity{}, 0, 0, err
	}
	if needG > total {
		return q, 0, needG - total, nil
	}
	if needG == total {
		return Quantity{Units: 0, LooseG: 0}, needG, 0, nil
	}
	remainTotal := total - needG
	remain, err = Normalize(specG, Quantity{Units: 0, LooseG: remainTotal})
	if err != nil {
		return Quantity{}, 0, 0, err
	}
	return remain, needG, 0, nil
}
