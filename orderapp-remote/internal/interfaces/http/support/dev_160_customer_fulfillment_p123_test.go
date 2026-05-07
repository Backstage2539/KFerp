package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerFulfillmentP123FrontendWiring(t *testing.T) {
	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CustomerFulfillmentView.vue")))
	api := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "api", "customer-fulfillment.js")))
	lib := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "customer-fulfillment.js")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md")))
	req := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))

	if strings.Contains(view, "template: `") {
		t.Fatal("customer fulfillment view must not use runtime template strings because production Vue runtime does not compile them")
	}
	for _, want := range []string{
		"当前可应用批次",
		"应用当前类型最新批次",
		"查看错误行",
		"应用预览",
		"selectedParsedBatch",
		"fetchCustomerFulfillmentImportRows",
		"fetchCustomerFulfillmentImportPreview",
	} {
		if !strings.Contains(view, want) && !strings.Contains(api, want) && !strings.Contains(lib, want) {
			t.Fatalf("customer fulfillment P1/P2/P3 wiring missing %q", want)
		}
	}
	for _, want := range []string{"应用当前类型最新批次", "错误行", "应用预览", "查看错误行"} {
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
