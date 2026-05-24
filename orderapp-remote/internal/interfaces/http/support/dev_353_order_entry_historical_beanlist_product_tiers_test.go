package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev353OrderEntryHistoricalBeanListProductTiers(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-353-ORDER-ENTRY-HISTORICAL-BEANLIST-PRODUCT-TIERS",
		"DEV-353-ORDER-ENTRY-HISTORICAL-BEANLIST-PRODUCT-TIERS",
		"UT-353-ORDER-ENTRY-HISTORICAL-BEANLIST-PRODUCT-TIERS",
		"API-353-ORDER-ENTRY-HISTORICAL-BEANLIST-PRODUCT-TIERS",
		"REV-353-ORDER-ENTRY-HISTORICAL-BEANLIST-PRODUCT-TIERS",
		"2026-05-24-order-entry-historical-beanlist-product-tiers.md",
	} {
		if !strings.Contains(reqStore, want) {
			t.Fatalf("dev 353 req seed missing %q", want)
		}
	}

	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-353-ORDER-ENTRY-HISTORICAL-BEANLIST-PRODUCT-TIERS",
			"历史熟豆",
			"商品阶梯",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-353-ORDER-ENTRY-HISTORICAL-BEANLIST-PRODUCT-TIERS",
			"publication_id",
			"只剩生豆",
		},
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"): {
			"PR-353-ORDER-ENTRY-HISTORICAL-BEANLIST-PRODUCT-TIERS",
			"历史版本",
			"发布快照",
		},
		filepath.Join("docs", "acceptance", "2026-05-24-order-entry-historical-beanlist-product-tiers.md"): {
			"PR-353-ORDER-ENTRY-HISTORICAL-BEANLIST-PRODUCT-TIERS",
			"TestMergeOrderPublicationTierMapsKeepsMultiplePublishedVersions",
			"TestOrderAPIFormReturnsAllPublishedCommercialBeanListTiersForVersionSwitching",
		},
		filepath.Join("internal", "infrastructure", "postgres", "sales", "order_form_queries.go"): {
			"mergeOrderPublicationTierMaps",
			"officialTiers = mergeOrderPublicationTierMaps",
			"customerTiers[ownerKey] = mergeOrderPublicationTierMaps",
		},
		filepath.Join("internal", "infrastructure", "postgres", "sales", "order_form_queries_static_test.go"): {
			"TestMergeOrderPublicationTierMapsKeepsMultiplePublishedVersions",
		},
		filepath.Join("internal", "interfaces", "http", "sales", "order_api_test.go"): {
			"TestOrderAPIFormReturnsAllPublishedCommercialBeanListTiersForVersionSwitching",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing dev 353 marker %q", rel, want)
			}
		}
	}
}
