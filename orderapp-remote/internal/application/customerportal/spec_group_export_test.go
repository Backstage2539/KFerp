package customerportal

import (
	"context"
	"testing"
)

type specPNGCacheRepo struct {
	*fakeRepository
	assets map[string]BeanListPublicationAsset
}

func (r *specPNGCacheRepo) LoadBeanListPublicationAsset(_ context.Context, _ int64, kind string) (BeanListPublicationAsset, error) {
	if a, ok := r.assets[kind]; ok {
		return a, nil
	}
	return BeanListPublicationAsset{}, ErrBeanListPublicationNotFound
}
func (r *specPNGCacheRepo) SaveBeanListPublicationAsset(_ context.Context, a BeanListPublicationAsset, _ string) (BeanListPublicationAsset, error) {
	r.assets[a.AssetType] = a
	return a, nil
}
func TestGroupPNGUsesVersionedAssetAndPreservesLegacyCache(t *testing.T) {
	base := &fakeRepository{context: CurrentContext{CurrentCustomerID: 77, Capabilities: []Capability{{Code: CapabilityProductOrder, Enabled: true}, {Code: CapabilityBeanList, Enabled: true}}}, beanList: BeanListSummary{ID: 11, CacheKey: "bean-list:11:V1"}}
	repo := &specPNGCacheRepo{fakeRepository: base, assets: map[string]BeanListPublicationAsset{"png": {AssetType: "png", CacheKey: "bean-list:11:V1", Payload: []byte("old")}}}
	svc := NewService(repo, fakeIdentityProvider{})
	calls := 0
	render := func(BeanListSummary) ([]byte, error) { calls++; return []byte("new"), nil }
	_, body, err := svc.GetBeanListPublicationPNG(context.Background(), "token", 11, render)
	if err != nil || string(body) != "old" || calls != 0 {
		t.Fatalf("legacy body=%q calls=%d err=%v", body, calls, err)
	}
	base.beanList.SpecPricingGroup = true
	_, body, err = svc.GetBeanListPublicationPNG(context.Background(), "token", 11, render)
	if err != nil || string(body) != "new" || calls != 1 {
		t.Fatalf("group body=%q calls=%d err=%v", body, calls, err)
	}
	_, body, err = svc.GetBeanListPublicationPNG(context.Background(), "token", 11, render)
	if err != nil || string(body) != "new" || calls != 1 || string(repo.assets["png"].Payload) != "old" {
		t.Fatalf("cache body=%q calls=%d err=%v assets=%+v", body, calls, err, repo.assets)
	}
}
