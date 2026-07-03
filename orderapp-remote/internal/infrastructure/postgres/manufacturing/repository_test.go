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
		"0::bigint",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("manufacturing repository missing operation-cost/workstation-applicability marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"sc.id=pro.standard_cost_capacity_id",
		"标准成本默认产能",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("manufacturing repository should not read route standard cost default capacity; found %q", forbidden)
		}
	}
}
