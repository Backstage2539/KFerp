package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev493PlanWorkOrderSplitEditContracts(t *testing.T) {
	files := map[string]string{
		"reqStore":        filepath.Join("internal", "interfaces", "http", "support", "req_store.go"),
		"workOrderRepo":   filepath.Join("internal", "infrastructure", "postgres", "production", "production_plan.go"),
		"workOrderSource": filepath.Join("internal", "infrastructure", "postgres", "production", "work_order.go"),
		"workOrderAPI":    filepath.Join("internal", "interfaces", "http", "production", "work_order_api.go"),
		"producePlanView": filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue"),
		"workOrdersView":  filepath.Join("frontend-vue-shell", "src", "views", "WorkOrdersView.vue"),
		"workOrdersLib":   filepath.Join("frontend-vue-shell", "src", "lib", "work-orders.js"),
		"requirements":    filepath.Join("docs", "REQUIREMENTS.md"),
		"acceptance":      filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		"manual":          filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"),
		"evidence":        filepath.Join("docs", "acceptance", "2026-06-13-plan-workorder-split-edit.md"),
	}
	contents := map[string]string{}
	for key, path := range files {
		contents[key] = string(readOrderAppFileForTest(t, path))
	}
	for _, marker := range []string{
		"PR-493-PLAN-WORKORDER-SPLIT-EDIT",
		"DEV-493-ROUTE-OPERATION-MASTER-NAME",
		"DEV-493-DRAFT-PLAN-SPLIT-EDITOR",
		"DEV-493-RELEASED-WORKORDER-SPLIT-EDITOR",
		"REV-493-PLAN-WORKORDER-SPLIT-EDIT",
	} {
		if !strings.Contains(contents["reqStore"], marker) {
			t.Fatalf("req_store.go missing %s", marker)
		}
	}
	for _, marker := range []string{
		"manufacturing_operations mo",
		"COALESCE(NULLIF(mo.name,''), pro.operation)",
	} {
		if !strings.Contains(contents["workOrderSource"], marker) {
			t.Fatalf("work_order.go missing latest operation-name marker %s", marker)
		}
	}
	for _, marker := range []string{
		"SaveWorkOrderOperationSplits",
		"work order must be released to edit operation splits",
		"status<>'pending'",
		"createPendingJobCardsForWorkOrderTx",
		"save_operation_splits",
	} {
		if !strings.Contains(contents["workOrderRepo"]+contents["workOrderAPI"], marker) {
			t.Fatalf("work order split backend missing %s", marker)
		}
	}
	for _, marker := range []string{
		"openProductionPlanSplitDrawer",
		"production-plan-split-drawer",
		"productionPlanSplitRows",
		"编辑拆分",
		"work-order-split-drawer",
		"buildWorkOrderOperationSplitPayload",
		"workOrderOperationSplitsEndpoint",
	} {
		if !strings.Contains(contents["producePlanView"]+contents["workOrdersView"]+contents["workOrdersLib"], marker) {
			t.Fatalf("frontend split edit marker missing %s", marker)
		}
	}
	for _, key := range []string{"requirements", "acceptance", "manual", "evidence"} {
		if !strings.Contains(contents[key], "PR-493-PLAN-WORKORDER-SPLIT-EDIT") {
			t.Fatalf("%s missing PR-493 marker", key)
		}
	}
}
