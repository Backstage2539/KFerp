package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev491ProductionDemandStatusJobCardContextContracts(t *testing.T) {
	files := map[string]string{
		"reqStore":            filepath.Join("internal", "interfaces", "http", "support", "req_store.go"),
		"producePlanView":     filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue"),
		"jobCardsView":        filepath.Join("frontend-vue-shell", "src", "views", "JobCardsView.vue"),
		"producePlanLib":      filepath.Join("frontend-vue-shell", "src", "lib", "produce-plan.js"),
		"producePlanTest":     filepath.Join("frontend-vue-shell", "src", "lib", "produce-plan.test.js"),
		"workOrdersTest":      filepath.Join("frontend-vue-shell", "src", "lib", "work-orders.test.js"),
		"unprodSummaryAPI":    filepath.Join("internal", "interfaces", "http", "production", "unprod_summary_page.go"),
		"unprodSummaryRepo":   filepath.Join("internal", "infrastructure", "postgres", "production", "plan_queries.go"),
		"unprodSummarySource": filepath.Join("internal", "infrastructure", "postgres", "production", "unprod_summary.go"),
		"workOrderRepository": filepath.Join("internal", "infrastructure", "postgres", "production", "work_order.go"),
		"requirements":        filepath.Join("docs", "REQUIREMENTS.md"),
		"acceptance":          filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		"manual":              filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"),
		"acceptanceEvidence":  filepath.Join("docs", "acceptance", "2026-06-13-production-demand-status-jobcard-context.md"),
	}
	contents := map[string]string{}
	for key, path := range files {
		contents[key] = string(readOrderAppFileForTest(t, path))
	}
	for _, marker := range []string{
		"PR-491-PRODUCTION-DEMAND-STATUS-JOBCARD-CONTEXT",
		"DEV-491-DEMAND-STATUS-FILTER",
		"DEV-491-DEMAND-PLAN-GUARD",
		"DEV-491-NESTED-SCROLL-PROPAGATION",
		"DEV-491-JOB-CARD-CONTEXT-DRAWER",
		"REV-491-PRODUCTION-DEMAND-STATUS-JOBCARD-CONTEXT",
	} {
		if !strings.Contains(contents["reqStore"], marker) {
			t.Fatalf("req_store.go missing %s", marker)
		}
	}
	for _, marker := range []string{
		"demandStatusFilter",
		"demand_status",
		"productionDemandSelectable",
		"productionDemandStatusLabel",
		"已进入生产计划的需求不可重复生成计划",
		"overscroll-behavior: auto",
	} {
		if !strings.Contains(contents["producePlanView"], marker) {
			t.Fatalf("ProducePlanView.vue missing %s", marker)
		}
	}
	if strings.Contains(contents["producePlanView"], "overscroll-behavior: contain") {
		t.Fatal("ProducePlanView.vue must release nested scroll at boundaries, not contain overscroll")
	}
	for _, marker := range []string{
		"productionDemandStatusLabel",
		"productionDemandSelectable",
		"buildProductionDemandSummaryQuery",
	} {
		if !strings.Contains(contents["producePlanLib"], marker) || !strings.Contains(contents["producePlanTest"], marker) {
			t.Fatalf("production demand helper/test missing %s", marker)
		}
	}
	for _, marker := range []string{
		"DemandStatus",
		"demand_status",
	} {
		if !strings.Contains(contents["unprodSummaryAPI"], marker) || !strings.Contains(contents["unprodSummaryRepo"], marker) {
			t.Fatalf("unproduced summary API/repo missing %s", marker)
		}
	}
	for _, marker := range []string{
		"productionDemandStatusByKey",
		"production_plan_items",
		"work_orders",
		"demand_status_label",
		"demand_selectable",
	} {
		if !strings.Contains(contents["unprodSummaryRepo"]+contents["unprodSummarySource"], marker) {
			t.Fatalf("unproduced demand status source missing %s", marker)
		}
	}
	for _, marker := range []string{
		"work_order_no",
		"product_name",
		"material_snapshot",
		"bom_version_id",
	} {
		if !strings.Contains(contents["workOrderRepository"], marker) {
			t.Fatalf("work_order.go missing job card context marker %s", marker)
		}
	}
	for _, marker := range []string{
		"openExecutionHub",
		"openWorkstation",
		"BOM/配方",
		"工序要求",
		"进入工位",
	} {
		if !strings.Contains(contents["jobCardsView"], marker) || !strings.Contains(contents["workOrdersTest"], marker) {
			t.Fatalf("job card context UI/test missing %s", marker)
		}
	}
	if strings.Contains(contents["jobCardsView"], "job-card-work-order-drawer") {
		t.Fatal("JobCardsView must use the shared execution hub instead of a duplicate work-order drawer")
	}
	for _, key := range []string{"requirements", "acceptance", "manual", "acceptanceEvidence"} {
		if !strings.Contains(contents[key], "PR-491-PRODUCTION-DEMAND-STATUS-JOBCARD-CONTEXT") {
			t.Fatalf("%s missing PR-491 marker", key)
		}
	}
}
