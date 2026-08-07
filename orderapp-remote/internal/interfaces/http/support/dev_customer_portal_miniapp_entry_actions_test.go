package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerPortalMiniappEntryActionRequirementSeeds(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatalf("read req_store.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"PR-CUSTOMER-PORTAL-MINIAPP-ENTRY-ACTIONS",
		"DEV-CUSTOMER-PORTAL-MINIAPP-ENTRY-ACTIONS-01",
		"UT-CUSTOMER-PORTAL-MINIAPP-ENTRY-ACTIONS-01",
		"API-CUSTOMER-PORTAL-MINIAPP-ENTRY-ACTIONS-01",
		"REV-CUSTOMER-PORTAL-MINIAPP-ENTRY-ACTIONS-01",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("customer portal miniapp entry action seed missing %q", want)
		}
	}
}

func TestCustomerPortalMiniappEntriesAreTappable(t *testing.T) {
	root := filepath.Join("..", "miniapp", "src")
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			t.Skip("miniapp source is not present in the orderapp-only Docker build context")
		}
		t.Fatalf("stat miniapp source: %v", err)
	}
	home, err := os.ReadFile(filepath.Join(root, "pages", "home", "home.vue"))
	if err != nil {
		t.Fatalf("read home.vue: %v", err)
	}
	pages, err := os.ReadFile(filepath.Join(root, "pages.json"))
	if err != nil {
		t.Fatalf("read pages.json: %v", err)
	}
	service, err := os.ReadFile(filepath.Join(root, "pages", "service", "service.vue"))
	if err != nil {
		t.Fatalf("read service.vue: %v", err)
	}
	if !strings.Contains(string(home), "@tap=\"openEntry(entry.url)\"") {
		t.Fatal("home entries must navigate on tap")
	}
	if !strings.Contains(string(pages), "pages/service/service") {
		t.Fatal("service page must be registered")
	}
	for _, want := range []string{"fetchServicePage", "CustomerDirectShipPanel", "CustomerProcessingPanel"} {
		if !strings.Contains(string(service), want) {
			t.Fatalf("service page must connect real business API, missing %q", want)
		}
	}
}
