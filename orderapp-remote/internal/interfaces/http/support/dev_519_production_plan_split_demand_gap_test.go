package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev519ProductionPlanSplitDemandGapContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-519-PRODUCTION-PLAN-SPLIT-DEMAND-GAP",
			"DEV-519-SPLIT-PREVIEW-API",
			"DEV-519-SPLIT-PREVIEW-CALCULATION",
			"DEV-519-SPLIT-PREVIEW-UI",
			"API-519-PRODUCTION-PLAN-SPLIT-DEMAND-GAP",
		},
		filepath.Join("internal", "application", "production", "service.go"): {
			"PreviewProductionPlanOperationSplits",
			"ProductionPlanOperationSplitPreview",
			"CoverageSummary",
		},
		filepath.Join("internal", "interfaces", "http", "production", "production_plan_api.go"): {
			"/api/production-plans/:id/operation-splits/preview",
			"PreviewProductionPlanOperationSplits",
		},
		filepath.Join("internal", "infrastructure", "postgres", "production", "production_plan.go"): {
			"PreviewProductionPlanOperationSplits",
			"previewProductionPlanMaterialSummary",
			"productionPlanPreviewStatus",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue"): {
			"产能安排总览",
			"用料需求差距",
			"productionPlanOperationSplitsPreviewEndpoint",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "produce-plan.js"): {
			"operation-splits/preview",
			"operationSplitPreviewStatusLabel",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-519-PRODUCTION-PLAN-SPLIT-DEMAND-GAP",
			"多工序同一计划行时",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-519-PRODUCTION-PLAN-SPLIT-DEMAND-GAP",
			"20kg 实际需求安排 12kg",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"产能安排总览",
			"用料需求差距",
		},
		filepath.Join("docs", "acceptance", "2026-07-05-production-plan-split-demand-gap.md"): {
			"PR-519",
			"不写库",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-519 marker %q", rel, want)
			}
		}
	}
}
