package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestProductFormulaMenuClickMatrixEvidenceExists(t *testing.T) {
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))
	for _, want := range []string{
		"PRODUCT_FORMULA_MENU_CLICK_MATRIX_SMOKE_OK",
		"views=4",
		"productSettings",
		"mallSettings",
		"costing",
		"bom",
		"create_public_product",
		"save_category",
		"select_customer_sku",
		"save_product_basics",
		"create_mall_product",
		"open_costing_settings",
		"save_costing_run",
		"publish_costing_run",
		"open_bean_list",
		"select_bom_product",
		"sync_bom",
		"save_bom_item",
		"save_bom_version",
		"port_18162_free",
		"port_9241_free",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing product/formula menu click matrix marker %q", want)
		}
	}

	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-271-PRODUCT-FORMULA-MENU-CLICK-MATRIX",
		"DEV-271-PRODUCT-FORMULA-MENU-CLICK-MATRIX",
		"PRODUCT_FORMULA_MENU_CLICK_MATRIX_SMOKE_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing product/formula menu click matrix marker %q", want)
		}
	}
}

func TestProductFormulaMenuClickMatrixViewsExposeActions(t *testing.T) {
	checks := map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"/api/product-settings",
			"/api/product-settings/products",
			"/api/product-settings/categories",
			"/api/products/",
			"创建公共产品",
			"选择履约客户",
			"商品基础信息已保存",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "MallSettingsView.vue"): {
			"/api/customer-portal/admin/mall-products",
			"新增商品",
			"保存商品",
			"商城商品已保存",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue"): {
			"/api/costing/bean-list",
			"/api/costing/bean-list/publications",
			"已发布价格表",
			"生成价格表",
		},
		filepath.Join("frontend-vue-shell", "src", "components", "CostingSettingsPanel.vue"): {
			"/api/costing/settings",
			"保存",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue"): {
			"/api/production-boms?status=all",
			"/api/production-boms/${id}${query}",
			"/api/production-bom-versions/${draftVersionID}/draft",
			"apiGet('/api/business-groups')",
			"/api/business-group-assignments",
			"/api/production-boms/${bomID}/versions",
			"/api/product-settings/units",
			"保存组件",
			"前往分组模板",
			"复制为新版草稿",
			"产出数量",
		},
	}

	for rel, wants := range checks {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing product/formula click matrix marker %q", rel, want)
			}
		}
	}
}
