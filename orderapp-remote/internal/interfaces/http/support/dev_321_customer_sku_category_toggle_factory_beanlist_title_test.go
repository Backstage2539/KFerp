package support

import (
	"strings"
	"testing"
)

func TestDev321CustomerSkuCategoryToggleFactoryBeanListTitle(t *testing.T) {
	store := string(readOrderAppFileForTest(t, "internal/interfaces/http/support/req_store.go"))
	for _, want := range []string{
		"PR-321-CUSTOMER-SKU-CATEGORY-TOGGLE-FACTORY-BEANLIST-TITLE",
		"DEV-321-CUSTOMER-SKU-CATEGORY-TOGGLE-FACTORY-BEANLIST-TITLE",
		"UT-321-CUSTOMER-SKU-CATEGORY-TOGGLE-FACTORY-BEANLIST-TITLE",
		"API-321-CUSTOMER-SKU-CATEGORY-TOGGLE-FACTORY-BEANLIST-TITLE",
		"REV-321-CUSTOMER-SKU-CATEGORY-TOGGLE-FACTORY-BEANLIST-TITLE",
		"docs/acceptance/2026-05-22-customer-sku-category-toggle-factory-beanlist-title.md",
		"1、定制咖啡熟豆",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing PR-321 marker %q", want)
		}
	}

	markers := map[string][]string{
		"docs/REQUIREMENTS.md": {
			"PR-321-CUSTOMER-SKU-CATEGORY-TOGGLE-FACTORY-BEANLIST-TITLE",
			"勾选再取消“是否使用公共商品分类”",
			"从客户账户切回“工厂总览”",
			"1、定制咖啡熟豆",
		},
		"docs/ACCEPTANCE_TESTS.md": {
			"PR-321-CUSTOMER-SKU-CATEGORY-TOGGLE-FACTORY-BEANLIST-TITLE",
			"客户自有产品类型“咖啡豆”和产品子类型“定制咖啡熟豆”仍保留",
			"SKU归属回到公共SKU",
		},
		"docs/OP_MANUAL_COSTING.md": {
			"客户商品还没有展示分类时，产品价格表会把它放到“未分类”",
			"“1、定制咖啡熟豆”分组",
			"schema 修复",
		},
		"docs/OP_MANUAL_INVENTORY_MATERIALS.md": {
			"客户自有分类和已归类客户商品不会被隐藏或删除",
			"自动恢复父分类或把子分类迁到同名 active 父分类",
		},
		"docs/OP_MANUAL_WORKSPACE_MODE.md": {
			"SKU设置 的内部归属也会回到公共SKU",
			"SKU归属显示公共SKU",
		},
		"docs/acceptance/2026-05-22-customer-sku-category-toggle-factory-beanlist-title.md": {
			"芬纳定制-红酒日晒-中深烘",
			"1、定制咖啡熟豆",
			"TestProductCategorySchemaRepairsActiveChildrenWithInactiveParents",
		},
		"frontend-vue-shell/src/lib/product-settings.js": {
			"if (categoryCustomerID === customerID) return true",
			"export function nextSkuContextCustomerID",
			"workspaceMode",
		},
		"frontend-vue-shell/src/views/ProductSettingsView.vue": {
			"nextSkuContextCustomerID",
			"watch(() => [props.workspaceMode, props.customerContextId]",
		},
		"frontend-vue-shell/src/lib/product-settings.test.js": {
			"customer category tree keeps owned categories with customer SKUs when public categories are toggled",
			"factory workspace forces SKU settings context back to public SKU",
		},
		"internal/domain/costing/engine.go": {
			"func customerBeanListCategoryName",
			"return firstNonEmptyString(secondary, primary)",
		},
		"internal/infrastructure/postgres/catalog/schema.go": {
			"UPDATE %[1]s.product_categories child",
			"inactive_parent.active=false",
			"UPDATE %[1]s.product_categories parent",
			"SET active=true",
		},
	}
	for rel, wants := range markers {
		body := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing PR-321 marker %q", rel, want)
			}
		}
	}

	engine := string(readOrderAppFileForTest(t, "internal/domain/costing/engine.go"))
	if strings.Contains(engine, `return primary + " / " + secondary`) {
		t.Fatalf("customer bean-list category title must not combine primary and secondary category names")
	}
}
