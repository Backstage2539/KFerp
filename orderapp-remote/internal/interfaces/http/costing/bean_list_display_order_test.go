package costing

import (
	"bytes"
	appcosting "orderapp/internal/application/costing"
	"os"
	"path/filepath"
	"testing"
)

func TestBeanListPDFKeepsPriceTableDisplayOrder(t *testing.T) {
	item := func(name string) map[string]any {
		return map[string]any{"name": name, "prices": []any{map[string]any{"label": "250g", "value": "30元/袋"}, map[string]any{"label": "1kg", "value": "100元/袋"}}}
	}
	row := appcosting.BeanListPublication{Version: "V3.0.5", ListType: "commercial", Config: map[string]any{"price_list_display_order": map[string]any{"roots": []string{"espresso", "filter"}}}, Content: map[string]any{"title": "排序验证价格表", "groups": []any{map[string]any{"category": "意式", "items": []any{item("红岩")}}, map[string]any{"category": "浅烘", "items": []any{item("耶加 G2"), item("花魁")}}}}}
	doc := beanListPublicationPDFDocument(row)
	if len(doc.Groups) != 2 || doc.Groups[0].Category != "意式" || doc.Groups[1].Items[0].Name != "耶加 G2" || doc.Groups[1].Items[1].Name != "花魁" || len(doc.Groups[1].Items[0].Prices) != 2 {
		t.Fatalf("published ordering changed: %+v", doc.Groups)
	}
	raw, err := renderBeanListPublicationPDF(row)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(raw, []byte("%PDF")) {
		t.Fatal("invalid PDF")
	}
	if dir := os.Getenv("ORDERAPP_TEST_EVIDENCE_DIR"); dir != "" {
		if err := os.WriteFile(filepath.Join(dir, "price-table-display-order.pdf"), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
