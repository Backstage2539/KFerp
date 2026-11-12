package customerportal

import (
	"bytes"
	"fmt"
	"image/png"
	app "orderapp/internal/application/customerportal"
	pdfinfra "orderapp/internal/infrastructure/pdf"
	"os"
	"path/filepath"
	"testing"
)

func TestBeanListGroupPNGDocumentRetainsEachSpecAndPrice(t *testing.T) {
	row := app.BeanListSummary{Groups: []app.BeanListGroupSummary{{Category: "咖啡", Items: []app.BeanListProductSummary{
		{Name: "模拟咖啡", AttributeLines: []string{"规格：227g"}, Prices: []app.BeanListPriceSummary{{Label: "基础价", Value: "33.80/袋"}}},
		{Name: "模拟咖啡", AttributeLines: []string{"规格：100g"}, Prices: []app.BeanListPriceSummary{{Label: "基础价", Value: "16.80/袋"}}},
		{Name: "模拟咖啡", AttributeLines: []string{"规格：50g"}, Prices: []app.BeanListPriceSummary{{Label: "基础价", Value: "9.80/袋"}}},
	}}}}
	doc := beanListPDFDocument(row)
	wantSpec := []string{"规格：227g", "规格：100g", "规格：50g"}
	wantPrice := []string{"33.80/袋", "16.80/袋", "9.80/袋"}
	for i, item := range doc.Groups[0].Items {
		if len(item.AttributeLines) != 1 || item.AttributeLines[0] != wantSpec[i] || len(item.Prices) != 1 || item.Prices[0].Value != wantPrice[i] {
			t.Fatalf("item %d: %+v", i, item)
		}
	}
}

func TestSpecGroupExportsRenderPDFAndPNG(t *testing.T) {
	specs := []struct{ name, price string }{{"227g", "33.80/袋"}, {"100g", "16.80/袋"}, {"50g", "9.80/袋"}}
	row := app.BeanListSummary{Title: "规格计价组测试", ListType: "commercial", VersionNo: "V1", ShowVersion: true, Groups: []app.BeanListGroupSummary{{Category: "咖啡", ShowCategory: true}}}
	for i, s := range specs {
		row.Groups[0].Items = append(row.Groups[0].Items, app.BeanListProductSummary{Code: fmt.Sprintf("1.%d", i+1), Name: "模拟咖啡", AttributeLines: []string{"规格：" + s.name}, Prices: []app.BeanListPriceSummary{{Label: "基础价", Value: s.price}}})
	}
	doc := beanListPDFDocument(row)
	for i, item := range doc.Groups[0].Items {
		if item.AttributeLines[0] != "规格："+specs[i].name || item.Prices[0].Value != specs[i].price {
			t.Fatalf("spec/price mismatch: %+v", item)
		}
	}
	pdfBytes, err := (pdfinfra.BeanListRenderer{}).Render(doc)
	if err != nil {
		t.Fatal(err)
	}
	pngBytes, err := (pdfinfra.BeanListRenderer{}).RenderPNG(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF")) || !bytes.HasPrefix(pngBytes, []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatal("invalid export")
	}
	if _, err = png.Decode(bytes.NewReader(pngBytes)); err != nil {
		t.Fatal(err)
	}
	if dir := os.Getenv("KFERP_SPEC_EXPORT_EVIDENCE"); dir != "" {
		if err = os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, "spec-pricing.pdf"), pdfBytes, 0600); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, "spec-pricing.png"), pngBytes, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
