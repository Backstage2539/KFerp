package customerportal

import (
	"os"
	"strings"
	"testing"

	customerportalapp "orderapp/internal/application/customerportal"
)

func TestApplyProcessingRequestDerivedFieldsRequiresInboundQuantityForCompletion(t *testing.T) {
	request := customerportalapp.ProcessingRequest{
		Items: []customerportalapp.ProcessingRequestItem{{
			ProductID:        943,
			ProductName:      "代发产品-蜜瓜",
			SpecG:            454,
			Qty:              100,
			ActualInboundQty: 10,
			Status:           "completed",
		}},
	}

	applyProcessingRequestDerivedFields(&request)

	if request.Status != "partially_completed" {
		t.Fatalf("expected partially_completed after 10/100 inbound, got %q", request.Status)
	}
}

func TestProcessingRequestReceiptTraceIncludesWorkOrderCompletionEntries(t *testing.T) {
	source, err := os.ReadFile("processing_requests.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "se.source_type='work_order_complete'") {
		t.Fatal("processing request receipt trace must include final work-order completion entries")
	}
}

func TestProcessingRequestListLoadsItemTraceInOneBatch(t *testing.T) {
	source, err := os.ReadFile("processing_requests.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(source)
	if !strings.Contains(body, "listProcessingRequestItemsForRequests(ctx, customerID, requestIDs)") {
		t.Fatal("processing request list must load all item traces in one batch")
	}
	if !strings.Contains(body, "i.request_id=ANY($2::bigint[])") {
		t.Fatal("processing request item trace query must filter the request page as one batch")
	}
}

func TestProcessingRequestInboundTraceAcceptsProductionRunFinishedBatches(t *testing.T) {
	source, err := os.ReadFile("processing_requests.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(source)
	if strings.Contains(body, "finished_batch.source_doc_type='stock_entry' AND finished_batch.source_doc_id=se.id") {
		t.Fatal("processing request inbound trace must not exclude finished batches created by production runs")
	}
	for _, want := range []string{
		"finished_batch.batch_code=si.batch_code",
		"finished_batch.item_id=si.product_id",
		"finished_batch.owner_customer_id=si.owner_customer_id",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("processing request inbound trace is missing %q", want)
		}
	}
}

func TestApplyProcessingRequestDerivedFieldsCompletesOnlyWhenEveryLineIsSatisfied(t *testing.T) {
	request := customerportalapp.ProcessingRequest{
		Items: []customerportalapp.ProcessingRequestItem{
			{ProductID: 943, ProductName: "代发产品-蜜瓜", SpecG: 454, Qty: 80, ActualInboundQty: 80, Status: "completed"},
			{ProductID: 944, ProductName: "礼盒", SpecG: 0, Qty: 20, ActualInboundQty: 20, Status: "completed"},
		},
	}

	applyProcessingRequestDerivedFields(&request)

	if request.Status != "completed" {
		t.Fatalf("expected completed after every line is satisfied, got %q", request.Status)
	}
}
