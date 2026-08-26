package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev465PriceListPricingRulePreviewContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-465-PRICE-LIST-PRICING-RULE-PREVIEW",
			"DEV-465-PRICE-LIST-PRICING-RULE-TRIAL",
			"DEV-465-PRICE-LIST-PREVIEW-FLAT-ROWS",
			"REV-465-PRICE-LIST-PRICING-RULE-PREVIEW",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"): {
			"priceTablePricingRuleTrialPayload",
			"applyPricingRuleTrialToPriceTableRow",
			"priceTablePricingRuleTrialCacheKey",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "costing-price-list-workflow.js"): {
			"priceListPricingRuleTrialRequestsForRows",
			"cached?.status === 'error'",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "bean-list-pdf.js"): {
			"applyPriceListFlatRowsToBeanListPdfGroups",
			"flatRowsForPdfItem",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue"): {
			"priceListPricingRuleTrialRequests",
			"/api/costing/pricing-rule-trial",
			"normalizePriceListPublicationGroups",
			"applyPriceListFlatRowsToBeanListPdfGroups(normalizedPriceListGroups.value, priceListFlatRows.value",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-465-PRICE-LIST-PRICING-RULE-PREVIEW",
			"熟豆-红岩拼配",
			"咖啡熟豆磅装模板",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-465-PRICE-LIST-PRICING-RULE-PREVIEW",
			"价格表预览",
			"模板试算价格",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-465-PRICE-LIST-PRICING-RULE-PREVIEW",
			"按价格模板计算",
			"价格计算模板试算",
		},
		filepath.Join("docs", "acceptance", "2026-06-10-price-list-pricing-rule-preview.md"): {
			"PR-465-PRICE-LIST-PRICING-RULE-PREVIEW",
			"熟豆-红岩拼配",
			"咖啡熟豆磅装模板",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-465 marker %q", rel, want)
			}
		}
	}
}
