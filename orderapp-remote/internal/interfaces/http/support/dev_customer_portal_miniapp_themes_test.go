package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerPortalMiniappThemeRequirementSeeds(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatalf("read req_store.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"PR-CUSTOMER-PORTAL-MINIAPP-THEMES",
		"DEV-CUSTOMER-PORTAL-MINIAPP-THEMES-01",
		"DEV-CUSTOMER-PORTAL-MINIAPP-THEMES-02",
		"DEV-CUSTOMER-PORTAL-MINIAPP-THEMES-03",
		"UT-CUSTOMER-PORTAL-MINIAPP-THEMES-01",
		"API-CUSTOMER-PORTAL-MINIAPP-THEMES-01",
		"REV-CUSTOMER-PORTAL-MINIAPP-THEMES-01",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("customer portal miniapp theme seed missing %q", want)
		}
	}
}

func TestCustomerPortalMiniappThemeSourceWiring(t *testing.T) {
	miniRoot := filepath.Join("..", "miniapp", "src")
	servicePath := filepath.Join(miniRoot, "pages", "service", "service.vue")
	homePath := filepath.Join(miniRoot, "pages", "home", "home.vue")
	themePath := filepath.Join(miniRoot, "utils", "themes.ts")
	for _, path := range []string{servicePath, homePath, themePath} {
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				t.Skip("miniapp source is not present in the orderapp-only Docker build context")
			}
			t.Fatalf("stat %s: %v", path, err)
		}
	}
	body, err := os.ReadFile(themePath)
	if err != nil {
		t.Fatalf("read %s: %v", themePath, err)
	}
	themeText := string(body)
	for _, want := range []string{"coffee_factory", "clean_ops", "premium_partner", "theme-coffee-factory", "theme-clean-ops", "theme-premium-partner"} {
		if !strings.Contains(themeText, want) {
			t.Fatalf("%s missing %q", themePath, want)
		}
	}

	for _, file := range []string{servicePath, homePath} {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		text := string(body)
		for _, want := range []string{"miniappThemeClass", "theme-coffee-factory", "theme-clean-ops", "theme-premium-partner"} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing %q", file, want)
			}
		}
	}
}
