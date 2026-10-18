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

func TestExactCustomerOrderTablesDoNotBackfillUnspecifiedProductTypes(t *testing.T) {
	options := ApplyNamedPriceTableDefaults(namedOrderTableOptions())
	for index := 0; index < 3; index++ {
		options[index].IsCustomerOwned = true
	}
	products := []ProductOption{
		{ID: 10, CustomerID: 42, ProductKind: "roasted_bean", Tiers: []ProductTierOption{{PublicationID: 2, UnitPrice: 38}}},
		{ID: 20, CustomerID: 42, ProductKind: "green_bean", Tiers: []ProductTierOption{{PublicationID: 4, UnitPrice: 16}}},
	}
	got, err := FilterOrderProductsForExactCustomerPublications(products, 42, options, nil, []int64{2}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 10 || len(got[0].Tiers) != 1 || got[0].Tiers[0].PublicationID != 2 {
		t.Fatalf("exact catalog leaked an unbound default table: %+v", got)
	}

	if _, err := FilterOrderProductsForExactCustomerPublications(products, 42, options, nil, []int64{1, 2}, false); err == nil {
		t.Fatal("two versions for one product type must not be accepted")
	}
}

func TestCustomerDefaultAndExplicitPublicNamedTable(t *testing.T) {
	options := []BeanListVersionOption{{ID: 11, CustomerID: 42, IsCustomerOwned: true, IsDefault: true, ListType: "commercial", ReleaseID: "C1", IsDefaultTable: true}, {ID: 12, CustomerID: 0, IsDefault: true, ListType: "commercial", ReleaseID: "P1", IsDefaultTable: true}, {ID: 14, CustomerID: 0, ListType: "commercial", ReleaseID: "P1"}, {ID: 13, CustomerID: 43, IsCustomerOwned: true, IsDefault: true, ListType: "commercial"}}
	defaults, e := ResolveOrderPriceTableSelection(options, 42, nil, true)
	if e != nil || len(defaults) != 1 || defaults[0].ID != 11 {
		t.Fatal(defaults, e)
	}
	selected, e := ResolveOrderPriceTableSelection(options, 42, []int64{14}, true)
	if e != nil || len(selected) != 1 || selected[0].ID != 14 || selected[0].CustomerID != 42 {
		t.Fatal(selected, e)
	}
	choices := CurrentOrderPriceTableOptions(options, 42)
	if len(choices) != 3 {
		t.Fatal(choices)
	}
	if _, e = ResolveOrderPriceTableSelection(options, 42, []int64{13}, true); e == nil {
		t.Fatal("other customer price table allowed")
	}
}

func TestExactBoundPublicTableAllowedWithoutDefaultsOrOtherCustomers(t *testing.T) {
	options := []BeanListVersionOption{
		{ID: 91, CustomerID: 0, IsCustomerOwned: false, ClassificationTemplateID: 10},
		{ID: 92, CustomerID: 43, IsCustomerOwned: true, ClassificationTemplateID: 10},
		{ID: 93, CustomerID: 42, IsCustomerOwned: true, ClassificationTemplateID: 20, IsDefault: true},
	}
	rows, err := ResolveExactCustomerOrderPriceTableSelection(options, 42, []int64{91})
	if err != nil || len(rows) != 1 || rows[0].ID != 91 || rows[0].CustomerID != 42 {
		t.Fatalf("bound public=%+v error=%v", rows, err)
	}
	if _, err := ResolveExactCustomerOrderPriceTableSelection(options, 42, []int64{92}); err == nil {
		t.Fatal("another customer's publication accepted")
	}
}

func TestExactBoundPublicCatalogKeepsCustomerReferenceAndOnlyBoundSpecifications(t *testing.T) {
	options := []BeanListVersionOption{{ID: 158, CustomerID: 0, ListType: "commercial", ClassificationTemplateID: 56}, {ID: 162, CustomerID: 74, IsCustomerOwned: true, ListType: "commercial", ClassificationTemplateID: 56}}
	products := []ProductOption{
		{ID: 58, Visibility: "public", ProductKind: "roasted_bean", Tiers: []ProductTierOption{{PublicationID: 158, ListType: "commercial", BomSpecID: 69, UnitPrice: 47}, {PublicationID: 159, ListType: "commercial", BomSpecID: 70, UnitPrice: 50}}},
		{ID: 58, CustomerID: 74, Visibility: "customer_reference", ProductKind: "roasted_bean", Tiers: []ProductTierOption{{PublicationID: 162, ListType: "commercial", BomSpecID: 69, UnitPrice: 49}}},
		{ID: 59, CustomerID: 75, Visibility: "customer_only", ProductKind: "roasted_bean", Tiers: []ProductTierOption{{PublicationID: 158, ListType: "commercial", BomSpecID: 71, UnitPrice: 50}}},
	}
	got, err := FilterOrderProductsForExactCustomerPublications(products, 74, options, []CustomerPublicUsageOption{{CustomerID: 74, UsePublicSKU: false}}, []int64{158}, false)
	if err != nil || len(got) != 1 || got[0].ID != 58 || len(got[0].Tiers) != 1 || got[0].Tiers[0].PublicationID != 158 || got[0].Tiers[0].BomSpecID != 69 {
		t.Fatalf("bound public reference catalog=%+v error=%v", got, err)
	}
}

func TestExplicitPublicBindingGrantsOnlyPublishedPublicRows(t *testing.T) {
	products := []ProductOption{{ID: 60, Visibility: "public", ProductKind: "roasted_bean", Tiers: []ProductTierOption{{PublicationID: 158, ListType: "commercial", BomSpecID: 69, UnitPrice: 47}, {PublicationID: 159, ListType: "commercial", BomSpecID: 70, UnitPrice: 48}}}}
	options := []BeanListVersionOption{{ID: 158, CustomerID: 0, ListType: "commercial", ClassificationTemplateID: 56}}
	rows, err := FilterOrderProductsForExactCustomerPublications(products, 74, options, []CustomerPublicUsageOption{{CustomerID: 74, UsePublicSKU: false}}, []int64{158}, false)
	if err != nil || len(rows) != 1 || len(rows[0].Tiers) != 1 || rows[0].Tiers[0].PublicationID != 158 {
		t.Fatalf("explicit public authorization=%+v error=%v", rows, err)
	}
}
