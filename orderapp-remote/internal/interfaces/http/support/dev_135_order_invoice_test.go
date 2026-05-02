package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOrderInvoiceRequirementSeeds(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(content)
	for _, want := range []string{
		"PR-135",
		"DEV-135-01",
		"DEV-135-02",
		"DEV-135-03",
		"UT-135-01",
		"API-135-01",
		"REV-135-01",
		"订单发票申请",
		"PDF和图片",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestOrderInvoiceVueAndAPISourceWiring(t *testing.T) {
	root := filepath.Join("frontend-vue-shell", "src")
	orders := string(readOrderInvoiceTestFile(t, filepath.Join(root, "views", "OrdersView.vue")))
	invoice := string(readOrderInvoiceTestFile(t, filepath.Join(root, "views", "OrderInvoiceView.vue")))
	app := string(readOrderInvoiceTestFile(t, filepath.Join(root, "App.vue")))
	server := string(readOrderInvoiceTestFile(t, filepath.Join("internal", "interfaces", "http", "sales", "order_invoice.go")))

	for _, want := range []string{"openInvoiceDrawer", "invoice-drawer-mask", "OrderInvoiceView", "invoice_status"} {
		if !strings.Contains(orders, want) {
			t.Fatalf("OrdersView.vue missing %q", want)
		}
	}
	for _, want := range []string{"/api/orders/${orderID.value}/invoice", "/invoice-request", "/invoice-file", "orderInvoiceFileAccept"} {
		if !strings.Contains(invoice, want) {
			t.Fatalf("OrderInvoiceView.vue missing %q", want)
		}
	}
	if !strings.Contains(app, "OrderInvoiceView") {
		t.Fatalf("App.vue missing OrderInvoiceView registration")
	}
	for _, want := range []string{`/api/orders/:id/invoice`, `/api/orders/:id/invoice-request`, `/api/orders/:id/invoice-file`, `classifyOrderInvoiceFile`} {
		if !strings.Contains(server, want) {
			t.Fatalf("order_invoice.go missing %q", want)
		}
	}
}

func readOrderInvoiceTestFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err == nil {
		return b
	}
	b, err = os.ReadFile(filepath.Join("..", "..", "..", "..", path))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}
