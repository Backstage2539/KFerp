package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerSettlementDuplicateTotalsGuardEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("..", "OP_MANUAL_CUSTOMER_FULFILLMENT.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"WHERE customer_id=$1",
		"AND settlement_batch_id=$2",
		"SET total_amount=$2",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("customer fulfillment repository missing duplicate settlement total marker %q", want)
		}
	}
	for _, want := range []string{
		"TestCreateSettlementDuplicatePeriodKeepsExistingBatchTotals",
		"duplicate settlement result",
		"want same batch",
		"10400",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer fulfillment repository test missing duplicate settlement marker %q", want)
		}
	}
	for _, want := range []string{
		"重复生成同一周期月结不会清零已结费用或结算批次金额",
		"同一周期月结重复点击后金额变成 0",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("customer fulfillment manual missing duplicate settlement marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-240-CUSTOMER-SETTLEMENT-DUPLICATE-PERIOD-TOTAL-GUARD",
		"DEV-240-CUSTOMER-SETTLEMENT-DUPLICATE-PERIOD-TOTAL-GUARD",
		"TestCreateSettlementDuplicatePeriodKeepsExistingBatchTotals",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing duplicate settlement marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing duplicate settlement marker %q", want)
		}
	}
	for _, want := range []string{
		"CUSTOMER_SETTLEMENT_DUPLICATE_UI_CLICK_OK",
		"total_amount_cents=10400",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing duplicate settlement UI marker %q", want)
		}
	}
}
