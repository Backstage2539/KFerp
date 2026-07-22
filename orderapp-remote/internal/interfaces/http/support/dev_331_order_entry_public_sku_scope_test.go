package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev331OrderEntryPublicSKUScopeSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-331-ORDER-ENTRY-PUBLIC-SKU-SCOPE",
		"DEV-331-ORDER-ENTRY-PUBLIC-SKU-SCOPE",
		"UT-331-ORDER-ENTRY-PUBLIC-SKU-SCOPE",
		"API-331-ORDER-ENTRY-PUBLIC-SKU-SCOPE",
		"REV-331-ORDER-ENTRY-PUBLIC-SKU-SCOPE",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("dev 330 requirement seed missing %q", want)
		}
	}
}

func TestDev331OrderEntryPublicSKUScopeWiring(t *testing.T) {
	for _, tc := range []struct {
		rel     string
		markers []string
	}{
		{
			rel: filepath.Join("internal", "application", "sales", "service.go"),
			markers: []string{
				"CustomerPublicUsageOption",
				"CustomerPublicUsages",
				"UsePublicSKU",
			},
		},
		{
			rel: filepath.Join("internal", "infrastructure", "postgres", "sales", "order_form_queries.go"),
			markers: []string{
				"fetchOrderCustomerPublicUsages",
				"customer_sku_public_usage",
				"SELECT customer_id, use_public_sku",
			},
		},
		{
			rel: filepath.Join("internal", "interfaces", "http", "sales", "order_api.go"),
			markers: []string{
				"customer_public_usages",
				"customerAllowsPublicOrderProducts",
				"productMatchesExplicitCustomerOwnedBeanListScope",
				"filterOrderProductsForCustomer(data.Products, customerID, data.BeanListVersionOptions, data.CustomerPublicUsages)",
			},
		},
		{
			rel: filepath.Join("frontend-vue-shell", "src", "lib", "order-entry.js"),
			markers: []string{
				"customerAllowsPublicSKU",
				"productMatchesExplicitPublicationScope",
				"export function filterProductsForCustomer(",
				"customerOwnedPublicationIDsByType = {}",
			},
		},
		{
			rel: filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue"),
			markers: []string{
				"customerPublicUsages",
				"data.customer_public_usages",
				"customerPublicUsages.value",
			},
		},
	} {
		src := string(readOrderAppFileForTest(t, tc.rel))
		for _, want := range tc.markers {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing dev 330 marker %q", tc.rel, want)
			}
		}
	}
}

func TestDev331OrderEntryPublicSKUScopeDocs(t *testing.T) {
	for _, rel := range []string{
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "acceptance", "2026-05-23-order-entry-public-sku-scope.md"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"PR-331",
			"use_public_sku",
			"岩师傅",
		} {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing dev 330 documentation marker %q", rel, want)
			}
		}
	}
}
