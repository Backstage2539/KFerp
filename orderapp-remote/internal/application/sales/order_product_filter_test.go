package sales

import "testing"

func TestCustomerScopedOrderFormKeepsExplicitPublicQuoteCandidates(t *testing.T) {
	products := []ProductOption{{ID: 1, Visibility: "public", Tiers: []ProductTierOption{{PublicationID: 12, ListType: "commercial"}}}, {ID: 1, CustomerID: 42, Visibility: "customer_reference", Tiers: []ProductTierOption{{PublicationID: 11, ListType: "commercial"}}}, {ID: 2, Visibility: "public", Tiers: []ProductTierOption{{PublicationID: 12, ListType: "commercial"}}}, {ID: 3, CustomerID: 43, Visibility: "customer_only", Tiers: []ProductTierOption{{PublicationID: 12, ListType: "commercial"}}}}
	options := []BeanListVersionOption{{ID: 11, CustomerID: 42, IsCustomerOwned: true, ListType: "commercial"}, {ID: 12, CustomerID: 0, ListType: "commercial"}}
	rows := FilterOrderProductsForCustomer(products, 42, options, []CustomerPublicUsageOption{{CustomerID: 42, UsePublicSKU: false}})
	if len(rows) != 2 {
		t.Fatalf("public choice candidates missing: %+v", rows)
	}
	for _, r := range rows {
		if r.ID == 1 && len(r.Tiers) != 2 {
			t.Fatalf("reference must retain customer and public quotes: %+v", r)
		}
	}
}
