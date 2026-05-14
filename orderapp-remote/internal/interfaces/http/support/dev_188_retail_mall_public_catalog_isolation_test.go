package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRetailMallPublicCatalogIsolationEvidenceExists(t *testing.T) {
	businessRepo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerportal", "business_repository.go")))
	adminRepo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerportal", "admin_repository.go")))
	repoTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerportal", "repository_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"mallProductPublicCatalogSQL",
		"LoadMallPage(ctx context.Context, customerID int64)",
		"CreateMallOrder(ctx context.Context",
		"mall product unavailable",
	} {
		if !strings.Contains(businessRepo, want) {
			t.Fatalf("business repository missing retail mall public catalog marker %q", want)
		}
	}
	for _, want := range []string{
		"mallProductPublicCatalogSQL",
		"productOptions",
		"SaveMallProduct(ctx context.Context",
		"mall product unavailable",
	} {
		if !strings.Contains(adminRepo, want) {
			t.Fatalf("admin repository missing retail mall public catalog marker %q", want)
		}
	}
	for _, want := range []string{
		"TestLoadMallPageFiltersCustomerOnlyMallProducts",
		"TestCreateMallOrderRejectsCustomerOnlyMallProduct",
		"TestListMallProductsExcludesCustomerOnlyOptions",
		"TestSaveMallProductRejectsCustomerOnlyProduct",
		"商城客户专属不应展示",
		"customer-only mall product created order",
	} {
		if !strings.Contains(repoTest, want) {
			t.Fatalf("customerportal repository test missing retail mall public catalog marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-188-RETAIL-MALL-PUBLIC-CATALOG-ISOLATION",
		"商城商品公共目录范围",
		"TestLoadMallPageFiltersCustomerOnlyMallProducts",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing retail mall public catalog marker %q", want)
		}
	}
}

func TestRetailMallPublicCatalogIsolationRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-188-RETAIL-MALL-PUBLIC-CATALOG-ISOLATION",
		"DEV-188-RETAIL-MALL-PUBLIC-CATALOG-ISOLATION",
		"UT-188-RETAIL-MALL-PUBLIC-CATALOG-ISOLATION",
		"API-188-RETAIL-MALL-PUBLIC-CATALOG-ISOLATION",
		"REV-188-RETAIL-MALL-PUBLIC-CATALOG-ISOLATION",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestRetailMallPublicCatalogIsolationManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"商城商品公共目录范围",
			"公共商品",
			"不能上架客户专属商品",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing retail mall public catalog marker %q", path, want)
			}
		}
	}
}
