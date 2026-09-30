package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev674PriceTableTabsRequirementAndManualContracts(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		`code: "PR-674-PRICE-TABLE-EDITOR-TABS"`,
		`code: "DEV-674-NAMED-TABLE-TABS"`,
		`code: "DEV-674-DOCS-DELIVERY"`,
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing PR-674 seed %q", want)
		}
	}

	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-674-PRICE-TABLE-EDITOR-TABS", "横向滚动", "价格表配置",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-674-PRICE-TABLE-EDITOR-TABS", "键盘", "草稿",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-674", "Tab", "横向滚动", "价格表配置",
		},
		filepath.Join("docs", "acceptance", "2026-09-29-price-table-editor-tabs.md"): {
			"PR-674", "RED", "GREEN", "API",
		},
	} {
		body := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing PR-674 marker %q", rel, want)
			}
		}
	}
}
