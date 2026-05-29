package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerDirectShipReimportStaleLineTrimEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"trimDirectShipStaleLinesTx",
		"customer_direct_ship_import_order_items",
		"WHERE import_order_id=$1 AND line_no>$2",
		"WHERE order_id=$1 AND line_no>$2",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("customer fulfillment repository missing direct ship stale line trim marker %q", want)
		}
	}
	for _, want := range []string{
		"TestApplyDirectShipImportReimportShorterOrderRemovesStaleItems",
		"YGS-SHORT-001",
		"retained line quantity",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer fulfillment repository test missing direct ship stale line trim marker %q", want)
		}
	}
	for _, want := range []string{
		"少行重传会移除旧的多余行",
		"ERP `order_items`",
		"保留已删除行",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("customer fulfillment manual missing direct ship stale line trim marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-252-CUSTOMER-DIRECT-SHIP-REIMPORT-STALE-LINE-TRIM",
		"DEV-252-CUSTOMER-DIRECT-SHIP-REIMPORT-STALE-LINE-TRIM",
		"TestApplyDirectShipImportReimportShorterOrderRemovesStaleItems",
		"CUSTOMER_DIRECT_SHIP_REIMPORT_STALE_LINE_TRIM_UI_CLICK_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing direct ship stale line trim marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing direct ship stale line trim marker %q", want)
		}
	}
}
