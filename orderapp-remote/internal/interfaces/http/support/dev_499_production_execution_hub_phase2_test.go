package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev499ProductionExecutionHubPhase2Contracts(t *testing.T) {
	files := map[string]string{
		"reqStore":        filepath.Join("internal", "interfaces", "http", "support", "req_store.go"),
		"service":         filepath.Join("internal", "application", "production", "service.go"),
		"overviewView":    filepath.Join("frontend-vue-shell", "src", "views", "ProductionOverviewView.vue"),
		"workstationView": filepath.Join("frontend-vue-shell", "src", "views", "WorkstationView.vue"),
		"workOrdersView":  filepath.Join("frontend-vue-shell", "src", "views", "WorkOrdersView.vue"),
		"jobCardsView":    filepath.Join("frontend-vue-shell", "src", "views", "JobCardsView.vue"),
		"appShell":        filepath.Join("frontend-vue-shell", "src", "App.vue"),
		"hubDrawer":       filepath.Join("frontend-vue-shell", "src", "components", "ProductionExecutionHubDrawer.vue"),
		"hubLib":          filepath.Join("frontend-vue-shell", "src", "lib", "production-execution-hub.js"),
		"requirements":    filepath.Join("docs", "REQUIREMENTS.md"),
		"acceptance":      filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		"manual":          filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"),
		"evidence":        filepath.Join("docs", "acceptance", "2026-06-21-production-execution-hub-phase2.md"),
	}
	contents := map[string]string{}
	for key, path := range files {
		contents[key] = string(readOrderAppFileForTest(t, path))
	}
	for _, marker := range []string{
		"PR-499-PRODUCTION-EXECUTION-HUB-PHASE2",
		"DEV-499-WORKORDER-EXECUTION-HUB",
		"DEV-499-TASK-READINESS-READMODEL",
		"DEV-499-PRODUCTION-HUB-LINKS",
		"DEV-499-WORKSTATION-LOAD-TIMELINE",
		"UT-499-PRODUCTION-EXECUTION-HUB-PHASE2",
		"API-499-PRODUCTION-EXECUTION-HUB-PHASE2",
		"REV-499-PRODUCTION-EXECUTION-HUB-PHASE2",
	} {
		if !strings.Contains(contents["reqStore"], marker) {
			t.Fatalf("req_store.go missing %s", marker)
		}
	}
	for _, marker := range []string{
		"WorkOrderExecutionHub",
		"ProductionExecutionReadiness",
		"ExecutionHub",
		"ReadinessDetail",
		"LoadStatus",
		"filterFinishedReceiptEntries",
	} {
		if !strings.Contains(contents["service"], marker) {
			t.Fatalf("production service missing %s", marker)
		}
	}
	frontend := contents["overviewView"] + contents["workstationView"] + contents["workOrdersView"] + contents["jobCardsView"] + contents["appShell"] + contents["hubDrawer"] + contents["hubLib"]
	for _, marker := range []string{
		"ProductionExecutionHubDrawer",
		"openExecutionHub",
		"productionContextParams",
		"buildExecutionHubActions",
		"executionHubTimelineFilters",
		"filterExecutionHubTimeline",
		"viewParams",
		"work_order_id",
		"running_item_id",
		"shortage_g",
	} {
		if !strings.Contains(frontend, marker) {
			t.Fatalf("frontend missing %s", marker)
		}
	}
	docs := contents["requirements"] + contents["acceptance"] + contents["manual"] + contents["evidence"]
	for _, marker := range []string{
		"PR-499-PRODUCTION-EXECUTION-HUB-PHASE2",
		"工单执行枢纽",
		"execution_hub",
		"readiness",
		"can_start",
		"blocking_reasons",
		"next_handler",
		"trace_timeline",
	} {
		if !strings.Contains(docs, marker) {
			t.Fatalf("docs missing %s", marker)
		}
	}
}
