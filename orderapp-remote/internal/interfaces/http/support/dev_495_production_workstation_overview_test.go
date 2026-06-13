package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev495ProductionWorkstationOverviewContracts(t *testing.T) {
	files := map[string]string{
		"reqStore":        filepath.Join("internal", "interfaces", "http", "support", "req_store.go"),
		"service":         filepath.Join("internal", "application", "production", "service.go"),
		"api":             filepath.Join("internal", "interfaces", "http", "production", "production_workstation_api.go"),
		"postgres":        filepath.Join("internal", "infrastructure", "postgres", "production", "work_order.go"),
		"topNav":          filepath.Join("frontend-vue-shell", "src", "components", "ProductionTopNav.vue"),
		"overviewView":    filepath.Join("frontend-vue-shell", "src", "views", "ProductionOverviewView.vue"),
		"workstationView": filepath.Join("frontend-vue-shell", "src", "views", "WorkstationView.vue"),
		"helper":          filepath.Join("frontend-vue-shell", "src", "lib", "production-workstation.js"),
		"menu":            filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js"),
		"requirements":    filepath.Join("docs", "REQUIREMENTS.md"),
		"acceptance":      filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		"manual":          filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"),
		"evidence":        filepath.Join("docs", "acceptance", "2026-06-13-production-workstation-overview.md"),
	}
	contents := map[string]string{}
	for key, path := range files {
		contents[key] = string(readOrderAppFileForTest(t, path))
	}
	for _, marker := range []string{
		"PR-495-PRODUCTION-WORKSTATION-OVERVIEW",
		"DEV-495-PRODUCTION-TOP-NAV",
		"DEV-495-WORKSTATION-READMODEL-ACTIONS",
		"UT-495-PRODUCTION-WORKSTATION-OVERVIEW",
		"API-495-PRODUCTION-WORKSTATION-OVERVIEW",
		"REV-495-PRODUCTION-WORKSTATION-OVERVIEW",
	} {
		if !strings.Contains(contents["reqStore"], marker) {
			t.Fatalf("req_store.go missing %s", marker)
		}
	}
	for _, marker := range []string{
		"ProductionWorkstationOverview",
		"ProductionWorkstationLoad",
		"BlockingReason",
		"AvailableActions",
	} {
		if !strings.Contains(contents["service"], marker) {
			t.Fatalf("production service missing %s", marker)
		}
	}
	for _, marker := range []string{
		"/api/production/workstation-overview",
		"/api/production/workstation/tasks/:id/exception",
		"/api/production/workstation/tasks/:id/material-call",
	} {
		if !strings.Contains(contents["api"], marker) {
			t.Fatalf("production workstation API missing %s", marker)
		}
	}
	for _, marker := range []string{
		"planned_start_at",
		"assigned_to",
		"priority",
		"work_center",
	} {
		if !strings.Contains(contents["postgres"], marker) {
			t.Fatalf("postgres production list query missing %s", marker)
		}
	}
	for _, marker := range []string{
		"productionOverview",
		"workstationView",
		"ProductionTopNav",
		"workstationTaskSections",
		"partial_finish",
		"不能做原因",
		"今日整体进度",
	} {
		if !strings.Contains(contents["topNav"]+contents["overviewView"]+contents["workstationView"]+contents["helper"]+contents["menu"]+contents["manual"]+contents["requirements"]+contents["acceptance"]+contents["evidence"], marker) {
			t.Fatalf("frontend/docs missing %s", marker)
		}
	}
}
