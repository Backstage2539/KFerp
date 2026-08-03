package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev440ProductGroupPriceRemodelSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-440-PRODUCT-GROUP-PRICE-REMODEL",
		"DEV-440-GENERIC-GROUP-MANAGEMENT",
		"DEV-440-PRODUCT-CUSTOMER-REFERENCES",
		"DEV-440-PRICING-RULES",
		"DEV-440-PRICE-TIER-TEMPLATES",
		"DEV-440-PRICE-LIST-FLAT-ROWS",
		"REV-440-PRODUCT-GROUP-PRICE-REMODEL",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PR-440 requirement seed missing %q", want)
		}
	}
}

func TestDev440ProductGroupPriceRemodelSchemaAndServiceContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "infrastructure", "postgres", "catalog", "schema.go"): {
			"business_groups",
			"business_group_items",
			"business_group_usages",
			"business_group_assignments",
			"product_customer_references",
			"product_pricing_rules",
			"price_tier_templates",
			"price_tier_template_tiers",
			"customer_product_aliases_legacy_readonly_idx",
		},
		filepath.Join("internal", "application", "catalog", "service.go"): {
			"type BusinessGroup struct",
			"type ProductCustomerReference struct",
			"type ProductPricingRule struct",
			"type PriceTierTemplate struct",
			"SaveProductPriceRecord",
			"product price records are legacy readonly",
			"SaveCustomerProductAlias",
			"customer products are legacy readonly",
			"ResolvePriceTableTemplateInheritance",
		},
		filepath.Join("internal", "interfaces", "http", "catalog", "product_routes.go"): {
			"/api/product-customer-references",
			"customer products are legacy readonly",
			"product price records are legacy readonly",
		},
		filepath.Join("internal", "interfaces", "http", "catalog", "business_group_routes.go"): {
			"/api/business-groups",
		},
		filepath.Join("internal", "interfaces", "http", "catalog", "pricing_routes.go"): {
			"/api/product-pricing-rules",
			"/api/price-tier-templates",
		},
		filepath.Join("internal", "infrastructure", "postgres", "catalog", "repository.go"): {
			"func (r Repository) SaveBusinessGroup",
			"func (r Repository) SaveProductCustomerReference",
			"func (r Repository) SaveProductPricingRule",
			"func (r Repository) SavePriceTierTemplate",
			"save_product_pricing_rule",
			"save_price_tier_template",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-440 contract marker %q", rel, want)
			}
		}
	}
}

func TestDev440ProductGroupPriceRemodelFrontendAndDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js"): {
			"key: 'businessSettings', label: '业务设置'",
			"groupManagement: '分组模板'",
			"key: 'productPriceManagement', label: '商品价格管理'",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"客户引用",
			"销售规格模板",
			"价格计算模板 / Pricing Rule",
			"父商品 > 所在分类 > 上级分类逐级向上 > 价格表",
			"平铺价格行",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue"): {
			"Price List / Item Price",
			"data-pr440-price-list-model",
			"分组项选品",
			"父商品 &gt; 所在分类 &gt; 上级分类逐级向上 &gt; 价格表",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue"): {
			"选择价格表",
			"报价来源：价格表",
			"非最新价格表",
		},
		filepath.Join("scripts", "scenario_acceptance.py"): {
			"material_to_price_list_order_settlement",
			"--dry-run",
			"SCENARIO_DATA_PREFIX",
			"create generated material",
			"create generated product",
			"create generated customer",
			"cleanup_generated_data",
			"void generated order",
			"withdraw generated price list",
			"deprecate generated material",
			"/api/costing/bean-list/publications",
			"/api/materials",
			"/api/product-settings/products/deactivate",
			"/api/orders",
			"/api/mini",
			"POST_DEPLOY_ACCEPTANCE_SCENARIOS",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-440-PRODUCT-GROUP-PRICE-REMODEL",
			"分组模板",
			"价格计算模板",
			"商品 > 子类 > 父类 > 价格表",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-440 / PR-453 / PR-528",
			"分组模板是 `设置 / 分组模板` 独立入口",
			"菜单不出现 `客户商品`",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-440-PRODUCT-GROUP-PRICE-REMODEL",
			"商品价格表就是 Price List",
			"价格计算模板 / Pricing Rule",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-440 marker %q", rel, want)
			}
		}
	}
	menuSrc := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js")))
	for _, forbidden := range []string{
		"key: 'customerProductAliases'",
		"label: '客户商品'",
		"label: '商品分类管理'",
	} {
		if strings.Contains(menuSrc, forbidden) {
			t.Fatalf("menu should not expose old customer product/category entry %q", forbidden)
		}
	}
}
