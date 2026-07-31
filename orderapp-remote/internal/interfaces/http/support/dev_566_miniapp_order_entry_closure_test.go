package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev566MiniappOrderEntryClosureContracts(t *testing.T) {
	files := map[string]string{
		"reqStore":      filepath.Join("internal", "interfaces", "http", "support", "req_store.go"),
		"familyBuilder": filepath.Join("internal", "application", "sales", "product_families.go"),
		"miniAPI":       filepath.Join("internal", "interfaces", "http", "customerportal", "mini_employee_api.go"),
		"webOrder":      filepath.Join("frontend-vue-shell", "src", "lib", "order-entry.js"),
		"requirements":  filepath.Join("docs", "REQUIREMENTS.md"),
		"acceptance":    filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		"manual":        filepath.Join("docs", "OP_MANUAL_MINIAPP_EMPLOYEE_ERP.md"),
		"evidence":      filepath.Join("docs", "acceptance", "2026-07-31-miniapp-order-entry-closure.md"),
	}
	contents := map[string]string{}
	for key, rel := range files {
		contents[key] = string(readOrderAppFileForTest(t, rel))
	}

	for _, marker := range []string{
		"PR-566-MINIAPP-ORDER-ENTRY-CLOSURE",
		"DEV-566-SHARED-PRODUCT-FAMILIES",
		"DEV-566-MINIAPP-SEARCH-RECEIVER",
		"DEV-566-MINIAPP-AUTH-COMPAT",
		"DEV-566-REMOTE-BUILD-RELEASE",
		"UT-566-MINIAPP-ORDER-ENTRY-CLOSURE",
		"API-566-MINIAPP-ORDER-ENTRY-CLOSURE",
		"REV-566-MINIAPP-ORDER-ENTRY-CLOSURE",
	} {
		if !strings.Contains(contents["reqStore"], marker) {
			t.Fatalf("req_store missing %s", marker)
		}
	}

	for _, marker := range []string{
		"BuildOrderProductFamilies",
		"customer_product_alias_id",
		"default_sku_id",
		"specs",
	} {
		if !strings.Contains(contents["familyBuilder"], marker) {
			t.Fatalf("shared family builder missing %q", marker)
		}
	}
	for _, marker := range []string{
		`"products": form.Products`,
		`"product_families": families`,
		"requireMiniEmployee",
		"miniEmployeeAuthError",
	} {
		if !strings.Contains(contents["miniAPI"], marker) {
			t.Fatalf("mini employee API missing %q", marker)
		}
	}
	for _, marker := range []string{
		"normalizeOrderProductFamilies",
		"orderProductFamilyIdentity",
		"isOrderProductFamily",
	} {
		if !strings.Contains(contents["webOrder"], marker) {
			t.Fatalf("ERP order entry missing %q", marker)
		}
	}

	for _, key := range []string{"requirements", "acceptance", "manual", "evidence"} {
		if !strings.Contains(contents[key], "PR-566") {
			t.Fatalf("%s missing PR-566 marker", key)
		}
	}

	miniappRoot := filepath.Join(findAncestorForTest(t, "go.mod"), "..", "miniapp")
	if _, err := os.Stat(miniappRoot); os.IsNotExist(err) {
		miniappRoot = filepath.Join(findAncestorForTest(t, "go.mod"), "miniapp")
	}
	miniPage, err := os.ReadFile(filepath.Join(miniappRoot, "src", "pages", "employee-order-entry", "employee-order-entry.vue"))
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{
		"订单日期",
		"搜索客户名称 / 拼音 / 首字母",
		"商品 / 别名 / 拼音 / 编码 / 规格",
		"选择客户后自动带入",
		"重新登录",
		"重试",
	} {
		if !strings.Contains(string(miniPage), marker) {
			t.Fatalf("miniapp order entry missing %q", marker)
		}
	}
}
