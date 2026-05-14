package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerProcessingWorkOrderInputSetCorrectionEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("..", "OP_MANUAL_CUSTOMER_FULFILLMENT.md")))
	requirements := string(readOrderAppFileForTest(t, filepath.Join("..", "REQUIREMENTS.md")))
	acceptanceTests := string(readOrderAppFileForTest(t, filepath.Join("..", "ACCEPTANCE_TESTS.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"type processingApplyState",
		"upsertProcessingWorkOrderInputTx",
		"trimProcessingWorkOrderStaleInputsTx",
		"refreshProcessingWorkOrderInputTotalTx",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("customer fulfillment repository missing processing work-order input marker %q", want)
		}
	}
	for _, want := range []string{
		"TestApplyProcessingImportReimportWorkOrderInputsReflectLatestRawBeanSet",
		"original work-order inputs",
		"corrected work-order inputs",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer fulfillment repository test missing processing work-order input marker %q", want)
		}
	}
	for _, want := range []string{
		"同一外部代加工生产工单可以有多行不同生豆投料",
		"旧生豆投料会被裁剪",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("customer fulfillment manual missing processing work-order input marker %q", want)
		}
	}
	for _, want := range []string{
		"多行生豆投料",
		"最新投料集合",
	} {
		if !strings.Contains(requirements, want) {
			t.Fatalf("REQUIREMENTS.md missing processing work-order input marker %q", want)
		}
		if !strings.Contains(acceptanceTests, want) {
			t.Fatalf("ACCEPTANCE_TESTS.md missing processing work-order input marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-261-CUSTOMER-PROCESSING-WORK-ORDER-INPUT-SET-CORRECTION",
		"DEV-261-CUSTOMER-PROCESSING-WORK-ORDER-INPUT-SET-CORRECTION",
		"TestApplyProcessingImportReimportWorkOrderInputsReflectLatestRawBeanSet",
		"CUSTOMER_PROCESSING_WORK_ORDER_INPUT_SET_UI_CLICK_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing processing work-order input marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing processing work-order input marker %q", want)
		}
	}
}
