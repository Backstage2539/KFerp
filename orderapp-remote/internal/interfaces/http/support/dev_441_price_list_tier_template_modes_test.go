package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev441PriceListTierTemplateModesSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-441-PRICE-LIST-TIER-TEMPLATE-MODES",
		"DEV-441-TIER-TEMPLATE-DRAWER",
		"DEV-441-THREE-PRICE-LIST-MODES",
		"DEV-441-PRICE-LIST-SNAPSHOT-MODES",
		"REV-441-PRICE-LIST-TIER-TEMPLATE-MODES",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PR-441 requirement seed missing %q", want)
		}
	}
}

func TestDev441PriceListTierTemplateModesContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "infrastructure", "postgres", "catalog", "schema.go"): {
			"ALTER TABLE %[1]s.price_tier_template_tiers ADD COLUMN IF NOT EXISTS pricing_rule_id BIGINT NOT NULL DEFAULT 0",
		},
		filepath.Join("internal", "application", "catalog", "service.go"): {
			"PricingRuleID",
			"DeletePriceTierTemplate",
			"pricing_rule_id required",
			"fixed_price",
		},
		filepath.Join("internal", "interfaces", "http", "catalog", "product_routes.go"): {
			"DELETE",
			"/api/price-tier-templates/:id",
			"deletePriceTierTemplateAPI",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"价格计算模板 / Pricing Rule",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue"): {
			"管理阶梯模板",
			"按阶梯模板计算",
			"按价格计算模板计算",
			"固定价",
			"商品 &gt; 子类 &gt; 父类 &gt; 价格表",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-441-PRICE-LIST-TIER-TEMPLATE-MODES",
			"按阶梯模板计算",
			"商品 > 子类 > 父类 > 价格表",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"管理阶梯模板",
			"固定价",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-441 contract marker %q", rel, want)
			}
		}
	}

	productSettings := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue")))
	if strings.Contains(productSettings, "保存阶梯模板") || strings.Contains(productSettings, "保存阶梯价模板") {
		t.Fatalf("ProductSettingsView must not own tier template maintenance")
	}
}
