package orderweight

import (
	"math"
	"testing"
)

func TestMaterialReferenceOnlyCurrentLayerMetricNetUsage(t *testing.T) {
	for _, unit := range []string{"kg", "KG", "公斤"} {
		g := MaterialReference([]Component{{ConsumeUnit: unit, Quantity: .227}, {ConsumeUnit: "个", Quantity: 1}, {ConsumeUnit: "袋", Quantity: 10}, {ConsumeUnit: "lb", Quantity: 9}}, 1, 0)
		if g == nil || *g != 227 {
			t.Fatalf("unit=%s got=%v, want 227g without packaging or nested pieces", unit, g)
		}
	}
	g := MaterialReference([]Component{{ConsumeUnit: "G", Quantity: 100.123456}, {ConsumeUnit: "克", Quantity: 126.876544}}, 1, 0)
	if g == nil || *g != 227 {
		t.Fatalf("sum=%v", g)
	}
}

func TestMaterialReferenceBasisUnknownAndPrecision(t *testing.T) {
	if g := MaterialReference([]Component{{ConsumeUnit: "袋", Quantity: 10}}, 1, 0); g != nil {
		t.Fatal("piece-only material must be unknown")
	}
	if g := MaterialReference([]Component{{Unit: "kg", ConsumeUnit: "ratio_pct", RatioPct: 100}}, 1, 0); g != nil {
		t.Fatal("ratio with no dimensional net basis must be unknown")
	}
	g := MaterialReference([]Component{{Unit: "kg", ConsumeUnit: "ratio_pct", RatioPct: 60}, {Unit: "kg", ConsumeUnit: "ratio_pct", RatioPct: 40}}, 1, 227)
	if g == nil || *g != 227 {
		t.Fatalf("net ratio=%v", g)
	}
	g = MaterialReference([]Component{{Unit: "kg", ConsumeUnit: "fixed_qty", Quantity: .000123}}, .5, 0)
	if g == nil || math.Abs(*g-.0615) > 1e-9 {
		t.Fatalf("fractional grams=%v", g)
	}
}

func TestMaterialReferenceRejectsUnresolvedLegacyPackagingBasisAndTinyZero(t *testing.T) {
	if g := MaterialReference([]Component{{ConsumeUnit: "g_per_bag", Quantity: 10}}, 1, 0, "盒"); g != nil {
		t.Fatal("unknown bag count in box was guessed")
	}
	if g := MaterialReference([]Component{{ConsumeUnit: "g_per_bag", Quantity: 10}}, 1, 0, "袋"); g == nil || *g != 10 {
		t.Fatalf("known bag basis %v", g)
	}
	if g := MaterialReference([]Component{{ConsumeUnit: "g", Quantity: 1e-8}}, 1, 0); g != nil {
		t.Fatal("rounded zero must remain unknown")
	}
}

func TestStableSpecRenameAndVersionChangePreserveFrozenIdentity(t *testing.T) {
	s := Snapshot{ProductID: 1, BomSpecID: 2, BomVariantID: 3, Unit: "袋", Spec: "old"}
	if !sameIdentity(s, Item{ProductID: 1, BomSpecID: 2, BomVariantID: 4, Unit: "袋", Spec: "new"}) {
		t.Fatal("same stable spec lost snapshot on version/name change")
	}
	if sameIdentity(s, Item{ProductID: 1, BomSpecID: 5}) {
		t.Fatal("different spec kept snapshot")
	}
}

func TestMaterialReferencePersistedPercentagesBelowOneRemainPercentages(t *testing.T) {
	for _, ratio := range []float64{.5, 1} {
		g := MaterialReference([]Component{{Unit: "kg", ConsumeUnit: "ratio_pct", RatioPct: ratio}, {Unit: "kg", ConsumeUnit: "ratio_pct", RatioPct: 100 - ratio}}, 1, 1000)
		if g == nil || *g != 1000 {
			t.Fatalf("ratio %v got %v, want 1000g", ratio, g)
		}
	}
}
