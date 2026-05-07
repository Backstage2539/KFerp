package costing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCostingViewHasBeanListPDFDrawerAndMobilePrintStyles(t *testing.T) {
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
		"生成豆单 PDF",
		"V3.0.5",
		"backgroundColor",
		"fontColor",
		"backgroundImage",
		"accept=\"image/*\"",
		"window.print",
		"@media print",
		"bean-list-pdf-page",
		"max-width: 430px",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PDF source missing %q", want)
		}
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
		"pricingCollapsed",
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
		"copyBeanListPublicationConfig",
		"copyBeanListPublicationContentGroups",
		"publicationScope",
		"selectedPriceSourcePublicationID",
		"复制官方价格来源",
		"我的客户豆单",
		"price_source_publication_id",
		"style_source_publication_id",
		"copyableBeanListPublications",
		"selectedCopyPublicationID",
		"applyCopiedBeanListPublicationConfig",
		"复制已有豆单配置",
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
		"redPriceLabels",
		"标红价格档",
		"可填 55/包",
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
