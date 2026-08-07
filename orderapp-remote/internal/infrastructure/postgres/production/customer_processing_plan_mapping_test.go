package production

import (
	"os"
	"strings"
	"testing"

	productionapp "orderapp/internal/application/production"
)

func TestCustomerProcessingPlanAndWorkOrderKeepRequestItemIdentity(t *testing.T) {
	schemaText, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	planText, err := os.ReadFile("production_plan.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"processing_request_item_id BIGINT", "target_warehouse TEXT", "customer_id BIGINT"} {
		if strings.Count(string(schemaText), want) < 2 {
			t.Fatalf("production plan items and work orders must both persist %q", want)
		}
	}
	for _, want := range []string{
		"UPDATE %s.processing_job_request_items",
		"UPDATE %s.customer_processing_production_demands",
		"UPDATE %s.customer_processing_material_reservations",
		"processing_request_item_id",
	} {
		if !strings.Contains(string(planText), want) {
			t.Fatalf("production plan/work order mapping missing %q", want)
		}
	}
}

func TestCustomerProcessingCompletionWarehouseIsFixed(t *testing.T) {
	wo := productionapp.WorkOrderRow{ProcessingRequestItemID: 101, TargetWarehouse: "CUSTOMER-8"}
	if _, err := completionWarehouseForWorkOrder(wo, "finished_goods"); err == nil {
		t.Fatal("customer processing completion must reject a different warehouse")
	}
	got, err := completionWarehouseForWorkOrder(wo, "")
	if err != nil || got != "CUSTOMER-8" {
		t.Fatalf("completion warehouse=%q err=%v, want CUSTOMER-8", got, err)
	}
}

func TestDerivedWorkOrderStatusIncludesPaused(t *testing.T) {
	if got := deriveWorkOrderStatusFromJobCardCounts("running", 2, 0, 0, 2); got != "paused" {
		t.Fatalf("status=%q, want paused", got)
	}
	if got := deriveWorkOrderStatusFromJobCardCounts("running", 2, 0, 1, 1); got != "running" {
		t.Fatalf("mixed running/paused status=%q, want running", got)
	}
}
