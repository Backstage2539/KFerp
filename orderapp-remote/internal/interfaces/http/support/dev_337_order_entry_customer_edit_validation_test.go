package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev337OrderEntryCustomerEditValidationSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-337-ORDER-ENTRY-CUSTOMER-EDIT-VALIDATION",
		"DEV-337-ORDER-ENTRY-CUSTOMER-EDIT-VALIDATION",
		"UT-337-ORDER-ENTRY-CUSTOMER-EDIT-VALIDATION",
		"API-337-ORDER-ENTRY-CUSTOMER-EDIT-VALIDATION",
		"REV-337-ORDER-ENTRY-CUSTOMER-EDIT-VALIDATION",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PR-337 requirement seed missing %q", want)
		}
	}
}

func TestDev337OrderEntryCustomerEditValidationWiring(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue")))
	for _, want := range []string{
		"openCustomerEditDrawer",
		"customerDrawerMode",
		"编辑客户",
		"`/api/customers/${form.customer_id}`",
		"method: customerDrawerMode.value === 'edit' ? 'PUT' : 'POST'",
		"fieldErrors = reactive({})",
		"raiseSaveError",
		"data-error-field=\"customer_id\"",
		"data-error-field=\"payment_method\"",
		"data-error-field=\"product_items\"",
		"scrollIntoView",
		"field-invalid",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("OrderEntryView.vue missing PR-337 marker %q", want)
		}
	}
}

func TestDev337OrderEntryCustomerEditValidationDocs(t *testing.T) {
	for _, rel := range []string{
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "acceptance", "2026-05-23-order-entry-customer-edit-validation.md"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"PR-337-ORDER-ENTRY-CUSTOMER-EDIT-VALIDATION",
			"编辑客户",
			"标红",
		} {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-337 documentation marker %q", rel, want)
			}
		}
	}
}
