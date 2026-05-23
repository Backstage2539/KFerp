package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev338CustomerSelectorClearDropdownSpacingSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-338-CUSTOMER-SELECTOR-CLEAR-DROPDOWN-SPACING",
		"DEV-338-CUSTOMER-SELECTOR-CLEAR-DROPDOWN-SPACING",
		"UT-338-CUSTOMER-SELECTOR-CLEAR-DROPDOWN-SPACING",
		"API-338-CUSTOMER-SELECTOR-CLEAR-DROPDOWN-SPACING",
		"REV-338-CUSTOMER-SELECTOR-CLEAR-DROPDOWN-SPACING",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PR-338 requirement seed missing %q", want)
		}
	}
}

func TestDev338CustomerSelectorClearDropdownSpacingWiring(t *testing.T) {
	searchableSelect := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "components", "SearchableSelect.vue")))
	for _, want := range []string{
		"class=\"select-clear\"",
		"aria-label=\"清除选择\"",
		"type=\"text\"",
		"right: 36px",
		"padding: 7px 70px 7px 9px",
		"clearSelection",
	} {
		if !strings.Contains(searchableSelect, want) {
			t.Fatalf("SearchableSelect.vue missing PR-338 marker %q", want)
		}
	}

	app := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "App.vue")))
	for _, want := range []string{
		"workspace-customer",
		"padding: 6px 70px 6px 8px",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("App.vue missing PR-338 marker %q", want)
		}
	}
}

func TestDev338CustomerSelectorClearDropdownSpacingDocs(t *testing.T) {
	for _, rel := range []string{
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_WORKSPACE_MODE.md"),
		filepath.Join("docs", "acceptance", "2026-05-23-customer-selector-clear-dropdown-spacing.md"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"PR-338-CUSTOMER-SELECTOR-CLEAR-DROPDOWN-SPACING",
			"清除选择",
			"展开",
		} {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-338 documentation marker %q", rel, want)
			}
		}
	}
}
