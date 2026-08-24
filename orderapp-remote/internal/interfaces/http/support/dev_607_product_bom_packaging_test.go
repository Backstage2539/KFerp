package support

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDev607ProductBomPackagingContracts(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, row := range []struct{ table, code, status, assignee string }{
		{"req_product", "PR-607-PRODUCT-BOM-PACKAGING", "review", "VA"},
		{"req_dev", "DEV-607-PRODUCT-BOM-MIGRATION", "done", "Codex"},
		{"req_dev", "DEV-607-STARTUP-BINDING-PROTECTION", "done", "Codex"},
		{"req_dev", "DEV-607-DOCS-RELEASE-ACCEPTANCE", "done", "Codex"},
		{"req_review", "REV-607-PRODUCT-BOM-PACKAGING", "todo", "VA"},
	} {
		pattern := regexp.MustCompile(`(?m)^[\t ]*\{table: "` + regexp.QuoteMeta(row.table) + `"[^\n]*code: "` + regexp.QuoteMeta(row.code) + `"[^\n]*status: "` + regexp.QuoteMeta(row.status) + `"[^\n]*assignee: "` + regexp.QuoteMeta(row.assignee) + `"[^\n]*\},[\t ]*$`)
		if !pattern.MatchString(reqStore) {
			t.Fatalf("missing %s %s", row.table, row.code)
		}
	}
	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"):                                           {"PR-607-PRODUCT-BOM-PACKAGING", "31 个商品 BOM", "标准咖啡熟豆规格80g-2.5kg"},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"):                                       {"PR-607-PRODUCT-BOM-PACKAGING", "26 个已发布默认", "5 个待补草稿"},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"):                                   {"半成品生产阶段", "商品分装阶段", "待补半成品配方"},
		filepath.Join("docs", "acceptance", "2026-08-25-product-bom-packaging-cutover.md"): {"PR-607", "## RED 证据", "## GREEN 证据", "生产数据"},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing %q", rel, want)
			}
		}
	}
	command := string(readOrderAppFileForTest(t, filepath.Join("cmd", "bom-product-packaging-cutover", "main.go")))
	for _, want := range []string{"PreviewPR607ProductPackagingCutover", "ApplyPR607ProductPackagingCutover", "RollbackPR607ProductPackagingCutover"} {
		if !strings.Contains(command, want) {
			t.Fatalf("command missing %q", want)
		}
	}
	bomView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue")))
	for _, want := range []string{"data-product-packaging-bom-help", "半成品 BOM 负责烘焙", "商品 BOM 负责按规格"} {
		if !strings.Contains(bomView, want) {
			t.Fatalf("BomView missing %q", want)
		}
	}
}
