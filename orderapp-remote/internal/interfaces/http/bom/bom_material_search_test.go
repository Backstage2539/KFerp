package bom

import (
	"os"
	"strings"
	"testing"
)

func TestBomViewUsesVueMaterialOptions(t *testing.T) {
	b, err := os.ReadFile("frontend-vue-shell/src/views/BomView.vue")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	required := []string{
		"apiGet('/api/bom/materials')",
		`:options="materials"`,
		"@submit.prevent=\"saveItem\"",
		"/api/product-settings/units",
		"选择物料",
		"componentTypeLabel",
		"商品组件",
		"component_product_id",
		"consume_unit",
		"qty_per_unit",
	}
	for _, want := range required {
		if !strings.Contains(src, want) {
			t.Fatalf("BomView.vue missing %q", want)
		}
	}
}
