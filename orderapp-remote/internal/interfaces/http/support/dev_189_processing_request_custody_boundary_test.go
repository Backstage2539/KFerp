package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestProcessingRequestCustodyBoundaryEvidenceExists(t *testing.T) {
	repo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerportal", "business_repository.go")))
	repoTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerportal", "repository_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"ensureProcessingInputInventoryTx",
		"ensureProcessingTargetProductTx",
		"input material unavailable",
		"target product unavailable",
		"customer_inventory_items",
	} {
		if !strings.Contains(repo, want) {
			t.Fatalf("customerportal repository missing processing custody boundary marker %q", want)
		}
	}
	for _, want := range []string{
		"TestCreateProcessingRequestRejectsAnotherCustomerInventory",
		"TestCreateProcessingRequestRejectsInsufficientCustomerInventory",
		"TestCreateProcessingRequestRejectsAnotherCustomerTargetProduct",
		"客户B托管生豆不应使用",
		"another customer processing request created",
	} {
		if !strings.Contains(repoTest, want) {
			t.Fatalf("customerportal repository test missing processing custody boundary marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-189-PROCESSING-REQUEST-CUSTODY-BOUNDARY",
		"代加工申请库存与目标产品范围",
		"TestCreateProcessingRequestRejectsAnotherCustomerInventory",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing processing custody boundary marker %q", want)
		}
	}
}

func TestProcessingRequestCustodyBoundaryRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-189-PROCESSING-REQUEST-CUSTODY-BOUNDARY",
		"DEV-189-PROCESSING-REQUEST-CUSTODY-BOUNDARY",
		"UT-189-PROCESSING-REQUEST-CUSTODY-BOUNDARY",
		"API-189-PROCESSING-REQUEST-CUSTODY-BOUNDARY",
		"REV-189-PROCESSING-REQUEST-CUSTODY-BOUNDARY",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestProcessingRequestCustodyBoundaryManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"代加工申请库存与目标产品范围",
			"当前客户托管库存",
			"不能使用其他客户",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing processing custody boundary marker %q", path, want)
			}
		}
	}
}
