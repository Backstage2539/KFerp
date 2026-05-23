package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev342OrderEntryPublicBeanListVersionSelectSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-342-ORDER-ENTRY-PUBLIC-BEANLIST-VERSION-SELECT",
		"DEV-342-ORDER-ENTRY-PUBLIC-BEANLIST-VERSION-SELECT",
		"UT-342-ORDER-ENTRY-PUBLIC-BEANLIST-VERSION-SELECT",
		"API-342-ORDER-ENTRY-PUBLIC-BEANLIST-VERSION-SELECT",
		"REV-342-ORDER-ENTRY-PUBLIC-BEANLIST-VERSION-SELECT",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PR-342 requirement seed missing %q", want)
		}
	}
}

func TestDev342OrderEntryPublicBeanListVersionSelectWiring(t *testing.T) {
	for _, tc := range []struct {
		rel     string
		markers []string
	}{
		{
			rel: filepath.Join("frontend-vue-shell", "src", "lib", "order-entry.js"),
			markers: []string{
				"beanListVersionOptionsForCustomer",
				"tiersForSelectedPublication",
				"tierPublicationID",
			},
		},
		{
			rel: filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue"),
			markers: []string{
				"beanListVersionOptionsForCustomer",
				"customerBeanListVersionOptions",
				"selectedBeanListPublicationIDsByType",
				"syncRowsForType()",
			},
		},
		{
			rel: filepath.Join("internal", "infrastructure", "postgres", "sales", "order_form_queries.go"),
			markers: []string{
				"b.status='published'",
				"row_number() OVER (PARTITION BY b.list_type ORDER BY b.published_at DESC, b.id DESC) = 1 AS is_default",
				"fetchCommercialOrderPublicationTiers",
			},
		},
		{
			rel: filepath.Join("internal", "infrastructure", "postgres", "orderbeans", "usage.go"),
			markers: []string{
				"AND blp.status='published'",
			},
		},
		{
			rel: filepath.Join("internal", "infrastructure", "postgres", "sales", "repository.go"),
			markers: []string{
				"status='published'",
				"requestedPublicationID",
			},
		},
		{
			rel: filepath.Join("internal", "interfaces", "http", "sales", "order_api_test.go"),
			markers: []string{
				"TestOrderAPIFormHidesWithdrawnPublicBeanListVersionsForFallbackCustomer",
				"TestOrderAPIRejectsWithdrawnPublicBeanListPublicationVersion",
			},
		},
	} {
		src := string(readOrderAppFileForTest(t, tc.rel))
		for _, want := range tc.markers {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-342 marker %q", tc.rel, want)
			}
		}
	}
}

func TestDev342OrderEntryPublicBeanListVersionSelectDocs(t *testing.T) {
	for _, rel := range []string{
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "acceptance", "2026-05-23-order-entry-public-beanlist-version-select.md"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"PR-342-ORDER-ENTRY-PUBLIC-BEANLIST-VERSION-SELECT",
			"公共豆单",
			"历史发布版本",
			"默认",
			"最新",
		} {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-342 documentation marker %q", rel, want)
			}
		}
	}
}
