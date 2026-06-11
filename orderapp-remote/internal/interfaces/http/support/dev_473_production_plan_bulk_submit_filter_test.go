package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev473ProductionPlanBulkSubmitFilterContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-473-PRODUCTION-PLAN-BULK-SUBMIT-FILTER",
			"DEV-473-PRODUCTION-PLAN-FILTER-API",
			"DEV-473-PRODUCTION-PLAN-BULK-SUBMIT",
			"DEV-473-VUE-PLAN-SELECTION-FILTERS",
			"DEV-473-DOCS-ACCEPTANCE",
			"UT-473-PRODUCTION-PLAN-BULK-SUBMIT-FILTER",
			"API-473-PRODUCTION-PLAN-BULK-SUBMIT-FILTER",
			"REV-473-PRODUCTION-PLAN-BULK-SUBMIT-FILTER",
		},
		filepath.Join("internal", "application", "production", "service.go"): {
			"type SubmitProductionPlansCommand",
			"type ProductionPlanSubmitBatchResult",
			"func (s *Service) SubmitProductionPlans",
			"TimeField string",
			"CompletedAt string",
		},
		filepath.Join("internal", "interfaces", "http", "production", "production_plan_api.go"): {
			"/api/production-plans/submit",
			"time_field",
			"from",
			"to",
		},
		filepath.Join("internal", "infrastructure", "postgres", "production", "production_plan.go"): {
			"productionPlanTimeFieldColumn",
			"pp.completed_at",
			"query.From",
			"query.To",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "produce-plan.js"): {
			"productionPlanBatchSubmitEndpoint",
			"productionPlanStatusLabel",
			"productionPlanSelectionState",
			"buildProductionPlanListQuery",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue"): {
			"selectedProductionPlans",
			"productionPlanHeaderCheckbox",
			"提交生成工单",
			"productionPlanBatchSubmitEndpoint",
			"productionPlanStatusLabel",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-473-PRODUCTION-PLAN-BULK-SUBMIT-FILTER",
			"生产计划列表勾选计划",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-473-PRODUCTION-PLAN-BULK-SUBMIT-FILTER",
			"提交生成工单",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-473-PRODUCTION-PLAN-BULK-SUBMIT-FILTER",
			"列表勾选草稿计划",
			"状态和时间过滤",
		},
		filepath.Join("docs", "acceptance", "2026-06-11-production-plan-bulk-submit-filter.md"): {
			"PR-473-PRODUCTION-PLAN-BULK-SUBMIT-FILTER",
			"选择缺口商品 -> 创建生产计划 -> 在生产计划列表勾选计划 -> 提交生成工单",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-473 marker %q", rel, want)
			}
		}
	}
}
