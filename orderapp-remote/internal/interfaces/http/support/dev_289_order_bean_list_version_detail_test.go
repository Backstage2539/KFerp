package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev289OrderBeanListVersionSharedBackbone(t *testing.T) {
	coreSchema := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "core", "schema.go")))
	for _, want := range []string{
		"bean_list_publication_id BIGINT",
		"bean_list_version_no TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS bean_list_publication_id BIGINT",
		"ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS bean_list_version_no TEXT NOT NULL DEFAULT ''",
	} {
		if !strings.Contains(coreSchema, want) {
			t.Fatalf("order_items schema must persist bean-list version snapshots; missing %q", want)
		}
	}

	salesRepo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "sales", "repository.go")))
	for _, want := range []string{
		"orderbeans.ResolveUsage",
		"bean_list_publication_id",
		"bean_list_version_no",
	} {
		if !strings.Contains(salesRepo, want) {
			t.Fatalf("ERP order save must write bean-list version fields through shared resolver; missing %q", want)
		}
	}

	customerPortalRepo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerportal", "business_repository.go")))
	for _, want := range []string{
		"orderbeans.ResolveUsage",
		"bean_list_publication_id",
		"bean_list_version_no",
		"BeanListVersionNo",
	} {
		if !strings.Contains(customerPortalRepo, want) {
			t.Fatalf("miniapp/customer portal orders must use shared bean-list version fields; missing %q", want)
		}
	}

	fulfillmentRepo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	for _, want := range []string{
		"orderbeans.ResolveUsage",
		"bean_list_publication_id",
		"bean_list_version_no",
	} {
		if !strings.Contains(fulfillmentRepo, want) {
			t.Fatalf("customer fulfillment orders must use shared bean-list version fields; missing %q", want)
		}
	}
}

func TestDev289OrderBeanListVersionDisplayedInDetailsAndManuals(t *testing.T) {
	files := map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue"): {
			"豆单版本",
			"bean_list_version_no",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "CustomerProcessingPortalView.vue"): {
			"豆单版本",
			"bean_list_version_no",
		},
		filepath.Join("..", "miniapp", "src", "pages", "service", "service.vue"): {
			"豆单版本",
			"bean_list_version_no",
		},
		filepath.Join("..", "miniapp", "src", "api", "customerPortal.ts"): {
			"bean_list_publication_id",
			"bean_list_version_no",
		},
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"): {
			"豆单版本",
			"订单明细",
		},
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"): {
			"豆单版本",
			"订单明细",
		},
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"): {
			"豆单版本",
			"订单明细",
		},
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"): {
			"豆单版本",
			"订单明细",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"豆单版本",
			"订单明细",
		},
		filepath.Join("..", "ACCEPTANCE_TESTS.md"): {
			"豆单版本",
			"订单明细",
		},
	}

	for rel, wants := range files {
		body := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(body, want) {
				if want == "豆单版本" && strings.Contains(body, "价格表版本") {
					continue
				}
				t.Fatalf("%s must document/display order bean-list version; missing %q", rel, want)
			}
		}
	}
}
