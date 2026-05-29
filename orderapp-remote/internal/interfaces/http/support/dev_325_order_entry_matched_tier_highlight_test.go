package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev325OrderEntryMatchedTierHighlightSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-325-ORDER-ENTRY-MATCHED-TIER-HIGHLIGHT",
		"DEV-325-ORDER-ENTRY-MATCHED-TIER-HIGHLIGHT",
		"UT-325-ORDER-ENTRY-MATCHED-TIER-HIGHLIGHT",
		"API-325-ORDER-ENTRY-MATCHED-TIER-HIGHLIGHT",
		"REV-325-ORDER-ENTRY-MATCHED-TIER-HIGHLIGHT",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("dev 325 requirement seed missing %q", want)
		}
	}
}

func TestDev325OrderEntryMatchedTierHighlightWiring(t *testing.T) {
	libSrc := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "order-entry.js")))
	for _, want := range []string{
		"export function isOrderTierActive(row, tier)",
		"rowTierID === 'auto'",
		"rowTierID === 'manual'",
		"return rowTierID === tierID",
	} {
		if !strings.Contains(libSrc, want) {
			t.Fatalf("order-entry lib tier highlight wiring missing %q", want)
		}
	}

	viewSrc := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue")))
	for _, want := range []string{
		"isOrderTierActive",
		"return isOrderTierActive(row, tier)",
	} {
		if !strings.Contains(viewSrc, want) {
			t.Fatalf("order entry view tier highlight wiring missing %q", want)
		}
	}
}

func TestDev325OrderEntryMatchedTierHighlightDocs(t *testing.T) {
	for _, rel := range []string{
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "acceptance", "2026-05-22-order-entry-matched-tier-highlight.md"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"PR-325",
			"高亮",
			"tier_id",
		} {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing dev 325 documentation marker %q", rel, want)
			}
		}
	}
}
