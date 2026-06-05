package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev424ShippingDeductsDefaultFinishedInventoryWithoutAllocation(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "sales", "order_stock_deductions.go")))
	if strings.Contains(src, `len(allocations) == 0 && warehouse != "finished_goods"`) {
		t.Fatal("shipment stock deduction must not skip default finished_goods orders without allocations")
	}
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "sales", "order_api_test.go")))
	for _, want := range []string{
		"TestOrdersShippingTrackingAPIDeductsDefaultFinishedInventoryWithoutAllocation",
		`source_doc_type='sales_order_shipment'`,
		`source_doc_id=33`,
		`qty_change_g=-908`,
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("order API test missing produced shipment deduction marker %q", want)
		}
	}
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-424-SHIPPING-DEDUCT-PRODUCED-FINISHED-STOCK",
		"DEV-424-SHIPPING-NO-ALLOCATION-FALLBACK",
		"UT-424-SHIPPING-DEFAULT-FINISHED-STOCK",
		"API-424-SHIPPING-DEDUCT-PRODUCED-FINISHED-STOCK",
		"REV-424-SHIPPING-DEDUCT-PRODUCED-FINISHED-STOCK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}
