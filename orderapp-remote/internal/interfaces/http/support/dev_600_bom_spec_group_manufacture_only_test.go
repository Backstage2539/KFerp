package support

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDev600BomSpecGroupManufactureOnlyContracts(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, row := range []struct {
		table, code, status, assignee string
	}{
		{"req_product", "PR-600-BOM-SPEC-GROUP-MANUFACTURE-ONLY-SEMI-FINISHED", "review", "VA"},
		{"req_dev", "DEV-600-BOM-RECIPE-MODE", "done", "Codex"},
		{"req_dev", "DEV-600-SEMI-FINISHED-MANUFACTURE-ONLY", "done", "Codex"},
		{"req_dev", "DEV-600-BOM-SPEC-TEMPLATE-GROUP", "done", "Codex"},
		{"req_dev", "DEV-600-BOM-SPEC-BUSINESS-IDENTITY", "done", "Codex"},
		{"req_dev", "DEV-600-PER-PRODUCT-MIGRATION", "done", "Codex"},
		{"req_dev", "DEV-600-VUE-DOCS-DEVELOPMENT-DELIVERY", "todo", "Codex"},
		{"req_review", "REV-600-BOM-SPEC-GROUP-MANUFACTURE-ONLY-SEMI-FINISHED", "todo", "VA"},
	} {
		requireDev600SeedRow(t, reqStore, row.table, row.code, row.status, row.assignee)
	}

	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-600-BOM-SPEC-GROUP-MANUFACTURE-ONLY-SEMI-FINISHED", "BOM 专属规格", "制造专供", "逐商品切换",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-600-BOM-SPEC-GROUP-MANUFACTURE-ONLY-SEMI-FINISHED", "整组原子发布", "不自动生成配方", "父商品 + BOM 规格",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-600", "BOM 规格模板", "比例模式", "固定模式",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-600", "半成品只能由生产获得", "采购价为 0",
		},
		filepath.Join("docs", "acceptance", "2026-08-17-bom-spec-group-manufacture-only.md"): {
			"PR-600", "## RED 证据", "## GREEN 证据", "development only",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-600 marker %q", rel, want)
			}
		}
	}

	bomView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue")))
	for _, forbidden := range []string{"有损耗的配方", "无损耗的配方", "material-loss-zones"} {
		if strings.Contains(bomView, forbidden) {
			t.Fatalf("BomView.vue retains superseded loss-zone marker %q", forbidden)
		}
	}
	for _, want := range []string{"BOM 规格模板", "规格组", "主投入物料"} {
		if !strings.Contains(bomView, want) {
			t.Fatalf("BomView.vue missing PR-600 marker %q", want)
		}
	}

	for _, rel := range []string{
		filepath.Join("internal", "application", "bom", "service.go"),
		filepath.Join("internal", "infrastructure", "postgres", "bom", "repository.go"),
		filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, forbidden := range []string{"bom_kind", "spec_packaging", "semi_finished_to_packaging"} {
			if strings.Contains(src, forbidden) {
				t.Fatalf("%s revives rejected special BOM marker %q", rel, forbidden)
			}
		}
	}
}

func requireDev600SeedRow(t *testing.T, src, table, code, status, assignee string) {
	t.Helper()
	pattern := regexp.MustCompile(`(?m)^[\t ]*\{table: "` + regexp.QuoteMeta(table) + `"[^\n]*code: "` + regexp.QuoteMeta(code) + `"[^\n]*status: "` + regexp.QuoteMeta(status) + `"[^\n]*assignee: "` + regexp.QuoteMeta(assignee) + `"[^\n]*\},[\t ]*$`)
	if !pattern.MatchString(src) {
		t.Fatalf("req_store.go missing one-line %s seed %s with status %s and assignee %s", table, code, status, assignee)
	}
}
