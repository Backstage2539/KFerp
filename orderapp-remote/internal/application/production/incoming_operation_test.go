package production

import "testing"

func TestIncomingOutputUsesWasteAndCompatibleWeightUnits(t *testing.T) {
	out, err := IncomingOutputQuantity(84, "kg", 1, "g", "清理废料")
	if err != nil || out != 83000 {
		t.Fatalf("got %v, %v, want 83000g", out, err)
	}
	for _, waste := range []float64{-1, 85} {
		if _, err := IncomingOutputQuantity(84, "kg", waste, "kg", "原因"); err == nil {
			t.Fatalf("waste %v should fail", waste)
		}
	}
	if _, err := IncomingOutputQuantity(84, "kg", 1, "kg", ""); err == nil {
		t.Fatal("loss without reason should fail")
	}
	if _, err := IncomingOutputQuantity(84, "kg", 1, "袋", "原因"); err == nil {
		t.Fatal("different dimensions should fail")
	}
}

func TestAllocateIncomingQuantitiesConservesGrams(t *testing.T) {
	got, err := AllocateIncomingQuantities(93001, []float64{60, 40})
	if err != nil || got[0] != 55801 || got[1] != 37200 {
		t.Fatalf("got %v, %v", got, err)
	}
	if _, err = AllocateIncomingQuantities(5, []float64{0, 0}); err == nil {
		t.Fatal("zero weights should fail")
	}
}
