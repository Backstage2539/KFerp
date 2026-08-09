package support

import (
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDev591MiniPullBrandSelfLoginGuardContracts(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, row := range []struct {
		table    string
		code     string
		status   string
		assignee string
	}{
		{table: "req_product", code: "PR-591-MINI-PULL-BRAND-SELF-LOGIN-GUARD", status: "doing", assignee: "Codex"},
		{table: "req_dev", code: "DEV-591-MINI-PULL-UP-BRAND", status: "done", assignee: "Codex"},
		{table: "req_dev", code: "DEV-591-SELF-LOGIN-DISABLE-GUARD", status: "done", assignee: "Codex"},
		{table: "req_dev", code: "DEV-591-DOCS-DEVELOPMENT-DELIVERY", status: "doing", assignee: "Codex"},
		{table: "req_review", code: "REV-591-MINI-PULL-BRAND-SELF-LOGIN-GUARD", status: "todo", assignee: "VA"},
	} {
		requireDev591SeedRow(t, reqStore, row.table, row.code, row.status, row.assignee)
	}

	orderAppRoot := findAncestorForTest(t, "go.mod")
	repoRoot := filepath.Dir(orderAppRoot)
	miniappRoot := filepath.Join(repoRoot, "miniapp", "src")
	componentPath := filepath.Join(miniappRoot, "components", "PullUpBrandFooter.vue")
	componentBytes, err := os.ReadFile(componentPath)
	if err != nil {
		t.Fatalf("read pull-up brand footer: %v", err)
	}
	component := string(componentBytes)
	for _, want := range []string{"Drived By", "/static/branding/kefan-wordmark-silver.png"} {
		if !strings.Contains(component, want) {
			t.Fatalf("PullUpBrandFooter.vue missing %q", want)
		}
	}
	compactComponent := strings.ReplaceAll(strings.ReplaceAll(component, " ", ""), "\t", "")
	if strings.Contains(compactComponent, "position:fixed") {
		t.Fatal("pull-up brand footer must remain in document flow, not use position: fixed")
	}

	for _, page := range []string{
		"login/login.vue",
		"home/home.vue",
		"mall/mall.vue",
		"service/service.vue",
		"customer-inventory-detail/customer-inventory-detail.vue",
		"factory-products/factory-products.vue",
		"customer-products/customer-products.vue",
		"price-table-settings/price-table-settings.vue",
		"profile/profile.vue",
		"employee-order-entry/employee-order-entry.vue",
		"employee-orders/employee-orders.vue",
		"employee-order-detail/employee-order-detail.vue",
		"employee-customers/employee-customers.vue",
	} {
		src, err := os.ReadFile(filepath.Join(miniappRoot, "pages", page))
		if err != nil {
			t.Fatalf("read miniapp page %s: %v", page, err)
		}
		if !strings.Contains(string(src), "<PullUpBrandFooter") {
			t.Fatalf("miniapp page %s must mount PullUpBrandFooter", page)
		}
	}
	indexSrc, err := os.ReadFile(filepath.Join(miniappRoot, "pages", "index", "index.vue"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(indexSrc), "<PullUpBrandFooter") {
		t.Fatal("transient routing page pages/index/index must not mount the brand footer")
	}

	assetPath := filepath.Join(miniappRoot, "static", "branding", "kefan-wordmark-silver.png")
	assetInfo, err := os.Stat(assetPath)
	if err != nil {
		t.Fatalf("stat compressed wordmark: %v", err)
	}
	if assetInfo.Size() > 64*1024 {
		t.Fatalf("compressed wordmark is %d bytes, want <= 65536", assetInfo.Size())
	}
	asset, err := os.Open(assetPath)
	if err != nil {
		t.Fatal(err)
	}
	defer asset.Close()
	img, _, err := image.Decode(asset)
	if err != nil {
		t.Fatalf("decode wordmark PNG: %v", err)
	}
	if img.Bounds().Dx() <= img.Bounds().Dy() {
		t.Fatalf("wordmark must be a compact horizontal image, got %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
	hasTransparentPixel := false
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y && !hasTransparentPixel; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			_, _, _, alpha := img.At(x, y).RGBA()
			if alpha < 0xffff {
				hasTransparentPixel = true
				break
			}
		}
	}
	if !hasTransparentPixel {
		t.Fatal("wordmark PNG must have a transparent background")
	}

	mobileAuth := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "mobile_auth.go")))
	for _, want := range []string{"!req.LoginEnabled", "currentEmployeeID(c) == req.EmployeeID", "cannot disable current account"} {
		if !strings.Contains(mobileAuth, want) {
			t.Fatalf("mobile account-state guard missing %q", want)
		}
	}
	staffView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CompanyStaffView.vue")))
	for _, want := range []string{"fetchCurrentActor", "isCurrentAccount", `:disabled="isCurrentAccount(row.id)"`, "当前账号不能关闭自己的登录"} {
		if !strings.Contains(staffView, want) {
			t.Fatalf("CompanyStaffView self-disable guard missing %q", want)
		}
	}

	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-591-MINI-PULL-BRAND-SELF-LOGIN-GUARD",
			"DEV-591-MINI-PULL-UP-BRAND",
			"DEV-591-SELF-LOGIN-DISABLE-GUARD",
			"DEV-591-DOCS-DEVELOPMENT-DELIVERY",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-591-MINI-PULL-BRAND-SELF-LOGIN-GUARD",
			"13 个实际业务页面",
			"不能关闭当前登录账号自身",
		},
		filepath.Join("docs", "OP_MANUAL_MINIAPP_EMPLOYEE_ERP.md"): {
			"Drived By",
			"继续向上拉",
			"透明银灰",
		},
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"): {
			"Drived By",
			"继续向上拉",
		},
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md"): {
			"Drived By",
			"继续向上拉",
			"PR-591-MINI-PULL-BRAND-SELF-LOGIN-GUARD",
		},
		filepath.Join("docs", "OP_MANUAL_SETTINGS_AUDIT.md"): {
			"当前登录账号不能停用自身",
			"cannot disable current account",
			"其他管理员",
		},
		filepath.Join("docs", "acceptance", "2026-08-10-mini-pull-brand-self-login-guard.md"): {
			"PR-591 小程序上拉品牌标识与当前账号防自停用验收记录",
			"生产即时恢复",
			"现有账号启停 API",
			"未记录账号、密码或其他个人信息",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-591 marker %q", rel, want)
			}
		}
		for _, forbiddenCredentialField := range []string{"用户名：", "密码："} {
			if strings.Contains(src, forbiddenCredentialField) {
				t.Fatalf("%s must not record a production credential field %q", rel, forbiddenCredentialField)
			}
		}
	}

	for rel, wants := range map[string][]string{
		"REQUIREMENTS.md":        {"## 55.", "PR-591-MINI-PULL-BRAND-SELF-LOGIN-GUARD"},
		"ACCEPTANCE_TESTS.md":    {"### K55.", "PR-591-MINI-PULL-BRAND-SELF-LOGIN-GUARD"},
		"ACTIVE_REQUIREMENTS.md": {"PR-591-MINI-PULL-BRAND-SELF-LOGIN-GUARD", "Status: doing", "REV-591-MINI-PULL-BRAND-SELF-LOGIN-GUARD"},
	} {
		src, err := os.ReadFile(filepath.Join(repoRoot, rel))
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range wants {
			if !strings.Contains(string(src), want) {
				t.Fatalf("root %s missing PR-591 marker %q", rel, want)
			}
		}
	}
	for _, removedCopy := range []string{"OPERATION_MANUALS.md", "OP_MANUAL_MINIAPP_EMPLOYEE_ERP.md", "OP_MANUAL_CUSTOMER_PORTAL.md", "OP_MANUAL_SETTINGS_AUDIT.md"} {
		if _, err := os.Stat(filepath.Join(repoRoot, removedCopy)); !os.IsNotExist(err) {
			t.Fatalf("root manual copy %s must remain absent", removedCopy)
		}
	}
}

func requireDev591SeedRow(t *testing.T, src, table, code, status, assignee string) {
	t.Helper()
	pattern := regexp.MustCompile(`(?m)^[\t ]*\{table: "` + regexp.QuoteMeta(table) + `"[^\n]*code: "` + regexp.QuoteMeta(code) + `"[^\n]*status: "` + regexp.QuoteMeta(status) + `"[^\n]*assignee: "` + regexp.QuoteMeta(assignee) + `"[^\n]*\},[\t ]*$`)
	if !pattern.MatchString(src) {
		t.Fatalf("req_store.go missing one-line %s seed %s with status %s and assignee %s", table, code, status, assignee)
	}
}
