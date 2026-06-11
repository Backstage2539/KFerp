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

func TestWorkOrderFreezesProductProductionConfigSnapshot(t *testing.T) {
	srcBytes, err := os.ReadFile("work_order.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(srcBytes)
	for _, want := range []string{
		"production_config_snapshot_json",
		"loadProductProductionConfigSnapshotForWorkOrderTx",
		"product_production_configs",
		"product_production_config_fields",
		"expected_loss_rate",
		"process_route_id",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("work order must freeze product production config snapshots; missing %q", want)
		}
	}
}

func TestWorkOrderFreezesProcessRouteAndUsesUsableDefaultBomPriority(t *testing.T) {
	srcBytes, err := os.ReadFile("work_order.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(srcBytes)
	for _, want := range []string{
		"loadProcessRouteSnapshotForWorkOrderTx",
		"process_route_operations",
		"operation_id",
		"workstation_id",
		"output_bom.bom_version_id",
		"EXISTS (SELECT 1 FROM %s.production_bom_version_items item WHERE item.version_id=v.id)",
		"CASE WHEN pb.id=COALESCE(NULLIF(ppc.production_bom_id,0), pbb.bom_id, 0)",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("work order must freeze process route and use usable default BOM priority; missing %q", want)
		}
	}
}

func TestMaterialSnapshotsUseUsableDefaultBomPriority(t *testing.T) {
	srcBytes, err := os.ReadFile("material_consumption.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(srcBytes)
	for _, want := range []string{
		"product_production_configs ppc",
		"output_bom.bom_version_id",
		"pb.output_product_id=p.id",
		"EXISTS (SELECT 1 FROM %s.production_bom_version_items item WHERE item.version_id=v.id)",
		"CASE WHEN v.id=COALESCE(NULLIF(ppc.production_bom_version_id,0), pbb.bom_version_id, 0)",
		"CASE WHEN pb.id=COALESCE(NULLIF(ppc.production_bom_id,0), pbb.bom_id, 0)",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("material snapshots must use usable default BOM priority; missing %q", want)
		}
	}
}
