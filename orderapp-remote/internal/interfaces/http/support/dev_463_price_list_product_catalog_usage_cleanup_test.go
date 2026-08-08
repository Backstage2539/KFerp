package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev463PriceListProductCatalogUsageCleanupContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-463-PRICE-LIST-PRODUCT-CATALOG-USAGE-CLEANUP",
			"DEV-463-PRICE-LIST-TYPE-PRODUCT-CATALOG",
			"DEV-463-PRICE-LIST-HIDE-PURPOSE-UI",
			"DEV-463-DOCS-ACCEPTANCE",
			"REV-463-PRICE-LIST-PRODUCT-CATALOG-USAGE-CLEANUP",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-price-list-types.js"): {
			"buildProductCatalogTemplatePriceListTypeOptions",
			"matchesProductCatalogPriceListType",
			"businessGroupItemsTreeForPriceList",
			"product-catalog:flat",
			"product_catalog",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue"): {
			"buildProductCatalogTemplatePriceListTypeOptions",
			"businessGroupFeatureSelectionIDs",
			"apiGet('/api/business-group-feature-selections/product_catalog')",
			"matchesProductCatalogPriceListType",
			"FACTORY_SUPPLY_PUBLICATION_PURPOSE",
			"publication_purpose",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-price-list-types.test.js"): {
			"price list product types partition products by the product catalog templates selected by product archive",
			"price list uses one safe flat product type when product archive selected no group templates",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "costing-bean-list-version-ui.test.js"): {
			"product price-list version list hides factory supply and customer resale purpose filter",
			"FACTORY_SUPPLY_PUBLICATION_PURPOSE",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-463-PRICE-LIST-PRODUCT-CATALOG-USAGE-CLEANUP",
			"咖啡熟豆 / 意式拼配豆",
			"商品价格表版本列表固定查询 `factory_supply`",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-463-PRICE-LIST-PRODUCT-CATALOG-USAGE-CLEANUP",
			"熟豆-红岩拼配",
			"不显示用途筛选",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-463-PRICE-LIST-PRODUCT-CATALOG-USAGE-CLEANUP",
			"商品价格表版本列表不再提供“用途”筛选",
			"咖啡熟豆 / 意式拼配豆",
		},
		filepath.Join("docs", "acceptance", "2026-06-10-price-list-product-catalog-usage-cleanup.md"): {
			"PR-463-PRICE-LIST-PRODUCT-CATALOG-USAGE-CLEANUP",
			"熟豆-红岩拼配",
			"用途筛选",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-463 marker %q", rel, want)
			}
		}
		if rel == filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue") {
			for _, avoid := range []string{
				"publicationPurposeFilter",
				"客户转售价格表",
				"工厂供货价格表",
				"buildProductCatalogPriceListTypeOptions(",
				"/api/business-group-feature-selections/price_list",
				"usage_key=price_list&object_key=product",
			} {
				if strings.Contains(src, avoid) {
					t.Fatalf("%s should not expose ERP purpose UI marker %q", rel, avoid)
				}
			}
		}
	}
}
