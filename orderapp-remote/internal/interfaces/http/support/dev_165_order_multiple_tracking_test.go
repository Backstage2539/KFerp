package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev165OrderMultipleTrackingRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-165",
		"DEV-165-01",
		"DEV-165-02",
		"UT-165-01",
		"API-165-01",
		"REV-165-01",
		"多个快递单号",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("dev 165 multiple tracking seed missing %q", want)
		}
	}
}

func TestDev165OrderTrackingPersistenceIsNormalized(t *testing.T) {
	for _, item := range []struct {
		rel   string
		wants []string
	}{
		{
			rel: filepath.Join("internal", "infrastructure", "postgres", "sales", "schema.go"),
			wants: []string{
				"order_shipping_trackings",
				"UNIQUE(order_id, tracking_no)",
			},
		},
		{
			rel: filepath.Join("internal", "infrastructure", "postgres", "sales", "order_tracking_numbers.go"),
			wants: []string{
				"appendOrderTrackingNumbersTx",
				"replaceOrderTrackingNumbersTx",
				"TrackingNumbersSummary",
			},
		},
		{
			rel: filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go"),
			wants: []string{
				"upsertCustomerFulfillmentOrderTrackingsTx",
				"order_shipping_trackings",
			},
		},
	} {
		src := string(readOrderAppFileForTest(t, item.rel))
		for _, want := range item.wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing multiple tracking marker %q", item.rel, want)
			}
		}
	}
}

func TestDev165OrderTrackingUIAndManualDocumentMultipleNumbers(t *testing.T) {
	for _, item := range []struct {
		rel   string
		wants []string
	}{
		{
			rel: filepath.Join("frontend-vue-shell", "src", "views", "OrdersView.vue"),
			wants: []string{
				"快递单号（可多个）",
				"formatTrackingSummary",
			},
		},
		{
			rel: filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue"),
			wants: []string{
				"多个单号",
			},
		},
		{
			rel: filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
			wants: []string{
				"多个快递单号",
				"换行、逗号或分号",
			},
		},
	} {
		src := string(readOrderAppFileForTest(t, item.rel))
		for _, want := range item.wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing multiple tracking marker %q", item.rel, want)
			}
		}
	}
}
