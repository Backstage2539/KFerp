package support

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDev606ProductionBomSemiFinishedCutoverContracts(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, row := range []struct {
		table, code, status, assignee string
	}{
		{"req_product", "PR-606-PRODUCTION-BOM-SEMI-FINISHED-CUTOVER", "review", "VA"},
		{"req_dev", "DEV-606-BOM-DRAFT-EDITOR-RELIABILITY", "done", "Codex"},
		{"req_dev", "DEV-606-PUBLISHED-OUTPUT-REPLACEMENT-DRAFT", "done", "Codex"},
		{"req_dev", "DEV-606-SEMI-FINISHED-CUTOVER-MIGRATION", "done", "Codex"},
		{"req_dev", "DEV-606-DOCS-RELEASE-ACCEPTANCE", "done", "Codex"},
		{"req_review", "REV-606-PRODUCTION-BOM-SEMI-FINISHED-CUTOVER", "todo", "VA"},
	} {
		pattern := regexp.MustCompile(`(?m)^[\t ]*\{table: "` + regexp.QuoteMeta(row.table) + `"[^\n]*code: "` + regexp.QuoteMeta(row.code) + `"[^\n]*status: "` + regexp.QuoteMeta(row.status) + `"[^\n]*assignee: "` + regexp.QuoteMeta(row.assignee) + `"[^\n]*\},[\t ]*$`)
		if !pattern.MatchString(reqStore) {
			t.Fatalf("missing %s %s status=%s assignee=%s", row.table, row.code, row.status, row.assignee)
		}
	}

	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"):                                                  {"PR-606-PRODUCTION-BOM-SEMI-FINISHED-CUTOVER", "published_output_identity_immutable", "31 个启用 BOM"},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"):                                              {"PR-606-PRODUCTION-BOM-SEMI-FINISHED-CUTOVER", "62.11%/18.63%/24.84%/18.63%", "26 个完整配方"},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"):                                          {"PR-606 已发布 BOM 替代草稿与损耗编辑", "保存失败", "替代 BOM/V001"},
		filepath.Join("docs", "acceptance", "2026-08-24-production-bom-semi-finished-cutover.md"): {"PR-606", "## RED 证据", "## GREEN 证据", "生产数据"},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing %q", rel, want)
			}
		}
	}

	bomView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue")))
	for _, want := range []string{"data-bom-workspace-feedback", "bomWorkspaceSaveFailed", "replacement-draft", "handleVersionMaterialLossRateInput"} {
		if !strings.Contains(bomView, want) {
			t.Fatalf("BomView missing PR-606 marker %q", want)
		}
	}
}
