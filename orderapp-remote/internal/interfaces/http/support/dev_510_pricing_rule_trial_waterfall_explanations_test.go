package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev510PricingRuleTrialWaterfallExplanationsContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-510-PRICING-RULE-TRIAL-WATERFALL-EXPLANATIONS",
			"DEV-510-TRIAL-EXPLANATION-API",
			"DEV-510-TRIAL-EXPLANATION-UI",
			"DEV-510-DOCS-ACCEPTANCE",
			"API-510-TRIAL-EXPLANATION-FIELDS",
			"REV-510-PRICING-RULE-TRIAL-WATERFALL-EXPLANATIONS",
		},
		filepath.Join("..", "REQUIREMENTS.md"): {
			"PR-510-PRICING-RULE-TRIAL-WATERFALL-EXPLANATIONS",
			"试算说明",
			"BOM+工序成本",
			"其他成本",
			"加价增加",
		},
		filepath.Join("..", "ACCEPTANCE_TESTS.md"): {
			"PR-510-PRICING-RULE-TRIAL-WATERFALL-EXPLANATIONS",
			"SKU-000573",
			"榛巧拼配227g袋装",
			"kg",
			"袋",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-510-PRICING-RULE-TRIAL-WATERFALL-EXPLANATIONS",
			"试算说明",
			"价格计算模板编辑区",
			"本次试算抽屉",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-510-PRICING-RULE-TRIAL-WATERFALL-EXPLANATIONS",
			"other_cost_details",
			"profit_explanation",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-510-PRICING-RULE-TRIAL-WATERFALL-EXPLANATIONS",
			"点击 `BOM+工序成本`",
			"点击 `其他成本`",
			"点击 `加价增加`",
		},
		filepath.Join("docs", "acceptance", "2026-06-30-pricing-rule-trial-waterfall-explanations.md"): {
			"PR-510-PRICING-RULE-TRIAL-WATERFALL-EXPLANATIONS",
			"RED",
			"GREEN",
			"Browser",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-510 marker %q", rel, want)
			}
		}
	}
}
