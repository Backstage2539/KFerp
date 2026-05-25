package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerDirectShipReimportSequenceCorrectionEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"WHERE customer_id=$1 AND external_order_no=$2",
		"FOR UPDATE",
		"external_seq=$3",
		"ON CONFLICT (customer_id, external_order_no, external_seq)",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("customer fulfillment repository missing direct ship sequence correction marker %q", want)
		}
	}
	for _, want := range []string{
		"TestApplyDirectShipImportReimportCorrectedSequenceNoUpdatesExistingOrder",
		"corrected direct ship sequence",
		"YGS-SEQ-001",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer fulfillment repository test missing direct ship sequence correction marker %q", want)
		}
	}
	for _, want := range []string{
		"序号",
		"外部订单号",
		"重复订单",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("customer fulfillment manual missing direct ship sequence correction marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-268-CUSTOMER-DIRECT-SHIP-REIMPORT-SEQUENCE-CORRECTION",
		"DEV-268-CUSTOMER-DIRECT-SHIP-REIMPORT-SEQUENCE-CORRECTION",
		"TestApplyDirectShipImportReimportCorrectedSequenceNoUpdatesExistingOrder",
		"CUSTOMER_DIRECT_SHIP_SEQUENCE_REIMPORT_UI_CLICK_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing direct ship sequence correction marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing direct ship sequence correction marker %q", want)
		}
	}
}
