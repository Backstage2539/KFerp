package finance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	authzapp "orderapp/internal/application/authz"
	appfinance "orderapp/internal/application/finance"
	"orderapp/internal/interfaces/http/support"
)

type dailyTestService struct {
	*fakeFinanceService
	filter appfinance.DailyFilter
}

func TestDailyFinanceEndpointsDenyChannelCustomer(t *testing.T) {
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { c.Set("employee_id", int64(7)); return next(c) }
	})
	e.Use(support.AuthorizationMiddleware(&fakeFinanceAuthzService{actor: authzapp.Actor{AccountType: support.AccountTypeChannelCustomer, Permissions: []string{"finance.read", "customer_processing.read"}}}))
	svc := &dailyTestService{fakeFinanceService: &fakeFinanceService{}}
	RegisterRoutes(e, Dependencies{Finance: svc})
	for _, path := range []string{"/api/finance/dashboard?month=2026-05", "/api/finance/daily-ledger?month=2026-05&customer_id=18"} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != 403 {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
	}
	if svc.filter.CustomerID != 0 {
		t.Fatal("customer request reached internal ledger")
	}
}

func (s *dailyTestService) DailyLedger(_ context.Context, f appfinance.DailyFilter) (appfinance.DailyLedgerResult, error) {
	s.filter = f
	return appfinance.DailyLedgerResult{Available: true, Rows: []appfinance.SourceDetail{}, Page: f.Page, PageSize: f.PageSize}, nil
}

func TestDailyLedgerAPIForwardsFiltersAndRejectsBadPagination(t *testing.T) {
	e := echo.New()
	svc := &dailyTestService{fakeFinanceService: &fakeFinanceService{}}
	RegisterRoutes(e, Dependencies{Finance: svc})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/finance/daily-ledger?month=2026-05&customer_id=18&date=2026-05-03&section=main_cost&unavailable=true&page=2&page_size=5", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"available":true`) || svc.filter.CustomerID != 18 || svc.filter.Date != "2026-05-03" || !svc.filter.Unavailable || svc.filter.Page != 2 || svc.filter.PageSize != 5 {
		t.Fatalf("status=%d body=%s filter=%+v", rec.Code, rec.Body.String(), svc.filter)
	}
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/finance/daily-ledger?month=2026-05&page=not-a-number", nil))
	if rec.Code != 400 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
