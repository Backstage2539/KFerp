package costing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCostingViewGroupsBeanListsByExcelCategoryAndShowsMetadata(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "frontend-vue-shell", "src", "views", "CostingView.vue")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"productPriceListPreviewSections",
		"productGroupsForType",
		"commercial_bean_list",
		"retail_bean_list",
		"bean-code",
		"recommended_use",
		"description",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("CostingView.vue missing %q", want)
		}
	}
}

func TestCostingViewDoesNotExposeDedicatedDripTemplateSource(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "frontend-vue-shell", "src", "views", "CostingView.vue")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"productPriceListPreviewSections",
		"productPriceListTypeOptions",
		"priceListRenderTypeForItem",
		"商品价格表",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("CostingView.vue missing product price-list source %q", want)
		}
	}
	for _, forbidden := range []string{
		"if (kind === 'drip_bag') return 'drip'",
		"categoryHint.includes('挂耳')",
		"categoryHint.includes('drip')",
		"section.listType === 'drip'",
		"/api/drip-price-templates",
		"/api/costing/drip-price-explanation",
		"openDripPriceExplanation",
		"loadDripPriceExplanation",
		"dripDisplayTiers",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("CostingView.vue should not expose dedicated drip template source %q", forbidden)
		}
	}
}

func TestCostingPreviewSectionsUseProductTypesInsteadOfLegacyCards(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "frontend-vue-shell", "src", "views", "CostingView.vue")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, forbidden := range []string{
		"<b>商用批发豆单</b>",
		"<b>挂耳豆单</b>",
		"<b>零售豆单</b>",
		"<b>生豆豆单</b>",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("CostingView.vue must render preview cards from product types, found legacy card %q", forbidden)
		}
	}
	for _, want := range []string{
		"v-for=\"section in productPriceListPreviewSections\"",
		"section.label",
		"section.groups",
		"section.listType",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("CostingView.vue missing dynamic product-type preview marker %q", want)
		}
	}
}
