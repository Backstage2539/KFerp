package production

import (
	"os"
	"strings"
	"testing"
)

func TestProductionPlanCreateAllowsDefaultInputForSelectedRows(t *testing.T) {
	src, err := os.ReadFile("production_plan.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, forbidden := range []string{
		"input_g required",
		"cmd.InputByKey[key] <= 0",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("formal production plan create must allow selected rows to use default input; found %q", forbidden)
		}
	}
	if !strings.Contains(text, "groupStartNeedsForRuns(needs, cmd.InputByKey") {
		t.Fatal("formal production plan create must delegate default input calculation to groupStartNeedsForRuns")
	}
}

func TestProductionPlanListSupportsStatusAndTimeFilters(t *testing.T) {
	src, err := os.ReadFile("production_plan.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, want := range []string{
		"productionPlanTimeFieldColumn",
		"pp.created_at",
		"pp.submitted_at",
		"pp.completed_at",
		"COALESCE(to_char(pp.completed_at",
		"query.From",
		"query.To",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("production plan list must support filter/query field %q", want)
		}
	}
}

func TestProductionPlanItemsResolveLatestUsableBomVersionRouteWithoutFallback(t *testing.T) {
	planSrc, err := os.ReadFile("production_plan.go")
	if err != nil {
		t.Fatal(err)
	}
	workOrderSrc, err := os.ReadFile("work_order.go")
	if err != nil {
		t.Fatal(err)
	}
	combined := string(planSrc) + "\n" + string(workOrderSrc)
	for _, want := range []string{
		"resolveLatestUsableBomRouteForProductTx",
		"latest usable production BOM version not found",
		"multiple active production BOMs found",
		"default production BOM is no longer an output BOM",
		"最新可用 BOM 版本未配置工艺路线",
		"production_bom_versions",
		"pb.output_product_id=$1",
		"ORDER BY v.published_at DESC NULLS LAST, v.created_at DESC, v.id DESC",
		"loadProcessRouteSnapshotByIDTx",
		"process_route_snapshot_json",
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("production plan must resolve latest usable BOM version route; missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"loadProcessRouteSnapshotForWorkOrderTx(ctx, tx, schema, group.ProductID)",
		"loadActiveProcessTemplateSnapshotTx(ctx, tx, schema, group.ProductID)",
	} {
		if strings.Contains(string(planSrc), forbidden) {
			t.Fatalf("production plan item creation must not fallback via product config or legacy process template; found %q", forbidden)
		}
	}
}
