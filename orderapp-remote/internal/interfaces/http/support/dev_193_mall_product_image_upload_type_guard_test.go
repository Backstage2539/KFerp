package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMallProductImageUploadTypeGuardEvidenceExists(t *testing.T) {
	adminAPI := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customerportal", "admin_api.go")))
	miniAPITest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customerportal", "mini_api_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"isAllowedMallProductImage",
		"image/png",
		"image/jpeg",
		"image file required",
	} {
		if !strings.Contains(adminAPI, want) {
			t.Fatalf("customerportal admin API missing mall product image type marker %q", want)
		}
	}
	for _, want := range []string{
		"TestMallAdminImageUploadRejectsNonImageAsset",
		"promo.html",
		"image file required",
		"tinyPNGForMallProductUploadTest",
	} {
		if !strings.Contains(miniAPITest, want) {
			t.Fatalf("customerportal API test missing mall product image type marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-193-MALL-PRODUCT-IMAGE-UPLOAD-TYPE-GUARD",
		"商城商品图片上传只接受图片文件",
		"TestMallAdminImageUploadRejectsNonImageAsset",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing mall product image type marker %q", want)
		}
	}
}

func TestMallProductImageUploadTypeGuardRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-193-MALL-PRODUCT-IMAGE-UPLOAD-TYPE-GUARD",
		"DEV-193-MALL-PRODUCT-IMAGE-UPLOAD-TYPE-GUARD",
		"UT-193-MALL-PRODUCT-IMAGE-UPLOAD-TYPE-GUARD",
		"API-193-MALL-PRODUCT-IMAGE-UPLOAD-TYPE-GUARD",
		"REV-193-MALL-PRODUCT-IMAGE-UPLOAD-TYPE-GUARD",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestMallProductImageUploadTypeGuardManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"商城商品图片上传只接受图片文件",
			"不能上传 HTML 或脚本文件",
			"图片文件格式不支持时接口返回无效请求",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing mall product image type marker %q", path, want)
			}
		}
	}
}
