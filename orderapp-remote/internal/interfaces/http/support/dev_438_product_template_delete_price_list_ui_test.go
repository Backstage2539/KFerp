package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev438ProductTemplateDeletePriceListUISeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-438-PRODUCT-TEMPLATE-DELETE-PRICE-LIST-UI",
		"DEV-438-PRICE-LIST-PUBLICATION-UI",
		"DEV-438-DELETED-TEMPLATE-HIDDEN",
		"DEV-438-BOM-ACTIVE-OUTPUT-PRODUCTS",
		"DEV-438-GRADIENT-UNIT-DICTIONARY",
		"DEV-438-CONFIG-TEMPLATE-DELETE-DEACTIVATE",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PR-438 requirement seed missing %q", want)
		}
	}
}

func TestDev438ProductTemplateDeletePriceListUIWiringAndDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue"): {
			"publicationListSearch",
			"publicationListCollapsed",
			"paginatedCurrentScopePublicationRows",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"visibleNonDeletedRows",
			"deleteProductConfigTemplate",
			"visibleProductUnitDefinitions",
			"visibleProductClassificationTemplates",
		},
		filepath.Join("internal", "infrastructure", "postgres", "catalog", "repository.go"): {
			"DeleteProductConfigTemplate",
			"deleted_at IS NULL",
			"unit_template_inactive_skipped_for_deactivate",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-438-PRODUCT-TEMPLATE-DELETE-PRICE-LIST-UI",
			"已发布价格表",
			"删除不等于失效",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-438-PRODUCT-TEMPLATE-DELETE-PRICE-LIST-UI",
			"分页",
			"删除后不再展示",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"已发布价格表",
			"搜索",
			"分页",
			"收起",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-438-PRODUCT-TEMPLATE-DELETE-PRICE-LIST-UI",
			"商品配置模板",
			"删除后不再展示",
		},
		filepath.Join("docs", "OP_MANUAL_SETTINGS_AUDIT.md"): {
			"全局单位字典",
			"删除后不再展示",
			"删除不等于失效",
		},
		filepath.Join("docs", "acceptance", "2026-06-06-product-template-delete-price-list-ui.md"): {
			"PR-438-PRODUCT-TEMPLATE-DELETE-PRICE-LIST-UI",
			"unit template inactive",
			"删除不等于失效",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-438 marker %q", rel, want)
			}
		}
	}
}
