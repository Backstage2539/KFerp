package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerPackagingJobReimportWorkOrderCorrectionEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("..", "OP_MANUAL_CUSTOMER_FULFILLMENT.md")))
	requirements := string(readOrderAppFileForTest(t, filepath.Join("..", "REQUIREMENTS.md")))
	acceptanceTests := string(readOrderAppFileForTest(t, filepath.Join("..", "ACCEPTANCE_TESTS.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"updatePackagingJobByWorkOrderNoTx",
		"WHERE customer_id=$1 AND work_order_no=$2",
		"DELETE FROM %s.customer_processing_packaging_jobs",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("customer fulfillment repository missing packaging job correction marker %q", want)
		}
	}
	for _, want := range []string{
		"TestApplyProcessingImportReimportPackagingJobWorkOrderNoUpdatesExistingJob",
		"PK-CORRECT-001",
		"corrected packaging job",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer fulfillment repository test missing packaging job correction marker %q", want)
		}
	}
	for _, want := range []string{
		"同一包装子工单编号重传并更正产品",
		"不会因为产品或耗材名称变化生成第二张包装子工单",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("customer fulfillment manual missing packaging job correction marker %q", want)
		}
	}
	for _, want := range []string{
		"同一工单编号",
		"重复包装子工单",
	} {
		if !strings.Contains(requirements, want) {
			t.Fatalf("REQUIREMENTS.md missing packaging job correction marker %q", want)
		}
	}
	for _, want := range []string{
		"同一包装子工单编号",
		"重复包装子工单",
	} {
		if !strings.Contains(acceptanceTests, want) {
			t.Fatalf("ACCEPTANCE_TESTS.md missing packaging job correction marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-263-CUSTOMER-PACKAGING-JOB-REIMPORT-WORK-ORDER-CORRECTION",
		"DEV-263-CUSTOMER-PACKAGING-JOB-REIMPORT-WORK-ORDER-CORRECTION",
		"TestApplyProcessingImportReimportPackagingJobWorkOrderNoUpdatesExistingJob",
		"CUSTOMER_PACKAGING_JOB_REIMPORT_WORK_ORDER_UI_CLICK_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing packaging job correction marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing packaging job correction marker %q", want)
		}
	}
}
