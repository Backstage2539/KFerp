package costing

import "fmt"

type Parameters struct {
	RoastYieldRate                float64   `json:"roast_yield_rate"`
	KgToLbFactor                  float64   `json:"kg_to_lb_factor"`
	SmallBatchProductionCostPerKg float64   `json:"small_batch_production_cost_per_kg"`
	LargeBatchProductionCostPerKg float64   `json:"large_batch_production_cost_per_kg"`
	WholesalePackageCostPerKg     float64   `json:"wholesale_package_cost_per_kg"`
	ProductLossPerKg              float64   `json:"product_loss_per_kg"`
	RetailBeanMarginRate          float64   `json:"retail_bean_margin_rate"`
	RetailTaxRate                 float64   `json:"retail_tax_rate"`
	RetailLogisticsPerKg          float64   `json:"retail_logistics_per_kg"`
	RetailDripLogisticsPer10Bags  float64   `json:"retail_drip_logistics_per_10_bags"`
	DripGreenRatioKgPerBag        float64   `json:"drip_green_ratio_kg_per_bag"`
	DripProcessCostPerBag         float64   `json:"drip_process_cost_per_bag"`
	DripExtraCostPerBag           float64   `json:"drip_extra_cost_per_bag"`
	DripPackingMaterialPerBag     float64   `json:"drip_packing_material_per_bag"`
	RetailDripMultiplier          float64   `json:"retail_drip_multiplier"`
	WholesaleKgMarginRates        []float64 `json:"wholesale_kg_margin_rates"`
	WholesaleDripMultipliers      []float64 `json:"wholesale_drip_multipliers"`
}

type ProductInput struct {
	ProductID                 int64     `json:"product_id"`
	Name                      string    `json:"name"`
	Flavor                    string    `json:"flavor,omitempty"`
	Origin                    string    `json:"origin,omitempty"`
	ProcessingStation         string    `json:"processing_station,omitempty"`
	Variety                   string    `json:"variety,omitempty"`
	ProcessMethod             string    `json:"process_method,omitempty"`
	Grade                     string    `json:"grade,omitempty"`
	Altitude                  string    `json:"altitude,omitempty"`
	BeanListNote              string    `json:"bean_list_note,omitempty"`
	GreenBeanCostPerKg        float64   `json:"green_bean_cost_per_kg"`
	YieldRate                 float64   `json:"yield_rate"`
	WholesaleTaxAddPerKg      float64   `json:"wholesale_tax_add_per_kg"`
	WholesaleTaxAddPerKgTiers []float64 `json:"wholesale_tax_add_per_kg_tiers"`
	DripTaxAddPerBag100       float64   `json:"drip_tax_add_per_bag_100"`
	DripTaxAddPerBagRetail    float64   `json:"drip_tax_add_per_bag_retail"`
	WholesaleKgMarginRates    []float64 `json:"wholesale_kg_margin_rates"`
	WholesaleDripMultipliers  []float64 `json:"wholesale_drip_multipliers"`
}

type CommercialWholesaleTier struct {
	Label      string   `json:"label"`
	MinLb      float64  `json:"min_lb"`
	MaxLb      *float64 `json:"max_lb,omitempty"`
	PricePerKg float64  `json:"price_per_kg"`
	PricePerLb float64  `json:"price_per_lb"`
}

type ProductResult struct {
	ProductID                      int64                     `json:"product_id"`
	Name                           string                    `json:"name"`
	Flavor                         string                    `json:"flavor,omitempty"`
	Origin                         string                    `json:"origin,omitempty"`
	ProcessingStation              string                    `json:"processing_station,omitempty"`
	Variety                        string                    `json:"variety,omitempty"`
	ProcessMethod                  string                    `json:"process_method,omitempty"`
	Grade                          string                    `json:"grade,omitempty"`
	Altitude                       string                    `json:"altitude,omitempty"`
	BeanListNote                   string                    `json:"bean_list_note,omitempty"`
	GreenBeanCostPerKg             float64                   `json:"green_bean_cost_per_kg"`
	RoastedBeanCostPerKg           float64                   `json:"roasted_bean_cost_per_kg"`
	SmallBatchCostPerKg            float64                   `json:"small_batch_cost_per_kg"`
	LargeBatchCostPerKg            float64                   `json:"large_batch_cost_per_kg"`
	DripBaseCostPerBag             float64                   `json:"drip_base_cost_per_bag"`
	RetailTaxPerKg                 float64                   `json:"retail_tax_per_kg"`
	WholesaleKgPrices              []float64                 `json:"wholesale_kg_prices"`
	WholesaleLbPrices              []float64                 `json:"wholesale_lb_prices"`
	CommercialWholesaleTiers       []CommercialWholesaleTier `json:"commercial_wholesale_tiers"`
	WholesaleDripBagPrices         []float64                 `json:"wholesale_drip_bag_prices"`
	WholesaleDripBagWithPackPrices []float64                 `json:"wholesale_drip_bag_with_pack_prices"`
	RetailKgPrice                  float64                   `json:"retail_kg_price"`
	Retail454gPrice                float64                   `json:"retail_454g_price"`
	Retail227gPrice                float64                   `json:"retail_227g_price"`
	Retail250gPrice                float64                   `json:"retail_250g_price"`
	Retail200gPrice                float64                   `json:"retail_200g_price"`
	Retail100gPrice                float64                   `json:"retail_100g_price"`
	RetailDrip10BagPrice           float64                   `json:"retail_drip_10_bag_price"`
}

func DefaultParameters() Parameters {
	return Parameters{
		RoastYieldRate:                0.8,
		KgToLbFactor:                  0.454,
		SmallBatchProductionCostPerKg: 6.2625,
		LargeBatchProductionCostPerKg: 3.1625,
		WholesalePackageCostPerKg:     1.7,
		ProductLossPerKg:              0.06,
		RetailBeanMarginRate:          1.45,
		RetailTaxRate:                 0.03,
		RetailLogisticsPerKg:          8,
		RetailDripLogisticsPer10Bags:  8,
		DripGreenRatioKgPerBag:        0.01,
		DripProcessCostPerBag:         0.44,
		DripExtraCostPerBag:           0.1,
		DripPackingMaterialPerBag:     0.2,
		RetailDripMultiplier:          2.5,
		WholesaleKgMarginRates:        []float64{0.5421052631578949, 0.3842105263157895, 0.27894736842105267, 0.2},
		WholesaleDripMultipliers:      []float64{2.2, 1.8, 1.6, 1.5},
	}
}

func ValidateProductInput(params Parameters, in ProductInput) (ProductInput, error) {
	if in.GreenBeanCostPerKg < 0 {
		return in, fmt.Errorf("green_bean_cost_per_kg must be >= 0")
	}
	if in.YieldRate == 0 {
		in.YieldRate = params.RoastYieldRate
	}
	if in.YieldRate <= 0 || in.YieldRate > 1 {
		return in, fmt.Errorf("yield_rate must be (0,1]")
	}
	if len(in.WholesaleKgMarginRates) == 0 {
		in.WholesaleKgMarginRates = params.WholesaleKgMarginRates
	}
	if len(in.WholesaleDripMultipliers) == 0 {
		in.WholesaleDripMultipliers = params.WholesaleDripMultipliers
	}
	return in, nil
}

func CalculateProduct(params Parameters, in ProductInput) ProductResult {
	in, _ = ValidateProductInput(params, in)
	roasted := in.GreenBeanCostPerKg / in.YieldRate
	small := roasted + params.SmallBatchProductionCostPerKg
	large := roasted + params.LargeBatchProductionCostPerKg
	dripBase := small*params.DripGreenRatioKgPerBag + params.DripProcessCostPerBag + params.DripExtraCostPerBag
	retailTax := small * params.RetailBeanMarginRate * params.RetailTaxRate

	out := ProductResult{
		ProductID:            in.ProductID,
		Name:                 in.Name,
		Flavor:               in.Flavor,
		Origin:               in.Origin,
		ProcessingStation:    in.ProcessingStation,
		Variety:              in.Variety,
		ProcessMethod:        in.ProcessMethod,
		Grade:                in.Grade,
		Altitude:             in.Altitude,
		BeanListNote:         in.BeanListNote,
		GreenBeanCostPerKg:   in.GreenBeanCostPerKg,
		RoastedBeanCostPerKg: roasted,
		SmallBatchCostPerKg:  small,
		LargeBatchCostPerKg:  large,
		DripBaseCostPerBag:   dripBase,
		RetailTaxPerKg:       retailTax,
	}

	for i, margin := range in.WholesaleKgMarginRates {
		base := small
		if i >= 2 {
			base = large
		}
		taxAdd := base * margin * params.RetailTaxRate
		if in.WholesaleTaxAddPerKg != 0 {
			taxAdd = in.WholesaleTaxAddPerKg
		}
		if i < len(in.WholesaleTaxAddPerKgTiers) {
			taxAdd = in.WholesaleTaxAddPerKgTiers[i]
		}
		kg := base*(1+margin) + params.WholesalePackageCostPerKg + params.ProductLossPerKg + taxAdd
		out.WholesaleKgPrices = append(out.WholesaleKgPrices, kg)
		out.WholesaleLbPrices = append(out.WholesaleLbPrices, kg*params.KgToLbFactor+1)
	}
	out.CommercialWholesaleTiers = buildCommercialWholesaleTiers(out.WholesaleKgPrices, out.WholesaleLbPrices)

	for _, multiplier := range in.WholesaleDripMultipliers {
		price := dripBase*multiplier + in.DripTaxAddPerBag100
		out.WholesaleDripBagPrices = append(out.WholesaleDripBagPrices, price)
		out.WholesaleDripBagWithPackPrices = append(out.WholesaleDripBagWithPackPrices, price+params.DripPackingMaterialPerBag)
	}

	out.RetailKgPrice = small*(1+params.RetailBeanMarginRate) + params.WholesalePackageCostPerKg + params.ProductLossPerKg + retailTax + params.RetailLogisticsPerKg
	out.Retail454gPrice = out.RetailKgPrice * params.KgToLbFactor
	out.Retail227gPrice = out.Retail454gPrice / 2
	out.Retail250gPrice = out.RetailKgPrice * 0.25
	out.Retail200gPrice = out.RetailKgPrice * 0.2
	out.Retail100gPrice = out.RetailKgPrice * 0.1
	out.RetailDrip10BagPrice = dripBase*10*params.RetailDripMultiplier + in.DripTaxAddPerBagRetail*10 + params.DripPackingMaterialPerBag + params.RetailDripLogisticsPer10Bags
	return out
}

func buildCommercialWholesaleTiers(kgPrices, lbPrices []float64) []CommercialWholesaleTier {
	defs := []struct {
		label string
		min   float64
		max   *float64
	}{
		{"2-13磅", 2, ptrFloat64(13)},
		{"14-23磅", 14, ptrFloat64(23)},
		{"24-47磅", 24, ptrFloat64(47)},
		{"大于47磅", 48, nil},
	}
	out := make([]CommercialWholesaleTier, 0, len(defs))
	for i, def := range defs {
		if i >= len(kgPrices) || i >= len(lbPrices) {
			break
		}
		out = append(out, CommercialWholesaleTier{
			Label:      def.label,
			MinLb:      def.min,
			MaxLb:      def.max,
			PricePerKg: kgPrices[i],
			PricePerLb: lbPrices[i],
		})
	}
	return out
}

func ptrFloat64(v float64) *float64 {
	return &v
}
