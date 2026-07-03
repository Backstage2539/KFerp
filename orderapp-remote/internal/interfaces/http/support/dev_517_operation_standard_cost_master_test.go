package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev517OperationStandardCostMasterContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-517-OPERATION-STANDARD-COST-MASTER",
			"DEV-517-OPERATION-STANDARD-COST-MASTER",
			"DEV-517-WORKSTATION-APPLICABLE-OPERATIONS",
			"DEV-517-ROUTE-CAPACITY-REMOVAL",
			"API-517-OPERATION-STANDARD-COST-MASTER",
		},
		filepath.Join("internal", "application", "manufacturing", "service.go"): {
			"StandardOperationCost",
			"ApplicableOperationIDs",
			"cmd.ApplicableOperationIDs = nil",
		},
		filepath.Join("internal", "infrastructure", "postgres", "manufacturing", "schema.go"): {
			"standard_operation_cost",
			"manufacturing_workstation_operations",
			"INSERT INTO %[1]s.manufacturing_workstation_operations",
		},
		filepath.Join("internal", "infrastructure", "postgres", "costing", "repository.go"): {
			"standard_operation_cost",
			"operation_master",
			"per_inventory_unit",
			"标准工序成本来自工序列表",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ManufacturingOperationsView.vue"): {
			"标准工序成本",
			"standard_operation_cost",
			"元/库存单位",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ManufacturingWorkstationsView.vue"): {
			"form.applicable_operation_ids",
			"适用工序",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"成本来源",
			"工序列表",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-517-OPERATION-STANDARD-COST-MASTER",
			"工序列表维护标准工序成本",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-517-OPERATION-STANDARD-COST-MASTER",
			"工位/设备",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"工序列表",
			"标准工序成本",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"工位维护机器/人工/其他小时成本和适用工序",
			"不选择标准成本默认产能",
		},
		filepath.Join("docs", "acceptance", "2026-07-02-operation-standard-cost-master.md"): {
			"PR-517-OPERATION-STANDARD-COST-MASTER",
			"工序列表维护标准工序成本",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-517 marker %q", rel, want)
			}
		}
	}
}

func TestDev517RemovesRouteDefaultCapacityFromNewPaths(t *testing.T) {
	for rel, forbidden := range map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "views", "ProcessTemplatesView.vue"): {
			"标准成本默认产能",
			"/api/manufacturing-workstation-capacities",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ManufacturingWorkstationsView.vue"): {
			"capacityForm.applicable_operation_ids",
		},
		filepath.Join("internal", "application", "manufacturing", "service.go"): {
			"validateProcessRouteStandardCostCapacity",
		},
		filepath.Join("internal", "infrastructure", "postgres", "costing", "repository.go"): {
			"请为工艺路线工序设置标准成本默认产能",
			"sc.id = pro.standard_cost_capacity_id",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, marker := range forbidden {
			if strings.Contains(src, marker) {
				t.Fatalf("%s still contains retired PR-516 marker %q", rel, marker)
			}
		}
	}
}
