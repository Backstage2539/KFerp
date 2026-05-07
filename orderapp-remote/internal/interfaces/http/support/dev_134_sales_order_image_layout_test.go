package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev134SalesOrderImageLayoutRequirementSeeds(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(content)
	for _, want := range []string{
		"PR-134",
		"DEV-134-01",
		"UT-134-01",
		"API-134-01",
		"REV-134-01",
		"销售单图片字体和排版",
		"PNG 字体高度不得超过行高",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestDev134SalesOrderPNGUsesReadableFontMetrics(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("internal", "infrastructure", "pdf", "sales_order_png.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(content)
	for _, want := range []string{
		"salesOrderPNGDPI",
		"DPI:     salesOrderPNGDPI",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("sales_order_png.go missing %q", want)
		}
	}

	testContent, err := os.ReadFile(filepath.Join("internal", "infrastructure", "pdf", "sales_order_pdf_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(testContent), "TestSalesOrderPNGTextMetricsFitConfiguredLineHeights") {
		t.Fatal("sales_order_pdf_test.go must guard PNG font height against configured line heights")
	}
}
