package sales

import (
	"encoding/json"
	salesapp "orderapp/internal/application/sales"
	"orderapp/internal/infrastructure/postgres/orderbeans"
	"strings"
	"testing"
)

func TestBOMTierSourceUsesExactPublishedRule(t *testing.T) {
	request := salesapp.SpecConversionQuoteRequest{ProductID: 7, PublicationID: 88, PriceRowKey: orderbeans.PublishedPriceRowKey(88, "commercial", 7, 9002, 9102, "flat", 0), SpecConversionSelection: salesapp.SpecConversionSelection{SourceSpecID: 9002, SourceVariantID: 9102}}
	raw := []byte(`{"price_rows":[{"product_id":7,"bom_spec_id":9002,"bom_variant_id":9102,"final_unit_price":48,"price_unit":"袋","inventory_unit":"袋","quantity_basis":"sales_spec_count","tier_label":"第二档","pricing_mode":"tier_template","pricing_rule_id":8,"tier_pricing_rule_id":8,"pricing_rule_snapshot":{"id":8,"name":"第二档模板","margin_rate":0.2,"active":true,"rounding_mode":"cent","formula_version":"v1"}}]}`)
	row, err := bomTierSource(raw, "commercial", request)
	if err != nil || row.Rule == nil || row.Rule.ID != 8 || row.Rule.MarginRate != 0.2 {
		t.Fatalf("source: %+v %v", row, err)
	}
	wrong := request
	wrong.PriceRowKey = "forged"
	if _, err = bomTierSource(raw, "commercial", wrong); err == nil {
		t.Fatal("forged key accepted")
	}
	var data map[string]any
	_ = json.Unmarshal(raw, &data)
	r := data["price_rows"].([]any)[0].(map[string]any)
	r["manual_adjusted"] = true
	b, _ := json.Marshal(data)
	row, err = bomTierSource(b, "commercial", request)
	if err != nil || !row.ManualAdjusted {
		t.Fatal("manual price marker lost")
	}
	delete(r, "pricing_rule_snapshot")
	b, _ = json.Marshal(data)
	row, err = bomTierSource(b, "commercial", request)
	if err != nil || row.Rule != nil {
		t.Fatal("must not infer a missing rule")
	}
	if !strings.Contains(string(raw), "第二档") {
		t.Fatal("fixture")
	}
}
