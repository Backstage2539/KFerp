package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev435PriceListCategoryAlignmentSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-435-PRICE-LIST-CATEGORY-ALIGNMENT",
		"DEV-435-PRICE-LIST-CURRENT-CLASSIFICATION",
		"DEV-435-PRICE-LIST-UNCLASSIFIED-MERGE",
		"REV-435-PRICE-LIST-CATEGORY-ALIGNMENT",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PR-435 requirement seed missing %q", want)
		}
	}
}

func TestDev435PriceListCategoryAlignmentWiringAndDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "lib", "product-price-list-types.js"): {
			"UNCLASSIFIED_PRODUCT_PRICE_LIST_TYPE_ID",
			"classification:unclassified",
			"matchesPublicationProductType",
			"classification_template_id",
			"current_classification_template_id",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue"): {
			"buildClassificationPriceListTypeOptionsFromItems",
			"greenTierPriceRows(item)",
			"metaKeyForItem(item)",
			"Price List / Item Price",
			"商品 &gt; 子类 &gt; 父类 &gt; 价格表",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-price-list-types.test.js"): {
			"legacy product type id does not count as current product archive classification",
			"未分类商品",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-435-PRICE-LIST-CATEGORY-ALIGNMENT",
			"商品价格表的商品类型必须和商品档案当前分类 Tab 保持一致",
			"旧 product_type_category_id",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-435-PRICE-LIST-CATEGORY-ALIGNMENT",
			"未分类商品",
			"不能再出现多个 `其他`",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-435-PRICE-LIST-CATEGORY-ALIGNMENT",
			"商品价格表的商品类型下拉",
			"旧 `product_type_category_id`",
		},
		filepath.Join("docs", "acceptance", "2026-06-06-price-list-category-alignment.md"): {
			"PR-435-PRICE-LIST-CATEGORY-ALIGNMENT",
			"商品档案",
			"商品价格表",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-435 marker %q", rel, want)
			}
		}
	}
}
