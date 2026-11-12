package finance

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	domain "orderapp/internal/domain/finance"
)

func TestDailyLedgerMatchesReportAndPreservesUnknownCost(t *testing.T) {
	repo := newFakeRepo()
	repo.reportStatus = domain.MonthStatusDraft
	repo.totals = domain.MonthlySourceTotals{Month: "2026-05", RevenueTaxInclusive: 1030, MainBusinessCost: 600, PeriodExpenses: 50, CostUnavailable: true}
	repo.sourceDetails = []SourceDetail{
		{Section: "revenue", Date: "2026-05-02", Amount: 1030},
		{Section: "main_cost", Date: "2026-05-02", Amount: 600},
		{Section: "main_cost", Date: "2026-05-02", CostUnavailable: true, PendingReason: "缺少成本"},
		{Section: "period_expense", Date: "2026-05-03", Amount: 50},
	}
	svc := NewService(repo)
	d, err := svc.Dashboard(context.Background(), "2026-05")
	if err != nil {
		t.Fatal(err)
	}
	if !d.Daily.Available || len(d.Daily.Days) != 31 {
		t.Fatalf("daily=%+v", d.Daily)
	}
	day := d.Daily.Days[1]
	if day.Revenue != 1000 || day.KnownCost != 600 || !day.CostUnavailable || day.Profit != nil || day.CostExpenses != nil {
		t.Fatalf("day=%+v", day)
	}
	ledger, err := svc.DailyLedger(context.Background(), DailyFilter{Month: "2026-05", Date: "2026-05-02", Unavailable: true, Page: 1, PageSize: 10})
	if err != nil || ledger.Total != 1 || ledger.Rows[0].PendingReason != "缺少成本" {
		t.Fatalf("ledger=%+v err=%v", ledger, err)
	}
}

type closedWithUnavailableSourcesRepo struct{ *closedDailySnapshotRepo }

func (r *closedWithUnavailableSourcesRepo) MonthlySourceTotals(context.Context, ReportFilter) (domain.MonthlySourceTotals, []Exception, error) {
	return domain.MonthlySourceTotals{}, nil, errors.New("source records are unavailable")
}

func TestClosedReportsReadFrozenSnapshotWhenSourcesUnavailable(t *testing.T) {
	base := newFakeRepo()
	base.reportStatus = domain.MonthStatusAdjusted
	base.adjustments = []AdjustmentRecord{{Type: domain.AdjustmentRevenue, Amount: 25}}
	snapshot := domain.BuildMonthlyReport(domain.DefaultSettings(), domain.MonthlySourceTotals{Month: "2026-05", RevenueTaxInclusive: 1030, MainBusinessCost: 600})
	snapshot.Status = domain.MonthStatusClosed
	repo := &closedWithUnavailableSourcesRepo{&closedDailySnapshotRepo{fakeRepo: base, snapshot: snapshot}}
	svc := NewService(repo)
	report, err := svc.DraftReport(context.Background(), ReportFilter{Month: "2026-05"})
	if err != nil || report.AdjustedGrossProfit != 425 {
		t.Fatalf("closed report = %+v, error = %v", report, err)
	}
	dashboard, err := svc.Dashboard(context.Background(), "2026-05")
	if err != nil || dashboard.Report.AdjustedGrossProfit != 425 {
		t.Fatalf("closed dashboard = %+v, error = %v", dashboard, err)
	}
	if len(dashboard.Exceptions) != 1 || dashboard.Exceptions[0].Code != "closed_source_check_unavailable" {
		t.Fatalf("failed source check must remain visible: %+v", dashboard.Exceptions)
	}
}

func TestClosedCustomerReportRejectsLiveDataWithoutCustomerSnapshot(t *testing.T) {
	repo := newFakeRepo()
	repo.reportStatus = domain.MonthStatusClosed
	repo.sourceDetails = []SourceDetail{{Section: "revenue", Date: "2026-05-01", Amount: 1030}}
	svc := NewService(repo)
	filter := ReportFilter{Month: "2026-05", CustomerID: 42}
	_, reportErr := svc.DraftReport(context.Background(), filter)
	_, detailErr := svc.ReportDrilldown(context.Background(), filter)
	_, dailyErr := svc.DailyLedger(context.Background(), DailyFilter{Month: "2026-05", CustomerID: 42})
	for _, err := range []error{reportErr, detailErr, dailyErr} {
		if err == nil || !strings.Contains(err.Error(), "客户已结账快照不可用") {
			t.Fatalf("closed customer history must not use live source data: %v", err)
		}
	}
}

func TestDailyLedgerRejectsInvalidFilter(t *testing.T) {
	svc := NewService(newFakeRepo())
	for _, f := range []DailyFilter{{Month: "2026-05", Date: "2026-06-01"}, {Month: "2026-05", Section: "tax"}, {Month: "2026-05", PageSize: 101}} {
		if _, err := svc.DailyLedger(context.Background(), f); err == nil {
			t.Fatalf("accepted %+v", f)
		}
	}
}

func TestDailyRevenueUsesGeneralTaxRateAndBalancesRounding(t *testing.T) {
	settings := domain.DefaultSettings()
	settings.TaxpayerType = domain.TaxpayerGeneral
	rows := []SourceDetail{
		{Section: "revenue", Date: "2026-05-01", Amount: 0.01},
		{Section: "revenue", Date: "2026-05-02", Amount: 113},
		{Section: "revenue", Date: "2026-05-03", Amount: -56.50},
	}
	snapshot, err := BuildDailySnapshot("2026-05", rows, settings)
	if err != nil {
		t.Fatal(err)
	}
	report := domain.BuildMonthlyReport(settings, domain.MonthlySourceTotals{Month: "2026-05", RevenueTaxInclusive: 56.51})
	var sum domain.Money
	for _, day := range snapshot.Summary.Days {
		sum += day.Revenue
	}
	if cents(sum) != cents(report.TaxExclusiveRevenue) || snapshot.Summary.Days[2].Revenue >= 0 {
		t.Fatalf("sum=%v report=%v days=%+v", sum, report.TaxExclusiveRevenue, snapshot.Summary.Days[:3])
	}
}

func TestDailyRevenueRoundsAfterSummingFractionalAmountsAndCredits(t *testing.T) {
	rows := []SourceDetail{
		{Section: "revenue", Date: "2026-05-01", Amount: 0.004},
		{Section: "revenue", Date: "2026-05-02", Amount: 0.004},
		{Section: "revenue", Date: "2026-05-03", Amount: 0.004},
		{Section: "revenue", Date: "2026-05-04", Amount: -0.009},
		{Section: "revenue", Date: "2026-05-05", Amount: 2.019},
	}
	for _, taxpayer := range []string{domain.TaxpayerSmallScale, domain.TaxpayerGeneral} {
		t.Run(taxpayer, func(t *testing.T) {
			settings := domain.DefaultSettings()
			settings.TaxpayerType = taxpayer
			snapshot, err := BuildDailySnapshot("2026-05", rows, settings)
			if err != nil {
				t.Fatal(err)
			}
			report := domain.BuildMonthlyReport(settings, domain.MonthlySourceTotals{Month: "2026-05", RevenueTaxInclusive: 2.022})
			var dailyCents, rowCents int64
			for _, day := range snapshot.Summary.Days {
				dailyCents += cents(day.Revenue)
			}
			for _, row := range snapshot.Rows {
				rowCents += cents(row.Amount)
			}
			if dailyCents != cents(report.TaxExclusiveRevenue) || rowCents != dailyCents {
				t.Fatalf("daily=%d, rows=%d, report=%d", dailyCents, rowCents, cents(report.TaxExclusiveRevenue))
			}
			if snapshot.Summary.Days[3].Revenue >= 0 {
				t.Fatalf("credit lost its negative sign: %+v", snapshot.Summary.Days[3])
			}
			if err := ValidateDailySnapshot(report, snapshot); err != nil {
				t.Fatalf("matching monthly report must be allowed to close: %v", err)
			}
		})
	}
}

func TestDailyCostsAndExpensesRoundAfterSummingFractionalAmountsAndCredits(t *testing.T) {
	settings := domain.DefaultSettings()
	rows := []SourceDetail{{Section: "revenue", Date: "2026-05-01", Amount: 103}}
	amounts := []domain.Money{0.004, 0.004, 0.004, -0.009, 2.019}
	for _, section := range []string{"main_cost", "period_expense"} {
		for i, amount := range amounts {
			rows = append(rows, SourceDetail{Section: section, Date: fmt.Sprintf("2026-05-%02d", i+1), Amount: amount})
		}
	}
	snapshot, err := BuildDailySnapshot("2026-05", rows, settings)
	if err != nil {
		t.Fatal(err)
	}
	report := domain.BuildMonthlyReport(settings, domain.MonthlySourceTotals{Month: "2026-05", RevenueTaxInclusive: 103, MainBusinessCost: 2.022, PeriodExpenses: 2.022})
	var cost, expense int64
	for _, day := range snapshot.Summary.Days {
		cost += cents(day.KnownCost)
		expense += cents(day.PeriodExpenses)
	}
	var rowCost, rowExpense int64
	for _, row := range snapshot.Rows {
		switch row.Section {
		case "main_cost":
			rowCost += cents(row.Amount)
		case "period_expense":
			rowExpense += cents(row.Amount)
		}
	}
	if cost != cents(report.MainBusinessCost) || expense != cents(report.PeriodExpenses) {
		t.Fatalf("daily cost=%d report cost=%d, daily expense=%d report expense=%d", cost, cents(report.MainBusinessCost), expense, cents(report.PeriodExpenses))
	}
	if rowCost != cost || rowExpense != expense {
		t.Fatalf("ledger cost=%d daily cost=%d, ledger expense=%d daily expense=%d", rowCost, cost, rowExpense, expense)
	}
	if snapshot.Summary.Days[3].KnownCost >= 0 || snapshot.Summary.Days[3].PeriodExpenses >= 0 {
		t.Fatalf("credits must retain their sign: %+v", snapshot.Summary.Days[3])
	}
	if err := ValidateDailySnapshot(report, snapshot); err != nil {
		t.Fatalf("matching monthly report must be allowed to close: %v", err)
	}
}

func TestClosingRejectsDailyTotalsThatDisagreeWithMonthlyReport(t *testing.T) {
	settings := domain.DefaultSettings()
	snapshot, err := BuildDailySnapshot("2026-05", []SourceDetail{
		{Section: "revenue", Date: "2026-05-02", Amount: 103},
		{Section: "main_cost", Date: "2026-05-02", Amount: 30},
		{Section: "period_expense", Date: "2026-05-02", Amount: 5},
	}, settings)
	if err != nil {
		t.Fatal(err)
	}
	report := domain.BuildMonthlyReport(settings, domain.MonthlySourceTotals{Month: "2026-05", RevenueTaxInclusive: 103, MainBusinessCost: 30, PeriodExpenses: 5})
	if err := ValidateDailySnapshot(report, snapshot); err != nil {
		t.Fatalf("matching report rejected: %v", err)
	}
	report.PeriodExpenses = 7
	if err := ValidateDailySnapshot(report, snapshot); err == nil {
		t.Fatal("closing must reject a report and daily snapshot read from different source states")
	}
}

type closedDailySnapshotRepo struct {
	*fakeRepo
	snapshot domain.MonthlyReport
}

type closedLegacySnapshotRepo struct{ *closedDailySnapshotRepo }

func (r *closedLegacySnapshotRepo) LoadDailySnapshot(context.Context, string) (DailySnapshot, bool, error) {
	return DailySnapshot{}, false, nil
}

func TestLegacyClosedAccountantHandoffDoesNotSilentlyOmitDetails(t *testing.T) {
	base := newFakeRepo()
	base.reportStatus = domain.MonthStatusClosed
	snapshot := domain.BuildMonthlyReport(domain.DefaultSettings(), domain.MonthlySourceTotals{Month: "2026-05", RevenueTaxInclusive: 1030})
	snapshot.Status = domain.MonthStatusClosed
	repo := &closedLegacySnapshotRepo{&closedDailySnapshotRepo{fakeRepo: base, snapshot: snapshot}}
	_, err := NewService(repo).AccountantHandoff(context.Background(), ReportFilter{Month: "2026-05"})
	if err == nil || !strings.Contains(err.Error(), "已结账历史来源明细不可用") {
		t.Fatalf("legacy handoff must report missing frozen details: %v", err)
	}
}

func (r *closedDailySnapshotRepo) LoadMonthlyReportSnapshot(context.Context, string) (domain.MonthlyReport, bool, error) {
	return r.snapshot, true, nil
}

func TestClosedMonthlyReportAppliesPostClosingAdjustmentsOnce(t *testing.T) {
	base := newFakeRepo()
	base.reportStatus = domain.MonthStatusAdjusted
	base.totals = domain.MonthlySourceTotals{Month: "2026-05", RevenueTaxInclusive: 1030, MainBusinessCost: 600}
	base.adjustments = []AdjustmentRecord{{Type: domain.AdjustmentRevenue, Amount: 25}, {Type: domain.AdjustmentExpense, Amount: 10}}
	snap := domain.BuildMonthlyReport(domain.DefaultSettings(), base.totals)
	snap.Status = domain.MonthStatusClosed
	repo := &closedDailySnapshotRepo{fakeRepo: base, snapshot: snap}
	report, err := NewService(repo).DraftReport(context.Background(), ReportFilter{Month: "2026-05"})
	if err != nil {
		t.Fatal(err)
	}
	if report.AdjustedTaxExclusiveRevenue != 1025 || report.AdjustedNetProfit != 395 {
		t.Fatalf("report=%+v", report)
	}
	repo.snapshot = domain.ApplyAdjustments(snap, []domain.Adjustment{{Type: domain.AdjustmentRevenue, Amount: 25}})
	report, err = NewService(repo).DraftReport(context.Background(), ReportFilter{Month: "2026-05"})
	if err != nil {
		t.Fatal(err)
	}
	if report.AdjustedTaxExclusiveRevenue != 1025 || report.AdjustedNetProfit != 395 {
		t.Fatalf("adjustment applied twice: %+v", report)
	}
}
