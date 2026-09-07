package sales

import "testing"

func namedOrderTableOptions() []BeanListVersionOption {
	return []BeanListVersionOption{
		{ID: 1, CustomerID: 42, ListType: "commercial", ClassificationTemplateID: 10, ReleaseID: "old", TableKey: "a", TableName: "227g", IsDefaultTable: true},
		{ID: 2, CustomerID: 42, ListType: "commercial", ClassificationTemplateID: 10, ReleaseID: "new", TableKey: "a", TableName: "227g", IsDefaultTable: true},
		{ID: 3, CustomerID: 42, ListType: "commercial", ClassificationTemplateID: 10, ReleaseID: "new", TableKey: "b", TableName: "1kg", IsDefault: true},
		{ID: 4, CustomerID: 42, ListType: "green", ClassificationTemplateID: 20, IsDefault: true},
		{ID: 5, CustomerID: 43, ListType: "commercial", ClassificationTemplateID: 10, IsDefault: true},
	}
}

func TestNamedOrderTablesDefaultSelectionAndCurrentBatch(t *testing.T) {
	options := ApplyNamedPriceTableDefaults(namedOrderTableOptions())
	if !options[1].IsDefault || options[2].IsDefault {
		t.Fatalf("default table=%+v", options)
	}
	for _, tc := range []struct {
		ids  []int64
		want int64
		bad  bool
	}{{nil, 2, false}, {[]int64{3}, 3, false}, {[]int64{2, 3}, 0, true}, {[]int64{5}, 0, true}, {[]int64{999}, 0, true}, {[]int64{1}, 0, true}} {
		selected, err := ResolveOrderPriceTableSelection(options, 42, tc.ids, true)
		if tc.bad {
			if err == nil {
				t.Fatalf("ids=%v accepted", tc.ids)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if len(selected) != 2 || selected[0].ID != tc.want {
			t.Fatalf("ids=%v selected=%+v", tc.ids, selected)
		}
	}
	selected, err := ResolveOrderPriceTableSelection(options, 42, []int64{1}, false)
	if err != nil || selected[0].ID != 1 {
		t.Fatalf("ERP published historical selection=%+v err=%v", selected, err)
	}
}

func TestNamedOrderTablesOnlySelectedProductsAndPrices(t *testing.T) {
	options := ApplyNamedPriceTableDefaults(namedOrderTableOptions())
	products := []ProductOption{{ID: 10, CustomerID: 42, ProductKind: "roasted_bean", Tiers: []ProductTierOption{{PublicationID: 2, UnitPrice: 38}, {PublicationID: 3, UnitPrice: 118}}}, {ID: 11, CustomerID: 42, ProductKind: "roasted_bean", Tiers: []ProductTierOption{{PublicationID: 2, UnitPrice: 48}}}}
	got, err := FilterOrderProductsForSelectedPublications(products, 42, options, nil, []int64{3}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0].Tiers) != 1 || got[0].Tiers[0].PublicationID != 3 || got[0].Tiers[0].UnitPrice != 118 {
		t.Fatalf("catalog=%+v", got)
	}
}
