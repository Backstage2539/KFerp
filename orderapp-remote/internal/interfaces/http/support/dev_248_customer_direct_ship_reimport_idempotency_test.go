package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerDirectShipReimportIdempotencyEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"FROM %s.customer_direct_ship_import_order_items",
		"WHERE import_order_id=$1 AND line_no=$2",
		"UPDATE %s.order_items",
		"DELETE FROM %s.order_items",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("customer fulfillment repository missing direct-ship reimport idempotency marker %q", want)
		}
	}
	for _, want := range []string{
		"TestApplyDirectShipImportReimportSameExternalOrderDoesNotDuplicateItems",
		"StoreParsedImport second",
		"customer_direct_ship_import_order_items",
		"order_items",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer fulfillment repository test missing direct-ship reimport idempotency marker %q", want)
		}
	}
	for _, want := range []string{
		"不同 Excel 批次重传",
		"order_items",
		"不会重复翻倍",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("customer fulfillment manual missing direct-ship reimport idempotency marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-248-CUSTOMER-DIRECT-SHIP-REIMPORT-IDEMPOTENCY",
		"DEV-248-CUSTOMER-DIRECT-SHIP-REIMPORT-IDEMPOTENCY",
		"TestApplyDirectShipImportReimportSameExternalOrderDoesNotDuplicateItems",
		"CUSTOMER_DIRECT_SHIP_REIMPORT_IDEMPOTENCY_UI_CLICK_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing direct-ship reimport idempotency marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing direct-ship reimport idempotency marker %q", want)
		}
	}
}
