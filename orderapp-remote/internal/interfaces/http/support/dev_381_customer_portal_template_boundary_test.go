package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev381CustomerPortalTemplateBoundaryRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-381-CUSTOMER-PORTAL-TEMPLATE-BOUNDARY",
		"DEV-381-CUSTOMER-PROFILE-PORTAL-SWITCH-ONLY",
		"UT-381-CUSTOMER-PORTAL-TEMPLATE-BOUNDARY",
		"API-381-CUSTOMER-PORTAL-TEMPLATE-BOUNDARY",
		"REV-381-CUSTOMER-PORTAL-TEMPLATE-BOUNDARY",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PR-381 requirement seed missing %q", want)
		}
	}
}

func TestDev381CustomerProfileDoesNotBindCapabilityTemplate(t *testing.T) {
	customerService := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "customer", "service.go")))
	if strings.Contains(customerService, "请维护客户门户/工作台：能力模板") {
		t.Fatal("customer profile service should not require capability template when portal switch is enabled")
	}

	customersView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CustomersView.vue")))
	for _, forbidden := range []string{
		`v-model="form.capability_template_key"`,
		"defaultCapabilityTemplateForCustomerType",
		"请选择能力模板",
	} {
		if strings.Contains(customersView, forbidden) {
			t.Fatalf("CustomersView.vue should not bind templates in customer profile: found %q", forbidden)
		}
	}
	if !strings.Contains(customersView, "开通客户门户/工作台") || !strings.Contains(customersView, "portal_enabled") {
		t.Fatal("CustomersView.vue should retain the portal/workbench switch")
	}

	orderEntryView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue")))
	for _, forbidden := range []string{
		`v-model="customerForm.capability_template_key"`,
		"defaultCapabilityTemplateForCustomerType",
		"请选择能力模板",
	} {
		if strings.Contains(orderEntryView, forbidden) {
			t.Fatalf("OrderEntryView.vue should not bind templates in customer drawer: found %q", forbidden)
		}
	}
	if !strings.Contains(orderEntryView, "开通客户门户/工作台") || !strings.Contains(orderEntryView, "portal_enabled") {
		t.Fatal("OrderEntryView.vue should retain the portal/workbench switch")
	}

	portalSettingsView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CustomerPortalSettingsView.vue")))
	for _, want := range []string{
		"能力模板",
		`v-model="row.form.capability_template_key"`,
		"/api/customer-portal/admin/customers",
	} {
		if !strings.Contains(portalSettingsView, want) {
			t.Fatalf("CustomerPortalSettingsView.vue should remain the template binding surface, missing %q", want)
		}
	}
}

func TestDev381CustomerPortalTemplateBoundaryDocs(t *testing.T) {
	for _, rel := range []string{
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "acceptance", "2026-05-26-customer-portal-template-boundary.md"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"PR-381-CUSTOMER-PORTAL-TEMPLATE-BOUNDARY",
			"客户档案",
			"门户客户配置",
			"能力模板",
		} {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-381 documentation marker %q", rel, want)
			}
		}
	}
}
