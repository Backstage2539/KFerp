package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev277MiniappPasswordSkuAliasRecords(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-277-MINIAPP-PASSWORD-SKU-ALIAS",
		"DEV-277-MINIAPP-PASSWORD-SKU-ALIAS",
		"UT-277-MINIAPP-PASSWORD-SKU-ALIAS",
		"API-277-MINIAPP-PASSWORD-SKU-ALIAS",
		"REV-277-MINIAPP-PASSWORD-SKU-ALIAS",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %s", want)
		}
	}
}

func TestDev277MiniappPasswordSkuAliasDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "REQUIREMENTS.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("..", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		body := string(readOrderAppFileForTest(t, path))
		if !strings.Contains(body, "PR-277-MINIAPP-PASSWORD-SKU-ALIAS") {
			t.Fatalf("%s missing PR-277-MINIAPP-PASSWORD-SKU-ALIAS", path)
		}
	}

	for _, path := range []string{
		filepath.Join("..", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
	} {
		body := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"密码",
			"掩码",
			"定制 SKU",
			"基础款",
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing %q", path, want)
			}
		}
	}
}

func TestDev277MiniappPasswordSkuAliasSourceGuards(t *testing.T) {
	login := string(readOrderAppFileForTest(t, filepath.Join("..", "miniapp", "src", "pages", "login", "login.vue")))
	for _, want := range []string{
		`password placeholder="密码"`,
		"loginWithPassword",
	} {
		if !strings.Contains(login, want) {
			t.Fatalf("login.vue missing %q", want)
		}
	}
	if strings.Contains(login, `type="text" placeholder="密码"`) {
		t.Fatalf("login.vue must not expose password as plain text input")
	}

	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerportal", "business_repository.go")))
	for _, want := range []string{
		"alias_products",
		"base_product_id",
		"customer_only",
		"portalProductVisibleToCustomerAliasSQL",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("business_repository.go missing %q", want)
		}
	}
}
