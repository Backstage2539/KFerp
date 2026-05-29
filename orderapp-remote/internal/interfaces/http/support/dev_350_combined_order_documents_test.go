package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev350CombinedOrderDocumentsSeeds(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-350-COMBINED-ORDER-DOCUMENTS",
		"DEV-350-COMBINED-ORDER-DOCUMENTS",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestDev350OrdersViewExposesCombinedDocumentSelection(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrdersView.vue")))
	for _, want := range []string{
		"组合单据",
		"selectedOrderIDs",
		"selectedOrdersShareSameCustomer(selectedOrderIDs.value",
		"openCombinedSalesOrderDrawer",
		"openCombinedDeliveryNoteDrawer",
		`<SalesOrderView :order-ids="activeCombinedSalesOrderIDs"`,
		`<DeliveryNoteView :order-ids="activeCombinedDeliveryNoteIDs"`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("OrdersView.vue missing combined document marker %q", want)
		}
	}
}

func TestDev350CombinedDocumentViewsUseCombinedAPIs(t *testing.T) {
	files := map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "views", "CombinedSalesOrderView.vue"): {
			"/api/orders/combined/sales-order-preview",
			"/api/orders/combined/sales-order-preview.pdf",
			"/api/orders/combined/sales-orders",
			"关联订单",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "CombinedDeliveryNoteView.vue"): {
			"/api/orders/combined/delivery-note-preview",
			"/api/orders/combined/delivery-note-preview.pdf",
			"/api/orders/combined/delivery-notes",
			"关联订单",
		},
	}
	for rel, wants := range files {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing %q", rel, want)
			}
		}
	}
}

func TestDev350CombinedDocumentsManualAndAcceptanceDocs(t *testing.T) {
	for _, rel := range []string{
		filepath.Join("..", "REQUIREMENTS.md"),
		filepath.Join("..", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("..", "docs", "acceptance", "2026-05-24-combined-order-documents.md"),
	} {
		body := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{"PR-350-COMBINED-ORDER-DOCUMENTS", "组合销售单", "组合出库单"} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing %q", rel, want)
			}
		}
	}
}
