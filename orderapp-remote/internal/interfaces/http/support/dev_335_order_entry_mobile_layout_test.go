package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev335OrderEntryMobileLayoutSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-335-ORDER-ENTRY-MOBILE-LAYOUT",
		"DEV-335-ORDER-ENTRY-MOBILE-LAYOUT",
		"UT-335-ORDER-ENTRY-MOBILE-LAYOUT",
		"API-335-ORDER-ENTRY-MOBILE-LAYOUT",
		"REV-335-ORDER-ENTRY-MOBILE-LAYOUT",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PR-335 requirement seed missing %q", want)
		}
	}
}

func TestDev335OrderEntryMobileLayoutWiring(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue")))
	for _, want := range []string{
		`class="file-upload-control"`,
		`class="file-name"`,
		`grid-template-columns: 1fr`,
		`safe-area-inset-left`,
		`safe-area-inset-right`,
		`overflow-x: hidden`,
		`text-overflow: ellipsis`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("OrderEntryView.vue missing mobile layout marker %q", want)
		}
	}
}

func TestDev335OrderEntryMobileLayoutDocs(t *testing.T) {
	for _, rel := range []string{
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "acceptance", "2026-05-23-order-entry-mobile-layout.md"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"PR-335-ORDER-ENTRY-MOBILE-LAYOUT",
			"手机",
			"收款凭证",
		} {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-335 documentation marker %q", rel, want)
			}
		}
	}
}
