package finance

import "testing"

func TestSmallScaleTaxEstimateAndOperatingProfit(t *testing.T) {
	settings := DefaultSettings()
	settings.TaxpayerType = TaxpayerSmallScale
	settings.SmallScaleVATRate = 0.03
	settings.SmallScaleVATThreshold = 0
	settings.SurtaxRate = 0.12
	settings.CITStandardRate = 0.25
	settings.SmallLowProfitEnabled = true
	settings.SmallLowProfitEffectiveRate = 0.05
	settings.SmallLowProfitAnnualProfitLimit = 3000000

	report := BuildMonthlyReport(settings, MonthlySourceTotals{
		Month:               "2026-05",
		RevenueTaxInclusive: 103000,
		MainBusinessCost:    45000,
		PeriodExpenses:      12000,
	})

	assertMoney(t, "tax exclusive revenue", report.TaxExclusiveRevenue, 100000)
	assertMoney(t, "vat payable", report.Tax.VATPayable, 3000)
	assertMoney(t, "surtax", report.Tax.Surtax, 360)
	assertMoney(t, "cit taxable income", report.Tax.CITTaxableIncome, 42640)
	assertMoney(t, "cit", report.Tax.CITPayable, 2132)
	assertMoney(t, "cit preference", report.Tax.CITPreferenceSaving, 8528)
	assertMoney(t, "operating net profit", report.OperatingNetProfit, 40508)
}

func TestSmallScaleThresholdExemptsVAT(t *testing.T) {
	settings := DefaultSettings()
	settings.TaxpayerType = TaxpayerSmallScale
	settings.SmallScaleVATRate = 0.03
	settings.SmallScaleVATThreshold = 100000
	settings.SurtaxRate = 0.12

	report := BuildMonthlyReport(settings, MonthlySourceTotals{
		Month:               "2026-05",
		RevenueTaxInclusive: 82400,
		MainBusinessCost:    30000,
		PeriodExpenses:      10000,
	})

	assertMoney(t, "tax exclusive revenue", report.TaxExclusiveRevenue, 80000)
	assertMoney(t, "vat payable", report.Tax.VATPayable, 0)
	assertMoney(t, "surtax", report.Tax.Surtax, 0)
}

func TestGeneralTaxpayerUsesOutputMinusDeductibleInputVAT(t *testing.T) {
	settings := DefaultSettings()
	settings.TaxpayerType = TaxpayerGeneral
	settings.GeneralOutputVATRate = 0.13
	settings.SurtaxRate = 0.12

	report := BuildMonthlyReport(settings, MonthlySourceTotals{
		Month:                 "2026-05",
		RevenueTaxInclusive:   113000,
		MainBusinessCost:      52000,
		PeriodExpenses:        8000,
		InputVAT:              9000,
		NonDeductibleInputVAT: 1500,
	})

	assertMoney(t, "tax exclusive revenue", report.TaxExclusiveRevenue, 100000)
	assertMoney(t, "output vat", report.Tax.OutputVAT, 13000)
	assertMoney(t, "deductible input vat", report.Tax.DeductibleInputVAT, 7500)
	assertMoney(t, "vat payable", report.Tax.VATPayable, 5500)
	assertMoney(t, "surtax", report.Tax.Surtax, 660)
}

func TestStrongLockRequiresAdjustmentsForClosedMonth(t *testing.T) {
	settings := DefaultSettings()
	settings.ClosingMode = ClosingModeStrongLock

	if CanEditSourceDocument(settings, MonthStatusClosed) {
		t.Fatal("strong lock must not allow direct source edits after close")
	}
	if !CanEditSourceDocument(Settings{ClosingMode: ClosingModeLightConfirmation}, MonthStatusClosed) {
		t.Fatal("light confirmation should allow direct source edits")
	}
	if !CanEditSourceDocument(settings, MonthStatusDraft) {
		t.Fatal("draft months should remain editable")
	}
}

func TestApplyAmountAdjustmentsUpdatesReportTotals(t *testing.T) {
	report := MonthlyReport{
		Month:               "2026-05",
		TaxExclusiveRevenue: 100000,
		MainBusinessCost:    42000,
		PeriodExpenses:      10000,
		Tax:                 TaxEstimate{Surtax: 300, CITPayable: 2385},
		OperatingNetProfit:  45315,
		AppliedAdjustments:  map[string]Money{},
		AdjustedNetProfit:   45315,
		AdjustedGrossProfit: 58000,
	}

	adjusted := ApplyAdjustments(report, []Adjustment{
		{Type: AdjustmentRevenue, Amount: 2000},
		{Type: AdjustmentMainCost, Amount: 500},
		{Type: AdjustmentExpense, Amount: -300},
		{Type: AdjustmentTax, Amount: 100},
	})

	assertMoney(t, "adjusted revenue", adjusted.AdjustedTaxExclusiveRevenue, 102000)
	assertMoney(t, "adjusted cost", adjusted.AdjustedMainBusinessCost, 42500)
	assertMoney(t, "adjusted expense", adjusted.AdjustedPeriodExpenses, 9700)
	assertMoney(t, "adjusted tax", adjusted.AdjustedTaxTotal, 2785)
	assertMoney(t, "adjusted gross profit", adjusted.AdjustedGrossProfit, 59500)
	assertMoney(t, "adjusted net profit", adjusted.AdjustedNetProfit, 47015)
}

func assertMoney(t *testing.T, label string, got, want Money) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %.2f, want %.2f", label, got, want)
	}
}
