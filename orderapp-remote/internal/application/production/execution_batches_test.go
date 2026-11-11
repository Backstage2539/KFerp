package production

import (
	"reflect"
	"testing"
)

func TestExecutionBatchPlanPreservesTail(t *testing.T) {
	got, err := ExecutionBatchPlan(59.962, 15, 4)
	if err != nil || !reflect.DeepEqual(got, []float64{15, 15, 15, 14.962}) {
		t.Fatalf("got %v, %v", got, err)
	}
	if _, err = ExecutionBatchPlan(59.962, 15, 3); err == nil {
		t.Fatal("must reject insufficient batch count")
	}
}

func TestExecutionBatchCompletionValidation(t *testing.T) {
	zero := 0.0
	if err := ValidateExecutionBatchCompletion(nil, ""); err == nil {
		t.Fatal("missing output must be rejected")
	}
	if err := ValidateExecutionBatchCompletion(&zero, ""); err == nil {
		t.Fatal("zero output must require reason")
	}
	if err := ValidateExecutionBatchCompletion(&zero, "全部损耗"); err != nil {
		t.Fatal(err)
	}
}
