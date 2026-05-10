package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerCapabilityTemplatesPageWired(t *testing.T) {
	app := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "App.vue")))
	menu := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js")))
	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CustomerCapabilityTemplatesView.vue")))
	authz := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "authz", "schema.go")))
	adminAPI := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customerportal", "admin_api.go")))

	for _, want := range []string{
		"CustomerCapabilityTemplatesView",
		"customerCapabilityTemplates: CustomerCapabilityTemplatesView",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("App.vue missing %q", want)
		}
	}
	for _, want := range []string{
		"customerCapabilityTemplates",
		"客户能力模板",
		"客户履约运营台",
		"客户代加工工作台",
	} {
		if !strings.Contains(menu, want) {
			t.Fatalf("menu-ia.js missing %q", want)
		}
	}
	for _, want := range []string{
		"/api/customer-portal/admin/capability-templates",
		"客户能力开关",
		"代发和公共 SKU 规则",
		"ERP 权限映射",
		"public_sku_aliases",
		"customer_sender",
		"external_recipients",
		"small_batch_price_rule",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("CustomerCapabilityTemplatesView.vue missing %q", want)
		}
	}
	if !strings.Contains(authz, `"customerCapabilityTemplates": "customers.write"`) {
		t.Fatal("customer capability templates should require customer write permission")
	}
	if !strings.Contains(adminAPI, "SaveCapabilityTemplate") || !strings.Contains(adminAPI, "/api/customer-portal/admin/capability-templates/:key") {
		t.Fatal("admin capability template save API is not wired")
	}
}
