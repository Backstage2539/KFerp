package sales

import (
	"encoding/json"
	"testing"
)

func TestSampleOrderEditDTOUsesTrustedMarker(t *testing.T) {
	for _, sample := range []bool{false, true} {
		got := editDataForAPI(&OrderEditData{IsSampleOrder: sample, Items: []OrderEditItem{{ItemID: 42, BomSpecID: 7, PriceOverride: true, UnitPrice: "0.00", Qty: "2"}}})
		if got["is_sample_order"] != sample {
			t.Fatalf("sample marker lost: %+v", got)
		}
	}
}

func TestSampleOrderEditJSONPreservesIdentityAndZeroForRepositoryValidation(t *testing.T) {
	var req orderSaveAPIRequest
	if err := json.Unmarshal([]byte(`{"edit_id":42,"order_date":"2026-10-15","customer_id":8,"item_id":["101"],"product_id":["10"],"bom_spec_id":["7"],"bom_variant_id":["70"],"tier_id":["manual"],"unit_price":["0"],"qty":["1"]}`), &req); err != nil {
		t.Fatal(err)
	}
	cmd, err := saveOrderCommandFromCreateRequest(req.toCreateRequest(), req.EditID, "employee:1")
	if err != nil || len(cmd.Items) != 1 || cmd.Items[0].ItemID != 101 || cmd.Items[0].ManualPrice == nil || *cmd.Items[0].ManualPrice != 0 {
		t.Fatalf("sample edit parse: %+v %v", cmd, err)
	}
	// Repository authorization still requires a persisted sample and exact line.
	if _, err := saveOrderCommandFromCreateRequest(req.toCreateRequest(), 0, "employee:1"); err == nil {
		t.Fatal("new free order accepted")
	}
}
