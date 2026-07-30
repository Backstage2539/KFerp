package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev555ProductionPlanDraftCancelContracts(t *testing.T) {
	checks := map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-555-PRODUCTION-PLAN-DRAFT-CANCEL",
			"DEV-555-DRAFT-CANCEL-LIFECYCLE",
			"DEV-555-DEMAND-RETURN-AUDIT",
			"DEV-555-PRODUCTION-PLAN-UI",
			"DEV-555-DOCS-DELIVERY",
			"REV-555-PRODUCTION-PLAN-DRAFT-CANCEL",
		},
		filepath.Join("internal", "interfaces", "http", "production", "production_plan_api.go"): {
			`/api/production-plans/:id/cancel`,
			"CancelProductionPlan",
		},
		filepath.Join("internal", "infrastructure", "postgres", "production", "production_plan.go"): {
			"CancelProductionPlan",
			"FOR UPDATE",
			"status='cancelled',cancelled_at=now()",
			`"production_plan"`,
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "produce-plan.js"): {
			"productionPlanCancelEndpoint",
			"/cancel",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue"): {
			"撤销草稿",
			"refreshProductionDemandAfterDraftCancel",
			"cancelProductionPlanDraft",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-555-PRODUCTION-PLAN-DRAFT-CANCEL",
			"软取消",
			"待生产需求",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-555-PRODUCTION-PLAN-DRAFT-CANCEL",
			"重复撤销幂等",
			"操作日志只写一次",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-555-PRODUCTION-PLAN-DRAFT-CANCEL",
			"撤销生产计划草稿",
			"取消生产工单",
		},
		filepath.Join("docs", "acceptance", "2026-07-26-production-plan-draft-cancel.md"): {
			"PR-555",
			"draft -> cancelled",
			"unplanned",
		},
	}
	for rel, wants := range checks {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-555 marker %q", rel, want)
			}
		}
	}
}
