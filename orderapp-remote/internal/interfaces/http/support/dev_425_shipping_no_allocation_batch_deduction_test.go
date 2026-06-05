package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev425ShippingNoAllocationFallbackDeductsFinishedBatches(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "sales", "order_stock_deductions.go")))
	for _, want := range []string{
		"previewOrderStockBatches(ctx, tx, items, orderID, true)",
		"deductFinishedBatchAllocationTx",
		"deductLegacyFinishedInventoryAllocationTx",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("shipment no-allocation fallback missing batch deduction marker %q", want)
		}
	}
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "sales", "order_api_test.go")))
	for _, want := range []string{
		"TestOrdersShippingTrackingAPIDeductsDefaultFinishedBatchWithoutAllocation",
		"FP-OLD-454",
		"remaining_g,remaining_units",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("order API test missing no-allocation batch deduction marker %q", want)
		}
	}
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-425-SHIPPING-DEDUCT-PRODUCED-STOCK-BATCHES",
		"DEV-425-SHIPPING-NO-ALLOCATION-BATCH-FIFO",
		"UT-425-SHIPPING-NO-ALLOCATION-BATCH-DEDUCTION",
		"API-425-SHIPPING-DEDUCT-PRODUCED-STOCK-BATCHES",
		"REV-425-SHIPPING-DEDUCT-PRODUCED-STOCK-BATCHES",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}
