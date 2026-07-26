package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev554ProductionSummaryConversionIsolationContracts(t *testing.T) {
	checks := map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-554-PRODUCTION-SUMMARY-CONVERSION-ISOLATION",
			"DEV-554-INACTIVE-PRODUCT-ISOLATION",
			"DEV-554-CONVERSION-BLOCKING-ROW",
			"DEV-554-PRODUCTION-DEMAND-UI",
			"DEV-554-DOCS-DELIVERY",
			"REV-554-PRODUCTION-SUMMARY-CONVERSION-ISOLATION",
		},
		filepath.Join("internal", "infrastructure", "postgres", "production", "unprod_summary.go"): {
			"p.active=true",
			"BlockingReason",
			"productionQuantitySnapshotBlockingReason",
		},
		filepath.Join("internal", "infrastructure", "postgres", "production", "production_plan.go"): {
			"row.BlockingReason",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "produce-plan.js"): {
			"blocking_reason",
			"demand_selectable",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue"): {
			"资料待完善",
			"blocking_reason",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-554-PRODUCTION-SUMMARY-CONVERSION-ISOLATION",
			"不得猜测",
			"其他有效需求",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-554-PRODUCTION-SUMMARY-CONVERSION-ISOLATION",
			"资料待完善",
			"禁止选择",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"资料待完善",
			"不会阻止其他有效需求",
		},
		filepath.Join("docs", "acceptance", "2026-07-26-production-summary-invalid-conversion-isolation.md"): {
			"PR-554-PRODUCTION-SUMMARY-CONVERSION-ISOLATION",
			"CDS-20260526-1186",
			"1件 = 1盒",
		},
	}
	for rel, wants := range checks {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-554 marker %q", rel, want)
			}
		}
	}
}
