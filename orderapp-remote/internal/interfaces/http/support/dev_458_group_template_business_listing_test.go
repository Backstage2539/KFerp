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
			"选择分组模板",
			"目标分类",
			"移动到分类",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"BusinessGroupControls",
			"productBusinessGroupControls",
			"groupRowsByBusinessGroupTemplate",
			"v-for=\"group in renderedDisplaySkuGroups\"",
			"businessGroupMoveAssignmentPayload",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue"): {
			"BusinessGroupControls",
			"productionBomDisplayGroups",
			"groupRowsByBusinessGroupTemplate",
			"businessGroupMoveAssignmentPayload",
			"v-for=\"group in productionBomDisplayGroups\"",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "WarehouseInventoryView.vue"): {
			"BusinessGroupControls",
			"warehouseDisplayGroups",
			"selectedWarehouseKeys",
			"groupRowsByBusinessGroupTemplate",
			"businessGroupMoveAssignmentPayload",
			"v-for=\"group in warehouseDisplayGroups\"",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-458-GROUP-TEMPLATE-BUSINESS-LISTING",
			"business-grouping",
			"BusinessGroupControls",
			"空大类和空小类也必须显示",
			"商品表格不再显示独立 `分类` 列",
			"仓库库存页面不再使用 `普通仓库`、`客户仓库` 固定分段",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-458-GROUP-TEMPLATE-BUSINESS-LISTING",
			"商品档案不出现分类过滤 Tab",
			"生产 BOM 页面不出现 `使用分组`，也不出现 `全部分类 / 未分类 / 分类项` 过滤 Tab",
			"仓库库存不出现 `普通仓库`、`客户仓库` 固定分段",
			"三处页面都引用共享 `BusinessGroupControls` 和 `business-grouping` helper",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-458-GROUP-TEMPLATE-BUSINESS-LISTING",
			"商品档案不再显示分类过滤 Tab",
			"生产 BOM 页面不再维护自己的大组",
			"页面不再固定分成 `普通仓库` / `客户仓库`",
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
