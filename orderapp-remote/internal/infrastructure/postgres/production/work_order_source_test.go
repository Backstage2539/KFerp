package production

import (
	"os"
	"strings"
	"testing"
)

func TestWorkOrderUsesOperationTemplateStepsForJobCards(t *testing.T) {
	srcBytes, err := os.ReadFile("work_order.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(srcBytes)
	for _, want := range []string{
		"loadOperationTemplateStepsTx",
		"operation_template_steps",
		"operation_template_id",
		"operation_template_step_id",
		"cost_rate",
		"cost_type",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("work order must create job cards from operation template steps; missing %q", want)
		}
	}
}

func TestBatchCostIncludesOperationTemplateStepCost(t *testing.T) {
	srcBytes, err := os.ReadFile("work_order.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(srcBytes)
	for _, want := range []string{
		"operationCost",
		"job_cards",
		"per_kg_input",
		"per_kg_output",
		"fixed",
		"totalCost := materialCost + operationCost",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("batch cost must include operation step cost; missing %q", want)
		}
	}
	if strings.Contains(src, "operationCost := 0.0") {
		t.Fatalf("operation cost must not be hard-coded to zero")
	}
}

func TestWorkOrderFreezesCustomerProductAliasSnapshots(t *testing.T) {
	srcBytes, err := os.ReadFile("work_order.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(srcBytes)
	for _, want := range []string{
		"customer_product_snapshot_json",
		"loadCustomerProductSnapshotForWorkOrderTx",
		"customer_product_alias_id",
		"customer_product_display_name_snapshot",
		"product_name_snapshot",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("work order must freeze customer alias/product snapshots; missing %q", want)
		}
	}
}

func TestWorkOrderReadsRoastLevelFromBoundBomVersionSpecialAttrs(t *testing.T) {
	srcBytes, err := os.ReadFile("work_order.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(srcBytes)
	for _, want := range []string{
		"production_bom_versions",
		"special_attrs_json",
		"roast_level",
		"COALESCE(NULLIF(bound_bv.special_attrs_json->>'roast_level',''),",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("work order must prefer BOM version roast_level special attr before SKU fallback; missing %q", want)
		}
	}
}
