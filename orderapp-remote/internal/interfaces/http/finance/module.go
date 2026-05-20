package finance

import (
	"context"

	customerfulfillmentapp "orderapp/internal/application/customerfulfillment"
	appfinance "orderapp/internal/application/finance"
	domain "orderapp/internal/domain/finance"

	"github.com/labstack/echo/v4"
)

type Service interface {
	Settings(context.Context, string) (appfinance.SettingsSnapshot, error)
	SaveSettings(context.Context, appfinance.SettingsSnapshot, string) (appfinance.SettingsSnapshot, error)
	SwitchClosingMode(context.Context, appfinance.SwitchClosingModeCommand) (appfinance.SettingsSnapshot, error)
	Dashboard(context.Context, string) (appfinance.Dashboard, error)
	CreateExpense(context.Context, appfinance.CreateExpenseCommand) (appfinance.Expense, error)
	ListExpenses(context.Context, appfinance.ExpenseFilter) ([]appfinance.Expense, error)
	ListExpenseEmployees(context.Context) ([]appfinance.ExpenseEmployee, error)
	ClosingReview(context.Context, appfinance.ReportFilter) (appfinance.ClosingReview, error)
	ReportDrilldown(context.Context, appfinance.ReportFilter) (appfinance.ReportDrilldown, error)
	ListTaxLedger(context.Context, string) ([]appfinance.TaxLedgerEntry, error)
	CreateTaxLedgerEntry(context.Context, appfinance.CreateTaxLedgerCommand) (appfinance.TaxLedgerEntry, error)
	AccountantHandoff(context.Context, appfinance.ReportFilter) (appfinance.AccountantHandoff, error)
	DraftReport(context.Context, appfinance.ReportFilter) (domain.MonthlyReport, error)
	CloseMonth(context.Context, appfinance.CloseMonthCommand) (domain.MonthlyReport, error)
	CreateAdjustment(context.Context, appfinance.CreateAdjustmentCommand) (appfinance.AdjustmentRecord, error)
}

type CustomerAccountContextService interface {
	CustomerPortalOverview(context.Context, int64) (customerfulfillmentapp.CustomerPortalOverview, error)
}

type Dependencies struct {
	Finance          Service
	CustomerAccounts CustomerAccountContextService
}

func RegisterRoutes(e *echo.Echo, deps Dependencies) {
	registerFinanceAPI(e, deps.Finance, deps.CustomerAccounts)
}
