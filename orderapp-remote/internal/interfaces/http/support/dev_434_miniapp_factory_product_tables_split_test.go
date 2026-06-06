package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev434MiniappFactoryProductTablesSplitSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-434-MINIAPP-FACTORY-PRODUCT-TABLES-SPLIT",
		"DEV-434-MINIAPP-PROFILE-SPLIT-ENTRIES",
		"DEV-434-FACTORY-PRODUCT-TABLE-OUTPUTS",
		"DEV-434-CUSTOMER-PRODUCT-SETTINGS-COPY",
		"DEV-434-DRAFT-CATEGORY-ITEM-BADGES",
		"REV-434-MINIAPP-FACTORY-PRODUCT-TABLES-SPLIT",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PR-434 requirement seed missing %q", want)
		}
	}
}

func TestDev434MiniappFactoryProductTablesSplitWiringAndDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "customerportal", "mini_api.go"): {
			"/api/mini/bean-lists/:id.png",
			"RenderPNG",
		},
		filepath.Join("internal", "application", "customerportal", "service.go"): {
			"CopySourceType",
			"PriceSource",
			"category_drafts",
		},
		filepath.Join("..", "miniapp", "src", "pages.json"): {
			"pages/factory-products/factory-products",
			"pages/customer-products/customer-products",
			"pages/price-table-settings/price-table-settings",
		},
		filepath.Join("..", "miniapp", "src", "pages", "profile", "profile.vue"): {
			"工厂商品表",
			"我的商品",
		},
		filepath.Join("..", "miniapp", "src", "pages", "factory-products", "factory-products.vue"): {
			"工厂商品表",
			"PDF",
			"长图",
		},
		filepath.Join("..", "miniapp", "src", "pages", "price-table-settings", "price-table-settings.vue"): {
			"工厂价格表",
			"我的已发布价格表",
			"商品配置",
			"标红词",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-434-MINIAPP-FACTORY-PRODUCT-TABLES-SPLIT",
			"工厂商品表",
			"价格表设置",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-434-MINIAPP-FACTORY-PRODUCT-TABLES-SPLIT",
			"PDF",
			"长图",
		},
		filepath.Join("docs", "customer-portal-miniapp-test.md"): {
			"工厂商品表联调",
			"我的商品联调",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-434 marker %q", rel, want)
			}
		}
	}
}
