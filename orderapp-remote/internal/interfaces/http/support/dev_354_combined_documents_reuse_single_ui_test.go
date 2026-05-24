package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev354CombinedDocumentsReuseSingleDocumentDrawers(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrdersView.vue")))
	for _, want := range []string{
		`<SalesOrderView :order-ids="activeCombinedSalesOrderIDs"`,
		`<DeliveryNoteView :order-ids="activeCombinedDeliveryNoteIDs"`,
		"openCombinedSalesOrderDrawer",
		"openCombinedDeliveryNoteDrawer",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("OrdersView.vue missing single drawer reuse marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"CombinedSalesOrderView from",
		"CombinedDeliveryNoteView from",
		"<CombinedSalesOrderView",
		"<CombinedDeliveryNoteView",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("OrdersView.vue should not contain copied combined document UI marker %q", forbidden)
		}
	}
}

func TestDev354SingleDocumentViewsSwitchContentForCombinedOrderIDs(t *testing.T) {
	files := map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderView.vue"): {
			"orderIds",
			"isCombinedSalesOrder",
			"/api/orders/combined/sales-order-preview",
			"/api/orders/combined/sales-order-preview.pdf",
			"/api/orders/combined/sales-orders",
			"销售单设置",
			"销售单备注",
			"PDF版本",
			"图片版本",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "DeliveryNoteView.vue"): {
			"orderIds",
			"isCombinedDeliveryNote",
			"/api/orders/combined/delivery-note-preview",
			"/api/orders/combined/delivery-note-preview.pdf",
			"/api/orders/combined/delivery-notes",
			"公章设置",
			"出库维护",
			"历史版本",
		},
	}
	for rel, wants := range files {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing reused document UI marker %q", rel, want)
			}
		}
	}
}

func TestDev354SeedsManualAndAcceptanceDocs(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-354-COMBINED-DOCUMENT-REUSE-SINGLE-UI",
		"DEV-354-COMBINED-DOCUMENT-REUSE-SINGLE-UI",
		"API-354-COMBINED-DOCUMENT-REUSE-SINGLE-UI",
		"REV-354-COMBINED-DOCUMENT-REUSE-SINGLE-UI",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
	for _, rel := range []string{
		filepath.Join("..", "REQUIREMENTS.md"),
		filepath.Join("..", "ACCEPTANCE_TESTS.md"),
		filepath.Join("..", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("..", "docs", "acceptance", "2026-05-24-combined-document-reuse-single-ui.md"),
	} {
		body := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{"PR-354-COMBINED-DOCUMENT-REUSE-SINGLE-UI", "复用", "组合销售单", "组合出库单"} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing %q", rel, want)
			}
		}
	}
}
