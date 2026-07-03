package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev494CapacityOperationAutoSplitContracts(t *testing.T) {
	files := map[string]string{
		"reqStore":            filepath.Join("internal", "interfaces", "http", "support", "req_store.go"),
		"manufacturing":       filepath.Join("internal", "application", "manufacturing", "service.go"),
		"manufacturingAPI":    filepath.Join("internal", "interfaces", "http", "manufacturing", "api.go"),
		"manufacturingRepo":   filepath.Join("internal", "infrastructure", "postgres", "manufacturing", "repository.go"),
		"manufacturingSchema": filepath.Join("internal", "infrastructure", "postgres", "manufacturing", "schema.go"),
		"productionPlan":      filepath.Join("internal", "infrastructure", "postgres", "production", "production_plan.go"),
		"producePlanLib":      filepath.Join("frontend-vue-shell", "src", "lib", "produce-plan.js"),
		"producePlanView":     filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue"),
		"workOrdersView":      filepath.Join("frontend-vue-shell", "src", "views", "WorkOrdersView.vue"),
		"workstationsView":    filepath.Join("frontend-vue-shell", "src", "views", "ManufacturingWorkstationsView.vue"),
		"requirements":        filepath.Join("docs", "REQUIREMENTS.md"),
		"acceptance":          filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		"manual":              filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"),
		"evidence":            filepath.Join("docs", "acceptance", "2026-06-13-capacity-operation-auto-split.md"),
	}
	contents := map[string]string{}
	for key, path := range files {
		contents[key] = string(readOrderAppFileForTest(t, path))
	}
	for _, marker := range []string{
		"PR-494-CAPACITY-OPERATION-AUTO-SPLIT",
		"DEV-494-CAPACITY-APPLICABLE-OPERATIONS",
		"DEV-494-PLAN-WORKORDER-AUTO-SPLIT",
		"DEV-494-COUNT-UNIT-SPLIT",
		"REV-494-CAPACITY-OPERATION-AUTO-SPLIT",
	} {
		if !strings.Contains(contents["reqStore"], marker) {
			t.Fatalf("req_store.go missing %s", marker)
		}
	}
	for _, marker := range []string{
		"ApplicableOperationIDs",
		"applicable_operation_ids",
		"manufacturing_workstation_capacity_operations",
		"attachWorkstationCapacityOperations",
	} {
		if !strings.Contains(contents["manufacturing"]+contents["manufacturingAPI"]+contents["manufacturingRepo"]+contents["manufacturingSchema"], marker) {
			t.Fatalf("manufacturing capacity applicable operation support missing %s", marker)
		}
	}
	for _, marker := range []string{
		"buildOperationCapacityAutoSplits",
		"maxAssignableQtyForCapacitySplit",
		"qtyFromGForCapacityUnit",
		"applicableOperationCapacities",
		"autoSplitCurrentPlanOperation",
		"autoSplitProductionPlanDrawerOperation",
		"autoSplitWorkOrderOperation",
		"自动拆分",
	} {
		if !strings.Contains(contents["producePlanLib"]+contents["producePlanView"]+contents["workOrdersView"], marker) {
			t.Fatalf("frontend auto split support missing %s", marker)
		}
	}
	for _, marker := range []string{
		"assignRemainingCurrentPlanSplitQty",
		"assignRemainingProductionPlanDrawerSplitQty",
		"assignRemainingWorkOrderSplitQty",
		"分配剩余产量",
		"分配剩余产能",
	} {
		if strings.Contains(contents["producePlanView"]+contents["workOrdersView"], marker) {
			t.Fatalf("obsolete assign remaining split control still present: %s", marker)
		}
	}
	for _, marker := range []string{
		"plannedCapacitySplitMetrics(split productionapp.ProductionPlanOperationSplit, specG ...int64)",
		"plannedCapacitySplitQtyG(qty float64, unit string, specG ...int64)",
		"case \"件\", \"个\", \"袋\", \"盒\", \"unit\", \"units\", \"pc\", \"pcs\":",
	} {
		if !strings.Contains(contents["productionPlan"], marker) {
			t.Fatalf("production count-unit split support missing %s", marker)
		}
	}
	for _, marker := range []string{
		"适用工序",
		"工位产能本身不再维护适用工序",
		"/api/manufacturing-operations",
	} {
		if !strings.Contains(contents["workstationsView"], marker) {
			t.Fatalf("workstation capacity UI missing %s", marker)
		}
	}
	for _, key := range []string{"requirements", "acceptance", "manual", "evidence"} {
		if !strings.Contains(contents[key], "PR-494-CAPACITY-OPERATION-AUTO-SPLIT") {
			t.Fatalf("%s missing PR-494 marker", key)
		}
	}
}
