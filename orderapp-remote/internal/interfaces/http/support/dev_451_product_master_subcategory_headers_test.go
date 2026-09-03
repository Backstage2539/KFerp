package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev451ProductMasterSubcategoryHeadersContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-451-PRODUCT-MASTER-SUBCATEGORY-HEADERS",
			"DEV-451-PRODUCT-GROUP-SUBCATEGORY-HEADERS",
			"DEV-451-PRODUCT-GROUP-MOVE-SUBCATEGORY",
			"DEV-451-PRODUCT-GROUP-ITEM-INDENT",
			"REV-451-PRODUCT-MASTER-SUBCATEGORY-HEADERS",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"): {
			"businessGroupItemInfo",
			"path_label",
			"title_label",
			"depth",
			"parent_group_item_id",
			"includeGroupName: false",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"classification-subgroup-row",
			"businessGroupInlineListState",
			"businessGroupVisibleRows",
			"--classification-group-indent",
			"--classification-item-indent",
			"classification-item-row",
			"{{ group.label }}",
			"BusinessGroupInlineWorkspace",
			"collapsedProductClassificationGroups",
			"productCategoryMoveActive",
			`@target="handleProductCategoryMoveTarget"`,
			"handleProductGroupPaginationChange",
			`#group="{ group }"`,
			"<thead>",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.test.js"): {
			"商品-咖啡熟豆 / 意式拼配豆",
			"depth: 1",
			"parent: 90",
			"--business-group-inline-depth",
			"businessGroupVisibleGroups",
			"classification-item-row",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-451-PRODUCT-MASTER-SUBCATEGORY-HEADERS",
			"层级缩进表达完整路径",
			"每个含商品的分类重复表头并独立分页",
			"直接点击大类、小类/后代分类或未分类标题立即移动",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-451-PRODUCT-MASTER-SUBCATEGORY-HEADERS",
			"子组作为独立分类标题缩进展示",
			"商品行跟随所在父组/子组缩进",
			"移动商品到子类后列表在子类标题下显示",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"页面按“模板 → 大类 → 小类/后代分类 → 商品表”内联展示",
			"空分类和统一 `未分类` 都保留",
			"直接点击大类、小类/后代分类或未分类标题即完成移动",
			"移动会覆盖旧归类",
		},
		filepath.Join("docs", "acceptance", "2026-06-08-product-master-subcategory-headers.md"): {
			"PR-451",
			"商品档案子类分组标题",
			"商品-咖啡熟豆 / 意式拼配豆",
			"商品行跟随所在父组/子组缩进",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-451 marker %q", rel, want)
			}
		}
	}
}
