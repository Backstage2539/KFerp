package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev459PriceListFollowProductGroupContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-459-PRICE-LIST-FOLLOW-PRODUCT-GROUP",
			"DEV-459-PRICE-LIST-PRODUCT-GROUP-LOAD",
			"DEV-459-PRODUCT-GROUP-DRAFT-BRIDGE",
			"DEV-459-DOCS-ACCEPTANCE",
			"REV-459-PRICE-LIST-FOLLOW-PRODUCT-GROUP",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue"): {
			"apiGet('/api/business-groups')",
			"apiGet('/api/business-group-assignments?usage_key=product_catalog&object_key=product')",
			"apiGet('/api/business-group-feature-selections/product_catalog')",
			"groupRowsByBusinessGroupTemplate",
			"businessGroupFeatureSelectionIDs(priceListProductCatalogFeatureSelection.value)",
			"selectedProductCatalogGroupTemplates",
			"business-group-unclassified",
			"group_source: 'product_catalog'",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"apiGet('/api/business-group-feature-selections/product_catalog')",
			"businessGroupRowsForFeatureSelection(businessGroups.value, productGroupFeatureSelectionIDs.value)",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-bean-list-split.test.js"): {
			"business-group-unclassified",
			"groupRowsByBusinessGroupTemplate",
			"product_catalog",
		},
		filepath.Join("internal", "interfaces", "http", "costing", "costing_vue_source_test.go"): {
			"TestCostingViewFollowsProductCatalogBusinessGroupTemplate",
			"selectedProductCatalogGroupTemplate",
			"business-group-unclassified",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-459-PRICE-LIST-FOLLOW-PRODUCT-GROUP",
			"商品档案页面当前选择的 `商品分组`",
			"group_source=product_catalog",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-459-PRICE-LIST-FOLLOW-PRODUCT-GROUP",
			"商品档案选择 `商品分组` 后切到商品价格表",
			"平铺价格行快照写入 `group_source=product_catalog`",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-459-PRICE-LIST-FOLLOW-PRODUCT-GROUP",
			"商品档案选择了 `商品分组`",
			"不会继续沿用其他模板的分类标题",
		},
		filepath.Join("docs", "acceptance", "2026-06-08-price-list-follow-product-group.md"): {
			"PR-459 商品价格表跟随商品档案商品分组",
			"product_catalog/product",
			"business-group-unclassified",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-459 marker %q", rel, want)
			}
		}
		if rel == filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue") {
			for _, avoid := range []string{
				"readFormDraft(productSettingsDraftKeyForPriceList())",
				"/api/business-group-feature-selections/price_list",
				"usage_key=price_list&object_key=product",
			} {
				if strings.Contains(src, avoid) {
					t.Fatalf("%s should not retain superseded price-list grouping marker %q", rel, avoid)
				}
			}
		}
	}
}
