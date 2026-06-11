package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionStartEmptySelectionInputGuardEvidenceExists(t *testing.T) {
	serviceTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "production", "service_flow_test.go")))
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "production", "production_flow_api_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_PRODUCTION.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"TestStartRejectsEmptySelectionWithoutOpeningWork",
		"TestStartDefaultsSelectedNeedInputWhenPlanRowHasNoEditableRoastInput",
		"empty production selection should fail",
		"repo.Start should not be called on empty selection",
		"selected production need without explicit input should use default input",
		"repo.Start should receive selected need for default input handling",
	} {
		if !strings.Contains(serviceTest, want) {
			t.Fatalf("production service test missing start guard marker %q", want)
		}
	}
	for _, want := range []string{
		"TestProduceStartAPIRejectsEmptySelectionWithoutOpeningWork",
		"TestProduceStartAPIDefaultsMissingInputFromPlanWithoutBlockingWork",
		"produce_running_items",
		"work_order_material_reservations",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("production API test missing start guard marker %q", want)
		}
	}
	for _, want := range []string{
		"未选择库存不足项目时",
		"系统按生产计划的默认投料数启动",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("production manual missing start guard marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-230-PRODUCTION-START-EMPTY-SELECTION-INPUT-GUARD",
		"DEV-230-PRODUCTION-START-EMPTY-SELECTION-INPUT-GUARD",
		"TestProduceStartAPIRejectsEmptySelectionWithoutOpeningWork",
		"TestProduceStartAPIDefaultsMissingInputFromPlanWithoutBlockingWork",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing production start guard marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-230-PRODUCTION-START-EMPTY-SELECTION-INPUT-GUARD",
		"DEV-230-PRODUCTION-START-EMPTY-SELECTION-INPUT-GUARD",
		"TestProduceStartAPIRejectsEmptySelectionWithoutOpeningWork",
		"TestProduceStartAPIRejectsMissingInputWithoutOpeningWork",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing production start guard marker %q", want)
		}
	}
	for _, want := range []string{
		"PRODUCTION_START_EMPTY_INPUT_UI_CLICK_OK",
		"empty_selection_alert_then_forced_api_and_missing_input_tamper",
		"empty_response=400",
		"missing_input_response=400",
		"db=running_0_work_orders_0_reservations_0_order_status_pending",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing production start UI marker %q", want)
		}
	}
}
