package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev560ProductionIssueCompactDiagnosticsContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-560-PRODUCTION-ISSUE-COMPACT-DIAGNOSTICS",
			"DEV-560-COMPACT-MATERIAL-ROWS",
			"DEV-560-RAW-STOCK-DIAGNOSTIC",
			"DEV-560-WIP-SHORTAGE-CONSISTENCY",
			"DEV-560-DOCS-DELIVERY",
		},
		filepath.Join("internal", "infrastructure", "postgres", "stock", "stock_document.go"): {
			"原料仓",
			"库存不足：",
			"当前剩余 WIP 缺口",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "StockEntriesView.vue"): {
			"compact-production-items",
			"compact-production-item-grid",
			"领用数量",
			"库存单位",
			"指定批次",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-560-PRODUCTION-ISSUE-COMPACT-DIAGNOSTICS",
			"原料仓库存不足",
			"过期草稿",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"K102. 生产领料紧凑明细与库存诊断",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-560-PRODUCTION-ISSUE-COMPACT-DIAGNOSTICS",
			"当前剩余 WIP 缺口",
		},
		filepath.Join("docs", "OP_MANUAL_STOCK.md"): {
			"PR-560-PRODUCTION-ISSUE-COMPACT-DIAGNOSTICS",
			"原料仓库存不足",
		},
		filepath.Join("docs", "acceptance", "2026-07-28-production-issue-compact-diagnostics.md"): {
			"PR-560 生产领料紧凑明细与库存诊断验收",
		},
	} {
		source := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(source, want) {
				t.Fatalf("%s missing PR-560 marker %q", rel, want)
			}
		}
	}
}
