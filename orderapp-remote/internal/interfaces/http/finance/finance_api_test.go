package finance

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appfinance "orderapp/internal/application/finance"
	domain "orderapp/internal/domain/finance"

	"github.com/labstack/echo/v4"
)

func TestFinanceDashboardAPI(t *testing.T) {
	e, _ := newFinanceTestEcho()
	req := httptest.NewRequest(http.MethodGet, "/api/finance/dashboard?month=2026-05", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"month":"2026-05"`, `"operating_net_profit"`, `"exceptions"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("dashboard response missing %q: %s", want, rec.Body.String())
		}
	}
}

func TestFinanceSettingsAndClosingModeAPI(t *testing.T) {
	e, svc := newFinanceTestEcho()
	req := httptest.NewRequest(http.MethodGet, "/api/finance/settings", nil)
	req.Header.Set("X-Actor", "Van")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"can_manage_close_mode":true`) {
		t.Fatalf("settings status=%d body=%s", rec.Code, rec.Body.String())
	}

	payload := strings.NewReader(`{"mode":"light_confirmation"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/finance/settings/closing-mode", payload)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set("X-Actor", "Van")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("switch status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.mode != domain.ClosingModeLightConfirmation {
		t.Fatalf("mode=%q", svc.mode)
	}
}

func TestFinanceExpenseEmployeesAPI(t *testing.T) {
	e, _ := newFinanceTestEcho()
	req := httptest.NewRequest(http.MethodGet, "/api/finance/employees", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"小王"`) {
		t.Fatalf("employees status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestFinanceExpenseAndClosingAPI(t *testing.T) {
	e, svc := newFinanceTestEcho()
	body := strings.NewReader(`{"date":"2026-05-02","category":"差旅费","amount":3800,"allocation":"period_expense","employee_id":7,"payment":"银行转账"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/finance/expenses", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"category":"差旅费"`) || !strings.Contains(rec.Body.String(), `"payment":"银行转账"`) || !strings.Contains(rec.Body.String(), `"employee_id":7`) {
		t.Fatalf("expense status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/finance/expenses?month=2026-05&employee_id=7", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"employee_name":"小王"`) {
		t.Fatalf("list expenses status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.lastListFilter.EmployeeID != 7 {
		t.Fatalf("employee filter = %d, want 7", svc.lastListFilter.EmployeeID)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/finance/reports/2026-05/close", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"closed"`) {
		t.Fatalf("close status=%d body=%s", rec.Code, rec.Body.String())
	}

	body = strings.NewReader(`{"month":"2026-05","type":"expense","amount":100,"reason":"补记费用"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/finance/adjustments", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"reason":"补记费用"`) {
		t.Fatalf("adjustment status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestFinanceReportExports(t *testing.T) {
	e, _ := newFinanceTestEcho()
	req := httptest.NewRequest(http.MethodGet, "/api/finance/reports/2026-05/pdf", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Header().Get(echo.HeaderContentType) != "application/pdf" || rec.Body.Len() == 0 {
		t.Fatalf("pdf status=%d type=%q bytes=%d", rec.Code, rec.Header().Get(echo.HeaderContentType), rec.Body.Len())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/finance/reports/2026-05/excel", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get(echo.HeaderContentType), "spreadsheetml.sheet") || rec.Body.Len() == 0 {
		t.Fatalf("xlsx status=%d type=%q bytes=%d", rec.Code, rec.Header().Get(echo.HeaderContentType), rec.Body.Len())
	}
}

func newFinanceTestEcho() (*echo.Echo, *fakeFinanceService) {
	e := echo.New()
	svc := &fakeFinanceService{mode: domain.ClosingModeStrongLock}
	RegisterRoutes(e, Dependencies{Finance: svc})
	return e, svc
}

type fakeFinanceService struct {
	mode           string
	lastListFilter appfinance.ExpenseFilter
}

func (s *fakeFinanceService) Settings(context.Context, string) (appfinance.SettingsSnapshot, error) {
	settings := domain.DefaultSettings()
	settings.ClosingMode = s.mode
	return appfinance.SettingsSnapshot{Settings: settings, CloseModeAdminUsers: []string{"Van"}, CanManageCloseMode: true}, nil
}

func (s *fakeFinanceService) SaveSettings(_ context.Context, snapshot appfinance.SettingsSnapshot, _ string) (appfinance.SettingsSnapshot, error) {
	s.mode = snapshot.ClosingMode
	return snapshot, nil
}

func (s *fakeFinanceService) SwitchClosingMode(_ context.Context, cmd appfinance.SwitchClosingModeCommand) (appfinance.SettingsSnapshot, error) {
	s.mode = cmd.Mode
	settings := domain.DefaultSettings()
	settings.ClosingMode = cmd.Mode
	return appfinance.SettingsSnapshot{Settings: settings, CanManageCloseMode: true}, nil
}

func (s *fakeFinanceService) Dashboard(context.Context, string) (appfinance.Dashboard, error) {
	report := domain.BuildMonthlyReport(domain.DefaultSettings(), domain.MonthlySourceTotals{Month: "2026-05", RevenueTaxInclusive: 103000})
	return appfinance.Dashboard{Report: report, Exceptions: []appfinance.Exception{{Code: "unclosed_month", Message: "本月未结账"}}}, nil
}

func (s *fakeFinanceService) ListExpenseEmployees(context.Context) ([]appfinance.ExpenseEmployee, error) {
	return []appfinance.ExpenseEmployee{{ID: 7, Name: "小王", Active: true}}, nil
}

func (s *fakeFinanceService) CreateExpense(_ context.Context, cmd appfinance.CreateExpenseCommand) (appfinance.Expense, error) {
	return appfinance.Expense{ID: 1, Date: cmd.Date, Month: "2026-05", Category: cmd.Category, Amount: cmd.Amount, Allocation: cmd.Allocation, EmployeeID: cmd.EmployeeID, EmployeeName: "小王", Payment: cmd.Payment}, nil
}

func (s *fakeFinanceService) ListExpenses(_ context.Context, filter appfinance.ExpenseFilter) ([]appfinance.Expense, error) {
	s.lastListFilter = filter
	return []appfinance.Expense{{ID: 1, Date: "2026-05-02", Month: "2026-05", Category: "房租", Amount: 3800, Allocation: appfinance.AllocationPeriodExpense, EmployeeID: 7, EmployeeName: "小王"}}, nil
}

func (s *fakeFinanceService) DraftReport(context.Context, string) (domain.MonthlyReport, error) {
	return domain.BuildMonthlyReport(domain.DefaultSettings(), domain.MonthlySourceTotals{Month: "2026-05", RevenueTaxInclusive: 103000}), nil
}

func (s *fakeFinanceService) CloseMonth(context.Context, appfinance.CloseMonthCommand) (domain.MonthlyReport, error) {
	report := domain.BuildMonthlyReport(domain.DefaultSettings(), domain.MonthlySourceTotals{Month: "2026-05", RevenueTaxInclusive: 103000})
	report.Status = domain.MonthStatusClosed
	return report, nil
}

func (s *fakeFinanceService) CreateAdjustment(_ context.Context, cmd appfinance.CreateAdjustmentCommand) (appfinance.AdjustmentRecord, error) {
	return appfinance.AdjustmentRecord{ID: 1, Month: cmd.Month, Type: cmd.Type, Amount: cmd.Amount, Reason: cmd.Reason}, nil
}

func decodeJSON(t *testing.T, body string, out any) {
	t.Helper()
	if err := json.Unmarshal([]byte(body), out); err != nil {
		t.Fatalf("invalid json: %v body=%s", err, body)
	}
}
