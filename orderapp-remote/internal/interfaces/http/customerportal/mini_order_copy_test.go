package customerportal

import (
	"context"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	portalapp "orderapp/internal/application/customerportal"
	salesapp "orderapp/internal/application/sales"
	"testing"
)

type miniOrderCopySales struct {
	miniEmployeeSalesFake
	calls      int
	customerID int64
}

func (s *miniOrderCopySales) CopyOrderDetails(_ context.Context, id int64) (salesapp.OrderDetailsText, error) {
	s.calls++
	return salesapp.OrderDetailsText{OrderID: id, CustomerID: s.customerID, Text: "订单号：SO-42\n付款状态：无需收款"}, nil
}
func TestMiniOrderCopyDetailsEmployeeScope(t *testing.T) {
	for _, allow := range []bool{true, false} {
		s := &miniOrderCopySales{}
		s.listResult = &salesapp.OrderListResult{}
		if allow {
			s.listResult.Rows = []salesapp.OrderRow{{ID: 42}}
		}
		e := echo.New()
		registerMiniEmployeeAPI(e, employeePortalService(), s)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/mini/employee/orders/42/copy-details", nil)
		req.Header.Set("Authorization", "Bearer token")
		e.ServeHTTP(rec, req)
		if allow {
			if rec.Code != 200 || s.calls != 1 || s.listQuery.Scope != "mine" || s.listQuery.EmployeeID != 7 {
				t.Fatalf("%d %s %+v", rec.Code, rec.Body, s.listQuery)
			}
		} else if rec.Code != 404 || s.calls != 0 {
			t.Fatalf("denial %d calls %d", rec.Code, s.calls)
		}
	}
	e := echo.New()
	s := &miniOrderCopySales{}
	p := employeePortalService()
	p.me.Permissions = nil
	registerMiniEmployeeAPI(e, p, s)
	r := httptest.NewRecorder()
	q := httptest.NewRequest("GET", "/api/mini/employee/orders/42/copy-details", nil)
	q.Header.Set("Authorization", "Bearer token")
	e.ServeHTTP(r, q)
	if r.Code != 403 || s.calls != 0 {
		t.Fatal(r.Code, s.calls)
	}
}
func TestMiniOrderCopyDetailsCustomerScope(t *testing.T) {
	for _, tc := range []struct {
		name, token string
		customer    int64
		capability  bool
		accessError error
		want        int
	}{{"own", "token", 8, true, nil, 200}, {"different customer", "token", 9, true, nil, 404}, {"no capability", "token", 8, false, nil, 403}, {"expired", "token", 8, true, portalapp.ErrMiniSessionNotFound, 401}, {"anonymous", "", 8, true, nil, 401}} {
		t.Run(tc.name, func(t *testing.T) {
			var checked int64
			s := &miniOrderCopySales{customerID: 8}
			p := fakeService{me: portalapp.CurrentContext{CurrentCustomerID: tc.customer}, orderAccessID: &checked, err: tc.accessError}
			if tc.capability {
				p.me.Capabilities = []portalapp.Capability{{Code: "mall", Enabled: true}}
			}
			e := echo.New()
			registerMiniOrderCopyAPI(e, p, s)
			r := httptest.NewRecorder()
			q := httptest.NewRequest("GET", "/api/mini/orders/42/copy-details", nil)
			if tc.token != "" {
				q.Header.Set("Authorization", "Bearer "+tc.token)
			}
			e.ServeHTTP(r, q)
			if r.Code != tc.want {
				t.Fatalf("%d %s", r.Code, r.Body)
			}
			if tc.want == 200 && (checked != 42 || s.calls != 1) {
				t.Fatal(checked, s.calls)
			}
			if tc.want == 401 || tc.name == "no capability" {
				if s.calls != 0 {
					t.Fatal("unauthorized read")
				}
			}
		})
	}
}

func TestMiniOrderCopyDetailsShippingQueryAndAdministrator(t *testing.T) {
	s := &miniOrderCopySales{customerID: 8}
	e := echo.New()
	registerMiniOrderCopyAPI(e, fakeService{me: portalapp.CurrentContext{CurrentCustomerID: 8, Capabilities: []portalapp.Capability{{Code: portalapp.CapabilityShippingQuery, Enabled: true}}}}, s)
	r := httptest.NewRecorder()
	q := httptest.NewRequest("GET", "/api/mini/orders/42/copy-details", nil)
	q.Header.Set("Authorization", "Bearer token")
	e.ServeHTTP(r, q)
	if r.Code != 200 {
		t.Fatal(r.Code, r.Body)
	}
	p := employeePortalService()
	p.me.Roles = []string{"admin"}
	s.listResult = &salesapp.OrderListResult{Rows: []salesapp.OrderRow{{ID: 42}}}
	e = echo.New()
	registerMiniEmployeeAPI(e, p, s)
	r = httptest.NewRecorder()
	q = httptest.NewRequest("GET", "/api/mini/employee/orders/42/copy-details", nil)
	q.Header.Set("Authorization", "Bearer token")
	e.ServeHTTP(r, q)
	if r.Code != 200 || s.listQuery.Scope != "" {
		t.Fatal(r.Code, s.listQuery)
	}
}
