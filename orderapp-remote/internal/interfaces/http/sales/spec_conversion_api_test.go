package sales

import (
	salesapp "orderapp/internal/application/sales"
	"testing"
)

func TestConversionOrderAPIPreservesSelectionAndItemIdentity(t *testing.T) {
	req := orderSaveAPIRequest{OrderDate: "2026-10-16", CustomerID: 3, ProductID: []string{"10"}, BomSpecID: []string{"102"}, BomVariantID: []string{"1002"}, Qty: []string{"2"}, ItemID: []string{"91"}, PriceSelectionMode: []string{"converted_tier"}, SelectedPriceRowKey: []string{"row-1"}, SpecConversion: []string{`{"group_id":7,"revision":2,"source_bom_spec_id":101,"source_bom_variant_id":1001}`}}
	cmd, err := saveOrderCommandFromCreateRequest(req.toCreateRequest(), 8, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if len(cmd.Items) != 1 || cmd.Items[0].ItemID != 91 || cmd.Items[0].SpecConversion == nil || cmd.Items[0].SpecConversion.GroupID != 7 || cmd.Items[0].ManualPrice != nil {
		t.Fatalf("bad conversion mapping %+v", cmd.Items)
	}
}
func TestConversionEditAPIExposesServerSnapshot(t *testing.T) {
	got := salesapp.SpecConversionFromSnapshot(`{"spec_conversion":{"group_id":7,"revision":2,"reprice":true}}`)
	if got == nil || got.GroupID != 7 || got.Reprice {
		t.Fatalf("bad frozen selection %+v", got)
	}
}
