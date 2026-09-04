package sales

import (
	"testing"

	salesapp "orderapp/internal/application/sales"
	postgresinfra "orderapp/internal/infrastructure/postgres"
)

func TestOrderProductSpecIdentityKeepsLegacyPublicParentPublicationPricing(t *testing.T) {
	products := []salesapp.ProductOption{
		{ID: 1043, ParentProductID: 1043, Visibility: "public", ProductKind: "roasted_bean"},
		{ID: 1035, Visibility: "public", ProductKind: "roasted_bean"},
	}
	selected := make([]salesapp.ProductOption, 0, len(products))
	for _, product := range products {
		if orderProductSpecIdentitySelectable(product, postgresinfra.ProductSpecIdentityOption{LegacyCatalogProduct: true}) {
			selected = append(selected, product)
		}
	}
	applyCommercialOrderPublicationTiers(selected, map[orderPublicationProductKey][]salesapp.ProductTierOption{
		{ProductID: 1043}: {{ID: 1100001, PublicationID: 110, UnitPrice: 65}},
		{ProductID: 1035}: {{ID: 1100002, PublicationID: 110, UnitPrice: 53}},
	})
	if len(selected) != 2 {
		t.Fatalf("legacy public parents available for published pricing = %d, want 2", len(selected))
	}
	for _, product := range selected {
		if len(product.Tiers) != 1 || product.Tiers[0].PublicationID != 110 || product.Tiers[0].UnitPrice <= 0 {
			t.Fatalf("public parent %d lost published pricing: %+v", product.ID, product.Tiers)
		}
		if product.BomSpecAuthoritative {
			t.Fatalf("legacy public parent %d was incorrectly marked BOM authoritative", product.ID)
		}
	}
}

func TestOrderProductSpecIdentityStillRequiresConfiguredCustomerParent(t *testing.T) {
	for _, tc := range []struct {
		name          string
		product       salesapp.ProductOption
		authoritative bool
		newProduct    bool
		want          bool
	}{
		{name: "unconfigured customer product", product: salesapp.ProductOption{ID: 1063, ParentProductID: 1063, CustomerID: 152, Visibility: "customer_only"}},
		{name: "configured customer product", product: salesapp.ProductOption{ID: 1063, ParentProductID: 1063, CustomerID: 152, Visibility: "customer_only"}, authoritative: true, want: true},
		{name: "unconfigured customer reference", product: salesapp.ProductOption{ID: 1043, ParentProductID: 1043, CustomerID: 152, Visibility: "customer_alias"}},
		{name: "legacy public child", product: salesapp.ProductOption{ID: 1044, ParentProductID: 1043, Visibility: "public"}},
		{name: "configured public child", product: salesapp.ProductOption{ID: 1044, ParentProductID: 1043, Visibility: "public"}, authoritative: true},
		{name: "nonpublic unconfigured parent", product: salesapp.ProductOption{ID: 1063, Visibility: "customer_only"}},
		{name: "new public unconfigured parent", product: salesapp.ProductOption{ID: 1064, Visibility: "public"}, newProduct: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := orderProductSpecIdentitySelectable(tc.product, postgresinfra.ProductSpecIdentityOption{BomSpecAuthoritative: tc.authoritative, LegacyCatalogProduct: !tc.newProduct}); got != tc.want {
				t.Fatalf("selectable = %v, want %v", got, tc.want)
			}
		})
	}
}
