package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev488ProductionPlanSplitQtyAutoBatchContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-488-PRODUCTION-PLAN-SPLIT-QTY-AUTOBATCH",
			"DEV-488-SPLIT-QTY-INPUT",
			"DEV-488-AUTOBATCH-FREEZE",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "produce-plan.js"): {
			"Math.ceil(plannedQty / batchSizeQty)",
			"planned_qty: plannedCapacitySplitMetrics(row).planned_qty",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue"): {
			"承担产量",
			"自动批次数",
			"capacityDefaultPlannedQty",
			"split.planned_qty",
		},
		filepath.Join("internal", "application", "production", "service.go"): {
			"planned_qty required",
		},
		filepath.Join("internal", "infrastructure", "postgres", "production", "production_plan.go"): {
			"math.Ceil(split.PlannedQty / split.BatchSizeQty)",
			"roundProductionPlanQuantity",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-488-PRODUCTION-PLAN-SPLIT-QTY-AUTOBATCH",
			"承担产量",
			"ceil(承担产量 / 标准批量)",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-488-PRODUCTION-PLAN-SPLIT-QTY-AUTOBATCH",
			"承担 20kg",
			"计划数量仍是 20kg",
		},
		filepath.Join("docs", "acceptance", "2026-06-12-production-plan-split-qty-autobatch.md"): {
			"PR-488 Production Plan Split Quantity Auto Batch",
			"承担产量",
			"自动显示 5 批",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-488 marker %q", rel, want)
			}
		}
	}

	viewSource := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue")))
	if strings.Contains(viewSource, `v-model.number="split.planned_batch_count"`) {
		t.Fatal("production plan split UI must not expose manual planned_batch_count input")
	}
}
