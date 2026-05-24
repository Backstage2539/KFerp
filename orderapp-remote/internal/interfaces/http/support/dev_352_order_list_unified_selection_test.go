package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev352OrderListUnifiedSelectionSeeds(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-352-ORDER-LIST-UNIFIED-SELECTION",
		"DEV-352-ORDER-LIST-UNIFIED-SELECTION",
		"UT-352-ORDER-LIST-UNIFIED-SELECTION",
		"API-352-ORDER-LIST-UNIFIED-SELECTION",
		"REV-352-ORDER-LIST-UNIFIED-SELECTION",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestDev352OrdersViewUsesOneSharedSelectionColumn(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrdersView.vue")))
	for _, want := range []string{
		"toggleOrderSelection",
		"togglePageOrderSelection",
		"pageSelectionState.indeterminate",
		"selectedVoidableOrderIDs",
		"selectedOrdersShareSameCustomer(selectedOrderIDs.value",
		"组合销售单",
		"组合出库单",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("OrdersView.vue missing unified selection marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"bulkSelectedOrderIDs",
		"documentSelectedOrderIDs",
		"clearBulkSelection",
		"clearDocumentSelection",
		`<th class="select-col">发货</th>`,
		`<th class="select-col">单据</th>`,
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("OrdersView.vue should not contain old per-feature selection marker %q", forbidden)
		}
	}
}

func TestDev352UnifiedSelectionManualAndAcceptanceDocs(t *testing.T) {
	for _, rel := range []string{
		filepath.Join("..", "REQUIREMENTS.md"),
		filepath.Join("..", "ACCEPTANCE_TESTS.md"),
		filepath.Join("..", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("..", "docs", "acceptance", "2026-05-24-order-list-unified-selection.md"),
	} {
		body := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{"PR-352-ORDER-LIST-UNIFIED-SELECTION", "三态", "横杆", "一个复选框"} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing %q", rel, want)
			}
		}
	}
}
