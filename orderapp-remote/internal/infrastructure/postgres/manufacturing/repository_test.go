package manufacturing

import (
	"os"
	"strings"
	"testing"
)

func TestManufacturingRepositoryKeepsOperationCostAndWorkstationApplicability(t *testing.T) {
	src, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, want := range []string{
		"standard_operation_cost",
		"manufacturing_workstation_operations",
		"attachWorkstationOperations",
		"applicable_operation_ids",
		"sc.id=pro.standard_cost_capacity_id",
		"StandardCostSummary",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("manufacturing repository missing operation-cost/workstation-applicability marker %q", want)
		}
	}
}
