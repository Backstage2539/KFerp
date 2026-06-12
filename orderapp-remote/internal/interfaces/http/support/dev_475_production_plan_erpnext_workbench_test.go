package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev475ProductionPlanERPNextWorkbenchContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-475-PRODUCTION-PLAN-ERPNEXT-WORKBENCH",
			"DEV-475-PRODUCTION-PLAN-WORKBENCH-UI",
			"DEV-475-CURRENT-PLAN-SUBMIT",
			"DEV-475-DOCS-ACCEPTANCE",
			"REV-475-PRODUCTION-PLAN-ERPNEXT-WORKBENCH",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue"): {
			"planning-workbench",
			"待生产需求",
			"当前生产计划",
			"loadSelectedPlanPreview",
			"提交当前计划生成工单",
			"productionPlanBatchSubmitEndpoint()",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "produce-plan.js"): {
			"buildCurrentProductionPlanSubmitPayload",
			"return { ids: [id] }",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-475-PRODUCTION-PLAN-ERPNEXT-WORKBENCH",
			"当前生产计划工作台",
			"提交当前计划生成工单",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-475-PRODUCTION-PLAN-ERPNEXT-WORKBENCH",
			"勾选库存不足商品后，右侧当前生产计划自动显示计划预览和物料需求汇总",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-475-PRODUCTION-PLAN-ERPNEXT-WORKBENCH",
			"左侧待生产需求",
			"右侧当前生产计划",
		},
		filepath.Join("docs", "acceptance", "2026-06-12-production-plan-erpnext-workbench.md"): {
			"PR-475-PRODUCTION-PLAN-ERPNEXT-WORKBENCH",
			"当前生产计划工作台",
			"提交当前计划生成工单",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-475 marker %q", rel, want)
			}
		}
	}
}
