package customerportal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	customerportalapp "orderapp/internal/application/customerportal"
	salesapp "orderapp/internal/application/sales"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestMiniEmployeeOrderCardsRejectInvalidCursorBeforeListQuery(t *testing.T) {
	for _, raw := range []string{"broken", "2026-02-31~10", "2026-11-01~0"} {
		e := echo.New()
		sales := &miniEmployeeSalesFake{}
		registerMiniEmployeeAPI(e, employeePortalService(), sales)
		req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/orders?lightweight=1&cursor="+raw, nil)
		req.Header.Set("Authorization", "Bearer token")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != 400 || !strings.Contains(rec.Body.String(), "分页") || sales.listQuery.Limit != 0 {
			t.Fatalf("invalid cursor %q got %d body %s legacy query %+v", raw, rec.Code, rec.Body.String(), sales.listQuery)
		}
	}
}

type employeeCardSalesFake struct {
	miniEmployeeSalesFake
	query salesapp.EmployeeOrderCardsQuery
}

func (s *employeeCardSalesFake) ListEmployeeOrderCards(_ context.Context, q salesapp.EmployeeOrderCardsQuery) (salesapp.EmployeeOrderCardsPage, error) {
	s.query = q
	return salesapp.EmployeeOrderCardsPage{Rows: []salesapp.EmployeeOrderCard{{ID: 12, OrderNo: "PAGE-12"}}, Next: &salesapp.EmployeeOrderCursor{Date: "2026-11-02", ID: 12}}, nil
}

func TestMiniEmployeeOrderCardsPreserveScopeAndReturnContinuation(t *testing.T) {
	for _, role := range []string{"sales", "admin"} {
		e := echo.New()
		sales := &employeeCardSalesFake{}
		portal := fakeService{me: customerportalapp.CurrentContext{AccountType: "employee", EmployeeID: 7, Roles: []string{role}, Permissions: []string{"orders.read"}}}
		registerMiniEmployeeAPI(e, portal, sales)
		req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/orders?lightweight=1&q=%20hello%20&cursor=2026-11-05~18", nil)
		req.Header.Set("Authorization", "Bearer token")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		var body struct {
			Rows    []salesapp.EmployeeOrderCard `json:"rows"`
			HasNext bool                         `json:"has_next"`
			Next    string                       `json:"next_cursor"`
		}
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &body) != nil || len(body.Rows) != 1 || body.Rows[0].ID != 12 || !body.HasNext || body.Next != "2026-11-02~12" || strings.Contains(rec.Body.String(), "\"summary\"") {
			t.Fatalf("role %s got %d %s", role, rec.Code, rec.Body.String())
		}
		q := sales.query
		if q.Limit != 4 || q.Q != "hello" || q.After == nil || q.After.ID != 18 || q.After.Date != "2026-11-05" || sales.listQuery.Limit != 0 {
			t.Fatalf("invalid request %+v", q)
		}
		if role == "sales" && (q.Scope != "mine" || q.EmployeeID != 7) || role == "admin" && (q.Scope != "" || q.EmployeeID != 0) {
			t.Fatalf("role %s scope %+v", role, q)
		}
	}
}
