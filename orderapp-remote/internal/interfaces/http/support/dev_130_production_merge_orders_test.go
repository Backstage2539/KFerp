package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev130ProductionMergeOrderRequirementSeeds(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(content)
	for _, want := range []string{
		"PR-130",
		"DEV-130-01",
		"DEV-130-02",
		"DEV-130-03",
		"UT-130-01",
		"API-130-01",
		"REV-130-01",
		"produce_running_outputs",
		"SO-20260427-0002",
		"SO-20260501-0001",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestDev130ProductionMergeOrderSourceWiring(t *testing.T) {
	checks := map[string][]string{
		filepath.Join("internal", "infrastructure", "postgres", "production", "running_merge.go"): {
			"groupStartNeedsForRuns",
			"ProduceRunOutputRow",
			"product:%d",
		},
		filepath.Join("internal", "infrastructure", "postgres", "production", "schema.go"): {
			"produce_running_outputs",
			"UNIQUE(running_item_id, product_id, bom_spec_id, spec_g)",
			"produce_running_outputs_identity_uq",
		},
		filepath.Join("internal", "infrastructure", "postgres", "production", "running_repository.go"): {
			"finishRunningOutputs",
			"loadRunningOutputsForUpdateTx",
			"restoreRunningOutputAllocationsTx",
		},
		filepath.Join("internal", "application", "production", "service.go"): {
			"RunningOutput",
			"FinishOutputCommand",
		},
		filepath.Join("internal", "interfaces", "http", "production", "production_flow_routes.go"): {
			"ProduceRunningAPIOutput",
			"ProduceRunningFinishOutputAPIRequest",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProduceRunningView.vue"): {
			"多规格",
			"multi-output-grid",
		},
	}
	for file, wants := range checks {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		src := string(content)
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing %q", file, want)
			}
		}
	}
}
