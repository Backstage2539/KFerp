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
		filepath.Join("frontend-vue-shell", "src", "views", "ManufacturingOperationsView.vue"): {
			"standard_operation_cost",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ManufacturingWorkstationsView.vue"): {
			"form.applicable_operation_ids",
			"适用工序",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-517-OPERATION-STANDARD-COST-MASTER",
			"PR-518 后新业务不再把",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-517-OPERATION-STANDARD-COST-MASTER",
			"工位/设备",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"标准工序成本",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"工序列表不再作为标准工序成本录入口",
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
