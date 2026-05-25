package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSalesOrderGeneratedFileCleanupEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "sales", "sales_order_repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "sales", "sales_order_repository_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"cleanupGeneratedSalesOrderAssetFile",
		"GenerateSalesOrderDocument",
		"GenerateSalesOrderImage",
		"fileWritten",
		"committed",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("sales order repository missing generated-file cleanup marker %q", want)
		}
	}
	for _, want := range []string{
		"TestGenerateSalesOrderDocumentCleansFileWhenDocumentInsertFails",
		"TestGenerateSalesOrderImageCleansFileWhenImageInsertFails",
		"sales_order_document_test_reject_latest",
		"sales_order_image_test_reject_latest",
		"assertSalesPostgresAssetDirEmpty",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("sales order repository test missing generated-file cleanup marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-204-SALES-ORDER-GENERATED-FILE-CLEANUP",
		"销售单 PDF/图片生成失败时必须清理刚写入的文件",
		"TestGenerateSalesOrderDocumentCleansFileWhenDocumentInsertFails",
		"TestGenerateSalesOrderImageCleansFileWhenImageInsertFails",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing sales order generated-file cleanup marker %q", want)
		}
	}
}

func TestSalesOrderGeneratedFileCleanupRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-204-SALES-ORDER-GENERATED-FILE-CLEANUP",
		"DEV-204-SALES-ORDER-GENERATED-FILE-CLEANUP",
		"UT-204-SALES-ORDER-GENERATED-FILE-CLEANUP",
		"API-204-SALES-ORDER-GENERATED-FILE-CLEANUP",
		"REV-204-SALES-ORDER-GENERATED-FILE-CLEANUP",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestSalesOrderGeneratedFileCleanupManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"销售单 PDF/图片生成失败时必须清理刚写入的文件",
			"不会留下公开孤儿销售单文件",
			"重新生成销售单 PDF 或图片",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing sales order generated-file cleanup marker %q", path, want)
			}
		}
	}
}
