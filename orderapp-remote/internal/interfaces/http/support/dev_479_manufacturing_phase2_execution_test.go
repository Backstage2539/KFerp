package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev479ManufacturingPhase2ExecutionContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-479-MANUFACTURING-PHASE2-EXECUTION-COST-CLOSED-LOOP",
			"DEV-479-STOCK-ENTRY-DOCUMENTS",
			"DEV-479-JOB-CARD-EXECUTION",
			"DEV-479-WORK-ORDER-COMPLETION",
			"DEV-479-PRODUCTION-EXECUTION-UI",
			"DEV-479-COST-TRACEABILITY",
			"DEV-479-DOCS-ACCEPTANCE",
			"REV-479-MANUFACTURING-PHASE2-EXECUTION-COST-CLOSED-LOOP",
		},
		filepath.Join("internal", "application", "production", "service.go"): {
			"StockEntryCommand",
			"JobCardActionCommand",
			"WorkOrderCompleteCommand",
			"CreateStockEntry",
			"StartJobCard",
			"PauseJobCard",
			"ResumeJobCard",
			"CompleteJobCard",
			"CompleteWorkOrder",
		},
		filepath.Join("internal", "interfaces", "http", "production", "work_order_api.go"): {
			"/api/job-cards/:id/start",
			"/api/job-cards/:id/pause",
			"/api/job-cards/:id/resume",
			"/api/job-cards/:id/complete",
			"/api/work-orders/:id/complete",
		},
		filepath.Join("internal", "interfaces", "http", "production", "stock_entry_api.go"): {
			"/api/stock-entries",
			"/api/stock-entries/:id",
			"material_issue_to_wip",
			"finished_receipt",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "WorkOrdersView.vue"): {
			"已领料",
			"已消耗",
			"可退料",
			"工序进度",
			"成本汇总",
			"执行枢纽",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "JobCardsView.vue"): {
			"工序要求",
			"实际分钟",
			"实际损耗",
			"损耗原因",
			"进入工位",
			"执行枢纽",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "WorkstationView.vue"): {
			"workstationVisibleActions",
			"开始",
			"暂停",
			"继续",
			"完成本工序",
			"actual_minutes",
			"actual_input_qty",
			"actual_output_qty",
			"leftover_qty",
			"loss_reason",
			"exception_reason",
		},
		filepath.Join("frontend-vue-shell", "src", "components", "ProductionExecutionHubDrawer.vue"): {
			"action_type",
			"command",
			"apiSend",
			"actionBusyKey",
			"updated",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "StockEntriesView.vue"): {
			"库存单据",
			"material_transfer_for_manufacture",
			"material_consumption_for_manufacture",
			"material_return_from_manufacture",
			"material_issue",
			"manufacture",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-479-MANUFACTURING-PHASE2-EXECUTION-COST-CLOSED-LOOP",
			"Stock Entry",
			"工位执行",
			"工单完工闭环",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-479-MANUFACTURING-PHASE2-EXECUTION-COST-CLOSED-LOOP",
			"工单领料到 WIP",
			"在工位视图依次执行第一工序开始、暂停、继续、完成",
			"生产成本页能看到实际成本拆解",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-479-MANUFACTURING-PHASE2-EXECUTION-COST-CLOSED-LOOP",
			"领料到 WIP",
			"工位执行",
			"完工入库",
		},
		filepath.Join("docs", "OP_MANUAL_STOCK.md"): {
			"PR-479-MANUFACTURING-PHASE2-EXECUTION-COST-CLOSED-LOOP",
			"Stock Entry单据",
			"WIP退料",
		},
		filepath.Join("docs", "acceptance", "2026-06-12-manufacturing-phase2-execution-cost-closed-loop.md"): {
			"PR-479-MANUFACTURING-PHASE2-EXECUTION-COST-CLOSED-LOOP",
			"Stock Entry",
			"工序卡",
			"成本拆解",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-479 marker %q", rel, want)
			}
		}
	}

	workOrders := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "WorkOrdersView.vue")))
	workOrderTemplate := workOrders
	if idx := strings.Index(workOrderTemplate, "<script setup>"); idx >= 0 {
		workOrderTemplate = workOrderTemplate[:idx]
	}
	for _, forbidden := range []string{"startWorkOrder(row)", "openStockDocument(row, 'finish')"} {
		if strings.Contains(workOrderTemplate, forbidden) {
			t.Fatalf("WorkOrdersView list must omit direct lifecycle action %q", forbidden)
		}
	}

	jobCards := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "JobCardsView.vue")))
	jobCardTemplate := jobCards
	if idx := strings.Index(jobCardTemplate, "<script setup>"); idx >= 0 {
		jobCardTemplate = jobCardTemplate[:idx]
	}
	for _, forbidden := range []string{"<input", "保存实际"} {
		if strings.Contains(jobCardTemplate, forbidden) {
			t.Fatalf("JobCardsView must be read-only and omit %q", forbidden)
		}
	}
	for _, forbidden := range []string{"runJobCardAction", "saveActuals"} {
		if strings.Contains(jobCards, forbidden) {
			t.Fatalf("JobCardsView must delegate execution to workstation and omit %q", forbidden)
		}
	}
}
