package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev544ParentPricingOrderPriceListSpecsContracts(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-544-PARENT-PRICING-ORDER-PRICE-LIST-SPECS",
		"DEV-544-PARENT-PRICING",
		"DEV-544-PUBLISH-GUARD",
		"DEV-544-ORDER-PRICE-LIST-SPECS",
		"DEV-544-DOCS-DEPLOY",
		"REV-544-PARENT-PRICING-ORDER-PRICE-LIST-SPECS",
	} {
		if !strings.Contains(reqStore, want) {
			t.Fatalf("req_store.go missing PR-544 seed %q", want)
		}
	}

	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-544-PARENT-PRICING-ORDER-PRICE-LIST-SPECS", "父商品只设置一次", "当前已选已发布价格表",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-544-PARENT-PRICING-ORDER-PRICE-LIST-SPECS", "乌拉嘎", "切换价格表版本",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-544", "商品计价", "固定价金额",
		},
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"): {
			"PR-544", "父商品", "价格表中的规格",
		},
		filepath.Join("docs", "acceptance", "2026-07-21-parent-pricing-order-price-list-specs.md"): {
			"PR-544", "RED", "GREEN", "生产环境未部署",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-544 marker %q", rel, want)
			}
		}
	}
}
