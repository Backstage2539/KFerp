package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerAssetMetadataFailureCleanupEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customer", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customer", "repository_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"cleanupCustomerAssetFile",
		"insertCustomerAsset",
		"saveAssetFile",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("customer repository missing asset metadata cleanup marker %q", want)
		}
	}
	for _, want := range []string{
		"TestSaveAssetCleansFileWhenMetadataInsertFails",
		"failed customer asset metadata insert left orphan asset entries",
		"ORDERAPP_TEST_DATABASE_URL",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer repository test missing asset metadata cleanup marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-200-CUSTOMER-ASSET-METADATA-FAILURE-CLEANUP",
		"客户档案资产元数据保存失败时必须清理刚写入的文件",
		"TestSaveAssetCleansFileWhenMetadataInsertFails",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing customer asset metadata cleanup marker %q", want)
		}
	}
}

func TestCustomerAssetMetadataFailureCleanupRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-200-CUSTOMER-ASSET-METADATA-FAILURE-CLEANUP",
		"DEV-200-CUSTOMER-ASSET-METADATA-FAILURE-CLEANUP",
		"UT-200-CUSTOMER-ASSET-METADATA-FAILURE-CLEANUP",
		"API-200-CUSTOMER-ASSET-METADATA-FAILURE-CLEANUP",
		"REV-200-CUSTOMER-ASSET-METADATA-FAILURE-CLEANUP",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestCustomerAssetMetadataFailureCleanupManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"客户档案资产元数据保存失败时必须清理刚写入的文件",
			"不会留下公开孤儿客户资产",
			"重新打开客户档案确认附件列表",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing customer asset metadata cleanup marker %q", path, want)
			}
		}
	}
}
