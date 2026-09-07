package customerportal

import (
	app "orderapp/internal/application/customerportal"
	salesapp "orderapp/internal/application/sales"
	"testing"
)

func TestCustomerNamedPriceTableCatalogIsolation(t *testing.T) {
	products := []app.ProductSummary{{ID: 10, BomSpecID: 227, BomVariantID: 1}, {ID: 10, BomSpecID: 1000, BomVariantID: 2}, {ID: 11, BomSpecID: 227, BomVariantID: 3}}
	tables := []salesapp.BeanListVersionOption{{ID: 21, TableName: "227g", ListType: "commercial"}, {ID: 22, TableName: "1kg", ListType: "commercial"}}
	snapshots := map[int64][]byte{21: []byte(`{"price_rows":[{"product_id":10,"bom_spec_id":227,"bom_variant_id":1,"min_qty":1,"final_unit_price":30,"inventory_unit":"袋"}]}`), 22: []byte(`{"price_rows":[{"product_id":10,"bom_spec_id":1000,"bom_variant_id":2,"min_qty":1,"final_unit_price":90,"inventory_unit":"袋"}]}`)}
	first := filterPortalSelectedPriceTableProducts(products, tables[:1], snapshots)
	second := filterPortalSelectedPriceTableProducts(products, tables[1:], snapshots)
	if len(first) != 1 || first[0].BomSpecID != 227 || first[0].BeanListPublicationID != 21 || first[0].DefaultPrice != "30.00" {
		t.Fatalf("wrong first table: %+v", first)
	}
	if len(second) != 1 || second[0].BomSpecID != 1000 || second[0].BeanListPublicationID != 22 || second[0].DefaultPrice != "90.00" {
		t.Fatalf("wrong second table: %+v", second)
	}
	if products[0].BeanListPublicationID != 0 {
		t.Fatal("catalog was mutated")
	}
}
