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
		"commercialGroups",
		"retailGroups",
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

func TestCostingViewSupportsDripBeanListSource(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "frontend-vue-shell", "src", "views", "CostingView.vue")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"dripGroups",
		"drip_bean_list",
		"drip_wholesale_tiers",
		"product_kind",
		"drip_bag",
		"挂耳豆单",
		"/api/drip-price-templates",
		"/api/costing/drip-price-explanation",
		"openDripPriceExplanation",
		"loadDripPriceExplanation",
		"dripDisplayTiers",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("CostingView.vue missing drip bean-list support %q", want)
		}
	}
}
