package production

import "testing"

func TestManufacturingNeedCanonicalQuantitiesPreservesCountedProductUnits(t *testing.T) {
	counted := materialConsumptionNeed{
		MaterialID: 7, ComponentType: "finished_product", ComponentProductID: 7,
		ConsumeUnit: "unit_per_bag", Qty: 3, DeductG: 3,
	}
	gotG, gotUnits := manufacturingNeedCanonicalQuantities(counted)
	if gotG != 0 || gotUnits != 3 {
		t.Fatalf("counted product quantity=(%dg,%d units), want (0g,3 units)", gotG, gotUnits)
	}

	counted.ComponentSpecG = 227
	gotG, gotUnits = manufacturingNeedCanonicalQuantities(counted)
	if gotG != 681 || gotUnits != 3 {
		t.Fatalf("counted concrete SKU quantity=(%dg,%d units), want (681g,3 units)", gotG, gotUnits)
	}
}
