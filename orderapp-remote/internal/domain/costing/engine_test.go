package costing

import (
	"strings"
	"testing"
)

func assertClose(t *testing.T, name string, got, want float64) {
	t.Helper()
	if got < want-0.0001 || got > want+0.0001 {
		t.Fatalf("%s = %.12f, want %.12f", name, got, want)
	}
}

func TestEngineMatchesExcelCachedGoldens(t *testing.T) {
	params := DefaultParameters()
	input := ProductInput{
		ProductID:                 1,
		Name:                      "单品：孟连水洗（SOE)",
		GreenBeanCostPerKg:        62,
		YieldRate:                 params.RoastYieldRate,
		WholesaleTaxAddPerKg:      1.3622427631578853,
		WholesaleTaxAddPerKgTiers: []float64{1.3622427631578853, 0, 0, 0.37693125000000355},
		DripTaxAddPerBag100:       0.04959450000000043,
		DripTaxAddPerBagRetail:    0.06199312500000055,
		WholesaleKgMarginRates:    []float64{0.5421052631578949, 0.3842105263157895, 0.27894736842105267, 0.2},
		WholesaleDripMultipliers:  []float64{2.2, 1.8, 1.6, 1.5},
	}

	got := CalculateProduct(params, input)

	assertClose(t, "roasted bean cost", got.RoastedBeanCostPerKg, 77.5)
	assertClose(t, "small batch cost", got.SmallBatchCostPerKg, 83.7625)
	assertClose(t, "drip base cost", got.DripBaseCostPerBag, 1.377625)
	assertClose(t, "retail tax", got.RetailTaxPerKg, 3.64366875)
	assertClose(t, "wholesale kg 1kg tier", got.WholesaleKgPrices[0], 132)
	assertClose(t, "wholesale lb 1kg tier", got.WholesaleLbPrices[0], 61)
	assertClose(t, "wholesale kg 24kg tier", got.WholesaleKgPrices[3], 99)
	assertClose(t, "wholesale drip 100", got.WholesaleDripBagPrices[0], 3)
	assertClose(t, "wholesale drip with pack 100", got.WholesaleDripBagWithPackPrices[0], 3)
	assertClose(t, "retail kg", got.RetailKgPrice, 219)
	assertClose(t, "retail lb", got.Retail454gPrice, 99)
	assertClose(t, "retail half lb", got.Retail227gPrice, 50)
	assertClose(t, "retail drip 10 bags", got.RetailDrip10BagPrice, 43)
}

func TestCommercialWholesaleTiersUse454gPackageRanges(t *testing.T) {
	params := DefaultParameters()
	got := CalculateProduct(params, ProductInput{
		ProductID:          1,
		Name:               "单品：孟连水洗（SOE)",
		GreenBeanCostPerKg: 62,
		YieldRate:          params.RoastYieldRate,
		WholesaleTaxAddPerKgTiers: []float64{
			1.3622427631578853,
			0,
			0,
			0.37693125000000355,
		},
	})
	if len(got.CommercialWholesaleTiers) != 4 {
		t.Fatalf("commercial tiers = %+v, want 4 tiers", got.CommercialWholesaleTiers)
	}
	wantLabels := []string{"2包-13包", "14包-23包", "24包-47包", "48包+"}
	wantMins := []float64{2, 14, 24, 48}
	wantMaxs := []*float64{f64(13), f64(23), f64(47), nil}
	for i := range wantLabels {
		tier := got.CommercialWholesaleTiers[i]
		if tier.Label != wantLabels[i] || tier.SpecG != 454 || tier.MinQty != wantMins[i] || tier.MinLb != wantMins[i] {
			t.Fatalf("tier %d = %+v", i, tier)
		}
		if (tier.MaxLb == nil) != (wantMaxs[i] == nil) {
			t.Fatalf("tier %d max = %+v, want %+v", i, tier.MaxLb, wantMaxs[i])
		}
		if tier.MaxLb != nil && *tier.MaxLb != *wantMaxs[i] {
			t.Fatalf("tier %d max = %+v, want %+v", i, *tier.MaxLb, *wantMaxs[i])
		}
	}
	assertClose(t, "2-13 lb price", got.CommercialWholesaleTiers[0].PricePerLb, 61)
	assertClose(t, "greater than 47 lb price", got.CommercialWholesaleTiers[3].PricePerLb, 46)
}

func TestNenkaExcelCommercialProfileMatchesWorkbook(t *testing.T) {
	params := DefaultParameters()
	input := ApplyExcelCommercialPricingProfile(params, ProductInput{
		ProductID:          28,
		Name:               "Nenka嫩咖",
		GreenBeanCostPerKg: 116,
		YieldRate:          0.8,
	})

	got := CalculateProduct(params, input)

	if got.CommercialWholesaleTiers[0].Label != "2包-13包" || got.CommercialWholesaleTiers[0].SpecG != 454 {
		t.Fatalf("first tier = %+v", got.CommercialWholesaleTiers[0])
	}
	assertClose(t, "Nenka 2-13 bags", got.CommercialWholesaleTiers[0].PricePerUnit, 127)
	assertClose(t, "Nenka 14-23 bags", got.CommercialWholesaleTiers[1].PricePerUnit, 111)
	assertClose(t, "Nenka 24-47 bags", got.CommercialWholesaleTiers[2].PricePerUnit, 98)
	assertClose(t, "Nenka 48+ bags", got.CommercialWholesaleTiers[3].PricePerUnit, 90)
}

func TestCommercialWholesaleTiersSupportExcelSchemes(t *testing.T) {
	params := DefaultParameters()

	kg := CalculateProduct(params, ProductInput{
		ProductID:           36,
		Name:                "曲奇拼配",
		GreenBeanCostPerKg:  38.3,
		YieldRate:           0.8,
		WholesaleTierScheme: WholesaleTierSchemeKgThree,
	})
	if len(kg.CommercialWholesaleTiers) != 3 {
		t.Fatalf("kg tiers = %+v", kg.CommercialWholesaleTiers)
	}
	if kg.CommercialWholesaleTiers[0].Label != "24-49kg" || kg.CommercialWholesaleTiers[0].SpecG != 1000 {
		t.Fatalf("kg tier 1 = %+v", kg.CommercialWholesaleTiers[0])
	}
	assertClose(t, "24-49kg uses fourth kg price", kg.CommercialWholesaleTiers[0].PricePerUnit, kg.WholesaleKgPrices[3])
	assertClose(t, "50-99kg uses fifth kg price", kg.CommercialWholesaleTiers[1].PricePerUnit, kg.WholesaleKgPrices[4])
	assertClose(t, "100-199kg uses sixth kg price", kg.CommercialWholesaleTiers[2].PricePerUnit, kg.WholesaleKgPrices[5])

	halfLb := CalculateProduct(params, ProductInput{
		ProductID:              29,
		Name:                   "白月光-瑰夏",
		GreenBeanCostPerKg:     362.5,
		YieldRate:              0.8,
		WholesaleTierScheme:    WholesaleTierScheme227GTwo,
		WholesaleKgMarginRates: premiumWholesaleMarginRates(params),
	})
	if len(halfLb.CommercialWholesaleTiers) != 2 {
		t.Fatalf("227g tiers = %+v", halfLb.CommercialWholesaleTiers)
	}
	if halfLb.CommercialWholesaleTiers[0].Label != "2包-7包" || halfLb.CommercialWholesaleTiers[0].SpecG != 227 {
		t.Fatalf("227g tier 1 = %+v", halfLb.CommercialWholesaleTiers[0])
	}
	assertClose(t, "2-7 bags uses rounded half-pound price", halfLb.CommercialWholesaleTiers[0].PricePerUnit, 192)
	assertClose(t, "8+ bags uses rounded second half-pound price", halfLb.CommercialWholesaleTiers[1].PricePerUnit, 166)
}

func TestGradientTemplateCommercialTiersMatchByWeightAndUseTemplateUnit(t *testing.T) {
	params := DefaultParameters()
	got := CalculateProduct(params, ProductInput{
		ProductID:          501,
		Name:               "模板拼配",
		GreenBeanCostPerKg: 51.75,
		YieldRate:          0.8,
		GradientTemplate: &GradientTemplate{
			ID:          9,
			Name:        "工厂量单模板",
			DisplayUnit: GradientDisplayUnitKg,
			Tiers: []GradientTemplateTier{
				{ID: 91, Label: "大客户量单", MinWeightG: 24000, MaxWeightG: f64(49000), MarginRate: 0.175, Position: 1},
				{ID: 92, Label: "战略客户", MinWeightG: 50000, MaxWeightG: nil, MarginRate: 0.12, Position: 2},
			},
		},
	})

	if len(got.CommercialWholesaleTiers) != 2 {
		t.Fatalf("commercial tiers = %+v, want 2 template tiers", got.CommercialWholesaleTiers)
	}
	first := got.CommercialWholesaleTiers[0]
	if first.Label != "大客户量单" || first.SpecG != 1000 || first.MinQty != 24 {
		t.Fatalf("first tier = %+v", first)
	}
	if first.MaxQty == nil || *first.MaxQty != 49 {
		t.Fatalf("first max qty = %+v, want 49kg", first.MaxQty)
	}
	if first.TemplateID != 9 || first.TemplateTierID != 91 || first.MarginRate != 0.175 || first.DisplayUnit != GradientDisplayUnitKg {
		t.Fatalf("template metadata = %+v", first)
	}
	assertClose(t, "kg price", first.PricePerUnit, 82)

	lb := CalculateProduct(params, ProductInput{
		ProductID:          502,
		Name:               "模板单品",
		GreenBeanCostPerKg: 62,
		YieldRate:          0.8,
		GradientTemplate: &GradientTemplate{
			ID:          10,
			Name:        "454g 包数模板",
			DisplayUnit: GradientDisplayUnitLb,
			Tiers: []GradientTemplateTier{
				{ID: 101, Label: "自定义小单", MinWeightG: 908, MaxWeightG: f64(5902), MarginRate: 0.5421052631578949, Position: 1},
			},
		},
	})
	if len(lb.CommercialWholesaleTiers) != 1 {
		t.Fatalf("lb tiers = %+v, want one tier", lb.CommercialWholesaleTiers)
	}
	if tier := lb.CommercialWholesaleTiers[0]; tier.Label != "自定义小单" || tier.SpecG != 454 || tier.MinQty != 2 || tier.DisplayUnit != GradientDisplayUnitLb {
		t.Fatalf("lb template tier = %+v", tier)
	}
	assertClose(t, "lb price", lb.CommercialWholesaleTiers[0].PricePerUnit, 61)

	smallUnit := CalculateProduct(params, ProductInput{
		ProductID:          503,
		Name:               "小包装模板单品",
		GreenBeanCostPerKg: 51.75,
		YieldRate:          0.8,
		GradientTemplate: &GradientTemplate{
			ID:          11,
			Name:        "227g 小包装模板",
			DisplayUnit: GradientDisplayUnit227G,
			Tiers: []GradientTemplateTier{
				{ID: 111, Label: "2-7份", MinWeightG: 454, MaxWeightG: f64(1589), MarginRate: 0.175, Position: 1},
			},
		},
	})
	if len(smallUnit.CommercialWholesaleTiers) != 1 {
		t.Fatalf("227g tiers = %+v, want one tier", smallUnit.CommercialWholesaleTiers)
	}
	if tier := smallUnit.CommercialWholesaleTiers[0]; tier.Label != "2-7份" || tier.SpecG != 227 || tier.MinQty != 2 || tier.MaxQty == nil || *tier.MaxQty != 7 || tier.DisplayUnit != GradientDisplayUnit227G {
		t.Fatalf("227g template tier = %+v", tier)
	}
	assertClose(t, "227g price", smallUnit.CommercialWholesaleTiers[0].PricePerUnit, 19)
}

func TestDripWholesaleTiersUseTemplateAndProductBagConfig(t *testing.T) {
	params := DefaultParameters()
	got := CalculateProduct(params, ProductInput{
		ProductID:          701,
		Name:               "Uraga乌拉嘎",
		ProductKind:        "drip_bag",
		DripBagGrams:       12,
		DripBoxBagCount:    10,
		GreenBeanCostPerKg: 60,
		YieldRate:          0.8,
		DripPriceTemplate: &DripPriceTemplate{
			ID:               5,
			Name:             "挂耳供应价",
			BagGrams:         12,
			BoxBagCount:      10,
			IncludePackaging: true,
			Tiers: []DripPriceTemplateTier{
				{ID: 51, Label: "100袋", MinBags: 100, Multiplier: 2.2, Position: 1},
				{ID: 52, Label: "1000袋", MinBags: 1000, Multiplier: 1.8, Position: 2},
			},
		},
	})

	if got.ProductKind != "drip_bag" || got.DripBagGrams != 12 || got.DripBoxBagCount != 10 {
		t.Fatalf("drip product config = %+v", got)
	}
	if got.DripBeanList.Code == "" || got.CommercialBeanList.Code != "" || got.RetailBeanList.Code != "" {
		t.Fatalf("drip product must only appear in drip bean list, got commercial=%+v drip=%+v retail=%+v", got.CommercialBeanList, got.DripBeanList, got.RetailBeanList)
	}
	if len(got.DripWholesaleTiers) != 2 {
		t.Fatalf("drip tiers = %+v, want template tiers", got.DripWholesaleTiers)
	}
	first := got.DripWholesaleTiers[0]
	if first.Label != "100袋" || first.MinBags != 100 || first.TemplateID != 5 || first.TemplateTierID != 51 {
		t.Fatalf("first drip tier metadata = %+v", first)
	}
	if first.BagGrams != 12 || first.BoxBagCount != 10 || first.Multiplier != 2.2 || first.TaxRate != params.RetailTaxRate {
		t.Fatalf("first drip tier pricing source = %+v", first)
	}
	if first.PackedPricePerBag <= first.LoosePricePerBag || first.PackedPricePerBag <= 0 {
		t.Fatalf("first drip tier prices = %+v", first)
	}
}

func TestProductMarginOverrideReplacesGradientTemplateTierMargin(t *testing.T) {
	params := DefaultParameters()
	input := ProductInput{
		ProductID:          501,
		Name:               "模板拼配",
		GreenBeanCostPerKg: 51.75,
		YieldRate:          0.8,
		MarginRateOverride: f64(0.30),
		GradientTemplate: &GradientTemplate{
			ID:          9,
			Name:        "工厂量单模板",
			DisplayUnit: GradientDisplayUnitKg,
			Tiers: []GradientTemplateTier{
				{ID: 91, Label: "大客户量单", MinWeightG: 24000, MaxWeightG: f64(49000), MarginRate: 0.175, Position: 1},
			},
		},
	}

	got := CalculateProduct(params, input)
	if len(got.CommercialWholesaleTiers) != 1 {
		t.Fatalf("commercial tiers = %+v, want one template tier", got.CommercialWholesaleTiers)
	}
	tier := got.CommercialWholesaleTiers[0]
	if tier.MarginRate != 0.30 {
		t.Fatalf("tier margin = %.3f, want product override 0.300; tier=%+v", tier.MarginRate, tier)
	}
	if tier.PricePerUnit != 91 {
		t.Fatalf("tier price = %.2f, want override price 91 from 0.30 margin; tier=%+v", tier.PricePerUnit, tier)
	}
}

func TestCommercialPriceExplanationIncludesFastCostParametersAndTemporaryOverrides(t *testing.T) {
	params := DefaultParameters()
	input := ProductInput{
		ProductID:          501,
		Name:               "模板拼配",
		GreenBeanCostPerKg: 51.75,
		YieldRate:          0.8,
		GradientTemplate: &GradientTemplate{
			ID:          9,
			Name:        "工厂量单模板",
			DisplayUnit: GradientDisplayUnitKg,
			Tiers: []GradientTemplateTier{
				{ID: 91, Label: "大客户量单", MinWeightG: 24000, MaxWeightG: f64(49000), MarginRate: 0.175, Position: 1},
			},
		},
	}

	explanation, err := ExplainCommercialPrice(params, input, PriceExplanationRequest{
		TierLabel: "大客户量单",
		Overrides: PriceExplanationOverrides{
			MarginRate: f64(0.30),
		},
	})
	if err != nil {
		t.Fatalf("ExplainCommercialPrice() error = %v", err)
	}
	if explanation.TemplateName != "工厂量单模板" || explanation.TierLabel != "大客户量单" || explanation.DisplayUnit != GradientDisplayUnitKg {
		t.Fatalf("explanation header = %+v", explanation)
	}
	if explanation.SavedFinalPrice == explanation.PreviewFinalPrice {
		t.Fatalf("temporary override should change preview price: %+v", explanation)
	}
	if explanation.SavedFinalPrice != 82 || explanation.PreviewFinalPrice != 91 {
		t.Fatalf("prices = saved %.2f preview %.2f, want 82/91", explanation.SavedFinalPrice, explanation.PreviewFinalPrice)
	}
	wantKeys := []string{
		"green_bean_cost_per_kg",
		"yield_rate",
		"roasted_bean_cost_per_kg",
		"large_batch_production_cost_per_kg",
		"wholesale_package_cost_per_kg",
		"product_loss_per_kg",
		"retail_tax_rate",
		"template_margin_rate",
		"display_unit_conversion",
	}
	for _, key := range wantKeys {
		if !explanation.HasStep(key) {
			t.Fatalf("missing explanation step %s in %+v", key, explanation.Steps)
		}
	}
}

func TestExcelBeanListCommercialPricesMatchRoundedWorkbook(t *testing.T) {
	params := DefaultParameters()
	cases := []struct {
		name      string
		greenCost float64
		want      map[string]float64
	}{
		{"曲奇拼配", 51.75, map[string]float64{"24-49kg": 82, "50-99kg": 78, "100-199kg": 73}},
		{"金色山脉", 62, map[string]float64{"2包-13包": 61, "14包-23包": 55, "24包-47包": 49, "48包+": 46}},
		{"酒心巧克力", 67, map[string]float64{"2包-13包": 65, "14包-23包": 59, "24包-47包": 53, "48包+": 48}},
		{"菠萝意式2.0", 95, map[string]float64{"2包-13包": 106, "14包-23包": 92, "24包-47包": 81, "48包+": 74}},
		{"橘皮乌龙", 62, map[string]float64{"2包-13包": 61, "14包-23包": 55, "24包-47包": 49, "48包+": 46}},
		{"芒霜2.0", 105, map[string]float64{"2包-13包": 116, "14包-23包": 101, "24包-47包": 89, "48包+": 81}},
		{"小菠萝2.0", 95, map[string]float64{"2包-13包": 106, "14包-23包": 92, "24包-47包": 81, "48包+": 74}},
		{"萨奇姆", 90, map[string]float64{"2包-13包": 100, "14包-23包": 87, "48包+": 70}},
		{"曜石2.0", 63.9, map[string]float64{"2包-13包": 63, "14包-23包": 56, "24包-47包": 50, "48包+": 47}},
		{"红岩2.0", 63.9, map[string]float64{"2包-13包": 63, "14包-23包": 56, "24包-47包": 50, "48包+": 47}},
		{"初晓", 75.5, map[string]float64{"2包-13包": 73, "14包-23包": 66, "24包-47包": 59, "48包+": 55}},
		{"松饼", 64.4, map[string]float64{"2包-13包": 63, "14包-23包": 57, "24包-47包": 51, "48包+": 48}},
		{"榛巧", 64.4, map[string]float64{"2包-13包": 63, "14包-23包": 57, "24包-47包": 51, "48包+": 48}},
		{"果语花香", 71.5, map[string]float64{"2包-13包": 69, "14包-23包": 62, "24包-47包": 56, "48包+": 52}},
		{"耶加雪菲G2", 76, map[string]float64{"2包-13包": 86, "14包-23包": 75, "24包-47包": 65, "48包+": 55}},
		{"Uraga乌拉嘎", 108, map[string]float64{"2包-13包": 119, "14包-23包": 104, "24包-47包": 91, "48包+": 84}},
		{"浣纱果园", 100, map[string]float64{"2包-13包": 111, "14包-23包": 96, "48包+": 100}},
		{"肯尼亚TOPAA", 120, map[string]float64{"2包-13包": 132, "14包-23包": 114, "48包+": 93}},
		{"森林瑰夏", 118, map[string]float64{"2包-13包": 130, "14包-23包": 113, "24包-47包": 99, "48包+": 91}},
		{"Nenka嫩咖", 116, map[string]float64{"2包-13包": 127, "14包-23包": 111, "24包-47包": 98, "48包+": 90}},
		{"曼特宁", 69, map[string]float64{"2包-13包": 79, "14包-23包": 68, "24包-47包": 60, "48包+": 55}},
		{"白月光-瑰夏", 360, map[string]float64{"2包-7包": 190, "8包+": 165}},
		{"芸上莓梦", 152, map[string]float64{"2包-7包": 82, "8包+": 72}},
		{"晨曦-娜伊", 450, map[string]float64{"2包-7包": 237, "8包+": 184}},
		{"晚香玉", 128, map[string]float64{"2包-7包": 70, "8包+": 61}},
	}
	for _, tc := range cases {
		got := CalculateProduct(params, ProductInput{Name: tc.name, GreenBeanCostPerKg: tc.greenCost, YieldRate: 0.8})
		tierPrices := commercialPriceMap(got.CommercialWholesaleTiers)
		if len(tierPrices) != len(tc.want) {
			t.Fatalf("%s tier count = %d (%+v), want %d", tc.name, len(tierPrices), got.CommercialWholesaleTiers, len(tc.want))
		}
		for label, want := range tc.want {
			if tierPrices[label] != want {
				t.Fatalf("%s %s = %.3f, want %.3f; tiers=%+v", tc.name, label, tierPrices[label], want, got.CommercialWholesaleTiers)
			}
		}
	}
}

func TestExcelRetailBeanListPricesMatchRoundedWorkbook(t *testing.T) {
	params := DefaultParameters()
	cases := []struct {
		name      string
		greenCost float64
		want      map[string]float64
	}{
		{"金色山脉", 62, map[string]float64{"227g": 50, "250g": 55}},
		{"酒心巧克力", 67, map[string]float64{"227g": 53, "250g": 59}},
		{"菠萝意式2.0", 95, map[string]float64{"227g": 73, "250g": 80}},
		{"橘皮乌龙", 62, map[string]float64{"227g": 50, "250g": 55}},
		{"芒霜2.0", 105, map[string]float64{"227g": 80, "250g": 88}},
		{"小菠萝2.0", 95, map[string]float64{"227g": 73, "250g": 80}},
		{"萨奇姆", 90, map[string]float64{"227g": 69, "250g": 76}},
		{"曜石2.0", 63.9, map[string]float64{"227g": 51, "250g": 56}},
		{"红岩2.0", 63.9, map[string]float64{"227g": 47, "250g": 51}},
		{"初晓", 75.5, map[string]float64{"227g": 59, "250g": 65}},
		{"松饼", 64.4, map[string]float64{"227g": 51, "250g": 57}},
		{"榛巧", 64.4, map[string]float64{"227g": 51, "250g": 57}},
		{"果语花香", 71.5, map[string]float64{"227g": 56, "250g": 62}},
		{"耶加雪菲G2", 76, map[string]float64{"227g": 60, "250g": 66}},
		{"Uraga乌拉嘎", 108, map[string]float64{"227g": 82, "250g": 90}},
		{"浣纱果园", 100, map[string]float64{"227g": 77, "250g": 84}},
		{"肯尼亚TOPAA", 120, map[string]float64{"227g": 91, "250g": 100}},
		{"森林瑰夏", 118, map[string]float64{"227g": 89, "250g": 98}},
		{"Nenka嫩咖", 116, map[string]float64{"227g": 88, "250g": 97}},
		{"曼特宁", 69, map[string]float64{"227g": 55, "250g": 60}},
		{"白月光-瑰夏", 360, map[string]float64{"100g": 115, "200g": 229}},
		{"芸上莓梦", 152, map[string]float64{"100g": 50, "200g": 100}},
		{"晨曦-娜伊", 450, map[string]float64{"100g": 143, "200g": 286}},
		{"晚香玉", 128, map[string]float64{"100g": 42, "200g": 85}},
	}
	for _, tc := range cases {
		got := CalculateProduct(params, ProductInput{Name: tc.name, GreenBeanCostPerKg: tc.greenCost, YieldRate: 0.8})
		retailPrices := retailPriceMap(got.RetailBeanTiers)
		if len(retailPrices) != len(tc.want) {
			t.Fatalf("%s retail tier count = %d (%+v), want %d", tc.name, len(retailPrices), got.RetailBeanTiers, len(tc.want))
		}
		for label, want := range tc.want {
			if retailPrices[label] != want {
				t.Fatalf("%s %s = %.3f, want %.3f; tiers=%+v", tc.name, label, retailPrices[label], want, got.RetailBeanTiers)
			}
		}
	}
}

func TestExcelBeanListDisplayMetadataMatchesWorkbook(t *testing.T) {
	params := DefaultParameters()

	uraga := CalculateProduct(params, ProductInput{Name: "Uraga乌拉嘎", GreenBeanCostPerKg: 108, YieldRate: 0.8})
	if uraga.CommercialBeanList.Code != "5.2" || uraga.CommercialBeanList.Category != "5、原产地精选豆：" {
		t.Fatalf("commercial list metadata = %+v", uraga.CommercialBeanList)
	}
	if uraga.CommercialBeanList.RecommendedUse != "手冲/SOE/冷萃" {
		t.Fatalf("commercial recommended use = %q", uraga.CommercialBeanList.RecommendedUse)
	}
	if uraga.CommercialBeanList.Flavor != "明显的花香、柑橘、荔枝，红糖甜，绿茶" {
		t.Fatalf("commercial flavor = %q", uraga.CommercialBeanList.Flavor)
	}
	if uraga.CommercialBeanList.Description != "埃塞·古吉·Uraga、74112水洗处理、浅度烘焙（新产季埃塞水洗）" {
		t.Fatalf("commercial description = %q", uraga.CommercialBeanList.Description)
	}
	if uraga.RetailBeanList.Code != "3.2" || uraga.RetailBeanList.Category != "3、原产地精选豆：" {
		t.Fatalf("retail list metadata = %+v", uraga.RetailBeanList)
	}
	if uraga.RetailBeanList.RecommendedUse != "手冲/SOE/冷萃" {
		t.Fatalf("retail recommended use = %q", uraga.RetailBeanList.RecommendedUse)
	}

	cookie := CalculateProduct(params, ProductInput{Name: "曲奇拼配", GreenBeanCostPerKg: 51.75, YieldRate: 0.8})
	if cookie.CommercialBeanList.Code != "1.1" || cookie.CommercialBeanList.Category != "1、工厂量单" {
		t.Fatalf("cookie commercial metadata = %+v", cookie.CommercialBeanList)
	}
	if cookie.RetailBeanList.Code != "" {
		t.Fatalf("cookie should not be shown in retail bean list, got %+v", cookie.RetailBeanList)
	}

	blend := CalculateProduct(params, ProductInput{Name: "红岩2.0", GreenBeanCostPerKg: 63.9, YieldRate: 0.8})
	if blend.CommercialBeanList.Code != "4.2" || blend.RetailBeanList.Code != "2.2" {
		t.Fatalf("blend commercial/retail numbering = %+v / %+v", blend.CommercialBeanList, blend.RetailBeanList)
	}
}

func TestCustomerCustomRoastBeanListUsesSkuCategoryMetadata(t *testing.T) {
	params := DefaultParameters()
	input := ProductInput{
		ProductID:                 902,
		Name:                      "芬纳定制-红酒日晒-中深烘",
		ProductKind:               "roasted",
		CustomerID:                74,
		CustomType:                "custom_roast",
		ProductCategoryID:         502,
		ProductCategoryPosition:   2,
		CategoryPrimaryName:       "咖啡豆",
		CategoryPrimaryPosition:   1,
		CategorySecondaryName:     "定制咖啡熟豆",
		CategorySecondaryPosition: 1,
		GreenBeanCostPerKg:        67,
		YieldRate:                 0.815,
		Flavor:                    "红酒、莓果",
		BeanListNote:              "客户自有定制熟豆",
	}

	got := CalculateProduct(params, input)

	if got.CommercialBeanList.Code != "1.2" {
		t.Fatalf("commercial code = %q, want 1.2; display=%+v", got.CommercialBeanList.Code, got.CommercialBeanList)
	}
	if got.CommercialBeanList.Category != "1、定制咖啡熟豆" {
		t.Fatalf("commercial category = %q", got.CommercialBeanList.Category)
	}
	if got.CommercialBeanList.DisplayName != "芬纳定制-红酒日晒-中深烘" {
		t.Fatalf("commercial display name = %q", got.CommercialBeanList.DisplayName)
	}
	if got.CommercialBeanList.Flavor != "红酒、莓果" || got.CommercialBeanList.Description != "客户自有定制熟豆" {
		t.Fatalf("commercial metadata should fall back to product bean-list fields: %+v", got.CommercialBeanList)
	}
}

func TestCustomerAliasBeanListOverridesExcelCategoryWithSkuCategory(t *testing.T) {
	params := DefaultParameters()
	input := ProductInput{
		ProductID:                 901,
		Name:                      "芬纳咖啡-Uraga乌拉嘎-中烘",
		BeanListTemplateName:      "Uraga乌拉嘎",
		ProductKind:               "roasted",
		CustomerID:                74,
		CustomType:                "public_sku_alias",
		BaseProductID:             1,
		ProductCategoryID:         502,
		ProductCategoryPosition:   1,
		CategoryPrimaryName:       "咖啡豆",
		CategoryPrimaryPosition:   1,
		CategorySecondaryName:     "定制咖啡熟豆",
		CategorySecondaryPosition: 1,
		GreenBeanCostPerKg:        108,
		YieldRate:                 0.8,
	}

	got := CalculateProduct(params, input)

	if got.CommercialBeanList.Code != "1.1" {
		t.Fatalf("commercial code = %q, want 1.1; display=%+v", got.CommercialBeanList.Code, got.CommercialBeanList)
	}
	if got.CommercialBeanList.Category != "1、定制咖啡熟豆" {
		t.Fatalf("commercial category should use customer SKU category, got %q", got.CommercialBeanList.Category)
	}
	if got.CommercialBeanList.DisplayName != "芬纳咖啡-Uraga乌拉嘎-中烘" {
		t.Fatalf("commercial display name = %q", got.CommercialBeanList.DisplayName)
	}
	if got.CommercialBeanList.RecommendedUse != "手冲/SOE/冷萃" || got.CommercialBeanList.Flavor == "" {
		t.Fatalf("customer alias should preserve Excel bean-list details: %+v", got.CommercialBeanList)
	}
}

func TestCustomerAliasBeanListWithoutSkuCategoryUsesUnclassifiedGroup(t *testing.T) {
	params := DefaultParameters()
	input := ProductInput{
		ProductID:            417,
		Name:                 "曲奇拼配2.0",
		BeanListTemplateName: "红岩2.0",
		ProductKind:          "roasted",
		CustomerID:           152,
		CustomType:           "public_sku_alias",
		BaseProductID:        199,
		GreenBeanCostPerKg:   63.9,
		YieldRate:            0.8,
	}

	got := CalculateProduct(params, input)

	if got.CommercialBeanList.Code != "999.2" {
		t.Fatalf("commercial code = %q, want 999.2; display=%+v", got.CommercialBeanList.Code, got.CommercialBeanList)
	}
	if got.CommercialBeanList.Category != "未分类" {
		t.Fatalf("commercial category = %q, want 未分类; display=%+v", got.CommercialBeanList.Category, got.CommercialBeanList)
	}
	if got.CommercialBeanList.DisplayName != "曲奇拼配2.0" {
		t.Fatalf("commercial display name = %q", got.CommercialBeanList.DisplayName)
	}
	if strings.Contains(got.CommercialBeanList.Category, "精品意式拼配") {
		t.Fatalf("customer alias without SKU category must not inherit Excel category: %+v", got.CommercialBeanList)
	}
}

func TestValidateProductInputRejectsInvalidInputs(t *testing.T) {
	params := DefaultParameters()
	if _, err := ValidateProductInput(params, ProductInput{Name: "bad", GreenBeanCostPerKg: 10, YieldRate: -0.1}); err == nil {
		t.Fatalf("expected invalid yield rate error")
	}
	if _, err := ValidateProductInput(params, ProductInput{Name: "bad", GreenBeanCostPerKg: -1, YieldRate: 0.8}); err == nil {
		t.Fatalf("expected invalid green bean cost error")
	}
}

func TestCalculateProductCarriesInactiveBomWarning(t *testing.T) {
	params := DefaultParameters()
	got := CalculateProduct(params, ProductInput{
		Name:               "曲奇拼配",
		GreenBeanCostPerKg: 51.75,
		YieldRate:          0.8,
		BomStatus:          "inactive",
		Warnings:           []string{"BOM已失效：请重新启用 BOM 后再发布价格策略"},
	})
	if got.BomStatus != "inactive" {
		t.Fatalf("BomStatus = %q, want inactive", got.BomStatus)
	}
	if len(got.Warnings) != 1 || got.Warnings[0] != "BOM已失效：请重新启用 BOM 后再发布价格策略" {
		t.Fatalf("warnings = %+v", got.Warnings)
	}
}

func TestDripWholesaleTiersMatchExcelFormula(t *testing.T) {
	params := DefaultParameters()
	params.RetailTaxRate = 0.03
	params.DripGreenRatioKgPerBag = 0.01
	params.DripProcessCostPerBag = 0.44
	params.DripExtraCostPerBag = 0.10
	params.DripPackingMaterialPerBag = 0.20
	params.WholesaleDripMultipliers = []float64{2.2, 1.8, 1.6, 1.35}

	out := CalculateProduct(params, ProductInput{
		Name:               "挂耳测试",
		GreenBeanCostPerKg: 80,
		YieldRate:          0.8,
	})
	if len(out.DripWholesaleTiers) != 4 {
		t.Fatalf("tiers len=%d", len(out.DripWholesaleTiers))
	}
	base := (80/0.8+params.SmallBatchProductionCostPerKg)*0.01 + 0.44 + 0.10
	wantLoose := roundPrice(base*2.2 + base*(2.2-1)*0.03)
	wantPacked := roundPrice(wantLoose + 0.20)
	if out.DripWholesaleTiers[0].MinBags != 100 || out.DripWholesaleTiers[0].LoosePricePerBag != wantLoose || out.DripWholesaleTiers[0].PackedPricePerBag != wantPacked {
		t.Fatalf("first tier=%+v want loose %.2f packed %.2f", out.DripWholesaleTiers[0], wantLoose, wantPacked)
	}
}

func f64(v float64) *float64 {
	return &v
}

func commercialPriceMap(tiers []CommercialWholesaleTier) map[string]float64 {
	out := make(map[string]float64, len(tiers))
	for _, tier := range tiers {
		out[tier.Label] = tier.PricePerUnit
	}
	return out
}

func retailPriceMap(tiers []RetailBeanTier) map[string]float64 {
	out := make(map[string]float64, len(tiers))
	for _, tier := range tiers {
		out[tier.Label] = tier.PricePerUnit
	}
	return out
}
