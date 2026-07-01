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
		"price-list-page-config",
		"categoryProductGroups",
		"pdfGroups",
		"productGroupsForType",
		"commercial_bean_list",
		"retail_bean_list",
		"bean-code",
		"attributeLines",
		"pdf-price-list",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("CostingView.vue missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"item.recommendedUse",
		"item.flavor",
		"item.description",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("CostingView.vue should not render legacy bean-list metadata %q", forbidden)
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
		"price-list-page-config",
		"pdfGroups",
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
		"price-list-page-config",
		"v-for=\"group in pdfGroups\"",
		"selectedProductPriceListLabel",
		"categoryProductGroups",
		"productPriceListTypeOptions",
		"productGroupsForType",
		"pdfTheme.value.listType",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("CostingView.vue missing inline product-type price-list marker %q", want)
		}
	}
	if strings.Contains(src, "productPriceListPreviewSections") || strings.Contains(src, "collapsible-bean-section") {
		t.Fatalf("CostingView.vue should not restore old product-type preview cards")
	}
}

func TestCostingViewFollowsProductCatalogBusinessGroupTemplate(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "frontend-vue-shell", "src", "views", "CostingView.vue")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"apiGet('/api/business-groups')",
		"apiGet('/api/business-group-assignments?usage_key=product_catalog&object_key=product')",
		"groupRowsByBusinessGroupTemplate",
		"selectedProductCatalogGroupTemplate",
		"priceListProductBusinessGroups",
		"priceListProductBusinessGroupAssignments",
		"group_source: 'product_catalog'",
		"business-group-unclassified",
		"usageKey: 'product_catalog'",
		"readFormDraft(productSettingsDraftKeyForPriceList())",
		"selectedProductGroupTemplateID",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("CostingView.vue must follow product catalog business group template; missing %q", want)
		}
	}
	if strings.Contains(src, "categoryCodeOfItem(item, listType) === category.code") {
		t.Fatalf("CostingView.vue should not group price-list products by legacy classification category code")
	}
}
