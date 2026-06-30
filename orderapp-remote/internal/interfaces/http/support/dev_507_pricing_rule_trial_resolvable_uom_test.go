package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev507PricingRuleTrialResolvableUOMContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-507-PRICING-RULE-TRIAL-RESOLVABLE-UOM",
			"DEV-507-TRIAL-UOM-CANDIDATES",
			"DEV-507-TRIAL-UOM-VALIDATION",
			"REV-507-PRICING-RULE-TRIAL-RESOLVABLE-UOM",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"): {
			"pricingRuleTrialUnitConversionFactor",
			"pricingRuleTrialQuoteUnitOptionsForProduct",
		},
		filepath.Join("internal", "application", "costing", "service.go"): {
			"pricingRuleTrialQuoteUnitResolvable",
			"销售单位",
			"缺少可解析的单位换算",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-507-PRICING-RULE-TRIAL-RESOLVABLE-UOM",
			"销售单位",
			"不得静默按 `kg` 试算",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-507-PRICING-RULE-TRIAL-RESOLVABLE-UOM",
			"不得仅因全局单位存在而出现",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-507-PRICING-RULE-TRIAL-RESOLVABLE-UOM",
			"先在商品档案的销售规格模板或单位换算中维护",
		},
		filepath.Join("docs", "acceptance", "2026-06-30-pricing-rule-trial-resolvable-uom.md"): {
			"PR-507-PRICING-RULE-TRIAL-RESOLVABLE-UOM",
			"quote_unit=盒",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-507 marker %q", rel, want)
			}
		}
	}
}
