package costing

import "testing"

func TestDirectMaterialCostUsesEstimateOnlyAfterActualSources(t *testing.T) {
	cases := []struct {
		name                         string
		weighted, purchase, estimate float64
		hasEstimate                  bool
		want                         string
	}{
		{name: "weighted batch wins", weighted: 12, purchase: 10, estimate: 8, hasEstimate: true, want: "weighted_batch_cost"},
		{name: "purchase price wins", purchase: 10, estimate: 8, hasEstimate: true, want: "purchase_price"},
		{name: "snapshot precedes estimate", estimate: 8, hasEstimate: true, want: "estimated_purchase_price"},
		{name: "zero estimate is explicit", estimate: 0, hasEstimate: true, want: "estimated_purchase_price"},
		{name: "unset estimate falls to zero", want: "zero_purchase_cost"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := materialDirectCostSource(test.weighted, test.purchase, 0, test.estimate, test.hasEstimate)
			if got != test.want {
				t.Fatalf("source=%q, want %q", got, test.want)
			}
		})
	}
}

func TestProductionBOMDirectCostLabelsEstimateAndExplicitZero(t *testing.T) {
	for _, test := range []struct {
		name string
		item productionBomCostItem
		want string
	}{
		{name: "estimated", item: productionBomCostItem{EstimatedUnitPrice: 3.25, HasEstimatedUnitPrice: true}, want: "estimated_purchase_price"},
		{name: "estimated zero", item: productionBomCostItem{HasEstimatedUnitPrice: true}, want: "estimated_purchase_price"},
		{name: "unset", item: productionBomCostItem{}, want: "zero_purchase_cost"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := productionBomDirectCostSource(test.item); got != test.want {
				t.Fatalf("source=%q, want %q", got, test.want)
			}
		})
	}
}
