package costing

import (
	"bytes"
	appcosting "orderapp/internal/application/costing"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestBeanListPDFIndustryAttributesGroupOnlyByFrozenTemplateIdentity(t *testing.T) {
	item := map[string]any{"product_attributes": []any{
		map[string]any{"key": "roast", "label": "烘焙度", "value": "中深烘", "template_id": float64(11)},
		map[string]any{"key": "origin", "label": "产地", "value": "云南", "template_id": float64(11)},
		map[string]any{"key": "pack", "label": "包装", "value": "250g", "template_id": float64(12)},
		map[string]any{"key": "old_a", "label": "历史甲", "value": "甲"},
		map[string]any{"key": "old_b", "label": "历史乙", "value": "乙"},
	}}
	got := beanListPublicationPDFAttributeLines(item)
	want := []string{"烘焙度：中深烘；产地：云南", "包装：250g", "历史甲：甲", "历史乙：乙"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("attribute lines = %#v, want %#v", got, want)
	}
	if got := beanListPublicationPDFAttributeLines(map[string]any{"attributeLines": []any{"旧历史分行：保留", "另一行：保留"}}); !reflect.DeepEqual(got, []string{"旧历史分行：保留", "另一行：保留"}) {
		t.Fatalf("historical attribute lines were regrouped: %#v", got)
	}
}

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
