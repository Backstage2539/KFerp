package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev351OrderEntryPublicBeanListNoCustomer(t *testing.T) {
	reqStore := readDev351Text(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	for _, want := range []string{
		"PR-351-ORDER-ENTRY-PUBLIC-BEANLIST-NO-CUSTOMER",
		"DEV-351-ORDER-ENTRY-PUBLIC-BEANLIST-NO-CUSTOMER",
		"UT-351-ORDER-ENTRY-PUBLIC-BEANLIST-NO-CUSTOMER",
		"API-351-ORDER-ENTRY-PUBLIC-BEANLIST-NO-CUSTOMER",
		"REV-351-ORDER-ENTRY-PUBLIC-BEANLIST-NO-CUSTOMER",
		"2026-05-24-order-entry-public-beanlist-no-customer.md",
	} {
		if !strings.Contains(reqStore, want) {
			t.Fatalf("dev 351 req seed missing %q", want)
		}
	}

	requireDev351Contains(t, "docs/REQUIREMENTS.md",
		"PR-351-ORDER-ENTRY-PUBLIC-BEANLIST-NO-CUSTOMER",
		"尚未选择客户",
		"公共已发布豆单",
	)
	requireDev351Contains(t, "docs/ACCEPTANCE_TESTS.md",
		"PR-351-ORDER-ENTRY-PUBLIC-BEANLIST-NO-CUSTOMER",
		"customer_id=0",
		"公共已发布",
	)
	requireDev351Contains(t, "docs/OP_MANUAL_ORDER_SALES.md",
		"PR-351-ORDER-ENTRY-PUBLIC-BEANLIST-NO-CUSTOMER",
		"尚未选择客户",
		"公共已发布价格表版本",
	)
	requireDev351Contains(t, "docs/acceptance/2026-05-24-order-entry-public-beanlist-no-customer.md",
		"customer_id=0",
		"Front",
	)
	requireDev351Contains(t, "internal/infrastructure/postgres/sales/order_form_queries.go",
		"global_public_versions AS",
		"0::bigint AS customer_id",
	)
	requireDev351Contains(t, "internal/interfaces/http/sales/order_api_test.go",
		"TestOrderAPIFormReturnsGlobalPublicBeanListVersionsBeforeCustomerSelected",
	)
	requireDev351Contains(t, "frontend-vue-shell/src/lib/order-entry.js",
		"beanListVersionOptionsForCustomer",
		"seen.add(key)",
	)
	requireDev351Contains(t, "frontend-vue-shell/src/views/OrderEntryView.vue",
		"canOpenBeanListDrawer",
		":disabled=\"!canOpenBeanListDrawer\"",
	)
}

func requireDev351Contains(t *testing.T, path string, wants ...string) {
	t.Helper()
	text := readDev351Text(t, path)
	for _, want := range wants {
		if !strings.Contains(text, want) {
			t.Fatalf("%s missing %q", path, want)
		}
	}
}

func readDev351Text(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}
