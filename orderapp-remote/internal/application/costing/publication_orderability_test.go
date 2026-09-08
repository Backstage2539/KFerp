package costing

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type orderabilityRepoFake struct {
	fakeRepo
	checks   int
	badTitle string
}

func (r *orderabilityRepoFake) ValidateBeanListOrderability(_ context.Context, cmd PublishBeanListCommand) error {
	r.checks++
	if r.badTitle == "" || stringValue(cmd.Content["title"]) == r.badTitle {
		return fmt.Errorf("商品「测试生豆」（#97）未配置可用于录单的默认已发布 BOM 规格")
	}
	return nil
}

func TestPublicationOrderabilityBlocksSingle(t *testing.T) {
	ctx := context.Background()
	r := &orderabilityRepoFake{}
	_, err := NewService(r).PublishBeanList(ctx, PublishBeanListCommand{ListType: "green", Version: "V3.0.8", OwnerType: "official", Content: orderabilityValidContent()})
	if err == nil || !strings.Contains(err.Error(), "测试生豆") || r.publishedBeanList.Content != nil {
		t.Fatalf("unorderable single table was published: err=%v saved=%+v", err, r.publishedBeanList)
	}
}

func TestPublicationOrderabilityBlocksDraftPDFBeforeCacheButPreservesHistory(t *testing.T) {
	for _, status := range []string{"draft", "published", "withdrawn", "archived"} {
		t.Run(status, func(t *testing.T) {
			r := &orderabilityRepoFake{}
			r.beanListPublication = &BeanListPublication{ID: 7, ListType: "green", Version: "V3.0.8", Status: status, OwnerType: "official", Content: orderabilityValidContent()}
			r.beanListAsset = BeanListPublicationAsset{PublicationID: 7, AssetType: "pdf", CacheKey: beanListPublicationPDFCacheKey(*r.beanListPublication), Payload: []byte("%PDF-frozen")}
			_, err := NewService(r).GenerateBeanListPublicationPDF(context.Background(), BeanListPublicationPDFCommand{PublicationID: 7, Query: BeanListPublicationQuery{ListType: "green", OwnerType: "official"}}, func(BeanListPublication) ([]byte, error) { t.Fatal("cached PDF rendered again"); return nil, nil })
			if status == "draft" && (err == nil || r.checks != 1) {
				t.Fatalf("unorderable draft cache bypass: err=%v checks=%d", err, r.checks)
			}
			if status != "draft" && (err != nil || r.checks != 0) {
				t.Fatalf("historical snapshot was revalidated: err=%v checks=%d", err, r.checks)
			}
		})
	}
}

func orderabilityValidContent() map[string]any {
	return map[string]any{"price_rows": []any{map[string]any{
		"product_id": 10, "final_unit_price": 30, "fixed_unit_price": 30, "price_unit": "袋", "inventory_unit": "袋", "inventory_conversion_json": map[string]any{"袋": map[string]any{"袋": 1}},
		"group_snapshot": map[string]any{"name": "咖啡豆"}, "group_source": "product_catalog", "pricing_mode": "fixed_price", "pricing_mode_source": "product", "cost_source_snapshot": map[string]any{"source": "fixed_price"}, "customer_reference_snapshot": map[string]any{}, "manual_adjusted": false,
	}}}
}
