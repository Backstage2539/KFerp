package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestInventoryMenuClickMatrixEvidenceExists(t *testing.T) {
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))
	for _, want := range []string{
		"INVENTORY_MENU_CLICK_MATRIX_SMOKE_OK",
		"views=4",
		"stockOperations",
		"stockOutboundLogs",
		"purchase",
		"materials",
		"tab_switch",
		"open_delivery_note",
		"save_supplier",
		"create_order",
		"receive_order",
		"material_search",
		"stock_backfill",
		"port_18161_free",
		"port_9240_free",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing inventory menu click matrix marker %q", want)
		}
	}

	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-270-INVENTORY-MENU-CLICK-MATRIX",
		"DEV-270-INVENTORY-MENU-CLICK-MATRIX",
		"INVENTORY_MENU_CLICK_MATRIX_SMOKE_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing inventory menu click matrix marker %q", want)
		}
	}
}

func TestInventoryMenuClickMatrixViewsExposeActions(t *testing.T) {
	checks := map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "views", "StockOperationsView.vue"): {
			"库存作业",
			"WIP领退/转仓",
			"成品转仓",
			"库存调整",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "StockOutboundLogsView.vue"): {
			"/api/stock/outbound-logs",
			"openDeliveryNote",
			"下载出库单",
			"loadPage(1)",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "PurchaseView.vue"): {
			"/api/purchase/suppliers",
			"/api/purchase/orders",
			"/api/purchase/receipts",
			"收货入库",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "MaterialsView.vue"): {
			"/api/materials",
			"openStockBackfill",
			"/api/stock/adjustments",
			"库存补录",
		},
	}

	for rel, wants := range checks {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing inventory click matrix marker %q", rel, want)
			}
		}
	}
}
