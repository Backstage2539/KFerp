package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestOutboundLogsStockRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-145",
		"DEV-145-01",
		"DEV-145-02",
		"UT-145-01",
		"API-145-01",
		"REV-145-01",
		"顺丰发货",
		"出库日志",
		"库存管理",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("outbound log requirement seed missing %q", want)
		}
	}
}

func TestInventoryMenuExposesOutboundLogsAndShippingUsesSFLabel(t *testing.T) {
	menu := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js")))
	for _, want := range []string{
		"stockOutboundLogs",
		"出库日志",
	} {
		if !strings.Contains(menu, want) {
			t.Fatalf("menu missing outbound log marker %q", want)
		}
	}

	app := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "App.vue")))
	for _, want := range []string{
		"StockOutboundLogsView",
		"stockOutboundLogs: StockOutboundLogsView",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("App.vue missing outbound log view marker %q", want)
		}
	}

	orders := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrdersView.vue")))
	for _, want := range []string{
		"顺丰发货",
		"生成顺丰发货 Excel",
	} {
		if !strings.Contains(orders, want) {
			t.Fatalf("OrdersView missing 顺丰发货 marker %q", want)
		}
	}
}

func TestStockOutboundLogsViewCanViewAndDownloadDeliveryNote(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "StockOutboundLogsView.vue")))
	for _, want := range []string{
		"/api/stock/outbound-logs",
		"DeliveryNoteView",
		"查看出库单",
		"下载出库单",
		"/delivery-note-latest.pdf",
		"出库日志",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("StockOutboundLogsView missing %q", want)
		}
	}
}
