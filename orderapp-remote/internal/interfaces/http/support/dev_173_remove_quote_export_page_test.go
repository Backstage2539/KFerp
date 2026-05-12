package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev173RemoveQuoteExportPageSourceGuard(t *testing.T) {
	checks := []struct {
		path      string
		forbidden []string
	}{
		{
			path: filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js"),
			forbidden: []string{
				"quotePrint",
				"报价导出",
			},
		},
		{
			path: filepath.Join("frontend-vue-shell", "src", "App.vue"),
			forbidden: []string{
				"ProductsView",
				"quotePrint",
			},
		},
		{
			path: filepath.Join("internal", "interfaces", "http", "catalog", "product_routes.go"),
			forbidden: []string{
				`"/products/print"`,
				"quotePrint",
				"func (h productHandler) print",
			},
		},
		{
			path: filepath.Join("internal", "infrastructure", "postgres", "authz", "schema.go"),
			forbidden: []string{
				`"quotePrint"`,
			},
		},
		{
			path: filepath.Join("templates", "docs.html"),
			forbidden: []string{
				"/products/print",
				"报价导出",
			},
		},
		{
			path: filepath.Join("templates", "doc_view.html"),
			forbidden: []string{
				"/products/print",
				"报价导出",
			},
		},
		{
			path: filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
			forbidden: []string{
				"报价导出",
			},
		},
		{
			path: filepath.Join("docs", "OPERATION_MANUALS.md"),
			forbidden: []string{
				"报价导出",
			},
		},
	}
	for _, tc := range checks {
		body, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", tc.path, err)
		}
		src := string(body)
		for _, forbidden := range tc.forbidden {
			if strings.Contains(src, forbidden) {
				t.Fatalf("%s should not contain removed quote export page marker %q", tc.path, forbidden)
			}
		}
	}

	if _, err := os.Stat(filepath.Join("frontend-vue-shell", "src", "views", "ProductsView.vue")); !os.IsNotExist(err) {
		t.Fatalf("ProductsView.vue should be removed with the quote export page, stat err=%v", err)
	}
}

func TestDev173RequirementSeeds(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	for _, want := range []string{
		"PR-173",
		"DEV-173-01",
		"UT-173-01",
		"API-173-01",
		"REV-173-01",
		"删除订单销售的报价导出页面",
		"移除 quotePrint 菜单、路由和权限",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("DEV-173 requirement seed missing %q", want)
		}
	}
}
