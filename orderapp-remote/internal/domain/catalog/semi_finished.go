package catalog

import "strings"

func IsWeightInventoryUnit(unit string) bool {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "kg", "g", "千克", "克":
		return true
	default:
		return false
	}
}

type SemiFinishedValidationInput struct {
	IsParent      bool
	InventoryUnit string
	UnitTemplateID int64
}

func ValidateSemiFinishedProduct(input SemiFinishedValidationInput) error {
	if !input.IsParent {
		return ErrSemiFinishedMustBeParent
	}
	if !IsWeightInventoryUnit(input.InventoryUnit) {
		return ErrSemiFinishedRequiresWeightUnit
	}
	if input.UnitTemplateID <= 0 {
		return ErrSemiFinishedRequiresUnitTemplate
	}
	return nil
}
