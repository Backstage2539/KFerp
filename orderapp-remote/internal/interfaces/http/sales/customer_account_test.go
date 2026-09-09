package sales

import (
	"context"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	app "orderapp/internal/application/customerfulfillment"
	salesapp "orderapp/internal/application/sales"
	"strings"
	"testing"
)

type accountScopeStub struct {
	customer int64
	finance  bool
}

func (s accountScopeStub) BoundCustomerID(context.Context, int64) (int64, error) {
	return s.customer, nil
}
func (s accountScopeStub) CustomerWorkspace(context.Context, int64, int64, string, int64) (map[string]any, error) {
	codes := []string{"direct_ship"}
	if s.finance {
		codes = append(codes, "settlement")
	}
	return map[string]any{"customer_id": s.customer, "capabilities": codes}, nil
}
func (s accountScopeStub) CustomerAccount(_ context.Context, q app.AccountQuery) (app.AccountData, error) {
	if q.CustomerID != s.customer {
		return app.AccountData{}, fmt.Errorf("wrong customer")
	}
	rows := []app.AccountOrder{}
	if q.OrderID == 0 || q.OrderID == s.customer {
		rows = append(rows, app.AccountOrder{ID: s.customer, OrderNo: "OWN", TotalCents: 10000})
	}
	return app.AccountData{Rows: rows, Summary: app.AccountSummary{TotalCents: 10000}}, nil
}
func TestCustomerAccountAPIScopeAndDocumentDenials(t *testing.T) {
	for _, customer := range []int64{1, 2} {
		e := echo.New()
		e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error { c.Set("employee_id", int64(1)); return next(c) }
		})
		h := orderAPIHandler{sales: salesapp.NewService(nil), customerScope: accountScopeStub{customer: customer, finance: true}}
		registerCustomerAccountRoutes(e, h)
		for _, tc := range []struct {
			method, path, body string
			status             int
		}{
			{"GET", accountPrefix + "/orders", "", 200},
			{"GET", accountPrefix + "/orders?customer_id=999", "", 403},
			{"GET", accountPrefix + "/orders/999/detail", "", 404},
			{"GET", accountPrefix + "/orders/999/sales-order-preview", "", 403},
			{"GET", accountPrefix + "/orders/999/sales-files/1.pdf", "", 403},
			{"GET", accountPrefix + "/orders/combined/sales-order-preview?order_ids=1,2", "", 403},
			{"POST", accountPrefix + "/orders/combined/sales-orders", `{"order_ids":[1,2]}`, 403},
			{"GET", accountPrefix + "/settlements/999", "", 404},
			{"GET", accountPrefix + "/statements?date_from=2026-09-30&date_to=2026-09-01", "", 400},
		} {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("customer %d %s: %d %s", customer, tc.path, rec.Code, rec.Body.String())
			}
		}
	}
}
