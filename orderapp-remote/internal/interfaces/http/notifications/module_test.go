package notifications

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	portal "orderapp/internal/application/customerportal"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestBusinessVerificationAPIRequiresAdminAndExplicitConfirmation(t *testing.T) {
	e := echo.New()
	h := &Handler{}
	for _, tc := range []struct {
		name, body string
		admin      bool
		want       int
	}{
		{"unauthorized", `{"confirmed_received_and_opened":true}`, false, 403},
		{"not_confirmed", `{"confirmed_received_and_opened":false}`, true, 400},
		{"missing_confirmation", `{}`, true, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/message-center/external/deliveries/1/verify-business-test", strings.NewReader(tc.body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetParamNames("id")
			c.SetParamValues("1")
			if tc.admin {
				c.Set("basic_auth_admin", true)
			}
			err := h.verifyBusinessTest(c)
			he, ok := err.(*echo.HTTPError)
			if !ok || he.Code != tc.want {
				t.Fatalf("got %v, want %d", err, tc.want)
			}
		})
	}
}

func TestOfficialAuthorizationErrorClassification(t *testing.T) {
	transient := errors.New("database temporarily unavailable")
	if err := officialAuthorizationError(transient); !errors.Is(err, transient) {
		t.Fatalf("lost transient error: %v", err)
	}
	if err := officialAuthorizationError(portal.ErrCustomerBindingNotFound); err != nil {
		t.Fatalf("stale binding should skip: %v", err)
	}
}

type detailPortal struct {
	current portal.CurrentContext
	err     error
}

func (p detailPortal) Me(context.Context, string) (portal.CurrentContext, error) {
	return p.current, p.err
}

func TestFinancialDetailAPIAuthorization(t *testing.T) {
	e := echo.New()
	cases := []struct {
		name    string
		current portal.CurrentContext
		want    int
	}{
		{"logged_out", portal.CurrentContext{}, 401},
		{"no_customer", portal.CurrentContext{MiniUserID: 1}, 403},
		{"missing_settlement", portal.CurrentContext{MiniUserID: 1, CurrentCustomerID: 8}, 403},
		{"employee", portal.CurrentContext{MiniUserID: 1, EmployeeID: 5, AccountType: "employee", Capabilities: []portal.Capability{{Code: portal.CapabilitySettlement, Enabled: true}}}, 403},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/mini/notification-details/receipts/1", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetParamNames("id")
			c.SetParamValues("1")
			h := &Handler{Portal: detailPortal{current: tc.current}}
			err := h.receiptDetail(c)
			if rec.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("financial response is cacheable")
			}
			he, ok := err.(*echo.HTTPError)
			if !ok || he.Code != tc.want {
				t.Fatalf("got %v want %d", err, tc.want)
			}
		})
	}
}
