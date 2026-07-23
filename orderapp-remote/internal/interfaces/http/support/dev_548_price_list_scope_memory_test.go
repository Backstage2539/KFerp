package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev548PriceListScopeMemoryContracts(t *testing.T) {
	store := readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	for _, want := range []string{
		`code: "PR-548-PRICE-LIST-SCOPE-MEMORY"`,
		`code: "DEV-548-COMPACT-TOP-TOOLBAR"`,
		`code: "DEV-548-BROWSER-SELECTION-MEMORY"`,
		`code: "DEV-548-DOCS-DELIVERY"`,
	} {
		if !strings.Contains(string(store), want) {
			t.Fatalf("req_store.go missing PR-548 seed %q", want)
		}
	}

	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {"PR-548-PRICE-LIST-SCOPE-MEMORY", "浏览器"},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {"K90", "价格表归属", "商品类型"},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {"PR-548", "管理阶梯模板", "记住"},
		filepath.Join("docs", "acceptance", "2026-07-23-price-list-scope-memory.md"): {"PR-548", "RED", "GREEN"},
	} {
		body := readOrderAppFileForTest(t, rel)
		for _, want := range wants {
			if !strings.Contains(string(body), want) {
				t.Fatalf("%s missing PR-548 marker %q", rel, want)
			}
		}
	}
}
