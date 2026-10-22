package orderbeans

import "testing"

func TestPublishedFixedBOMSpecPricingRejectsAmbiguousAndBoundedPrices(t *testing.T) {
	for _, tc := range []struct {
		rows string
		ok   bool
	}{{`{"product_id":7,"bom_spec_id":19,"bom_variant_id":1,"min_qty":1,"final_unit_price":30}`, true}, {`{"product_id":7,"bom_spec_id":19,"min_qty":2,"final_unit_price":30}`, false}, {`{"product_id":7,"bom_spec_id":19,"min_qty":1,"max_qty":2,"final_unit_price":30}`, false}, {`{"product_id":7,"bom_spec_id":19,"min_qty":1,"final_unit_price":30},{"product_id":7,"bom_spec_id":19,"min_qty":1,"final_unit_price":40}`, false}} {
		p, ok := PublishedFixedBOMSpecPricing([]byte(`{"price_rows":[`+tc.rows+`]}`), 7, 19, "commercial")
		if ok != tc.ok || ok && p.UnitPrice != 30 {
			t.Fatalf("rows%s ok%v p%+v", tc.rows, ok, p)
		}
	}
}
func TestPublishedFixedBOMSpecPricingAcceptsGroupedTierSpecIdentity(t *testing.T) {
	for _, row := range []string{`{"bom_spec_id":19,"effective_sales_spec":{"bom_spec_id":19,"bom_variant_id":30},"min_qty":1,"final_unit_price":30}`, `{"effective_sales_spec":{"bom_spec_id":19,"bom_variant_id":30},"min_qty":1,"final_unit_price":30}`} {
		raw := []byte(`{"groups":[{"items":[{"product_id":7,"commercial_wholesale_tiers":[` + row + `]}]}]}`)
		p, ok := PublishedFixedBOMSpecPricing(raw, 7, 19, "commercial")
		if !ok || p.UnitPrice != 30 {
			t.Fatalf("grouped catalog spec rejected row%s p%+v ok%v", row, p, ok)
		}
		if _, ok = PublishedFixedBOMSpecPricing(raw, 7, 20, "commercial"); ok {
			t.Fatal("other stable spec took grouped price")
		}
	}
}
