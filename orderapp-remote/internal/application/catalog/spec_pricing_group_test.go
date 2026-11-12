package catalog

import "testing"

func TestValidateSpecPricingGroup(t *testing.T) {
	good := SpecPricingGroup{Name: "小包装", SpecTemplateID: 1, SpecTemplateVersionID: 2, Active: true, Items: []SpecPricingGroupItem{{SpecKey: "227g", InventoryUnit: "袋", PricingRuleID: 5}}}
	if err := validateSpecPricingGroup(good); err != nil {
		t.Fatal(err)
	}
	cases := []SpecPricingGroup{good, good, good, good}
	cases[0].Name = ""
	cases[1].SpecTemplateVersionID = 0
	cases[2].Items = nil
	cases[3].Items = []SpecPricingGroupItem{{SpecKey: "x", InventoryUnit: "袋", PricingRuleID: 1}, {SpecKey: "X", InventoryUnit: "袋", PricingRuleID: 2}}
	for i, c := range cases {
		if validateSpecPricingGroup(c) == nil {
			t.Errorf("case %d: expected validation error", i)
		}
	}
}
