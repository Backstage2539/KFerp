package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPR657ProductionRosterAutoDispatchDeliveryContract(t *testing.T) {
	files := map[string]string{
		"store":        filepath.Join("internal", "interfaces", "http", "support", "req_store.go"),
		"roster":       filepath.Join("internal", "application", "production", "production_roster.go"),
		"replan":       filepath.Join("internal", "application", "production", "production_replan.go"),
		"schedule":     filepath.Join("frontend-vue-shell", "src", "views", "ProductionScheduleView.vue"),
		"workstation":  filepath.Join("frontend-vue-shell", "src", "views", "WorkstationView.vue"),
		"replan_ui":    filepath.Join("frontend-vue-shell", "src", "components", "ProductionReplanWorkspace.vue"),
		"requirements": filepath.Join("docs", "REQUIREMENTS.md"),
		"acceptance":   filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		"manual":       filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"),
		"evidence":     filepath.Join("docs", "acceptance", "2026-09-13-production-roster-auto-dispatch.md"),
	}
	contents := map[string]string{}
	for key, path := range files {
		contents[key] = string(readOrderAppFileForTest(t, path))
	}
	for _, marker := range []string{"PR-657-PRODUCTION-ROSTER-AUTO-DISPATCH", "DEV-657-PRODUCTION-ROSTER-DATA", "DEV-658-PRODUCTION-ROSTER-UI", "DEV-659-PRODUCTION-AUTO-DISPATCH-HANDOVER", "DEV-660-PRODUCTION-DEMAND-REPLAN", "REV-657-PRODUCTION-ROSTER-AUTO-DISPATCH"} {
		if !strings.Contains(contents["store"], marker) {
			t.Fatalf("req_store.go missing %s", marker)
		}
	}
	for _, marker := range []string{"未排班", "ResolveWorkstationOwner", "ExpectedVersion", "HandoverWorkstation"} {
		if !strings.Contains(contents["roster"], marker) {
			t.Fatalf("roster contract missing %s", marker)
		}
	}
	for _, marker := range []string{"撤回并重新安排", "共用上游", "我的今日工位", "工位安排", "恢复自动安排"} {
		joined := contents["manual"] + contents["schedule"] + contents["workstation"] + contents["replan_ui"]
		if !strings.Contains(joined, marker) {
			t.Fatalf("product flow missing %s", marker)
		}
	}
	for _, key := range []string{"requirements", "acceptance", "manual", "evidence"} {
		if !strings.Contains(contents[key], "PR-657") {
			t.Fatalf("%s missing PR-657 marker", key)
		}
	}
}
