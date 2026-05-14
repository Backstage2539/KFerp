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
	ID            int64        `json:"id"`
	Date          string       `json:"date"`
	Month         string       `json:"month"`
	Category      string       `json:"category"`
	Amount        domain.Money `json:"amount"`
	Allocation    string       `json:"allocation"`
	EmployeeID    int64        `json:"employee_id,omitempty"`
	EmployeeName  string       `json:"employee_name,omitempty"`
	OrderID       int64        `json:"order_id,omitempty"`
	CustomerID    int64        `json:"customer_id,omitempty"`
	ProductID     int64        `json:"product_id,omitempty"`
	BatchNo       string       `json:"batch_no,omitempty"`
	DimensionNote string       `json:"dimension_note,omitempty"`
	Payment       string       `json:"payment,omitempty"`
	Note          string       `json:"note,omitempty"`
	Actor         string       `json:"actor,omitempty"`
	CreatedAt     string       `json:"created_at,omitempty"`
}

type CreateExpenseCommand struct {
	Date          string       `json:"date"`
	Month         string       `json:"month,omitempty"`
	Category      string       `json:"category"`
	Amount        domain.Money `json:"amount"`
	Allocation    string       `json:"allocation"`
	EmployeeID    int64        `json:"employee_id,omitempty"`
	OrderID       int64        `json:"order_id,omitempty"`
	CustomerID    int64        `json:"customer_id,omitempty"`
	ProductID     int64        `json:"product_id,omitempty"`
	BatchNo       string       `json:"batch_no,omitempty"`
	DimensionNote string       `json:"dimension_note,omitempty"`
	Payment       string       `json:"payment,omitempty"`
	Note          string       `json:"note,omitempty"`
	Actor         string       `json:"-"`
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

type ClosingReview struct {
	Month string             `json:"month"`
	Items []ClosingCheckItem `json:"items"`
}

func (r ClosingReview) HasCode(code string) bool {
	for _, item := range r.Items {
		if item.Code == code {
			return true
		}
	}
	return false
}

type ClosingCheckItem struct {
	Code     string `json:"code"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Count    int    `json:"count,omitempty"`
}

type SourceDetail struct {
	Section      string       `json:"section"`
	SourceType   string       `json:"source_type"`
	SourceID     int64        `json:"source_id,omitempty"`
	Date         string       `json:"date,omitempty"`
	Name         string       `json:"name"`
	Category     string       `json:"category,omitempty"`
	Counterparty string       `json:"counterparty,omitempty"`
	Amount       domain.Money `json:"amount"`
	Link         string       `json:"link,omitempty"`
}

type DrilldownSection struct {
	Section string         `json:"section"`
	Title   string         `json:"title"`
	Total   domain.Money   `json:"total"`
	Rows    []SourceDetail `json:"rows"`
}

type ReportDrilldown struct {
	Month    string             `json:"month"`
	Sections []DrilldownSection `json:"sections"`
}

func (d ReportDrilldown) SectionTotal(section string) domain.Money {
	for _, row := range d.Sections {
		if row.Section == section {
			return row.Total
		}
	}
	return 0
}

type TaxLedgerEntry struct {
	ID           int64        `json:"id"`
	Month        string       `json:"month"`
	Kind         string       `json:"kind"`
	InvoiceNo    string       `json:"invoice_no,omitempty"`
	Counterparty string       `json:"counterparty,omitempty"`
	TotalAmount  domain.Money `json:"total_amount"`
	TaxAmount    domain.Money `json:"tax_amount"`
	Status       string       `json:"status"`
	Note         string       `json:"note,omitempty"`
	Actor        string       `json:"actor,omitempty"`
	CreatedAt    string       `json:"created_at,omitempty"`
}

type CreateTaxLedgerCommand struct {
	Month        string       `json:"month"`
	Kind         string       `json:"kind"`
	InvoiceNo    string       `json:"invoice_no,omitempty"`
	Counterparty string       `json:"counterparty,omitempty"`
	TotalAmount  domain.Money `json:"total_amount"`
	TaxAmount    domain.Money `json:"tax_amount"`
	Status       string       `json:"status"`
	Note         string       `json:"note,omitempty"`
	Actor        string       `json:"-"`
}

type VoucherDraft struct {
	Summary string       `json:"summary"`
	Debit   string       `json:"debit"`
	Credit  string       `json:"credit"`
	Amount  domain.Money `json:"amount"`
	Source  string       `json:"source,omitempty"`
}

type AccountantHandoff struct {
	Month         string               `json:"month"`
	Report        domain.MonthlyReport `json:"report"`
	Checklist     []ClosingCheckItem   `json:"checklist"`
	Drilldown     ReportDrilldown      `json:"drilldown"`
	TaxLedger     []TaxLedgerEntry     `json:"tax_ledger"`
	VoucherDrafts []VoucherDraft       `json:"voucher_drafts"`
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
	FinanceSourceDetails(ctx context.Context, month string) ([]SourceDetail, error)
	ListTaxLedger(ctx context.Context, month string) ([]TaxLedgerEntry, error)
	CreateTaxLedgerEntry(ctx context.Context, cmd CreateTaxLedgerCommand) (TaxLedgerEntry, error)
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
	if report.Status != domain.MonthStatusAdjusted {
		report.Status = domain.MonthStatusClosed
	}
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
	if err := s.ensureActiveExpenseEmployee(ctx, normalized.EmployeeID); err != nil {
		return Expense{}, err
	}
	return s.repo.CreateExpense(ctx, normalized)
}

func (s *Service) ensureActiveExpenseEmployee(ctx context.Context, employeeID int64) error {
	if employeeID <= 0 {
		return nil
	}
	rows, err := s.repo.ListExpenseEmployees(ctx)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.ID != employeeID {
			continue
		}
		if !row.Active {
			return fmt.Errorf("employee inactive")
		}
		return nil
	}
	return fmt.Errorf("employee not found")
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

func (s *Service) ClosingReview(ctx context.Context, month string) (ClosingReview, error) {
	if err := validateMonth(month); err != nil {
		return ClosingReview{}, err
	}
	dashboard, err := s.Dashboard(ctx, month)
	if err != nil {
		return ClosingReview{}, err
	}
	ledger, err := s.ListTaxLedger(ctx, month)
	if err != nil {
		return ClosingReview{}, err
	}
	drilldown, err := s.ReportDrilldown(ctx, month)
	if err != nil {
		return ClosingReview{}, err
	}
	items := []ClosingCheckItem{
		{
			Code:     "source_exceptions",
			Title:    "来源异常",
			Status:   statusFromCount(len(dashboard.Exceptions)),
			Severity: severityFromCount(len(dashboard.Exceptions)),
			Message:  sourceExceptionMessage(len(dashboard.Exceptions)),
			Count:    len(dashboard.Exceptions),
		},
		{
			Code:     "source_drilldown",
			Title:    "报表来源明细",
			Status:   statusFromCount(emptyDrilldownSections(drilldown)),
			Severity: severityFromCount(emptyDrilldownSections(drilldown)),
			Message:  drilldownMessage(drilldown),
			Count:    len(drilldown.Sections),
		},
		{
			Code:     "tax_ledger",
			Title:    "票税台账",
			Status:   statusFromCount(emptyTaxLedgerCount(ledger)),
			Severity: severityFromCount(emptyTaxLedgerCount(ledger)),
			Message:  taxLedgerMessage(ledger),
			Count:    len(ledger),
		},
		{
			Code:     "cost_matching",
			Title:    "成本配比",
			Status:   costMatchingStatus(dashboard.Report),
			Severity: costMatchingSeverity(dashboard.Report),
			Message:  costMatchingMessage(dashboard.Report),
		},
		{
			Code:     "accountant_handoff",
			Title:    "会计交接",
			Status:   "ok",
			Severity: "info",
			Message:  "可导出会计交接 Excel，包含结账检查、来源明细、票税台账和凭证草稿。",
		},
	}
	return ClosingReview{Month: month, Items: items}, nil
}

func (s *Service) ReportDrilldown(ctx context.Context, month string) (ReportDrilldown, error) {
	if err := validateMonth(month); err != nil {
		return ReportDrilldown{}, err
	}
	if s.repo == nil {
		return ReportDrilldown{Month: month}, nil
	}
	rows, err := s.repo.FinanceSourceDetails(ctx, month)
	if err != nil {
		return ReportDrilldown{}, err
	}
	return drilldownFromDetails(month, rows), nil
}

func (s *Service) ListTaxLedger(ctx context.Context, month string) ([]TaxLedgerEntry, error) {
	if err := validateMonth(month); err != nil {
		return nil, err
	}
	if s.repo == nil {
		return []TaxLedgerEntry{}, nil
	}
	return s.repo.ListTaxLedger(ctx, month)
}

func (s *Service) CreateTaxLedgerEntry(ctx context.Context, cmd CreateTaxLedgerCommand) (TaxLedgerEntry, error) {
	if s.repo == nil {
		return TaxLedgerEntry{}, fmt.Errorf("repository required")
	}
	normalized, err := normalizeTaxLedgerCommand(cmd)
	if err != nil {
		return TaxLedgerEntry{}, err
	}
	settings, err := s.loadSettings(ctx)
	if err != nil {
		return TaxLedgerEntry{}, err
	}
	status, err := s.repo.MonthlyReportStatus(ctx, normalized.Month)
	if err != nil {
		return TaxLedgerEntry{}, err
	}
	if !domain.CanEditSourceDocument(settings.Settings, status) {
		return TaxLedgerEntry{}, fmt.Errorf("month is closed by strong lock")
	}
	return s.repo.CreateTaxLedgerEntry(ctx, normalized)
}

func (s *Service) AccountantHandoff(ctx context.Context, month string) (AccountantHandoff, error) {
	if err := validateMonth(month); err != nil {
		return AccountantHandoff{}, err
	}
	report, err := s.DraftReport(ctx, month)
	if err != nil {
		return AccountantHandoff{}, err
	}
	review, err := s.ClosingReview(ctx, month)
	if err != nil {
		return AccountantHandoff{}, err
	}
	drilldown, err := s.ReportDrilldown(ctx, month)
	if err != nil {
		return AccountantHandoff{}, err
	}
	ledger, err := s.ListTaxLedger(ctx, month)
	if err != nil {
		return AccountantHandoff{}, err
	}
	return AccountantHandoff{
		Month:         month,
		Report:        report,
		Checklist:     review.Items,
		Drilldown:     drilldown,
		TaxLedger:     ledger,
		VoucherDrafts: voucherDraftsFromReport(report),
	}, nil
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
	if cmd.OrderID < 0 {
		return CreateExpenseCommand{}, fmt.Errorf("invalid order_id")
	}
	if cmd.CustomerID < 0 {
		return CreateExpenseCommand{}, fmt.Errorf("invalid customer_id")
	}
	if cmd.ProductID < 0 {
		return CreateExpenseCommand{}, fmt.Errorf("invalid product_id")
	}
	cmd.BatchNo = strings.TrimSpace(cmd.BatchNo)
	cmd.DimensionNote = strings.TrimSpace(cmd.DimensionNote)
	cmd.Payment = strings.TrimSpace(cmd.Payment)
	cmd.Note = strings.TrimSpace(cmd.Note)
	cmd.Actor = strings.TrimSpace(cmd.Actor)
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

func normalizeTaxLedgerCommand(cmd CreateTaxLedgerCommand) (CreateTaxLedgerCommand, error) {
	cmd.Month = strings.TrimSpace(cmd.Month)
	if err := validateMonth(cmd.Month); err != nil {
		return CreateTaxLedgerCommand{}, err
	}
	cmd.Kind = strings.TrimSpace(cmd.Kind)
	switch cmd.Kind {
	case "sales_invoice", "purchase_invoice", "tax_payment", "other":
	default:
		return CreateTaxLedgerCommand{}, fmt.Errorf("invalid tax ledger kind")
	}
	cmd.InvoiceNo = strings.TrimSpace(cmd.InvoiceNo)
	cmd.Counterparty = strings.TrimSpace(cmd.Counterparty)
	if cmd.TotalAmount <= 0 {
		return CreateTaxLedgerCommand{}, fmt.Errorf("total amount must be > 0")
	}
	if cmd.TaxAmount < 0 {
		return CreateTaxLedgerCommand{}, fmt.Errorf("tax amount must be >= 0")
	}
	if cmd.TaxAmount > cmd.TotalAmount {
		return CreateTaxLedgerCommand{}, fmt.Errorf("tax amount exceeds total amount")
	}
	cmd.Status = strings.TrimSpace(cmd.Status)
	if cmd.Status == "" {
		cmd.Status = "pending"
	}
	switch cmd.Status {
	case "pending", "confirmed", "matched":
	default:
		return CreateTaxLedgerCommand{}, fmt.Errorf("invalid tax ledger status")
	}
	cmd.Note = strings.TrimSpace(cmd.Note)
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	return cmd, nil
}

func drilldownFromDetails(month string, rows []SourceDetail) ReportDrilldown {
	order := []string{"revenue", "main_cost", "period_expense", "tax"}
	seen := map[string]*DrilldownSection{}
	for _, row := range rows {
		row.Section = strings.TrimSpace(row.Section)
		if row.Section == "" {
			row.Section = "other"
		}
		if _, ok := seen[row.Section]; !ok {
			seen[row.Section] = &DrilldownSection{Section: row.Section, Title: titleForDrilldownSection(row.Section)}
			if !containsString(order, row.Section) {
				order = append(order, row.Section)
			}
		}
		section := seen[row.Section]
		section.Total += row.Amount
		section.Rows = append(section.Rows, row)
	}
	sections := make([]DrilldownSection, 0, len(seen))
	for _, key := range order {
		if section, ok := seen[key]; ok {
			sections = append(sections, *section)
		}
	}
	return ReportDrilldown{Month: month, Sections: sections}
}

func titleForDrilldownSection(section string) string {
	switch section {
	case "revenue":
		return "收入来源"
	case "main_cost":
		return "主营成本来源"
	case "period_expense":
		return "期间费用来源"
	case "tax":
		return "票税来源"
	default:
		return "其他来源"
	}
}

func containsString(rows []string, want string) bool {
	for _, row := range rows {
		if row == want {
			return true
		}
	}
	return false
}

func statusFromCount(count int) string {
	if count > 0 {
		return "warn"
	}
	return "ok"
}

func severityFromCount(count int) string {
	if count > 0 {
		return "warning"
	}
	return "info"
}

func sourceExceptionMessage(count int) string {
	if count > 0 {
		return fmt.Sprintf("发现 %d 项来源异常，结账前建议处理。", count)
	}
	return "未发现来源异常。"
}

func emptyDrilldownSections(drilldown ReportDrilldown) int {
	if len(drilldown.Sections) == 0 {
		return 1
	}
	return 0
}

func drilldownMessage(drilldown ReportDrilldown) string {
	if len(drilldown.Sections) == 0 {
		return "本月暂未归集到可钻取来源。"
	}
	return "收入、成本、费用来源明细已生成，可在经营报告中钻取查看。"
}

func emptyTaxLedgerCount(rows []TaxLedgerEntry) int {
	if len(rows) == 0 {
		return 1
	}
	return 0
}

func taxLedgerMessage(rows []TaxLedgerEntry) string {
	if len(rows) == 0 {
		return "本月暂无票税台账记录，结账前建议补齐发票、税款或未取票说明。"
	}
	return fmt.Sprintf("本月已有 %d 条票税台账记录。", len(rows))
}

func costMatchingStatus(report domain.MonthlyReport) string {
	if report.RevenueTaxInclusive > 0 && report.MainBusinessCost <= 0 {
		return "warn"
	}
	return "ok"
}

func costMatchingSeverity(report domain.MonthlyReport) string {
	if costMatchingStatus(report) == "warn" {
		return "warning"
	}
	return "info"
}

func costMatchingMessage(report domain.MonthlyReport) string {
	if costMatchingStatus(report) == "warn" {
		return "本月有收入但未匹配主营成本，需确认生产成本、进货成本或主营成本费用归集。"
	}
	return "主营成本已参与毛利测算。"
}

func voucherDraftsFromReport(report domain.MonthlyReport) []VoucherDraft {
	rows := []VoucherDraft{
		{Summary: report.Month + " 收入结转", Debit: "应收账款/银行存款", Credit: "主营业务收入", Amount: report.TaxExclusiveRevenue, Source: "finance_report.revenue"},
		{Summary: report.Month + " 主营成本结转", Debit: "主营业务成本", Credit: "库存商品/生产成本", Amount: report.MainBusinessCost, Source: "finance_report.main_cost"},
		{Summary: report.Month + " 期间费用归集", Debit: "销售费用/管理费用", Credit: "银行存款/应付账款", Amount: report.PeriodExpenses, Source: "finance_report.expense"},
		{Summary: report.Month + " 增值税估算", Debit: "应交税费-应交增值税", Credit: "应交税费-未交增值税", Amount: report.Tax.VATPayable, Source: "finance_report.tax"},
		{Summary: report.Month + " 附加税估算", Debit: "税金及附加", Credit: "应交税费-附加税", Amount: report.Tax.Surtax, Source: "finance_report.tax"},
		{Summary: report.Month + " 企业所得税估算", Debit: "所得税费用", Credit: "应交税费-企业所得税", Amount: report.Tax.CITPayable, Source: "finance_report.tax"},
		{Summary: report.Month + " 本年利润结转", Debit: "主营业务收入", Credit: "本年利润", Amount: report.AdjustedNetProfit, Source: "finance_report.net_profit"},
	}
	out := make([]VoucherDraft, 0, len(rows))
	for _, row := range rows {
		if row.Amount < 0 {
			row.Amount = -row.Amount
		}
		out = append(out, row)
	}
	return out
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
