package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev444PricingRuleCostSourceUXContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-444-PRICING-RULE-COST-SOURCE-UX",
			"DEV-444-PRICING-RULE-OTHER-COSTS",
			"DEV-444-PRICING-RULE-EDIT-DEACTIVATE",
		},
		filepath.Join("internal", "application", "catalog", "service.go"): {
			"normalizePricingRuleCostSourceMode",
			"normalizePricingRuleOtherCosts",
			"cost_components",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"): {
			"normalizePricingRuleCostSourceMode",
			"pricingRuleOtherCostMapFromForm",
			"other_costs",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"pricing-rule-name-button",
			"PricingRuleEditorForm",
		},
		filepath.Join("frontend-vue-shell", "src", "components", "PricingRuleEditorForm.vue"): {
			"基础成本",
			"生产 BOM 成本（物料+工序）",
			"其他成本",
			"全局币种配置",
			"失效",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-444-PRICING-RULE-COST-SOURCE-UX",
			"基础成本固定为 `生产 BOM 成本（物料+工序）`",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-444-PRICING-RULE-COST-SOURCE-UX",
			"成本名和成本价格",
		},
		filepath.Join("docs", "acceptance", "2026-06-07-pricing-rule-cost-source-ux.md"): {
			"PR-444 Pricing Rule 成本配置简化",
			"其他成本 KV",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-444 contract marker %q", rel, want)
			}
		}
	}

	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue")))
	pane := productPriceManagementPaneForDev443(view)
	for _, forbidden := range []string{"商品成本上下文", "成本取数口径", "成本项配置", "库存成本", "手工成本", "最近采购成本"} {
		if strings.Contains(pane, forbidden) {
			t.Fatalf("pricing rule pane must not expose removed cost marker %q", forbidden)
		}
	}
}
