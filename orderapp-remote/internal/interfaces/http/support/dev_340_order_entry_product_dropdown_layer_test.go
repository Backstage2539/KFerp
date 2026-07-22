package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev340OrderEntryProductDropdownLayerSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-340-ORDER-ENTRY-PRODUCT-DROPDOWN-LAYER",
		"DEV-340-ORDER-ENTRY-PRODUCT-DROPDOWN-LAYER",
		"UT-340-ORDER-ENTRY-PRODUCT-DROPDOWN-LAYER",
		"API-340-ORDER-ENTRY-PRODUCT-DROPDOWN-LAYER",
		"REV-340-ORDER-ENTRY-PRODUCT-DROPDOWN-LAYER",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PR-340 requirement seed missing %q", want)
		}
	}
}

func TestDev340OrderEntryProductDropdownLayerWiring(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue")))
	for _, want := range []string{
		`class="product-combobox combobox product-cell"`,
		`:class="{ open: row.product_open }"`,
		".combobox.open { z-index: 30; }",
		".product-cell { z-index: 3; }",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("OrderEntryView.vue missing PR-340 marker %q", want)
		}
	}
}

func TestDev340OrderEntryProductDropdownLayerDocs(t *testing.T) {
	for _, rel := range []string{
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "acceptance", "2026-05-23-order-entry-product-dropdown-layer.md"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"PR-340-ORDER-ENTRY-PRODUCT-DROPDOWN-LAYER",
			"商品下拉",
			"后续商品行",
		} {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-340 documentation marker %q", rel, want)
			}
		}
	}
}
