package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev433MiniappCustomerProductsPriceListsSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-433-MINIAPP-CUSTOMER-PRODUCTS-PRICE-LISTS",
		"DEV-433-MINIAPP-CUSTOMER-PRODUCTS-API",
		"DEV-433-CUSTOMER-CATEGORY-TEMPLATE-MINI",
		"DEV-433-PRICE-TABLE-GROUPED-UI",
		"DEV-433-RESALE-EDITOR-SIMPLIFY-PRICE-STYLE",
		"DEV-433-MINIAPP-OUTPUT-PREVIEW",
		"DEV-433-MINIAPP-BUILD-DEPLOY",
		"REV-433-MINIAPP-CUSTOMER-PRODUCTS-PRICE-LISTS",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PR-433 requirement seed missing %q", want)
		}
	}
}

func TestDev433MiniappCustomerProductsPriceListsWiring(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "customerportal", "mini_api.go"): {
			"/api/mini/customer-products",
			"/api/mini/customer-products/categories",
			"/api/mini/customer-products/categories/:id/move",
			"/api/mini/customer-products/:id/category",
		},
		filepath.Join("internal", "application", "customerportal", "service.go"): {
			"GetCustomerProducts",
			"CreateCustomerProductCategory",
			"MoveCustomerProductCategory",
			"AssignCustomerProductCategory",
			"CapabilityBeanList",
		},
		filepath.Join("internal", "infrastructure", "postgres", "customerportal", "business_repository.go"): {
			"ensureCustomerProductClassificationTemplate",
			"customer_product_alias_classification_template_usages",
			"customer_product_alias_classification_assignments",
			"allow_customer_resale",
			"use_public_gradient_templates",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-433 wiring marker %q", rel, want)
			}
		}
	}
}

func TestDev433MiniappCustomerProductsPriceListsMiniappAndDeploy(t *testing.T) {
	home := string(readOrderAppFileForTest(t, filepath.Join("..", "miniapp", "src", "utils", "capabilities.ts")))
	profile := string(readOrderAppFileForTest(t, filepath.Join("..", "miniapp", "src", "pages", "profile", "profile.vue")))
	service := string(readOrderAppFileForTest(t, filepath.Join("..", "miniapp", "src", "pages", "service", "service.vue")))

	if strings.Contains(home, "我的商品") || strings.Contains(home, "beanList") {
		t.Fatalf("home capability entries must not expose 我的商品/beanList as a home shortcut")
	}
	for _, want := range []string{"我的商品", "/pages/service/service?key=beanList"} {
		if !strings.Contains(profile, want) {
			t.Fatalf("profile.vue missing customer product entry marker %q", want)
		}
	}
	for _, want := range []string{
		"商品分类",
		"商品价格表",
		"我的价格表设置",
		"已发布商品价格表",
		"预览 PDF",
		"预览长图",
		"resaleStyleColorPresets",
		"resaleCardsPerRowOptions",
		"uni.openDocument",
		"showMenu: true",
		"uni.previewImage",
	} {
		if !strings.Contains(service, want) {
			t.Fatalf("service.vue missing PR-433 miniapp marker %q", want)
		}
	}
	for _, forbidden := range []string{"覆盖档位", "单品价", `placeholder="背景色 #f8f1e5"`, `placeholder="每行卡片数"`} {
		if strings.Contains(service, forbidden) {
			t.Fatalf("service.vue must remove legacy resale editor marker %q", forbidden)
		}
	}
	if deployBytes, err := os.ReadFile(repoFilePath(t, filepath.Join("..", "deploy_orderapp.sh"))); err == nil {
		deploy := string(deployBytes)
		for _, want := range []string{"npm ci", "npm run typecheck", "npm run build:mp-weixin", "miniapp/dist/build/mp-weixin"} {
			if !strings.Contains(deploy, want) {
				t.Fatalf("deploy_orderapp.sh missing miniapp build marker %q", want)
			}
		}
		if strings.Contains(deploy, "--exclude='./dist'") {
			t.Fatalf("deploy_orderapp.sh must sync miniapp dist after build")
		}
	} else if !os.IsNotExist(err) {
		t.Fatalf("read deploy_orderapp.sh: %v", err)
	}
}

func TestDev433MiniappCustomerProductsPriceListsDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-433-MINIAPP-CUSTOMER-PRODUCTS-PRICE-LISTS",
			"`/api/mini/customer-products`",
			"`list_type/list_type_label`",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-433-MINIAPP-CUSTOMER-PRODUCTS-PRICE-LISTS",
			"首页不出现 `我的商品`",
			"部署脚本执行小程序构建",
		},
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md"): {
			"我的商品",
			"商品价格表",
			"我的价格表设置",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"客户转售商品价格表",
			"覆盖档位",
			"单品价",
		},
		filepath.Join("docs", "customer-portal-miniapp-test.md"): {
			"我的商品联调",
			"预览 PDF",
			"预览长图",
			"npm run build:mp-weixin",
		},
		filepath.Join("docs", "acceptance", "2026-06-06-miniapp-customer-products-price-lists.md"): {
			"PR-433",
			"商品分类",
			"小程序构建",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-433 documentation marker %q", rel, want)
			}
		}
	}
}
