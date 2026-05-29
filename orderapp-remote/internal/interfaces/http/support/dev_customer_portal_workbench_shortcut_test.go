package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDevCustomerPortalWorkbenchShortcut(t *testing.T) {
	api, err := readOrderAppFile(filepath.Join("internal", "interfaces", "http", "customerfulfillment", "api.go"))
	if err != nil {
		t.Fatal(err)
	}
	apiText := string(api)
	for _, want := range []string{
		"internalCustomerPortalOverview",
		"internalCustomerPortalOptions",
		"parseID(c.Param(\"customer_id\"), \"customer\")",
	} {
		if !strings.Contains(apiText, want) {
			t.Fatalf("api.go missing %q", want)
		}
	}

	module, err := readOrderAppFile(filepath.Join("internal", "interfaces", "http", "customerfulfillment", "module.go"))
	if err != nil {
		t.Fatal(err)
	}
	moduleText := string(module)
	for _, want := range []string{
		"InternalCustomerPortalOverview",
		"InternalCustomerPortalOptions",
		"/api/customer-processing/internal/:customer_id/overview",
		"/api/customer-processing/internal/:customer_id/options",
	} {
		if !strings.Contains(moduleText, want) {
			t.Fatalf("module.go missing %q", want)
		}
	}

	repo, err := readOrderAppFile(filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go"))
	if err != nil {
		t.Fatal(err)
	}
	repoText := string(repo)
	for _, want := range []string{
		"InternalCustomerPortalOverview",
		"requirePortalCustomerWithWorkbench",
		"buildOverview",
	} {
		if !strings.Contains(repoText, want) {
			t.Fatalf("repository.go missing %q", want)
		}
	}

	svc, err := readOrderAppFile(filepath.Join("internal", "application", "customerfulfillment", "service.go"))
	if err != nil {
		t.Fatal(err)
	}
	svcText := string(svc)
	for _, want := range []string{
		"InternalCustomerPortalOverview",
		"InternalCustomerPortalOptions",
	} {
		if !strings.Contains(svcText, want) {
			t.Fatalf("service.go missing %q", want)
		}
	}

	portalView, err := readOrderAppFile(filepath.Join("frontend-vue-shell", "src", "views", "CustomerPortalSettingsView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	portalText := string(portalView)
	for _, want := range []string{
		"打开客户履约工作台",
		"key: 'customerProcessingPortal'",
		"params: { customer_id: customerID }",
	} {
		if !strings.Contains(portalText, want) {
			t.Fatalf("CustomerPortalSettingsView.vue missing %q", want)
		}
	}

	processingView, err := readOrderAppFile(filepath.Join("frontend-vue-shell", "src", "views", "CustomerProcessingPortalView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	processingText := string(processingView)
	for _, want := range []string{
		"internalCustomerID",
		"isInternalContext",
		"fetchInternalCustomerProcessingPortalOverview",
		"fetchInternalCustomerProcessingPortalOptions",
	} {
		if !strings.Contains(processingText, want) {
			t.Fatalf("CustomerProcessingPortalView.vue missing %q", want)
		}
	}

	apiJS, err := readOrderAppFile(filepath.Join("frontend-vue-shell", "src", "api", "customer-fulfillment.js"))
	if err != nil {
		t.Fatal(err)
	}
	apiJSText := string(apiJS)
	for _, want := range []string{
		"fetchInternalCustomerProcessingPortalOverview",
		"fetchInternalCustomerProcessingPortalOptions",
	} {
		if !strings.Contains(apiJSText, want) {
			t.Fatalf("customer-fulfillment.js missing %q", want)
		}
	}
}

func readOrderAppFile(path string) ([]byte, error) {
	return readFileForTest(t, filepath.Join("..", "..", "..", "..", path))
}
