package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionStartStalePlanGuardEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "production", "repository.go")))
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "production", "production_flow_api_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_PRODUCTION.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"lockStartRefsTx",
		"ensureStartRefsNotRunningTx",
		"production already started",
		"FOR UPDATE",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("production repository missing stale start marker %q", want)
		}
	}
	for _, want := range []string{
		"TestProduceStartRepositoryRejectsStaleNeedAlreadyRunning",
		"SO-DUP-START",
		"production already started",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("production API test missing stale start marker %q", want)
		}
	}
	for _, want := range []string{
		"不会重复生成生产中记录、生产工单或 WIP 占用",
		"production already started",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("production manual missing stale start marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-243-PRODUCTION-START-STALE-PLAN-DUPLICATE-GUARD",
		"DEV-243-PRODUCTION-START-STALE-PLAN-DUPLICATE-GUARD",
		"TestProduceStartRepositoryRejectsStaleNeedAlreadyRunning",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing stale start marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing stale start marker %q", want)
		}
	}
	for _, want := range []string{
		"PRODUCTION_START_STALE_UI_CLICK_OK",
		"two_tabs_same_plan_first_start_then_stale_second_click",
		"error=no_startable_production_data",
		"db=running_1_work_orders_1_reservations_1",
		"reserved_g=600",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing stale start UI marker %q", want)
		}
	}
}
