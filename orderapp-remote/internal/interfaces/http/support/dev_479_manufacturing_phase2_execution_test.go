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
			"openStockDocument(row, 'finish')",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "JobCardsView.vue"): {
			"开始",
			"暂停",
			"继续",
			"完成",
			"保存实际",
			"损耗原因",
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
			"工序卡执行",
			"工单完工闭环",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-479-MANUFACTURING-PHASE2-EXECUTION-COST-CLOSED-LOOP",
			"工单领料到 WIP",
			"第一工序开始、暂停、继续、完成",
			"生产成本页能看到实际成本拆解",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-479-MANUFACTURING-PHASE2-EXECUTION-COST-CLOSED-LOOP",
			"领料到 WIP",
			"工序卡执行",
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
}
