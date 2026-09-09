package sales

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	salesapp "orderapp/internal/application/sales"
)

func TestOrderBOMSpecPublishedPriceUsesSelectedGreenTablePostgres(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
		pool.Close()
	}()
	_, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %[1]s.bean_list_publications (
			id BIGINT PRIMARY KEY, version_no TEXT, list_type TEXT,
			owner_type TEXT, owner_key TEXT, status TEXT, publication_purpose TEXT,
			content_json JSONB
		);
		INSERT INTO %[1]s.bean_list_publications VALUES
			(53,'V3.0.14','green','official','','published','factory_supply',
			'{"price_rows":[
				{"product_id":911,"bom_spec_id":269,"bom_variant_id":411,"min_qty":1,"max_qty":59,"final_unit_price":98,"price_unit":"kg","quantity_basis":"sales_spec_count"},
				{"product_id":911,"bom_spec_id":269,"bom_variant_id":411,"min_qty":60,"final_unit_price":96,"price_unit":"kg","quantity_basis":"sales_spec_count"}
			]}'),
			(54,'OTHER-CUSTOMER','green','customer','43','published','factory_supply',
			'{"price_rows":[{"product_id":911,"bom_spec_id":269,"bom_variant_id":411,"min_qty":1,"final_unit_price":1}]}');
	`, schema))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	// Product archives can retain the historical roasted kind after the product
	// has been published in a green table. Save must use the selected snapshot.
	identity := orderBOMSpecIdentity{
		ProductID: 911, ProductKind: "roasted", BomSpecID: 269, BomVariantID: 411,
		BomSpecName: "1KG", InventoryUnit: "kg",
	}
	cmd := salesapp.SaveOrderCommand{CustomerID: 42, GreenBeanListPublicationID: 53}
	for _, tc := range []struct {
		name      string
		qty       int64
		retail    bool
		wantPrice float64
	}{
		{"three kilograms", 3, false, 98},
		{"first tier upper boundary", 59, false, 98},
		{"second tier", 60, false, 96},
		{"retail order selecting green table", 3, true, 98},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sourceType := orderBeanListTypeFromPriceSource(`{"publication_id":53,"list_type":"green"}`)
			usage, pricing, listType, err := resolveOrderBOMSpecPublishedPriceTx(ctx, tx, schema, cmd, identity, 53, sourceType, tc.retail, tc.qty, 0)
			if err != nil {
				t.Fatalf("save pricing for %d kg in selected green table: %v", tc.qty, err)
			}
			if usage.PublicationID != 53 || usage.VersionNo != "V3.0.14" || listType != "green" || pricing.UnitPrice != tc.wantPrice || pricing.QuantityBasis != "sales_spec_count" {
				t.Fatalf("usage=%+v pricing=%+v listType=%q", usage, pricing, listType)
			}
			if tc.qty == 3 && pricing.UnitPrice*float64(tc.qty) != 294 {
				t.Fatalf("3 kg total = %v, want 294", pricing.UnitPrice*float64(tc.qty))
			}
			var source map[string]any
			raw := withOrderBOMSpecPriceSourceJSON(beanListPriceSourceJSONWithPricing(listType, usage, identity.ProductID, pricing), identity)
			if err := json.Unmarshal([]byte(raw), &source); err != nil {
				t.Fatal(err)
			}
			if source["list_type"] != "green" || source["publication_id"] != float64(53) || source["bom_spec_id"] != float64(269) || source["price_unit"] != "kg" {
				t.Fatalf("saved price source = %s", raw)
			}
		})
	}
	for _, tc := range []struct {
		name          string
		publicationID int64
		sourceType    string
		specID        int64
	}{
		{"other customer", 54, "green", 269},
		{"wrong list type", 53, "commercial", 269},
		{"unpublished spec", 53, "green", 270},
		{"missing publication", 999, "green", 269},
	} {
		t.Run(tc.name, func(t *testing.T) {
			invalid := identity
			invalid.BomSpecID = tc.specID
			_, _, _, err := resolveOrderBOMSpecPublishedPriceTx(ctx, tx, schema, cmd, invalid, tc.publicationID, tc.sourceType, false, 3, 0)
			if err == nil {
				t.Fatal("invalid selected price source must remain rejected")
			}
		})
	}
}
