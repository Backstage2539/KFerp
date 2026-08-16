package productspecmigration

import "testing"

func TestFinalizeReadinessRequiresPublishedSpecsAndZeroLegacyBusinessState(t *testing.T) {
	ready := finalizeReadiness(readinessCounts{ActiveSpecs: 2, PublishedSpecs: 2})
	if !ready.Ready || len(ready.Blockers) != 0 {
		t.Fatalf("ready = %+v", ready)
	}

	blocked := finalizeReadiness(readinessCounts{
		ActiveSpecs:           2,
		PublishedSpecs:        1,
		AmbiguousLegacySpecs:  9,
		LegacyStock:           3,
		Reservations:          4,
		UnfinishedOrders:      5,
		UnfinishedPlans:       6,
		UnfinishedWorkOrders:  7,
		UnfinishedFulfillment: 8,
	})
	if blocked.Ready {
		t.Fatal("readiness unexpectedly passed")
	}
	want := map[string]int64{
		"unpublished_specs":      1,
		"ambiguous_legacy_specs": 9,
		"legacy_stock":           3,
		"legacy_reservations":    4,
		"unfinished_orders":      5,
		"unfinished_plans":       6,
		"unfinished_work_orders": 7,
		"unfinished_fulfillment": 8,
	}
	for _, blocker := range blocked.Blockers {
		if got, ok := want[blocker.Code]; !ok || got != blocker.Count {
			t.Fatalf("unexpected blocker %+v (want=%v)", blocker, want)
		}
		delete(want, blocker.Code)
	}
	if len(want) != 0 {
		t.Fatalf("missing blockers %v", want)
	}
}

func TestFinalizeReadinessRequiresTemplateProvenanceAndActiveMainInput(t *testing.T) {
	got := finalizeReadiness(readinessCounts{
		ActiveSpecs:                   1,
		PublishedSpecs:                1,
		InvalidSpecTemplateProvenance: 1,
		InactiveMainInputMaterial:     1,
	})
	if got.Ready {
		t.Fatal("readiness unexpectedly passed without BOM specification-template authority")
	}
	want := map[string]bool{
		"missing_published_spec_template_provenance": false,
		"inactive_main_input_material":               false,
	}
	for _, blocker := range got.Blockers {
		if _, ok := want[blocker.Code]; ok {
			want[blocker.Code] = true
		}
	}
	for code, found := range want {
		if !found {
			t.Fatalf("readiness blockers=%+v, want %q", got.Blockers, code)
		}
	}
}

func TestFinalizeReadinessRejectsProductWithoutActiveSpecs(t *testing.T) {
	got := finalizeReadiness(readinessCounts{})
	if got.Ready || len(got.Blockers) != 1 || got.Blockers[0].Code != "no_active_specs" {
		t.Fatalf("readiness = %+v", got)
	}
}
