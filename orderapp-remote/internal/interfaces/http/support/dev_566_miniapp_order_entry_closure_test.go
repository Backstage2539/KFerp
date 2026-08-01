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
	// The first server-side Go gate runs from the complete repository and owns
	// this root release-script contract. The Docker safety gate intentionally
	// receives only orderapp-remote as its build context, so the root script is
	// absent there and was already checked before the image build started.
	orderappRoot := findAncestorForTest(t, "go.mod")
	workspaceRoot := filepath.Dir(orderappRoot)
	if _, err := os.Stat(filepath.Join(workspaceRoot, "deploy_orderapp.sh")); err == nil {
		releaseBytes, err := os.ReadFile(filepath.Join(workspaceRoot, "scripts", "remote_orderapp_release.sh"))
		if err != nil {
			t.Fatal(err)
		}
		releaseScript := string(releaseBytes)
		for _, marker := range []string{
			"wait_for_orderapp_http 60 3",
			"wait_for_public_http 15",
			"docker logs --tail 200",
			"development:https://dev.qacoohee.com/app",
			"production:https://erp.qacoohee.com/app",
			"--resolve dev.qacoohee.com:443:127.0.0.1",
		} {
			if !strings.Contains(releaseScript, marker) {
				t.Fatalf("remote release script missing readiness retry %q", marker)
			}
		}
		if strings.Contains(releaseScript, "STABLE_CHECKS") {
			t.Fatal("remote release script must not accept Running=true as HTTP readiness")
		}
		deployBytes, err := os.ReadFile(filepath.Join(workspaceRoot, "deploy_orderapp.sh"))
		if err != nil {
			t.Fatal(err)
		}
		deployScript := string(deployBytes)
		for _, marker := range []string{
			"external_smoke()",
			"KFERP_DEVELOPMENT_PUBLIC_IP",
			"--resolve \"dev.qacoohee.com:443:",
			"stop before promoting the next environment",
		} {
			if !strings.Contains(deployScript, marker) {
				t.Fatalf("deploy entrypoint missing external smoke marker %q", marker)
			}
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
