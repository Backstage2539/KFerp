package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev557ProductionPlanBomLossDemandContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-557-PRODUCTION-PLAN-BOM-LOSS-DEMAND",
			"DEV-557-RESOLVED-BOM-LOSS-INPUT",
			"DEV-557-MATERIAL-DEMAND-SINGLE-LOSS",
			"DEV-557-PREVIEW-DOCS",
		},
		filepath.Join("internal", "domain", "production", "yield.go"): {
			"PlannedInputGramsFromMaterialLoss",
			"math.Round",
		},
		filepath.Join("internal", "infrastructure", "postgres", "production", "plan_queries.go"): {
			"theoreticalInputByKey",
			"loadPlanBomItemsFromRows",
			"resolveProductionBomForDemandProductPreviewTx",
			"calcProducePlanMaterialsFromFinalInputs(materialPreviewRows, theoreticalInputByKey",
		},
		filepath.Join("internal", "infrastructure", "postgres", "production", "material_consumption.go"): {
			"InputIncludesMaterialLoss",
			`json:"input_includes_material_loss,omitempty"`,
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue"): {
			"{{ productionPlanBomSummary(row) }}",
			"物料需求汇总（预计消耗）",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-557-PRODUCTION-PLAN-BOM-LOSS-DEMAND",
			"成品需求 ÷ (1 - BOM原料损耗率)",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"K99. 生产计划 BOM 损耗与理论物料需求",
			"7752g",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-557-PRODUCTION-PLAN-BOM-LOSS-DEMAND",
			"不再单列“计划投料”",
		},
		filepath.Join("docs", "acceptance", "2026-07-27-production-plan-bom-loss-demand.md"): {
			"PR-557 Production Plan BOM Loss Demand",
			"V004仍为draft",
		},
	} {
		source := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(source, want) {
				t.Fatalf("%s missing PR-557 marker %q", rel, want)
			}
		}
	}

	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue")))
	if strings.Contains(view, "<th>计划投料(g)</th>") || strings.Contains(view, "<td>{{ row.input_g }}</td>") {
		t.Fatalf("current plan preview must not expose the removed planned-input column")
	}
}
