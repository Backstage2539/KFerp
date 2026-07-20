package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev541PriceListProductSpecSelectionContracts(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-541-PRICE-LIST-PRODUCT-SPEC-SELECTION",
		"DEV-541-PRODUCT-DEFAULT-SKU",
		"DEV-541-PRICE-LIST-SPEC-SELECTION",
		"DEV-541-SALES-SPEC-TIER-SNAPSHOT",
		"DEV-541-DOCS-DEPLOY",
		"REV-541-PRICE-LIST-PRODUCT-SPEC-SELECTION",
	} {
		if !strings.Contains(reqStore, want) {
			t.Fatalf("req_store.go missing PR-541 seed %q", want)
		}
	}

	for rel, wants := range map[string][]string{
		filepath.Join("internal", "infrastructure", "postgres", "catalog", "schema.go"): {
			"default_sku_id",
		},
		filepath.Join("internal", "interfaces", "http", "catalog", "product_routes.go"): {
			"default-sku", "default_sku_id",
		},
		filepath.Join("internal", "application", "costing", "service.go"): {
			"quantity_basis", "sales_spec_count", "effective_sales_spec",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue"): {
			"product_spec_selections", "默认规格", "销售规格",
		},
		filepath.Join("..", "REQUIREMENTS.md"): {
			"PR-541-PRICE-LIST-PRODUCT-SPEC-SELECTION", "default_sku_id", "销售规格件数", "具体 SKU",
		},
		filepath.Join("..", "ACCEPTANCE_TESTS.md"): {
			"PR-541-PRICE-LIST-PRODUCT-SPEC-SELECTION", "X款商品 / Y个规格", "sales_spec_count", "历史发布",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"默认规格", "选择分类和产品", "销售规格件数", "固定价",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"设置默认规格", "操作日志",
		},
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"): {
			"sales_spec_count", "具体 SKU", "历史价格表",
		},
		filepath.Join("docs", "acceptance", "2026-07-20-price-list-product-spec-selection.md"): {
			"PR-541", "RED", "GREEN", "生产环境未部署",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-541 marker %q", rel, want)
			}
		}
	}
}
