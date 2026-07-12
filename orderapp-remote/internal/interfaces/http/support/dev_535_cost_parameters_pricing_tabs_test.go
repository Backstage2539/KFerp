package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev535CostParametersPricingTabsContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-535-COST-PARAMETERS-PRICING-TABS", "DEV-535-PRICE-MANAGEMENT-TABS", "DEV-535-COMPAT-DOCS-DEPLOY",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"商品价格管理功能", "activeProductPriceManagementTab", "pricing-rules", "cost-parameters", "costing-settings-tab-panel",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "price-management-tabs.test.js"): {
			"价格计算模板", "成本参数设置", "CostingSettingsPanel",
		},
		filepath.Join("docs", "REQUIREMENTS.md"):                                          {"PR-535-COST-PARAMETERS-PRICING-TABS"},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"):                                      {"PR-535-COST-PARAMETERS-PRICING-TABS"},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"):                                     {"与 `价格计算模板` 并排的 `成本参数设置` Tab"},
		filepath.Join("docs", "acceptance", "2026-07-12-cost-parameters-pricing-tabs.md"): {"PR-535 商品价格管理成本参数 Tab 验收"},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-535 marker %q", rel, want)
			}
		}
	}
}
