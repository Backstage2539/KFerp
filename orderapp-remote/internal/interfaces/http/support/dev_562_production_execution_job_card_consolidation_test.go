package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev562ProductionExecutionJobCardConsolidationContracts(t *testing.T) {
	files := map[string]string{
		"reqStore":     filepath.Join("internal", "interfaces", "http", "support", "req_store.go"),
		"workOrders":   filepath.Join("frontend-vue-shell", "src", "views", "WorkOrdersView.vue"),
		"jobCards":     filepath.Join("frontend-vue-shell", "src", "views", "JobCardsView.vue"),
		"workstation":  filepath.Join("frontend-vue-shell", "src", "views", "WorkstationView.vue"),
		"hub":          filepath.Join("frontend-vue-shell", "src", "components", "ProductionExecutionHubDrawer.vue"),
		"requirements": filepath.Join("docs", "REQUIREMENTS.md"),
		"acceptance":   filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		"manual":       filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"),
		"evidence":     filepath.Join("docs", "acceptance", "2026-07-29-workstation-job-card-execution.md"),
	}
	contents := map[string]string{}
	for key, rel := range files {
		contents[key] = string(readOrderAppFileForTest(t, rel))
	}

	for _, marker := range []string{
		"PR-562-PRODUCTION-EXECUTION-JOB-CARD-CONSOLIDATION",
		"DEV-562-WORKORDER-EXECUTION-HUB-COMMAND",
		"DEV-562-JOB-CARD-READONLY-PROJECTION",
		"DEV-562-WORKSTATION-STATE-ACTUALS",
		"DEV-562-DOCS-DELIVERY",
		"REV-562-PRODUCTION-EXECUTION-JOB-CARD-CONSOLIDATION",
	} {
		if !strings.Contains(contents["reqStore"], marker) {
			t.Fatalf("req_store missing %s", marker)
		}
	}

	workOrderTemplate := beforeScript(contents["workOrders"])
	for _, marker := range []string{"执行枢纽", "编辑拆分", "打印", `@updated="load"`} {
		if !strings.Contains(contents["workOrders"], marker) {
			t.Fatalf("WorkOrdersView missing %s", marker)
		}
	}
	for _, forbidden := range []string{"startWorkOrder(row)", `openStockDocument(row, 'finish')`} {
		if strings.Contains(workOrderTemplate, forbidden) {
			t.Fatalf("WorkOrdersView list must not expose %s", forbidden)
		}
	}

	jobCardTemplate := beforeScript(contents["jobCards"])
	for _, marker := range []string{"工序要求", "进入工位", "执行枢纽", "process_requirement", "按冻结工艺路线执行"} {
		if !strings.Contains(contents["jobCards"], marker) {
			t.Fatalf("JobCardsView missing %s", marker)
		}
	}
	for _, forbidden := range []string{"<input", "保存实际", "runJobCardAction", "saveActuals"} {
		if strings.Contains(jobCardTemplate, forbidden) || strings.Contains(contents["jobCards"], forbidden) {
			t.Fatalf("JobCardsView must be read-only and omit %s", forbidden)
		}
	}

	for _, marker := range []string{"workstationVisibleActions", "actual_minutes", "actual_input_qty", "actual_output_qty", "leftover_qty", "loss_reason", "exception_reason"} {
		if !strings.Contains(contents["workstation"], marker) {
			t.Fatalf("WorkstationView missing %s", marker)
		}
	}
	for _, marker := range []string{"action_type", "command", "apiSend", "actionBusyKey", "updated"} {
		if !strings.Contains(contents["hub"], marker) {
			t.Fatalf("ExecutionHub missing %s", marker)
		}
	}

	for _, key := range []string{"requirements", "acceptance", "manual", "evidence"} {
		if !strings.Contains(contents[key], "PR-562-PRODUCTION-EXECUTION-JOB-CARD-CONSOLIDATION") {
			t.Fatalf("%s missing PR-562 marker", key)
		}
	}
}

func beforeScript(source string) string {
	if idx := strings.Index(source, "<script setup>"); idx >= 0 {
		return source[:idx]
	}
	return source
}
