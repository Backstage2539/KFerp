package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFinanceAdjustmentDraftMonthAPIGuardEvidenceExists(t *testing.T) {
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "finance", "finance_api_test.go")))
	service := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "finance", "service.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_FINANCE.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"status != domain.MonthStatusClosed && status != domain.MonthStatusAdjusted",
		"month must be closed before adjustment",
	} {
		if !strings.Contains(service, want) {
			t.Fatalf("finance service missing draft adjustment guard marker %q", want)
		}
	}
	for _, want := range []string{
		"TestFinanceAdjustmentAPIRejectsDraftMonthWithoutWritingAdjustment",
		"month must be closed before adjustment",
		"finance_adjustments",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("finance API test missing draft adjustment guard marker %q", want)
		}
	}
	for _, want := range []string{
		"未结账月份不能新增结账后调整",
		"month must be closed before adjustment",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("finance manual missing draft adjustment guard marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-235-FINANCE-ADJUSTMENT-DRAFT-MONTH-API-GUARD",
		"DEV-235-FINANCE-ADJUSTMENT-DRAFT-MONTH-API-GUARD",
		"TestFinanceAdjustmentAPIRejectsDraftMonthWithoutWritingAdjustment",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing draft adjustment guard marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing draft adjustment guard marker %q", want)
		}
	}
	for _, want := range []string{
		"FINANCE_ADJUSTMENT_DRAFT_MONTH_UI_CLICK_OK",
		"draft_month_disabled_then_forced_adjustment_click",
		"error=month_must_be_closed_before_adjustment",
		"db=finance_adjustments_0",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing draft adjustment UI marker %q", want)
		}
	}
}
