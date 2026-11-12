package production

import (
	"math"
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

func TestExecutionBatchPreviewUsesFrozenPlanAndKeepsTail(t *testing.T) {
	rows, err := ExecutionBatchPreviewPlan(59.962, 15, 4, "kg", "kg", 72, 60000, 48000, true, 0)
	if err != nil || len(rows) != 4 {
		t.Fatalf("preview %v: %v", rows, err)
	}
	if rows[3].PlannedInputQty != 14.962 || rows[0].PlannedOutputQty == nil || *rows[0].PlannedOutputQty != 12 || rows[3].PlannedMinutes == nil || *rows[3].PlannedMinutes != 18 {
		t.Fatalf("wrong frozen preview: %+v", rows)
	}
	missing, err := ExecutionBatchPreviewPlan(15, 15, 1, "kg", "kg", 0, 0, 0, false, 0)
	if err != nil || missing[0].PlannedOutputQty != nil || missing[0].DefaultUnavailableReason == "" {
		t.Fatalf("missing frozen data guessed: %+v %v", missing, err)
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

func TestExecutionBatchPlannedOutputUsesFrozenManufacturingRatio(t *testing.T) {
	for _, tc := range []struct {
		input                 float64
		inputUnit, outputUnit string
		plannedIn, plannedOut int64
		want                  float64
		ok                    bool
	}{
		{1.25, "kg", "kg", 1250, 1000, 1, true},
		{1250, "g", "kg", 1250, 1000, 1, true},
		{0.5, "kg", "kg", 1250, 1000, 0.4, true},
		{1.25, "kg", "kg", 1250, 1250, 1.25, true},
		{1.25, "kg", "kg", 0, 0, 0, false},
		{1.25, "bag", "kg", 1250, 1000, 0, false},
	} {
		got, ok := ExecutionBatchPlannedOutput(tc.input, tc.inputUnit, tc.outputUnit, tc.plannedIn, tc.plannedOut)
		if ok != tc.ok || (ok && math.Abs(got-tc.want) > 1e-7) {
			t.Errorf("%+v got %v %v", tc, got, ok)
		}
	}
}

func TestExecutionBatchPlannedOutputCumulativeRounding(t *testing.T) {
	var prior float64
	var total float64
	for i, want := range []float64{0.333333, 0.333333, 0.333334} {
		got, ok := ExecutionBatchPlannedOutputSlice(prior, 1, "kg", "kg", 3000, 1000)
		if !ok || math.Abs(got-want) > 1e-9 {
			t.Fatalf("batch %d got %.9f want %.9f", i+1, got, want)
		}
		prior++
		total += got
	}
	if math.Abs(total-1) > 1e-9 {
		t.Fatalf("total %.9f", total)
	}
	got, ok := ExecutionBatchPlannedOutputSlice(1, 1, "kg", "kg", 3000, 1000)
	if !ok || math.Abs(0.333333+got-0.666666) > 1e-9 {
		t.Fatalf("short schedule incorrectly assigned %.9f", got)
	}
	if got, ok := ExecutionBatchPlannedOutputSlice(2000, 1000, "g", "kg", 3000, 1000); !ok || math.Abs(got-0.333334) > 1e-9 {
		t.Fatalf("unit conversion lost rounding remainder: %.9f %v", got, ok)
	}
}
