package costing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCostingViewHasBeanListPDFDrawerAndStoredPreviewPDFWorkflow(t *testing.T) {
	view, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "frontend-vue-shell", "src", "views", "CostingView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "frontend-vue-shell", "src", "lib", "bean-list-pdf.js"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(view) + "\n" + string(helper)
	for _, want := range []string{
		"pdfDrawerOpen",
		"生成价格表 PDF",
		"V3.0.5",
		"backgroundColor",
		"fontColor",
		"backgroundImage",
		"accept=\"image/*\"",
		"@media print",
		"bean-list-pdf-page",
		"max-width: 430px",
		"beanListPdfGenerating",
		"apiSend('/api/costing/bean-list/drafts'",
		"/api/costing/bean-list/publications/${row.id}/pdf?${params.toString()}",
		"downloadBeanListPublicationPDF(document)",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PDF source missing %q", want)
		}
	}
	if strings.Contains(src, "window.print") {
		t.Fatalf("PDF generation must use stored backend PDFs, not system print")
	}
}

func TestCostingViewPDFPreviewShowsFullBeanCardsBeforePrinting(t *testing.T) {
	view, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "frontend-vue-shell", "src", "views", "CostingView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(view)
	for _, want := range []string{
		"pdf-preview-title",
		"预览",
		"v-for=\"item in group.items\"",
		"item.recommendedUse",
		"item.flavor",
		"item.description",
		"item.prices",
		"报价",
		"风味",
		"特点",
		"出品建议",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PDF preview source missing %q", want)
		}
	}
	if strings.Contains(src, "group.items.length }} 款") {
		t.Fatalf("PDF preview must render product cards, not only category item counts")
	}
}

func TestCostingViewPDFPrintDoesNotKeepWholeGroupsOnOnePage(t *testing.T) {
	view, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "frontend-vue-shell", "src", "views", "CostingView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(view)
	if strings.Contains(src, ".pdf-group { break-inside: avoid;") {
		t.Fatalf("PDF groups must be allowed to flow across pages so only the first category is not printed")
	}
	for _, want := range []string{
		"page-break-inside: avoid",
		"break-inside: avoid",
		".pdf-item",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PDF item print guard missing %q", want)
		}
	}
}

func TestCostingViewSupportsConfigurableBeanListPublishingWorkflow(t *testing.T) {
	view, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "frontend-vue-shell", "src", "views", "CostingView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "frontend-vue-shell", "src", "lib", "bean-list-pdf.js"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(view) + "\n" + string(helper)
	for _, want := range []string{
		"bean-list-generate-bar",
		"categoryProductGroups",
		"selectedProductIDs",
		"showCategoryNumbers",
		"visibleCategoryCodes",
		"togglePdfCategoryProducts",
		"cardRows(group)",
		"cardRowStyle(row)",
		"pdf-card-row",
		".pdf-card-grid { display: grid; gap: 18px; }",
		"padding: 10px 10px 18px;",
		"priceDisplay(priceRow)",
		"priceValueParts(priceRow, item)",
		"pdf-price-label",
		"pdf-price-value",
		"layoutStyle",
		"cardsPerRow",
		"table",
		"badge",
		"highlightTerms",
		"publishBeanList",
		"withdrawBeanList",
		"showVersion",
		"showChangelog",
		"brandName",
		"logoImage",
		"brandIntro",
		"pdf-bottom-changelog",
		"publicBeanListURL",
		"copyPublicBeanListURL",
		"copyBeanListPublicationContentGroups",
		"publicationScope",
		"selectedPriceSourcePublicationID",
		"复制官方价格来源",
		"当前归属",
		"syncPublicationScopeFromPageContext",
		"price_source_publication_id",
		"style_source_publication_id",
		"fetchCurrentActor",
		"isBeanListAdmin",
		"saveBeanListDraft",
		"保存修改",
		"/api/costing/bean-list/drafts",
		"/public/bean-list/",
		"客户访问链接",
		"<Teleport to=\"body\">",
		"body.bean-list-pdf-printing #app",
		"body.bean-list-pdf-printing .bean-list-pdf-page",
		"/api/costing/bean-list/publications",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("configurable bean-list workflow missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"pricingCollapsed",
		"价格试算",
		"保存试算",
		"试算批次",
		"redPriceLabels",
		"标红价格档",
		"可填 55/包",
		"复制已有豆单配置",
		"selectedCopyPublicationID",
		"applyCopiedBeanListPublicationConfig",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("configurable bean-list workflow should not expose old price-red option %q", forbidden)
		}
	}
	if strings.Contains(src, `<div v-if="pdfTheme.showChangelog && pdfTheme.changelog" class="pdf-changelog">
            <b>更新</b>`) {
		t.Fatalf("PDF changelog should render at the bottom, not directly under the cover")
	}
}

func TestBeanListPDFKeepsLegacyDripSnapshotsButViewDoesNotInferDripType(t *testing.T) {
	view, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "frontend-vue-shell", "src", "views", "CostingView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "frontend-vue-shell", "src", "lib", "bean-list-pdf.js"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(view) + "\n" + string(helper)
	for _, want := range []string{
		"drip_bean_list",
		"drip_wholesale_tiers",
		"productPriceListTypeOptions",
		"priceListRenderTypeForItem",
		"price-list-page-config",
		"pdfGroups",
		"sales_unit",
		"unit_bag_count",
		"packed_price_per_bag",
		"packed_price_per_box",
		"盒(",
		"bag",
		"box",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PDF source must support drip bean-list pricing; missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"if (kind === 'drip_bag') return 'drip'",
		"categoryHint.includes('挂耳')",
		"section.listType === 'drip'",
	} {
		if strings.Contains(string(view), forbidden) {
			t.Fatalf("CostingView.vue should not infer dedicated drip price-list type %q", forbidden)
		}
	}
}

func TestCostingViewHasInlineBeanListConfiguration(t *testing.T) {
	view, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "frontend-vue-shell", "src", "views", "CostingView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(view)
	for _, want := range []string{
		"price-list-page-config",
		"Price List / Item Price 生成规则",
		"productSelection",
		"categoryProductGroups",
		"price-list-rules-dialog",
		"pdf-preview-phone",
		"v-for=\"group in pdfGroups\"",
		"selectedProductPriceListLabel",
		"pdfGroups",
		"productGroupsForType",
		"green_bean_list",
		"green_bean_sale_tiers",
		"商品价格表",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("CostingView.vue missing collapsible bean-list preview support %q", want)
		}
	}
	if strings.Contains(src, "生成挂耳豆单") {
		t.Fatalf("outer drip bean-list button should be removed; use the drawer list type selector")
	}
	if strings.Contains(src, "collapsible-bean-section") {
		t.Fatalf("old collapsible preview sections should not return; use the inline price-list configuration")
	}
}
