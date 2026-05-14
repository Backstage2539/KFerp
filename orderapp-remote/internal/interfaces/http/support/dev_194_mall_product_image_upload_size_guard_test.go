package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMallProductImageUploadSizeGuardEvidenceExists(t *testing.T) {
	adminAPI := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customerportal", "admin_api.go")))
	miniAPITest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customerportal", "mini_api_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"maxMallProductImageUploadBytes",
		"maxMallProductImageUploadBytes+1",
		"image file too large",
	} {
		if !strings.Contains(adminAPI, want) {
			t.Fatalf("customerportal admin API missing mall product image size marker %q", want)
		}
	}
	for _, want := range []string{
		"TestMallAdminImageUploadRejectsOversizedAsset",
		"huge.png",
		"image file too large",
	} {
		if !strings.Contains(miniAPITest, want) {
			t.Fatalf("customerportal API test missing mall product image size marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-194-MALL-PRODUCT-IMAGE-UPLOAD-SIZE-GUARD",
		"商城商品图片上传超过 8MB 时必须拒绝",
		"TestMallAdminImageUploadRejectsOversizedAsset",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing mall product image size marker %q", want)
		}
	}
}

func TestMallProductImageUploadSizeGuardRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-194-MALL-PRODUCT-IMAGE-UPLOAD-SIZE-GUARD",
		"DEV-194-MALL-PRODUCT-IMAGE-UPLOAD-SIZE-GUARD",
		"UT-194-MALL-PRODUCT-IMAGE-UPLOAD-SIZE-GUARD",
		"API-194-MALL-PRODUCT-IMAGE-UPLOAD-SIZE-GUARD",
		"REV-194-MALL-PRODUCT-IMAGE-UPLOAD-SIZE-GUARD",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestMallProductImageUploadSizeGuardManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"商城商品图片上传超过 8MB 时必须拒绝",
			"不能截断后保存为公开商品图片",
			"图片文件过大时接口返回无效请求",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing mall product image size marker %q", path, want)
			}
		}
	}
}
