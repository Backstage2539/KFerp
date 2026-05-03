package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerPortalVisibilityAndOrderDetailRequirementSeeds(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatalf("read req_store.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"PR-CUSTOMER-PORTAL-VISIBILITY-ORDERS",
		"DEV-CUSTOMER-PORTAL-VISIBILITY-ORDERS-01",
		"DEV-CUSTOMER-PORTAL-VISIBILITY-ORDERS-02",
		"DEV-CUSTOMER-PORTAL-VISIBILITY-ORDERS-03",
		"UT-CUSTOMER-PORTAL-VISIBILITY-ORDERS-01",
		"API-CUSTOMER-PORTAL-VISIBILITY-ORDERS-01",
		"REV-CUSTOMER-PORTAL-VISIBILITY-ORDERS-01",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("customer portal visibility/order seed missing %q", want)
		}
	}
}

func TestCustomerPortalSettingsViewAndMiniappOrderDetailSource(t *testing.T) {
	viewRoot := filepath.Join("frontend-vue-shell", "src")
	settingsView, err := os.ReadFile(filepath.Join(viewRoot, "views", "CustomerPortalSettingsView.vue"))
	if err != nil {
		t.Fatalf("read CustomerPortalSettingsView.vue: %v", err)
	}
	appVue, err := os.ReadFile(filepath.Join(viewRoot, "App.vue"))
	if err != nil {
		t.Fatalf("read App.vue: %v", err)
	}
	menu, err := os.ReadFile(filepath.Join(viewRoot, "lib", "menu-ia.js"))
	if err != nil {
		t.Fatalf("read menu-ia.js: %v", err)
	}
	settingsSrc := string(settingsView)
	for _, want := range []string{
		"/api/customer-portal/admin/customers",
		"capabilities",
		"bindings",
		"我的豆单",
		"一件代发",
	} {
		if !strings.Contains(settingsSrc, want) {
			t.Fatalf("CustomerPortalSettingsView missing %q", want)
		}
	}
	if !strings.Contains(string(appVue), "CustomerPortalSettingsView") || !strings.Contains(string(menu), "customerPortalSettings") {
		t.Fatal("Vue shell must register customerPortalSettings menu/view")
	}

	miniRoot := filepath.Join("..", "miniapp", "src")
	servicePath := filepath.Join(miniRoot, "pages", "service", "service.vue")
	if _, err := os.Stat(servicePath); err != nil {
		if os.IsNotExist(err) {
			t.Skip("miniapp source is not present in the orderapp-only Docker build context")
		}
		t.Fatalf("stat miniapp service page: %v", err)
	}
	servicePage, err := os.ReadFile(servicePath)
	if err != nil {
		t.Fatalf("read miniapp service page: %v", err)
	}
	serviceSrc := string(servicePage)
	for _, want := range []string{"item.items", "grand_total", "unit_price", "line_total", "order_date"} {
		if !strings.Contains(serviceSrc, want) {
			t.Fatalf("miniapp order detail source missing %q", want)
		}
	}
}
