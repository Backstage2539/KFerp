package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerPortalOrderBeanListUXRequirementSeeds(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatalf("read req_store.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"PR-CUSTOMER-PORTAL-ORDER-BEANLIST-UX",
		"DEV-CUSTOMER-PORTAL-ORDER-BEANLIST-UX-01",
		"DEV-CUSTOMER-PORTAL-ORDER-BEANLIST-UX-02",
		"DEV-CUSTOMER-PORTAL-ORDER-BEANLIST-UX-03",
		"UT-CUSTOMER-PORTAL-ORDER-BEANLIST-UX-01",
		"API-CUSTOMER-PORTAL-ORDER-BEANLIST-UX-01",
		"REV-CUSTOMER-PORTAL-ORDER-BEANLIST-UX-01",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("customer portal order/beanlist UX seed missing %q", want)
		}
	}
}

func TestCustomerPortalSettingsInlineListAndMiniappBeanListSource(t *testing.T) {
	viewRoot := filepath.Join("frontend-vue-shell", "src")
	settingsView, err := os.ReadFile(filepath.Join(viewRoot, "views", "CustomerPortalSettingsView.vue"))
	if err != nil {
		t.Fatalf("read CustomerPortalSettingsView.vue: %v", err)
	}
	settingsSrc := string(settingsView)
	for _, want := range []string{
		"portalRows",
		"saveVisibility(row)",
		"capability-grid",
		"binding-list",
	} {
		if !strings.Contains(settingsSrc, want) {
			t.Fatalf("CustomerPortalSettingsView inline list config missing %q", want)
		}
	}
	if strings.Contains(settingsSrc, `v-if="detail"`) {
		t.Fatal("CustomerPortalSettingsView should configure portal capabilities inside the customer list, not a bottom detail panel")
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
	for _, want := range []string{"bean-list-items", "bean-list-product", "item.groups", "orders"} {
		if !strings.Contains(serviceSrc, want) {
			t.Fatalf("miniapp bean list/order UX source missing %q", want)
		}
	}
}
