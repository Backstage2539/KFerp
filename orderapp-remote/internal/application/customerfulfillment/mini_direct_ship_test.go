package customerfulfillment

import "testing"

func TestNormalizeMiniDirectShipItemsMergesSameSKUAndSpec(t *testing.T) {
	got, err := normalizeMiniDirectShipItems([]MiniDirectShipItemCommand{
		{ProductID: 911, SpecG: 1000, Qty: 2},
		{ProductID: 912, SpecG: 227, Qty: 1},
		{ProductID: 911, SpecG: 1000, Qty: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ProductID != 911 || got[0].Qty != 5 || got[1].ProductID != 912 || got[1].Qty != 1 {
		t.Fatalf("normalized items = %#v", got)
	}
}

func TestNormalizeMiniDirectShipItemsRejectsInvalidConcreteSKU(t *testing.T) {
	for _, items := range [][]MiniDirectShipItemCommand{
		nil,
		{{ProductID: 0, SpecG: 1000, Qty: 1}},
		{{ProductID: 911, SpecG: 0, Qty: 1}},
		{{ProductID: 911, SpecG: 1000, Qty: 0}},
	} {
		if _, err := normalizeMiniDirectShipItems(items); err == nil {
			t.Fatalf("items %#v should be rejected", items)
		}
	}
}
