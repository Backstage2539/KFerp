package finance

import (
	"math"
	"strings"
)

type Money float64

const (
	CompanyTypeRoaster   = "coffee_roaster"
	CompanyTypeTrader    = "coffee_trader"
	CompanyTypeProcessor = "coffee_processor"
	CompanyTypeCombined  = "combined"

	TaxpayerSmallScale = "small_scale"
	TaxpayerGeneral    = "general"

	ClosingModeStrongLock        = "strong_lock"
	ClosingModeLightConfirmation = "light_confirmation"

	MonthStatusDraft    = "draft"
	MonthStatusClosed   = "closed"
	MonthStatusAdjusted = "adjusted"

	AdjustmentRevenue  = "revenue"
	AdjustmentMainCost = "main_cost"
	AdjustmentExpense  = "expense"
	AdjustmentTax      = "tax"
	AdjustmentOther    = "other"
)

type Settings struct {
	CompanyType                     string  `json:"company_type"`
	TaxpayerType                    string  `json:"taxpayer_type"`
	DeclarationPeriod               string  `json:"declaration_period"`
	ClosingMode                     string  `json:"closing_mode"`
	SmallScaleVATRate               float64 `json:"small_scale_vat_rate"`
	SmallScaleVATThreshold          Money   `json:"small_scale_vat_threshold"`
	GeneralOutputVATRate            float64 `json:"general_output_vat_rate"`
	DefaultInputVATRate             float64 `json:"default_input_vat_rate"`
	SurtaxRate                      float64 `json:"surtax_rate"`
	CITStandardRate                 float64 `json:"cit_standard_rate"`
	SmallLowProfitEnabled           bool    `json:"small_low_profit_enabled"`
	SmallLowProfitEffectiveRate     float64 `json:"small_low_profit_effective_rate"`
	SmallLowProfitAnnualProfitLimit Money   `json:"small_low_profit_annual_profit_limit"`
}

type MonthlySourceTotals struct {
	Month                 string `json:"month"`
	RevenueTaxInclusive   Money  `json:"revenue_tax_inclusive"`
	MainBusinessCost      Money  `json:"main_business_cost"`
	PeriodExpenses        Money  `json:"period_expenses"`
	InputVAT              Money  `json:"input_vat"`
	NonDeductibleInputVAT Money  `json:"non_deductible_input_vat"`
}

type TaxEstimate struct {
	TaxpayerType        string `json:"taxpayer_type"`
	OutputVAT           Money  `json:"output_vat"`
	InputVAT            Money  `json:"input_vat"`
	NonDeductibleInput  Money  `json:"non_deductible_input_vat"`
	DeductibleInputVAT  Money  `json:"deductible_input_vat"`
	VATPayable          Money  `json:"vat_payable"`
	Surtax              Money  `json:"surtax"`
	CITTaxableIncome    Money  `json:"cit_taxable_income"`
	CITPayable          Money  `json:"cit_payable"`
	CITPreferenceSaving Money  `json:"cit_preference_saving"`
	TotalTax            Money  `json:"total_tax"`
	EstimateNote        string `json:"estimate_note"`
}

type MonthlyReport struct {
	Month                       string           `json:"month"`
	Status                      string           `json:"status"`
	RevenueTaxInclusive         Money            `json:"revenue_tax_inclusive"`
	TaxExclusiveRevenue         Money            `json:"tax_exclusive_revenue"`
	MainBusinessCost            Money            `json:"main_business_cost"`
	PeriodExpenses              Money            `json:"period_expenses"`
	GrossProfit                 Money            `json:"gross_profit"`
	GrossMargin                 float64          `json:"gross_margin"`
	Tax                         TaxEstimate      `json:"tax"`
	OperatingNetProfit          Money            `json:"operating_net_profit"`
	AppliedAdjustments          map[string]Money `json:"applied_adjustments"`
	AdjustedTaxExclusiveRevenue Money            `json:"adjusted_tax_exclusive_revenue"`
	AdjustedMainBusinessCost    Money            `json:"adjusted_main_business_cost"`
	AdjustedPeriodExpenses      Money            `json:"adjusted_period_expenses"`
	AdjustedTaxTotal            Money            `json:"adjusted_tax_total"`
	AdjustedGrossProfit         Money            `json:"adjusted_gross_profit"`
	AdjustedNetProfit           Money            `json:"adjusted_net_profit"`
}

type Adjustment struct {
	Type   string `json:"type"`
	Amount Money  `json:"amount"`
}

func DefaultSettings() Settings {
	return Settings{
		CompanyType:                     CompanyTypeRoaster,
		TaxpayerType:                    TaxpayerSmallScale,
		DeclarationPeriod:               "monthly",
		ClosingMode:                     ClosingModeStrongLock,
		SmallScaleVATRate:               0.03,
		SmallScaleVATThreshold:          100000,
		GeneralOutputVATRate:            0.13,
		DefaultInputVATRate:             0.13,
		SurtaxRate:                      0.12,
		CITStandardRate:                 0.25,
		SmallLowProfitEnabled:           true,
		SmallLowProfitEffectiveRate:     0.05,
		SmallLowProfitAnnualProfitLimit: 3000000,
	}
}

func NormalizeSettings(s Settings) Settings {
	defaults := DefaultSettings()
	if strings.TrimSpace(s.CompanyType) == "" {
		s.CompanyType = defaults.CompanyType
	}
	if strings.TrimSpace(s.TaxpayerType) == "" {
		s.TaxpayerType = defaults.TaxpayerType
	}
	if strings.TrimSpace(s.DeclarationPeriod) == "" {
		s.DeclarationPeriod = defaults.DeclarationPeriod
	}
	if strings.TrimSpace(s.ClosingMode) == "" {
		s.ClosingMode = defaults.ClosingMode
	}
	if s.SmallScaleVATRate <= 0 {
		s.SmallScaleVATRate = defaults.SmallScaleVATRate
	}
	if s.GeneralOutputVATRate <= 0 {
		s.GeneralOutputVATRate = defaults.GeneralOutputVATRate
	}
	if s.CITStandardRate <= 0 {
		s.CITStandardRate = defaults.CITStandardRate
	}
	if s.SmallLowProfitEffectiveRate <= 0 {
		s.SmallLowProfitEffectiveRate = defaults.SmallLowProfitEffectiveRate
	}
	if s.SmallLowProfitAnnualProfitLimit <= 0 {
		s.SmallLowProfitAnnualProfitLimit = defaults.SmallLowProfitAnnualProfitLimit
	}
	return s
}

func BuildMonthlyReport(settings Settings, totals MonthlySourceTotals) MonthlyReport {
	settings = NormalizeSettings(settings)
	taxExclusiveRevenue, tax := estimateTax(settings, totals)
	grossProfit := roundMoney(taxExclusiveRevenue - totals.MainBusinessCost)
	if taxExclusiveRevenue != 0 {
		tax.TotalTax = roundMoney(tax.Surtax + tax.CITPayable)
	}
	netProfit := roundMoney(taxExclusiveRevenue - totals.MainBusinessCost - totals.PeriodExpenses - tax.Surtax - tax.CITPayable)
	report := MonthlyReport{
		Month:                       strings.TrimSpace(totals.Month),
		Status:                      MonthStatusDraft,
		RevenueTaxInclusive:         roundMoney(totals.RevenueTaxInclusive),
		TaxExclusiveRevenue:         taxExclusiveRevenue,
		MainBusinessCost:            roundMoney(totals.MainBusinessCost),
		PeriodExpenses:              roundMoney(totals.PeriodExpenses),
		GrossProfit:                 grossProfit,
		GrossMargin:                 ratio(grossProfit, taxExclusiveRevenue),
		Tax:                         tax,
		OperatingNetProfit:          netProfit,
		AppliedAdjustments:          map[string]Money{},
		AdjustedTaxExclusiveRevenue: taxExclusiveRevenue,
		AdjustedMainBusinessCost:    roundMoney(totals.MainBusinessCost),
		AdjustedPeriodExpenses:      roundMoney(totals.PeriodExpenses),
		AdjustedTaxTotal:            tax.TotalTax,
		AdjustedGrossProfit:         grossProfit,
		AdjustedNetProfit:           netProfit,
	}
	return report
}

func estimateTax(settings Settings, totals MonthlySourceTotals) (Money, TaxEstimate) {
	switch settings.TaxpayerType {
	case TaxpayerGeneral:
		return estimateGeneralTaxpayer(settings, totals)
	default:
		return estimateSmallScale(settings, totals)
	}
}

func estimateSmallScale(settings Settings, totals MonthlySourceTotals) (Money, TaxEstimate) {
	exclusive := taxExclusive(totals.RevenueTaxInclusive, settings.SmallScaleVATRate)
	outputVAT := roundMoney(totals.RevenueTaxInclusive - exclusive)
	vatPayable := outputVAT
	if settings.SmallScaleVATThreshold > 0 && exclusive <= settings.SmallScaleVATThreshold {
		vatPayable = 0
	}
	surtax := roundMoney(vatPayable * Money(settings.SurtaxRate))
	citTaxable := positiveMoney(exclusive - totals.MainBusinessCost - totals.PeriodExpenses - surtax)
	citPayable, preference := estimateCIT(settings, citTaxable)
	return exclusive, TaxEstimate{
		TaxpayerType:        TaxpayerSmallScale,
		OutputVAT:           outputVAT,
		VATPayable:          vatPayable,
		Surtax:              surtax,
		CITTaxableIncome:    citTaxable,
		CITPayable:          citPayable,
		CITPreferenceSaving: preference,
		TotalTax:            roundMoney(surtax + citPayable),
		EstimateNote:        "税费为经营管理估算，不作为正式申报结果。",
	}
}

func estimateGeneralTaxpayer(settings Settings, totals MonthlySourceTotals) (Money, TaxEstimate) {
	exclusive := taxExclusive(totals.RevenueTaxInclusive, settings.GeneralOutputVATRate)
	outputVAT := roundMoney(totals.RevenueTaxInclusive - exclusive)
	deductible := positiveMoney(totals.InputVAT - totals.NonDeductibleInputVAT)
	vatPayable := positiveMoney(outputVAT - deductible)
	surtax := roundMoney(vatPayable * Money(settings.SurtaxRate))
	citTaxable := positiveMoney(exclusive - totals.MainBusinessCost - totals.PeriodExpenses - surtax)
	citPayable, preference := estimateCIT(settings, citTaxable)
	return exclusive, TaxEstimate{
		TaxpayerType:        TaxpayerGeneral,
		OutputVAT:           outputVAT,
		InputVAT:            roundMoney(totals.InputVAT),
		NonDeductibleInput:  roundMoney(totals.NonDeductibleInputVAT),
		DeductibleInputVAT:  deductible,
		VATPayable:          vatPayable,
		Surtax:              surtax,
		CITTaxableIncome:    citTaxable,
		CITPayable:          citPayable,
		CITPreferenceSaving: preference,
		TotalTax:            roundMoney(surtax + citPayable),
		EstimateNote:        "税费为经营管理估算，不作为正式申报结果。",
	}
}

func estimateCIT(settings Settings, taxable Money) (Money, Money) {
	if taxable <= 0 {
		return 0, 0
	}
	standard := roundMoney(taxable * Money(settings.CITStandardRate))
	rate := settings.CITStandardRate
	if settings.SmallLowProfitEnabled && taxable*12 <= settings.SmallLowProfitAnnualProfitLimit {
		rate = settings.SmallLowProfitEffectiveRate
	}
	payable := roundMoney(taxable * Money(rate))
	return payable, positiveMoney(standard - payable)
}

func CanEditSourceDocument(settings Settings, monthStatus string) bool {
	settings = NormalizeSettings(settings)
	switch strings.TrimSpace(monthStatus) {
	case MonthStatusClosed, MonthStatusAdjusted:
		return settings.ClosingMode != ClosingModeStrongLock
	default:
		return true
	}
}

func ApplyAdjustments(report MonthlyReport, adjustments []Adjustment) MonthlyReport {
	if report.AppliedAdjustments == nil {
		report.AppliedAdjustments = map[string]Money{}
	}
	revenue := firstNonZero(report.AdjustedTaxExclusiveRevenue, report.TaxExclusiveRevenue)
	mainCost := firstNonZero(report.AdjustedMainBusinessCost, report.MainBusinessCost)
	expenses := firstNonZero(report.AdjustedPeriodExpenses, report.PeriodExpenses)
	taxTotal := firstNonZero(report.AdjustedTaxTotal, report.Tax.Surtax+report.Tax.CITPayable)
	for _, adjustment := range adjustments {
		kind := normalizeAdjustmentType(adjustment.Type)
		amount := roundMoney(adjustment.Amount)
		report.AppliedAdjustments[kind] = roundMoney(report.AppliedAdjustments[kind] + amount)
		switch kind {
		case AdjustmentRevenue:
			revenue = roundMoney(revenue + amount)
		case AdjustmentMainCost:
			mainCost = roundMoney(mainCost + amount)
		case AdjustmentExpense:
			expenses = roundMoney(expenses + amount)
		case AdjustmentTax:
			taxTotal = roundMoney(taxTotal + amount)
		}
	}
	report.AdjustedTaxExclusiveRevenue = revenue
	report.AdjustedMainBusinessCost = mainCost
	report.AdjustedPeriodExpenses = expenses
	report.AdjustedTaxTotal = taxTotal
	report.AdjustedGrossProfit = roundMoney(revenue - mainCost)
	report.AdjustedNetProfit = roundMoney(revenue - mainCost - expenses - taxTotal)
	if len(adjustments) > 0 && report.Status == MonthStatusClosed {
		report.Status = MonthStatusAdjusted
	}
	return report
}

func normalizeAdjustmentType(kind string) string {
	switch strings.TrimSpace(kind) {
	case AdjustmentRevenue, AdjustmentMainCost, AdjustmentExpense, AdjustmentTax:
		return strings.TrimSpace(kind)
	default:
		return AdjustmentOther
	}
}

func taxExclusive(inclusive Money, rate float64) Money {
	if rate <= 0 {
		return roundMoney(inclusive)
	}
	return roundMoney(inclusive / Money(1+rate))
}

func ratio(numerator, denominator Money) float64 {
	if denominator == 0 {
		return 0
	}
	return math.Round(float64(numerator/denominator)*10000) / 10000
}

func firstNonZero(primary, fallback Money) Money {
	if primary != 0 {
		return primary
	}
	return fallback
}

func positiveMoney(value Money) Money {
	if value <= 0 {
		return 0
	}
	return roundMoney(value)
}

func roundMoney(value Money) Money {
	return Money(math.Round(float64(value)*100) / 100)
}
