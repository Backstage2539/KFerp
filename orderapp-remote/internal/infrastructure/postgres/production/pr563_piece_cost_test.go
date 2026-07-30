package production

import (
	productionapp "orderapp/internal/application/production"
	"os"
	"strings"
	"testing"
)

func TestPR563PieceCostUsesPlannedFinishedQuantityInsteadOfBatchCount(t *testing.T) {
	got := plannedCapacitySplitMetrics(productionapp.ProductionPlanOperationSplit{
		PlannedQty:      100,
		BatchSizeQty:    20,
		BatchSizeUnit:   "件",
		StandardMinutes: 5,
		HourlyRate:      300,
		CostMethod:      "piece",
		PieceRate:       0.5,
	}, 227)

	if got.PlannedBatchCount != 5 {
		t.Fatalf("planned batch count = %d, want 5", got.PlannedBatchCount)
	}
	if got.PlannedQtyG != 22700 {
		t.Fatalf("planned quantity g = %d, want 22700", got.PlannedQtyG)
	}
	if got.PlannedMinutes != 25 {
		t.Fatalf("planned minutes = %d, want 25", got.PlannedMinutes)
	}
	if got.PlannedOperationCost != 50 {
		t.Fatalf("planned operation cost = %.4f, want 50", got.PlannedOperationCost)
	}
}

func TestPR563JobCardRecalculatesPieceSnapshotUnitRateAsWorkOrderTotal(t *testing.T) {
	got := plannedJobCardMetrics(processSnapshotOperation{
		BatchSizeQty:         20,
		BatchSizeUnit:        "件",
		StandardMinutes:      5,
		CostMethod:           "piece",
		PieceRate:            0.5,
		PlannedOperationCost: 0.5, // process-template snapshot stores the unit rate
	}, 22700, 100)
	if got.PlannedOperationCost != 50 {
		t.Fatalf("job-card planned piece cost = %.4f, want 50", got.PlannedOperationCost)
	}
}

func TestPR563CountSplitUsesFrozenPlanSalesSpecRatioWithoutSpecG(t *testing.T) {
	got := plannedCapacitySplitMetricsWithBasis(productionapp.ProductionPlanOperationSplit{
		PlannedQty:      100,
		BatchSizeQty:    20,
		BatchSizeUnit:   "件",
		StandardMinutes: 5,
		CostMethod:      "piece",
		PieceRate:       0.5,
	}, 0, 100, 22700)
	if got.PlannedQtyG != 22700 {
		t.Fatalf("planned inventory projection = %d, want 22700 from frozen plan ratio", got.PlannedQtyG)
	}
	if got.PlannedOperationCost != 50 {
		t.Fatalf("planned piece cost = %.4f, want 50", got.PlannedOperationCost)
	}
}

func TestPR563CountSplitUsesFinishedOutputInsteadOfLossAdjustedInput(t *testing.T) {
	item := productionapp.ProductionPlanItem{
		PlannedG:       25795,
		PlannedOutputG: 22700,
		SalesSpecCount: 100,
		ProcessSnapshotJSON: `{"operations":[
			{"seq":1,"operation":"烘焙"},
			{"seq":2,"operation":"包装"}
		]}`,
	}
	if got := productionPlanItemOutputTargetG(item); got != 22700 {
		t.Fatalf("finished output projection = %d, want 22700", got)
	}
	splits := []productionapp.ProductionPlanOperationSplit{
		{OperationSeq: 1, Operation: "烘焙", BatchSizeUnit: "kg", PlannedQty: 25.795, PlannedQtyG: 25795},
		{OperationSeq: 2, Operation: "包装", BatchSizeUnit: "件", PlannedQty: 100, PlannedQtyG: 22700, CostMethod: "piece", PieceRate: 0.5},
	}
	if err := validateProductionPlanOperationSplitCoverage(item, splits); err != nil {
		t.Fatalf("valid loss-adjusted operation coverage: %v", err)
	}

	preview := previewProductionPlanOperationSplits([]productionapp.ProductionPlanItem{item}, splits)
	if preview.CoverageSummary.RequiredG != 25795 || preview.CoverageSummary.ArrangedG != 25795 || preview.CoverageSummary.Status != "matched" {
		t.Fatalf("coverage summary = %+v, want input-equivalent 25795g matched", preview.CoverageSummary)
	}
	if len(preview.OperationCoverage) != 2 ||
		preview.OperationCoverage[0].RequiredG != 25795 ||
		preview.OperationCoverage[1].RequiredG != 22700 ||
		preview.OperationCoverage[1].ArrangedG != 22700 {
		t.Fatalf("operation coverage = %+v, want roast input and package finished output", preview.OperationCoverage)
	}
}

func TestPR563OperationSplitCoverageRejectsShortMissingAndMixedDimensions(t *testing.T) {
	item := productionapp.ProductionPlanItem{
		ProductName:    "初晓拼配",
		PlannedG:       25795,
		PlannedOutputG: 22700,
		SalesSpecCount: 100,
		ProcessSnapshotJSON: `{"operations":[
			{"seq":1,"operation":"烘焙"},
			{"seq":2,"operation":"包装"}
		]}`,
	}
	roast := productionapp.ProductionPlanOperationSplit{
		OperationSeq: 1, Operation: "烘焙", BatchSizeUnit: "kg", PlannedQty: 25.795, PlannedQtyG: 25795,
	}
	shortPackage := productionapp.ProductionPlanOperationSplit{
		OperationSeq: 2, Operation: "包装", BatchSizeUnit: "件", PlannedQty: 90, PlannedQtyG: 20430,
	}
	if err := validateProductionPlanOperationSplitCoverage(item, []productionapp.ProductionPlanOperationSplit{roast, shortPackage}); err == nil || !strings.Contains(err.Error(), "件数产能不足") {
		t.Fatalf("short package coverage error = %v", err)
	}
	if err := validateProductionPlanOperationSplitCoverage(item, []productionapp.ProductionPlanOperationSplit{roast}); err == nil || !strings.Contains(err.Error(), "包装") || !strings.Contains(err.Error(), "尚未选择工位产能") {
		t.Fatalf("missing package split error = %v", err)
	}
	mixedPackage := append([]productionapp.ProductionPlanOperationSplit{roast},
		productionapp.ProductionPlanOperationSplit{OperationSeq: 2, Operation: "包装", BatchSizeUnit: "件", PlannedQty: 100, PlannedQtyG: 22700},
		productionapp.ProductionPlanOperationSplit{OperationSeq: 2, Operation: "包装", BatchSizeUnit: "kg", PlannedQty: 1, PlannedQtyG: 1000},
	)
	if err := validateProductionPlanOperationSplitCoverage(item, mixedPackage); err == nil || !strings.Contains(err.Error(), "不能同时使用重量和件数") {
		t.Fatalf("mixed package coverage error = %v", err)
	}
}

func TestPR563OperationSplitRejectsConflictingOrAmbiguousFrozenOperationIdentity(t *testing.T) {
	item := productionapp.ProductionPlanItem{
		ProcessSnapshotJSON: `{"operations":[
			{"seq":1,"operation_id":101,"operation":"烘焙"},
			{"seq":2,"operation_id":102,"operation":"包装"}
		]}`,
	}
	conflicting := []productionapp.ProductionPlanOperationSplit{{
		OperationSeq: 1,
		OperationID:  102,
		Operation:    "烘焙",
	}}
	if err := validateProductionPlanOperationSplitIdentities(item, conflicting); err == nil || !strings.Contains(err.Error(), "身份冲突") {
		t.Fatalf("conflicting operation identity error = %v", err)
	}

	unmatched := []productionapp.ProductionPlanOperationSplit{{
		OperationSeq: 3,
		Operation:    "包装",
	}}
	if err := validateProductionPlanOperationSplitIdentities(item, unmatched); err == nil || !strings.Contains(err.Error(), "无法匹配") {
		t.Fatalf("unmatched operation identity error = %v", err)
	}

	valid := []productionapp.ProductionPlanOperationSplit{{
		OperationSeq: 2,
		OperationID:  102,
		Operation:    "包装",
	}}
	if err := validateProductionPlanOperationSplitIdentities(item, valid); err != nil {
		t.Fatalf("valid operation identity error = %v", err)
	}
	if got := operationSplitsForSnapshotOperation(
		processSnapshotOperation{Seq: 1, OperationID: 101, Operation: "烘焙"},
		conflicting,
	); len(got) != 1 {
		t.Fatalf("priority operation matching returned %d rows, want exactly one", len(got))
	}
	if got := operationSplitsForSnapshotOperation(
		processSnapshotOperation{Seq: 2, OperationID: 102, Operation: "包装"},
		conflicting,
	); len(got) != 0 {
		t.Fatalf("conflicting split matched a second operation: %d rows", len(got))
	}
}

func TestPR563TimeCostRemainsUnchanged(t *testing.T) {
	got := plannedCapacitySplitMetrics(productionapp.ProductionPlanOperationSplit{
		PlannedQty:      20,
		BatchSizeQty:    18,
		BatchSizeUnit:   "kg",
		StandardMinutes: 15,
		HourlyRate:      300,
		CostMethod:      "time",
	})

	if got.PlannedOperationCost != 150 {
		t.Fatalf("planned operation cost = %.4f, want 150", got.PlannedOperationCost)
	}
}

func TestPR563PieceCostNeverFallsBackToHourlyCostWithoutPieceQuantity(t *testing.T) {
	got := plannedCapacitySplitMetrics(productionapp.ProductionPlanOperationSplit{
		PlannedQty:      0,
		BatchSizeQty:    20,
		BatchSizeUnit:   "件",
		StandardMinutes: 5,
		HourlyRate:      300,
		CostMethod:      "piece",
		PieceRate:       0.5,
	})
	if got.PlannedOperationCost != 0 {
		t.Fatalf("piece cost without piece quantity = %.4f, want 0 instead of hourly fallback", got.PlannedOperationCost)
	}
}

func TestPR563InitialBlend100BagsKeepsRoastSortAndPackageCostsSeparate(t *testing.T) {
	roast := plannedCapacitySplitMetrics(productionapp.ProductionPlanOperationSplit{
		Operation:       "烘焙",
		PlannedQty:      22.7,
		BatchSizeQty:    10,
		BatchSizeUnit:   "kg",
		StandardMinutes: 30,
		HourlyRate:      24,
		CostMethod:      "time",
	}, 227)
	sortCost := plannedCapacitySplitMetrics(productionapp.ProductionPlanOperationSplit{
		Operation:       "色选",
		PlannedQty:      22.7,
		BatchSizeQty:    25,
		BatchSizeUnit:   "kg",
		StandardMinutes: 30,
		HourlyRate:      12,
		CostMethod:      "time",
	}, 227)
	pack := plannedCapacitySplitMetrics(productionapp.ProductionPlanOperationSplit{
		Operation:       "包装",
		PlannedQty:      100,
		BatchSizeQty:    20,
		BatchSizeUnit:   "件",
		StandardMinutes: 5,
		HourlyRate:      300,
		CostMethod:      "piece",
		PieceRate:       0.5,
	}, 227)

	if roast.PlannedOperationCost != 36 {
		t.Fatalf("roast cost = %.2f, want 36", roast.PlannedOperationCost)
	}
	if sortCost.PlannedOperationCost != 6 {
		t.Fatalf("sort cost = %.2f, want 6", sortCost.PlannedOperationCost)
	}
	if pack.PlannedOperationCost != 50 {
		t.Fatalf("package cost = %.2f, want 50", pack.PlannedOperationCost)
	}
	if total := roast.PlannedOperationCost + sortCost.PlannedOperationCost + pack.PlannedOperationCost; total != 92 {
		t.Fatalf("operation total = %.2f, want 92", total)
	}
}

func TestPR563ProductionSchemaFreezesCostMethodAndPieceRate(t *testing.T) {
	src, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, want := range []string{
		"cost_method TEXT NOT NULL DEFAULT 'time'",
		"piece_rate NUMERIC(14,4) NOT NULL DEFAULT 0",
		"ALTER TABLE %s.production_plan_operation_splits ADD COLUMN IF NOT EXISTS cost_method",
		"ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS cost_method",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("production schema must freeze piece costing; missing %q", want)
		}
	}
}

func TestPR563ActualPieceQuantityUsesFinishedUnitsBeforeInventoryConversion(t *testing.T) {
	if got := actualPieceQuantity(`{"finished_units":100}`, 22.7, 0.227); got != 100 {
		t.Fatalf("actual piece quantity = %.4f, want 100 frozen finished units", got)
	}
	if got := actualPieceQuantity(`{}`, 22.7, 0.227); got != 100 {
		t.Fatalf("actual piece quantity = %.4f, want 100 converted units", got)
	}
	if got := actualPieceQuantity(`{}`, 22.7, 0); got != 0 {
		t.Fatalf("actual piece quantity = %.4f, want 0 without conversion", got)
	}
}

func TestPR563JobCardCompletionDoesNotTreatInventoryQuantityAsPieceCount(t *testing.T) {
	expectedExpression := map[string]string{
		"stock_entry.go": "ROUND($12::numeric * COALESCE(piece_rate,0), 4)",
		"work_order.go":  "ROUND($11::numeric * COALESCE(piece_rate,0), 4)",
	}
	for file, expression := range expectedExpression {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(src)
		for _, want := range []string{
			"cost_method",
			"piece_rate",
			"actual_output_qty",
			"actualPieceQuantity",
			expression,
		} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s must calculate completed piece cost from actual output; missing %q", file, want)
			}
		}
	}
	stockEntrySource, err := os.ReadFile("stock_entry.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(stockEntrySource), "计件工序完成时必须填写成品件数") {
		t.Fatal("piece-costed completion must reject a missing actual piece quantity")
	}
}
