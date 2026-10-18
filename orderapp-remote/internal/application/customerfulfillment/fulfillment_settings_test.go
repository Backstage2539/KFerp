package customerfulfillment

import "testing"

func TestCalculateFulfillmentFeesUsesModeAndRoundedContinuationWeight(t *testing.T) {
	settings := FulfillmentSettings{CustomerID: 1, Revision: 2, Modes: []FulfillmentModeSettings{{Mode: "stock", Enabled: true, WarehouseCode: "cust_1_finished", FeeConfigured: true, FixedFeeCents: 300, PerPackageFeeCents: 50, ShippingTemplateID: 8}}}
	template := ShippingFeeTemplate{ID: 8, Revision: 3, Name: "云南", Active: true, PackagingWeightG: 200, Rules: []ShippingFeeRegionRule{{Province: "云南省", FirstWeightG: 1000, FirstFeeCents: 800, AdditionalWeightG: 1000, AdditionalFeeCents: 200}}}
	quote, err := CalculateFulfillmentFees(settings, template, FulfillmentFeeQuoteCommand{CustomerID: 1, FulfillmentMode: "stock", Province: "云南省", City: "大理市"}, 12, 454*12)
	if err != nil || quote.ServiceFeeCents != 900 || quote.ShippingFeeCents != 1800 || quote.ChargeableWeightG != 5648 || quote.FeeQuoteToken == "" {
		t.Fatalf("quote=%+v err=%v", quote, err)
	}
	for _, grams := range []int64{800, 801, 1800, 1801} {
		q, e := CalculateFulfillmentFees(settings, template, FulfillmentFeeQuoteCommand{CustomerID: 1, FulfillmentMode: "stock", Province: "云南省"}, 1, grams)
		want := int64(800)
		if grams+200 > 1000 {
			want = 1000
		}
		if grams+200 > 2000 {
			want = 1200
		}
		if e != nil || q.ShippingFeeCents != want {
			t.Fatalf("weight=%d quote=%+v err=%v", grams, q, e)
		}
	}
}

func TestFulfillmentFeesRequireExplicitConfigurationAndTrustedAdjustmentReason(t *testing.T) {
	s := FulfillmentSettings{CustomerID: 1, Revision: 1, Modes: []FulfillmentModeSettings{{Mode: "on_demand", Enabled: true, FeeConfigured: true, ShippingTemplateID: 2}}}
	tm := ShippingFeeTemplate{ID: 2, Revision: 1, Active: true, Rules: []ShippingFeeRegionRule{{Province: "云南省", FirstWeightG: 1000, AdditionalWeightG: 1000}}}
	c := FulfillmentFeeQuoteCommand{CustomerID: 1, FulfillmentMode: "on_demand", Province: "云南省"}
	if _, err := CalculateFulfillmentFees(s, tm, c, 1, 454); err != nil {
		t.Fatal(err)
	}
	s.Modes[0].FeeConfigured = false
	if _, err := CalculateFulfillmentFees(s, tm, c, 1, 454); err == nil {
		t.Fatal("missing config accepted as free")
	}
	s.Modes[0].FeeConfigured = true
	c.Province = "四川省"
	if _, err := CalculateFulfillmentFees(s, tm, c, 1, 454); err == nil {
		t.Fatal("uncovered region accepted")
	}
	c.Province = "云南省"
	v := int64(100)
	c.ShippingOverrideCents = &v
	if _, err := CalculateFulfillmentFees(s, tm, c, 1, 454); err == nil {
		t.Fatal("adjustment without reason accepted")
	}
	c.AdjustmentReason = "实际运费"
	c.EmployeeAdjustment = true
	if q, err := CalculateFulfillmentFees(s, tm, c, 1, 454); err != nil || q.ShippingFeeCents != 100 {
		t.Fatalf("adjustment=%+v/%v", q, err)
	}
}

func TestShippingRulePrefersSpecificRegionAndRejectsDuplicateSelectors(t *testing.T) {
	tm := ShippingFeeTemplate{Name: "模板", Active: true, Rules: []ShippingFeeRegionRule{{Province: "*", FirstWeightG: 1000, AdditionalWeightG: 1000, FirstFeeCents: 2000}, {Province: "云南省", City: "大理市", FirstWeightG: 1000, AdditionalWeightG: 1000, FirstFeeCents: 1000}}}
	if err := ValidateShippingFeeTemplate(tm); err != nil {
		t.Fatal(err)
	}
	s := FulfillmentSettings{CustomerID: 1, Modes: []FulfillmentModeSettings{{Mode: "on_demand", Enabled: true, FeeConfigured: true}}}
	q, err := CalculateFulfillmentFees(s, tm, FulfillmentFeeQuoteCommand{CustomerID: 1, FulfillmentMode: "on_demand", Province: "云南省", City: "大理市"}, 1, 454)
	if err != nil || q.ShippingFeeCents != 1000 {
		t.Fatalf("quote=%+v err=%v", q, err)
	}
	tm.Rules = append(tm.Rules, tm.Rules[0])
	if ValidateShippingFeeTemplate(tm) == nil {
		t.Fatal("duplicate rules accepted")
	}
}
