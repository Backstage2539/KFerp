package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerSettlementEmptyPeriodGuardEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"if feeItems == 0",
		"no fees for settlement period",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("customer fulfillment repository missing empty settlement marker %q", want)
		}
	}
	for _, want := range []string{
		"TestCreateSettlementRejectsEmptyPeriodWithoutWritingBatch",
		"no fees for settlement period",
		"customer_settlement_batches",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer fulfillment repository test missing empty settlement marker %q", want)
		}
	}
	for _, want := range []string{
		"没有费用可结算时不会生成 0 元空结算批次",
		"no fees for settlement period",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("customer fulfillment manual missing empty settlement marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-241-CUSTOMER-SETTLEMENT-EMPTY-PERIOD-GUARD",
		"DEV-241-CUSTOMER-SETTLEMENT-EMPTY-PERIOD-GUARD",
		"TestCreateSettlementRejectsEmptyPeriodWithoutWritingBatch",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing empty settlement marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing empty settlement marker %q", want)
		}
	}
	for _, want := range []string{
		"CUSTOMER_SETTLEMENT_EMPTY_UI_CLICK_OK",
		"db=june_batches_0",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing empty settlement UI marker %q", want)
		}
	}
}
