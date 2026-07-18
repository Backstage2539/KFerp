package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev468PricingRuleListActionsUXContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-468-PRICING-RULE-LIST-ACTIONS-UX",
			"DEV-468-PRICING-RULE-TRIAL-LAUNCHER",
			"DEV-468-PRICING-RULE-NAME-EDIT-COPY-INACTIVE",
			"DEV-468-DOCS-ACCEPTANCE",
			"REV-468-PRICING-RULE-LIST-ACTIONS-UX",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"价格试算",
			"请选择启用的价格计算模板",
			"pricing-rule-name-button",
			"pricing-rule-copy-action",
			"copyPricingRule",
			"activePricingRuleTrialOptions",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"): {
			"buildPricingRuleCopyPayload",
			"pricingRuleCopyName",
			"pricingRuleCopyCode",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.test.js"): {
			"pricing rule copy payload creates an active unique template from inactive source",
			"@click=\"openPricingRuleTrial\\(\\)\"",
			"pricing-rule-copy-action",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-468-PRICING-RULE-LIST-ACTIONS-UX",
			"价格试算",
			"点击模板名称",
			"停用模板置灰",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-468-PRICING-RULE-LIST-ACTIONS-UX",
			"价格试算",
			"普通停用模板置灰",
			"历史 `fixed_add` 等隔离模板除外",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-468-PRICING-RULE-LIST-ACTIONS-UX",
			"价格试算",
			"复制",
		},
		filepath.Join("docs", "acceptance", "2026-06-11-pricing-rule-list-actions-ux.md"): {
			"PR-468-PRICING-RULE-LIST-ACTIONS-UX",
			"价格试算",
			"停用模板置灰",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-468 marker %q", rel, want)
			}
		}
	}
}
