package production

import (
	app "orderapp/internal/application/production"
	"strings"
	"testing"
)

func TestFlexibleCapacityAllowsShortAndOverWithWarnings(t *testing.T) {
	for _, tc := range []struct {
		name, reporting, mode, unit string
		target, qty                 float64
		grams                       int64
	}{
		{"batch_short", "batch", "batch", "kg", 45.113, 45, 45000},
		{"batch_over", "batch", "batch", "kg", 45.113, 56.1, 56100},
		{"incoming_short", "incoming", "quantity", "kg", 45.113, 45, 45000},
		{"incoming_over", "incoming", "quantity", "kg", 45.113, 56.1, 56100},
		{"piece_short", "piece", "quantity", "件", 100, 90, 20430},
		{"piece_over", "piece", "quantity", "件", 100, 110, 24970},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := app.ProductionPlanItem{ID: 1, ProductName: "初晓", PlannedG: 45113, PlannedOutputG: 22700, SalesSpecCount: 100,
				ProcessSnapshotJSON: `{"operations":[{"seq":1,"operation":"加工","execution_json":"{\"reporting_type\":\"` + tc.reporting + `\"}"}]}`}
			rows := []app.ProductionPlanOperationSplit{{ProductionPlanItemID: 1, OperationSeq: 1, Operation: "加工", SchedulingMode: tc.mode, BatchSizeUnit: tc.unit, PlannedQty: tc.qty, PlannedQtyG: tc.grams}}
			if err := validateProductionPlanOperationSplitCoverage(item, rows); err != nil {
				t.Fatalf("valid arrangement rejected: %v", err)
			}
			preview := previewProductionPlanOperationSplits([]app.ProductionPlanItem{item}, rows)
			if len(preview.Warnings) != 1 || !strings.Contains(preview.Warnings[0], "不影响继续") {
				t.Fatalf("expected nonblocking variance warning: %+v", preview)
			}
			readiness := productionPlanReadiness(app.ProductionPlanDetail{Status: "draft", Items: []app.ProductionPlanItem{item}, OperationSplits: rows})
			found := false
			for _, issue := range readiness.Issues {
				if issue.Category == "operation" {
					if issue.Severity != "warning" {
						t.Fatalf("capacity blocked: %+v", issue)
					}
					found = true
				}
			}
			if !found || readiness.WarningCount != 1 {
				t.Fatalf("missing readiness warning: %+v", readiness)
			}
		})
	}
}

func TestFlexibleCapacityStillRejectsMissingAndZeroArrangements(t *testing.T) {
	item := app.ProductionPlanItem{ID: 1, PlannedG: 45113, SalesSpecCount: 100, ProcessSnapshotJSON: `{"operations":[{"seq":1,"operation":"加工"}]}`}
	for _, rows := range [][]app.ProductionPlanOperationSplit{nil, {{ProductionPlanItemID: 1, OperationSeq: 1, Operation: "加工", BatchSizeUnit: "kg"}}, {{ProductionPlanItemID: 1, OperationSeq: 1, Operation: "加工", BatchSizeUnit: "件"}}} {
		if err := validateProductionPlanOperationSplitCoverage(item, rows); err == nil {
			t.Fatalf("invalid arrangement accepted: %+v", rows)
		}
	}
}
