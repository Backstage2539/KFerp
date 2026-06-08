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
			"自动试算",
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

func TestDev454PricingRuleTrialExcelParityContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-454-PRICING-RULE-TRIAL-EXCEL-PARITY",
			"DEV-454-EXCEL-PARITY-ANALYSIS",
			"DEV-454-TRIAL-FORMULA-PARITY",
			"DEV-454-TRIAL-FORMULA-NODES",
			"REV-454-PRICING-RULE-TRIAL-EXCEL-PARITY",
		},
		filepath.Join("internal", "application", "costing", "service.go"): {
			"PostMarkupCosts",
			"supplier_tier_markup",
			"price_after_markup",
			"post_markup_cost_total",
			"售价后附加成本",
		},
		filepath.Join("internal", "application", "costing", "service_test.go"): {
			"TestPricingRuleTrialMatchesExcelSupplierPriceSamples",
			"测试用 1kg-2磅",
			"单品：孟连红果厌氧慢速日晒 24kg-48磅",
			"116.70915065789475",
			"177.11505625",
		},
		filepath.Join("internal", "interfaces", "http", "costing", "costing_api_test.go"): {
			"PostMarkupCosts",
			"PriceAfterMarkup",
			"PostMarkupCostTotal",
			"post_markup_cost_total",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"加价后价格",
			"pricingRuleTrialQuoteUnitOptions",
			"schedulePricingRuleTrial",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-454-PRICING-RULE-TRIAL-EXCEL-PARITY",
			"物料成本",
			"生产项目",
			"供应售价",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-454 商品价格管理试算 Excel 对账",
			"2 个产品 × 2 个供应售价档位",
			"生产项目",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"Excel 供应售价对账",
			"加价前生产项目成本",
			"前端试算抽屉不再提供售价后附加成本录入口",
		},
		filepath.Join("docs", "acceptance", "2026-06-08-pricing-rule-trial-excel-parity.md"): {
			"PR-453",
			"刘豪-成本核算3",
			"测试用 1kg-2磅",
			"单品：孟连红果厌氧慢速日晒 24kg-48磅",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-453 marker %q", rel, want)
			}
		}
	}
}

func TestDev455PricingRuleTrialPr439UnitContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-455-PRICING-RULE-TRIAL-PR439-UNIT",
			"DEV-455-TRIAL-AUTO-UNIT-UI",
			"DEV-455-TRIAL-PUBLISHED-SNAPSHOT-FALLBACK",
			"REV-455-PRICING-RULE-TRIAL-PR439-UNIT",
		},
		filepath.Join("internal", "application", "costing", "service.go"): {
			"pricingRuleTrialInferBaseCostFromPublishedSnapshot",
			"published_price_snapshot",
			"未找到BOM/工序成本，已按发布售价快照反推成本基数",
		},
		filepath.Join("internal", "application", "costing", "service_test.go"): {
			"TestPricingRuleTrialInfersCostFromPublishedPriceSnapshotWhenBomCostMissing",
			"PR439-20260606182321 熟豆下单商品",
			"88.5",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"): {
			"buildPricingRuleTrialPayload",
			"other_costs",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"pricingRuleTrialQuoteUnitOptions",
			"schedulePricingRuleTrial",
			"activeProductUnitDefinitions",
			"试算中...",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-455-PRICING-RULE-TRIAL-PR439-UNIT",
			"PR439-20260606182321 熟豆下单商品",
			"全局单位字典",
			"发布售价快照",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-455-PRICING-RULE-TRIAL-PR439-UNIT",
			"不显示 `重新试算`",
			"不显示 `售价后附加成本`",
			"88.5/kg",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-455-PRICING-RULE-TRIAL-PR439-UNIT",
			"全局单位字典",
			"自动试算",
			"发布售价快照反推成本基数",
		},
		filepath.Join("docs", "acceptance", "2026-06-08-pricing-rule-trial-pr439-unit.md"): {
			"PR-455-PRICING-RULE-TRIAL-PR439-UNIT",
			"PR439-20260606182321 熟豆下单商品",
			"88.5/kg",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-455 marker %q", rel, want)
			}
		}
	}

	for rel, forbidden := range map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"重新试算",
			"售价后附加成本",
			"post_markup_cost_rows",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"): {
			"post_markup_costs",
			"postMarkupCosts",
			"post_markup_cost_rows",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, value := range forbidden {
			if strings.Contains(src, value) {
				t.Fatalf("%s should not expose PR-455 removed marker %q", rel, value)
			}
		}
	}
}
