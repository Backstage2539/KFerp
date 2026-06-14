package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev496ProductionFlowPhase1OptimizationContracts(t *testing.T) {
	files := map[string]string{
		"reqStore":       filepath.Join("internal", "interfaces", "http", "support", "req_store.go"),
		"service":        filepath.Join("internal", "application", "production", "service.go"),
		"topNav":         filepath.Join("frontend-vue-shell", "src", "components", "ProductionTopNav.vue"),
		"overviewView":   filepath.Join("frontend-vue-shell", "src", "views", "ProductionOverviewView.vue"),
		"planView":       filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue"),
		"runningView":    filepath.Join("frontend-vue-shell", "src", "views", "ProduceRunningView.vue"),
		"stockView":      filepath.Join("frontend-vue-shell", "src", "views", "StockOperationsView.vue"),
		"wipView":        filepath.Join("frontend-vue-shell", "src", "views", "WipMaterialsView.vue"),
		"workstationLib": filepath.Join("frontend-vue-shell", "src", "lib", "production-workstation.js"),
		"planLib":        filepath.Join("frontend-vue-shell", "src", "lib", "produce-plan.js"),
		"runningLib":     filepath.Join("frontend-vue-shell", "src", "lib", "produce-running.js"),
		"urlState":       filepath.Join("frontend-vue-shell", "src", "lib", "url-state.js"),
		"requirements":   filepath.Join("docs", "REQUIREMENTS.md"),
		"acceptance":     filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		"manual":         filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"),
		"evidence":       filepath.Join("docs", "acceptance", "2026-06-14-production-flow-phase1-optimization.md"),
	}
	contents := map[string]string{}
	for key, path := range files {
		contents[key] = string(readOrderAppFileForTest(t, path))
	}
	for _, marker := range []string{
		"PR-496-PRODUCTION-FLOW-PHASE1-OPTIMIZATION",
		"DEV-496-PRODUCTION-NAV-BADGES-READMODEL",
		"DEV-496-PRODUCTION-PLAN-STEPPER-NEXT",
		"DEV-496-RUNNING-COMPLETION-PANEL-WIP-CONTEXT",
		"DEV-496-STOCK-WIP-PREFILL",
		"UT-496-PRODUCTION-FLOW-PHASE1-OPTIMIZATION",
		"API-496-PRODUCTION-FLOW-PHASE1-OPTIMIZATION",
		"REV-496-PRODUCTION-FLOW-PHASE1-OPTIMIZATION",
	} {
		if !strings.Contains(contents["reqStore"], marker) {
			t.Fatalf("req_store.go missing %s", marker)
		}
	}
	for _, marker := range []string{
		"TodaySummary",
		"NavBadges",
		"Readiness",
		"ReadinessLabel",
	} {
		if !strings.Contains(contents["service"], marker) {
			t.Fatalf("production service missing %s", marker)
		}
	}
	for _, marker := range []string{
		"navItemsWithProductionBadges",
		"nav-badge",
		"stockOperationContextParams",
		"productionPlanSteps",
		"currentProductionPlanStep",
		"buildProductionPlanNextActions",
		"buildFinishPanelModel",
		"productionFinishErrorDetail",
		"viewParams",
		"shortage_g",
	} {
		combined := contents["topNav"] + contents["overviewView"] + contents["planView"] + contents["runningView"] + contents["stockView"] + contents["wipView"] + contents["workstationLib"] + contents["planLib"] + contents["runningLib"] + contents["urlState"]
		if !strings.Contains(combined, marker) {
			t.Fatalf("frontend missing %s", marker)
		}
	}
	for _, marker := range []string{
		"PR-496-PRODUCTION-FLOW-PHASE1-OPTIMIZATION",
		"步骤条",
		"完成面板",
		"WIP 上下文",
		"today_summary",
		"nav_badges",
		"readiness",
	} {
		combined := contents["requirements"] + contents["acceptance"] + contents["manual"] + contents["evidence"]
		if !strings.Contains(combined, marker) {
			t.Fatalf("docs missing %s", marker)
		}
	}
}
