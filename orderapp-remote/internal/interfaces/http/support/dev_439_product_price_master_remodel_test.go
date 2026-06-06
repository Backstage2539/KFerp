package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev439ProductPriceMasterRemodelSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-439-PRODUCT-PRICE-MASTER-REMODEL",
		"DEV-439-PRODUCT-ARCHIVE-MASTER-DATA",
		"DEV-439-CUSTOMER-PRODUCT-SNAPSHOT-SUMMARY",
		"DEV-439-LEGACY-TEMPLATE-WRITE-CUTOFF",
		"DEV-439-COPY-NO-PRICE-BOM",
		"DEV-439-PRICE-MASTER-DATA",
		"DEV-439-TIER-SCHEME-FINAL-PRICE-REFERENCE",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PR-439 requirement seed missing %q", want)
		}
	}
}

func TestDev439ProductPriceMasterRemodelWiringAndDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"商品价格管理",
			"product-price-management-pane",
			"商品价格记录",
			"阶梯价格方案",
			"价格摘要",
			"暂无价格表价格",
			"productPriceSummaryLabel",
			"aliasPriceSummaryLabel",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"): {
			"buildCustomerProductAliasPayload",
			"buildProductPriceRecordPayload",
			"buildProductTierPriceSchemePayload",
			"include_in_price_list",
		},
		filepath.Join("internal", "interfaces", "http", "catalog", "product_routes.go"): {
			"ProductConfigTemplateID:  0",
			"ClassificationTemplateID: 0",
			"Tiers:                    nil",
			"/api/product-price-records",
			"/api/product-tier-price-schemes",
		},
		filepath.Join("internal", "application", "catalog", "service.go"): {
			"ProductPriceRecord",
			"SaveProductTierPriceScheme",
			"SourcePriceRecordID",
		},
		filepath.Join("internal", "infrastructure", "postgres", "catalog", "repository.go"): {
			"product_production_config_fields",
			"copy_product_archive",
			"save_product_price_record",
			"save_product_tier_price_scheme",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-439-PRODUCT-PRICE-MASTER-REMODEL",
			"价格摘要来自商品价格表快照",
			"普通商品和客户商品保存不再写入旧模板字段",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-439-PRODUCT-PRICE-MASTER-REMODEL",
			"商品档案和客户商品不出现旧模板字段",
			"复制为商品档案不复制 BOM、价格或价格表快照",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-439-PRODUCT-PRICE-MASTER-REMODEL",
			"商品档案只维护商品资料、库存单位、规格、整数限制和行业字段",
			"价格摘要来自商品价格表快照",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-439-PRODUCT-PRICE-MASTER-REMODEL",
			"商品价格管理维护最终价格记录",
			"录单单位和价格来自已发布价格表快照",
		},
		filepath.Join("docs", "acceptance", "2026-06-06-product-price-master-remodel.md"): {
			"PR-439-PRODUCT-PRICE-MASTER-REMODEL",
			"旧模板字段不再进入新业务写入",
			"Product Design",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-439 marker %q", rel, want)
			}
		}
	}
}
