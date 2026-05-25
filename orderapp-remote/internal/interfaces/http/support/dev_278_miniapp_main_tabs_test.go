package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev278MiniappMainTabsRecords(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-278-MINIAPP-MAIN-TABS",
		"DEV-278-MINIAPP-MAIN-TABS",
		"UT-278-MINIAPP-MAIN-TABS",
		"API-278-MINIAPP-MAIN-TABS",
		"REV-278-MINIAPP-MAIN-TABS",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %s", want)
		}
	}
}

func TestDev278MiniappMainTabsSource(t *testing.T) {
	pages := string(readOrderAppFileForTest(t, filepath.Join("..", "miniapp", "src", "pages.json")))
	index := strings.Index(pages, `"path": "pages/index/index"`)
	login := strings.Index(pages, `"path": "pages/login/login"`)
	if index < 0 || login < 0 || index > login {
		t.Fatalf("pages/index/index must be the first miniapp route before login")
	}

	tabBar := string(readOrderAppFileForTest(t, filepath.Join("..", "miniapp", "src", "components", "MainTabBar.vue")))
	for _, want := range []string{
		"首页",
		"订单中心",
		"费用中心",
		"个人中心",
		"uni.reLaunch",
		"/pages/service/service?key=orders",
		"/pages/service/service?key=settlement",
	} {
		if !strings.Contains(tabBar, want) {
			t.Fatalf("MainTabBar.vue missing %q", want)
		}
	}

	for _, path := range []string{
		filepath.Join("..", "miniapp", "src", "pages", "home", "home.vue"),
		filepath.Join("..", "miniapp", "src", "pages", "mall", "mall.vue"),
		filepath.Join("..", "miniapp", "src", "pages", "service", "service.vue"),
		filepath.Join("..", "miniapp", "src", "pages", "profile", "profile.vue"),
	} {
		source := string(readOrderAppFileForTest(t, path))
		if !strings.Contains(source, "MainTabBar") {
			t.Fatalf("%s missing MainTabBar", path)
		}
	}

	service := string(readOrderAppFileForTest(t, filepath.Join("..", "miniapp", "src", "pages", "service", "service.vue")))
	for _, want := range []string{"销售单", "出库单", "openOrderDocument", "uni.downloadFile", "uni.openDocument"} {
		if !strings.Contains(service, want) {
			t.Fatalf("service.vue missing %q", want)
		}
	}

	miniAPI := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customerportal", "mini_api.go")))
	for _, want := range []string{
		"/api/mini/orders/:id/sales-order-latest.pdf",
		"/api/mini/orders/:id/delivery-note-latest.pdf",
		"EnsureOrderAccess",
	} {
		if !strings.Contains(miniAPI, want) {
			t.Fatalf("mini_api.go missing %q", want)
		}
	}
}

func TestDev278MiniappMainTabsDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "REQUIREMENTS.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("..", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		body := string(readOrderAppFileForTest(t, path))
		if !strings.Contains(body, "PR-278-MINIAPP-MAIN-TABS") {
			t.Fatalf("%s missing PR-278-MINIAPP-MAIN-TABS", path)
		}
	}

	for _, path := range []string{
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
	} {
		body := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{"底部四个入口", "首页", "订单中心", "费用中心", "个人中心", "启动页", "销售单", "出库单"} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing %q", path, want)
			}
		}
	}
}
