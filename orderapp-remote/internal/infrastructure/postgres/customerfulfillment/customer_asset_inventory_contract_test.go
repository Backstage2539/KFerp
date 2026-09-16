package customerfulfillment

import (
	"os"
	"strings"
	"testing"
)

func TestCustomerAssetInventoryCoversOwnedFinishedGreenPackagingAndSemiFinished(t *testing.T) {
	source, err := os.ReadFile("customer_asset_inventory.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"finished_product",
		"green_bean",
		"packaging",
		"semi_finished",
		"owner_customer_id",
		"material_batch_locations",
		"customer_processing_material_reservations",
		"ListCustomerAssetInventoryLedger",
		"GREATEST(i.target_qty-COALESCE(receipt.actual_inbound_qty,0),0)",
		"COALESCE(is_processing_product,false)=true",
		"COALESCE(customer_id,0)=$2",
	} {
		if !strings.Contains(string(source), want) {
			t.Fatalf("customer asset inventory source missing %q", want)
		}
	}
}

func TestFinishedReceiptQueriesIncludeWorkOrderCompletionCompatibility(t *testing.T) {
	for _, name := range []string{"customer_asset_inventory.go", "mini_direct_ship.go"} {
		source, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(source), "se.source_type='work_order_complete'") {
			t.Fatalf("%s must count final work-order completion receipts", name)
		}
	}
}

func TestCustomerMaterialAssetTypeDoesNotMisclassifyOtherMaterialsAsGreenBeans(t *testing.T) {
	for input, want := range map[string]string{
		"bean": "green_bean", "green_bean": "green_bean", "pack": "packaging", "packaging": "packaging",
		"other": "", "cleaning": "", "": "",
	} {
		if got := customerMaterialAssetType(input, false); got != want {
			t.Fatalf("customerMaterialAssetType(%q)=%q, want %q", input, got, want)
		}
	}
	if got := customerMaterialAssetType("other", true); got != "semi_finished" {
		t.Fatalf("semi-finished material type=%q", got)
	}
}
