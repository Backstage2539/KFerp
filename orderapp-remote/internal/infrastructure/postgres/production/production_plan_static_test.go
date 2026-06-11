package production

import (
	"os"
	"strings"
	"testing"
)

func TestProductionPlanCreateAllowsDefaultInputForSelectedRows(t *testing.T) {
	src, err := os.ReadFile("production_plan.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, forbidden := range []string{
		"input_g required",
		"cmd.InputByKey[key] <= 0",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("formal production plan create must allow selected rows to use default input; found %q", forbidden)
		}
	}
	if !strings.Contains(text, "groupStartNeedsForRuns(needs, cmd.InputByKey") {
		t.Fatal("formal production plan create must delegate default input calculation to groupStartNeedsForRuns")
	}
}

func TestProductionPlanListSupportsStatusAndTimeFilters(t *testing.T) {
	src, err := os.ReadFile("production_plan.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, want := range []string{
		"productionPlanTimeFieldColumn",
		"pp.created_at",
		"pp.submitted_at",
		"pp.completed_at",
		"COALESCE(to_char(pp.completed_at",
		"query.From",
		"query.To",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("production plan list must support filter/query field %q", want)
		}
	}
}
