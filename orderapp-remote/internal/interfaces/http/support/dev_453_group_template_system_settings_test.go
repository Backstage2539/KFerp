package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev453GroupTemplateSystemSettingsContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-453-GROUP-TEMPLATE-SYSTEM-SETTINGS",
			"DEV-453-GROUP-TEMPLATE-SETTINGS-UI",
			"DEV-453-GROUP-TEMPLATE-CONSUMER-PAGES",
			"DEV-453-GROUP-TEMPLATE-DOWNSTREAM",
			"DEV-453-GROUP-TEMPLATE-DOCS",
			"REV-453-GROUP-TEMPLATE-SYSTEM-SETTINGS",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js"): {
			"{ key: 'uiSettings', label: '系统设置'",
			"{ key: 'businessSettings', label: '业务设置'",
			"groupManagement: '分组模板'",
			"productCategoryManagement: '分组模板'",
		},
		filepath.Join("frontend-vue-shell", "src", "App.vue"): {
			"groupManagement: GroupTemplatesView",
			"groupTemplates: GroupTemplatesView",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "GroupTemplatesView.vue"): {
			`data-section-mode="groupTemplates"`,
			"新增分组模板",
			"新增大类",
			"新增小类",
			"/api/business-groups",
			"/api/business-group-items",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"selectedProductGroupTemplateID",
			"BusinessGroupControls",
			"productBusinessGroupControls",
			"clearProductBusinessGroupAssignment",
			"businessGroupMoveAssignmentPayload",
			"/api/business-group-assignments",
			`data-section-mode="groupTemplatesRetired"`,
		},
		filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue"): {
			"selectedProductionBomTemplateID",
			"productionBomTemplateOptions",
			"BusinessGroupControls",
			"groupRowsByBusinessGroupTemplate",
			"businessGroupMoveAssignmentPayload",
			"/api/business-group-assignments",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "WarehouseInventoryView.vue"): {
			"selectedWarehouseGroupTemplateID",
			"warehouseGroupItemOptions",
			"BusinessGroupControls",
			"groupRowsByBusinessGroupTemplate",
			"/api/business-group-assignments",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-453-GROUP-TEMPLATE-SYSTEM-SETTINGS",
			"设置 / 分组模板",
			"生产 BOM 取消 `使用分组`",
			"usage_key",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-453-GROUP-TEMPLATE-SYSTEM-SETTINGS",
			"独立 `分组模板` 页面只维护模板名、大类、小类",
			"商品档案、生产 BOM、仓库库存页面先选择 `分组模板`",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-453-GROUP-TEMPLATE-SYSTEM-SETTINGS",
			"设置 / 分组模板",
			"生产 BOM 页面不再维护自己的大组、组内分类或小分类，也不再显示 `使用分组`",
		},
		filepath.Join("docs", "acceptance", "2026-06-08-group-template-system-settings.md"): {
			"PR-453",
			"分组模板移入系统设置",
			"生产 BOM 不再显示 `使用分组`",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-453 marker %q", rel, want)
			}
		}
	}
}

func TestDev453GroupTemplatePagesDoNotExposeLegacyObjectManagement(t *testing.T) {
	menu := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js")))
	productGroupStart := strings.Index(menu, "id: 'product'")
	settingsGroupStart := strings.Index(menu, "id: 'settings'")
	if productGroupStart < 0 || settingsGroupStart < 0 || settingsGroupStart <= productGroupStart {
		t.Fatalf("menu group order markers missing")
	}
	productMenu := menu[productGroupStart:settingsGroupStart]
	if strings.Contains(productMenu, "groupManagement") || strings.Contains(productMenu, "分组管理") {
		t.Fatalf("product menu must not expose ordinary group management entry")
	}

	panel := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "GroupTemplatesView.vue")))
	for _, forbidden := range []string{"移动到分类", "/api/business-group-assignments", "已选", "勾选对象"} {
		if strings.Contains(panel, forbidden) {
			t.Fatalf("group template page must not manage business objects, found %q", forbidden)
		}
	}
	settings := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "UISettingsView.vue")))
	for _, forbidden := range []string{"data-section-mode=\"groupTemplates\"", "/api/business-groups", "/api/business-group-items"} {
		if strings.Contains(settings, forbidden) {
			t.Fatalf("system settings page must not manage group templates, found %q", forbidden)
		}
	}

	bom := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue")))
	for _, forbidden := range []string{"selectedProductionBomUseGroupID", "productionBomUseGroupOptions", "useSelectedProductionBomGroup", "/api/business-groups/${groupID}/usages"} {
		if strings.Contains(bom, forbidden) {
			t.Fatalf("BOM page must not expose old use-group flow, found %q", forbidden)
		}
	}
	if strings.Contains(bom, ">使用分组<") {
		t.Fatalf("BOM page must not show 使用分组 button")
	}

	warehouse := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "WarehouseInventoryView.vue")))
	if strings.Contains(warehouse, "business-groups?usage_key=warehouse_inventory") {
		t.Fatalf("warehouse inventory should load enabled group templates without usage-key prefilter")
	}
}
