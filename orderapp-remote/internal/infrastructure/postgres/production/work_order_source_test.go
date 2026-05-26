package production

import (
	"os"
	"strings"
	"testing"
)

func TestWorkOrderUsesOperationTemplateStepsForJobCards(t *testing.T) {
	srcBytes, err := os.ReadFile("work_order.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(srcBytes)
	for _, want := range []string{
		"loadOperationTemplateStepsTx",
		"operation_template_steps",
		"operation_template_id",
		"operation_template_step_id",
		"cost_rate",
		"cost_type",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("work order must create job cards from operation template steps; missing %q", want)
		}
	}
}

func TestBatchCostIncludesOperationTemplateStepCost(t *testing.T) {
	srcBytes, err := os.ReadFile("work_order.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(srcBytes)
	for _, want := range []string{
		"operationCost",
		"job_cards",
		"per_kg_input",
		"per_kg_output",
		"fixed",
		"totalCost := materialCost + operationCost",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("batch cost must include operation step cost; missing %q", want)
		}
	}
	if strings.Contains(src, "operationCost := 0.0") {
		t.Fatalf("operation cost must not be hard-coded to zero")
	}
}
