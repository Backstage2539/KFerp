package finance

import (
	"context"
	"strings"
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
	repo.expenseEmployees = []ExpenseEmployee{{ID: 7, Name: "小王", Active: true}}
	svc := NewService(repo)

	expense, err := svc.CreateExpense(context.Background(), CreateExpenseCommand{
		Date:       "2026-05-02",
		Category:   "房租",
		Amount:     3800,
		Allocation: AllocationPeriodExpense,
		EmployeeID: 7,
		Actor:      "Van",
	})
	if err != nil {
		t.Fatal(err)
	}
	if expense.Month != "2026-05" || expense.Allocation != AllocationPeriodExpense {
		t.Fatalf("unexpected expense: %#v", expense)
	}
	if expense.EmployeeID != 7 {
		t.Fatalf("employee_id = %d, want 7", expense.EmployeeID)
	}
	if _, err := svc.CreateExpense(context.Background(), CreateExpenseCommand{Date: "2026-05-02", Category: "房租", Amount: 1, OrderID: -1}); err == nil {
		t.Fatal("negative order_id should fail")
	}
	if _, err := svc.CreateExpense(context.Background(), CreateExpenseCommand{Date: "bad", Category: "房租", Amount: 1}); err == nil {
		t.Fatal("invalid date should fail")
	}
	if _, err := svc.CreateExpense(context.Background(), CreateExpenseCommand{Date: "2026-05-02", Category: "房租", Amount: -1}); err == nil {
		t.Fatal("negative amount should fail")
	}
	if _, err := svc.CreateExpense(context.Background(), CreateExpenseCommand{Date: "2026-05-02", Category: "房租", Amount: 1, EmployeeID: -1}); err == nil {
		t.Fatal("negative employee_id should fail")
	}
}

func TestCreateExpenseRejectsInactiveEmployee(t *testing.T) {
	repo := newFakeRepo()
	repo.reportStatus = domain.MonthStatusDraft
	repo.expenseEmployees = []ExpenseEmployee{
		{ID: 7, Name: "离职员工", Active: false},
		{ID: 8, Name: "在职员工", Active: true},
	}
	svc := NewService(repo)

	_, err := svc.CreateExpense(context.Background(), CreateExpenseCommand{
		Date:       "2026-05-02",
		Category:   "样品费",
		Amount:     120,
		Allocation: AllocationPeriodExpense,
		EmployeeID: 7,
		Actor:      "Van",
	})
	if err == nil || !strings.Contains(err.Error(), "employee inactive") {
		t.Fatalf("inactive employee err=%v, want employee inactive", err)
	}
	if len(repo.expenses) != 0 {
		t.Fatalf("inactive employee wrote expenses: %#v", repo.expenses)
	}

	_, err = svc.CreateExpense(context.Background(), CreateExpenseCommand{
		Date:       "2026-05-02",
		Category:   "样品费",
		Amount:     120,
		Allocation: AllocationPeriodExpense,
		EmployeeID: 8,
		Actor:      "Van",
	})
	if err != nil {
		t.Fatalf("active employee should be accepted: %v", err)
	}
}

func TestCreateExpenseCapturesFinanceDimensions(t *testing.T) {
	repo := newFakeRepo()
	repo.reportStatus = domain.MonthStatusDraft
	svc := NewService(repo)

	expense, err := svc.CreateExpense(context.Background(), CreateExpenseCommand{
		Date:          "2026-05-02",
		Category:      "样品费",
		Amount:        120,
		Allocation:    AllocationPeriodExpense,
		OrderID:       256,
		CustomerID:    18,
		ProductID:     9,
		BatchNo:       "BATCH-0503",
		DimensionNote: "客户杯测样品",
	})
	if err != nil {
		t.Fatal(err)
	}
	if expense.OrderID != 256 || expense.CustomerID != 18 || expense.ProductID != 9 || expense.BatchNo != "BATCH-0503" || expense.DimensionNote != "客户杯测样品" {
		t.Fatalf("finance dimensions not preserved: %#v", expense)
	}
}

func TestFinanceClosingReviewDrilldownTaxLedgerAndHandoff(t *testing.T) {
	repo := newFakeRepo()
	repo.reportStatus = domain.MonthStatusDraft
	repo.totals = domain.MonthlySourceTotals{Month: "2026-05", RevenueTaxInclusive: 1030, MainBusinessCost: 300, PeriodExpenses: 120}
	repo.sourceDetails = []SourceDetail{
		{Section: "revenue", SourceType: "order", SourceID: 256, Date: "2026-05-02", Name: "SO-20260502-0001", Amount: 1030},
		{Section: "main_cost", SourceType: "production_cost", SourceID: 9, Date: "2026-05-03", Name: "烘焙批次成本", Amount: 300},
		{Section: "period_expense", SourceType: "expense", SourceID: 1, Date: "2026-05-03", Name: "房租", Amount: 120},
	}
	repo.taxLedger = []TaxLedgerEntry{
		{ID: 1, Month: "2026-05", Kind: "sales_invoice", InvoiceNo: "INV-001", Counterparty: "咖啡客户A", TotalAmount: 1030, TaxAmount: 30, Status: "confirmed"},
	}
	svc := NewService(repo)

	review, err := svc.ClosingReview(context.Background(), "2026-05")
	if err != nil {
		t.Fatal(err)
	}
	if len(review.Items) < 4 || !review.HasCode("source_exceptions") || !review.HasCode("tax_ledger") || !review.HasCode("accountant_handoff") {
		t.Fatalf("closing review missing expected checks: %#v", review.Items)
	}

	drilldown, err := svc.ReportDrilldown(context.Background(), "2026-05")
	if err != nil {
		t.Fatal(err)
	}
	if drilldown.SectionTotal("revenue") != 1030 || drilldown.SectionTotal("main_cost") != 300 || drilldown.SectionTotal("period_expense") != 120 {
		t.Fatalf("unexpected drilldown totals: %#v", drilldown.Sections)
	}

	ledgerRow, err := svc.CreateTaxLedgerEntry(context.Background(), CreateTaxLedgerCommand{
		Month:        "2026-05",
		Kind:         "purchase_invoice",
		InvoiceNo:    "PINV-001",
		Counterparty: "生豆供应商",
		TotalAmount:  500,
		TaxAmount:    15,
		Status:       "pending",
		Actor:        "Van",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ledgerRow.Kind != "purchase_invoice" || ledgerRow.InvoiceNo != "PINV-001" {
		t.Fatalf("unexpected ledger row: %#v", ledgerRow)
	}
	ledger, err := svc.ListTaxLedger(context.Background(), "2026-05")
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger) != 2 {
		t.Fatalf("ledger rows = %d, want 2", len(ledger))
	}

	handoff, err := svc.AccountantHandoff(context.Background(), "2026-05")
	if err != nil {
		t.Fatal(err)
	}
	if handoff.Month != "2026-05" || len(handoff.VoucherDrafts) < 4 || len(handoff.TaxLedger) != 2 {
		t.Fatalf("unexpected handoff: %#v", handoff)
	}
}

func TestListExpensesFiltersByEmployee(t *testing.T) {
	repo := newFakeRepo()
	repo.expenses = []Expense{
		{ID: 1, Date: "2026-05-02", Month: "2026-05", Category: "工资", EmployeeID: 7, EmployeeName: "小王"},
	}
	svc := NewService(repo)

	rows, err := svc.ListExpenses(context.Background(), ExpenseFilter{Month: "2026-05", EmployeeID: 7})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].EmployeeID != 7 || rows[0].EmployeeName != "小王" {
		t.Fatalf("unexpected rows: %#v", rows)
	}
	if repo.listExpenseFilter.EmployeeID != 7 {
		t.Fatalf("employee filter = %d, want 7", repo.listExpenseFilter.EmployeeID)
	}
	if _, err := svc.ListExpenses(context.Background(), ExpenseFilter{Month: "2026-05", EmployeeID: -1}); err == nil {
		t.Fatal("negative employee filter should fail")
	}
}

func TestListExpenseEmployeesReturnsRepositoryRows(t *testing.T) {
	repo := newFakeRepo()
	repo.expenseEmployees = []ExpenseEmployee{{ID: 7, Name: "小王", Active: true}}
	svc := NewService(repo)

	rows, err := svc.ListExpenseEmployees(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != 7 || rows[0].Name != "小王" || !rows[0].Active {
		t.Fatalf("unexpected employees: %#v", rows)
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

func TestCloseMonthPreservesAdjustedStatus(t *testing.T) {
	repo := newFakeRepo()
	repo.settings = domain.DefaultSettings()
	repo.reportStatus = domain.MonthStatusAdjusted
	repo.totals = domain.MonthlySourceTotals{Month: "2026-05", RevenueTaxInclusive: 103000}
	repo.adjustments = []AdjustmentRecord{{Type: domain.AdjustmentExpense, Amount: 100, Reason: "补记费用"}}
	svc := NewService(repo)

	report, err := svc.CloseMonth(context.Background(), CloseMonthCommand{Month: "2026-05", Actor: "Van"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != domain.MonthStatusAdjusted {
		t.Fatalf("status = %q, want adjusted so repeat close does not downgrade post-close adjustments", report.Status)
	}
	if repo.reportStatus != domain.MonthStatusAdjusted {
		t.Fatalf("saved status = %q, want adjusted", repo.reportStatus)
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

func TestCreateTaxLedgerRespectsStrongLockForClosedMonth(t *testing.T) {
	repo := newFakeRepo()
	repo.settings = domain.DefaultSettings()
	repo.reportStatus = domain.MonthStatusClosed
	svc := NewService(repo)

	_, err := svc.CreateTaxLedgerEntry(context.Background(), CreateTaxLedgerCommand{
		Month:        "2026-05",
		Kind:         "sales_invoice",
		InvoiceNo:    "INV-CLOSED",
		Counterparty: "客户A",
		TotalAmount:  1000,
		TaxAmount:    30,
		Status:       "confirmed",
		Actor:        "Van",
	})
	if err == nil {
		t.Fatal("closed strong-lock month should reject new tax ledger source document")
	}

	repo.settings.ClosingMode = domain.ClosingModeLightConfirmation
	row, err := svc.CreateTaxLedgerEntry(context.Background(), CreateTaxLedgerCommand{
		Month:        "2026-05",
		Kind:         "sales_invoice",
		InvoiceNo:    "INV-LIGHT",
		Counterparty: "客户A",
		TotalAmount:  1000,
		TaxAmount:    30,
		Status:       "confirmed",
		Actor:        "Van",
	})
	if err != nil {
		t.Fatalf("light confirmation should allow tax ledger entry with warning path: %v", err)
	}
	if row.InvoiceNo != "INV-LIGHT" {
		t.Fatalf("unexpected tax ledger row: %#v", row)
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
	listExpenseFilter   ExpenseFilter
	expenseEmployees    []ExpenseEmployee
	sourceDetails       []SourceDetail
	taxLedger           []TaxLedgerEntry
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
	row := Expense{ID: int64(len(r.expenses) + 1), Date: cmd.Date, Month: monthFromDate(cmd.Date), Category: cmd.Category, Amount: cmd.Amount, Allocation: cmd.Allocation, EmployeeID: cmd.EmployeeID, OrderID: cmd.OrderID, CustomerID: cmd.CustomerID, ProductID: cmd.ProductID, BatchNo: cmd.BatchNo, DimensionNote: cmd.DimensionNote, Actor: cmd.Actor}
	r.expenses = append(r.expenses, row)
	return row, nil
}

func (r *fakeRepo) ListExpenses(_ context.Context, filter ExpenseFilter) ([]Expense, error) {
	r.listExpenseFilter = filter
	return r.expenses, nil
}

func (r *fakeRepo) ListExpenseEmployees(context.Context) ([]ExpenseEmployee, error) {
	return r.expenseEmployees, nil
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

func (r *fakeRepo) FinanceSourceDetails(context.Context, string) ([]SourceDetail, error) {
	return r.sourceDetails, nil
}

func (r *fakeRepo) ListTaxLedger(context.Context, string) ([]TaxLedgerEntry, error) {
	return r.taxLedger, nil
}

func (r *fakeRepo) CreateTaxLedgerEntry(_ context.Context, cmd CreateTaxLedgerCommand) (TaxLedgerEntry, error) {
	row := TaxLedgerEntry{ID: int64(len(r.taxLedger) + 1), Month: cmd.Month, Kind: cmd.Kind, InvoiceNo: cmd.InvoiceNo, Counterparty: cmd.Counterparty, TotalAmount: cmd.TotalAmount, TaxAmount: cmd.TaxAmount, Status: cmd.Status, Note: cmd.Note, Actor: cmd.Actor}
	r.taxLedger = append(r.taxLedger, row)
	return row, nil
}

func assertMoney(t *testing.T, label string, got, want domain.Money) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %.2f, want %.2f", label, got, want)
	}
}
