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
