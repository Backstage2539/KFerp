package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerDirectShipReimportOrderHeaderCorrectionEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md")))
	requirements := string(readOrderAppFileForTest(t, filepath.Join("..", "REQUIREMENTS.md")))
	acceptanceTests := string(readOrderAppFileForTest(t, filepath.Join("..", "ACCEPTANCE_TESTS.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"orderDate := parseDateValue",
		"notes=$8",
		"order_date=$7",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("customer fulfillment repository missing direct-ship header correction marker %q", want)
		}
	}
	for _, want := range []string{
		"TestApplyDirectShipImportReimportCorrectedOrderHeaderUpdatesERPOrderSnapshot",
		"ERP direct ship order date after corrected reimport",
		"修正备注",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer fulfillment repository test missing direct-ship header correction marker %q", want)
		}
	}
	for _, want := range []string{
		"同一外部代发订单重传并更正订单日期、收件信息或备注",
		"订单列表和详情显示最新 Excel 值",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("customer fulfillment manual missing direct-ship header correction marker %q", want)
		}
	}
	for _, want := range []string{
		"订单日期、收件信息或备注",
		"ERP 代发订单头",
	} {
		if !strings.Contains(requirements, want) {
			t.Fatalf("REQUIREMENTS.md missing direct-ship header correction marker %q", want)
		}
		if !strings.Contains(acceptanceTests, want) {
			t.Fatalf("ACCEPTANCE_TESTS.md missing direct-ship header correction marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-259-CUSTOMER-DIRECT-SHIP-REIMPORT-ORDER-HEADER-CORRECTION",
		"DEV-259-CUSTOMER-DIRECT-SHIP-REIMPORT-ORDER-HEADER-CORRECTION",
		"TestApplyDirectShipImportReimportCorrectedOrderHeaderUpdatesERPOrderSnapshot",
		"CUSTOMER_DIRECT_SHIP_HEADER_REIMPORT_UI_CLICK_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing direct-ship header correction marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing direct-ship header correction marker %q", want)
		}
	}
}
