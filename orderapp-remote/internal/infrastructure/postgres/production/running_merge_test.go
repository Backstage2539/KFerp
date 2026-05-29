package production

import (
	"reflect"
	"testing"

	productionapp "orderapp/internal/application/production"
)

func TestGroupStartNeedsForRunsMergesSpecsByProductAndKeepsOutputs(t *testing.T) {
	needs := []productionapp.StartNeed{
		{ProductID: 1, ProductName: "Uraga乌拉嘎", SpecG: 454, GapG: 10896, OrderNos: "SO-MERGE-454", OperationTemplateID: 22},
		{ProductID: 1, ProductName: "Uraga乌拉嘎", SpecG: 227, GapG: 454, OrderNos: "SO-MERGE-227", OperationTemplateID: 22},
		{ProductID: 2, ProductName: "曲奇拼配", SpecG: 1000, GapG: 1000, OrderNos: "SO-OTHER"},
	}
	groups := groupStartNeedsForRuns(needs, map[string]int64{
		"1-454":  16000,
		"1-227":  600,
		"2-1000": 2000,
	}, map[int64]float64{1: 0.82, 2: 0.8})

	if len(groups) != 2 {
		t.Fatalf("groups = %d, want 2", len(groups))
	}
	first := groups[0]
	if first.ProductID != 1 || first.ProductName != "Uraga乌拉嘎" {
		t.Fatalf("first group product = %d/%q, want 1/Uraga乌拉嘎", first.ProductID, first.ProductName)
	}
	if first.NeedG != 11350 || first.InputG != 16600 {
		t.Fatalf("first group need/input = %d/%d, want 11350/16600", first.NeedG, first.InputG)
	}
	if first.OrderNos != "SO-MERGE-454,SO-MERGE-227" {
		t.Fatalf("first group order_nos = %q", first.OrderNos)
	}
	if first.OperationTemplateID != 22 {
		t.Fatalf("first group operation template = %d, want 22", first.OperationTemplateID)
	}
	gotSpecs := []int64{first.Outputs[0].SpecG, first.Outputs[1].SpecG}
	if !reflect.DeepEqual(gotSpecs, []int64{454, 227}) {
		t.Fatalf("first output specs = %#v, want 454 and 227", gotSpecs)
	}
	if first.Outputs[0].PlanUnits != 24 || first.Outputs[1].PlanUnits != 2 {
		t.Fatalf("first output plan units = %d/%d, want 24/2", first.Outputs[0].PlanUnits, first.Outputs[1].PlanUnits)
	}

	second := groups[1]
	if second.ProductID != 2 || len(second.Outputs) != 1 || second.SpecG != 1000 {
		t.Fatalf("second group = %+v, want one normal spec run", second)
	}
}
