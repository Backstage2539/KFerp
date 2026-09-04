package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev626GroupListInteractionDeliveryContracts(t *testing.T) {
	checks := map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-626-GROUP-LIST-SEARCH-PAGINATION-MOVE",
			"DEV-626-GROUP-SEARCH-FOCUS",
			"DEV-626-GROUP-PAGINATION-THRESHOLD",
			"DEV-626-COMPACT-MOVE-TARGETS",
			"DEV-626-BULK-EXPAND-COLLAPSE",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"# PR-626-GROUP-LIST-SEARCH-PAGINATION-MOVE",
			"分类总数不超过 10 条",
			"全部展开",
			"全部收缩",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"## PR-626-GROUP-LIST-SEARCH-PAGINATION-MOVE",
			"搜索前的折叠和滚动位置",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-626 分组列表搜索、分页与移动模式",
			"全部收缩",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-626 分组列表交互",
			"全部收缩",
		},
		filepath.Join("docs", "acceptance", "2026-09-03-group-list-search-pagination-move.md"): {
			"# PR-626",
			"RED",
			"GREEN",
		},
		filepath.Join("frontend-vue-shell", "src", "components", "BusinessGroupInlineWorkspace.vue"): {
			"searchQuery",
			"data-business-group-item-row",
			"businessGroupMoveCollapsedKeys",
			"setAllGroupsCollapsed",
		},
	}
	for path, markers := range checks {
		body := string(readOrderAppFileForTest(t, path))
		for _, marker := range markers {
			if !strings.Contains(body, marker) {
				t.Errorf("%s missing %q", path, marker)
			}
		}
	}
}
