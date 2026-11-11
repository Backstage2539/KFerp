package production

import (
	app "orderapp/internal/application/production"
	"testing"
)

func TestFrozenExecutionInputUnit(t *testing.T) {
	ops := []processSnapshotOperation{
		{Seq: 1, ExecutionJSON: `{"version":1,"reporting_type":"batch","output_unit":"kg"}`},
		{Seq: 2, ExecutionJSON: `{"version":1,"reporting_type":"batch","output_unit":"g"}`},
	}
	if got, ok := frozenExecutionInputUnit(ops, 2, "g"); !ok || got != "kg" {
		t.Fatalf("input %q valid=%v; need predecessor kg", got, ok)
	}
	unit, ok := frozenExecutionInputUnit(ops, 2, "g")
	if !ok {
		t.Fatal("missing input unit")
	}
	plan, err := app.ExecutionBatchPlan(15000/app.OperationUnitFactor(unit), 15*app.OperationUnitFactor("kg")/app.OperationUnitFactor(unit), 1)
	if err != nil || len(plan) != 1 || plan[0] != 15 {
		t.Fatalf("15000g downstream input should be 15kg; got %v %v", plan, err)
	}
	if got, ok := frozenExecutionInputUnit(ops, 1, "g"); !ok || got != "kg" {
		t.Fatalf("first input %q valid=%v", got, ok)
	}
	if _, ok := frozenExecutionInputUnit(ops, 3, "g"); ok {
		t.Fatal("missing target route must not infer a unit")
	}
	ops[0].ExecutionJSON = `invalid`
	if _, ok := frozenExecutionInputUnit(ops, 2, "g"); ok {
		t.Fatal("invalid frozen predecessor must not infer a unit")
	}
}
