package finance

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	domain "orderapp/internal/domain/finance"
)

const (
	AllocationPeriodExpense = "period_expense"
	AllocationMainCost      = "main_cost"
)

type SettingsSnapshot struct {
	domain.Settings
	CloseModeAdminUsers []string `json:"close_mode_admin_users,omitempty"`
	CanManageCloseMode  bool     `json:"can_manage_close_mode"`
}

type Expense struct {
	ID           int64        `json:"id"`
	Date         string       `json:"date"`
	Month        string       `json:"month"`
	Category     string       `json:"category"`
	Amount       domain.Money `json:"amount"`
	Allocation   string       `json:"allocation"`
	EmployeeID   int64        `json:"employee_id,omitempty"`
	EmployeeName string       `json:"employee_name,omitempty"`
	Payment      string       `json:"payment,omitempty"`
	Note         string       `json:"note,omitempty"`
	Actor        string       `json:"actor,omitempty"`
	CreatedAt    string       `json:"created_at,omitempty"`
}

type CreateExpenseCommand struct {
	Date       string       `json:"date"`
	Month      string       `json:"month,omitempty"`
	Category   string       `json:"category"`
	Amount     domain.Money `json:"amount"`
	Allocation string       `json:"allocation"`
	EmployeeID int64        `json:"employee_id,omitempty"`
	Payment    string       `json:"payment,omitempty"`
	Note       string       `json:"note,omitempty"`
	Actor      string       `json:"-"`
}

type ExpenseFilter struct {
	Month      string `json:"month"`
	EmployeeID int64  `json:"employee_id,omitempty"`
}

type ExpenseEmployee struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type Exception struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Count   int    `json:"count,omitempty"`
}

type Dashboard struct {
	Settings   SettingsSnapshot     `json:"settings"`
	Report     domain.MonthlyReport `json:"report"`
	Exceptions []Exception          `json:"exceptions"`
}

type CloseMonthCommand struct {
	Month string `json:"month"`
	Actor string `json:"-"`
}

type SwitchClosingModeCommand struct {
	Mode  string `json:"mode"`
	Actor string `json:"-"`
}

type CreateAdjustmentCommand struct {
	Month  string       `json:"month"`
	Type   string       `json:"type"`
	Amount domain.Money `json:"amount"`
	Reason string       `json:"reason"`
	Note   string       `json:"note,omitempty"`
	Actor  string       `json:"-"`
}

type AdjustmentRecord struct {
	ID        int64        `json:"id"`
	Month     string       `json:"month"`
	Type      string       `json:"type"`
	Amount    domain.Money `json:"amount"`
	Reason    string       `json:"reason"`
	Note      string       `json:"note,omitempty"`
	Actor     string       `json:"actor,omitempty"`
	CreatedAt string       `json:"created_at,omitempty"`
}

type Repository interface {
	LoadSettings(ctx context.Context) (SettingsSnapshot, error)
	SaveSettings(ctx context.Context, snapshot SettingsSnapshot, actor string) (SettingsSnapshot, error)
	MonthlySourceTotals(ctx context.Context, month string) (domain.MonthlySourceTotals, []Exception, error)
	ListAdjustments(ctx context.Context, month string) ([]AdjustmentRecord, error)
	CreateExpense(ctx context.Context, cmd CreateExpenseCommand) (Expense, error)
	ListExpenses(ctx context.Context, filter ExpenseFilter) ([]Expense, error)
	ListExpenseEmployees(ctx context.Context) ([]ExpenseEmployee, error)
	SaveMonthlyReport(ctx context.Context, report domain.MonthlyReport, actor string) (domain.MonthlyReport, error)
	MonthlyReportStatus(ctx context.Context, month string) (string, error)
	CreateAdjustment(ctx context.Context, cmd CreateAdjustmentCommand) (AdjustmentRecord, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Settings(ctx context.Context, actor string) (SettingsSnapshot, error) {
	snapshot, err := s.loadSettings(ctx)
	if err != nil {
		return SettingsSnapshot{}, err
	}
	snapshot.CanManageCloseMode = canManageCloseMode(snapshot.CloseModeAdminUsers, actor)
	return snapshot, nil
}

func (s *Service) SaveSettings(ctx context.Context, snapshot SettingsSnapshot, actor string) (SettingsSnapshot, error) {
	if s.repo == nil {
		return SettingsSnapshot{}, fmt.Errorf("repository required")
	}
	current, err := s.loadSettings(ctx)
	if err != nil {
		return SettingsSnapshot{}, err
	}
	snapshot.Settings = domain.NormalizeSettings(snapshot.Settings)
	canManageMode := canManageCloseMode(current.CloseModeAdminUsers, actor)
	if snapshot.ClosingMode != current.ClosingMode && !canManageMode {
		return SettingsSnapshot{}, fmt.Errorf("closing mode switch requires whitelist")
	}
	if !canManageMode || len(snapshot.CloseModeAdminUsers) == 0 {
		snapshot.CloseModeAdminUsers = current.CloseModeAdminUsers
	}
	saved, err := s.repo.SaveSettings(ctx, snapshot, actor)
	if err != nil {
		return SettingsSnapshot{}, err
	}
	saved.CanManageCloseMode = canManageCloseMode(saved.CloseModeAdminUsers, actor)
	return saved, nil
}

func (s *Service) SwitchClosingMode(ctx context.Context, cmd SwitchClosingModeCommand) (SettingsSnapshot, error) {
	if cmd.Mode != domain.ClosingModeStrongLock && cmd.Mode != domain.ClosingModeLightConfirmation {
		return SettingsSnapshot{}, fmt.Errorf("invalid closing mode")
	}
	current, err := s.loadSettings(ctx)
	if err != nil {
		return SettingsSnapshot{}, err
	}
	if !canManageCloseMode(current.CloseModeAdminUsers, cmd.Actor) {
		return SettingsSnapshot{}, fmt.Errorf("closing mode switch requires whitelist")
	}
	current.ClosingMode = cmd.Mode
	return s.SaveSettings(ctx, current, cmd.Actor)
}

func (s *Service) Dashboard(ctx context.Context, month string) (Dashboard, error) {
	if err := validateMonth(month); err != nil {
		return Dashboard{}, err
	}
	settings, err := s.loadSettings(ctx)
	if err != nil {
		return Dashboard{}, err
	}
	totals, exceptions, err := s.loadTotals(ctx, month)
	if err != nil {
		return Dashboard{}, err
	}
	report, err := s.reportFromTotals(ctx, settings.Settings, totals)
	if err != nil {
		return Dashboard{}, err
	}
	return Dashboard{Settings: settings, Report: report, Exceptions: exceptions}, nil
}

func (s *Service) DraftReport(ctx context.Context, month string) (domain.MonthlyReport, error) {
	if err := validateMonth(month); err != nil {
		return domain.MonthlyReport{}, err
	}
	settings, err := s.loadSettings(ctx)
	if err != nil {
		return domain.MonthlyReport{}, err
	}
	totals, _, err := s.loadTotals(ctx, month)
	if err != nil {
		return domain.MonthlyReport{}, err
	}
	return s.reportFromTotals(ctx, settings.Settings, totals)
}

func (s *Service) CloseMonth(ctx context.Context, cmd CloseMonthCommand) (domain.MonthlyReport, error) {
	if err := validateMonth(cmd.Month); err != nil {
		return domain.MonthlyReport{}, err
	}
	report, err := s.DraftReport(ctx, cmd.Month)
	if err != nil {
		return domain.MonthlyReport{}, err
	}
	report.Status = domain.MonthStatusClosed
	if s.repo == nil {
		return report, nil
	}
	return s.repo.SaveMonthlyReport(ctx, report, strings.TrimSpace(cmd.Actor))
}

func (s *Service) CreateExpense(ctx context.Context, cmd CreateExpenseCommand) (Expense, error) {
	if s.repo == nil {
		return Expense{}, fmt.Errorf("repository required")
	}
	normalized, err := normalizeExpenseCommand(cmd)
	if err != nil {
		return Expense{}, err
	}
	settings, err := s.loadSettings(ctx)
	if err != nil {
		return Expense{}, err
	}
	status, err := s.repo.MonthlyReportStatus(ctx, normalized.Month)
	if err != nil {
		return Expense{}, err
	}
	if !domain.CanEditSourceDocument(settings.Settings, status) {
		return Expense{}, fmt.Errorf("month is closed by strong lock")
	}
	return s.repo.CreateExpense(ctx, normalized)
}

func (s *Service) ListExpenses(ctx context.Context, filter ExpenseFilter) ([]Expense, error) {
	normalized, err := normalizeExpenseFilter(filter)
	if err != nil {
		return nil, err
	}
	if s.repo == nil {
		return []Expense{}, nil
	}
	return s.repo.ListExpenses(ctx, normalized)
}

func (s *Service) ListExpenseEmployees(ctx context.Context) ([]ExpenseEmployee, error) {
	if s.repo == nil {
		return []ExpenseEmployee{}, nil
	}
	return s.repo.ListExpenseEmployees(ctx)
}

func (s *Service) CreateAdjustment(ctx context.Context, cmd CreateAdjustmentCommand) (AdjustmentRecord, error) {
	if s.repo == nil {
		return AdjustmentRecord{}, fmt.Errorf("repository required")
	}
	normalized, err := normalizeAdjustmentCommand(cmd)
	if err != nil {
		return AdjustmentRecord{}, err
	}
	status, err := s.repo.MonthlyReportStatus(ctx, normalized.Month)
	if err != nil {
		return AdjustmentRecord{}, err
	}
	if status != domain.MonthStatusClosed && status != domain.MonthStatusAdjusted {
		return AdjustmentRecord{}, fmt.Errorf("month must be closed before adjustment")
	}
	return s.repo.CreateAdjustment(ctx, normalized)
}

func (s *Service) loadSettings(ctx context.Context) (SettingsSnapshot, error) {
	if s.repo == nil {
		return SettingsSnapshot{Settings: domain.DefaultSettings()}, nil
	}
	snapshot, err := s.repo.LoadSettings(ctx)
	if err != nil {
		return SettingsSnapshot{}, err
	}
	snapshot.Settings = domain.NormalizeSettings(snapshot.Settings)
	return snapshot, nil
}

func (s *Service) loadTotals(ctx context.Context, month string) (domain.MonthlySourceTotals, []Exception, error) {
	if s.repo == nil {
		return domain.MonthlySourceTotals{Month: month}, nil, nil
	}
	totals, exceptions, err := s.repo.MonthlySourceTotals(ctx, month)
	if err != nil {
		return domain.MonthlySourceTotals{}, nil, err
	}
	totals.Month = month
	return totals, exceptions, nil
}

func (s *Service) reportFromTotals(ctx context.Context, settings domain.Settings, totals domain.MonthlySourceTotals) (domain.MonthlyReport, error) {
	report := domain.BuildMonthlyReport(settings, totals)
	if s.repo == nil {
		return report, nil
	}
	rows, err := s.repo.ListAdjustments(ctx, totals.Month)
	if err != nil {
		return domain.MonthlyReport{}, err
	}
	adjustments := make([]domain.Adjustment, 0, len(rows))
	for _, row := range rows {
		adjustments = append(adjustments, domain.Adjustment{Type: row.Type, Amount: row.Amount})
	}
	report = domain.ApplyAdjustments(report, adjustments)
	status, err := s.repo.MonthlyReportStatus(ctx, totals.Month)
	if err != nil {
		return domain.MonthlyReport{}, err
	}
	report.Status = status
	return report, nil
}

func normalizeExpenseCommand(cmd CreateExpenseCommand) (CreateExpenseCommand, error) {
	cmd.Date = strings.TrimSpace(cmd.Date)
	if _, err := time.Parse("2006-01-02", cmd.Date); err != nil {
		return CreateExpenseCommand{}, fmt.Errorf("invalid date")
	}
	cmd.Category = strings.TrimSpace(cmd.Category)
	if cmd.Category == "" {
		return CreateExpenseCommand{}, fmt.Errorf("category required")
	}
	if cmd.Amount <= 0 {
		return CreateExpenseCommand{}, fmt.Errorf("amount must be > 0")
	}
	if cmd.EmployeeID < 0 {
		return CreateExpenseCommand{}, fmt.Errorf("invalid employee_id")
	}
	cmd.Allocation = strings.TrimSpace(cmd.Allocation)
	if cmd.Allocation == "" {
		cmd.Allocation = AllocationPeriodExpense
	}
	if cmd.Allocation != AllocationPeriodExpense && cmd.Allocation != AllocationMainCost {
		return CreateExpenseCommand{}, fmt.Errorf("invalid allocation")
	}
	cmd.Month = monthFromDate(cmd.Date)
	return cmd, nil
}

func normalizeExpenseFilter(filter ExpenseFilter) (ExpenseFilter, error) {
	filter.Month = strings.TrimSpace(filter.Month)
	if err := validateMonth(filter.Month); err != nil {
		return ExpenseFilter{}, err
	}
	if filter.EmployeeID < 0 {
		return ExpenseFilter{}, fmt.Errorf("invalid employee_id")
	}
	return filter, nil
}

func normalizeAdjustmentCommand(cmd CreateAdjustmentCommand) (CreateAdjustmentCommand, error) {
	cmd.Month = strings.TrimSpace(cmd.Month)
	if err := validateMonth(cmd.Month); err != nil {
		return CreateAdjustmentCommand{}, err
	}
	cmd.Type = strings.TrimSpace(cmd.Type)
	switch cmd.Type {
	case domain.AdjustmentRevenue, domain.AdjustmentMainCost, domain.AdjustmentExpense, domain.AdjustmentTax, domain.AdjustmentOther:
	default:
		return CreateAdjustmentCommand{}, fmt.Errorf("invalid adjustment type")
	}
	if cmd.Amount == 0 {
		return CreateAdjustmentCommand{}, fmt.Errorf("amount required")
	}
	cmd.Reason = strings.TrimSpace(cmd.Reason)
	if cmd.Reason == "" {
		return CreateAdjustmentCommand{}, fmt.Errorf("reason required")
	}
	return cmd, nil
}

func validateMonth(month string) error {
	month = strings.TrimSpace(month)
	if _, err := time.Parse("2006-01", month); err != nil {
		return fmt.Errorf("invalid month")
	}
	return nil
}

func monthFromDate(date string) string {
	if len(date) >= 7 {
		return date[:7]
	}
	return ""
}

func canManageCloseMode(users []string, actor string) bool {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return false
	}
	for _, user := range closeModeAdminCandidates(users) {
		if strings.EqualFold(strings.TrimSpace(user), actor) {
			return true
		}
	}
	return false
}

func closeModeAdminCandidates(users []string) []string {
	out := append([]string{}, users...)
	for _, part := range strings.FieldsFunc(os.Getenv("FINANCE_CLOSE_MODE_ADMIN_USERS"), func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	}) {
		if value := strings.TrimSpace(part); value != "" {
			out = append(out, value)
		}
	}
	return out
}
