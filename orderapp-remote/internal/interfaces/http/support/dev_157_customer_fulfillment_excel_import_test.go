package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerFulfillmentExcelImportRequirementSeeds(t *testing.T) {
	body := string(readDev157File(t, "internal/interfaces/http/support/req_store.go"))
	for _, code := range []string{
		"PR-157",
		"DEV-157-01",
		"DEV-157-02",
		"DEV-157-03",
		"DEV-157-04",
		"UT-157-01",
		"API-157-01",
		"REV-157-01",
	} {
		if !strings.Contains(body, code) {
			t.Fatalf("req_store.go missing %s", code)
		}
	}
}

func TestCustomerFulfillmentExcelImportArchitectureWiring(t *testing.T) {
	cases := []struct {
		path string
		want []string
	}{
		{
			path: filepath.Join("internal", "appmain", "app_routes.go"),
			want: []string{
				"postgrescustomerfulfillment",
				"customerfulfillmenthttp.RegisterRoutes",
			},
		},
		{
			path: filepath.Join("internal", "appmain", "schema_setup.go"),
			want: []string{
				"postgrescustomerfulfillment",
			},
		},
		{
			path: filepath.Join("internal", "interfaces", "http", "support", "authz_middleware.go"),
			want: []string{
				"/api/customer-fulfillment",
			},
		},
		{
			path: filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js"),
			want: []string{
				"customerFulfillment",
				"客户履约账户",
			},
		},
		{
			path: filepath.Join("frontend-vue-shell", "src", "App.vue"),
			want: []string{
				"CustomerFulfillmentView",
			},
		},
	}
	for _, tc := range cases {
		body := string(readDev157File(t, tc.path))
		for _, want := range tc.want {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing %q", tc.path, want)
			}
		}
	}
	if _, err := os.Stat(filepath.Join("templates", "customer_fulfillment.html")); err == nil {
		t.Fatal("customer fulfillment must be implemented in Vue/Vite, not templates/customer_fulfillment.html")
	}
}

func readDev157File(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
