package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestProducePlanRoastEditAndWIPShortageRequirementSeeds(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-272-PRODUCTION-PLAN-WIP-SHORTAGE-ROAST-EDIT",
		"DEV-272-01",
		"DEV-272-02",
		"UT-272-01",
		"API-272-01",
		"REV-272-01",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestProducePlanCapacitySuggestionCompatibilitySourceGuard(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue")))
	for _, avoid := range []string{
		"machineOptionsForRow",
		"v-model=\"row.machine\"",
		"v-model.number=\"row.batch_count\"",
		"/api/produce/machines",
		"normalizeRoastPlans",
		"syncRoastPlanRow",
	} {
		if strings.Contains(src, avoid) {
			t.Fatalf("ProducePlanView.vue should no longer consume roast capacity suggestion marker %q", avoid)
		}
	}

	machineAPI := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "production", "machine_capacity.go")))
	for _, want := range []string{
		"/api/produce/machines",
		"RoastMachine",
	} {
		if !strings.Contains(machineAPI, want) {
			t.Fatalf("machine_capacity.go must retain compatibility marker %q", want)
		}
	}
}

func TestProducePlanWIPShortageAggregatedMessageSourceGuard(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "production", "material_consumption.go")))
	for _, want := range []string{
		"shortages := make([]string, 0)",
		"WIP stock insufficient:",
		"strings.Join(shortages, \"; \")",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("material_consumption.go missing %q", want)
		}
	}
}
