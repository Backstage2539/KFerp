package stock

import "testing"

func TestPlanMaterialTransferValidation(t *testing.T) {
	good := planMaterialTransferState{PlanStatus: "draft", PlanRevision: 2, LinkRevision: 2, SourceExists: true, ItemExists: true, QtyG: 100, LinkQtyG: 100, IdentityMatches: true, SourceSnapshotMatches: true}
	if err := validatePlanMaterialTransferState(good); err != nil {
		t.Fatalf("valid transfer rejected: %v", err)
	}
	changedRevision := good
	changedRevision.PlanRevision++
	if err := validatePlanMaterialTransferState(changedRevision); err != nil {
		t.Fatalf("unrelated plan revision must remain postable: %v", err)
	}
	for name, mutate := range map[string]func(*planMaterialTransferState){
		"cancelled plan":    func(s *planMaterialTransferState) { s.PlanStatus = "cancelled" },
		"changed source":    func(s *planMaterialTransferState) { s.SourceSnapshotMatches = false },
		"missing source":    func(s *planMaterialTransferState) { s.SourceExists = false },
		"missing item":      func(s *planMaterialTransferState) { s.ItemExists = false },
		"quantity modified": func(s *planMaterialTransferState) { s.QtyG++ },
		"wrong identity":    func(s *planMaterialTransferState) { s.IdentityMatches = false },
	} {
		t.Run(name, func(t *testing.T) {
			s := good
			mutate(&s)
			if err := validatePlanMaterialTransferState(s); err == nil {
				t.Fatal("invalid plan transfer accepted")
			}
		})
	}
}

func TestPlanWIPAvailableCacheSeparatesFrozenVariants(t *testing.T) {
	one := planWIPSource{Kind: "product", ItemID: 7, SpecID: 9, VariantID: 11, SpecG: 454}
	other := one
	other.VariantID = 12
	if planWIPAvailableCacheKey(one, "wip") == planWIPAvailableCacheKey(other, "wip") {
		t.Fatal("different frozen variants must not share availability cache")
	}
}

func TestPlanWIPOptionAvailableIncludesPackedAndLooseWeight(t *testing.T) {
	cases := []struct {
		name                     string
		kind                     string
		spec, g, n, wantG, wantN int64
	}{
		{"packed plus loose", "finished_product", 1000, 500, 2, 2500, 2},
		{"no size count only", "finished_product", 0, 0, 3, 0, 3},
		{"material no conversion", "material", 1000, 500, 2, 500, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := planWIPOptionAvailable(tc.kind, tc.spec, tc.g, tc.n)
			if err != nil || got.g != tc.wantG || got.n != tc.wantN {
				t.Fatalf("got %+v %v", got, err)
			}
		})
	}
}

func TestPlanWIPMixedLegacyAndFrozenVariantsAreBlocked(t *testing.T) {
	legacy := planWIPSource{Kind: "product", ItemID: 9, SpecID: 4, VariantID: 0, SpecG: 1000, RequiredUnits: 1}
	frozen := legacy
	frozen.VariantID = 11
	if err := validatePlanWIPNoMixedVariants([]planWIPSource{legacy, frozen}); err == nil {
		t.Fatal("legacy all-variant pool must not overlap frozen pool")
	}
	if err := validatePlanWIPNoMixedVariants([]planWIPSource{legacy}); err != nil {
		t.Fatalf("legacy only must preserve existing behavior: %v", err)
	}
	if err := validatePlanWIPNoMixedVariants([]planWIPSource{frozen}); err != nil {
		t.Fatalf("frozen only must remain valid: %v", err)
	}
}
