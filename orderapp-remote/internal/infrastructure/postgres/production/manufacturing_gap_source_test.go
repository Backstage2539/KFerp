package production

import (
	"os"
	"strings"
	"testing"
)

func TestManufacturingGapSchemaAndReservationGuards(t *testing.T) {
	files := []string{
		"schema.go",
		"material_consumption.go",
		"work_order.go",
		"material_plan.go",
		"quality.go",
	}
	var combined strings.Builder
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		combined.Write(b)
		combined.WriteByte('\n')
	}
	src := combined.String()
	for _, want := range []string{
		"work_order_material_reservations",
		"reserved_g",
		"consumed_g",
		"createMaterialReservationsForRunningItemTx",
		"releaseMaterialReservationsForRunningItemTx",
		"updateMaterialReservationConsumedTx",
		"availableG - reservedG",
		"quality_inspections",
		"MaterialPlan(ctx context.Context",
		"purchaseSuggestionG",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("manufacturing gap source missing %q", want)
		}
	}
}
