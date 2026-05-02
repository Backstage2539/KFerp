package finance

import (
	"context"

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
	DraftReport(context.Context, string) (domain.MonthlyReport, error)
	CloseMonth(context.Context, appfinance.CloseMonthCommand) (domain.MonthlyReport, error)
	CreateAdjustment(context.Context, appfinance.CreateAdjustmentCommand) (appfinance.AdjustmentRecord, error)
}

type Dependencies struct {
	Finance Service
}

func RegisterRoutes(e *echo.Echo, deps Dependencies) {
	registerFinanceAPI(e, deps.Finance)
}
