package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerPortalProcessingFulfillmentRequirements(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatalf("read req_store.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"PR-CUSTOMER-PORTAL-PROCESSING-FULFILLMENT",
		"DEV-CUSTOMER-PORTAL-PROCESSING-FULFILLMENT-01",
		"DEV-CUSTOMER-PORTAL-PROCESSING-FULFILLMENT-02",
		"DEV-CUSTOMER-PORTAL-PROCESSING-FULFILLMENT-03",
		"UT-CUSTOMER-PORTAL-PROCESSING-FULFILLMENT-01",
		"API-CUSTOMER-PORTAL-PROCESSING-FULFILLMENT-01",
		"REV-CUSTOMER-PORTAL-PROCESSING-FULFILLMENT-01",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("customer portal processing fulfillment seed missing %q", want)
		}
	}
}

func TestCustomerPortalProcessingFulfillmentOrderSourceFields(t *testing.T) {
	checks := []struct {
		path string
		want []string
	}{
		{
			path: filepath.Join("internal", "infrastructure", "postgres", "core", "schema.go"),
			want: []string{"receiver_name TEXT NOT NULL DEFAULT ''", "receiver_phone TEXT NOT NULL DEFAULT ''", "receiver_address TEXT NOT NULL DEFAULT ''", "portal_service_code TEXT NOT NULL DEFAULT ''", "source_warehouse TEXT NOT NULL DEFAULT ''"},
		},
		{
			path: filepath.Join("internal", "application", "sales", "service.go"),
			want: []string{"ReceiverName", "ReceiverPhone", "ReceiverAddress", "PortalServiceCode", "SourceWarehouse"},
		},
		{
			path: filepath.Join("internal", "infrastructure", "postgres", "sales", "order_queries.go"),
			want: []string{"NULLIF(o.receiver_name,''), NULLIF(c.contact,''), c.name", "COALESCE(o.portal_service_code,'')", "COALESCE(o.source_warehouse,'')"},
		},
	}
	for _, check := range checks {
		body, err := os.ReadFile(check.path)
		if err != nil {
			t.Fatalf("read %s: %v", check.path, err)
		}
		text := string(body)
		for _, want := range check.want {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing %q", check.path, want)
			}
		}
	}
}

func TestCustomerPortalProcessingFulfillmentProductionDemandSource(t *testing.T) {
	checks := []struct {
		path string
		want []string
	}{
		{
			path: filepath.Join("internal", "infrastructure", "postgres", "customerportal", "schema.go"),
			want: []string{"customer_processing_production_demands", "target_warehouse TEXT NOT NULL DEFAULT ''", "linked_running_item_id BIGINT NOT NULL DEFAULT 0"},
		},
		{
			path: filepath.Join("internal", "infrastructure", "postgres", "customerportal", "business_repository.go"),
			want: []string{"customer_processing_production_demands", "processingWarehouseForCustomerTx", "target_warehouse", "customer warehouse binding required"},
		},
		{
			path: filepath.Join("internal", "infrastructure", "postgres", "production", "unprod_summary.go"),
			want: []string{"customer_processing_production_demands", "request_no", "status='planned'"},
		},
		{
			path: filepath.Join("internal", "infrastructure", "postgres", "production", "repository.go"),
			want: []string{"markProcessingDemandsRunningTx", "linked_running_item_id", "linked_work_order_id"},
		},
		{
			path: filepath.Join("internal", "infrastructure", "postgres", "production", "running_repository.go"),
			want: []string{"finishWarehouseForRunningItemTx", "target_warehouse", "markProcessingDemandsDoneTx", "markProcessingDemandsPlannedTx"},
		},
	}
	for _, check := range checks {
		body, err := os.ReadFile(check.path)
		if err != nil {
			t.Fatalf("read %s: %v", check.path, err)
		}
		text := string(body)
		for _, want := range check.want {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing %q", check.path, want)
			}
		}
	}
}

func TestCustomerPortalProcessingFulfillmentMiniOrderSource(t *testing.T) {
	checks := []struct {
		path string
		want []string
	}{
		{
			path: filepath.Join("internal", "infrastructure", "postgres", "core", "schema.go"),
			want: []string{"sender_id BIGINT NOT NULL DEFAULT 0"},
		},
		{
			path: filepath.Join("internal", "application", "customerportal", "service.go"),
			want: []string{"CreateFulfillmentOrderCommand", "RecipientName", "PortalServiceCode", "ProcessingShipment"},
		},
		{
			path: filepath.Join("internal", "interfaces", "http", "customerportal", "mini_api.go"),
			want: []string{"/api/mini/fulfillment-orders", "fulfillmentOrderRequest", "ServiceCode"},
		},
		{
			path: filepath.Join("internal", "infrastructure", "postgres", "customerportal", "business_repository.go"),
			want: []string{"CreateFulfillmentOrder", "portal_service_code", "source_warehouse", "receiver_name", "sender_id"},
		},
		{
			path: filepath.Join("internal", "infrastructure", "postgres", "sales", "order_queries.go"),
			want: []string{"COALESCE(NULLIF(ship_sender.sender_id,0), NULLIF(o.sender_id,0), 0)"},
		},
		{
			path: filepath.Join("internal", "infrastructure", "postgres", "sales", "order_shipping_export_queries.go"),
			want: []string{"COALESCE(o.sender_id,0) AS sender_id"},
		},
	}
	for _, check := range checks {
		body, err := os.ReadFile(check.path)
		if err != nil {
			t.Fatalf("read %s: %v", check.path, err)
		}
		text := string(body)
		for _, want := range check.want {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing %q", check.path, want)
			}
		}
	}
}

func TestCustomerPortalProcessingFulfillmentStockDeductsSourceWarehouse(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("internal", "infrastructure", "postgres", "sales", "order_stock_deductions.go"))
	if err != nil {
		t.Fatalf("read order_stock_deductions.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"orderSourceWarehouseTx",
		"deductOrderSourceWarehouseItemsTx",
		"source_warehouse",
		"recordOrderStockDeductionTx(ctx, tx, orderID, alloc, productName, warehouse",
		"VALUES('finished_product',$1,$2,$3,$4,$5",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("order_stock_deductions.go missing %q", want)
		}
	}
}
