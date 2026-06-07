package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev443PricingRuleCalculationTemplateContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "infrastructure", "postgres", "catalog", "schema.go"): {
			"calculation_json JSONB NOT NULL DEFAULT '{}'::jsonb",
			"formula_version TEXT NOT NULL DEFAULT 'v1'",
		},
		filepath.Join("internal", "application", "catalog", "service.go"): {
			"CalculationJSON",
			"FormulaVersion",
			"pricing rule must not contain quantity tiers",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"成本项配置",
			"利润方式",
			"税费方式",
			"最低毛利",
			"公式版本",
			"试算说明",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue"): {
			"pricing_rule_calculation",
			"pricing_rule_formula_version",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-443-PRICING-RULE-CALCULATION-TEMPLATE",
			"Pricing Rule 不保存数量档位",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"Pricing Rule 不保存数量档位",
			"数量档位只属于阶梯模板或商品价格表生成上下文",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-443 contract marker %q", rel, want)
			}
		}
	}

	schema := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "catalog", "schema.go")))
	if strings.Contains(schema, "product_pricing_rules (\n\tid BIGSERIAL PRIMARY KEY,\n\tmin_qty") ||
		strings.Contains(schema, "product_pricing_rules (\n\tid BIGSERIAL PRIMARY KEY,\n\tmax_qty") ||
		strings.Contains(schema, "product_pricing_rules (\n\tid BIGSERIAL PRIMARY KEY,\n\ttier_label") {
		t.Fatalf("product_pricing_rules must not add quantity tier columns")
	}
}
