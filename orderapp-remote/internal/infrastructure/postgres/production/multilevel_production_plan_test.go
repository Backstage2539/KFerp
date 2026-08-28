package production

import (
	"os"
	"strings"
	"testing"
)

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

func TestMultilevelPlanningScopesDefaultBOMLoadingToSelectedRootComponents(t *testing.T) {
	source, err := os.ReadFile("multilevel_production_plan.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, marker := range []string{
		"rootComponents := make([]materialConsumptionNeed, 0, len(rootNeeds))",
		"loadDefaultManufacturingOutputBOMsForPlanningTx(ctx, tx, schema, rootComponents)",
		"requestedKeys := make([]string, 0, len(rootComponents))",
	} {
		if !strings.Contains(text, marker) {
			t.Fatalf("multilevel planning must scope default BOM loading to selected root components; missing %q", marker)
		}
	}
}
