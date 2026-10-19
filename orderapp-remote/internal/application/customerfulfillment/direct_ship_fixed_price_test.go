package customerfulfillment

import (
	salesapp "orderapp/internal/application/sales"
	"testing"
)

func TestDirectShipFixedPriceHasNoQuantityGateAndRejectsAmbiguousTiers(t *testing.T) {
	item := MiniDirectShipItemCommand{ProductID: 91, BomSpecID: 801, SalesUnit: "袋", Qty: 1}
	fixed := salesapp.ProductTierOption{BomSpecID: 801, UnitPrice: 47, SalesUnit: "袋", MinQty: 0}
	products := []salesapp.ProductOption{{ID: 91, Tiers: []salesapp.ProductTierOption{fixed}}}
	for _, quantity := range []int64{1, 12, 10000} {
		item.Qty = quantity
		if err := ValidateDirectShipFixedPrices([]MiniDirectShipItemCommand{item}, products); err != nil {
			t.Fatal(err)
		}
		rows, total, err := priceMiniDirectShipItems([]MiniDirectShipItemCommand{item}, products)
		if err != nil || len(rows) != 1 || rows[0].UnitPrice != 47 || total != 47*float64(quantity) {
			t.Fatalf("quantity=%d rows=%+v total=%v error=%v", quantity, rows, total, err)
		}
	}
	for _, tiers := range [][]salesapp.ProductTierOption{
		{{BomSpecID: 801, UnitPrice: 47, SalesUnit: "袋", MinQty: 24}},
		{fixed, fixed},
		{{BomSpecID: 802, UnitPrice: 47, SalesUnit: "袋"}},
	} {
		products[0].Tiers = tiers
		if err := ValidateDirectShipFixedPrices([]MiniDirectShipItemCommand{item}, products); err == nil {
			t.Fatalf("accepted invalid fixed prices %+v", tiers)
		}
	}
}
