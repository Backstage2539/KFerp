package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev487ProductionPlanCapacitySplitContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-514-WORKSTATION-COST-COMPONENTS",
			"DEV-514-WORKSTATION-COST-COMPONENTS",
			"DEV-514-CAPACITY-BATCH-TIME-ONLY",
			"DEV-514-PLAN-SPLIT-DERIVED-OPERATION-COST",
			"DEV-514-ROUTE-TEMPLATE-ONLY",
			"API-514-WORKSTATION-COST-COMPONENTS",
			"PR-487-PRODUCTION-PLAN-CAPACITY-SPLITS",
			"DEV-487-ROUTE-SEQUENCE-ONLY",
			"DEV-487-PRODUCTION-PLAN-SPLITS",
			"DEV-487-JOBCARD-FREEZE",
		},
		filepath.Join("internal", "infrastructure", "postgres", "manufacturing", "schema.go"): {
			"manufacturing_workstation_capacities",
		},
		filepath.Join("internal", "interfaces", "http", "manufacturing", "api.go"): {
			"/api/manufacturing-workstation-capacities",
		},
		filepath.Join("internal", "infrastructure", "postgres", "production", "schema.go"): {
			"production_plan_operation_splits",
			"planned_qty_g",
			"planned_operation_cost",
		},
		filepath.Join("internal", "infrastructure", "postgres", "production", "production_plan.go"): {
			"SaveProductionPlanOperationSplits",
			"plannedCapacitySplitMetrics",
			"validateProductionPlanOperationSplitCoverage",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProcessTemplatesView.vue"): {
			"路线工序",
			"工序名称快照",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ManufacturingOperationsView.vue"): {
			"工序不决定工时",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ManufacturingWorkstationsView.vue"): {
			"工位产能",
			"机器成本/小时",
			"人工成本/小时",
			"其他成本/小时",
			"小时成本合计",
			"继承工位小时成本",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue"): {
			"工序产能拆分",
			"添加拆分",
			"saveCurrentPlanOperationSplits",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-487-PRODUCTION-PLAN-CAPACITY-SPLITS",
			"工艺路线只定义工序顺序",
			"工位产能",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-487-PRODUCTION-PLAN-CAPACITY-SPLITS",
			"布勒 18kg",
			"计划工序成本",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-487-PRODUCTION-PLAN-CAPACITY-SPLITS",
			"工位产能",
			"标准分钟/批",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-487 marker %q", rel, want)
			}
		}
	}
	routeSource := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProcessTemplatesView.vue")))
	for _, forbidden := range []string{"工位产能", "workstation_capacity_id", "planned_operation_cost", "自动折算计划工序成本"} {
		if strings.Contains(routeSource, forbidden) {
			t.Fatalf("process route page must not own production plan capacity split field %q", forbidden)
		}
	}
}
