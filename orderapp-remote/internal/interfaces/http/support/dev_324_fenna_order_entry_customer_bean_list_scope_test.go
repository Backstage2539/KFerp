package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev324FennaOrderEntryCustomerBeanListScopeSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-324-FENNA-ORDER-ENTRY-CUSTOMER-BEAN-LIST-SCOPE",
		"DEV-324-FENNA-ORDER-ENTRY-CUSTOMER-BEAN-LIST-SCOPE",
		"UT-324-FENNA-ORDER-ENTRY-CUSTOMER-BEAN-LIST-SCOPE",
		"API-324-FENNA-ORDER-ENTRY-CUSTOMER-BEAN-LIST-SCOPE",
		"REV-324-FENNA-ORDER-ENTRY-CUSTOMER-BEAN-LIST-SCOPE",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("dev 324 requirement seed missing %q", want)
		}
	}
}

func TestDev324FennaOrderEntryCustomerBeanListScopeWiring(t *testing.T) {
	querySrc := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "sales", "order_form_queries.go")))
	for _, want := range []string{
		"fetchCommercialOrderPublicationTiers",
		"commercialOrderTierMapFromPublicationContent",
		"commercial_wholesale_tiers",
		"applyCommercialOrderPublicationTiers",
	} {
		if !strings.Contains(querySrc, want) {
			t.Fatalf("order form query wiring missing %q", want)
		}
	}

	apiSrc := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "sales", "order_api.go")))
	for _, want := range []string{
		"customerOwnedPublicationIDsByListType",
		"productMatchesCustomerOwnedBeanListScope",
		"filterOrderProductsForCustomer(data.Products, customerID, data.BeanListVersionOptions, data.CustomerPublicUsages)",
	} {
		if !strings.Contains(apiSrc, want) {
			t.Fatalf("order API customer bean-list scope wiring missing %q", want)
		}
	}

	libSrc := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "order-entry.js")))
	for _, want := range []string{
		"normalizePublicationIDsByType",
		"productMatchesPublicationScope",
		"filterProductsForCustomer(products, customerID, publicationIDsByType = {}, publicUsages = [])",
	} {
		if !strings.Contains(libSrc, want) {
			t.Fatalf("order-entry lib scope wiring missing %q", want)
		}
	}

	viewSrc := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue")))
	for _, want := range []string{
		"customerOwnedBeanListPublicationIDsByType",
		"customerPublicUsages.value",
	} {
		if !strings.Contains(viewSrc, want) {
			t.Fatalf("order entry view scope wiring missing %q", want)
		}
	}
}

func TestDev324FennaOrderEntryCustomerBeanListScopeDocs(t *testing.T) {
	for _, rel := range []string{
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "acceptance", "2026-05-22-fenna-order-entry-customer-bean-list-scope.md"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"PR-324",
			"芬纳",
			"commercial_wholesale_tiers",
		} {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing dev 324 documentation marker %q", rel, want)
			}
		}
	}
}
