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
			"--classification-group-indent",
			"path_label",
			"productBusinessGroupItemOptions",
			"includeGroupName: false",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.test.js"): {
			"商品-咖啡熟豆 / 意式拼配豆",
			"depth: 1",
			"parent: 90",
			"classification-subgroup-row",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-451-PRODUCT-MASTER-SUBCATEGORY-HEADERS",
			"父组和子组都可以成为分类标题",
			"可把商品移动到具体子类",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-451-PRODUCT-MASTER-SUBCATEGORY-HEADERS",
			"子组作为独立分类标题缩进展示",
			"目标分组可选择父组或子组",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"父组和子组都可以形成分组标题",
			"目标分组",
			"选择子组会把商品移动到该子类",
			"不显示“商品分组 /”",
		},
		filepath.Join("docs", "acceptance", "2026-06-08-product-master-subcategory-headers.md"): {
			"PR-451",
			"商品档案子类分组标题",
			"商品-咖啡熟豆 / 意式拼配豆",
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
