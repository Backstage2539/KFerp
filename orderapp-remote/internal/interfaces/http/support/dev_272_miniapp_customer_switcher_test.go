package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMiniappCustomerSwitcherRequirementSeedsExist(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatalf("read req_store.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"PR-272-MINIAPP-CUSTOMER-SWITCHER",
		"DEV-272-MINIAPP-CUSTOMER-SWITCHER",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("customer switcher seed missing %q", want)
		}
	}
}

func TestMiniappCustomerSwitcherSourceWiring(t *testing.T) {
	miniRoot := filepath.Join("..", "miniapp", "src")
	if _, err := os.Stat(miniRoot); err != nil {
		if os.IsNotExist(err) {
			t.Skip("miniapp source is not present in the orderapp-only Docker build context")
		}
		t.Fatalf("stat miniapp source: %v", err)
	}
	for _, path := range []string{
		filepath.Join(miniRoot, "pages", "home", "home.vue"),
		filepath.Join(miniRoot, "pages", "mall", "mall.vue"),
		filepath.Join(miniRoot, "pages", "service", "service.vue"),
	} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(body)
		for _, want := range []string{"MainTabBar"} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing bottom account entry marker %q", path, want)
			}
		}
		for _, gone := range []string{"openProfile"} {
			if strings.Contains(text, gone) {
				t.Fatalf("%s still contains old top profile action marker %q", path, gone)
			}
		}
	}
	profile, err := os.ReadFile(filepath.Join(miniRoot, "pages", "profile", "profile.vue"))
	if err != nil {
		t.Fatalf("read profile.vue: %v", err)
	}
	for _, want := range []string{"switchCurrentCustomer", "customerPickerLabels", "handleCustomerSwitch", "切换用户", "退出登录"} {
		if !strings.Contains(string(profile), want) {
			t.Fatalf("profile.vue missing customer switcher marker %q", want)
		}
	}
	api, err := os.ReadFile(filepath.Join(miniRoot, "api", "customerPortal.ts"))
	if err != nil {
		t.Fatalf("read customerPortal.ts: %v", err)
	}
	for _, want := range []string{"buildSwitchCustomerPath", "/api/mini/current-customer", "switchCurrentCustomer"} {
		if !strings.Contains(string(api), want) {
			t.Fatalf("customerPortal.ts missing %q", want)
		}
	}
}

func TestMiniappCustomerSwitcherDocsExist(t *testing.T) {
	manuals := []string{
		filepath.Join("..", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
	}
	for _, path := range manuals {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(body)
		for _, want := range []string{"我的", "切换用户", "13800138075"} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing customer switcher manual marker %q", path, want)
			}
		}
	}
	for _, path := range []string{
		filepath.Join("..", "REQUIREMENTS.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("..", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(body)
		for _, want := range []string{"我的", "/api/mini/current-customer"} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing customer switcher doc marker %q", path, want)
			}
		}
	}
}
