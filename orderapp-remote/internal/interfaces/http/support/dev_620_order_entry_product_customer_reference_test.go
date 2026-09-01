package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev620OrderEntryProductCustomerReferenceContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "infrastructure", "postgres", "sales", "order_form_queries.go"): {
			"fetchOrderCustomerReferenceProducts",
			"product_customer_references",
			"customer_reference",
			"aliasFamilies",
		},
		filepath.Join("internal", "application", "sales", "order_product_filter.go"): {
			"customerScopedProductIDs",
			"customer_reference",
		},
		filepath.Join("internal", "interfaces", "http", "sales", "order_api_test.go"): {
			"TestOrderAPIFormUsesProductCustomerReferenceForCustomerPublicationTiers",
			"TestOrderAPIFormUsesProductCustomerReferenceForGreenPublication",
			"legacy alias should keep precedence",
		},
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-620-ORDER-ENTRY-PRODUCT-CUSTOMER-REFERENCE",
			"DEV-620-CUSTOMER-REFERENCE-ORDER-PROJECTION",
			"DEV-620-LEGACY-ALIAS-PRECEDENCE",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-620-ORDER-ENTRY-PRODUCT-CUSTOMER-REFERENCE",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-620-ORDER-ENTRY-PRODUCT-CUSTOMER-REFERENCE",
		},
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"): {
			"`客户引用` 不定义价格",
			"旧客户商品别名",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-620 marker %q", rel, want)
			}
		}
	}
}
