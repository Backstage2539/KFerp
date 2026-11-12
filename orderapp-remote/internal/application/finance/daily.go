package finance

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	domain "orderapp/internal/domain/finance"
)

type DailyPoint struct {
	Date            string        `json:"date"`
	Revenue         domain.Money  `json:"revenue"`
	KnownCost       domain.Money  `json:"known_cost"`
	PeriodExpenses  domain.Money  `json:"period_expenses"`
	CostExpenses    *domain.Money `json:"cost_expenses"`
	Profit          *domain.Money `json:"profit"`
	CostUnavailable bool          `json:"cost_unavailable"`
}
type DailySummary struct {
	Available             bool         `json:"available"`
	HasMonthlyAdjustments bool         `json:"has_monthly_adjustments"`
	Days                  []DailyPoint `json:"days"`
}
type DailySnapshot struct {
	Summary    DailySummary   `json:"summary"`
	Rows       []SourceDetail `json:"rows"`
	SourceRows []SourceDetail `json:"source_rows"`
}

// CustomerClosingSnapshot is keyed by customer ID in the same atomic monthly
// snapshot as the factory totals; never derive a historical customer report
// from the current state of orders or expenses.
type CustomerClosingSnapshot struct {
	Report domain.MonthlyReport `json:"report"`
	Daily  DailySnapshot        `json:"daily"`
}

type CustomerClosingSnapshots struct {
	Version   int                                `json:"version"`
	Customers map[string]CustomerClosingSnapshot `json:"customers"`
}

type customerSnapshotLoader interface {
	LoadCustomerSnapshot(context.Context, string, int64) (CustomerClosingSnapshot, bool, error)
}
type DailyFilter struct {
	Month          string
	CustomerID     int64
	Date           string
	Section        string
	Unavailable    bool
	Page, PageSize int
}
type DailyLedgerResult struct {
	Available bool           `json:"available"`
	Rows      []SourceDetail `json:"rows"`
	Total     int            `json:"total"`
	Page      int            `json:"page"`
	PageSize  int            `json:"page_size"`
}

func cents(v domain.Money) int64      { return int64(math.Round(float64(v) * 100)) }
func moneyCents(v int64) domain.Money { return domain.Money(float64(v) / 100) }

func allocateDailyAmount(cumulative *big.Rat, previous *int64, amount domain.Money, taxRate float64) (domain.Money, error) {
	value, ok := new(big.Rat).SetString(strconv.FormatFloat(float64(amount), 'f', -1, 64))
	if !ok {
		return 0, fmt.Errorf("无效的财务金额")
	}
	cumulative.Add(cumulative, value)
	total, _ := cumulative.Float64()
	if taxRate > 0 {
		total /= 1 + taxRate
	}
	next := int64(math.Round(total * 100))
	allocated := moneyCents(next - *previous)
	*previous = next
	return allocated, nil
}

// ValidateDailySnapshot prevents closing or rebuilding a month with daily and
// monthly amounts taken from different committed versions of the source data.
func ValidateDailySnapshot(report domain.MonthlyReport, snapshot DailySnapshot) error {
	var revenue, cost, expenses int64
	if !snapshot.Summary.Available {
		return fmt.Errorf("每日财务明细不可用，暂不能结账")
	}
	for _, day := range snapshot.Summary.Days {
		if day.CostUnavailable {
			return fmt.Errorf("销售成本待核算，暂不能结账")
		}
		revenue += cents(day.Revenue)
		cost += cents(day.KnownCost)
		expenses += cents(day.PeriodExpenses)
	}
	if revenue != cents(report.TaxExclusiveRevenue) || cost != cents(report.MainBusinessCost) || expenses != cents(report.PeriodExpenses) {
		return fmt.Errorf("财务来源数据在结账期间发生变化或每日与月度金额不一致，请刷新后重试")
	}
	return nil
}

// BuildDailySnapshot uses the same source rows as the monthly report. Revenue
// is allocated in cents by cumulative rounding, so the sum matches the report.
func BuildDailySnapshot(month string, rows []SourceDetail, settings domain.Settings) (DailySnapshot, error) {
	start, err := time.Parse("2006-01", month)
	if err != nil {
		return DailySnapshot{}, err
	}
	end := start.AddDate(0, 1, 0)
	copied := append([]SourceDetail(nil), rows...)
	sort.SliceStable(copied, func(i, j int) bool {
		if copied[i].Date != copied[j].Date {
			return copied[i].Date < copied[j].Date
		}
		if copied[i].SourceType != copied[j].SourceType {
			return copied[i].SourceType < copied[j].SourceType
		}
		return copied[i].SourceID < copied[j].SourceID
	})
	points := make([]DailyPoint, 0, 31)
	index := map[string]int{}
	for day := start; day.Before(end); day = day.AddDate(0, 0, 1) {
		date := day.Format("2006-01-02")
		index[date] = len(points)
		points = append(points, DailyPoint{Date: date})
	}
	settings = domain.NormalizeSettings(settings)
	rate := settings.SmallScaleVATRate
	if settings.TaxpayerType == domain.TaxpayerGeneral {
		rate = settings.GeneralOutputVATRate
	}
	// Monthly revenue sums its NUMERIC source amounts before rounding. Keep
	// every fractional source value until the cumulative tax-exclusive total
	// is rounded; rounding each row first can shift the month by a cent.
	gross, cost, expense := new(big.Rat), new(big.Rat), new(big.Rat)
	var netCents, costCents, expenseCents int64
	kept := make([]SourceDetail, 0, len(copied))
	for _, row := range copied {
		idx, ok := index[row.Date]
		if !ok || (row.Section != "revenue" && row.Section != "main_cost" && row.Section != "period_expense") {
			continue
		}
		point := &points[idx]
		switch row.Section {
		case "revenue":
			row.Amount, err = allocateDailyAmount(gross, &netCents, row.Amount, rate)
			if err != nil {
				return DailySnapshot{}, err
			}
			point.Revenue = moneyCents(cents(point.Revenue) + cents(row.Amount))
		case "main_cost":
			if row.CostUnavailable {
				point.CostUnavailable = true
			} else {
				row.Amount, err = allocateDailyAmount(cost, &costCents, row.Amount, 0)
				if err != nil {
					return DailySnapshot{}, err
				}
				point.KnownCost = moneyCents(cents(point.KnownCost) + cents(row.Amount))
			}
		case "period_expense":
			row.Amount, err = allocateDailyAmount(expense, &expenseCents, row.Amount, 0)
			if err != nil {
				return DailySnapshot{}, err
			}
			point.PeriodExpenses = moneyCents(cents(point.PeriodExpenses) + cents(row.Amount))
		}
		kept = append(kept, row)
	}
	for i := range points {
		p := &points[i]
		if !p.CostUnavailable {
			cost := moneyCents(cents(p.KnownCost) + cents(p.PeriodExpenses))
			profit := moneyCents(cents(p.Revenue) - cents(cost))
			p.CostExpenses = &cost
			p.Profit = &profit
		}
	}
	return DailySnapshot{Summary: DailySummary{Available: true, Days: points}, Rows: kept, SourceRows: append([]SourceDetail{}, rows...)}, nil
}

func (s *Service) DailySnapshotDraft(ctx context.Context, filter ReportFilter) (DailySnapshot, error) {
	if s.repo == nil {
		return DailySnapshot{Summary: DailySummary{Days: []DailyPoint{}}, Rows: []SourceDetail{}}, nil
	}
	settings, err := s.loadSettings(ctx)
	if err != nil {
		return DailySnapshot{}, err
	}
	rows, err := s.repo.FinanceSourceDetails(ctx, filter)
	if err != nil {
		return DailySnapshot{}, err
	}
	return BuildDailySnapshot(filter.Month, rows, settings.Settings)
}

func (s *Service) dailyData(ctx context.Context, filter ReportFilter, report domain.MonthlyReport) (DailySnapshot, error) {
	if report.Status == domain.MonthStatusClosed || report.Status == domain.MonthStatusAdjusted {
		if filter.CustomerID > 0 {
			if loader, ok := s.repo.(customerSnapshotLoader); ok {
				data, found, err := loader.LoadCustomerSnapshot(ctx, filter.Month, filter.CustomerID)
				if err != nil {
					return DailySnapshot{}, err
				}
				if found {
					return data.Daily, nil
				}
			}
			return DailySnapshot{Summary: DailySummary{Days: []DailyPoint{}}, Rows: []SourceDetail{}}, nil
		}
		if filter.CustomerID == 0 {
			if loader, ok := s.repo.(interface {
				LoadDailySnapshot(context.Context, string) (DailySnapshot, bool, error)
			}); ok {
				data, found, err := loader.LoadDailySnapshot(ctx, filter.Month)
				if err != nil {
					return DailySnapshot{}, err
				}
				if found {
					if report.Status == domain.MonthStatusAdjusted {
						adjustments, err := s.repo.ListAdjustments(ctx, filter.Month)
						if err != nil {
							return DailySnapshot{}, err
						}
						data.Summary.HasMonthlyAdjustments = len(adjustments) > 0
					}
					return data, nil
				}
			}
		}
		return DailySnapshot{Summary: DailySummary{Days: []DailyPoint{}}, Rows: []SourceDetail{}}, nil
	}
	return s.DailySnapshotDraft(ctx, filter)
}

func (s *Service) DailyLedger(ctx context.Context, f DailyFilter) (DailyLedgerResult, error) {
	f.Month = strings.TrimSpace(f.Month)
	f.Date = strings.TrimSpace(f.Date)
	f.Section = strings.TrimSpace(f.Section)
	if err := validateMonth(f.Month); err != nil {
		return DailyLedgerResult{}, err
	}
	if f.Date != "" {
		d, err := time.Parse("2006-01-02", f.Date)
		if err != nil || d.Format("2006-01") != f.Month {
			return DailyLedgerResult{}, fmt.Errorf("invalid date")
		}
	}
	if f.Section != "" && f.Section != "revenue" && f.Section != "main_cost" && f.Section != "period_expense" {
		return DailyLedgerResult{}, fmt.Errorf("invalid section")
	}
	if f.Page < 0 || f.PageSize < 0 || f.PageSize > 100 || f.CustomerID < 0 {
		return DailyLedgerResult{}, fmt.Errorf("invalid pagination or customer")
	}
	if f.Page == 0 {
		f.Page = 1
	}
	if f.PageSize == 0 {
		f.PageSize = 20
	}
	report, err := s.DraftReport(ctx, ReportFilter{Month: f.Month, CustomerID: f.CustomerID})
	if err != nil {
		return DailyLedgerResult{}, err
	}
	data, err := s.dailyData(ctx, ReportFilter{Month: f.Month, CustomerID: f.CustomerID}, report)
	if err != nil {
		return DailyLedgerResult{}, err
	}
	filtered := make([]SourceDetail, 0)
	for _, row := range data.Rows {
		if f.Date != "" && row.Date != f.Date || f.Section != "" && row.Section != f.Section || f.Unavailable && !row.CostUnavailable {
			continue
		}
		filtered = append(filtered, row)
	}
	total := len(filtered)
	start := total
	if f.Page-1 <= total/f.PageSize {
		start = (f.Page - 1) * f.PageSize
		if start > total {
			start = total
		}
	}
	end := start + f.PageSize
	if end > total {
		end = total
	}
	return DailyLedgerResult{Available: data.Summary.Available, Rows: filtered[start:end], Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}
