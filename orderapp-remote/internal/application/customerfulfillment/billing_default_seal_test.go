package customerfulfillment

import "testing"

func TestBillingSealDefaultOnlyWithoutSavedSettings(t *testing.T) {
	initial := ApplyBillingSealDefault(BillingPDFSettings{SealAssetID: 3}, false)
	if len(initial.Placements) != 1 || initial.Placements[0].PageNumber != 1 || initial.Placements[0].X < 670 || initial.Placements[0].Y < 100 {
		t.Fatalf("missing default placement: %+v", initial)
	}
	saved := ApplyBillingSealDefault(BillingPDFSettings{SealAssetID: 3, Placements: []BillingPDFPlacement{}}, true)
	if len(saved.Placements) != 0 {
		t.Fatalf("saved empty selection changed: %+v", saved)
	}
}
