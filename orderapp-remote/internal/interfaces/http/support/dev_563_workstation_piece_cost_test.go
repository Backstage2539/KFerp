package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev563WorkstationPieceCostContracts(t *testing.T) {
	files := map[string]string{
		"reqStore":         filepath.Join("internal", "interfaces", "http", "support", "req_store.go"),
		"manufacturing":    filepath.Join("internal", "application", "manufacturing", "service.go"),
		"production":       filepath.Join("internal", "infrastructure", "postgres", "production", "production_plan.go"),
		"workstations":     filepath.Join("frontend-vue-shell", "src", "views", "ManufacturingWorkstationsView.vue"),
		"requirements":     filepath.Join("docs", "REQUIREMENTS.md"),
		"acceptance":       filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		"productionManual": filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"),
		"costingManual":    filepath.Join("docs", "OP_MANUAL_COSTING.md"),
		"evidence":         filepath.Join("docs", "acceptance", "2026-07-30-workstation-piece-cost.md"),
	}
	contents := map[string]string{}
	for key, rel := range files {
		contents[key] = string(readOrderAppFileForTest(t, rel))
	}

	for _, marker := range []string{
		"PR-563-WORKSTATION-PIECE-COST",
		"DEV-563-CAPACITY-COST-METHOD",
		"DEV-563-STANDARD-COST-SNAPSHOT",
		"DEV-563-PLAN-ACTUAL-PIECE-COST",
		"DEV-563-UI-AUDIT",
		"DEV-563-DOCS-ACCEPTANCE",
		"REV-563-WORKSTATION-PIECE-COST",
	} {
		if !strings.Contains(contents["reqStore"], marker) {
			t.Fatalf("req_store missing %s", marker)
		}
	}

	for _, marker := range []string{"CostMethod", "PieceRate", "cost_method", "piece_rate"} {
		if !strings.Contains(contents["manufacturing"], marker) {
			t.Fatalf("manufacturing service missing %s", marker)
		}
	}
	for _, marker := range []string{"normalizeProductionCostMethod", "PlannedQty * split.PieceRate"} {
		if !strings.Contains(contents["production"], marker) {
			t.Fatalf("production piece costing missing %s", marker)
		}
	}
	for _, marker := range []string{"按时间", "按件", "计件成本", "capacityForm.cost_method", "capacityForm.piece_rate"} {
		if !strings.Contains(contents["workstations"], marker) {
			t.Fatalf("workstation editor missing %s", marker)
		}
	}
	for _, key := range []string{"requirements", "acceptance", "productionManual", "costingManual", "evidence"} {
		if !strings.Contains(contents[key], "PR-563") {
			t.Fatalf("%s missing PR-563 marker", key)
		}
	}
}
