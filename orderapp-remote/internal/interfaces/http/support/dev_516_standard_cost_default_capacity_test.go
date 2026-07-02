package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev516StandardCostDefaultCapacityContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-516-STANDARD-COST-DEFAULT-CAPACITY",
			"DEV-516-ROUTE-STANDARD-CAPACITY",
			"DEV-516-PRICING-STANDARD-CAPACITY",
			"API-516-STANDARD-COST-DEFAULT-CAPACITY",
		},
		filepath.Join("internal", "application", "manufacturing", "service.go"): {
			"StandardCostCapacityID",
			"标准成本默认产能",
		},
		filepath.Join("internal", "infrastructure", "postgres", "manufacturing", "schema.go"): {
			"standard_cost_capacity_id",
		},
		filepath.Join("internal", "infrastructure", "postgres", "costing", "repository.go"): {
			"capacity_selection_source",
			"唯一匹配产能",
			"请为工艺路线工序设置标准成本默认产能",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProcessTemplatesView.vue"): {
			"标准成本默认产能",
			"standard_cost_capacity_id",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"产能来源",
			"请为工艺路线工序设置标准成本默认产能",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-516-STANDARD-COST-DEFAULT-CAPACITY",
			"标准成本默认产能",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-516-STANDARD-COST-DEFAULT-CAPACITY",
			"多条启用适用产能",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"标准成本默认产能",
			"唯一匹配产能",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"标准成本默认产能",
			"不锁定生产计划实际设备",
		},
		filepath.Join("docs", "acceptance", "2026-07-02-standard-cost-default-capacity.md"): {
			"PR-516-STANDARD-COST-DEFAULT-CAPACITY",
			"标准成本默认产能",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-516 marker %q", rel, want)
			}
		}
	}
}
