package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev492ProductionDemandAddonOrderSplitContracts(t *testing.T) {
	files := map[string]string{
		"reqStore":           filepath.Join("internal", "interfaces", "http", "support", "req_store.go"),
		"producePlanView":    filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue"),
		"producePlanTest":    filepath.Join("frontend-vue-shell", "src", "lib", "produce-plan.test.js"),
		"planQueries":        filepath.Join("internal", "infrastructure", "postgres", "production", "plan_queries.go"),
		"planQueriesTest":    filepath.Join("internal", "infrastructure", "postgres", "production", "plan_queries_add_on_test.go"),
		"producePlanAPITest": filepath.Join("internal", "interfaces", "http", "production", "produce_plan_api_test.go"),
		"requirements":       filepath.Join("docs", "REQUIREMENTS.md"),
		"acceptance":         filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		"manual":             filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"),
		"evidence":           filepath.Join("docs", "acceptance", "2026-06-13-production-demand-addon-order-split.md"),
	}
	contents := map[string]string{}
	for key, path := range files {
		contents[key] = string(readOrderAppFileForTest(t, path))
	}
	for _, marker := range []string{
		"PR-492-PRODUCTION-DEMAND-ADDON-ORDER-SPLIT",
		"DEV-492-DEMAND-ORDER-SPLIT",
		"DEV-492-DEMAND-SELECTION-KEY",
		"REV-492-PRODUCTION-DEMAND-ADDON-ORDER-SPLIT",
	} {
		if !strings.Contains(contents["reqStore"], marker) {
			t.Fatalf("req_store.go missing %s", marker)
		}
	}
	for _, marker := range []string{
		"splitUnproducedNeedsByProductionPlan",
		"fetchProductionDemandParts",
		"splitProductionDemandRowByParts",
		"calcProductionDemandGap",
		"source_need",
	} {
		if !strings.Contains(contents["planQueries"], marker) {
			t.Fatalf("plan_queries.go missing %s", marker)
		}
	}
	for _, marker := range []string{
		"TestSplitProductionDemandRowByPartsKeepsAddOnSelectable",
		"SO-NEW",
		"in_production",
		"unplanned",
	} {
		if !strings.Contains(contents["planQueriesTest"], marker) {
			t.Fatalf("plan_queries_add_on_test.go missing %s", marker)
		}
	}
	for _, marker := range []string{
		"TestProducePlanSummaryAPILeavesAddOnOrdersSelectableWhenOlderOrdersPlanned",
		"SO-ADD-OLD",
		"SO-ADD-NEW",
	} {
		if !strings.Contains(contents["producePlanAPITest"], marker) {
			t.Fatalf("produce_plan_api_test.go missing %s", marker)
		}
	}
	for _, marker := range []string{
		"productionDemandSelectionKey",
		"isProductionDemandSelected",
		"row.demand_status || 'unplanned'",
	} {
		if !strings.Contains(contents["producePlanView"]+contents["producePlanTest"], marker) {
			t.Fatalf("frontend demand add-on key split missing %s", marker)
		}
	}
	if strings.Contains(contents["producePlanView"], "selected[rowKey(row)]") {
		t.Fatal("ProducePlanView.vue must not use rowKey as the production demand selection key")
	}
	for _, key := range []string{"requirements", "acceptance", "manual", "evidence"} {
		if !strings.Contains(contents[key], "PR-492-PRODUCTION-DEMAND-ADDON-ORDER-SPLIT") {
			t.Fatalf("%s missing PR-492 marker", key)
		}
	}
}
