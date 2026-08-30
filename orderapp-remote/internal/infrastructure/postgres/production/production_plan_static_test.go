package production

import (
	productionapp "orderapp/internal/application/production"
	"os"
	"strings"
	"testing"
)

func TestWorkOrderStartLeavesJobCardsPendingForWorkstationExecution(t *testing.T) {
	src, err := os.ReadFile("production_plan.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	if strings.Contains(text, "UPDATE %s.job_cards SET status='running'") {
		t.Fatal("work order start must not start every job card; workstation owns each operation start")
	}
}

func TestPendingJobCardsDoNotPretendTheyAlreadyStarted(t *testing.T) {
	src, err := os.ReadFile("production_plan.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, forbidden := range []string{
		"status,started_at,operator,planned_input_qty",
		"'pending',now()",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("pending job cards must leave start time empty until the workstation starts them; found %q", forbidden)
		}
	}
}

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

func TestNewProductionPlanUsesBomMaterialLossAsOnlyInputAdjustment(t *testing.T) {
	src, err := os.ReadFile("production_plan.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	start := strings.Index(text, "func createProductionPlanItemForGroupTx")
	end := strings.Index(text, "func linkProcessingRequestItemToPlanTx")
	if start < 0 || end <= start {
		t.Fatal("createProductionPlanItemForGroupTx source not found")
	}
	body := text[start:end]
	for _, want := range []string{
		"productionInputGFromBomMaterialLoss(group.NeedG, bomMaterialLossRate)",
		"plannedFinishedInventoryAddition(group.SpecG, group.NeedG)",
		"BomYieldRate:        1",
		"processSnapshot.YieldRate = 1",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("new production plans must freeze target output and apply only BOM material loss; missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"normalizedYield := 1 - bomMaterialLossRate",
		"runningInventoryPlan(group.SpecG, group.NeedG, group.InputG",
		"bomRoute.YieldRate",
		"expected_loss_rate",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("new production plan must not derive input/output from legacy overall yield; found %q", forbidden)
		}
	}
}

func TestProductionPlanCreateSplitsOrderLevelDemandBeforeFilteringSelectedRows(t *testing.T) {
	src, err := os.ReadFile("production_plan.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, want := range []string{
		"fetchUnproducedNeeds(ctx, tx",
		"splitUnproducedNeedsByProductionPlanQuery(ctx, tx, rows)",
		"attachProductionDemandStatusesQuery(ctx, tx, appRows)",
		"selectedProductionPlanStartNeeds(appRows, cmd.Selected)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("formal production plan create must reuse order-level demand split before selected filtering; missing %q", want)
		}
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

func TestProductionPlanDemandUsesFrozenQuantitySnapshotWithoutParsingSpecLabel(t *testing.T) {
	unproducedSrc, err := os.ReadFile("unprod_summary.go")
	if err != nil {
		t.Fatal(err)
	}
	planSrc, err := os.ReadFile("production_plan.go")
	if err != nil {
		t.Fatal(err)
	}
	combined := string(unproducedSrc) + "\n" + string(planSrc)
	for _, want := range []string{
		"production_quantity_snapshot",
		"inventory_qty_per_sales_unit",
		"inventory_unit",
		"sales_spec_snapshot_json",
		"planned_inventory_qty",
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("production planning must use and freeze authoritative sales-spec quantity conversion; missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"regexp_replace(COALESCE(oi.spec",
		"regexp_replace(COALESCE(oi.spec,''",
	} {
		if strings.Contains(combined, forbidden) {
			t.Fatalf("formal production planning must not parse order item spec labels; found %q", forbidden)
		}
	}
}

func TestProductionPlanResolvesOneBomVersionForMaterialsAndRoute(t *testing.T) {
	planSrc, err := os.ReadFile("production_plan.go")
	if err != nil {
		t.Fatal(err)
	}
	materialSrc, err := os.ReadFile("material_consumption.go")
	if err != nil {
		t.Fatal(err)
	}
	combined := string(planSrc) + "\n" + string(materialSrc)
	for _, want := range []string{
		"resolveProductionBomForDemandProductSpecTx",
		"buildMaterialSnapshotForBomVersionVariantTx",
		"bomRoute.BomVersionID",
		"group.BomVariantID",
		"BomInherited",
		"BomSourceProductID",
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("production plan must freeze one resolved BOM version for materials and route; missing %q", want)
		}
	}
}

func TestProductionPlanOperationSplitsOwnCapacityBatchPlanning(t *testing.T) {
	src, err := os.ReadFile("production_plan.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, want := range []string{
		"SaveProductionPlanOperationSplits",
		"loadProductionPlanOperationSplitsTx",
		"production_plan_operation_splits",
		"plannedCapacitySplitMetrics",
		"validateProductionPlanOperationSplitCoverage",
		"workstation_capacity_id",
		"planned_batch_count",
		"planned_qty_g",
		"planned_minutes",
		"planned_operation_cost",
		"createPendingJobCardsForWorkOrderTx(ctx, tx, schema, id, item.ProcessSnapshotJSON, item.OperationTemplateID, item.PlannedG, item.SalesSpecCount, splits)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("production plan must own operation capacity split planning; missing %q", want)
		}
	}
}

func TestPlannedCapacitySplitMetricsDerivesBatchCountFromPlannedQty(t *testing.T) {
	got := plannedCapacitySplitMetrics(productionapp.ProductionPlanOperationSplit{
		PlannedQty:      20,
		BatchSizeQty:    18,
		BatchSizeUnit:   "kg",
		StandardMinutes: 15,
		HourlyRate:      300,
	})
	if got.PlannedBatchCount != 2 {
		t.Fatalf("PlannedBatchCount=%d, want 2", got.PlannedBatchCount)
	}
	if got.PlannedQty != 20 || got.PlannedQtyG != 20000 {
		t.Fatalf("planned quantity = %v / %d, want 20kg / 20000g", got.PlannedQty, got.PlannedQtyG)
	}
	if got.PlannedMinutes != 30 || got.PlannedOperationCost != 150 {
		t.Fatalf("planned cost metrics = %d / %.2f, want 30 / 150", got.PlannedMinutes, got.PlannedOperationCost)
	}
}

func TestPlannedCapacitySplitMetricsDerivesCountUnitQuantityFromSpec(t *testing.T) {
	got := plannedCapacitySplitMetrics(productionapp.ProductionPlanOperationSplit{
		PlannedQty:      23,
		BatchSizeQty:    10,
		BatchSizeUnit:   "袋",
		StandardMinutes: 5,
		HourlyRate:      120,
	}, 454)
	if got.PlannedBatchCount != 3 {
		t.Fatalf("PlannedBatchCount=%d, want 3", got.PlannedBatchCount)
	}
	if got.PlannedQtyG != 10442 {
		t.Fatalf("PlannedQtyG=%d, want 10442", got.PlannedQtyG)
	}
	if got.PlannedMinutes != 15 || got.PlannedOperationCost != 30 {
		t.Fatalf("planned cost metrics = %d / %.2f, want 15 / 30", got.PlannedMinutes, got.PlannedOperationCost)
	}
}

func TestProductionDemandInventoryUnitCompatibility(t *testing.T) {
	weightDemand := startRunGroup{
		ProductID:     789,
		ProductName:   "如目达摩",
		SpecLabel:     "454g",
		InventoryUnit: "kg",
		OrderNos:      "SO-20260725-0001",
	}
	if err := validateProductionDemandInventoryUnitAgainstBomOutput(
		weightDemand,
		latestUsableBomRoute{BomOutputUnit: "g"},
	); err != nil {
		t.Fatalf("kg demand should be compatible with g BOM output: %v", err)
	}
	err := validateProductionDemandInventoryUnitAgainstBomOutput(
		weightDemand,
		latestUsableBomRoute{BomOutputUnit: "件"},
	)
	if err == nil {
		t.Fatal("kg demand should reject count BOM output")
	}
	for _, want := range []string{"SO-20260725-0001", "如目达摩", "454g", "kg", "件"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("unit mismatch error %q missing %q", err, want)
		}
	}

	countDemand := weightDemand
	countDemand.SpecLabel = "1件"
	countDemand.InventoryUnit = "件"
	if err := validateProductionDemandInventoryUnitAgainstBomOutput(
		countDemand,
		latestUsableBomRoute{BomOutputUnit: "件"},
	); err != nil {
		t.Fatalf("direct product count demand should accept the exact BOM output unit: %v", err)
	}
	countDemand.InventoryUnit = "盒"
	err = validateProductionDemandInventoryUnitAgainstBomOutput(
		countDemand,
		latestUsableBomRoute{BomOutputUnit: "袋"},
	)
	if err == nil || !strings.Contains(err.Error(), "unit incompatible") {
		t.Fatalf("direct product named-unit mismatch must be rejected, err=%v", err)
	}
}
