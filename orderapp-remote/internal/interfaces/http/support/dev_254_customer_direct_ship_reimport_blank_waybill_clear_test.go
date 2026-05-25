package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerDirectShipReimportBlankWaybillClearEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"retainClause := \"\"",
		"len(trackingSet) > 0",
		"refreshCustomerFulfillmentOrderTrackingSummaryTx",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("customer fulfillment repository missing blank waybill clear marker %q", want)
		}
	}
	for _, want := range []string{
		"TestApplyDirectShipImportReimportBlankWaybillClearsImportedTrackings",
		"SF-REMOVE-001",
		"want empty",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer fulfillment repository test missing blank waybill clear marker %q", want)
		}
	}
	for _, want := range []string{
		"如果最新 Excel 运单号为空",
		"系统会清空旧导入运单",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("customer fulfillment manual missing blank waybill clear marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-254-CUSTOMER-DIRECT-SHIP-REIMPORT-BLANK-WAYBILL-CLEAR",
		"DEV-254-CUSTOMER-DIRECT-SHIP-REIMPORT-BLANK-WAYBILL-CLEAR",
		"TestApplyDirectShipImportReimportBlankWaybillClearsImportedTrackings",
		"CUSTOMER_DIRECT_SHIP_WAYBILL_CLEAR_UI_CLICK_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing blank waybill clear marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing blank waybill clear marker %q", want)
		}
	}
}
