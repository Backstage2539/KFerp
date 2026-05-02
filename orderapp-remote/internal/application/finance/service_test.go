package finance

import (
	"context"
	"testing"

	domain "orderapp/internal/domain/finance"
)

func TestDashboardBuildsMonthlyReportFromSettingsSourcesAndAdjustments(t *testing.T) {
	repo := newFakeRepo()
	repo.settings = domain.DefaultSettings()
	repo.totals = domain.MonthlySourceTotals{
		Month:               "2026-05",
		RevenueTaxInclusive: 103000,
		MainBusinessCost:    45000,
		PeriodExpenses:      12000,
	}
	repo.adjustments = []AdjustmentRecord{
		{Type: domain.AdjustmentRevenue, Amount: 1000, Reason: "补记收入"},
	}
	svc := NewService(repo)

	dashboard, err := svc.Dashboard(context.Background(), "2026-05")
	if err != nil {
		t.Fatal(err)
	}

	assertMoney(t, "revenue", dashboard.Report.TaxExclusiveRevenue, 100000)
	assertMoney(t, "adjusted revenue", dashboard.Report.AdjustedTaxExclusiveRevenue, 101000)
	if len(dashboard.Exceptions) != 1 || dashboard.Exceptions[0].Code != "uncategorized_expense" {
		t.Fatalf("unexpected exceptions: %#v", dashboard.Exceptions)
	}
}

func TestCreateExpenseValidatesAndNormalizesPeriodExpense(t *testing.T) {
	repo := newFakeRepo()
	repo.reportStatus = domain.MonthStatusDraft
	svc := NewService(repo)

	expense, err := svc.CreateExpense(context.Background(), CreateExpenseCommand{
		Date:       "2026-05-02",
		Category:   "房租",
		Amount:     3800,
		Allocation: AllocationPeriodExpense,
		Actor:      "Van",
	})
	if err != nil {
		t.Fatal(err)
	}
	if expense.Month != "2026-05" || expense.Allocation != AllocationPeriodExpense {
		t.Fatalf("unexpected expense: %#v", expense)
	}
	if _, err := svc.CreateExpense(context.Background(), CreateExpenseCommand{Date: "bad", Category: "房租", Amount: 1}); err == nil {
		t.Fatal("invalid date should fail")
	}
	if _, err := svc.CreateExpense(context.Background(), CreateExpenseCommand{Date: "2026-05-02", Category: "房租", Amount: -1}); err == nil {
		t.Fatal("negative amount should fail")
	}
}

func TestCloseMonthUsesStrongLockSnapshot(t *testing.T) {
	repo := newFakeRepo()
	repo.settings = domain.DefaultSettings()
	repo.totals = domain.MonthlySourceTotals{Month: "2026-05", RevenueTaxInclusive: 103000}
	svc := NewService(repo)

	report, err := svc.CloseMonth(context.Background(), CloseMonthCommand{Month: "2026-05", Actor: "Van"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != domain.MonthStatusClosed {
		t.Fatalf("status = %q, want closed", report.Status)
	}
	if repo.closedBy != "Van" {
		t.Fatalf("closed actor = %q", repo.closedBy)
	}
}

func TestDraftReportKeepsSavedClosedOrAdjustedStatus(t *testing.T) {
	repo := newFakeRepo()
	repo.reportStatus = domain.MonthStatusAdjusted
	repo.totals = domain.MonthlySourceTotals{Month: "2026-05", RevenueTaxInclusive: 103000}
	repo.adjustments = []AdjustmentRecord{{Type: domain.AdjustmentExpense, Amount: 100, Reason: "补记费用"}}
	svc := NewService(repo)

	report, err := svc.DraftReport(context.Background(), "2026-05")
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != domain.MonthStatusAdjusted {
		t.Fatalf("status = %q, want adjusted", report.Status)
	}
	assertMoney(t, "adjusted expenses", report.AppliedAdjustments[domain.AdjustmentExpense], 100)
}

func TestCreateExpenseRespectsStrongLockForClosedMonth(t *testing.T) {
	repo := newFakeRepo()
	repo.settings = domain.DefaultSettings()
	repo.reportStatus = domain.MonthStatusClosed
	svc := NewService(repo)

	_, err := svc.CreateExpense(context.Background(), CreateExpenseCommand{
		Date:       "2026-05-02",
		Category:   "房租",
		Amount:     100,
		Allocation: AllocationPeriodExpense,
		Actor:      "Van",
	})
	if err == nil {
		t.Fatal("closed strong-lock month should reject new source expense")
	}

	repo.settings.ClosingMode = domain.ClosingModeLightConfirmation
	_, err = svc.CreateExpense(context.Background(), CreateExpenseCommand{
		Date:       "2026-05-02",
		Category:   "房租",
		Amount:     100,
		Allocation: AllocationPeriodExpense,
		Actor:      "Van",
	})
	if err != nil {
		t.Fatalf("light confirmation should allow expense with warning path: %v", err)
	}
}

func TestSwitchClosingModeRequiresWhitelist(t *testing.T) {
	repo := newFakeRepo()
	repo.settings = domain.DefaultSettings()
	repo.closeModeAdminUsers = []string{"Van"}
	svc := NewService(repo)

	if _, err := svc.SwitchClosingMode(context.Background(), SwitchClosingModeCommand{Mode: domain.ClosingModeLightConfirmation, Actor: "Other"}); err == nil {
		t.Fatal("non-whitelisted actor should not switch closing mode")
	}
	settings, err := svc.SwitchClosingMode(context.Background(), SwitchClosingModeCommand{Mode: domain.ClosingModeLightConfirmation, Actor: "Van"})
	if err != nil {
		t.Fatal(err)
	}
	if settings.ClosingMode != domain.ClosingModeLightConfirmation {
		t.Fatalf("mode = %q", settings.ClosingMode)
	}
}

func TestSwitchClosingModeAllowsEnvironmentWhitelist(t *testing.T) {
	t.Setenv("FINANCE_CLOSE_MODE_ADMIN_USERS", "Van,财务主管")
	repo := newFakeRepo()
	repo.settings = domain.DefaultSettings()
	svc := NewService(repo)

	settings, err := svc.Settings(context.Background(), "财务主管")
	if err != nil {
		t.Fatal(err)
	}
	if !settings.CanManageCloseMode {
		t.Fatal("environment whitelist user should see close mode management")
	}
	changed, err := svc.SwitchClosingMode(context.Background(), SwitchClosingModeCommand{Mode: domain.ClosingModeLightConfirmation, Actor: "财务主管"})
	if err != nil {
		t.Fatal(err)
	}
	if changed.ClosingMode != domain.ClosingModeLightConfirmation {
		t.Fatalf("mode = %q", changed.ClosingMode)
	}
}

func TestSaveSettingsDoesNotLetFinanceWriterGrantCloseModeWhitelist(t *testing.T) {
	repo := newFakeRepo()
	repo.settings = domain.DefaultSettings()
	svc := NewService(repo)

	_, err := svc.SaveSettings(context.Background(), SettingsSnapshot{
		Settings:            domain.DefaultSettings(),
		CloseModeAdminUsers: []string{"Other"},
	}, "Other")
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.closeModeAdminUsers) != 0 {
		t.Fatalf("non-whitelisted writer changed close mode admins: %#v", repo.closeModeAdminUsers)
	}
}

func TestCreateAdjustmentRequiresClosedMonth(t *testing.T) {
	repo := newFakeRepo()
	repo.reportStatus = domain.MonthStatusDraft
	svc := NewService(repo)

	if _, err := svc.CreateAdjustment(context.Background(), CreateAdjustmentCommand{
		Month:  "2026-05",
		Type:   domain.AdjustmentExpense,
		Amount: 100,
		Reason: "补记费用",
		Actor:  "Van",
	}); err == nil {
		t.Fatal("draft month adjustment should fail")
	}
	repo.reportStatus = domain.MonthStatusClosed
	row, err := svc.CreateAdjustment(context.Background(), CreateAdjustmentCommand{
		Month:  "2026-05",
		Type:   domain.AdjustmentExpense,
		Amount: 100,
		Reason: "补记费用",
		Actor:  "Van",
	})
	if err != nil {
		t.Fatal(err)
	}
	if row.Type != domain.AdjustmentExpense || row.Month != "2026-05" {
		t.Fatalf("unexpected adjustment: %#v", row)
	}
}

type fakeRepo struct {
	settings            domain.Settings
	closeModeAdminUsers []string
	totals              domain.MonthlySourceTotals
	adjustments         []AdjustmentRecord
	reportStatus        string
	closedBy            string
	expenses            []Expense
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{settings: domain.DefaultSettings(), reportStatus: domain.MonthStatusClosed}
}

func (r *fakeRepo) LoadSettings(context.Context) (SettingsSnapshot, error) {
	return SettingsSnapshot{Settings: r.settings, CloseModeAdminUsers: r.closeModeAdminUsers}, nil
}

func (r *fakeRepo) SaveSettings(_ context.Context, snapshot SettingsSnapshot, _ string) (SettingsSnapshot, error) {
	r.settings = snapshot.Settings
	r.closeModeAdminUsers = snapshot.CloseModeAdminUsers
	return snapshot, nil
}

func (r *fakeRepo) MonthlySourceTotals(context.Context, string) (domain.MonthlySourceTotals, []Exception, error) {
	return r.totals, []Exception{{Code: "uncategorized_expense", Message: "有未分类费用"}}, nil
}

func (r *fakeRepo) ListAdjustments(context.Context, string) ([]AdjustmentRecord, error) {
	return r.adjustments, nil
}

func (r *fakeRepo) CreateExpense(_ context.Context, cmd CreateExpenseCommand) (Expense, error) {
	row := Expense{ID: int64(len(r.expenses) + 1), Date: cmd.Date, Month: monthFromDate(cmd.Date), Category: cmd.Category, Amount: cmd.Amount, Allocation: cmd.Allocation, Actor: cmd.Actor}
	r.expenses = append(r.expenses, row)
	return row, nil
}

func (r *fakeRepo) ListExpenses(context.Context, string) ([]Expense, error) {
	return r.expenses, nil
}

func (r *fakeRepo) SaveMonthlyReport(_ context.Context, report domain.MonthlyReport, actor string) (domain.MonthlyReport, error) {
	r.reportStatus = report.Status
	r.closedBy = actor
	return report, nil
}

func (r *fakeRepo) MonthlyReportStatus(context.Context, string) (string, error) {
	return r.reportStatus, nil
}

func (r *fakeRepo) CreateAdjustment(_ context.Context, cmd CreateAdjustmentCommand) (AdjustmentRecord, error) {
	row := AdjustmentRecord{ID: int64(len(r.adjustments) + 1), Month: cmd.Month, Type: cmd.Type, Amount: cmd.Amount, Reason: cmd.Reason, Actor: cmd.Actor}
	r.adjustments = append(r.adjustments, row)
	return row, nil
}

func assertMoney(t *testing.T, label string, got, want domain.Money) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %.2f, want %.2f", label, got, want)
	}
}
