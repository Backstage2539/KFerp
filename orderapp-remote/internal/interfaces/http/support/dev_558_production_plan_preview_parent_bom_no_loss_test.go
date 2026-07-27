package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev558ProductionPlanPreviewParentBomNoLossContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-558-PRODUCTION-PLAN-PREVIEW-PARENT-BOM-NO-LOSS",
			"DEV-558-EXACT-UNPLANNED-DEMAND-SCOPE",
			"DEV-558-PREVIEW-BOM-ROUTE-DECOUPLING",
			"DEV-558-FORMAL-CREATE-ROUTE-GUARD",
			"DEV-558-DOCS-DELIVERY",
		},
		filepath.Join("internal", "application", "production", "service.go"): {
			"IncludedDemandKeys",
		},
		filepath.Join("internal", "infrastructure", "postgres", "production", "material_plan.go"): {
			"splitUnproducedNeedsByProductionPlan",
			"query.IncludedDemandKeys",
			`row.DemandStatus != "unplanned"`,
			"resolveProductionBomForDemandProductPreviewTx",
			"isProductionBomNotConfiguredError",
		},
		filepath.Join("internal", "infrastructure", "postgres", "production", "plan_queries.go"): {
			"includedMaterialPlanDemandKeys",
			"IncludedDemandKeys:    includedMaterialPlanDemandKeys",
			"resolveProductionBomForDemandProductPreviewTx",
		},
		filepath.Join("internal", "infrastructure", "postgres", "production", "work_order.go"): {
			"resolveProductionBomForDemandProductPreviewTx",
			"resolveProductionBomForDemandProductWithRouteRequirementTx",
			"productionBomMissingRouteConfigurationError",
			"productionBomErrorReasonNotConfigured",
		},
		filepath.Join("internal", "interfaces", "http", "production", "produce_plan_api_test.go"): {
			"TestProducePlanSummaryAPIIgnoresInProductionSiblingDemandForParentBomMaterialPlan",
			"TestProducePlanSummaryAPIPreviewsNoLossParentBomMaterialsWhenRouteMissing",
			"TestProducePlanSummaryAPIDoesNotReplaceInvalidFormalBomWithAnotherRecipe",
			"formal plan creation must still reject the missing route",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-558-PRODUCTION-PLAN-PREVIEW-PARENT-BOM-NO-LOSS",
			"14 × 0.454Kg = 6.356Kg",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"K100. 生产计划精确需求与缺路线父 BOM 预览",
			"组件预计消耗合计均为6356g",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-558-PRODUCTION-PLAN-PREVIEW-PARENT-BOM-NO-LOSS",
			"同一规格已进入旧生产计划的订单不会重复加入",
		},
		filepath.Join("docs", "acceptance", "2026-07-27-production-plan-preview-parent-bom-no-loss.md"): {
			"PR-558 Production Plan Preview Parent BOM No Loss",
			"预览仍返回三条真实组件、合计6356g",
		},
	} {
		source := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(source, want) {
				t.Fatalf("%s missing PR-558 marker %q", rel, want)
			}
		}
	}
}
