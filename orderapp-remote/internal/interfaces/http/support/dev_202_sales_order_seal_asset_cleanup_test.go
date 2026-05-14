package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSalesOrderSealAssetCleanupEvidenceExists(t *testing.T) {
	settingsAPI := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "sales", "sales_order_settings.go")))
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "sales", "sales_order_api_test.go")))
	service := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "sales", "service.go")))
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "sales", "sales_order_repository.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"cleanupSavedSalesOrderAsset",
		"cleanupSalesOrderAssetFile",
		"DeleteSalesOrderAsset",
	} {
		if !strings.Contains(settingsAPI, want) {
			t.Fatalf("sales order settings API missing seal cleanup marker %q", want)
		}
	}
	for _, want := range []string{
		"TestSalesOrderSealUploadCleansFileWhenAssetMetadataFails",
		"TestSalesOrderSealUploadCleansAssetWhenSettingsUpdateFails",
		"assertAssetDirEmpty",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("sales order API test missing seal cleanup marker %q", want)
		}
	}
	for _, want := range []string{
		"DeleteSalesOrderAsset(ctx context.Context, id int64, actor string) error",
		"func (s *Service) DeleteSalesOrderAsset",
	} {
		if !strings.Contains(service, want) {
			t.Fatalf("sales service missing seal cleanup marker %q", want)
		}
	}
	if !strings.Contains(repository, "DELETE FROM %s.sales_order_assets WHERE id=$1") {
		t.Fatalf("sales repository missing asset cleanup delete SQL")
	}
	for _, want := range []string{
		"PR-202-SALES-ORDER-SEAL-ASSET-CLEANUP",
		"销售单公章上传或去除背景失败时不能留下公开孤儿公章资产",
		"TestSalesOrderSealUploadCleansFileWhenAssetMetadataFails",
		"TestSalesOrderSealUploadCleansAssetWhenSettingsUpdateFails",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing seal cleanup marker %q", want)
		}
	}
}

func TestSalesOrderSealAssetCleanupRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-202-SALES-ORDER-SEAL-ASSET-CLEANUP",
		"DEV-202-SALES-ORDER-SEAL-ASSET-CLEANUP",
		"UT-202-SALES-ORDER-SEAL-ASSET-CLEANUP",
		"API-202-SALES-ORDER-SEAL-ASSET-CLEANUP",
		"REV-202-SALES-ORDER-SEAL-ASSET-CLEANUP",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestSalesOrderSealAssetCleanupManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"销售单公章上传或去除背景失败时必须清理刚写入的公章资产",
			"不会留下公开孤儿公章文件",
			"重新上传公章",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing seal cleanup marker %q", path, want)
			}
		}
	}
}
