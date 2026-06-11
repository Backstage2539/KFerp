package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev470PriceListArchiveWarningContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-470-PRICE-LIST-ARCHIVE-WARNING-FALLBACK",
			"DEV-470-PRICE-LIST-WARNING-FALLBACK",
			"DEV-470-PRICE-LIST-PUBLICATION-ARCHIVE",
			"DEV-470-DOCS-ACCEPTANCE",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue"): {
			"visibleItemWarnings",
			"itemHasResolvedPriceListPricingMethod",
			"归档选中",
			"归档列表",
			"移出归档",
			"selectedPublicationArchiveIDs",
			"currentScopeArchivedPublicationRows",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "costing-bean-list-version-ui.test.js"): {
			"product price-list suppresses missing pricing method warning when price-list fallback resolves",
			"product price-list published versions can be archived and restored from archive list",
		},
		filepath.Join("internal", "interfaces", "http", "costing", "costing_api_test.go"): {
			"TestBeanListPublicationArchiveAPI",
			"/api/costing/bean-list/publications/archive",
			"/api/costing/bean-list/publications/unarchive",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-470-PRICE-LIST-ARCHIVE-WARNING-FALLBACK",
			"价格表的计价模式托底",
			"归档列表",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-470-PRICE-LIST-ARCHIVE-WARNING-FALLBACK",
			"未设置计价方式",
			"移出归档",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-470-PRICE-LIST-ARCHIVE-WARNING-FALLBACK",
			"归档选中",
			"归档列表",
		},
		filepath.Join("docs", "acceptance", "2026-06-11-price-list-archive-warning-fallback.md"): {
			"PR-470-PRICE-LIST-ARCHIVE-WARNING-FALLBACK",
			"未设置计价方式",
			"归档列表",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-470 marker %q", rel, want)
			}
		}
	}
}
