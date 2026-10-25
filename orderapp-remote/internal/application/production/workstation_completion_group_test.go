package production

import (
	"reflect"
	"testing"
)

func TestAllocateOperationGroupOutputInWorkOrderOrder(t *testing.T) {
	planned := []float64{10, 6, 4, 1}
	for _, tc := range []struct {
		planned []float64
		actual  float64
		want    []float64
	}{
		{planned: planned, actual: 21, want: []float64{10, 6, 4, 1}},
		{planned: planned, actual: 6, want: []float64{6, 0, 0, 0}},
		{planned: []float64{4, 6, 4, 1}, actual: 15, want: []float64{4, 6, 4, 1}},
	} {
		got, err := AllocateOperationGroupOutput(tc.planned, tc.actual)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("AllocateOperationGroupOutput(%v, %v) = %v, want %v", tc.planned, tc.actual, got, tc.want)
		}
	}
}

func TestOperationGroupMaxOutputQtyUsesSharedMaterialAvailabilityOnce(t *testing.T) {
	materials := []OperationCompletionGroupMaterial{
		{RequiredQty: 9534, AvailableQty: 11627},
		{RequiredQty: 21, AvailableQty: 21},
	}
	if got := OperationGroupMaxOutputQty(21, "袋", materials); got != 21 {
		t.Fatalf("max output with 21 bags available = %v, want 21", got)
	}
	materials[1].AvailableQty = 15
	if got := OperationGroupMaxOutputQty(21, "袋", materials); got != 15 {
		t.Fatalf("max output with 15 bags available = %v, want 15", got)
	}
	materials[1].AvailableQty = 0
	if got := OperationGroupMaxOutputQty(21, "袋", materials); got != 0 {
		t.Fatalf("max output without packaging bags = %v, want 0", got)
	}
	if got := OperationGroupMaxOutputQty(21, "袋", nil); got != 21 {
		t.Fatalf("max output without material constraints = %v, want 21", got)
	}
}

func TestAllocateOperationGroupMinutesLeavesZeroOutputCardsWithoutTime(t *testing.T) {
	got, err := AllocateOperationGroupMinutes([]float64{5, 0, 0, 0}, 7)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []int{7, 0, 0, 0}) {
		t.Fatalf("time allocation for one active source = %v, want [7 0 0 0]", got)
	}
	got, err = AllocateOperationGroupMinutes([]float64{10, 6, 4, 1}, 21)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []int{10, 6, 4, 1}) {
		t.Fatalf("time allocation by output = %v, want [10 6 4 1]", got)
	}
}
