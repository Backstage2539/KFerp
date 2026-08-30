package support

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDev616DripMultistageFlowContracts(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, row := range []struct{ table, code, status, assignee string }{
		{"req_product", "PR-616-DRIP-MULTISTAGE-MANUFACTURING-FLOW", "review", "VA"},
		{"req_dev", "DEV-616-PRICING-RULE-RIGHT-DRAWER", "done", "Codex"},
		{"req_dev", "DEV-616-DRIP-MULTISTAGE-BOM-CONFIG", "done", "Codex"},
		{"req_dev", "DEV-616-DRIP-PRICE-LIST-ORDER-E2E", "done", "Codex"},
		{"req_dev", "DEV-616-DOCS-DEVELOPMENT-DELIVERY", "done", "Codex"},
		{"req_review", "REV-616-DRIP-MULTISTAGE-MANUFACTURING-FLOW", "todo", "VA"},
	} {
		pattern := regexp.MustCompile(`(?m)^[\t ]*\{table: "` + regexp.QuoteMeta(row.table) + `"[^\n]*code: "` + regexp.QuoteMeta(row.code) + `"[^\n]*status: "` + regexp.QuoteMeta(row.status) + `"[^\n]*assignee: "` + regexp.QuoteMeta(row.assignee) + `"[^\n]*\},[\t ]*$`)
		if !pattern.MatchString(reqStore) {
			t.Fatalf("missing %s %s", row.table, row.code)
		}
	}

	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-616-DRIP-MULTISTAGE-MANUFACTURING-FLOW", "生豆 → 烘焙熟豆半成品 → 咖啡粉 → 挂耳包 → 盒装挂耳", "右侧抽屉",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-616-DRIP-MULTISTAGE-MANUFACTURING-FLOW", "默认已发布 BOM", "价格表", "录单",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-616", "编辑价格模板", "右侧抽屉",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-616", "研磨", "挂耳包装", "盒装包装",
		},
		filepath.Join("docs", "acceptance", "2026-08-29-drip-multistage-manufacturing-flow.md"): {
			"PR-616", "## RED 证据", "## GREEN 证据", "development", "操作日志",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing %q", rel, want)
			}
		}
	}
}
