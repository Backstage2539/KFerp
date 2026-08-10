package catalog

import "testing"

func TestIsWeightInventoryUnit(t *testing.T) {
	cases := map[string]bool{
		"kg": true, "g": true, "千克": true, "克": true,
		"袋": false, "盒": false, "": false, "unit": false,
	}
	for unit, want := range cases {
		if got := IsWeightInventoryUnit(unit); got != want {
			t.Fatalf("IsWeightInventoryUnit(%q) = %v, want %v", unit, got, want)
		}
	}
}

func TestValidateSemiFinishedProduct(t *testing.T) {
	if err := ValidateSemiFinishedProduct(SemiFinishedValidationInput{IsParent: false, InventoryUnit: "kg", UnitTemplateID: 1}); err != ErrSemiFinishedMustBeParent {
		t.Fatalf("non-parent should fail, got %v", err)
	}
	if err := ValidateSemiFinishedProduct(SemiFinishedValidationInput{IsParent: true, InventoryUnit: "袋", UnitTemplateID: 1}); err != ErrSemiFinishedRequiresWeightUnit {
		t.Fatalf("non-weight unit should fail, got %v", err)
	}
	if err := ValidateSemiFinishedProduct(SemiFinishedValidationInput{IsParent: true, InventoryUnit: "kg", UnitTemplateID: 0}); err != ErrSemiFinishedRequiresUnitTemplate {
		t.Fatalf("missing unit template should fail, got %v", err)
	}
	if err := ValidateSemiFinishedProduct(SemiFinishedValidationInput{IsParent: true, InventoryUnit: "g", UnitTemplateID: 5}); err != nil {
		t.Fatalf("valid semi-finished should pass, got %v", err)
	}
}
