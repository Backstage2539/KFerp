package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDevCustomerPortalWorkbenchShortcut(t *testing.T) {
	api := readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customerfulfillment", "api.go"))
	for _, want := range []string{
		"internalCustomerPortalOverview",
		"internalCustomerPortalOptions",
	} {
		if !strings.Contains(string(api), want) {
			t.Fatalf("api.go missing %q", want)
		}
	}

	module := readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customerfulfillment", "module.go"))
	for _, want := range []string{
		"InternalCustomerPortalOverview",
		"InternalCustomerPortalOptions",
		"/api/customer-processing/internal/:customer_id/overview",
		"/api/customer-processing/internal/:customer_id/options",
	} {
		if !strings.Contains(string(module), want) {
			t.Fatalf("module.go missing %q", want)
		}
	}

	repo := readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go"))
	for _, want := range []string{
		"InternalCustomerPortalOverview",
		"requirePortalCustomerWithWorkbench",
		"resolveCustomerName",
		"buildOverview",
	} {
		if !strings.Contains(string(repo), want) {
			t.Fatalf("repository.go missing %q", want)
		}
	}

	svc := readOrderAppFileForTest(t, filepath.Join("internal", "application", "customerfulfillment", "service.go"))
	for _, want := range []string{
		"InternalCustomerPortalOverview",
		"InternalCustomerPortalOptions",
	} {
		if !strings.Contains(string(svc), want) {
			t.Fatalf("service.go missing %q", want)
		}
	}

	cps := readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CustomerPortalSettingsView.vue"))
	for _, want := range []string{
		"打开客户履约工作台",
		"key: 'customerProcessingPortal'",
	} {
		if !strings.Contains(string(cps), want) {
			t.Fatalf("CustomerPortalSettingsView.vue missing %q", want)
		}
	}

	cpv := readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CustomerProcessingPortalView.vue"))
	for _, want := range []string{
		"internalCustomerID",
		"isInternalContext",
		"fetchInternalCustomerProcessingPortalOverview",
		"fetchInternalCustomerProcessingPortalOptions",
	} {
		if !strings.Contains(string(cpv), want) {
			t.Fatalf("CustomerProcessingPortalView.vue missing %q", want)
		}
	}

	js := readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "api", "customer-fulfillment.js"))
	for _, want := range []string{
		"fetchInternalCustomerProcessingPortalOverview",
		"fetchInternalCustomerProcessingPortalOptions",
	} {
		if !strings.Contains(string(js), want) {
			t.Fatalf("customer-fulfillment.js missing %q", want)
		}
	}

	menu := readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js"))
	for _, want := range []string{
		"key: 'customerProcessingPortal', label: '客户履约工作台'",
	} {
		if !strings.Contains(string(menu), want) {
			t.Fatalf("menu-ia.js missing %q", want)
		}
	}

	wh := readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "WarehouseInventoryView.vue"))
	for _, want := range []string{
		"CUSTOMER_WORKSPACE_MODE",
		"isCustomerInventoryContext",
		"url.searchParams.set('customer_id'",
		"绑定客户后，只有该客户可查看此外部库存。",
		"kindLabel(row.kind)",
	} {
		if !strings.Contains(string(wh), want) {
			t.Fatalf("WarehouseInventoryView.vue missing %q", want)
		}
	}
	for _, removed := range []string{"warehouseSections", "customerWarehouses", "generalWarehouses", "普通仓库", "客户仓库"} {
		if strings.Contains(string(wh), removed) {
			t.Fatalf("WarehouseInventoryView.vue should not restore fixed warehouse section %q", removed)
		}
	}
}
