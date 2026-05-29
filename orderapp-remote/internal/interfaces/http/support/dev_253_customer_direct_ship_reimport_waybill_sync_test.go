package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerDirectShipReimportWaybillSyncEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"trackingNosByOrderID",
		"trimDirectShipStaleTrackingsTx",
		"customer_fulfillment_direct_ship_item",
		"refreshCustomerFulfillmentOrderTrackingSummaryTx",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("customer fulfillment repository missing direct ship waybill sync marker %q", want)
		}
	}
	for _, want := range []string{
		"TestApplyDirectShipImportReimportCorrectedWaybillReplacesImportedTrackings",
		"SF-OLD-001",
		"SF-NEW-001",
		"ship tracking summary",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer fulfillment repository test missing direct ship waybill sync marker %q", want)
		}
	}
	for _, want := range []string{
		"客户履约导入来源的旧运单号会被新 Excel 运单号替换",
		"不会保留已更正的旧导入运单",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("customer fulfillment manual missing direct ship waybill sync marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-253-CUSTOMER-DIRECT-SHIP-REIMPORT-WAYBILL-SYNC",
		"DEV-253-CUSTOMER-DIRECT-SHIP-REIMPORT-WAYBILL-SYNC",
		"TestApplyDirectShipImportReimportCorrectedWaybillReplacesImportedTrackings",
		"CUSTOMER_DIRECT_SHIP_WAYBILL_REPLACE_UI_CLICK_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing direct ship waybill sync marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing direct ship waybill sync marker %q", want)
		}
	}
}
