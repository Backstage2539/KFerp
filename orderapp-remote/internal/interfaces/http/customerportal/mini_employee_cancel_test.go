package customerportal

import (
	"context"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	"strings"
	"testing"
)

type employeeCancelRecorder struct {
	id, employee  int64
	actor, reason string
	all           bool
}

func (s *employeeCancelRecorder) CancelEmployeeOrder(_ context.Context, id, employee int64, all bool, reason, actor string) error {
	s.id, s.employee, s.all, s.reason, s.actor = id, employee, all, reason, actor
	return nil
}

func TestMiniEmployeeCancelOrder(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"sales", `{"reason":"entry_error"}`, 200}, {"admin", `{"reason":"customer_cancel"}`, 200},
		{"anonymous", "{}", 401}, {"customer", "{}", 403}, {"no-permission", "{}", 403}, {"other-order", "{}", 404},
		{"no-reason", "{}", 400}, {"other", `{"reason":"other"}`, 400}, {"extra-field", `{"reason":"duplicate","scope":"all"}`, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			portal := employeePortalService()
			if tc.name != "no-permission" {
				portal.me.Permissions = append(portal.me.Permissions, "orders.resolve")
			}
			if tc.name == "customer" {
				portal.me.AccountType = "customer"
			}
			if tc.name == "admin" {
				portal.me.Roles = []string{"admin"}
			}
			sales := miniEmployeeStatusFixture()
			if tc.name == "other-order" {
				sales.listResult.Rows = nil
			}
			recorder := &employeeCancelRecorder{}
			e := echo.New()
			registerMiniEmployeeAPI(e, portal, sales, Dependencies{EmployeeOrderCancellation: recorder})
			req := httptest.NewRequest("POST", "/api/mini/employee/orders/42/cancel", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			if tc.name != "anonymous" {
				req.Header.Set("Authorization", "Bearer token")
			}
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if tc.status == 200 {
				if recorder.id != 42 || recorder.employee != 7 || recorder.all != (tc.name == "admin") || recorder.actor != "mini-employee:7:销售甲" || !strings.Contains(tc.body, recorder.reason) || recorder.reason == "" {
					t.Fatalf("command=%+v", recorder)
				}
			} else if recorder.id != 0 {
				t.Fatalf("unexpected cancel: %+v", recorder)
			}
		})
	}
}
