package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDeliveryNoteGeneratedFileCleanupEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "sales", "delivery_note_repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "sales", "delivery_note_repository_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"cleanupGeneratedDeliveryNoteAssetFile",
		"GenerateDeliveryNoteDocument",
		"fileWritten",
		"committed",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("delivery note repository missing generated-file cleanup marker %q", want)
		}
	}
	for _, want := range []string{
		"TestGenerateDeliveryNoteDocumentCleansFileWhenDocumentInsertFails",
		"delivery_note_document_test_reject_latest",
		"fakeDeliveryNoteRenderer",
		"assertSalesPostgresAssetDirEmpty",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("delivery note repository test missing generated-file cleanup marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-205-DELIVERY-NOTE-GENERATED-FILE-CLEANUP",
		"出库单 PDF 生成失败时必须清理刚写入的文件",
		"TestGenerateDeliveryNoteDocumentCleansFileWhenDocumentInsertFails",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing delivery note generated-file cleanup marker %q", want)
		}
	}
}

func TestDeliveryNoteGeneratedFileCleanupRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-205-DELIVERY-NOTE-GENERATED-FILE-CLEANUP",
		"DEV-205-DELIVERY-NOTE-GENERATED-FILE-CLEANUP",
		"UT-205-DELIVERY-NOTE-GENERATED-FILE-CLEANUP",
		"API-205-DELIVERY-NOTE-GENERATED-FILE-CLEANUP",
		"REV-205-DELIVERY-NOTE-GENERATED-FILE-CLEANUP",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestDeliveryNoteGeneratedFileCleanupManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"出库单 PDF 生成失败时必须清理刚写入的文件",
			"不会留下公开孤儿出库单文件",
			"重新生成出库单 PDF",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing delivery note generated-file cleanup marker %q", path, want)
			}
		}
	}
}
