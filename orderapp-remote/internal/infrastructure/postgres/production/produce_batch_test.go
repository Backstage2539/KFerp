package production

import "testing"

func TestAggregateBatchSummary(t *testing.T) {
	items := []ProduceBatchOrderItem{
		{OrderItemID: 1, OrderID: 10, ProductID: 100, ProductName: "A", SpecG: 454, NeedUnits: 2},
		{OrderItemID: 2, OrderID: 11, ProductID: 100, ProductName: "A", SpecG: 454, NeedUnits: 3},
		{OrderItemID: 3, OrderID: 10, ProductID: 100, ProductName: "A", SpecG: 227, NeedUnits: 1},
		{OrderItemID: 4, OrderID: 12, ProductID: 200, ProductName: "B", SpecG: 454, NeedUnits: 4},
	}
	out := aggregateBatchSummary(items)
	if len(out) != 3 {
		t.Fatalf("len(out)=%d want=3", len(out))
	}
	find := func(pid, spec int64) *ProduceBatchSummaryItem {
		for i := range out {
			if out[i].ProductID == pid && out[i].SpecG == spec {
				return &out[i]
			}
		}
		return nil
	}
	a454 := find(100, 454)
	if a454 == nil || a454.NeedUnits != 5 || a454.NeedG != 2270 {
		t.Fatalf("A454 got=%+v", a454)
	}
	a227 := find(100, 227)
	if a227 == nil || a227.NeedUnits != 1 || a227.NeedG != 227 {
		t.Fatalf("A227 got=%+v", a227)
	}
	b454 := find(200, 454)
	if b454 == nil || b454.NeedUnits != 4 || b454.NeedG != 1816 {
		t.Fatalf("B454 got=%+v", b454)
	}
}

func TestCalcRemainingUnits(t *testing.T) {
	remain, err := calcRemainingUnits(10, 4)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if remain != 6 {
		t.Fatalf("remain=%d want=6", remain)
	}
	if _, err := calcRemainingUnits(5, 6); err == nil {
		t.Fatalf("want err when allocated > total")
	}
}

func TestValidateAllocateUnits(t *testing.T) {
	if err := validateAllocateUnits(10, 4, 3); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if err := validateAllocateUnits(10, 4, 7); err == nil {
		t.Fatalf("want exceed remain err")
	}
	if err := validateAllocateUnits(10, 4, 0); err == nil {
		t.Fatalf("want request>0 err")
	}
}

func TestAggregateBatchSummaryFiltersInvalidRows(t *testing.T) {
	items := []ProduceBatchOrderItem{
		{ProductID: 100, ProductName: "A", SpecG: 454, NeedUnits: 1},
		{ProductID: 0, ProductName: "X", SpecG: 454, NeedUnits: 1},
		{ProductID: 101, ProductName: "Y", SpecG: 0, NeedUnits: 1},
		{ProductID: 102, ProductName: "Z", SpecG: 454, NeedUnits: 0},
	}
	out := aggregateBatchSummary(items)
	if len(out) != 1 {
		t.Fatalf("len(out)=%d want=1", len(out))
	}
	if out[0].ProductID != 100 || out[0].NeedUnits != 1 || out[0].NeedG != 454 {
		t.Fatalf("unexpected row: %+v", out[0])
	}
}
