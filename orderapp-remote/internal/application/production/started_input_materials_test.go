package production

import (
	"math"
	"testing"
)

func TestValidateStartedInputMaterials(t *testing.T) {
	expected := []OperationMaterialUse{
		{Key: "material:34:0:0", MaterialID: 34, Unit: "kg", Warehouse: "wip", Qty: 3},
		{Key: "material:95:0:0", MaterialID: 95, Unit: "kg", Warehouse: "wip", Qty: 7},
	}
	actual := append([]OperationMaterialUse(nil), expected...)
	if err := ValidateStartedInputMaterials(expected, actual, ""); err != nil {
		t.Fatal(err)
	}
	actual[0].Qty = 3.1
	if err := ValidateStartedInputMaterials(expected, actual, ""); err == nil {
		t.Fatal("changed material quantity requires a reason")
	}
	if err := ValidateStartedInputMaterials(expected, actual, "实际称重"); err != nil {
		t.Fatal(err)
	}
	actual[1].MaterialID = 34
	if err := ValidateStartedInputMaterials(expected, actual, "实际称重"); err == nil {
		t.Fatal("material identity substitution must be rejected")
	}
	actual = append([]OperationMaterialUse(nil), expected...)
	actual[1].Key = actual[0].Key
	if err := ValidateStartedInputMaterials(expected, actual, ""); err == nil {
		t.Fatal("duplicate material must be rejected")
	}
	if err := ValidateStartedInputMaterials(expected, actual[:1], ""); err == nil {
		t.Fatal("missing material must be rejected")
	}
}

func TestValidateStartedInputMaterialsSmallestQuantityChangeRequiresReason(t *testing.T) {
	expected := []OperationMaterialUse{{Key: "material:34:0:0", MaterialID: 34, Unit: "kg", Warehouse: "wip", Qty: 1}}
	actual := append([]OperationMaterialUse(nil), expected...)
	actual[0].Qty = 1.00000001
	if err := ValidateStartedInputMaterials(expected, actual, ""); err == nil {
		t.Fatal("a one-ledger-unit change requires a reason")
	}
}

func TestStartedInputRejectsUnrepresentableStockAndMismatchedTotal(t *testing.T) {
	rows := []OperationMaterialUse{{Key: "material:1:0:0", MaterialID: 1, Unit: "kg", Qty: 0.0001}}
	if err := ValidateStartedInputMaterials(rows, rows, ""); err == nil {
		t.Fatal("fractional stock gram was silently rounded")
	}
	if err := ValidateStartedInputTotals([]OperationMaterialUse{{Unit: "kg", Qty: 1.5}, {Unit: "g", Qty: 3500}}, 5, "kg", true); err != nil {
		t.Fatal(err)
	}
	if err := ValidateStartedInputTotals([]OperationMaterialUse{{Unit: "kg", Qty: 1.5}, {Unit: "g", Qty: 3000}}, 5, "kg", true); err == nil {
		t.Fatal("ratio input does not match actual ingredients")
	}
	if err := ValidateStartedInputTotals([]OperationMaterialUse{{Unit: "kg", Qty: 0}}, 5, "kg", false); err == nil {
		t.Fatal("empty physical input accepted")
	}
}

func TestBOMStartWeightUnitsAndRatioRemainder(t *testing.T) {
	rows := []OperationMaterialUse{{Key: "a", Name: "A", Unit: "kg", RatioPct: 50}, {Key: "b", Name: "B", Unit: "kg", RatioPct: 50}}
	out, err := ResolveBOMStartMaterialUses(rows, 1, "g")
	if err != nil {
		t.Fatal(err)
	}
	if int64(math.Round(out[0].Qty*1000))+int64(math.Round(out[1].Qty*1000)) != 1 {
		t.Fatalf("one gram distributed more than once: %+v", out)
	}
	if err = ValidateStartedInputTotals(out, 1, "g", true); err != nil {
		t.Fatal(err)
	}
	lbs, err := ResolveBOMStartMaterialUses(rows, 1, "lb")
	if err != nil {
		t.Fatal(err)
	}
	if err = ValidateStartedInputTotals(lbs, 1, "lb", true); err != nil {
		t.Fatal(err)
	}
	if int64(math.Round(lbs[0].Qty*1000))+int64(math.Round(lbs[1].Qty*1000)) != 454 {
		t.Fatalf("one pound should consume 454g: %+v", lbs)
	}
}
