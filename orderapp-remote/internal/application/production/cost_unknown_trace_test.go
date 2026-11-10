package production

import "testing"

func TestExecutionTraceDoesNotShowUnknownCostAsNumber(t *testing.T) {
	wo := WorkOrderRow{ID: 1, RunningItemID: 2, ActualCost: 12.5, ActualCostUnavailable: true}
	fallback := batchCostFallback(wo)
	if !fallback.ActualCostUnavailable {
		t.Fatal("fallback lost unavailable status")
	}
	rows := buildProductionTraceTimeline(wo, nil, nil, nil, nil, ProductionLogsResult{}, fallback)
	for _, row := range rows {
		if row.Type == "cost" {
			if row.Summary != "待核算" {
				t.Fatalf("unknown cost shown as %q", row.Summary)
			}
			return
		}
	}
	t.Fatal("cost trace missing")
}
