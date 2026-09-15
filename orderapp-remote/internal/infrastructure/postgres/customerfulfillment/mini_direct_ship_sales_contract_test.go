package customerfulfillment

import (
	"os"
	"strings"
	"testing"
)

func TestMiniDirectShipUsesAssignedPriceTableAndSharedSalesOrder(t *testing.T) {
	repoSource, err := os.ReadFile("mini_direct_ship.go")
	if err != nil {
		t.Fatal(err)
	}
	appSource, err := os.ReadFile("../../../application/customerfulfillment/mini_direct_ship.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"PrepareMiniDirectShipOrder",
		"RecordMiniDirectShipOrder",
		"customer_order_price_table_bindings",
		"usage_code=$2",
		"b.owner_type='customer'",
		"LEFT JOIN %s.order_shipping_trackings tracking",
		"tracking_no=$2",
	} {
		if !strings.Contains(string(repoSource), want) {
			t.Fatalf("repository source missing %q", want)
		}
	}
	prepareStart := strings.Index(string(repoSource), "func (r *Repository) PrepareMiniDirectShipOrder")
	prepareEnd := strings.Index(string(repoSource)[prepareStart:], "func (r *Repository) RecordMiniDirectShipOrder")
	if prepareStart < 0 || prepareEnd < 0 {
		t.Fatal("PrepareMiniDirectShipOrder source not found")
	}
	prepare := string(repoSource)[prepareStart : prepareStart+prepareEnd]
	if existing := strings.Index(prepare, "customer_direct_ship_requests"); existing < 0 || existing > strings.Index(prepare, "resolveMiniDirectShipItems") {
		t.Fatal("idempotent retry must return its historical record before current product or price-table validation")
	}
	for _, want := range []string{
		"CustomerRequestID",
		"SelectedPriceTableIDs",
		"RecordMiniDirectShipOrder",
	} {
		if !strings.Contains(string(appSource), want) {
			t.Fatalf("application source missing %q", want)
		}
	}
}

func TestRecordedDirectShipOrderReservesStockAndProcessingOutputBeforeRequestCommit(t *testing.T) {
	repoSource, err := os.ReadFile("mini_direct_ship.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(repoSource)
	start := strings.Index(source, "func (r *Repository) RecordMiniDirectShipOrder")
	end := strings.Index(source[start:], "func (r *Repository) SubmitMiniDirectShip")
	if start < 0 || end < 0 {
		t.Fatal("RecordMiniDirectShipOrder body not found")
	}
	body := source[start : start+end]
	for _, want := range []string{
		"loadMiniCustomerFinishedStock(ctx, tx, cmd.CustomerID, true)",
		"loadMiniProcessingOutputCandidates(ctx, tx, cmd.CustomerID, true)",
		"planMiniDirectShipFulfillment",
		"customer_processing_output_reservations",
		"order_stock_batch_allocations",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("atomic fulfillment record missing %q", want)
		}
	}
}

func TestMiniDirectShipCancellationAllowsEveryUnshippedReservationState(t *testing.T) {
	for _, status := range []string{"pending", "reserved", "submitted"} {
		if !miniDirectShipCancellationAllowed(status) {
			t.Fatalf("status %q should remain cancellable", status)
		}
	}
	for _, status := range []string{"partially_shipped", "shipped", "delivered", "cancelled"} {
		if miniDirectShipCancellationAllowed(status) {
			t.Fatalf("ERP order status %q must not use legacy direct cancellation", status)
		}
	}
}
