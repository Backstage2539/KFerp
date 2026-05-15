package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerAccountPortalTypesWiredAcrossUIAndAPIs(t *testing.T) {
	customerSchema := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "core", "schema.go")))
	companySchema := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "company", "schema.go")))
	customerRepo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customer", "repository.go")))
	portalAdmin := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customerportal", "admin_api.go")))
	employeeMaintenance := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CompanyStaffView.vue")))
	portalSettings := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CustomerPortalSettingsView.vue")))
	customersView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CustomersView.vue")))

	for _, want := range []string{
		"customer_type TEXT NOT NULL DEFAULT 'retail'",
		"ADD COLUMN IF NOT EXISTS customer_type TEXT NOT NULL DEFAULT 'retail'",
	} {
		if !strings.Contains(customerSchema, want) {
			t.Fatalf("core schema missing %q", want)
		}
	}
	if !strings.Contains(companySchema, "account_type TEXT NOT NULL DEFAULT 'internal_employee'") {
		t.Fatal("company schema must default historical accounts to internal employees")
	}
	for _, want := range []string{
		"ensureDefaultRetailPortalTx",
		"retail_mall",
		"miniapp_entry_mode='mall'",
	} {
		if !strings.Contains(customerRepo, want) {
			t.Fatalf("customer repository missing retail/ecommerce portal default %q", want)
		}
	}
	for _, want := range []string{
		"/api/customer-portal/admin/customers/:id/erp-binding",
		"UpsertPortalERPBinding",
	} {
		if !strings.Contains(portalAdmin, want) {
			t.Fatalf("portal admin API missing ERP binding endpoint %q", want)
		}
	}
	for _, want := range []string{
		"fetchInternalAuthAccounts",
		"内部权限",
		"resetEmployeePassword",
	} {
		if !strings.Contains(employeeMaintenance, want) {
			t.Fatalf("CompanyStaffView.vue missing internal permission behavior %q", want)
		}
	}
	for _, forbidden := range []string{"account_type", "渠道客户", "setAccountType", "isChannelCustomer", "公共SKU代发客户", "代加工客户"} {
		if strings.Contains(employeeMaintenance, forbidden) {
			t.Fatalf("employee maintenance page must not show external account behavior %q", forbidden)
		}
	}
	for _, want := range []string{
		"erp_binding",
		"goToFulfillmentAccount",
		"去履约运营台管理",
	} {
		if !strings.Contains(portalSettings, want) {
			t.Fatalf("CustomerPortalSettingsView.vue missing fulfillment account handoff UI %q", want)
		}
	}
	for _, forbidden := range []string{"saveERPBinding", "/api/auth/accounts", "channel_customer"} {
		if strings.Contains(portalSettings, forbidden) {
			t.Fatalf("CustomerPortalSettingsView.vue must not manage external accounts directly %q", forbidden)
		}
	}
	for _, want := range []string{
		"customer_type",
		"批发客户",
		"零售客户",
		"电商客户",
	} {
		if !strings.Contains(customersView, want) {
			t.Fatalf("CustomersView.vue missing customer type UI %q", want)
		}
	}
}

func TestCustomerAccountPortalRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-173-CUSTOMER-ACCOUNT-PORTAL-TYPES",
		"DEV-173-CUSTOMER-ACCOUNT-PORTAL-TYPES",
		"UT-173-CUSTOMER-ACCOUNT-PORTAL-TYPES",
		"API-173-CUSTOMER-ACCOUNT-PORTAL-TYPES",
		"REV-173-CUSTOMER-ACCOUNT-PORTAL-TYPES",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}
