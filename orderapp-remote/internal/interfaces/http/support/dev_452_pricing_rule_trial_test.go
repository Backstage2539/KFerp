package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev452PricingRuleTrialContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-452-PRICING-RULE-TRIAL",
			"DEV-452-PRICING-RULE-TRIAL-API",
			"DEV-452-PRICING-RULE-TRIAL-UI",
			"DEV-452-PRICING-RULE-TRIAL-DOCS",
			"REV-452-PRICING-RULE-TRIAL",
		},
		filepath.Join("internal", "application", "costing", "service.go"): {
			"type PricingRuleTrialCommand struct",
			"type PricingRuleTrialResult struct",
			"func (s *Service) PricingRuleTrial",
			"LoadProductPricingRule",
			"试算毛利率低于最低毛利",
		},
		filepath.Join("internal", "infrastructure", "postgres", "costing", "repository.go"): {
			"func (r Repository) LoadProductPricingRule",
			"product_pricing_rules",
			"calculation_json",
			"formula_version",
		},
		filepath.Join("internal", "interfaces", "http", "costing", "costing_api.go"): {
			"/api/costing/pricing-rule-trial",
			"PricingRuleTrialCommand",
			"PricingRuleTrial",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"): {
			"buildPricingRuleTrialPayload",
			"pricing_rule_id",
			"other_costs",
			"expected_loss_rate",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"价格计算模板试算",
			"openPricingRuleTrial",
			"pricingRuleTrialDrawerOpen",
			"/api/costing/pricing-rule-trial",
			"试算商品",
			"公式步骤",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-452-PRICING-RULE-TRIAL",
			"价格计算模板试算",
			"不保存到模板、商品价格表、发布快照或订单",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-452-PRICING-RULE-TRIAL",
			"模板行显示 `试算`",
			"试算结果不回写商品价格表",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"价格计算模板试算",
			"选择商品",
			"重新试算",
			"只读结果",
		},
		filepath.Join("docs", "acceptance", "2026-06-08-pricing-rule-trial.md"): {
			"PR-452",
			"商品价格管理模板试算",
			"POST /api/costing/pricing-rule-trial",
			"浏览器验收",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-452 marker %q", rel, want)
			}
		}
	}
}
