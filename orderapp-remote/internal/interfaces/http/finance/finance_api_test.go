package finance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	authzapp "orderapp/internal/application/authz"
	customerfulfillmentapp "orderapp/internal/application/customerfulfillment"
	appfinance "orderapp/internal/application/finance"
	domain "orderapp/internal/domain/finance"
	postgresfinance "orderapp/internal/infrastructure/postgres/finance"
	"orderapp/internal/interfaces/http/support"

	"github.com/jackc/pgx/v5/pgxpool"
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
	body := strings.NewReader(`{"date":"2026-05-02","category":"差旅费","amount":3800,"allocation":"period_expense","employee_id":7,"payment":"银行转账","order_id":256,"customer_id":18,"product_id":9,"batch_no":"BATCH-0503","dimension_note":"客户样品"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/finance/expenses", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"category":"差旅费"`) || !strings.Contains(rec.Body.String(), `"payment":"银行转账"`) || !strings.Contains(rec.Body.String(), `"employee_id":7`) || !strings.Contains(rec.Body.String(), `"order_id":256`) {
		t.Fatalf("expense status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/finance/expenses?month=2026-05&employee_id=7&customer_id=18", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"employee_name":"小王"`) {
		t.Fatalf("list expenses status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.lastListFilter.EmployeeID != 7 {
		t.Fatalf("employee filter = %d, want 7", svc.lastListFilter.EmployeeID)
	}
	if svc.lastListFilter.CustomerID != 18 {
		t.Fatalf("customer filter = %d, want 18", svc.lastListFilter.CustomerID)
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

func TestFinanceImprovementAPIs(t *testing.T) {
	e, _ := newFinanceTestEcho()
	for _, tc := range []struct {
		path string
		want string
	}{
		{"/api/finance/reports/2026-05/closing-review", `"source_exceptions"`},
		{"/api/finance/reports/2026-05/drilldown", `"section":"revenue"`},
		{"/api/finance/tax-ledger?month=2026-05", `"invoice_no":"INV-001"`},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), tc.want) {
			t.Fatalf("%s status=%d body=%s", tc.path, rec.Code, rec.Body.String())
		}
	}

	body := strings.NewReader(`{"month":"2026-05","kind":"purchase_invoice","invoice_no":"PINV-001","counterparty":"生豆供应商","total_amount":500,"tax_amount":15,"status":"pending"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/finance/tax-ledger", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"invoice_no":"PINV-001"`) {
		t.Fatalf("create tax ledger status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/finance/reports/2026-05/accountant-handoff.xlsx", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get(echo.HeaderContentType), "spreadsheetml.sheet") || rec.Body.Len() == 0 {
		t.Fatalf("accountant handoff status=%d type=%q bytes=%d", rec.Code, rec.Header().Get(echo.HeaderContentType), rec.Body.Len())
	}
}

func TestFinanceReportAPIAppliesCustomerScope(t *testing.T) {
	e, svc := newFinanceTestEcho()

	req := httptest.NewRequest(http.MethodGet, "/api/finance/reports/2026-05?customer_id=18", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("report status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.lastDraftReportFilter.CustomerID != 18 {
		t.Fatalf("draft report customer filter = %d, want 18", svc.lastDraftReportFilter.CustomerID)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/finance/reports/2026-05/closing-review?customer_id=18", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("closing review status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.lastClosingReviewFilter.CustomerID != 18 {
		t.Fatalf("closing review customer filter = %d, want 18", svc.lastClosingReviewFilter.CustomerID)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/finance/reports/2026-05/drilldown?customer_id=18", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("drilldown status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.lastDrilldownFilter.CustomerID != 18 {
		t.Fatalf("drilldown customer filter = %d, want 18", svc.lastDrilldownFilter.CustomerID)
	}
}

func TestCustomerAccountFinanceReadAPIDerivesBoundCustomerAndRejectsCrossCustomer(t *testing.T) {
	e := echo.New()
	svc := &fakeFinanceService{mode: domain.ClosingModeStrongLock}
	customerAccounts := &fakeFinanceCustomerAccounts{
		overview: customerfulfillmentapp.CustomerPortalOverview{CustomerID: 18, CustomerName: "客户A"},
	}
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(7))
			return next(c)
		}
	})
	e.Use(support.AuthorizationMiddleware(&fakeFinanceAuthzService{
		actor: authzapp.Actor{Permissions: []string{"customer_processing.read"}},
	}))
	RegisterRoutes(e, Dependencies{Finance: svc, CustomerAccounts: customerAccounts})

	req := httptest.NewRequest(http.MethodGet, "/api/finance/expenses?month=2026-05", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("customer expenses status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.lastListFilter.CustomerID != 18 {
		t.Fatalf("customer expenses filter customer=%d, want bound customer 18", svc.lastListFilter.CustomerID)
	}
	if customerAccounts.lastEmployeeID != 7 {
		t.Fatalf("customer context employee=%d, want 7", customerAccounts.lastEmployeeID)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/finance/reports/2026-05", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("customer report status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.lastDraftReportFilter.CustomerID != 18 {
		t.Fatalf("customer report filter customer=%d, want bound customer 18", svc.lastDraftReportFilter.CustomerID)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/finance/reports/2026-05?customer_id=19", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "customer finance scope denied") {
		t.Fatalf("cross-customer report status=%d body=%s, want 403 scope denied", rec.Code, rec.Body.String())
	}

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/finance/employees"},
		{http.MethodPost, "/api/finance/expenses"},
		{http.MethodGet, "/api/finance/reports/2026-05/accountant-handoff.xlsx"},
	} {
		req = httptest.NewRequest(tc.method, tc.path, nil)
		rec = httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s %s status=%d body=%s, want 403", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}

func TestFinanceTaxLedgerAPIReturnsBadRequestWhenServiceRejectsClosedMonth(t *testing.T) {
	e, svc := newFinanceTestEcho()
	svc.taxLedgerErr = errors.New("month is closed by strong lock")

	body := strings.NewReader(`{"month":"2026-05","kind":"sales_invoice","invoice_no":"INV-CLOSED","counterparty":"客户A","total_amount":1000,"tax_amount":30,"status":"confirmed"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/finance/tax-ledger", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "month is closed by strong lock") {
		t.Fatalf("tax ledger closed month status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestFinanceTaxLedgerAPIRejectsDuplicateInvoiceNoWithoutWritingLedger(t *testing.T) {
	pool, schema := newFinanceAPIPostgresTestDB(t)
	ctx := context.Background()
	e := echo.New()
	RegisterRoutes(e, Dependencies{Finance: appfinance.NewService(postgresfinance.NewRepository(pool, schema))})

	first := strings.NewReader(`{"month":"2026-05","kind":"purchase_invoice","invoice_no":"PINV-DUP-001","counterparty":"生豆供应商A","total_amount":1000,"tax_amount":30,"status":"confirmed"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/finance/tax-ledger", first)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"invoice_no":"PINV-DUP-001"`) {
		t.Fatalf("first tax ledger status=%d body=%s", rec.Code, rec.Body.String())
	}

	duplicate := strings.NewReader(`{"month":"2026-06","kind":"purchase_invoice","invoice_no":"PINV-DUP-001","counterparty":"生豆供应商B","total_amount":1200,"tax_amount":36,"status":"confirmed"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/finance/tax-ledger", duplicate)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "tax ledger invoice already exists") {
		t.Fatalf("duplicate tax ledger status=%d body=%s, want 400 duplicate invoice", rec.Code, rec.Body.String())
	}

	var ledgerCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*)::int FROM %s.finance_tax_ledger WHERE kind='purchase_invoice' AND invoice_no='PINV-DUP-001'`, schema)).Scan(&ledgerCount); err != nil {
		t.Fatalf("query duplicate tax ledger count: %v", err)
	}
	if ledgerCount != 1 {
		t.Fatalf("duplicate invoice ledger rows = %d, want 1", ledgerCount)
	}
}

func TestFinanceAdjustmentAPIRejectsDraftMonthWithoutWritingAdjustment(t *testing.T) {
	pool, schema := newFinanceAPIPostgresTestDB(t)
	ctx := context.Background()
	e := echo.New()
	RegisterRoutes(e, Dependencies{Finance: appfinance.NewService(postgresfinance.NewRepository(pool, schema))})

	body := strings.NewReader(`{"month":"2026-06","type":"expense","amount":100,"reason":"未结账月误调"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/finance/adjustments", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "month must be closed before adjustment") {
		t.Fatalf("draft month adjustment status=%d body=%s, want 400 month must be closed", rec.Code, rec.Body.String())
	}

	var adjustmentCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*)::int FROM %s.finance_adjustments WHERE month='2026-06'`, schema)).Scan(&adjustmentCount); err != nil {
		t.Fatalf("query adjustment count: %v", err)
	}
	if adjustmentCount != 0 {
		t.Fatalf("draft month adjustment rows = %d, want 0", adjustmentCount)
	}
}

func TestFinanceExpenseAPIRejectsInactiveEmployeeWithoutWritingExpense(t *testing.T) {
	pool, schema := newFinanceAPIPostgresTestDB(t)
	ctx := context.Background()
	mustExecFinanceAPISQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.company_employees(id, name, active) VALUES (7, '离职员工', false), (8, '在职员工', true);
	`, schema))
	e := echo.New()
	RegisterRoutes(e, Dependencies{Finance: appfinance.NewService(postgresfinance.NewRepository(pool, schema))})

	body := strings.NewReader(`{"date":"2026-05-02","category":"样品费","amount":120,"allocation":"period_expense","employee_id":7}`)
	req := httptest.NewRequest(http.MethodPost, "/api/finance/expenses", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "employee inactive") {
		t.Fatalf("inactive employee expense status=%d body=%s, want 400 employee inactive", rec.Code, rec.Body.String())
	}
	var expenseCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*)::int FROM %s.finance_expenses WHERE employee_id=7`, schema)).Scan(&expenseCount); err != nil {
		t.Fatalf("query inactive employee expenses: %v", err)
	}
	if expenseCount != 0 {
		t.Fatalf("inactive employee expense rows = %d, want 0", expenseCount)
	}

	body = strings.NewReader(`{"date":"2026-05-02","category":"样品费","amount":120,"allocation":"period_expense","employee_id":8}`)
	req = httptest.NewRequest(http.MethodPost, "/api/finance/expenses", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"employee_id":8`) {
		t.Fatalf("active employee expense status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestFinanceExpenseAPIRejectsMissingDimensionReferencesWithoutWritingExpense(t *testing.T) {
	pool, schema := newFinanceAPIPostgresTestDB(t)
	ctx := context.Background()
	mustExecFinanceAPISQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.customers(id, name, company_name) VALUES (18, '维度客户', '维度客户公司');
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, total_amount, grand_total, is_void) VALUES (256, 'SO-DIM-OK', '2026-05-02', 18, 120, 120, false);
		INSERT INTO %s.products(id, name, active) VALUES (9, '维度商品', true);
		INSERT INTO %s.order_items(order_id, line_no, product_id, item_name, qty, unit, spec, unit_price, line_total) VALUES (256, 1, 9, '维度商品', 1, '件', '454g', 120, 120);
	`, schema, schema, schema, schema))
	e := echo.New()
	RegisterRoutes(e, Dependencies{Finance: appfinance.NewService(postgresfinance.NewRepository(pool, schema))})

	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "missing order",
			body: `{"date":"2026-05-02","category":"样品费","amount":120,"allocation":"period_expense","order_id":999,"customer_id":18,"product_id":9}`,
			want: "finance dimension order not found",
		},
		{
			name: "missing customer",
			body: `{"date":"2026-05-02","category":"样品费","amount":120,"allocation":"period_expense","order_id":256,"customer_id":999,"product_id":9}`,
			want: "finance dimension customer not found",
		},
		{
			name: "missing product",
			body: `{"date":"2026-05-02","category":"样品费","amount":120,"allocation":"period_expense","order_id":256,"customer_id":18,"product_id":999}`,
			want: "finance dimension product not found",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/finance/expenses", strings.NewReader(tc.body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("%s status=%d body=%s, want 400 %s", tc.name, rec.Code, rec.Body.String(), tc.want)
			}
		})
	}

	var expenseCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*)::int FROM %s.finance_expenses`, schema)).Scan(&expenseCount); err != nil {
		t.Fatalf("query dimension expense count: %v", err)
	}
	if expenseCount != 0 {
		t.Fatalf("missing dimension expense rows = %d, want 0", expenseCount)
	}

	body := strings.NewReader(`{"date":"2026-05-02","category":"样品费","amount":120,"allocation":"period_expense","order_id":256,"customer_id":18,"product_id":9}`)
	req := httptest.NewRequest(http.MethodPost, "/api/finance/expenses", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"order_id":256`) || !strings.Contains(rec.Body.String(), `"customer_id":18`) || !strings.Contains(rec.Body.String(), `"product_id":9`) {
		t.Fatalf("valid dimension expense status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestFinanceExpenseAPIRejectsOrderCustomerMismatchWithoutWritingExpense(t *testing.T) {
	pool, schema := newFinanceAPIPostgresTestDB(t)
	ctx := context.Background()
	mustExecFinanceAPISQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.customers(id, name, company_name) VALUES (18, '订单归属客户', '订单归属客户公司'), (19, '错误归集客户', '错误归集客户公司');
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, total_amount, grand_total, is_void) VALUES (256, 'SO-CUSTOMER-MATCH', '2026-05-02', 18, 120, 120, false);
	`, schema, schema))
	e := echo.New()
	RegisterRoutes(e, Dependencies{Finance: appfinance.NewService(postgresfinance.NewRepository(pool, schema))})

	body := strings.NewReader(`{"date":"2026-05-02","category":"样品费","amount":120,"allocation":"period_expense","order_id":256,"customer_id":19}`)
	req := httptest.NewRequest(http.MethodPost, "/api/finance/expenses", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "finance dimension customer does not match order") {
		t.Fatalf("mismatched order/customer status=%d body=%s, want 400 mismatch", rec.Code, rec.Body.String())
	}

	var expenseCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*)::int FROM %s.finance_expenses`, schema)).Scan(&expenseCount); err != nil {
		t.Fatalf("query mismatched dimension expense count: %v", err)
	}
	if expenseCount != 0 {
		t.Fatalf("mismatched dimension expense rows = %d, want 0", expenseCount)
	}

	body = strings.NewReader(`{"date":"2026-05-02","category":"样品费","amount":120,"allocation":"period_expense","order_id":256,"customer_id":18}`)
	req = httptest.NewRequest(http.MethodPost, "/api/finance/expenses", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"order_id":256`) || !strings.Contains(rec.Body.String(), `"customer_id":18`) {
		t.Fatalf("matching order/customer expense status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestFinanceExpenseAPIRejectsOrderProductMismatchWithoutWritingExpense(t *testing.T) {
	pool, schema := newFinanceAPIPostgresTestDB(t)
	ctx := context.Background()
	mustExecFinanceAPISQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.customers(id, name, company_name) VALUES (18, '订单归属客户', '订单归属客户公司');
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, total_amount, grand_total, is_void) VALUES (256, 'SO-PRODUCT-MATCH', '2026-05-02', 18, 120, 120, false);
		INSERT INTO %s.products(id, name, active) VALUES (9, '订单商品', true), (10, '非订单商品', true);
		INSERT INTO %s.order_items(order_id, line_no, product_id, item_name, qty, unit, spec, unit_price, line_total) VALUES (256, 1, 9, '订单商品', 1, '件', '454g', 120, 120);
	`, schema, schema, schema, schema))
	e := echo.New()
	RegisterRoutes(e, Dependencies{Finance: appfinance.NewService(postgresfinance.NewRepository(pool, schema))})

	body := strings.NewReader(`{"date":"2026-05-02","category":"样品费","amount":120,"allocation":"period_expense","order_id":256,"product_id":10}`)
	req := httptest.NewRequest(http.MethodPost, "/api/finance/expenses", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "finance dimension product does not match order") {
		t.Fatalf("mismatched order/product status=%d body=%s, want 400 mismatch", rec.Code, rec.Body.String())
	}

	var expenseCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*)::int FROM %s.finance_expenses`, schema)).Scan(&expenseCount); err != nil {
		t.Fatalf("query mismatched product dimension expense count: %v", err)
	}
	if expenseCount != 0 {
		t.Fatalf("mismatched product dimension expense rows = %d, want 0", expenseCount)
	}

	body = strings.NewReader(`{"date":"2026-05-02","category":"样品费","amount":120,"allocation":"period_expense","order_id":256,"product_id":9}`)
	req = httptest.NewRequest(http.MethodPost, "/api/finance/expenses", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"order_id":256`) || !strings.Contains(rec.Body.String(), `"product_id":9`) {
		t.Fatalf("matching order/product expense status=%d body=%s", rec.Code, rec.Body.String())
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
	mode                    string
	lastListFilter          appfinance.ExpenseFilter
	lastDraftReportFilter   appfinance.ReportFilter
	lastClosingReviewFilter appfinance.ReportFilter
	lastDrilldownFilter     appfinance.ReportFilter
	taxLedgerErr            error
}

type fakeFinanceCustomerAccounts struct {
	overview       customerfulfillmentapp.CustomerPortalOverview
	err            error
	lastEmployeeID int64
}

func (s *fakeFinanceCustomerAccounts) CustomerPortalOverview(ctx context.Context, employeeID int64) (customerfulfillmentapp.CustomerPortalOverview, error) {
	s.lastEmployeeID = employeeID
	if s.err != nil {
		return customerfulfillmentapp.CustomerPortalOverview{}, s.err
	}
	return s.overview, nil
}

type fakeFinanceAuthzService struct {
	actor authzapp.Actor
}

func (s *fakeFinanceAuthzService) ActorByEmployeeID(ctx context.Context, employeeID int64) (authzapp.Actor, error) {
	s.actor.EmployeeID = employeeID
	return s.actor, nil
}

func (s *fakeFinanceAuthzService) ListRoles(ctx context.Context) ([]authzapp.Role, error) {
	return nil, nil
}

func (s *fakeFinanceAuthzService) AssignEmployeeRoles(ctx context.Context, cmd authzapp.AssignmentCommand) error {
	return nil
}

func (s *fakeFinanceAuthzService) ListEmployeeRoles(ctx context.Context) (map[int64][]string, error) {
	return nil, nil
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
	return appfinance.Expense{ID: 1, Date: cmd.Date, Month: "2026-05", Category: cmd.Category, Amount: cmd.Amount, Allocation: cmd.Allocation, EmployeeID: cmd.EmployeeID, EmployeeName: "小王", Payment: cmd.Payment, OrderID: cmd.OrderID, CustomerID: cmd.CustomerID, ProductID: cmd.ProductID, BatchNo: cmd.BatchNo, DimensionNote: cmd.DimensionNote}, nil
}

func (s *fakeFinanceService) ListExpenses(_ context.Context, filter appfinance.ExpenseFilter) ([]appfinance.Expense, error) {
	s.lastListFilter = filter
	return []appfinance.Expense{{ID: 1, Date: "2026-05-02", Month: "2026-05", Category: "房租", Amount: 3800, Allocation: appfinance.AllocationPeriodExpense, EmployeeID: 7, EmployeeName: "小王"}}, nil
}

func (s *fakeFinanceService) DraftReport(_ context.Context, filter appfinance.ReportFilter) (domain.MonthlyReport, error) {
	s.lastDraftReportFilter = filter
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

func (s *fakeFinanceService) ClosingReview(_ context.Context, filter appfinance.ReportFilter) (appfinance.ClosingReview, error) {
	s.lastClosingReviewFilter = filter
	return appfinance.ClosingReview{Month: "2026-05", Items: []appfinance.ClosingCheckItem{{Code: "source_exceptions", Status: "warn", Message: "有未处理异常"}}}, nil
}

func (s *fakeFinanceService) ReportDrilldown(_ context.Context, filter appfinance.ReportFilter) (appfinance.ReportDrilldown, error) {
	s.lastDrilldownFilter = filter
	return appfinance.ReportDrilldown{Month: "2026-05", Sections: []appfinance.DrilldownSection{{Section: "revenue", Title: "收入", Total: 103000, Rows: []appfinance.SourceDetail{{Section: "revenue", SourceType: "order", SourceID: 256, Name: "SO-20260502-0001", Amount: 103000}}}}}, nil
}

func (s *fakeFinanceService) ListTaxLedger(context.Context, string) ([]appfinance.TaxLedgerEntry, error) {
	return []appfinance.TaxLedgerEntry{{ID: 1, Month: "2026-05", Kind: "sales_invoice", InvoiceNo: "INV-001", Counterparty: "咖啡客户A", TotalAmount: 1030, TaxAmount: 30, Status: "confirmed"}}, nil
}

func (s *fakeFinanceService) CreateTaxLedgerEntry(_ context.Context, cmd appfinance.CreateTaxLedgerCommand) (appfinance.TaxLedgerEntry, error) {
	if s.taxLedgerErr != nil {
		return appfinance.TaxLedgerEntry{}, s.taxLedgerErr
	}
	return appfinance.TaxLedgerEntry{ID: 2, Month: cmd.Month, Kind: cmd.Kind, InvoiceNo: cmd.InvoiceNo, Counterparty: cmd.Counterparty, TotalAmount: cmd.TotalAmount, TaxAmount: cmd.TaxAmount, Status: cmd.Status}, nil
}

func (s *fakeFinanceService) AccountantHandoff(context.Context, appfinance.ReportFilter) (appfinance.AccountantHandoff, error) {
	return appfinance.AccountantHandoff{Month: "2026-05", VoucherDrafts: []appfinance.VoucherDraft{{Summary: "收入结转", Debit: "应收账款", Credit: "主营业务收入", Amount: 1000}}}, nil
}

func decodeJSON(t *testing.T, body string, out any) {
	t.Helper()
	if err := json.Unmarshal([]byte(body), out); err != nil {
		t.Fatalf("invalid json: %v body=%s", err, body)
	}
}

func newFinanceAPIPostgresTestDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for finance API postgres tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("test_finance_api_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	})
	mustExecFinanceAPISQL(t, ctx, pool, financeAPITestDDL(schema))
	if err := postgresfinance.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("finance EnsureSchema: %v", err)
	}
	return pool, schema
}

func financeAPITestDDL(schema string) string {
	return fmt.Sprintf(`
CREATE TABLE %s.customers (
	id BIGINT PRIMARY KEY,
	name TEXT NOT NULL DEFAULT '',
	company_name TEXT NOT NULL DEFAULT ''
);
CREATE TABLE %s.orders (
	id BIGINT PRIMARY KEY,
	order_no TEXT NOT NULL DEFAULT '',
	order_date DATE NOT NULL,
	customer_id BIGINT NOT NULL DEFAULT 0,
	total_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
	shipping_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
	discount_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
	payment_method TEXT NOT NULL DEFAULT '',
	grand_total NUMERIC(12,2) NOT NULL DEFAULT 0,
	is_void BOOLEAN NOT NULL DEFAULT false
);
CREATE TABLE %s.order_items (
	id BIGSERIAL PRIMARY KEY,
	order_id BIGINT REFERENCES %s.orders(id) ON DELETE CASCADE,
	line_no INTEGER NOT NULL DEFAULT 0,
	product_id BIGINT,
	item_name TEXT NOT NULL DEFAULT '',
	qty NUMERIC NOT NULL DEFAULT 0,
	unit TEXT NOT NULL DEFAULT '',
	spec TEXT NOT NULL DEFAULT '',
	unit_price NUMERIC NOT NULL DEFAULT 0,
	line_total NUMERIC NOT NULL DEFAULT 0
);
CREATE TABLE %s.production_batch_costs (
	id BIGSERIAL PRIMARY KEY,
	product_name TEXT NOT NULL DEFAULT '',
	total_cost NUMERIC(12,2) NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.products (
	id BIGINT PRIMARY KEY,
	name TEXT NOT NULL DEFAULT '',
	active BOOLEAN NOT NULL DEFAULT true
);
CREATE TABLE %s.company_employees (
	id BIGINT PRIMARY KEY,
	name TEXT NOT NULL DEFAULT '',
	active BOOLEAN NOT NULL DEFAULT true
);
CREATE TABLE %s.audit_logs (
	id BIGSERIAL PRIMARY KEY,
	actor TEXT NOT NULL DEFAULT '',
	entity_type TEXT NOT NULL DEFAULT '',
	entity_id BIGINT NULL,
	action TEXT NOT NULL DEFAULT '',
	field TEXT NULL,
	old_value TEXT NULL,
	new_value TEXT NULL,
	meta JSONB NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
`, schema, schema, schema, schema, schema, schema, schema, schema)
}

func mustExecFinanceAPISQL(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string) {
	t.Helper()
	if _, err := pool.Exec(ctx, sql); err != nil {
		t.Fatalf("exec sql: %v\n%s", err, sql)
	}
}
