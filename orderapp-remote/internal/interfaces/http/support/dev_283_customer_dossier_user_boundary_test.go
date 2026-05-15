package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerDossierUserBoundarySourceGuards(t *testing.T) {
	customersView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CustomersView.vue")))
	companyStaffView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CompanyStaffView.vue")))
	companyRepo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "company", "repository.go")))
	mobileAuth := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "mobile_auth.go")))
	fulfillmentAPI := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customerfulfillment", "api.go")))
	fulfillmentView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CustomerFulfillmentView.vue")))
	portalSettings := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CustomerPortalSettingsView.vue")))

	for _, want := range []string{"customerDrawerOpen", "openCustomerDrawer", "closeCustomerDrawer", "drawer-mask", "@click=\"openCustomerDrawer(row.id)\""} {
		if !strings.Contains(customersView, want) {
			t.Fatalf("CustomersView.vue missing drawer marker %q", want)
		}
	}
	if strings.Contains(customersView, "<th>操作</th>") || strings.Contains(customersView, "editCustomer(row.id)\">编辑") {
		t.Fatal("customer list must not render the old edit operation column/button")
	}
	if strings.Contains(companyStaffView, "setAccountType") || strings.Contains(companyStaffView, "渠道客户") || strings.Contains(companyStaffView, "account_type") {
		t.Fatal("employee maintenance view must not expose external account type controls")
	}
	for _, want := range []string{"/api/auth/internal-accounts", "account_type='internal_employee'"} {
		if !strings.Contains(mobileAuth, want) {
			t.Fatalf("auth API missing internal account marker %q", want)
		}
	}
	if !strings.Contains(companyRepo, "account_type='internal_employee'") {
		t.Fatal("company employee repository must filter employee maintenance to internal users")
	}
	for _, want := range []string{"external-users", "CreateExternalUser", "ResetExternalUserPassword", "SetExternalUserLoginEnabled"} {
		if !strings.Contains(fulfillmentAPI, want) {
			t.Fatalf("customer fulfillment API missing external user marker %q", want)
		}
	}
	for _, want := range []string{"外部用户配置已移到“门户客户配置”", "选择客户", "载入账户"} {
		if !strings.Contains(fulfillmentView, want) {
			t.Fatalf("CustomerFulfillmentView.vue missing fulfillment handoff marker %q", want)
		}
	}
	for _, want := range []string{"外部用户", "createExternalUser", "resetExternalUserPassword", "toggleExternalUserLogin"} {
		if !strings.Contains(portalSettings, want) {
			t.Fatalf("CustomerPortalSettingsView.vue missing external user management marker %q", want)
		}
	}
}

func TestCustomerDossierUserBoundaryRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-283-CUSTOMER-DOSSIER-USER-BOUNDARY",
		"DEV-283-CUSTOMER-DRAWER",
		"DEV-283-INTERNAL-USER-PERMISSIONS",
		"DEV-283-FULFILLMENT-EXTERNAL-USERS",
		"DEV-283-PORTAL-ACCOUNT-HANDOFF",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}
