package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev651ProductionPlanCapacityWorkspaceContracts(t *testing.T) {
	files := map[string]string{
		"store":        filepath.Join("internal", "interfaces", "http", "support", "req_store.go"),
		"service":      filepath.Join("internal", "application", "production", "service.go"),
		"repo":         filepath.Join("internal", "infrastructure", "postgres", "production", "production_plan.go"),
		"view":         filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue"),
		"component":    filepath.Join("frontend-vue-shell", "src", "components", "ProductionPlanCapacityWorkspace.vue"),
		"lib":          filepath.Join("frontend-vue-shell", "src", "lib", "produce-plan.js"),
		"requirements": filepath.Join("docs", "REQUIREMENTS.md"),
		"acceptance":   filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		"manual":       filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"),
		"evidence":     filepath.Join("docs", "acceptance", "2026-09-11-production-plan-capacity-workspace.md"),
	}
	contents := map[string]string{}
	for key, path := range files {
		contents[key] = string(readOrderAppFileForTest(t, path))
	}
	for _, marker := range []string{
		"PR-651-PRODUCTION-PLAN-CAPACITY-WORKSPACE",
		"DEV-651-OPERATION-GROUPING",
		"DEV-651-TASK-COVERAGE",
		"DEV-651-CAPACITY-WORKSPACE",
		"DEV-651-DEVELOPMENT-DELIVERY",
	} {
		if !strings.Contains(contents["store"], marker) {
			t.Fatalf("req_store.go missing %s", marker)
		}
	}
	for _, marker := range []string{
		`code: "DEV-651-OPERATION-GROUPING", title: "按计划工艺快照中的实际工序名称分组展示", status: "done"`,
		`code: "DEV-651-TASK-COVERAGE", title: "按重量或销售件数返回逐任务产能覆盖并保留来源订单", status: "done"`,
		`code: "DEV-651-CAPACITY-WORKSPACE", title: "全页产能拆分、实时核对、草稿保存与安排确认", status: "done"`,
		`code: "DEV-651-DEVELOPMENT-DELIVERY", title: "操作手册、验证、设计核对、开发部署与截图", status: "done"`,
	} {
		if !strings.Contains(contents["store"], marker) {
			t.Fatalf("req_store.go missing completed DEV-651 marker %s", marker)
		}
	}
	for _, marker := range []string{"required_qty", "arranged_qty", "diff_qty", "unit"} {
		if !strings.Contains(contents["service"], marker) {
			t.Fatalf("native operation coverage response missing %s", marker)
		}
	}
	for _, marker := range []string{"opRequiredQty", "opArrangedQty", "opUnit", "productionPlanItemSalesUnit"} {
		if !strings.Contains(contents["repo"], marker) {
			t.Fatalf("native operation coverage calculation missing %s", marker)
		}
	}
	for _, marker := range []string{
		"ProductionPlanCapacityWorkspace",
		"buildProductionPlanCapacityGroups",
		"productionPlanCapacityReadiness",
		"group.operation",
		"来源订单",
		"保存草稿",
		"确认安排",
	} {
		if !strings.Contains(contents["view"]+contents["component"]+contents["lib"], marker) {
			t.Fatalf("capacity workspace missing %s", marker)
		}
	}
	if strings.Contains(contents["component"], "烘焙熟豆") || strings.Contains(contents["component"], "包装成品") {
		t.Fatal("capacity workspace must not hard-code production stage names")
	}
	for _, key := range []string{"requirements", "acceptance", "manual", "evidence"} {
		if !strings.Contains(contents[key], "PR-651") {
			t.Fatalf("%s missing PR-651 marker", key)
		}
	}
}
