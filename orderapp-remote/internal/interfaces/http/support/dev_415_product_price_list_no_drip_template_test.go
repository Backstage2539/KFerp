package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev415ProductPriceListNoDripTemplateRequirementSeed(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-415-PRODUCT-PRICE-LIST-NO-DRIP-TEMPLATE",
		"DEV-415-COSTING-VIEW-NO-DRIP-INFERENCE",
		"DEV-415-LEGACY-DRIP-PDF-COMPAT",
		"UT-415-PRODUCT-PRICE-LIST-NO-DRIP-TEMPLATE",
		"API-415-PRODUCT-PRICE-LIST-NO-DRIP-TEMPLATE",
		"REV-415-PRODUCT-PRICE-LIST-NO-DRIP-TEMPLATE",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing PR-415 seed marker %q", want)
		}
	}
}

func TestDev415ProductPriceListNoDedicatedDripView(t *testing.T) {
	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue")))
	for _, forbidden := range []string{
		"if (kind === 'drip_bag') return 'drip'",
		"categoryHint.includes('挂耳')",
		"categoryHint.includes('drip')",
		"section.listType === 'drip'",
		"/api/drip-price-templates",
		"/api/costing/drip-price-explanation",
		"openDripPriceExplanation",
		"loadDripPriceExplanation",
	} {
		if strings.Contains(view, forbidden) {
			t.Fatalf("CostingView.vue should not expose dedicated drip price-list logic %q", forbidden)
		}
	}
	for _, want := range []string{
		"price-list-page-config",
		"priceListRenderTypeForItem",
		"commercial_wholesale_tiers",
		"openPriceExplanation",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("CostingView.vue missing normal product price-list marker %q", want)
		}
	}
}

func TestDev415ProductPriceListManualDocumentsNoDedicatedDripTemplate(t *testing.T) {
	for _, path := range []string{
		filepath.Join("docs", "OP_MANUAL_COSTING.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "acceptance", "2026-06-05-product-price-list-no-drip-template.md"),
	} {
		body := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"PR-415-PRODUCT-PRICE-LIST-NO-DRIP-TEMPLATE",
			"挂耳",
			"商品配置模板",
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing PR-415 manual marker %q", path, want)
			}
		}
	}
}
