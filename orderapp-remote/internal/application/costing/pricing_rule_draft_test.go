package costing

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	domain "orderapp/internal/domain/costing"
)

func draftTrialRepo() *fakeRepo {
	return &fakeRepo{
		inputs:       []domain.ProductInput{{ProductID: 10, Name: "商品", InventoryUnit: "kg", QuoteUnit: "kg", BomVersionID: 50, BomStatus: "active", YieldRate: 1}},
		costDetails:  []PricingRuleTrialBaseCostDetail{{Key: "material:10", Type: "material", Name: "原料", ConsumeUnit: "ratio_pct", RatioPct: 100, UnitCost: 10.4, AmountPerKg: 10.4, Unit: "kg"}},
		pricingRules: map[int64]ProductPricingRule{7: {ID: 7, Name: "原模板", Active: true, MarginRate: .5, TaxRate: .1, RoundingMode: "none", CalculationJSON: map[string]any{"profit_method": "markup", "tax_mode": "tax_excluded", "other_costs": map[string]any{"包装": 9.0}}}},
	}
}

const draftTrialJSON = `{"pricing_rule_id":7,"product_id":10,"bom_version_id":50,"pricing_rule_draft":{"id":7,"name":"未保存模板","active":true,"margin_rate":0,"tax_rate":0,"rounding_mode":"yuan","calculation_json":{"profit_method":"markup","tax_mode":"none","other_costs":{},"minimum_margin_rate":0.8}}}`

func decodeDraftTrial(t *testing.T, body string) PricingRuleTrialCommand {
	t.Helper()
	var cmd PricingRuleTrialCommand
	if err := json.Unmarshal([]byte(body), &cmd); err != nil {
		t.Fatal(err)
	}
	return cmd
}

func TestPricingRuleDraftUsesCompleteUnsavedConfigurationWithoutWriting(t *testing.T) {
	repo := draftTrialRepo()
	before, _ := json.Marshal(repo.pricingRules)
	got, err := NewService(repo).PricingRuleTrial(context.Background(), decodeDraftTrial(t, draftTrialJSON))
	if err != nil {
		t.Fatal(err)
	}
	if got.OtherCostTotal != 0 || got.ProfitMarkupAmount != 0 || got.TaxAmount != 0 || got.FinalUnitPrice != pricingRuleTrialRoundedPrice(10.4, "yuan") || got.MinimumMarginRate != .8 {
		t.Fatalf("draft not used: %+v", got)
	}
	after, _ := json.Marshal(repo.pricingRules)
	if string(before) != string(after) || len(repo.savedItems) != 0 || repo.publishedID != 0 {
		t.Fatal("trial wrote business data")
	}
	var wire struct {
		Draft ProductPricingRule `json:"pricing_rule_draft"`
	}
	_ = json.Unmarshal([]byte(draftTrialJSON), &wire)
	repo.pricingRules[7] = wire.Draft
	saved, err := NewService(repo).PricingRuleTrial(context.Background(), PricingRuleTrialCommand{PricingRuleID: 7, ProductID: 10, BomVersionID: 50})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, saved) {
		t.Fatalf("draft and persisted configuration differ: draft=%+v saved=%+v", got, saved)
	}
}

func TestPricingRuleDraftRejectsInvalidOrAmbiguousConfiguration(t *testing.T) {
	cases := []struct{ name, body string }{
		{"overrides", strings.Replace(draftTrialJSON, `"product_id":10`, `"product_id":10,"overrides":{"margin_rate":0}`, 1)},
		{"empty override map", strings.Replace(draftTrialJSON, `"product_id":10`, `"product_id":10,"overrides":{}`, 1)},
		{"negative rate", strings.Replace(draftTrialJSON, `"margin_rate":0`, `"margin_rate":-1`, 1)},
		{"quantity tiers", strings.Replace(draftTrialJSON, `"other_costs":{}`, `"other_costs":{},"tiers":[]`, 1)},
		{"legacy draft", strings.Replace(draftTrialJSON, `"markup"`, `"fixed_add"`, 1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewService(draftTrialRepo()).PricingRuleTrial(context.Background(), decodeDraftTrial(t, tc.body))
			if err == nil {
				t.Fatal("expected invalid draft to be rejected")
			}
		})
	}
	repo := draftTrialRepo()
	rule := repo.pricingRules[7]
	rule.CalculationJSON["profit_method"] = "fixed_add"
	repo.pricingRules[7] = rule
	if _, err := NewService(repo).PricingRuleTrial(context.Background(), decodeDraftTrial(t, draftTrialJSON)); err == nil {
		t.Fatal("draft bypassed quarantined persisted template")
	}
}

func TestPricingRuleDraftRejectedByBatch(t *testing.T) {
	if _, err := NewService(draftTrialRepo()).PricingRuleTrialBatch(context.Background(), []PricingRuleTrialCommand{decodeDraftTrial(t, draftTrialJSON)}); err == nil {
		t.Fatal("batch must reject draft template")
	}
}
