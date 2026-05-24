package costing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCostingViewUsesProductPriceListLanguage(t *testing.T) {
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
		"产品价格表",
		"发布价格表",
		"生成价格表",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("product price list source missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"产品豆单",
		"发布豆单",
		"生成豆单",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("product price list source still contains old primary label %q", forbidden)
		}
	}
}
