package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev311CustomerPortalAccountProfile(t *testing.T) {
	appVue := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "App.vue")))
	settingsView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CustomerPortalSettingsView.vue")))
	workspaceMode := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "workspace-mode.js")))
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	customerPortalManual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md")))
	customerFulfillmentManual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md")))
	requirements := string(readOrderAppFileForTest(t, filepath.Join("docs", "REQUIREMENTS.md")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("docs", "ACCEPTANCE_TESTS.md")))

	for _, want := range []string{
		"WORKSPACE_CUSTOMERS_REFRESH_EVENT",
		"workspaceCustomersRefreshEventName",
		"handleWorkspaceCustomersRefresh",
	} {
		if !strings.Contains(appVue, want) {
			t.Fatalf("App.vue missing workspace customer refresh marker %q", want)
		}
	}

	for _, want := range []string{
		"打开客户档案",
		"customerDossierNavigationDetail(row)",
		"refreshWorkspaceCustomers()",
		"workspaceCustomersRefreshEvent",
	} {
		if !strings.Contains(settingsView, want) {
			t.Fatalf("CustomerPortalSettingsView.vue missing portal account/profile marker %q", want)
		}
	}
	if !strings.Contains(workspaceMode, "kferp:workspace-customers-refresh") {
		t.Fatal("workspace-mode.js must define stable workspace customer refresh event")
	}
	if strings.Contains(settingsView, "openCustomerProcessingPortal") {
		t.Fatal("CustomerPortalSettingsView.vue must not keep stale customer processing portal jump helper")
	}

	for _, doc := range []string{reqStore, requirements, acceptance, customerPortalManual, customerFulfillmentManual} {
		if !strings.Contains(doc, "PR-311-CUSTOMER-PORTAL-ACCOUNT-PROFILE") {
			t.Fatal("PR-311 marker missing from requirement, acceptance, seed, or manual docs")
		}
	}
}
