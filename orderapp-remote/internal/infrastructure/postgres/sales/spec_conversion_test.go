package sales

import (
	salesapp "orderapp/internal/application/sales"
	"orderapp/internal/infrastructure/postgres/orderbeans"
	"testing"
)

func TestConversionSourceTierRequiresExplicitSpecUnit(t *testing.T) {
	raw := []byte(`{"price_rows":[{"product_id":10,"bom_spec_id":101,"bom_variant_id":1001,"quantity_basis":"sales_spec_count","final_unit_price":48,"price_unit":"袋","inventory_unit":"袋","tier_label":"48袋以上"}]}`)
	request := salesapp.SpecConversionQuoteRequest{ProductID: 10, PublicationID: 8, SpecConversionSelection: salesapp.SpecConversionSelection{SourceSpecID: 101, SourceVariantID: 1001}, PriceRowKey: orderbeans.PublishedPriceRowKey(8, "commercial", 10, 101, 1001, "flat", 0)}
	price, label, err := conversionSourceTier(raw, "commercial", request, "袋")
	if err != nil || price != "48" || label != "48袋以上" {
		t.Fatalf("got %s %s %v", price, label, err)
	}
	request.ProductID = 11
	if _, _, err := conversionSourceTier(raw, "commercial", request, "袋"); err == nil {
		t.Fatal("cross-product tier accepted")
	}
	request.ProductID = 10
	if _, _, err := conversionSourceTier(raw, "commercial", request, "盒"); err == nil {
		t.Fatal("unit mismatch accepted")
	}
	if _, _, err := conversionSourceTier([]byte(`{"price_rows":[{"product_id":10,"bom_spec_id":101,"bom_variant_id":1001,"price_per_lb":48,"spec_g":454}]}`), "commercial", request, "袋"); err == nil {
		t.Fatal("implicit weight fallback accepted")
	}
}
