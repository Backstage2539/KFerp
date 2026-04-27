package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOrderShippingExcelRequirementSeeds(t *testing.T) {
	b := readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	src := string(b)
	for _, needle := range []string{
		"PR-101",
		"DEV-101-01",
		"DEV-101-02",
		"DEV-101-03",
		"UT-101-01",
		"API-101-01",
		"REV-101-01",
		"订单列表选择生产完成订单后按服务器快递模板生成快递录单 Excel",
		"包裹件数 1、托寄物默认茶叶、重量 0.1",
	} {
		if !strings.Contains(src, needle) {
			t.Fatalf("req_store.go missing %q", needle)
		}
	}
}

func TestOrderEntryVueShowsTierPricesWithoutShippingExcelLink(t *testing.T) {
	b := readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue"))
	src := string(b)
	for _, needle := range []string{
		"tier-prices",
		"tier-price-chip",
		"defaultWholesaleSpec",
		"wholesaleTierPriceRows",
	} {
		if !strings.Contains(src, needle) {
			t.Fatalf("OrderEntryView.vue missing %q", needle)
		}
	}
	for _, needle := range []string{"shippingExcelUrl", "下载快递录单 Excel"} {
		if strings.Contains(src, needle) {
			t.Fatalf("OrderEntryView.vue should not contain %q", needle)
		}
	}
}

func TestOrdersVueGeneratesShippingExcelForProductionCompletedSelection(t *testing.T) {
	b := readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrdersView.vue"))
	src := string(b)
	for _, needle := range []string{
		"selectedOrderIDs",
		"/api/orders/shipping-excel",
		"生产完成",
		"生成快递录单 Excel",
		"shippingExcelUrl",
	} {
		if !strings.Contains(src, needle) {
			t.Fatalf("OrdersView.vue missing %q", needle)
		}
	}
}

func TestOrderShippingExcelServerUsesTemplateAndPersistentExportDir(t *testing.T) {
	server := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "sales", "order_shipping_excel.go")))
	for _, needle := range []string{"/app/data/ship_temp.xlsx", "/app/data/shipping_exports"} {
		if !strings.Contains(server, needle) {
			t.Fatalf("order_shipping_excel.go missing %q", needle)
		}
	}
}

func readOrderAppFileForTest(t *testing.T, rel string) []byte {
	t.Helper()
	root := findAncestorForTest(t, "go.mod")
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func findAncestorForTest(t *testing.T, marker string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return dir
		}
		next := filepath.Dir(dir)
		if next == dir {
			t.Fatalf("could not find ancestor with %s", marker)
		}
		dir = next
	}
}
