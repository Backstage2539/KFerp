package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev349OrderEntryBeanListSummaryWarning(t *testing.T) {
	reqStore := readDev349Text(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	for _, want := range []string{
		"PR-349-ORDER-ENTRY-BEANLIST-SUMMARY-WARNING",
		"DEV-349-ORDER-ENTRY-BEANLIST-SUMMARY-WARNING",
		"UT-349-ORDER-ENTRY-BEANLIST-SUMMARY-WARNING",
		"API-349-ORDER-ENTRY-BEANLIST-SUMMARY-WARNING",
		"REV-349-ORDER-ENTRY-BEANLIST-SUMMARY-WARNING",
		"2026-05-23-order-entry-beanlist-summary-warning.md",
	} {
		if !strings.Contains(reqStore, want) {
			t.Fatalf("dev 349 req seed missing %q", want)
		}
	}

	requireDev349Contains(t, "docs/REQUIREMENTS.md",
		"PR-349-ORDER-ENTRY-BEANLIST-SUMMARY-WARNING",
		"熟豆、生豆、挂耳豆单分别按行展示",
	)
	requireDev349Contains(t, "docs/ACCEPTANCE_TESTS.md",
		"PR-349-ORDER-ENTRY-BEANLIST-SUMMARY-WARNING",
		"右侧“豆单版本”标红并显示感叹号",
	)
	requireDev349Contains(t, "docs/OP_MANUAL_ORDER_SALES.md",
		"标题区会把当前使用的三个价格表按行展示",
		"切换熟豆、生豆或挂耳价格表版本后，已有商品行会按当前选择的价格表发布 ID 重新取价",
	)
	requireDev349Contains(t, "docs/acceptance/2026-05-23-order-entry-beanlist-summary-warning.md",
		"商品明细标题区能同时看到熟豆豆单、生豆豆单、挂耳豆单三行完整版本信息",
		"旧版本不是该类型最新发布时",
	)

	requireDev349Contains(t, "frontend-vue-shell/src/views/OrderEntryView.vue",
		"selectedBeanListSummaryItems",
		"bean-list-summary-list",
		"v-for=\"item in selectedBeanListSummaryItems\"",
		"function syncRowBeanListVersionFromSelection(row)",
		"syncRowBeanListVersionFromSelection(row)",
	)
	requireDev349Contains(t, "frontend-vue-shell/src/lib/order-entry.js",
		"compareBeanListVersionOption",
		"latestBeanListVersionOption",
		"published_at",
	)
	requireDev349Contains(t, "frontend-vue-shell/src/lib/order-entry.test.js",
		"latestBeanListVersionOption uses the newest published version instead of default flag alone",
		"OrderEntryView shows selected bean lists as readable rows and refreshes row versions from selection",
	)
}

func requireDev349Contains(t *testing.T, path string, wants ...string) {
	t.Helper()
	text := readDev349Text(t, path)
	for _, want := range wants {
		if !strings.Contains(text, want) {
			t.Fatalf("%s missing %q", path, want)
		}
	}
}

func readDev349Text(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}
