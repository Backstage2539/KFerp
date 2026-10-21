package sales

import "testing"

func TestClassifiedSelectedPublicationOverridesLegacyOrderTypeRouting(t *testing.T) {
	products := []ProductOption{
		{ID: 1, Visibility: "public", ProductKind: "roasted", Tiers: []ProductTierOption{{ID: 11, PublicationID: 158, ListType: "commercial", UnitPrice: 38}}},
		{ID: 2, Visibility: "public", ProductKind: "green_bean", Tiers: []ProductTierOption{{ID: 21, PublicationID: 73, ListType: "green", UnitPrice: 68}}},
		{ID: 3, Visibility: "public", ProductKind: "generic", Tiers: []ProductTierOption{{ID: 31, PublicationID: 113, ListType: "commercial", UnitPrice: 18}}},
		{ID: 4, Visibility: "public", ProductKind: "roasted", Tiers: []ProductTierOption{{ID: 41, PublicationID: 157, ListType: "commercial", UnitPrice: 30}}},
	}
	options := []BeanListVersionOption{
		{ID: 158, CustomerID: 363, ClassificationTemplateID: 56, ListType: "commercial", IsDefault: true},
		{ID: 73, CustomerID: 363, ClassificationTemplateID: 162, ListType: "green", IsDefault: true},
		{ID: 113, CustomerID: 363, ClassificationTemplateID: 464, ListType: "commercial", IsDefault: true},
	}
	for _, retail := range []bool{false, true} {
		got := FilterOrderProductsForDefaultPublications(products, 363, options, nil, retail)
		if len(got) != 3 {
			t.Fatalf("retail=%t: selected published roasted/green/custom products must remain available; got=%+v", retail, got)
		}
		for _, p := range got {
			if p.ID == 4 {
				t.Fatal("unselected historical publication leaked")
			}
		}
	}
}

func TestOrderCatalogReadRebasesSameNamedTableAfterPublishButSaveSelectionRemainsStrict(t *testing.T) {
	form := OrderFormData{Products: []ProductOption{{ID: 1, Visibility: "public", Tiers: []ProductTierOption{{PublicationID: 12, UnitPrice: 48}}}}, BeanListVersionOptions: []BeanListVersionOption{
		{ID: 10, ClassificationTemplateID: 56, ListType: "commercial", ReleaseID: "old", TableKey: "large"},
		{ID: 11, ClassificationTemplateID: 56, ListType: "commercial", ReleaseID: "new", TableKey: "small", IsDefault: true},
		{ID: 12, ClassificationTemplateID: 56, ListType: "commercial", ReleaseID: "new", TableKey: "large"},
	}}
	read, err := ResolveOrderCatalog(form, 487, []int64{10}, true)
	if err != nil || len(read.SelectedPriceTableIDs) != 1 || read.SelectedPriceTableIDs[0] != 12 {
		t.Fatalf("new release must retain named-table choice: %+v err=%v", read, err)
	}
	if _, err := ResolveOrderPriceTableSelection(form.BeanListVersionOptions, 487, []int64{10}, true); err == nil {
		t.Fatal("submitting stale prices must still fail")
	}
}

func TestOrderCatalogReadDoesNotReplaceRemovedNamedTableWithAnotherTable(t *testing.T) {
	form := OrderFormData{BeanListVersionOptions: []BeanListVersionOption{
		{ID: 10, ClassificationTemplateID: 56, ListType: "commercial", ReleaseID: "old", TableKey: "large"},
		{ID: 11, ClassificationTemplateID: 56, ListType: "commercial", ReleaseID: "new", TableKey: "small", IsDefault: true},
	}}
	if _, err := ResolveOrderCatalog(form, 487, []int64{10}, true); err == nil {
		t.Fatal("removed named table must require a manual choice, not silently load another table")
	}
}
