package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRetailMallERPWorkbenchGuardEvidenceExists(t *testing.T) {
	serviceTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "customerportal", "service_test.go")))
	adminAPITest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customerportal", "mini_api_test.go")))
	customerFulfillmentDBTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))
	portalSettingsView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CustomerPortalSettingsView.vue")))
	portalSettingsTest := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "customer-portal-theme.test.js")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"TestUpsertPortalERPBindingRejectsRetailMallTemplate",
		"CapabilityTemplateRetailMall",
		"ERP workbench",
	} {
		if !strings.Contains(serviceTest, want) {
			t.Fatalf("customerportal service test missing marker %q", want)
		}
	}
	for _, want := range []string{
		"TestPortalAdminERPBindingRejectsTemplatesWithoutWorkbench",
		"/api/customer-portal/admin/customers/147/erp-binding",
		"ERP workbench",
	} {
		if !strings.Contains(adminAPITest, want) {
			t.Fatalf("customerportal admin API test missing marker %q", want)
		}
	}
	if !strings.Contains(customerFulfillmentDBTest, "TestUpsertCustomerERPBindingDoesNotGrantHiddenTemplateRoles") {
		t.Fatal("customer fulfillment PostgreSQL test must document that templates do not grant hidden ERP roles")
	}
	if strings.Contains(customerFulfillmentDBTest, "customer_direct_ship_customer") {
		t.Fatal("customer fulfillment PostgreSQL tests must not expect the removed hidden customer_direct_ship_customer role")
	}
	for _, want := range []string{
		"templateSupportsERPWorkbench",
		"该模板不开放 ERP 工作台",
	} {
		if !strings.Contains(portalSettingsView, want) {
			t.Fatalf("customer portal settings view missing marker %q", want)
		}
		if !strings.Contains(portalSettingsTest, want) {
			t.Fatalf("customer portal settings frontend test missing marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-178-RETAIL-MALL-ERP-WORKBENCH-GUARD",
		"零售商城客户不开放 ERP 工作台绑定",
		"TestUpsertPortalERPBindingRejectsRetailMallTemplate",
		"TestPortalAdminERPBindingRejectsTemplatesWithoutWorkbench",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing marker %q", want)
		}
	}
}

func TestRetailMallERPWorkbenchGuardRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-178-RETAIL-MALL-ERP-WORKBENCH-GUARD",
		"DEV-178-RETAIL-MALL-ERP-WORKBENCH-GUARD",
		"UT-178-RETAIL-MALL-ERP-WORKBENCH-GUARD",
		"API-178-RETAIL-MALL-ERP-WORKBENCH-GUARD",
		"REV-178-RETAIL-MALL-ERP-WORKBENCH-GUARD",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}
