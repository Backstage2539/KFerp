package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev339OrderEntryKGTierUnitVersionWarningSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-339-ORDER-ENTRY-KG-TIER-UNIT-VERSION-WARNING",
		"DEV-339-ORDER-ENTRY-KG-TIER-UNIT-VERSION-WARNING",
		"UT-339-ORDER-ENTRY-KG-TIER-UNIT-VERSION-WARNING",
		"API-339-ORDER-ENTRY-KG-TIER-UNIT-VERSION-WARNING",
		"REV-339-ORDER-ENTRY-KG-TIER-UNIT-VERSION-WARNING",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PR-339 requirement seed missing %q", want)
		}
	}
}

func TestDev339OrderEntryKGTierUnitVersionWarningWiring(t *testing.T) {
	for _, tc := range []struct {
		rel     string
		markers []string
	}{
		{
			rel: filepath.Join("frontend-vue-shell", "src", "lib", "order-entry.js"),
			markers: []string{
				"resolveWholesaleTierPrice",
				"orderRowPriceUnit",
				"tierPriceLabel",
				"belowMinTier",
				"beanListVersionNo",
			},
		},
		{
			rel: filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue"),
			markers: []string{
				"tier_price_label",
				"price_missing",
				"当前数量无已发布价格，不能保存",
				"hasUnpricedPublishedRow",
				"ensureRowBeanListVersion",
				"priceUnitLabel(row)",
			},
		},
		{
			rel: filepath.Join("internal", "infrastructure", "postgres", "orderbeans", "usage.go"),
			markers: []string{
				"PublishedPricing",
				"ResolvePublishedPricingForPublicationWithUnit",
				"DisplayUnit",
				"PriceUnit",
			},
		},
		{
			rel: filepath.Join("internal", "infrastructure", "postgres", "sales", "repository.go"),
			markers: []string{
				"wholesaleLineTotalFromPriceUnit",
				"beanListPriceSourceJSONWithPricing",
				"pricing.UnitG",
				"price_unit",
			},
		},
	} {
		src := string(readOrderAppFileForTest(t, tc.rel))
		for _, want := range tc.markers {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-339 marker %q", tc.rel, want)
			}
		}
	}
}

func TestDev339OrderEntryKGTierUnitVersionWarningDocs(t *testing.T) {
	for _, rel := range []string{
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "acceptance", "2026-05-23-order-entry-kg-tier-unit-version-warning.md"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"PR-339-ORDER-ENTRY-KG-TIER-UNIT-VERSION-WARNING",
			"元/kg",
			"当前数量无已发布价格，不能保存",
			"豆单版本",
		} {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-339 documentation marker %q", rel, want)
			}
		}
	}
}
