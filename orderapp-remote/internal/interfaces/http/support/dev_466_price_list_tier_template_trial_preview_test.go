package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev466PriceListTierTemplateTrialPreviewContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-466-PRICE-LIST-TIER-TEMPLATE-TRIAL-PREVIEW",
			"DEV-466-TIER-TEMPLATE-TRIAL-PAYLOAD",
			"DEV-466-TIER-TEMPLATE-TRIAL-APPLY",
			"REV-466-PRICE-LIST-TIER-TEMPLATE-TRIAL-PREVIEW",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"): {
			"row.tier_pricing_rule_id",
			"['pricing_rule', 'tier_template'].includes(pricingMode)",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.test.js"): {
			"price table tier-template preview rows use their tier pricing rule trial result",
			"熟豆-红岩拼配",
			"咖啡熟豆磅装模板-v1",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-466-PRICE-LIST-TIER-TEMPLATE-TRIAL-PREVIEW",
			"按阶梯模板价计算",
			"咖啡熟豆磅装模板",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-466-PRICE-LIST-TIER-TEMPLATE-TRIAL-PREVIEW",
			"红岩拼配",
			"两个阶梯",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-466-PRICE-LIST-TIER-TEMPLATE-TRIAL-PREVIEW",
			"阶梯价模板",
			"价格计算模板试算",
		},
		filepath.Join("docs", "acceptance", "2026-06-10-price-list-tier-template-trial-preview.md"): {
			"PR-466-PRICE-LIST-TIER-TEMPLATE-TRIAL-PREVIEW",
			"红岩拼配",
			"咖啡熟豆磅装模板",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-466 marker %q", rel, want)
			}
		}
	}
}
