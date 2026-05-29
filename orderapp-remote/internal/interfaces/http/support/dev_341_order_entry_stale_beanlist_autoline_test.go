package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev341OrderEntryStaleBeanListAutoLineSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-341-ORDER-ENTRY-STALE-BEANLIST-AUTOLINE",
		"DEV-341-ORDER-ENTRY-STALE-BEANLIST-AUTOLINE",
		"UT-341-ORDER-ENTRY-STALE-BEANLIST-AUTOLINE",
		"API-341-ORDER-ENTRY-STALE-BEANLIST-AUTOLINE",
		"REV-341-ORDER-ENTRY-STALE-BEANLIST-AUTOLINE",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PR-340 requirement seed missing %q", want)
		}
	}
}

func TestDev341OrderEntryStaleBeanListAutoLineWiring(t *testing.T) {
	for _, tc := range []struct {
		rel     string
		markers []string
	}{
		{
			rel: filepath.Join("frontend-vue-shell", "src", "lib", "order-entry.js"),
			markers: []string{
				"latestBeanListVersionOption",
				"rowUsesStaleBeanListPublication",
				"needsTrailingBlankOrderLine",
				"isBlankOrderLine",
			},
		},
		{
			rel: filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue"),
			markers: []string{
				"bean-list-version-warning",
				"非新版本豆单",
				"toggleBeanListVersionTip",
				"ensureTrailingBlankRow",
				"class=\"line-actions\"",
			},
		},
		{
			rel: filepath.Join("internal", "interfaces", "http", "sales", "order_api_test.go"),
			markers: []string{
				"TestOrderAPIFormReturnsLatestBeanListVersionDefaultForStaleWarning",
				"IsDefault",
				"old=false latest=true",
			},
		},
	} {
		src := string(readOrderAppFileForTest(t, tc.rel))
		for _, want := range tc.markers {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-340 marker %q", tc.rel, want)
			}
		}
	}
}

func TestDev341OrderEntryStaleBeanListAutoLineDocs(t *testing.T) {
	for _, rel := range []string{
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "acceptance", "2026-05-23-order-entry-stale-beanlist-autoline.md"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"PR-341-ORDER-ENTRY-STALE-BEANLIST-AUTOLINE",
			"非新版本豆单",
			"自动补一个空明细",
			"新增明细",
		} {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-340 documentation marker %q", rel, want)
			}
		}
	}
}
