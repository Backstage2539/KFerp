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
