package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev556ProductionPlanDraftSplitUXContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-556-PRODUCTION-PLAN-DRAFT-SPLIT-UX",
			"DEV-556-BOM-LOSS-SUMMARY",
			"DEV-556-DRAFT-TO-SPLIT",
			"DEV-556-REMOVE-DUPLICATE-CREATE",
			"DEV-556-DOCS-DELIVERY",
		},
		filepath.Join("internal", "application", "production", "service.go"): {
			`BomMaterialLossRate float64 ` + "`json:\"bom_material_loss_rate\"`",
			"BomSummaryError",
			"`json:\"bom_summary_error,omitempty\"`",
		},
		filepath.Join("internal", "infrastructure", "postgres", "production", "plan_queries.go"): {
			"loadResolvedPlanBomSummaries",
			"resolveProductionBomForDemandProductTx",
			"productionPlanBomMaterialLossRate",
			"isProductionBomConfigurationError",
			"BomMaterialLossRate = bomSummaries[i].MaterialLossRate",
			"BomSummaryError = bomSummaries[i].Error",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "produce-plan.js"): {
			"productionPlanBomSummary",
			"bom_material_loss_rate",
			"bom_summary_error",
			"默认 BOM / 预期损耗",
			"BOM 配置待完善",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue"): {
			"{{ productionPlanBomSummary(row) }}",
			"await openCurrentPlanSplitDrawer()",
			`v-if="currentPlan" class="primary" type="button" @click="submitCurrentProductionPlan"`,
			`@click="cancelProductionPlanDraft(currentPlan, 'current')"`,
			"创建成功后会自动打开拆分产能",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-556-PRODUCTION-PLAN-DRAFT-SPLIT-UX",
			"bom_material_loss_rate",
			"生成草稿成功后必须立即打开",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"K98. 生产计划损耗摘要与草稿拆分衔接",
			"当前计划区底部不再显示“创建生产计划”",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-556-PRODUCTION-PLAN-DRAFT-SPLIT-UX",
			"成功后自动打开拆分产能抽屉",
		},
		filepath.Join("docs", "acceptance", "2026-07-27-production-plan-draft-split-ux.md"): {
			"PR-556 Production Plan Draft Split UX",
			"不从 `bom_yield_rate` 推导损耗",
		},
	} {
		source := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(source, want) {
				t.Fatalf("%s missing PR-556 marker %q", rel, want)
			}
		}
	}

	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue")))
	if strings.Contains(view, "预期产出率") {
		t.Fatalf("production plan preview must not show the removed expected-yield label")
	}
	if strings.Contains(view, `@click="createProductionPlan"`) {
		t.Fatalf("current production plan must not keep a duplicate create button")
	}
}
