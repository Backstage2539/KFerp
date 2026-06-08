package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev450BomGroupUsageSelectionContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-450-BOM-GROUP-USAGE-SELECTION",
			"DEV-450-BOM-GROUP-USAGE-ENABLE",
			"DEV-450-BOM-GROUP-USED-TABS",
			"DEV-450-BOM-GROUP-LABELS",
			"DEV-450-BOM-GROUP-LIST-LAYOUT",
			"REV-450-BOM-GROUP-USAGE-SELECTION",
		},
		filepath.Join("internal", "interfaces", "http", "catalog", "product_routes.go"): {
			`POST("/api/business-groups/:id/usages"`,
			"ensureBusinessGroupUsageAPI",
			"EnsureBusinessGroupUsage",
		},
		filepath.Join("internal", "application", "catalog", "service.go"): {
			"EnsureBusinessGroupUsage",
			"BusinessGroupUsageProductionBOM",
		},
		filepath.Join("internal", "infrastructure", "postgres", "catalog", "repository.go"): {
			"func (r Repository) EnsureBusinessGroupUsage",
			"ensureBusinessGroupUsageForAssignmentTx",
			"ensure_business_group_usage",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue"): {
			"selectedProductionBomTemplateID",
			"productionBomTemplateOptions",
			"productionBomMoveGroupOptions",
			"productionBomDisplayGroups",
			"BusinessGroupControls",
			"groupRowsByBusinessGroupTemplate",
			"businessGroupMoveAssignmentPayload",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"): {
			"includeGroupName",
			"businessGroupAssignmentLabel",
			"businessGroupItemMoveOptions",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"生产 BOM 页面不再维护自己的大组、组内分类或小分类，也不再显示 `使用分组`",
			"列表会按模板完整大类 / 小类树和 `未分类` 自动整理",
			"旧 `production_bom_groups` / `production_bom_group_categories` 只用于历史只读兼容",
		},
		filepath.Join("docs", "acceptance", "2026-06-08-production-bom-group-usage-selection.md"): {
			"PR-450",
			"历史兼容",
			"POST /api/business-groups/:id/usages",
			"普通生产 BOM 页面不再暴露 `使用分组`",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-450 marker %q", rel, want)
			}
		}
	}
}
