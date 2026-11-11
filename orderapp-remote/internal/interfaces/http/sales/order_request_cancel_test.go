package sales

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	authzapp "orderapp/internal/application/authz"
	app "orderapp/internal/application/sales"
	"orderapp/internal/interfaces/http/support"
)

type captureCancelOrderRepo struct {
	capturingSaveOrderRepo
	cmd app.OrderRequestCancelCommand
}

func TestPendingOrderStatusThroughAuthorizationMiddleware(t *testing.T) {
	for _, tc := range []struct {
		name        string
		permissions []string
		account     string
		want        int
	}{
		{"writer", []string{"orders.write"}, "employee", 200},
		{"reader", []string{"orders.read"}, "employee", 403},
		{"none", nil, "employee", 403},
		{"channel", []string{"orders.write"}, support.AccountTypeChannelCustomer, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &captureCancelOrderRepo{}
			authz := &orderAPIAuthzService{actor: authzapp.Actor{Permissions: tc.permissions, AccountType: tc.account}}
			e := echo.New()
			e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
				return func(c echo.Context) error { c.Set("employee_id", int64(11)); return next(c) }
			})
			e.Use(support.AuthorizationMiddleware(authz))
			registerOrderAPI(e, app.NewService(repo), nil, authz)
			e.GET("/api/order/private", func(c echo.Context) error { return c.String(200, "sensitive") })
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/order/pending-request/employee-order-request-12345/status", nil))
			if rec.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, tc.want, rec.Body.String())
			}
			if tc.want == 200 && strings.TrimSpace(rec.Body.String()) != `{"cancelled":true}` {
				t.Fatalf("unexpected data: %s", rec.Body.String())
			}
			if tc.name == "writer" {
				private := httptest.NewRecorder()
				e.ServeHTTP(private, httptest.NewRequest(http.MethodGet, "/api/order/private", nil))
				if private.Code != 403 {
					t.Fatalf("writer gained private read: %d", private.Code)
				}
			}
		})
	}
}

func (r *captureCancelOrderRepo) OrderRequestCancelled(context.Context, string) (bool, error) {
	return true, nil
}

func (r *captureCancelOrderRepo) CancelUncommittedOrderRequest(_ context.Context, cmd app.OrderRequestCancelCommand) (app.OrderRequestResolution, error) {
	r.cmd = cmd
	return app.OrderRequestResolution{Status: "cancelled"}, nil
}

func TestResolvePendingOrderRequestRequiresAdminAndBindsOriginal(t *testing.T) {
	repo := &captureCancelOrderRepo{}
	body := `{"original":{"customer_id":3,"request_id":"receipt-order-request-123","prepayment_amount":"20"},"reason":"查无订单"}`
	for _, admin := range []bool{false, true} {
		e := echo.New()
		e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				if admin {
					c.Set("basic_auth_admin", true)
				}
				c.Set("operator_employee", "tester")
				return next(c)
			}
		})
		registerOrderAPI(e, app.NewService(repo), nil)
		req := httptest.NewRequest(http.MethodPost, "/api/order/pending-request/resolve", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if !admin {
			if rec.Code != 403 {
				t.Fatalf("non-admin %d %s", rec.Code, rec.Body.String())
			}
			continue
		}
		var original struct {
			Original orderSaveAPIRequest `json:"original"`
		}
		if err := json.Unmarshal([]byte(body), &original); err != nil {
			t.Fatal(err)
		}
		if rec.Code != 200 || repo.cmd.RequestID != "receipt-order-request-123" || repo.cmd.RequestActor != "tester" || repo.cmd.Actor != "tester" || repo.cmd.RequestHash != confirmationRequestHash(original.Original) || repo.cmd.Reason != "查无订单" {
			t.Fatalf("admin %d cmd=%+v %s", rec.Code, repo.cmd, rec.Body.String())
		}
	}
}

func TestAdminCanResolveEmployeeRequestByIDAndOnlyInternalAccountsCheckStatus(t *testing.T) {
	repo := &captureCancelOrderRepo{}
	for _, admin := range []bool{false, true} {
		e := echo.New()
		e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				if admin {
					c.Set("basic_auth_admin", true)
				}
				c.Set("operator_employee", "admin-test")
				return next(c)
			}
		})
		registerOrderAPI(e, app.NewService(repo), nil)
		request := httptest.NewRequest(http.MethodPost, "/api/order/pending-request/resolve", strings.NewReader(`{"request_id":"employee-order-request-12345","reason":"核对"}`))
		request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		recorder := httptest.NewRecorder()
		e.ServeHTTP(recorder, request)
		want := 403
		if admin {
			want = 200
		}
		if recorder.Code != want {
			t.Fatalf("admin=%v status=%d %s", admin, recorder.Code, recorder.Body.String())
		}
		if admin && (repo.cmd.RequestID != "employee-order-request-12345" || repo.cmd.Actor != "admin-test" || repo.cmd.RequestActor != "unknown") {
			t.Fatalf("command=%+v", repo.cmd)
		}
		statusReq := httptest.NewRequest(http.MethodGet, "/api/order/pending-request/employee-order-request-12345/status", nil)
		statusRec := httptest.NewRecorder()
		e.ServeHTTP(statusRec, statusReq)
		if statusRec.Code != want || (admin && !strings.Contains(statusRec.Body.String(), `"cancelled":true`)) {
			t.Fatalf("admin=%v status=%d %s", admin, statusRec.Code, statusRec.Body.String())
		}
	}
}
