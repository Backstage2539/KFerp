package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerDirectShipReimportStatusCorrectionEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md")))
	requirements := string(readOrderAppFileForTest(t, filepath.Join("..", "REQUIREMENTS.md")))
	acceptanceTests := string(readOrderAppFileForTest(t, filepath.Join("..", "ACCEPTANCE_TESTS.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"directShipImportShipStatusID",
		"ship_status_id=COALESCE($9::bigint, ship_status_id)",
		"customerFulfillmentStatusID(ctx, tx, schema, \"ship_statuses\", \"已发货\")",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("customer fulfillment repository missing direct-ship status correction marker %q", want)
		}
	}
	for _, want := range []string{
		"TestApplyDirectShipImportReimportCorrectedStatusUpdatesERPShipStatus",
		"ERP direct ship status after corrected reimport",
		"YGS-STATUS-001",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer fulfillment repository test missing direct-ship status correction marker %q", want)
		}
	}
	for _, want := range []string{
		"同一外部代发订单重传并更正发货状态",
		"未知状态不会覆盖已有 ERP 状态",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("customer fulfillment manual missing direct-ship status correction marker %q", want)
		}
	}
	for _, want := range []string{
		"发货状态",
		"ERP 代发订单发货状态",
	} {
		if !strings.Contains(requirements, want) {
			t.Fatalf("REQUIREMENTS.md missing direct-ship status correction marker %q", want)
		}
		if !strings.Contains(acceptanceTests, want) {
			t.Fatalf("ACCEPTANCE_TESTS.md missing direct-ship status correction marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-260-CUSTOMER-DIRECT-SHIP-REIMPORT-STATUS-CORRECTION",
		"DEV-260-CUSTOMER-DIRECT-SHIP-REIMPORT-STATUS-CORRECTION",
		"TestApplyDirectShipImportReimportCorrectedStatusUpdatesERPShipStatus",
		"CUSTOMER_DIRECT_SHIP_STATUS_REIMPORT_UI_CLICK_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing direct-ship status correction marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing direct-ship status correction marker %q", want)
		}
	}
}
