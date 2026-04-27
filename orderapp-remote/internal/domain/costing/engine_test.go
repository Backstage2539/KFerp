package costing

import "testing"

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
	assertClose(t, "wholesale kg 1kg tier", got.WholesaleKgPrices[0], 132.29283486842107)
	assertClose(t, "wholesale lb 1kg tier", got.WholesaleLbPrices[0], 61.060947030263165)
	assertClose(t, "wholesale kg 24kg tier", got.WholesaleKgPrices[3], 98.93193124999999)
	assertClose(t, "wholesale drip 100", got.WholesaleDripBagPrices[0], 3.0803695000000006)
	assertClose(t, "wholesale drip with pack 100", got.WholesaleDripBagWithPackPrices[0], 3.280369500000001)
	assertClose(t, "retail kg", got.RetailKgPrice, 218.62179375)
	assertClose(t, "retail lb", got.Retail454gPrice, 99.2542943625)
	assertClose(t, "retail half lb", got.Retail227gPrice, 49.62714718125)
	assertClose(t, "retail drip 10 bags", got.RetailDrip10BagPrice, 43.26055625000001)
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
	assertClose(t, "2-13 lb price", got.CommercialWholesaleTiers[0].PricePerLb, 61.060947030263165)
	assertClose(t, "greater than 47 lb price", got.CommercialWholesaleTiers[3].PricePerLb, 45.9150967875)
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
	assertClose(t, "Nenka 2-13 bags", got.CommercialWholesaleTiers[0].PricePerUnit, 127.43119209605264)
	assertClose(t, "Nenka 14-23 bags", got.CommercialWholesaleTiers[1].PricePerUnit, 110.84119876710525)
	assertClose(t, "Nenka 24-47 bags", got.CommercialWholesaleTiers[2].PricePerUnit, 97.784917425)
	assertClose(t, "Nenka 48+ bags", got.CommercialWholesaleTiers[3].PricePerUnit, 89.5535767875)
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
	assertClose(t, "2-7 bags uses half of first 454g price", halfLb.CommercialWholesaleTiers[0].PricePerUnit, halfLb.WholesaleLbPrices[0]/2)
	assertClose(t, "8+ bags uses half of second 454g price", halfLb.CommercialWholesaleTiers[1].PricePerUnit, halfLb.WholesaleLbPrices[1]/2)
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

func f64(v float64) *float64 {
	return &v
}
