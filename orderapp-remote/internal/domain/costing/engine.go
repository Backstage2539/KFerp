package costing

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

const (
	WholesaleTierScheme454GFour = "bag_454_four"
	WholesaleTierSchemeKgThree  = "kg_three"
	WholesaleTierScheme227GTwo  = "bag_227_two"

	GradientDisplayUnitLb   = "lb"
	GradientDisplayUnitKg   = "kg"
	GradientDisplayUnit227G = "g227"
	GradientDisplayUnit100G = "g100"
	GradientDisplayUnit250G = "g250"
)

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
	ProductID                 int64                     `json:"product_id"`
	Name                      string                    `json:"name"`
	ProductKind               string                    `json:"product_kind,omitempty"`
	CustomerID                int64                     `json:"customer_id,omitempty"`
	BaseProductID             int64                     `json:"base_product_id,omitempty"`
	Visibility                string                    `json:"visibility,omitempty"`
	CustomType                string                    `json:"custom_type,omitempty"`
	ProductCategoryID         int64                     `json:"product_category_id,omitempty"`
	BeanListTemplateName      string                    `json:"bean_list_template_name,omitempty"`
	Flavor                    string                    `json:"flavor,omitempty"`
	Origin                    string                    `json:"origin,omitempty"`
	ProcessingStation         string                    `json:"processing_station,omitempty"`
	Variety                   string                    `json:"variety,omitempty"`
	ProcessMethod             string                    `json:"process_method,omitempty"`
	Grade                     string                    `json:"grade,omitempty"`
	Altitude                  string                    `json:"altitude,omitempty"`
	BeanListNote              string                    `json:"bean_list_note,omitempty"`
	BomStatus                 string                    `json:"bom_status,omitempty"`
	Warnings                  []string                  `json:"warnings,omitempty"`
	GreenBeanCostPerKg        float64                   `json:"green_bean_cost_per_kg"`
	YieldRate                 float64                   `json:"yield_rate"`
	WholesaleTaxAddPerKg      float64                   `json:"wholesale_tax_add_per_kg"`
	WholesaleTaxAddPerKgTiers []float64                 `json:"wholesale_tax_add_per_kg_tiers"`
	DripTaxAddPerBag100       float64                   `json:"drip_tax_add_per_bag_100"`
	DripTaxAddPerBagRetail    float64                   `json:"drip_tax_add_per_bag_retail"`
	WholesaleKgMarginRates    []float64                 `json:"wholesale_kg_margin_rates"`
	WholesaleDripMultipliers  []float64                 `json:"wholesale_drip_multipliers"`
	WholesaleTierScheme       string                    `json:"wholesale_tier_scheme,omitempty"`
	MarginRateOverride        *float64                  `json:"margin_rate_override,omitempty"`
	GradientTemplate          *GradientTemplate         `json:"gradient_template,omitempty"`
	GreenBeanSaleTiers        []CommercialWholesaleTier `json:"green_bean_sale_tiers,omitempty"`
	BeanListQuality           BeanListQuality           `json:"bean_list_quality,omitempty"`
}

type CommercialWholesaleTier struct {
	Label          string   `json:"label"`
	Scheme         string   `json:"scheme,omitempty"`
	SpecG          int64    `json:"spec_g,omitempty"`
	MinQty         float64  `json:"min_qty,omitempty"`
	MaxQty         *float64 `json:"max_qty,omitempty"`
	PricePerUnit   float64  `json:"price_per_unit"`
	MinLb          float64  `json:"min_lb"`
	MaxLb          *float64 `json:"max_lb,omitempty"`
	PricePerKg     float64  `json:"price_per_kg"`
	PricePerLb     float64  `json:"price_per_lb"`
	TemplateID     int64    `json:"template_id,omitempty"`
	TemplateTierID int64    `json:"template_tier_id,omitempty"`
	DisplayUnit    string   `json:"display_unit,omitempty"`
	MinWeightG     float64  `json:"min_weight_g,omitempty"`
	MaxWeightG     *float64 `json:"max_weight_g,omitempty"`
	MarginRate     float64  `json:"margin_rate,omitempty"`
}

type GradientTemplate struct {
	ID          int64                  `json:"id,omitempty"`
	Name        string                 `json:"name"`
	DisplayUnit string                 `json:"display_unit"`
	Active      bool                   `json:"active"`
	Tiers       []GradientTemplateTier `json:"tiers"`
}

type GradientTemplateTier struct {
	ID         int64    `json:"id,omitempty"`
	Label      string   `json:"label"`
	MinWeightG float64  `json:"min_weight_g"`
	MaxWeightG *float64 `json:"max_weight_g,omitempty"`
	MarginRate float64  `json:"margin_rate"`
	Position   int      `json:"position"`
}

type PriceExplanationRequest struct {
	TierLabel string                    `json:"tier_label"`
	Overrides PriceExplanationOverrides `json:"overrides,omitempty"`
}

type PriceExplanationOverrides struct {
	GreenBeanCostPerKg *float64 `json:"green_bean_cost_per_kg,omitempty"`
	YieldRate          *float64 `json:"yield_rate,omitempty"`
	MarginRate         *float64 `json:"margin_rate,omitempty"`
}

type PriceExplanationStep struct {
	Key     string  `json:"key"`
	Label   string  `json:"label"`
	Source  string  `json:"source"`
	Value   float64 `json:"value"`
	Unit    string  `json:"unit,omitempty"`
	Changed bool    `json:"changed,omitempty"`
}

type PriceExplanation struct {
	ProductID         int64                  `json:"product_id"`
	ProductName       string                 `json:"product_name"`
	TemplateID        int64                  `json:"template_id,omitempty"`
	TemplateName      string                 `json:"template_name,omitempty"`
	TierLabel         string                 `json:"tier_label"`
	DisplayUnit       string                 `json:"display_unit"`
	MinWeightG        float64                `json:"min_weight_g"`
	MaxWeightG        *float64               `json:"max_weight_g,omitempty"`
	SavedFinalPrice   float64                `json:"saved_final_price"`
	PreviewFinalPrice float64                `json:"preview_final_price"`
	Steps             []PriceExplanationStep `json:"steps"`
}

func (e PriceExplanation) HasStep(key string) bool {
	for _, step := range e.Steps {
		if step.Key == key {
			return true
		}
	}
	return false
}

type RetailBeanTier struct {
	Label        string  `json:"label"`
	SpecG        int64   `json:"spec_g"`
	PricePerUnit float64 `json:"price_per_unit"`
}

type BeanListDisplay struct {
	Code           string `json:"code,omitempty"`
	Category       string `json:"category,omitempty"`
	DisplayName    string `json:"display_name,omitempty"`
	RecommendedUse string `json:"recommended_use,omitempty"`
	Flavor         string `json:"flavor,omitempty"`
	Description    string `json:"description,omitempty"`
}

type BeanListQuality struct {
	FactoryFlavorDescription string `json:"factory_flavor_description,omitempty"`
	Moisture                 string `json:"moisture,omitempty"`
	Density                  string `json:"density,omitempty"`
	InspectionCreatedAt      string `json:"inspection_created_at,omitempty"`
	InspectionReferenceNo    string `json:"inspection_reference_no,omitempty"`
}

type ProductResult struct {
	ProductID                      int64                     `json:"product_id"`
	Name                           string                    `json:"name"`
	ProductKind                    string                    `json:"product_kind,omitempty"`
	CustomerID                     int64                     `json:"customer_id,omitempty"`
	BaseProductID                  int64                     `json:"base_product_id,omitempty"`
	Visibility                     string                    `json:"visibility,omitempty"`
	CustomType                     string                    `json:"custom_type,omitempty"`
	ProductCategoryID              int64                     `json:"product_category_id,omitempty"`
	MarginRateOverride             *float64                  `json:"margin_rate_override,omitempty"`
	GradientTemplate               *GradientTemplate         `json:"gradient_template,omitempty"`
	CommercialBeanList             BeanListDisplay           `json:"commercial_bean_list"`
	RetailBeanList                 BeanListDisplay           `json:"retail_bean_list"`
	GreenBeanList                  BeanListDisplay           `json:"green_bean_list"`
	BeanListQuality                BeanListQuality           `json:"bean_list_quality,omitempty"`
	Flavor                         string                    `json:"flavor,omitempty"`
	Origin                         string                    `json:"origin,omitempty"`
	ProcessingStation              string                    `json:"processing_station,omitempty"`
	Variety                        string                    `json:"variety,omitempty"`
	ProcessMethod                  string                    `json:"process_method,omitempty"`
	Grade                          string                    `json:"grade,omitempty"`
	Altitude                       string                    `json:"altitude,omitempty"`
	BeanListNote                   string                    `json:"bean_list_note,omitempty"`
	BomStatus                      string                    `json:"bom_status,omitempty"`
	Warnings                       []string                  `json:"warnings,omitempty"`
	YieldRate                      float64                   `json:"yield_rate"`
	GreenBeanCostPerKg             float64                   `json:"green_bean_cost_per_kg"`
	RoastedBeanCostPerKg           float64                   `json:"roasted_bean_cost_per_kg"`
	SmallBatchCostPerKg            float64                   `json:"small_batch_cost_per_kg"`
	LargeBatchCostPerKg            float64                   `json:"large_batch_cost_per_kg"`
	DripBaseCostPerBag             float64                   `json:"drip_base_cost_per_bag"`
	RetailTaxPerKg                 float64                   `json:"retail_tax_per_kg"`
	WholesaleKgPrices              []float64                 `json:"wholesale_kg_prices"`
	WholesaleLbPrices              []float64                 `json:"wholesale_lb_prices"`
	CommercialWholesaleTiers       []CommercialWholesaleTier `json:"commercial_wholesale_tiers"`
	GreenBeanSaleTiers             []CommercialWholesaleTier `json:"green_bean_sale_tiers,omitempty"`
	WholesaleDripBagPrices         []float64                 `json:"wholesale_drip_bag_prices"`
	WholesaleDripBagWithPackPrices []float64                 `json:"wholesale_drip_bag_with_pack_prices"`
	RetailKgPrice                  float64                   `json:"retail_kg_price"`
	Retail454gPrice                float64                   `json:"retail_454g_price"`
	Retail227gPrice                float64                   `json:"retail_227g_price"`
	Retail250gPrice                float64                   `json:"retail_250g_price"`
	Retail200gPrice                float64                   `json:"retail_200g_price"`
	Retail100gPrice                float64                   `json:"retail_100g_price"`
	RetailBeanTiers                []RetailBeanTier          `json:"retail_bean_tiers"`
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
		WholesaleKgMarginRates:        []float64{0.5421052631578949, 0.3842105263157895, 0.27894736842105267, 0.2, 0.12, 0.045},
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
	if in.MarginRateOverride != nil && *in.MarginRateOverride < 0 {
		return in, fmt.Errorf("margin_rate_override must be >= 0")
	}
	in = ApplyExcelCommercialPricingProfile(params, in)
	if len(in.WholesaleKgMarginRates) == 0 {
		in.WholesaleKgMarginRates = params.WholesaleKgMarginRates
	}
	in.WholesaleKgMarginRates = normalizeWholesaleMarginRates(params, in.WholesaleKgMarginRates)
	if len(in.WholesaleTaxAddPerKgTiers) == 0 {
		in.WholesaleTaxAddPerKgTiers = defaultWholesaleTaxAddPerKgTiers(params, in)
	}
	if len(in.WholesaleDripMultipliers) == 0 {
		in.WholesaleDripMultipliers = params.WholesaleDripMultipliers
	}
	in.BomStatus = normalizeBomStatus(in.BomStatus)
	in.Warnings = normalizeWarnings(in.Warnings)
	return in, nil
}

func ApplyExcelCommercialPricingProfile(params Parameters, in ProductInput) ProductInput {
	profileName := costingProfileName(in)
	if strings.TrimSpace(in.WholesaleTierScheme) == "" {
		in.WholesaleTierScheme = inferWholesaleTierScheme(profileName)
	} else {
		in.WholesaleTierScheme = normalizeWholesaleTierScheme(in.WholesaleTierScheme)
	}
	if len(in.WholesaleKgMarginRates) == 0 {
		switch {
		case isCookieBlend(profileName):
			in.WholesaleKgMarginRates = cookieWholesaleMarginRates(params)
		case isWineSunBean(profileName):
			in.WholesaleKgMarginRates = wineSunWholesaleMarginRates(params)
		case isYirgacheffeG2(profileName):
			in.WholesaleKgMarginRates = premiumFirstThreeWholesaleMarginRates(params)
		case isPremiumCommercialBean(profileName):
			in.WholesaleKgMarginRates = premiumWholesaleMarginRates(params)
		default:
			in.WholesaleKgMarginRates = normalizeWholesaleMarginRates(params, nil)
		}
	}
	return in
}

func CalculateProduct(params Parameters, in ProductInput) ProductResult {
	if strings.TrimSpace(in.ProductKind) == "green_bean" {
		return calculateGreenBeanProduct(params, in)
	}
	in, _ = ValidateProductInput(params, in)
	profileName := costingProfileName(in)
	roasted := in.GreenBeanCostPerKg / in.YieldRate
	small := roasted + params.SmallBatchProductionCostPerKg
	large := roasted + params.LargeBatchProductionCostPerKg
	dripBase := small*params.DripGreenRatioKgPerBag + params.DripProcessCostPerBag + params.DripExtraCostPerBag
	retailTax := small * params.RetailBeanMarginRate * params.RetailTaxRate
	retailSmall := small
	if retailGreenCost, ok := excelRetailGreenCostOverride(profileName); ok {
		retailSmall = retailGreenCost/in.YieldRate + params.SmallBatchProductionCostPerKg
		retailTax = retailSmall * params.RetailBeanMarginRate * params.RetailTaxRate
	}
	commercialDisplay := commercialBeanListDisplay(profileName)
	retailDisplay := retailBeanListDisplay(profileName)
	if in.CustomerID > 0 {
		if commercialDisplay.Code != "" {
			commercialDisplay.DisplayName = in.Name
		}
		if retailDisplay.Code != "" {
			retailDisplay.DisplayName = in.Name
		}
	}

	out := ProductResult{
		ProductID:            in.ProductID,
		Name:                 in.Name,
		ProductKind:          "roasted",
		CustomerID:           in.CustomerID,
		BaseProductID:        in.BaseProductID,
		Visibility:           in.Visibility,
		CustomType:           in.CustomType,
		ProductCategoryID:    in.ProductCategoryID,
		MarginRateOverride:   in.MarginRateOverride,
		GradientTemplate:     in.GradientTemplate,
		CommercialBeanList:   commercialDisplay,
		RetailBeanList:       retailDisplay,
		BeanListQuality:      in.BeanListQuality,
		Flavor:               in.Flavor,
		Origin:               in.Origin,
		ProcessingStation:    in.ProcessingStation,
		Variety:              in.Variety,
		ProcessMethod:        in.ProcessMethod,
		Grade:                in.Grade,
		Altitude:             in.Altitude,
		BeanListNote:         in.BeanListNote,
		BomStatus:            in.BomStatus,
		Warnings:             append([]string(nil), in.Warnings...),
		YieldRate:            in.YieldRate,
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
	out.CommercialWholesaleTiers = buildCommercialWholesaleTiers(params, in, out.WholesaleKgPrices, out.WholesaleLbPrices)

	for _, multiplier := range in.WholesaleDripMultipliers {
		price := dripBase*multiplier + in.DripTaxAddPerBag100
		out.WholesaleDripBagPrices = append(out.WholesaleDripBagPrices, price)
		out.WholesaleDripBagWithPackPrices = append(out.WholesaleDripBagWithPackPrices, price+params.DripPackingMaterialPerBag)
	}

	out.RetailKgPrice = retailSmall*(1+params.RetailBeanMarginRate) + params.WholesalePackageCostPerKg + params.ProductLossPerKg + retailTax + params.RetailLogisticsPerKg
	out.Retail454gPrice = out.RetailKgPrice * params.KgToLbFactor
	out.Retail227gPrice = out.Retail454gPrice / 2
	out.Retail250gPrice = out.RetailKgPrice * 0.25
	out.Retail200gPrice = out.RetailKgPrice * 0.2
	out.Retail100gPrice = out.RetailKgPrice * 0.1
	out.RetailDrip10BagPrice = dripBase*10*params.RetailDripMultiplier + in.DripTaxAddPerBagRetail*10 + params.DripPackingMaterialPerBag + params.RetailDripLogisticsPer10Bags
	out.RetailBeanTiers = buildRetailBeanTiers(profileName, out)
	roundProductPrices(&out)
	return out
}

func calculateGreenBeanProduct(params Parameters, in ProductInput) ProductResult {
	displayName := strings.TrimSpace(in.BeanListTemplateName)
	if displayName == "" {
		displayName = strings.TrimSpace(in.Name)
	}
	code := fmt.Sprintf("G.%d", in.ProductID)
	if in.ProductID <= 0 {
		code = "G.0"
	}
	tiers := buildGreenBeanTemplateSaleTiers(params, in)
	bomStatus := "bom_cost_template_price"
	if len(tiers) == 0 {
		tiers = normalizeLegacyGreenBeanSaleTiers(in.GreenBeanSaleTiers)
		bomStatus = "missing_green_bean_template"
		if len(tiers) > 0 {
			bomStatus = "direct_sale_price"
		}
	}
	return ProductResult{
		ProductID:         in.ProductID,
		Name:              in.Name,
		ProductKind:       "green_bean",
		CustomerID:        in.CustomerID,
		BaseProductID:     in.BaseProductID,
		Visibility:        in.Visibility,
		CustomType:        in.CustomType,
		ProductCategoryID: in.ProductCategoryID,
		BeanListQuality:   in.BeanListQuality,
		GreenBeanList: BeanListDisplay{
			Code:           code,
			Category:       "生豆销售",
			DisplayName:    displayName,
			RecommendedUse: "生豆销售",
			Flavor:         in.Flavor,
			Description:    firstNonEmptyString(in.BeanListNote, in.Origin),
		},
		Flavor:             in.Flavor,
		Origin:             in.Origin,
		ProcessingStation:  in.ProcessingStation,
		Variety:            in.Variety,
		ProcessMethod:      in.ProcessMethod,
		Grade:              in.Grade,
		Altitude:           in.Altitude,
		BeanListNote:       in.BeanListNote,
		GreenBeanCostPerKg: in.GreenBeanCostPerKg,
		BomStatus:          bomStatus,
		GreenBeanSaleTiers: tiers,
	}
}

func buildGreenBeanTemplateSaleTiers(params Parameters, in ProductInput) []CommercialWholesaleTier {
	template := normalizeGradientTemplate(in.GradientTemplate)
	if template == nil {
		return nil
	}
	out := make([]CommercialWholesaleTier, 0, len(template.Tiers))
	for _, tier := range template.Tiers {
		margin := tier.MarginRate
		if in.MarginRateOverride != nil {
			margin = *in.MarginRateOverride
		}
		displayUnit := normalizeGradientDisplayUnit(template.DisplayUnit)
		specG := specGForGradientDisplayUnit(displayUnit)
		pricePerKg := roundPrice(in.GreenBeanCostPerKg * (1 + margin))
		pricePerLb := pricePerKg * params.KgToLbFactor
		pricePerUnit := pricePerKg
		switch displayUnit {
		case GradientDisplayUnitLb:
			pricePerUnit = roundPrice(pricePerLb)
		case GradientDisplayUnitKg:
			pricePerUnit = pricePerKg
		default:
			pricePerUnit = roundPrice(pricePerKg * float64(specG) / 1000.0)
		}
		minQty := roundQuantity(tier.MinWeightG / float64(specG))
		var maxQty *float64
		if tier.MaxWeightG != nil {
			v := roundQuantity(*tier.MaxWeightG / float64(specG))
			maxQty = &v
		}
		minLb := roundQuantity(tier.MinWeightG / 454.0)
		var maxLb *float64
		if tier.MaxWeightG != nil {
			v := roundQuantity(*tier.MaxWeightG / 454.0)
			maxLb = &v
		}
		out = append(out, CommercialWholesaleTier{
			Label:          tier.Label,
			Scheme:         "green_bean_template",
			SpecG:          int64(specG),
			MinQty:         minQty,
			MaxQty:         maxQty,
			PricePerUnit:   pricePerUnit,
			MinLb:          minLb,
			MaxLb:          maxLb,
			PricePerKg:     pricePerKg,
			PricePerLb:     pricePerLb,
			TemplateID:     template.ID,
			TemplateTierID: tier.ID,
			DisplayUnit:    displayUnit,
			MinWeightG:     tier.MinWeightG,
			MaxWeightG:     tier.MaxWeightG,
			MarginRate:     margin,
		})
	}
	return out
}

func normalizeLegacyGreenBeanSaleTiers(source []CommercialWholesaleTier) []CommercialWholesaleTier {
	tiers := make([]CommercialWholesaleTier, 0, len(source))
	for i, tier := range source {
		next := tier
		if strings.TrimSpace(next.Label) == "" {
			next.Label = greenBeanTierLabel(next)
		}
		if next.SpecG <= 0 {
			next.SpecG = 1000
		}
		if strings.TrimSpace(next.DisplayUnit) == "" {
			next.DisplayUnit = GradientDisplayUnitKg
		}
		if next.MinQty <= 0 {
			next.MinQty = float64(i + 1)
		}
		if next.PricePerUnit > 0 && next.PricePerLb == 0 {
			next.PricePerLb = next.PricePerUnit * 454.0 / float64(next.SpecG)
		}
		if next.PricePerUnit > 0 && next.PricePerKg == 0 {
			next.PricePerKg = next.PricePerUnit * 1000.0 / float64(next.SpecG)
		}
		next.Scheme = "green_bean_direct"
		tiers = append(tiers, next)
	}
	return tiers
}

func greenBeanTierLabel(tier CommercialWholesaleTier) string {
	min := tier.MinQty
	if min <= 0 {
		min = tier.MinLb
	}
	unit := "kg"
	switch tier.DisplayUnit {
	case GradientDisplayUnitLb:
		unit = "磅"
	case GradientDisplayUnit100G:
		unit = "100g"
	case GradientDisplayUnit227G:
		unit = "227g"
	case GradientDisplayUnit250G:
		unit = "250g"
	}
	if min <= 0 {
		return "全部数量"
	}
	return fmt.Sprintf("%s%s+", trimFloatZero(min), unit)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if s := strings.TrimSpace(value); s != "" {
			return s
		}
	}
	return ""
}

func trimFloatZero(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func normalizeBomStatus(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return "active"
	}
	return status
}

func normalizeWarnings(warnings []string) []string {
	out := make([]string, 0, len(warnings))
	seen := map[string]bool{}
	for _, warning := range warnings {
		warning = strings.TrimSpace(warning)
		if warning == "" || seen[warning] {
			continue
		}
		out = append(out, warning)
		seen[warning] = true
	}
	return out
}

func buildCommercialWholesaleTiers(params Parameters, in ProductInput, kgPrices, lbPrices []float64) []CommercialWholesaleTier {
	if template := normalizeGradientTemplate(in.GradientTemplate); template != nil {
		return buildGradientTemplateCommercialTiers(params, in, *template)
	}
	profileName := costingProfileName(in)
	type tierDef struct {
		label      string
		specG      int64
		minQty     float64
		maxQty     *float64
		priceIndex int
		priceMode  string
	}
	scheme := normalizeWholesaleTierScheme(in.WholesaleTierScheme)
	defs := []tierDef{
		{"2包-13包", 454, 2, ptrFloat64(13), 0, "lb"},
		{"14包-23包", 454, 14, ptrFloat64(23), 1, "lb"},
		{"24包-47包", 454, 24, ptrFloat64(47), 2, "lb"},
		{"48包+", 454, 48, nil, 3, "lb"},
	}
	switch scheme {
	case WholesaleTierSchemeKgThree:
		defs = []tierDef{
			{"24-49kg", 1000, 24, ptrFloat64(49), 3, "kg"},
			{"50-99kg", 1000, 50, ptrFloat64(99), 4, "kg"},
			{"100-199kg", 1000, 100, ptrFloat64(199), 5, "kg"},
		}
	case WholesaleTierScheme227GTwo:
		defs = []tierDef{
			{"2包-7包", 227, 2, ptrFloat64(7), 0, "half_lb"},
			{"8包+", 227, 8, nil, 1, "half_lb"},
		}
		if isMorningNayi(profileName) {
			defs[1].priceIndex = 2
		}
	}
	out := make([]CommercialWholesaleTier, 0, len(defs))
	for _, def := range defs {
		if def.priceIndex >= len(kgPrices) || def.priceIndex >= len(lbPrices) {
			break
		}
		pricePerKg := kgPrices[def.priceIndex]
		pricePerLb := lbPrices[def.priceIndex]
		pricePerUnit := pricePerLb
		if def.priceMode == "kg" {
			pricePerUnit = pricePerKg
		} else if def.priceMode == "half_lb" {
			pricePerUnit = pricePerLb / 2
		}
		out = append(out, CommercialWholesaleTier{
			Label:        def.label,
			Scheme:       scheme,
			SpecG:        def.specG,
			MinQty:       def.minQty,
			MaxQty:       def.maxQty,
			PricePerUnit: pricePerUnit,
			MinLb:        def.minQty,
			MaxLb:        def.maxQty,
			PricePerKg:   pricePerKg,
			PricePerLb:   pricePerLb,
		})
	}
	return applyCommercialTierOverrides(profileName, out)
}

type commercialPriceParts struct {
	RoastedCostPerKg    float64
	BaseCostPerKg       float64
	ProductionCostPerKg float64
	ProductionKey       string
	MarginRate          float64
	TaxAddPerKg         float64
	RawPricePerKg       float64
	RawPricePerLb       float64
	FinalPricePerKg     float64
	FinalPricePerLb     float64
	FinalPricePerUnit   float64
	DisplayUnit         string
}

func buildGradientTemplateCommercialTiers(params Parameters, in ProductInput, template GradientTemplate) []CommercialWholesaleTier {
	out := make([]CommercialWholesaleTier, 0, len(template.Tiers))
	for _, tier := range template.Tiers {
		parts := commercialPriceForGradientTier(params, in, template.DisplayUnit, tier, in.MarginRateOverride)
		specG := specGForGradientDisplayUnit(template.DisplayUnit)
		minQty := roundQuantity(tier.MinWeightG / float64(specG))
		var maxQty *float64
		if tier.MaxWeightG != nil {
			v := roundQuantity(*tier.MaxWeightG / float64(specG))
			maxQty = &v
		}
		minLb := roundQuantity(tier.MinWeightG / 454.0)
		var maxLb *float64
		if tier.MaxWeightG != nil {
			v := roundQuantity(*tier.MaxWeightG / 454.0)
			maxLb = &v
		}
		out = append(out, CommercialWholesaleTier{
			Label:          tier.Label,
			Scheme:         "gradient_template",
			SpecG:          int64(specG),
			MinQty:         minQty,
			MaxQty:         maxQty,
			PricePerUnit:   parts.FinalPricePerUnit,
			MinLb:          minLb,
			MaxLb:          maxLb,
			PricePerKg:     parts.FinalPricePerKg,
			PricePerLb:     parts.FinalPricePerLb,
			TemplateID:     template.ID,
			TemplateTierID: tier.ID,
			DisplayUnit:    template.DisplayUnit,
			MinWeightG:     tier.MinWeightG,
			MaxWeightG:     tier.MaxWeightG,
			MarginRate:     parts.MarginRate,
		})
	}
	return out
}

func commercialPriceForGradientTier(params Parameters, in ProductInput, displayUnit string, tier GradientTemplateTier, marginOverride *float64) commercialPriceParts {
	greenCost := in.GreenBeanCostPerKg
	yieldRate := in.YieldRate
	if yieldRate <= 0 {
		yieldRate = params.RoastYieldRate
	}
	roasted := greenCost / yieldRate
	productionCost := params.SmallBatchProductionCostPerKg
	productionKey := "small_batch_production_cost_per_kg"
	if tier.MinWeightG >= 10000 {
		productionCost = params.LargeBatchProductionCostPerKg
		productionKey = "large_batch_production_cost_per_kg"
	}
	base := roasted + productionCost
	margin := tier.MarginRate
	if marginOverride != nil {
		margin = *marginOverride
	}
	taxAdd := base * margin * params.RetailTaxRate
	rawKg := base*(1+margin) + params.WholesalePackageCostPerKg + params.ProductLossPerKg + taxAdd
	rawLb := rawKg*params.KgToLbFactor + 1
	finalKg := roundPrice(rawKg)
	finalLb := roundPrice(rawLb)
	normalizedDisplayUnit := normalizeGradientDisplayUnit(displayUnit)
	finalUnit := finalLb
	if normalizedDisplayUnit == GradientDisplayUnitKg {
		finalUnit = finalKg
	} else if normalizedDisplayUnit != GradientDisplayUnitLb {
		finalUnit = roundPrice(rawKg * float64(specGForGradientDisplayUnit(normalizedDisplayUnit)) / 1000.0)
	}
	return commercialPriceParts{
		RoastedCostPerKg:    roasted,
		BaseCostPerKg:       base,
		ProductionCostPerKg: productionCost,
		ProductionKey:       productionKey,
		MarginRate:          margin,
		TaxAddPerKg:         taxAdd,
		RawPricePerKg:       rawKg,
		RawPricePerLb:       rawLb,
		FinalPricePerKg:     finalKg,
		FinalPricePerLb:     finalLb,
		FinalPricePerUnit:   finalUnit,
		DisplayUnit:         normalizedDisplayUnit,
	}
}

func ExplainCommercialPrice(params Parameters, in ProductInput, req PriceExplanationRequest) (PriceExplanation, error) {
	validated, err := ValidateProductInput(params, in)
	if err != nil {
		return PriceExplanation{}, err
	}
	template := normalizeGradientTemplate(validated.GradientTemplate)
	if template == nil {
		return PriceExplanation{}, fmt.Errorf("gradient template required")
	}
	label := strings.TrimSpace(req.TierLabel)
	var tier GradientTemplateTier
	found := false
	for _, row := range template.Tiers {
		if label == "" || row.Label == label {
			tier = row
			found = true
			break
		}
	}
	if !found {
		return PriceExplanation{}, fmt.Errorf("tier not found")
	}
	saved := commercialPriceForGradientTier(params, validated, template.DisplayUnit, tier, validated.MarginRateOverride)
	previewInput := validated
	if req.Overrides.GreenBeanCostPerKg != nil {
		previewInput.GreenBeanCostPerKg = *req.Overrides.GreenBeanCostPerKg
	}
	if req.Overrides.YieldRate != nil {
		previewInput.YieldRate = *req.Overrides.YieldRate
	}
	previewMarginOverride := validated.MarginRateOverride
	if req.Overrides.MarginRate != nil {
		previewMarginOverride = req.Overrides.MarginRate
	}
	preview := commercialPriceForGradientTier(params, previewInput, template.DisplayUnit, tier, previewMarginOverride)
	marginSource := "gradient_template"
	if validated.MarginRateOverride != nil {
		marginSource = "product_margin_override"
	}
	if req.Overrides.MarginRate != nil {
		marginSource = "temporary_override"
	}
	return PriceExplanation{
		ProductID:         validated.ProductID,
		ProductName:       validated.Name,
		TemplateID:        template.ID,
		TemplateName:      template.Name,
		TierLabel:         tier.Label,
		DisplayUnit:       template.DisplayUnit,
		MinWeightG:        tier.MinWeightG,
		MaxWeightG:        tier.MaxWeightG,
		SavedFinalPrice:   saved.FinalPricePerUnit,
		PreviewFinalPrice: preview.FinalPricePerUnit,
		Steps: []PriceExplanationStep{
			{Key: "green_bean_cost_per_kg", Label: "生豆成本", Source: "product", Value: previewInput.GreenBeanCostPerKg, Unit: "元/kg", Changed: req.Overrides.GreenBeanCostPerKg != nil},
			{Key: "yield_rate", Label: "出成率", Source: "product_bom", Value: previewInput.YieldRate, Unit: "ratio", Changed: req.Overrides.YieldRate != nil},
			{Key: "roasted_bean_cost_per_kg", Label: "熟豆成本", Source: "formula", Value: preview.RoastedCostPerKg, Unit: "元/kg", Changed: req.Overrides.GreenBeanCostPerKg != nil || req.Overrides.YieldRate != nil},
			{Key: preview.ProductionKey, Label: "生产成本", Source: "cost_parameter", Value: preview.ProductionCostPerKg, Unit: "元/kg"},
			{Key: "wholesale_package_cost_per_kg", Label: "批发包装成本", Source: "cost_parameter", Value: params.WholesalePackageCostPerKg, Unit: "元/kg"},
			{Key: "product_loss_per_kg", Label: "产品损耗", Source: "cost_parameter", Value: params.ProductLossPerKg, Unit: "元/kg"},
			{Key: "retail_tax_rate", Label: "税费比例", Source: "cost_parameter", Value: params.RetailTaxRate, Unit: "ratio"},
			{Key: "template_margin_rate", Label: "利润率", Source: marginSource, Value: preview.MarginRate, Unit: "ratio", Changed: req.Overrides.MarginRate != nil},
			{Key: "display_unit_conversion", Label: "展示单位克重", Source: "gradient_template", Value: float64(specGForGradientDisplayUnit(template.DisplayUnit)), Unit: "g"},
			{Key: "final_price", Label: "最终价格", Source: "formula", Value: preview.FinalPricePerUnit, Unit: template.DisplayUnit, Changed: preview.FinalPricePerUnit != saved.FinalPricePerUnit},
		},
	}, nil
}

func normalizeGradientTemplate(template *GradientTemplate) *GradientTemplate {
	if template == nil || len(template.Tiers) == 0 {
		return nil
	}
	out := *template
	out.Name = strings.TrimSpace(out.Name)
	out.DisplayUnit = normalizeGradientDisplayUnit(out.DisplayUnit)
	out.Tiers = append([]GradientTemplateTier(nil), template.Tiers...)
	sort.SliceStable(out.Tiers, func(i, j int) bool {
		if out.Tiers[i].Position != out.Tiers[j].Position {
			return out.Tiers[i].Position < out.Tiers[j].Position
		}
		if out.Tiers[i].MinWeightG != out.Tiers[j].MinWeightG {
			return out.Tiers[i].MinWeightG < out.Tiers[j].MinWeightG
		}
		return out.Tiers[i].ID < out.Tiers[j].ID
	})
	filtered := make([]GradientTemplateTier, 0, len(out.Tiers))
	for _, tier := range out.Tiers {
		tier.Label = strings.TrimSpace(tier.Label)
		if tier.Label == "" {
			tier.Label = fmt.Sprintf("%.0fg+", tier.MinWeightG)
		}
		if tier.MinWeightG <= 0 || tier.MarginRate < 0 {
			continue
		}
		if tier.MaxWeightG != nil && *tier.MaxWeightG <= tier.MinWeightG {
			continue
		}
		filtered = append(filtered, tier)
	}
	if len(filtered) == 0 {
		return nil
	}
	out.Tiers = filtered
	return &out
}

func normalizeGradientDisplayUnit(unit string) string {
	switch strings.TrimSpace(unit) {
	case GradientDisplayUnitKg:
		return GradientDisplayUnitKg
	case GradientDisplayUnit227G:
		return GradientDisplayUnit227G
	case GradientDisplayUnit100G:
		return GradientDisplayUnit100G
	case GradientDisplayUnit250G:
		return GradientDisplayUnit250G
	case GradientDisplayUnitLb:
		return GradientDisplayUnitLb
	default:
		return GradientDisplayUnitLb
	}
}

func specGForGradientDisplayUnit(unit string) int {
	switch normalizeGradientDisplayUnit(unit) {
	case GradientDisplayUnitKg:
		return 1000
	case GradientDisplayUnit227G:
		return 227
	case GradientDisplayUnit100G:
		return 100
	case GradientDisplayUnit250G:
		return 250
	default:
		return 454
	}
}

func roundQuantity(v float64) float64 {
	return math.Round(v*100) / 100
}

func costingProfileName(in ProductInput) string {
	name := strings.TrimSpace(in.BeanListTemplateName)
	if name != "" {
		return name
	}
	return in.Name
}

func buildRetailBeanTiers(name string, out ProductResult) []RetailBeanTier {
	if isRetailSmallPackBean(name) {
		return []RetailBeanTier{
			{Label: "100g", SpecG: 100, PricePerUnit: out.Retail100gPrice},
			{Label: "200g", SpecG: 200, PricePerUnit: out.Retail200gPrice},
		}
	}
	return []RetailBeanTier{
		{Label: "227g", SpecG: 227, PricePerUnit: out.Retail227gPrice},
		{Label: "250g", SpecG: 250, PricePerUnit: out.Retail250gPrice},
	}
}

func roundProductPrices(out *ProductResult) {
	for i := range out.WholesaleKgPrices {
		out.WholesaleKgPrices[i] = roundPrice(out.WholesaleKgPrices[i])
	}
	for i := range out.WholesaleLbPrices {
		out.WholesaleLbPrices[i] = roundPrice(out.WholesaleLbPrices[i])
	}
	for i := range out.CommercialWholesaleTiers {
		out.CommercialWholesaleTiers[i].PricePerKg = roundPrice(out.CommercialWholesaleTiers[i].PricePerKg)
		out.CommercialWholesaleTiers[i].PricePerLb = roundPrice(out.CommercialWholesaleTiers[i].PricePerLb)
		out.CommercialWholesaleTiers[i].PricePerUnit = roundPrice(out.CommercialWholesaleTiers[i].PricePerUnit)
	}
	for i := range out.WholesaleDripBagPrices {
		out.WholesaleDripBagPrices[i] = roundPrice(out.WholesaleDripBagPrices[i])
	}
	for i := range out.WholesaleDripBagWithPackPrices {
		out.WholesaleDripBagWithPackPrices[i] = roundPrice(out.WholesaleDripBagWithPackPrices[i])
	}
	out.RetailKgPrice = roundPrice(out.RetailKgPrice)
	out.Retail454gPrice = roundPrice(out.Retail454gPrice)
	out.Retail227gPrice = roundPrice(out.Retail227gPrice)
	out.Retail250gPrice = roundPrice(out.Retail250gPrice)
	out.Retail200gPrice = roundPrice(out.Retail200gPrice)
	out.Retail100gPrice = roundPrice(out.Retail100gPrice)
	out.RetailDrip10BagPrice = roundPrice(out.RetailDrip10BagPrice)
	for i := range out.RetailBeanTiers {
		out.RetailBeanTiers[i].PricePerUnit = roundPrice(out.RetailBeanTiers[i].PricePerUnit)
	}
}

func roundPrice(v float64) float64 {
	return math.Round(v)
}

func ptrFloat64(v float64) *float64 {
	return &v
}

func normalizeWholesaleTierScheme(s string) string {
	switch strings.TrimSpace(s) {
	case WholesaleTierSchemeKgThree:
		return WholesaleTierSchemeKgThree
	case WholesaleTierScheme227GTwo:
		return WholesaleTierScheme227GTwo
	default:
		return WholesaleTierScheme454GFour
	}
}

func inferWholesaleTierScheme(name string) string {
	switch {
	case isCookieBlend(name):
		return WholesaleTierSchemeKgThree
	case containsAnyNormalized(name, []string{"白月光", "芸上莓梦", "晨曦", "晚香玉"}):
		return WholesaleTierScheme227GTwo
	default:
		return WholesaleTierScheme454GFour
	}
}

func normalizeWholesaleMarginRates(params Parameters, rates []float64) []float64 {
	fallback := []float64{0.5421052631578949, 0.3842105263157895, 0.27894736842105267, 0.2, 0.12, 0.045}
	for i := range fallback {
		if i < len(params.WholesaleKgMarginRates) && params.WholesaleKgMarginRates[i] > 0 {
			fallback[i] = params.WholesaleKgMarginRates[i]
		}
	}
	out := append([]float64(nil), fallback...)
	for i := range rates {
		if i < len(out) {
			out[i] = rates[i]
		} else {
			out = append(out, rates[i])
		}
	}
	return out
}

func premiumWholesaleMarginRates(params Parameters) []float64 {
	base := normalizeWholesaleMarginRates(params, nil)
	out := append([]float64(nil), base...)
	for i := 0; i < 4 && i < len(out); i++ {
		out[i] = out[i] * 1.5
	}
	return out
}

func premiumFirstThreeWholesaleMarginRates(params Parameters) []float64 {
	base := normalizeWholesaleMarginRates(params, nil)
	out := append([]float64(nil), base...)
	for i := 0; i < 3 && i < len(out); i++ {
		out[i] = out[i] * 1.5
	}
	return out
}

func wineSunWholesaleMarginRates(params Parameters) []float64 {
	out := normalizeWholesaleMarginRates(params, nil)
	if len(out) > 3 {
		out[3] = 0.17
	}
	return out
}

func cookieWholesaleMarginRates(params Parameters) []float64 {
	out := normalizeWholesaleMarginRates(params, nil)
	out[3] = 0.175
	out[4] = 0.12
	out[5] = 0.045
	return out
}

func defaultWholesaleTaxAddPerKgTiers(params Parameters, in ProductInput) []float64 {
	rates := defaultWholesaleTaxMarginRates(params)
	roasted := in.GreenBeanCostPerKg / in.YieldRate
	small := roasted + params.SmallBatchProductionCostPerKg
	out := make([]float64, len(in.WholesaleKgMarginRates))
	for i := range out {
		rate := rates[0]
		if i < len(rates) {
			rate = rates[i]
		}
		out[i] = small * rate * params.RetailTaxRate
	}
	return out
}

func defaultWholesaleTaxMarginRates(params Parameters) []float64 {
	rates := normalizeWholesaleMarginRates(params, nil)
	if len(rates) > 3 {
		rates[3] = 0.15
	}
	return rates
}

func isCookieBlend(name string) bool {
	return containsAnyNormalized(name, []string{"曲奇"})
}

func isWineSunBean(name string) bool {
	return containsAnyNormalized(name, []string{"酒心巧克力"})
}

func isYirgacheffeG2(name string) bool {
	return containsAnyNormalized(name, []string{"耶加雪菲G2", "耶加雪菲g2"})
}

func isPremiumCommercialBean(name string) bool {
	return containsAnyNormalized(name, []string{
		"Nenka", "嫩咖",
		"TOPAA", "肯尼亚",
		"浣纱", "果园",
		"Uraga", "乌拉嘎",
		"曼特宁", "森林瑰夏", "白月光", "芸上莓梦", "晨曦", "晚香玉",
		"萨奇姆", "芒霜", "小菠萝", "菠萝意式",
	})
}

func isMorningNayi(name string) bool {
	return containsAnyNormalized(name, []string{"晨曦", "娜伊", "娜依"})
}

func isRetailSmallPackBean(name string) bool {
	return containsAnyNormalized(name, []string{"白月光", "芸上莓梦", "晨曦", "晚香玉"})
}

func excelRetailGreenCostOverride(name string) (float64, bool) {
	if containsAnyNormalized(name, []string{"红岩"}) {
		return 57.9, true
	}
	return 0, false
}

func applyCommercialTierOverrides(name string, tiers []CommercialWholesaleTier) []CommercialWholesaleTier {
	keep := map[string]bool{}
	switch {
	case containsAnyNormalized(name, []string{"浣纱", "肯尼亚", "萨奇姆"}):
		keep["2包-13包"] = true
		keep["14包-23包"] = true
		keep["48包+"] = true
	}
	if len(keep) > 0 {
		filtered := make([]CommercialWholesaleTier, 0, len(keep))
		for _, tier := range tiers {
			if keep[tier.Label] {
				filtered = append(filtered, tier)
			}
		}
		tiers = filtered
	}
	if containsAnyNormalized(name, []string{"浣纱"}) {
		for i := range tiers {
			if tiers[i].Label == "48包+" {
				tiers[i].PricePerUnit = 100
				tiers[i].PricePerLb = 100
				tiers[i].PricePerKg = (100 - 1) / 0.454
			}
		}
	}
	return tiers
}

func containsAnyNormalized(s string, needles []string) bool {
	n := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), " ", ""))
	for _, needle := range needles {
		if strings.Contains(n, strings.ToLower(strings.ReplaceAll(needle, " ", ""))) {
			return true
		}
	}
	return false
}
