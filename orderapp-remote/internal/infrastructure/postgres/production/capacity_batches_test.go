package production

import (
	productionapp "orderapp/internal/application/production"
	"testing"
)

func TestCapacityBatchMetricsUnitsAndCosts(t *testing.T) {
	for _, tc := range []struct {
		unit   string
		size   float64
		count  int
		method string
		qty    float64
		grams  int64
		cost   float64
	}{
		{"kg", 20, 6, "time", 120, 120000, 255},
		{"g", 200, 2, "time", 400, 400, 85},
		{"件", 20, 6, "piece", 120, 27240, 24},
		{"kg", 0.1, 3, "time", 0.3, 300, 127.5},
	} {
		got := plannedCapacitySplitMetricsWithBasis(productionapp.ProductionPlanOperationSplit{BatchSizeQty: tc.size, BatchSizeUnit: tc.unit, PlannedBatchCount: tc.count, StandardMinutes: 25, HourlyRate: 102, CostMethod: tc.method, PieceRate: 0.2}, 227, 0, 0)
		if got.PlannedQty != tc.qty || got.PlannedQtyG != tc.grams || got.PlannedBatchCount != tc.count || got.PlannedMinutes != tc.count*25 || got.PlannedOperationCost != tc.cost {
			t.Errorf("%+v: %+v", tc, got)
		}
	}
}
