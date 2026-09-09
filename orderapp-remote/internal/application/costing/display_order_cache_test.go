package costing

import "testing"

func TestPriceTableDraftDisplayOrderChangesPDFCacheWithoutRewritingPublishedAssets(t *testing.T) {
	row := BeanListPublication{ID: 7, Version: "V3.0.5", Status: "draft", Config: map[string]any{"price_list_display_order": map[string]any{"roots": []string{"a", "b"}}}, Content: map[string]any{"groups": []string{"a", "b"}}}
	before := beanListPublicationPDFCacheKey(row)
	row.Config["price_list_display_order"] = map[string]any{"roots": []string{"b", "a"}}
	row.Content["groups"] = []string{"b", "a"}
	if after := beanListPublicationPDFCacheKey(row); after == before {
		t.Fatal("draft sorting reused stale PDF cache")
	}
	row.Status = "published"
	if got := beanListPublicationPDFCacheKey(row); got != "bean-list-preview-style-v4:7:V3.0.5" {
		t.Fatalf("published cache changed: %s", got)
	}
}
