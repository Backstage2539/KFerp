package production

import (
	app "orderapp/internal/application/production"
	"testing"
)

func TestSemiFinishedCapacityReplanPreservesStationAndRecalculatesMetrics(t *testing.T) {
	before := app.ProductionPlanItem{ID: 1, PlannedG: 60000}
	after := app.ProductionPlanItem{ID: 1, PlannedG: 45000}
	splits := []app.ProductionPlanOperationSplit{{ProductionPlanItemID: 1, Operation: "烘焙", WorkstationID: 3, WorkstationCapacityID: 4, BatchSizeUnit: "kg", BatchSizeQty: 10, StandardMinutes: 20, HourlyRate: 60, PlannedQty: 60}}
	got, err := replanSemiFinishedCapacity(before, after, splits)
	if err != nil || len(got) != 1 {
		t.Fatalf("replan: %v %#v", err, got)
	}
	if got[0].WorkstationID != 3 || got[0].PlannedQty != 45 || got[0].PlannedBatchCount != 5 || got[0].PlannedMinutes != 100 || got[0].PlannedOperationCost != 100 {
		t.Fatalf("unexpected result: %#v", got[0])
	}
}

func TestSemiFinishedCapacityReplanZeroAndUnknownBasis(t *testing.T) {
	split := app.ProductionPlanOperationSplit{ProductionPlanItemID: 1, BatchSizeUnit: "kg", PlannedQty: 10}
	got, err := replanSemiFinishedCapacity(app.ProductionPlanItem{ID: 1, PlannedG: 10000}, app.ProductionPlanItem{ID: 1}, []app.ProductionPlanOperationSplit{split})
	if err != nil || len(got) != 0 {
		t.Fatalf("zero target: %#v %v", got, err)
	}
	_, err = replanSemiFinishedCapacity(app.ProductionPlanItem{ID: 1}, app.ProductionPlanItem{ID: 1, PlannedG: 5000}, []app.ProductionPlanOperationSplit{split})
	if err == nil {
		t.Fatal("unknown previous quantity must be reviewed manually")
	}
}

func TestSemiFinishedCapacityReplanQuantityAndUnsupportedUnits(t *testing.T) {
	before := app.ProductionPlanItem{ID: 1, SalesSpecCount: 100}
	after := app.ProductionPlanItem{ID: 1, SalesSpecCount: 50}
	split := app.ProductionPlanOperationSplit{ProductionPlanItemID: 1, Operation: "包装", SchedulingMode: "quantity", BatchSizeUnit: "袋", PlannedQty: 100, ThroughputPerHour: 25, CostMethod: "piece", PieceRate: 0.2}
	got, err := replanSemiFinishedCapacity(before, after, []app.ProductionPlanOperationSplit{split})
	if err != nil || len(got) != 1 || got[0].PlannedQty != 50 || got[0].PlannedMinutes != 120 || got[0].PlannedOperationCost != 10 {
		t.Fatalf("quantity split: %#v %v", got, err)
	}
	split.BatchSizeUnit = "unknown"
	if _, err = replanSemiFinishedCapacity(before, after, []app.ProductionPlanOperationSplit{split}); err == nil {
		t.Fatal("unsupported units must fail closed")
	}
}

func TestSemiFinishedCapacityReplanLegacyBatchQuantity(t *testing.T) {
	before := app.ProductionPlanItem{ID: 1, PlannedG: 20000}
	after := app.ProductionPlanItem{ID: 1, PlannedG: 10000}
	split := app.ProductionPlanOperationSplit{ProductionPlanItemID: 1, BatchSizeUnit: "kg", BatchSizeQty: 10, PlannedBatchCount: 2}
	got, err := replanSemiFinishedCapacity(before, after, []app.ProductionPlanOperationSplit{split})
	if err != nil || len(got) != 1 || got[0].PlannedQty != 10 {
		t.Fatalf("legacy batches: %#v %v", got, err)
	}
}
