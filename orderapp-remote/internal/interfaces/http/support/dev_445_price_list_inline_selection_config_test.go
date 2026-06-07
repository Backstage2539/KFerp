package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev445PriceListInlineSelectionConfigContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-445-PRICE-LIST-INLINE-SELECTION-CONFIG",
			"DEV-445-PRICE-LIST-CATEGORY-INLINE-CONFIG",
			"DEV-445-PRICE-LIST-PRODUCT-INLINE-CONFIG",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue"): {
			"category-inline-pricing-config",
			"product-inline-pricing-config",
			"priceListProductRowForItem",
			"分类和单品计价直接在下方选品位置处理",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-445-PRICE-LIST-INLINE-SELECTION-CONFIG",
			"分类头部 A 位置",
			"商品勾选行 B 位置",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-441 / PR-445",
			"父类/子类计价放在分类头 A 位置",
			"商品行覆盖放在商品勾选行 B 位置",
		},
		filepath.Join("docs", "acceptance", "2026-06-07-price-list-inline-selection-config.md"): {
			"PR-445 价格表计价配置内联到选品位置",
			"分类头 A 位置",
			"商品勾选行 B 位置",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-445 contract marker %q", rel, want)
			}
		}
	}

	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue")))
	builderStart := strings.Index(view, `<div class="pdf-picker price-list-template-builder"`)
	selectionStart := strings.Index(view, `<div class="pdf-picker productSelection">`)
	if builderStart < 0 || selectionStart <= builderStart {
		t.Fatalf("price-list builder and product selection blocks not found")
	}
	builder := view[builderStart:selectionStart]
	for _, forbidden := range []string{"父类计价配置", "子类计价配置", "product-override-row"} {
		if strings.Contains(builder, forbidden) {
			t.Fatalf("price-list builder must not expose standalone config marker %q", forbidden)
		}
	}
}
