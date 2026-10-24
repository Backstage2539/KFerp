package production

import "testing"

func TestOperationCompletionActualQuantityAndLocation(t *testing.T) {
	for _, qty := range []float64{18, 28, 0} {
		got, err := PlanOperationCompletion(OperationCompletionInput{PlannedQty: 20, ActualQty: qty, Unit: "袋", Final: true, ReceiptEnabled: true, TargetWarehouse: "finished_goods", CurrentWarehouse: "wip", DifferenceReason: "现场核实"})
		if err != nil || got.OutputQty != qty || !got.CloseOperation || got.Warehouse != "finished_goods" {
			t.Fatalf("qty %v: %+v, %v", qty, got, err)
		}
	}
	got, err := PlanOperationCompletion(OperationCompletionInput{PlannedQty: 11.2, ActualQty: 11.2, Unit: "kg", CurrentWarehouse: "packing"})
	if err != nil || got.Warehouse != "packing" || got.Saleable {
		t.Fatalf("intermediate: %+v %v", got, err)
	}
}

func TestOperationCompletionRejectsFractionalBagsAndUnexplainedDifference(t *testing.T) {
	for _, in := range []OperationCompletionInput{
		{PlannedQty: 2, ActualQty: 1.5, Unit: "袋", DifferenceReason: "核实"},
		{PlannedQty: 20, ActualQty: 28, Unit: "袋"},
		{ActualQty: -1, Unit: "kg"},
	} {
		if _, err := PlanOperationCompletion(in); err == nil {
			t.Fatalf("accepted invalid completion: %+v", in)
		}
	}
}

func TestOperationHandoffAllocatesOnlyAvailableFinishedBatches(t *testing.T) {
	rows := []OperationBatchBalance{{ID: 1, Remaining: 8.6, Unit: "kg", Warehouse: "roast"}, {ID: 2, Remaining: 2.6, Unit: "kg", Warehouse: "packing"}}
	got, err := AllocateOperationHandoff(rows, 10000, "g")
	if err != nil || len(got) != 2 || got[0].Qty != 8.6 || got[1].Qty != 1.4 || got[1].Warehouse != "packing" {
		t.Fatalf("allocation: %+v %v", got, err)
	}
	if _, err := AllocateOperationHandoff(rows, 12, "kg"); err == nil {
		t.Fatal("must reject consuming unavailable output")
	}
	if _, err := AllocateOperationHandoff(rows, 1, "袋"); err == nil {
		t.Fatal("must not guess weight to bags")
	}
}

func TestOperationOutputWeightUsesFrozenNumericConversion(t *testing.T) {
	if got := OperationOutputWeight(28, "袋", 227, 0, 0); got != 6356 {
		t.Fatal(got)
	}
	if got := OperationOutputWeight(28, "袋", 0, 4540, 20); got != 6356 {
		t.Fatal(got)
	}
	if got := OperationOutputWeight(1.25, "kg", 0, 0, 0); got != 1250 {
		t.Fatal(got)
	}
	if got := OperationOutputWeight(28, "袋", 0, 0, 0); got != 0 {
		t.Fatal("missing snapshot must not infer a weight", got)
	}
}
