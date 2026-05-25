package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBeanListPublicationTenantIsolationEvidenceExists(t *testing.T) {
	repo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerportal", "business_repository.go")))
	repoTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerportal", "repository_test.go")))
	miniAPITest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customerportal", "mini_api_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"LoadBeanListPublication(ctx context.Context, customerID, publicationID int64)",
		"owner_type='customer' AND owner_key=$2",
		"owner_type='official'",
	} {
		if !strings.Contains(repo, want) {
			t.Fatalf("customerportal business repository missing bean list tenant marker %q", want)
		}
	}
	for _, want := range []string{
		"TestLoadBeanListPublicationRejectsAnotherCustomerPublication",
		"TestLoadBeanListPublicationAllowsOfficialPublication",
		"客户B专属豆单不应下载",
	} {
		if !strings.Contains(repoTest, want) {
			t.Fatalf("customerportal repository test missing bean list tenant marker %q", want)
		}
	}
	if !strings.Contains(miniAPITest, "TestMiniBeanListPDFPublicationNotFoundMapsToNotFound") {
		t.Fatal("mini API tests missing bean list publication 404 marker")
	}
	for _, want := range []string{
		"PR-192-BEAN-LIST-PUBLICATION-TENANT-ISOLATION",
		"客户专属豆单 PDF 只能由归属客户访问",
		"TestLoadBeanListPublicationRejectsAnotherCustomerPublication",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing bean list tenant marker %q", want)
		}
	}
}

func TestBeanListPublicationTenantIsolationRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-192-BEAN-LIST-PUBLICATION-TENANT-ISOLATION",
		"DEV-192-BEAN-LIST-PUBLICATION-TENANT-ISOLATION",
		"UT-192-BEAN-LIST-PUBLICATION-TENANT-ISOLATION",
		"API-192-BEAN-LIST-PUBLICATION-TENANT-ISOLATION",
		"REV-192-BEAN-LIST-PUBLICATION-TENANT-ISOLATION",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestBeanListPublicationTenantIsolationManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"客户专属豆单 PDF 只能由归属客户访问",
			"官方已发布豆单可作为公共兜底访问",
			"不能下载其他客户专属豆单",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing bean list tenant marker %q", path, want)
			}
		}
	}
}
