package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerSKUReimportExternalKeyCorrectionEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("..", "OP_MANUAL_CUSTOMER_FULFILLMENT.md")))
	requirements := string(readOrderAppFileForTest(t, filepath.Join("..", "REQUIREMENTS.md")))
	acceptanceTests := string(readOrderAppFileForTest(t, filepath.Join("..", "ACCEPTANCE_TESTS.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"appliedCustomerProductIDByExternalKeyTx",
		"UPDATE %s.products",
		`fmt.Sprintf("product:%d", row.ProductID)`,
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("customer fulfillment repository missing SKU correction marker %q", want)
		}
	}
	for _, want := range []string{
		"TestApplyProcessingImportReimportCustomerSKUExternalKeyUpdatesExistingProduct",
		"YGS-HK-227",
		"customer SKU options after corrected reimport",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer fulfillment repository test missing SKU correction marker %q", want)
		}
	}
	for _, want := range []string{
		"同一客户 SKU 重传并更正名称或烘焙度",
		"工作台商品选择器只显示最新名称",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("customer fulfillment manual missing SKU correction marker %q", want)
		}
	}
	for _, want := range []string{
		"同一外部 SKU 编码",
		"工作台选择器",
	} {
		if !strings.Contains(requirements, want) {
			t.Fatalf("REQUIREMENTS.md missing SKU correction marker %q", want)
		}
		if !strings.Contains(acceptanceTests, want) {
			t.Fatalf("ACCEPTANCE_TESTS.md missing SKU correction marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-257-CUSTOMER-SKU-REIMPORT-EXTERNAL-KEY-CORRECTION",
		"DEV-257-CUSTOMER-SKU-REIMPORT-EXTERNAL-KEY-CORRECTION",
		"TestApplyProcessingImportReimportCustomerSKUExternalKeyUpdatesExistingProduct",
		"CUSTOMER_SKU_REIMPORT_EXTERNAL_KEY_UI_CLICK_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing SKU correction marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing SKU correction marker %q", want)
		}
	}
}
