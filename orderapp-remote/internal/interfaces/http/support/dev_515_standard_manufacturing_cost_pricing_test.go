package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev515StandardManufacturingCostPricingContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-515-STANDARD-MANUFACTURING-COST-PRICING",
			"DEV-515-STANDARD-MANUFACTURING-COST-API",
			"DEV-515-STANDARD-OPERATION-COST",
			"DEV-515-PRICING-UI-DOCS",
		},
		filepath.Join("internal", "application", "costing", "service.go"): {
			"standard_manufacturing_unit_cost",
			"cost_source = standard_manufacturing_cost",
			"标准制造成本",
		},
		filepath.Join("internal", "infrastructure", "postgres", "costing", "repository.go"): {
			"standard_operation_cost",
			"operation_master",
			"标准工序成本",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"标准制造成本",
			"BOM物料成本",
			"标准工序成本",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-515-STANDARD-MANUFACTURING-COST-PRICING",
			"标准制造成本",
			"价格表不直接绑定生产计划里的产能和批次数",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-515-STANDARD-MANUFACTURING-COST-PRICING",
			"工位小时费率由机器、人工和其他小时成本相加",
			"价格试算读取标准制造成本",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"标准制造成本",
			"价格计算模板只做利润、税率和取整",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"标准制造成本",
			"生产计划/工单选择真实工位和产能不回改历史价格",
		},
		filepath.Join("docs", "acceptance", "2026-07-01-standard-manufacturing-cost-pricing.md"): {
			"PR-515-STANDARD-MANUFACTURING-COST-PRICING",
			"标准制造成本",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-515 marker %q", rel, want)
			}
		}
	}
}
