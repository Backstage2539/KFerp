package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerConversionJobReimportJobNoCorrectionEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md")))
	requirements := string(readOrderAppFileForTest(t, filepath.Join("..", "REQUIREMENTS.md")))
	acceptanceTests := string(readOrderAppFileForTest(t, filepath.Join("..", "ACCEPTANCE_TESTS.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"updateConversionJobByJobNoTx",
		"WHERE customer_id=$1 AND job_no=$2",
		"DELETE FROM %s.customer_inventory_conversion_jobs",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("customer fulfillment repository missing conversion job correction marker %q", want)
		}
	}
	for _, want := range []string{
		"TestApplyProcessingImportReimportConversionJobNoUpdatesExistingJob",
		"CV-CORRECT-001",
		"corrected conversion job",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer fulfillment repository test missing conversion job correction marker %q", want)
		}
	}
	for _, want := range []string{
		"同一库存转换单号重传并更正转换前产品",
		"不会因为商品名称变化生成第二张转换工单",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("customer fulfillment manual missing conversion job correction marker %q", want)
		}
	}
	for _, want := range []string{
		"同一转换单号",
		"重复转换工单",
	} {
		if !strings.Contains(requirements, want) {
			t.Fatalf("REQUIREMENTS.md missing conversion job correction marker %q", want)
		}
		if !strings.Contains(acceptanceTests, want) {
			t.Fatalf("ACCEPTANCE_TESTS.md missing conversion job correction marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-262-CUSTOMER-CONVERSION-JOB-REIMPORT-JOB-NO-CORRECTION",
		"DEV-262-CUSTOMER-CONVERSION-JOB-REIMPORT-JOB-NO-CORRECTION",
		"TestApplyProcessingImportReimportConversionJobNoUpdatesExistingJob",
		"CUSTOMER_CONVERSION_JOB_REIMPORT_JOB_NO_UI_CLICK_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing conversion job correction marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing conversion job correction marker %q", want)
		}
	}
}
