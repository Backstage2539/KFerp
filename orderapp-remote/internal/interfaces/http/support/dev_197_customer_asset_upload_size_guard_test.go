package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerAssetUploadSizeGuardEvidenceExists(t *testing.T) {
	routes := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customer", "customer_routes.go")))
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customer", "customer_api_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"maxCustomerAssetUploadBytes",
		"MaxBytes:    maxCustomerAssetUploadBytes",
	} {
		if !strings.Contains(routes, want) {
			t.Fatalf("customer routes missing asset upload size guard marker %q", want)
		}
	}
	for _, want := range []string{
		"TestCustomerAssetUploadUsesImageSizeLimit",
		"8<<20",
		"logo.png",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("customer API test missing asset upload size guard marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-197-CUSTOMER-ASSET-UPLOAD-SIZE-GUARD",
		"客户档案资产图片上传超过 8MB 时必须拒绝",
		"TestCustomerAssetUploadUsesImageSizeLimit",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing customer asset upload size guard marker %q", want)
		}
	}
}

func TestCustomerAssetUploadSizeGuardRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-197-CUSTOMER-ASSET-UPLOAD-SIZE-GUARD",
		"DEV-197-CUSTOMER-ASSET-UPLOAD-SIZE-GUARD",
		"UT-197-CUSTOMER-ASSET-UPLOAD-SIZE-GUARD",
		"API-197-CUSTOMER-ASSET-UPLOAD-SIZE-GUARD",
		"REV-197-CUSTOMER-ASSET-UPLOAD-SIZE-GUARD",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestCustomerAssetUploadSizeGuardManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"客户档案资产图片上传超过 8MB 时必须拒绝",
			"PNG/JPEG/WebP",
			"压缩到 8MB 内后再上传",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing customer asset upload size guard marker %q", path, want)
			}
		}
	}
}
