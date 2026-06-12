package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev486WorkstationCapacityRouteCostContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-486-WORKSTATION-CAPACITY-ROUTE-COST",
			"DEV-486-WORKSTATION-CAPACITY-MASTER",
			"DEV-486-ROUTE-OPERATION-COST-SNAPSHOT",
			"DEV-486-JOB-CARD-TIME-COST-FREEZE",
		},
		filepath.Join("internal", "infrastructure", "postgres", "manufacturing", "schema.go"): {
			"manufacturing_workstation_capacities",
			"workstation_capacity_id",
			"planned_operation_cost",
		},
		filepath.Join("internal", "interfaces", "http", "manufacturing", "api.go"): {
			"/api/manufacturing-workstation-capacities",
		},
		filepath.Join("internal", "infrastructure", "postgres", "production", "work_order.go"): {
			"plannedJobCardOperationCost",
			"actual_operation_cost",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProcessTemplatesView.vue"): {
			"工位产能",
			"标准分钟/批",
			"计划工序成本",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ManufacturingOperationsView.vue"): {
			"工序不决定工时",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ManufacturingWorkstationsView.vue"): {
			"工位产能",
			"默认小时费率",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-486-WORKSTATION-CAPACITY-ROUTE-COST",
			"工艺路线工序行",
			"工位产能",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-486-WORKSTATION-CAPACITY-ROUTE-COST",
			"布勒 18kg",
			"计划工序成本",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-486-WORKSTATION-CAPACITY-ROUTE-COST",
			"工位产能",
			"标准分钟/批",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-486 marker %q", rel, want)
			}
		}
	}
}
