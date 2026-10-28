package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerFulfillmentP123FrontendWiring(t *testing.T) {
	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CustomerFulfillmentView.vue")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md")))
	req := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))

	if strings.Contains(view, "template: `") {
		t.Fatal("customer fulfillment view must not use runtime template strings because production Vue runtime does not compile them")
	}
	for _, old := range []string{"当前可应用批次", "应用当前类型最新批次", "fetchCustomerFulfillmentImportRows"} {
		if strings.Contains(view, old) {
			t.Fatalf("obsolete import UI still present: %q", old)
		}
	}
	for _, want := range []string{"历史导入", "地址簿", "库存代发"} {
		if !strings.Contains(manual, want) {
			t.Fatalf("customer fulfillment manual missing %q", want)
		}
	}
	for _, want := range []string{"PR-160", "DEV-160-01", "DEV-160-02", "DEV-160-03", "UT-160-01", "API-160-01", "REV-160-01"} {
		if !strings.Contains(req, want) {
			t.Fatalf("requirement seed missing %s", want)
		}
	}
}

func TestCustomerFulfillmentP123Permissions(t *testing.T) {
	for _, path := range []string{
		"/api/customer-fulfillment/imports/55/rows",
		"/api/customer-fulfillment/imports/55/preview",
	} {
		if got := requiredPermissionForRequest("GET", path); got != "stock.read" {
			t.Fatalf("GET %s permission = %q, want stock.read", path, got)
		}
	}
}
