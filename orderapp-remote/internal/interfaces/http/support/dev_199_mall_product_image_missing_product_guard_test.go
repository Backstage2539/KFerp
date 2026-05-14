package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMallProductImageMissingProductGuardEvidenceExists(t *testing.T) {
	adminAPI := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customerportal", "admin_api.go")))
	miniAPITest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customerportal", "mini_api_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"readUploadedMallProductImage",
		"ensureMallProductImageUploadTarget",
		"saveMallProductImageData",
		"cleanupMallProductImageAsset",
		"mall product unavailable",
	} {
		if !strings.Contains(adminAPI, want) {
			t.Fatalf("customerportal admin API missing mall product image missing-product marker %q", want)
		}
	}
	for _, want := range []string{
		"TestMallAdminImageUploadRejectsMissingMallProductWithoutWritingAsset",
		"TestMallAdminImageUploadCleansAssetWhenImageUpdateFails",
		"mall product unavailable",
		"orphan asset entries",
	} {
		if !strings.Contains(miniAPITest, want) {
			t.Fatalf("customerportal API test missing mall product image missing-product marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-199-MALL-PRODUCT-IMAGE-MISSING-PRODUCT-GUARD",
		"商城商品图片上传必须先确认商品存在",
		"TestMallAdminImageUploadRejectsMissingMallProductWithoutWritingAsset",
		"TestMallAdminImageUploadCleansAssetWhenImageUpdateFails",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing mall product image missing-product marker %q", want)
		}
	}
}

func TestMallProductImageMissingProductGuardRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-199-MALL-PRODUCT-IMAGE-MISSING-PRODUCT-GUARD",
		"DEV-199-MALL-PRODUCT-IMAGE-MISSING-PRODUCT-GUARD",
		"UT-199-MALL-PRODUCT-IMAGE-MISSING-PRODUCT-GUARD",
		"API-199-MALL-PRODUCT-IMAGE-MISSING-PRODUCT-GUARD",
		"REV-199-MALL-PRODUCT-IMAGE-MISSING-PRODUCT-GUARD",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestMallProductImageMissingProductGuardManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"商城商品图片上传必须先确认商品存在",
			"缺失商品或图片更新失败时不能留下公开孤儿资产",
			"商品不存在时接口返回无效请求",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing mall product image missing-product marker %q", path, want)
			}
		}
	}
}
