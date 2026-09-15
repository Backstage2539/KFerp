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
