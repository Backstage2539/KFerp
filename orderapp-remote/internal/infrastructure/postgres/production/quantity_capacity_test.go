package production

import (
	productionapp "orderapp/internal/application/production"
	"testing"
)

func TestQuantityCapacityUsesProportionalSeconds(t *testing.T) {
	got := plannedCapacitySplitMetrics(productionapp.ProductionPlanOperationSplit{SchedulingMode: "quantity", ThroughputPerHour: 600, PlannedQty: 23, BatchSizeQty: 10, BatchSizeUnit: "kg", StandardMinutes: 10, HourlyRate: 120})
	if got.PlannedBatchCount != 0 || (got.PlannedSeconds == nil || *got.PlannedSeconds != 138) || got.PlannedOperationCost != 4.6 {
		t.Fatalf("unexpected quantity schedule: %+v", got)
	}
}

func TestSchedulingModeMatchesReportingType(t *testing.T) {
	for _, tc := range []struct {
		reporting, mode, unit string
		valid                 bool
	}{
		{"batch", "batch", "kg", true}, {"batch", "quantity", "kg", false},
		{"incoming", "quantity", "kg", true}, {"incoming", "batch", "kg", false}, {"incoming", "quantity", "件", false},
		{"piece", "quantity", "件", true}, {"piece", "quantity", "kg", false}, {"legacy", "batch", "kg", true},
	} {
		if err := validateSchedulingModeForReporting(tc.reporting, tc.mode, tc.unit); (err == nil) != tc.valid {
			t.Errorf("%+v: %v", tc, err)
		}
	}
}

func TestQuantityProcessingCostUsesInputAndFrozenSpeed(t *testing.T) {
	cost, ok := quantityProcessingCost(23000, "g", "kg", 600, 120)
	if !ok || cost != 4.6 {
		t.Fatalf("cost=%v available=%v", cost, ok)
	}
	if _, ok := quantityProcessingCost(23, "kg", "kg", 0, 120); ok {
		t.Fatal("zero speed must report unavailable")
	}
}

func TestQuantityPieceCardKeepsNativeCount(t *testing.T) {
	split := productionapp.ProductionPlanOperationSplit{SchedulingMode: "quantity", BatchSizeUnit: "件", PlannedQty: 180, PlannedQtyG: 81720}
	if got := jobCardPlannedInputQty(split, 100000); got != 180 {
		t.Fatalf("count replaced by weight: %v", got)
	}
}

func TestQuantityProcessingCostSplitMatchesWhole(t *testing.T) {
	half, _ := quantityProcessingCost(1, "kg", "kg", 200, 1)
	whole, _ := quantityProcessingCost(2, "kg", "kg", 200, 1)
	if half+half != whole {
		t.Fatalf("split %.6f whole %.6f", half+half, whole)
	}
}

func TestQuantitySplitMayDifferFromFrozenCountTarget(t *testing.T) {
	item := productionapp.ProductionPlanItem{ID: 1, SalesSpecCount: 20, ProcessSnapshotJSON: `{"operations":[{"seq":1,"operation":"包装","execution_json":"{\"reporting_type\":\"piece\"}"}]}`}
	split := productionapp.ProductionPlanOperationSplit{ProductionPlanItemID: 1, OperationSeq: 1, Operation: "包装", SchedulingMode: "quantity", BatchSizeUnit: "件", PlannedQty: 21}
	for _, qty := range []float64{19, 20, 21} {
		split.PlannedQty = qty
		if err := validateProductionPlanOperationSplitCoverage(item, []productionapp.ProductionPlanOperationSplit{split}); err != nil {
			t.Fatalf("valid quantity %v rejected: %v", qty, err)
		}
	}
}
