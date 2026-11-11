package costing

import (
	"context"
	domain "orderapp/internal/domain/costing"
	"testing"
)

func TestCostRecheckPreservesPublishedDocumentSnapshot(t *testing.T) {
	if !shouldRecheckBeanListCost(&BeanListPublication{Status: "draft"}) {
		t.Fatal("draft must check current costs")
	}
	if shouldRecheckBeanListCost(&BeanListPublication{Status: "published"}) {
		t.Fatal("published document must remain frozen")
	}
	if shouldRecheckBeanListCost(&BeanListPublication{Status: "archived"}) {
		t.Fatal("archived document must remain frozen")
	}
}

type guardedCostRepo struct{ *fakeRepo }

func (r guardedCostRepo) FindUnknownCostProductRefs(_ context.Context, refs []domain.ProductInput) ([]bool, error) {
	out := make([]bool, len(refs))
	for i, ref := range refs {
		out[i] = ref.ProductID == 1
	}
	return out, nil
}

func TestCalculateRechecksUnknownCostAgainstRepository(t *testing.T) {
	svc := NewService(guardedCostRepo{&fakeRepo{}})
	result, err := svc.Calculate(context.Background(), CalculateRequest{Products: []domain.ProductInput{{ProductID: 1, Name: "待核算", GreenBeanCostPerKg: 10}, {ProductID: 2, Name: "正常", GreenBeanCostPerKg: 10}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 2 || !result.Items[0].CostUnavailable || len(result.Items[0].CommercialWholesaleTiers) > 0 || result.Items[1].CostUnavailable || len(result.Items[1].WholesaleKgPrices) == 0 {
		t.Fatalf("unexpected results %+v", result.Items)
	}
}
