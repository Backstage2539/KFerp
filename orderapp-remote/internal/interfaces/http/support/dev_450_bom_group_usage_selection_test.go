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
			"selectedProductionBomUseGroupID",
			"productionBomUseGroupOptions",
			"productionBomMoveGroupOptions",
			"useSelectedProductionBomGroup",
			"使用分组",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"): {
			"includeGroupName",
			"businessGroupAssignmentLabel",
			"businessGroupItemMoveOptions",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"使用分组",
			"顶部筛选 Tab 只显示已经被生产 BOM 启用的分组项",
			"不显示“商品分组 /”这类分组集名称",
		},
		filepath.Join("docs", "acceptance", "2026-06-08-production-bom-group-usage-selection.md"): {
			"PR-450",
			"POST /api/business-groups/:id/usages",
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
