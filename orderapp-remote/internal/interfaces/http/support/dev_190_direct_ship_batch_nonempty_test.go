package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDirectShipBatchNonemptyEvidenceExists(t *testing.T) {
	service := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "customerportal", "service.go")))
	serviceTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "customerportal", "service_test.go")))
	repo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerportal", "business_repository.go")))
	repoTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerportal", "repository_test.go")))
	miniAPITest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customerportal", "mini_api_test.go")))
	miniappService := string(readOrderAppFileForTest(t, filepath.Join("..", "miniapp", "src", "pages", "service", "service.vue")))
	miniappTest := string(readOrderAppFileForTest(t, filepath.Join("..", "miniapp", "src", "utils", "mall.test.ts")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"cmd.TotalRows <= 0",
		"total_rows invalid",
	} {
		if !strings.Contains(service, want) {
			t.Fatalf("customerportal service missing direct ship nonempty marker %q", want)
		}
	}
	for _, want := range []string{
		"cmd.TotalRows <= 0",
		"total_rows invalid",
	} {
		if !strings.Contains(repo, want) {
			t.Fatalf("customerportal repository missing direct ship nonempty marker %q", want)
		}
	}
	for _, want := range []string{
		"TestCreateDirectShipBatchRejectsEmptyRows",
		"空批次",
	} {
		if !strings.Contains(serviceTest, want) {
			t.Fatalf("customerportal service test missing direct ship nonempty marker %q", want)
		}
	}
	for _, want := range []string{
		"TestCreateDirectShipBatchRejectsEmptyRows",
		"empty direct ship batch inserted",
	} {
		if !strings.Contains(repoTest, want) {
			t.Fatalf("customerportal repository test missing direct ship nonempty marker %q", want)
		}
	}
	if !strings.Contains(miniAPITest, "TestMiniDirectShipBatchEmptyRowsMapsToBadRequest") {
		t.Fatal("mini API tests missing direct ship empty rows bad request marker")
	}
	for _, want := range []string{
		"CustomerDirectShipPanel",
		"show-create",
	} {
		if !strings.Contains(miniappService, want) {
			t.Fatalf("miniapp service page missing replacement direct ship marker %q", want)
		}
	}
	if !strings.Contains(miniappTest, "replaces the mini-program batch form with the single-write direct shipment panel") {
		t.Fatal("miniapp tests missing replacement direct ship frontend marker")
	}
	for _, want := range []string{
		"PR-190-DIRECT-SHIP-BATCH-NONEMPTY",
		"代发批次不能为空",
		"TestCreateDirectShipBatchRejectsEmptyRows",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing direct ship nonempty marker %q", want)
		}
	}
}

func TestDirectShipBatchNonemptyRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-190-DIRECT-SHIP-BATCH-NONEMPTY",
		"DEV-190-DIRECT-SHIP-BATCH-NONEMPTY",
		"UT-190-DIRECT-SHIP-BATCH-NONEMPTY",
		"API-190-DIRECT-SHIP-BATCH-NONEMPTY",
		"REV-190-DIRECT-SHIP-BATCH-NONEMPTY",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestDirectShipBatchNonemptyManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"订单行数必须大于 0",
			"小程序",
			"停止受理",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing direct ship nonempty marker %q", path, want)
			}
		}
	}
}
