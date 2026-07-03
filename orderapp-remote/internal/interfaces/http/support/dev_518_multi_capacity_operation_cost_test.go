package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev518MultiCapacityOperationCostContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-518-MULTI-CAPACITY-OPERATION-COST",
			"DEV-518-ROUTE-STANDARD-COST-CAPACITY",
			"DEV-518-BOM-OPERATION-COST-SNAPSHOT",
			"DEV-518-PRICING-BOM-SNAPSHOT-COST",
			"API-518-MULTI-CAPACITY-OPERATION-COST",
		},
		filepath.Join("internal", "application", "manufacturing", "service.go"): {
			"applyStandardCostCapacitySnapshot",
			"StandardCostCapacityID",
			"standard_cost_capacity_id",
		},
		filepath.Join("internal", "infrastructure", "postgres", "manufacturing", "repository.go"): {
			"standard_cost_capacity_id",
			"sc.id=pro.standard_cost_capacity_id",
			"请为工艺路线工序设置标准成本产能档",
		},
		filepath.Join("internal", "infrastructure", "postgres", "bom", "schema.go"): {
			"production_bom_version_operation_costs",
			"operation_unit_cost",
			"operation_cost_unit",
		},
		filepath.Join("internal", "infrastructure", "postgres", "bom", "repository.go"): {
			"refreshProductionBomVersionOperationCostSnapshotsTx",
			"工艺路线工序缺少标准成本产能档",
			"operation_cost_snapshot_count",
		},
		filepath.Join("internal", "infrastructure", "postgres", "costing", "repository.go"): {
			"production_bom_version_operation_costs",
			"bom_operation_snapshot",
			"bom_operation_snapshot_missing",
		},
		filepath.Join("internal", "application", "costing", "service.go"): {
			"pricingRuleTrialBomOperationSnapshotMissingSource",
			"请先发布包含标准成本产能档快照的 BOM",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProcessTemplatesView.vue"): {
			"标准成本产能档",
			"/api/manufacturing-workstation-capacities?status=active",
			"只用于 BOM/价格标准成本",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"BOM工序成本快照",
			"BOM工序成本快照缺失",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-518-MULTI-CAPACITY-OPERATION-COST",
			"发布 BOM 版本时冻结工序成本快照",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-518-MULTI-CAPACITY-OPERATION-COST",
			"请先发布包含标准成本产能档快照的 BOM",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"标准成本产能档",
			"不代表生产计划实际排产",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"BOM工序成本快照",
			"没有冻结工序成本快照",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-518 marker %q", rel, want)
			}
		}
	}
}
