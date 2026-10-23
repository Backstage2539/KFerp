package production

import (
	"math"
	"testing"
)

func TestBatchMaterialIssueAllocationPrioritizesDemandThenFreeWIP(t *testing.T) {
	g := BatchMaterialIssueGroup{MaxQuantity: 60, Allocations: []BatchMaterialIssueAllocation{{JobCardID: 1, Available: 15, ExtraAvailable: 35}, {JobCardID: 2, Available: 10}}}
	rows, err := AllocateBatchMaterialIssue(g, 60)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Quantity != 50 || rows[0].Extra != 35 || rows[1].Quantity != 10 {
		t.Fatalf("allocation: %+v", rows)
	}
	rows, err = AllocateBatchMaterialIssue(g, 20)
	if err != nil || rows[0].Quantity != 15 || rows[1].Quantity != 5 {
		t.Fatalf("partial allocation: %+v %v", rows, err)
	}
	if _, err = AllocateBatchMaterialIssue(g, 61); err == nil {
		t.Fatal("must reject unavailable quantity")
	}
}
func TestBatchMaterialIssueUnitAndRequestValidation(t *testing.T) {
	g := BatchMaterialIssueGroup{Unit: "kg", QuantityBasis: "weight"}
	if q, err := BatchMaterialIssueBaseQuantity(g, 7.785); err != nil || q != 7785 {
		t.Fatalf("kg: %d %v", q, err)
	}
	if _, err := BatchMaterialIssueBaseQuantity(BatchMaterialIssueGroup{Unit: "件", QuantityBasis: "count"}, 1.5); err == nil {
		t.Fatal("fractional pieces")
	}
	for _, q := range []float64{0, -1, math.Inf(1), math.NaN()} {
		_, err := normalizeBatchMaterialIssue(BatchMaterialIssueCommand{JobCardIDs: []int64{1}, RequestID: "r", Groups: []BatchMaterialIssueSelection{{Key: "g", Version: "v", Quantity: q}}}, true)
		if err == nil {
			t.Fatal("invalid quantity accepted")
		}
	}
}
