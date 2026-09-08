package costing

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type orderabilityRepoFake struct {
	batchRepoFake
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

func TestPublicationOrderabilityBlocksSingleAndEntireBatch(t *testing.T) {
	ctx := context.Background()
	r := &orderabilityRepoFake{}
	_, err := NewService(r).PublishBeanList(ctx, PublishBeanListCommand{ListType: "green", Version: "V3.0.8", OwnerType: "official", Content: namedBatchValidContent()})
	if err == nil || !strings.Contains(err.Error(), "测试生豆") || r.publishedBeanList.Content != nil {
		t.Fatalf("unorderable single table was published: err=%v saved=%+v", err, r.publishedBeanList)
	}
	r = &orderabilityRepoFake{badTitle: "规格3价格表"}
	_, err = NewService(r).PublishBeanListBatch(ctx, batchFixture(3))
	if err == nil || !strings.Contains(err.Error(), "规格3价格表") || !strings.Contains(err.Error(), "测试生豆") || r.calls != 0 {
		t.Fatalf("unorderable third table did not block entire batch: err=%v writes=%d", err, r.calls)
	}
}

func TestPublicationOrderabilityBlocksDraftPDFBeforeCacheButPreservesHistory(t *testing.T) {
	for _, status := range []string{"draft", "published", "withdrawn", "archived"} {
		t.Run(status, func(t *testing.T) {
			r := &orderabilityRepoFake{}
			r.beanListPublication = &BeanListPublication{ID: 7, ListType: "green", Version: "V3.0.8", Status: status, OwnerType: "official", Content: namedBatchValidContent()}
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
