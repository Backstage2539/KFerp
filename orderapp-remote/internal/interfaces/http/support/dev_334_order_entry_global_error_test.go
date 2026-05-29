package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev334OrderEntryGlobalErrorSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-334-ORDER-ENTRY-GLOBAL-ERROR-TOAST",
		"DEV-334-ORDER-ENTRY-GLOBAL-ERROR-TOAST",
		"UT-334-ORDER-ENTRY-GLOBAL-ERROR-TOAST",
		"API-334-ORDER-ENTRY-GLOBAL-ERROR-TOAST",
		"REV-334-ORDER-ENTRY-GLOBAL-ERROR-TOAST",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PR-334 requirement seed missing %q", want)
		}
	}
}

func TestDev334OrderEntryGlobalErrorWiring(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue")))
	for _, want := range []string{
		`class="global-error-toast notice error"`,
		`role="alert"`,
		`class="toast-close"`,
		`position: fixed`,
		`z-index: 80`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("OrderEntryView.vue missing global error marker %q", want)
		}
	}
}

func TestDev334OrderEntryGlobalErrorDocs(t *testing.T) {
	for _, rel := range []string{
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "acceptance", "2026-05-23-order-entry-global-error-toast.md"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"PR-334-ORDER-ENTRY-GLOBAL-ERROR-TOAST",
			"全局",
			"错误提示",
		} {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-334 documentation marker %q", rel, want)
			}
		}
	}
}
