package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev561WIPBulkIssueContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-561-WIP-BULK-MATERIAL-ISSUE",
			"DEV-561-WIP-SHORTAGE-AS-SUGGESTION",
			"DEV-561-BULK-DRAFT-PRESERVATION",
			"DEV-561-CONSUMPTION-BOUNDARY",
			"DEV-561-DRAFT-IDENTITY-QUANTITY-SAFETY",
			"DEV-561-ISSUE-UX-MANUAL",
			"DEV-561-DOCS-DELIVERY",
		},
		filepath.Join("internal", "application", "production", "service.go"): {
			"当前建议领用",
			"草稿保留",
			"作为可用 WIP 库存保留",
			"指定库存草稿不存在",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "StockEntriesView.vue"): {
			"工单建议领用量仅用于默认填充",
			"不限制实际领料",
			"生产消耗仍需另行记录",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-561-WIP-BULK-MATERIAL-ISSUE",
			"建议领用量",
			"真实原料仓库存",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"K103. 工单批量领料与 WIP 建议量",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-561-WIP-BULK-MATERIAL-ISSUE",
			"60Kg",
			"可用 WIP",
		},
		filepath.Join("docs", "OP_MANUAL_STOCK.md"): {
			"PR-561-WIP-BULK-MATERIAL-ISSUE",
			"建议值",
			"生产消耗",
		},
		filepath.Join("docs", "acceptance", "2026-07-28-wip-bulk-material-issue.md"): {
			"PR-561 工单批量领料与 WIP 建议量验收",
		},
	} {
		source := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(source, want) {
				t.Fatalf("%s missing PR-561 marker %q", rel, want)
			}
		}
	}

	stockSource := string(readOrderAppFileForTest(t, filepath.Join(
		"internal", "infrastructure", "postgres", "stock", "stock_document.go",
	)))
	if strings.Contains(stockSource, "物料领用数量超过当前剩余 WIP 缺口") {
		t.Fatalf("production issue must not keep the historical WIP shortage hard-limit error")
	}
	if !strings.Contains(stockSource, "%s库存不足：%s，需领用%s，可用%s，缺口%s%s") {
		t.Fatalf("production issue must retain the real raw-material stock guard")
	}
	if !strings.Contains(stockSource, "生产消耗数量超过工单剩余需求") {
		t.Fatalf("bulk WIP issue must not weaken the work-order production-consumption boundary")
	}
}
