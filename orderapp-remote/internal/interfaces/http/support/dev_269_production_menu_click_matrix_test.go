package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionMenuClickMatrixEvidenceExists(t *testing.T) {
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))
	for _, want := range []string{
		"PRODUCTION_MENU_CLICK_MATRIX_SMOKE_OK",
		"views=5",
		"workOrders",
		"jobCards",
		"qualityInspections",
		"produceLogs",
		"productionCosts",
		"status_filter",
		"print",
		"select_work_order",
		"save_quality",
		"batch_operator_filter",
		"refresh",
		"port_18160_free",
		"port_9239_free",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing production menu click matrix marker %q", want)
		}
	}

	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-269-PRODUCTION-MENU-CLICK-MATRIX",
		"DEV-269-PRODUCTION-MENU-CLICK-MATRIX",
		"PRODUCTION_MENU_CLICK_MATRIX_SMOKE_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing production menu click matrix marker %q", want)
		}
	}
}

func TestProductionMenuClickMatrixViewsExposeActions(t *testing.T) {
	checks := map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "views", "WorkOrdersView.vue"): {
			"/api/produce/work-orders",
			"status.value",
			"printWorkOrder",
			"window.print",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "JobCardsView.vue"): {
			"/api/produce/job-cards",
			"status.value",
			"查询",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "QualityInspectionsView.vue"): {
			"/api/produce/quality-inspections",
			"openTargetDrawer",
			"selectTarget",
			"保存质检",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductionLogsView.vue"): {
			"/api/produce/logs",
			"filters.batch_id",
			"filters.operator",
			"筛选",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductionCostsView.vue"): {
			"/api/produce/costs",
			"刷新",
		},
	}

	for rel, wants := range checks {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing production click matrix marker %q", rel, want)
			}
		}
	}
}
