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
			"基础成本",
			"生产 BOM 成本（物料+工序）",
			"其他成本",
			"加价率",
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

	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue")))
	pane := productPriceManagementPaneForDev443(view)
	for _, forbidden := range []string{"商品成本上下文", "成本取数口径", "成本项配置", "库存成本", "手工成本", "最近采购成本"} {
		if strings.Contains(pane, forbidden) {
			t.Fatalf("pricing rule pane must not expose removed cost marker %q", forbidden)
		}
	}
}

func productPriceManagementPaneForDev443(source string) string {
	start := strings.Index(source, `<div v-show="showProductPriceManagementPane"`)
	if start < 0 {
		return ""
	}
	end := strings.Index(source[start:], `<div v-if="classificationTemplateCreateDrawerOpen"`)
	if end < 0 {
		return source[start:]
	}
	return source[start : start+end]
}
