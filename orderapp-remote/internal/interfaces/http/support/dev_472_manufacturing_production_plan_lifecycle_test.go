package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev472ManufacturingProductionPlanLifecycleContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-472-MANUFACTURING-PRODUCTION-PLAN-WORKORDER-LIFECYCLE",
			"DEV-472-PRODUCTION-PLAN-SCHEMA-REPOSITORY",
			"DEV-472-PLAN-SUBMIT-WORKORDER-JOBCARDS",
			"DEV-472-WORKORDER-START-LIFECYCLE",
			"DEV-472-LEGACY-PRODUCE-START-COMPAT",
			"DEV-472-VUE-PRODUCTION-PLAN-WORKORDERS",
			"DEV-472-DOCS-ACCEPTANCE",
			"UT-472-MANUFACTURING-PRODUCTION-PLAN-LIFECYCLE",
			"API-472-MANUFACTURING-PRODUCTION-PLAN-LIFECYCLE",
			"REV-472-MANUFACTURING-PRODUCTION-PLAN-WORKORDER-LIFECYCLE",
		},
		filepath.Join("internal", "infrastructure", "postgres", "production", "schema.go"): {
			"CREATE TABLE IF NOT EXISTS %s.production_plans",
			"CREATE TABLE IF NOT EXISTS %s.production_plan_items",
			"work_orders_running_item_started_uq",
			"WHERE running_item_id > 0",
		},
		filepath.Join("internal", "infrastructure", "postgres", "production", "production_plan.go"): {
			"func (r Repository) CreateProductionPlan",
			"func (r Repository) SubmitProductionPlan",
			"func (r Repository) StartWorkOrder",
			"createPendingJobCardsForWorkOrderTx",
			"createLegacyProductionPlanForStartGroupsTx",
			"work order already started",
		},
		filepath.Join("internal", "interfaces", "http", "production", "production_plan_api.go"): {
			"/api/production-plans",
			"/api/production-plans/:id/submit",
		},
		filepath.Join("internal", "interfaces", "http", "production", "work_order_api.go"): {
			"/api/work-orders/:id/start",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue"): {
			"/api/production-plans",
			"productionPlanBatchSubmitEndpoint",
			"创建生产计划",
			"提交生成工单",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "WorkOrdersView.vue"): {
			"workOrderStatusOptions",
			"执行枢纽",
			"编辑拆分",
			"打印",
			"@updated=\"load\"",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "work-orders.js"): {
			"draft",
			"released",
			"running",
			"workOrderStartEndpoint",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-472-MANUFACTURING-PRODUCTION-PLAN-WORKORDER-LIFECYCLE",
			"生产计划 -> 生产工单 -> 工序卡 -> 执行枢纽开始生产",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-472-MANUFACTURING-PRODUCTION-PLAN-WORKORDER-LIFECYCLE",
			"重复命令必须返回错误",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-472-MANUFACTURING-PRODUCTION-PLAN-WORKORDER-LIFECYCLE",
			"计划提交生成生产工单和工序卡",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-472-MANUFACTURING-PRODUCTION-PLAN-WORKORDER-LIFECYCLE",
			"价格表不能决定生产 BOM",
		},
		filepath.Join("docs", "acceptance", "2026-06-11-manufacturing-production-plan-workorder-lifecycle.md"): {
			"PR-472-MANUFACTURING-PRODUCTION-PLAN-WORKORDER-LIFECYCLE",
			"production_plans",
			"/api/work-orders/:id/start",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-472 marker %q", rel, want)
			}
		}
	}

	workOrders := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "WorkOrdersView.vue")))
	template := workOrders
	if idx := strings.Index(template, "<script setup>"); idx >= 0 {
		template = template[:idx]
	}
	for _, forbidden := range []string{"startWorkOrder(row)", "openStockDocument(row, 'finish')"} {
		if strings.Contains(template, forbidden) {
			t.Fatalf("WorkOrdersView list must delegate lifecycle commands to execution hub and omit %q", forbidden)
		}
	}
}
