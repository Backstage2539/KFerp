package customerportal

import (
	salesapp "orderapp/internal/application/sales"
	"testing"
)

func TestMiniEmployeeExplicitPublicPublication(t *testing.T) {
	form := salesapp.OrderFormData{BeanListVersionOptions: []salesapp.BeanListVersionOption{{ID: 11, CustomerID: 42, IsCustomerOwned: true, IsDefault: true, ListType: "commercial"}, {ID: 12, CustomerID: 0, IsDefault: true, ListType: "commercial"}, {ID: 13, CustomerID: 43, IsCustomerOwned: true, IsDefault: true, ListType: "commercial"}}, Products: []salesapp.ProductOption{{ID: 1, Visibility: "public", Tiers: []salesapp.ProductTierOption{{PublicationID: 12, ListType: "commercial"}}}, {ID: 1, CustomerID: 42, Visibility: "customer_reference", Tiers: []salesapp.ProductTierOption{{PublicationID: 11, ListType: "commercial"}}}}}
	selected, e := miniEmployeeSelectPublications(form, 42, []int64{12})
	if e != nil {
		t.Fatal(e)
	}
	products := salesapp.FilterOrderProductsForDefaultPublications(selected.Products, 42, selected.BeanListVersionOptions, selected.CustomerPublicUsages)
	if len(products) != 1 || len(products[0].Tiers) != 1 || products[0].Tiers[0].PublicationID != 12 {
		t.Fatalf("public quote not available %+v", products)
	}
	if _, e = miniEmployeeSelectPublications(form, 42, []int64{13}); e == nil {
		t.Fatal("cross customer price allowed")
	}
	if _, e = miniEmployeeSelectPublications(form, 42, []int64{999}); e == nil {
		t.Fatal("unknown/unpublished price allowed")
	}
}
