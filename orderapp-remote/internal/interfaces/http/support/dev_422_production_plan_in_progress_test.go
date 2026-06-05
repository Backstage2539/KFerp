package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev422ProductionPlanIncludesInProgressOrders(t *testing.T) {
	unprodSummary := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "production", "unprod_summary.go")))
	planQueries := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "production", "plan_queries.go")))
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "production", "produce_plan_api_test.go")))

	for _, src := range []struct {
		name string
		body string
	}{
		{"unprod_summary.go", unprodSummary},
		{"plan_queries.go", planQueries},
	} {
		if !strings.Contains(src.body, "productionPlanOpenStatusFilter") {
			t.Fatalf("%s must use the shared production plan status filter", src.name)
		}
	}
	for _, want := range []string{
		"productionPlanOpenStatusNames",
		"生产中",
	} {
		if !strings.Contains(unprodSummary+planQueries, want) {
			t.Fatalf("production plan status filter missing %q", want)
		}
	}
	for _, want := range []string{
		"TestProducePlanIncludesInProgressOrdersWithRemainingItems",
		"SO-IN-PROGRESS-REMAINING",
		"drip_bag",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("produce plan API test missing in-progress marker %q", want)
		}
	}
}
