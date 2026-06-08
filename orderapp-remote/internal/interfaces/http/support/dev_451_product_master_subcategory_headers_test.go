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
			"classificationGroupIndentStyle",
			"classificationItemIndentStyle",
			"--classification-group-indent",
			"--classification-item-indent",
			"classification-item-row",
			"path_label",
			"productBusinessGroupItemOptions",
			"includeGroupName: false",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.test.js"): {
			"商品-咖啡熟豆 / 意式拼配豆",
			"depth: 1",
			"parent: 90",
			"classification-subgroup-row",
			"classificationItemIndentStyle",
			"classification-item-row",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-451-PRODUCT-MASTER-SUBCATEGORY-HEADERS",
			"父组和子组都可以成为分类标题",
			"商品行跟随所在父组/子组缩进",
			"可把商品移动到具体小类",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-451-PRODUCT-MASTER-SUBCATEGORY-HEADERS",
			"子组作为独立分类标题缩进展示",
			"商品行跟随所在父组/子组缩进",
			"移动商品到子类后列表在子类标题下显示",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"父组和子组都可以形成分类标题",
			"商品行跟随所在父组/子组缩进",
			"移动目标可选 `未分类`、大类或小类",
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
