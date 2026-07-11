package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev480To484ManufacturingPhase3Contracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-480-MANUFACTURING-PHASE3-SCHEDULE-CAPACITY",
			"PR-481-MANUFACTURING-PHASE3-SCHEDULING-WORKBENCH",
			"PR-482-MANUFACTURING-PHASE3-MRP-SUGGESTIONS",
			"PR-483-MANUFACTURING-PHASE3-INDUSTRY-CALCULATORS",
			"PR-484-MANUFACTURING-PHASE3-TRACEABILITY-ANALYTICS",
			"REV-484-MANUFACTURING-PHASE3-TRACEABILITY-ANALYTICS",
		},
		filepath.Join("internal", "application", "production", "service.go"): {
			"ScheduleBoardQuery",
			"CapacityCalendarCommand",
			"MRPSuggestionQuery",
			"ProductionTraceAnalyticsQuery",
		},
		filepath.Join("internal", "interfaces", "http", "production", "production_schedule_api.go"): {
			"/api/production-schedule",
			"/api/production-schedule/assign",
			"/api/production-capacity-calendar",
			"/api/mrp/suggestions",
			"/api/production-trace/analytics",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductionScheduleView.vue"): {
			"生产排程工作台",
			"甘特",
			"工位负载",
			"MRP",
			"采购建议",
			"调拨建议",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "IndustryFieldTemplatesView.vue"): {
			"行业字段模板",
			"字段定义",
			"新增字段",
			"保存模板",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductionCostsView.vue"): {
			"/api/production-trace/analytics",
			"追溯链路",
			"成本差异",
			"异常损耗",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-480-MANUFACTURING-PHASE3-SCHEDULE-CAPACITY",
			"PR-482-MANUFACTURING-PHASE3-MRP-SUGGESTIONS",
			"PR-483-MANUFACTURING-PHASE3-INDUSTRY-CALCULATORS",
			"PR-484-MANUFACTURING-PHASE3-TRACEABILITY-ANALYTICS",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-480-MANUFACTURING-PHASE3-SCHEDULE-CAPACITY",
			"生产排程工作台",
			"MRP",
			"异常损耗",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-480-MANUFACTURING-PHASE3-SCHEDULE-CAPACITY",
			"生产排程工作台",
			"MRP 建议",
			"生产追溯分析",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-483-MANUFACTURING-PHASE3-INDUSTRY-CALCULATORS",
			"PR-527-REMOVE-INDUSTRY-CALCULATION-PREVIEW",
		},
		filepath.Join("docs", "OP_MANUAL_STOCK.md"): {
			"PR-482-MANUFACTURING-PHASE3-MRP-SUGGESTIONS",
			"调拨建议",
		},
		filepath.Join("docs", "acceptance", "2026-06-12-manufacturing-phase3-production-coordination.md"): {
			"PR-480-MANUFACTURING-PHASE3-SCHEDULE-CAPACITY",
			"PR-481-MANUFACTURING-PHASE3-SCHEDULING-WORKBENCH",
			"PR-482-MANUFACTURING-PHASE3-MRP-SUGGESTIONS",
			"PR-483-MANUFACTURING-PHASE3-INDUSTRY-CALCULATORS",
			"PR-484-MANUFACTURING-PHASE3-TRACEABILITY-ANALYTICS",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing phase3 marker %q", rel, want)
			}
		}
	}
}
