package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev458GroupTemplateBusinessListingContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-458-GROUP-TEMPLATE-BUSINESS-LISTING",
			"DEV-458-GROUPING-HELPER-CONTROL",
			"DEV-458-PRODUCT-LIST-TEMPLATE-TREE",
			"DEV-458-BOM-LIST-TEMPLATE-TREE",
			"DEV-458-WAREHOUSE-LIST-TEMPLATE-TREE",
			"DEV-458-DOCS-ACCEPTANCE",
			"REV-458-GROUP-TEMPLATE-BUSINESS-LISTING",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "business-grouping.js"): {
			"groupRowsByBusinessGroupTemplate",
			"businessGroupMoveAssignmentPayload",
			"businessGroupControlOptions",
			"includeGroupsWithoutUsage: true",
			"business-group-unclassified",
		},
		filepath.Join("frontend-vue-shell", "src", "components", "BusinessGroupControls.vue"): {
			"data-business-group-controls",
			"moveActive",
			"breadcrumb",
			"移动到分类",
		},
		filepath.Join("frontend-vue-shell", "src", "components", "BusinessGroupWorkspace.vue"): {
			"data-business-group-workspace",
			"请选择要移动到的分类",
			"点击大类、小类或未分类后立即移动",
			"emit('target'",
			"business-group-list-disabled",
			"beginBusinessGroupMoveState",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"BusinessGroupWorkspace",
			"selectedProductBusinessGroupCategoryKey",
			"productCategoryMoveActive",
			`@target="handleProductCategoryMoveTarget"`,
			"productBusinessGroupControls",
			"groupRowsByBusinessGroupTemplates",
			"v-for=\"group in renderedDisplaySkuGroups\"",
			"businessGroupMoveAssignmentPayload",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue"): {
			"BusinessGroupWorkspace",
			"selectedProductionBomCategoryKey",
			"productionBomCategoryMoveActive",
			`@target="handleProductionBomCategoryMoveTarget"`,
			"productionBomDisplayGroups",
			"groupRowsByBusinessGroupTemplates",
			"businessGroupMoveAssignmentPayload",
			`v-for="group in productionBomDisplayGroups"`,
		},
		filepath.Join("frontend-vue-shell", "src", "views", "WarehouseInventoryView.vue"): {
			"BusinessGroupWorkspace",
			"selectedInventoryCategoryKey",
			"inventoryCategoryMoveActive",
			`@target="handleInventoryCategoryMoveTarget"`,
			"inventoryDisplayGroups",
			"selectedInventoryItemKeys",
			"groupRowsByBusinessGroupTemplates",
			"businessGroupMoveAssignmentPayload",
			`v-for="row in renderedInventoryRows"`,
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-458-GROUP-TEMPLATE-BUSINESS-LISTING",
			"business-grouping",
			"BusinessGroupControls",
			"空大类和空小类也必须显示",
			"表格不再显示独立 `分类` 列",
			"PR-458 的仓库 code 归类是历史口径",
			"warehouse_inventory_item",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-458-GROUP-TEMPLATE-BUSINESS-LISTING",
			"商品档案不出现分类过滤 Tab",
			"生产 BOM 页面不出现 `使用分组`，也不出现 `全部分类 / 未分类 / 分类项` 过滤 Tab",
			"PR-458 历史仓库 code 归类已由 PR-595 取代",
			"仓内物品/规格",
			"四处页面都引用共享 `BusinessGroupWorkspace` 和 `business-grouping` helper",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-458-GROUP-TEMPLATE-BUSINESS-LISTING",
			"商品档案归类使用 `product_catalog`",
			"左侧常驻分类结构",
			"生产 BOM 页面不再维护自己的大组",
			"外层仓库列表保持不变",
			"全部仓库和客户库存上下文仅在分类层面平铺且不可勾选移动",
			"既有 WIP/追溯等上下文能力不变",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"空分类也显示，不再提供分类过滤 Tab",
		},
		filepath.Join("docs", "acceptance", "2026-06-08-group-template-business-listing.md"): {
			"PR-458-GROUP-TEMPLATE-BUSINESS-LISTING",
			"分组模板驱动业务列表整理",
			"商品档案没有分类过滤 Tab",
			"仓库库存不再显示 `普通仓库`、`客户仓库` 固定分段",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-458 marker %q", rel, want)
			}
		}
	}
}

func TestDev458BusinessPagesDoNotExposeRetiredGroupingUI(t *testing.T) {
	controls := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "components", "BusinessGroupControls.vue")))
	for _, forbidden := range []string{"选择分组模板", "目标分类", "<select"} {
		if strings.Contains(controls, forbidden) {
			t.Fatalf("shared grouping controls must not expose the retired target dropdown marker %q", forbidden)
		}
	}

	product := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue")))
	for _, forbidden := range []string{"product-classification-tabs", "<th>分类</th>"} {
		if strings.Contains(product, forbidden) {
			t.Fatalf("product archive must not expose old classification tab/column marker %q", forbidden)
		}
	}

	bom := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue")))
	for _, forbidden := range []string{"bom-list-tabs-row", "productionBomUsedGroupOptions", "selectedProductionBomGroupItemID", ">全部分类<", ">分类项<"} {
		if strings.Contains(bom, forbidden) {
			t.Fatalf("production BOM must not expose old category tab marker %q", forbidden)
		}
	}

	warehouse := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "WarehouseInventoryView.vue")))
	for _, forbidden := range []string{"warehouseSections", "generalWarehouses", "customerWarehouses", "普通仓库", "客户仓库"} {
		if strings.Contains(warehouse, forbidden) {
			t.Fatalf("warehouse inventory must not expose fixed warehouse section marker %q", forbidden)
		}
	}
}
