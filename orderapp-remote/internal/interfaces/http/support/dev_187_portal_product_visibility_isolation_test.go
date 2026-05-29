package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPortalProductVisibilityIsolationEvidenceExists(t *testing.T) {
	repo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerportal", "business_repository.go")))
	repoTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerportal", "repository_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"listProducts(ctx context.Context, customerID int64",
		"portalProductVisibleToCustomerSQL",
		"portalProductVisibleToCustomerAliasSQL",
		"WHEN COALESCE(%[1]scustomer_id,0)>0 THEN COALESCE(NULLIF(%[1]svisibility,''),'customer_only')",
		"ELSE COALESCE(NULLIF(%[1]svisibility,''),'public')",
		"OR COALESCE(%[1]scustomer_id,0)=%[2]s",
		"product unavailable",
	} {
		if !strings.Contains(repo, want) {
			t.Fatalf("customerportal repository missing product visibility isolation marker %q", want)
		}
	}
	for _, want := range []string{
		"TestLoadProductOrderServicePageFiltersCustomerOnlyProducts",
		"TestCreateFulfillmentOrderRejectsAnotherCustomerOnlyProduct",
		"客户A专属深烘",
		"客户B不应显示专属深烘",
		"another customer product created order",
	} {
		if !strings.Contains(repoTest, want) {
			t.Fatalf("customerportal repository test missing product visibility isolation marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-187-PORTAL-PRODUCT-VISIBILITY-ISOLATION",
		"现货下单商品可见范围",
		"TestLoadProductOrderServicePageFiltersCustomerOnlyProducts",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing product visibility isolation marker %q", want)
		}
	}
}

func TestPortalProductVisibilityIsolationRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-187-PORTAL-PRODUCT-VISIBILITY-ISOLATION",
		"DEV-187-PORTAL-PRODUCT-VISIBILITY-ISOLATION",
		"UT-187-PORTAL-PRODUCT-VISIBILITY-ISOLATION",
		"API-187-PORTAL-PRODUCT-VISIBILITY-ISOLATION",
		"REV-187-PORTAL-PRODUCT-VISIBILITY-ISOLATION",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestPortalProductVisibilityIsolationManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"现货下单商品可见范围",
			"公共商品",
			"不能显示其他客户专属商品",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing product visibility isolation marker %q", path, want)
			}
		}
	}
}
