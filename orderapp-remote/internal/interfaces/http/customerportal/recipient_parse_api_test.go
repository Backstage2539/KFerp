package customerportal

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	authzapp "orderapp/internal/application/authz"
	customerapp "orderapp/internal/application/customer"
	customerportalapp "orderapp/internal/application/customerportal"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func recipientParseMiniEmployee(permissions ...string) customerportalapp.CurrentContext {
	return customerportalapp.CurrentContext{
		AccountType:  "employee",
		EmployeeID:   7,
		EmployeeName: "销售甲",
		Roles:        []string{"sales"},
		Permissions:  permissions,
	}
}

func recipientParseMiniCustomer(capabilities ...string) customerportalapp.CurrentContext {
	result := customerportalapp.CurrentContext{
		AccountType:       "customer",
		CurrentCustomerID: 8,
	}
	for _, code := range capabilities {
		result.Capabilities = append(result.Capabilities, customerportalapp.Capability{Code: code, Enabled: true})
	}
	return result
}

type recipientParseAuthz struct {
	actor authzapp.Actor
	err   error
}

func (f recipientParseAuthz) ActorByEmployeeID(context.Context, int64) (authzapp.Actor, error) {
	return f.actor, f.err
}

func (recipientParseAuthz) ListRoles(context.Context) ([]authzapp.Role, error) {
	return nil, nil
}

func (recipientParseAuthz) ListEmployeeRoles(context.Context) (map[int64][]string, error) {
	return nil, nil
}

func (recipientParseAuthz) AssignEmployeeRoles(context.Context, authzapp.AssignmentCommand) error {
	return nil
}

func TestRecipientParseAPIUsesOneContractForERPAndMiniEmployeeTokens(t *testing.T) {
	for _, tc := range []struct {
		name     string
		portal   Service
		authz    recipientParseAuthz
		erpActor bool
	}{
		{
			name:     "ERP employee session",
			authz:    recipientParseAuthz{actor: authzapp.Actor{EmployeeID: 7, Permissions: []string{"customers.read"}}},
			erpActor: true,
		},
		{
			name:   "mini employee session",
			portal: fakeService{me: recipientParseMiniEmployee("customers.read")},
		},
		{
			name:   "mini customer direct ship session",
			portal: fakeService{me: recipientParseMiniCustomer(customerportalapp.CapabilityDirectShip)},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			if tc.erpActor {
				e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
					return func(c echo.Context) error {
						c.Set("employee_id", int64(7))
						return next(c)
					}
				})
			}
			registerRecipientParseAPI(e, tc.portal, tc.authz)

			req := httptest.NewRequest(http.MethodPost, "/api/customer-recipient/parse", strings.NewReader(`{"text":"张三 13800138000 云南省普洱市思茅区咖啡路 88 号"}`))
			req.Header.Set(echo.HeaderAuthorization, "Bearer token")
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			for _, want := range []string{`"recipient_name":"张三"`, `"phone":"13800138000"`, `"address":"云南省普洱市思茅区咖啡路 88 号"`, `"province":"云南省"`, `"city":"普洱市"`, `"district":"思茅区"`, `"detail_address":"咖啡路 88 号"`} {
				if !strings.Contains(rec.Body.String(), want) {
					t.Fatalf("body=%s missing %s", rec.Body.String(), want)
				}
			}
			if strings.Contains(rec.Body.String(), `"text"`) {
				t.Fatalf("response must not echo request text: %s", rec.Body.String())
			}
		})
	}
}

func TestRecipientParseAPIRequiresCustomerReadPermissionForBothSessionTypes(t *testing.T) {
	tests := []struct {
		name     string
		portal   Service
		authz    recipientParseAuthz
		erpActor bool
		want     int
	}{
		{name: "no token", portal: fakeService{}, want: http.StatusUnauthorized},
		{name: "ERP missing read", authz: recipientParseAuthz{actor: authzapp.Actor{EmployeeID: 7, Permissions: []string{"customers.write"}}}, erpActor: true, want: http.StatusForbidden},
		{name: "mini missing read", portal: fakeService{me: recipientParseMiniEmployee("customers.write")}, want: http.StatusForbidden},
		{name: "mini customer missing fulfillment capability", portal: fakeService{me: recipientParseMiniCustomer(customerportalapp.CapabilitySettlement)}, want: http.StatusForbidden},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			if tc.erpActor {
				e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
					return func(c echo.Context) error {
						c.Set("employee_id", int64(7))
						return next(c)
					}
				})
			}
			registerRecipientParseAPI(e, tc.portal, tc.authz)
			req := httptest.NewRequest(http.MethodPost, "/api/customer-recipient/parse", strings.NewReader(`{"text":"张三 13800138000 云南省昆明市"}`))
			if tc.name != "no token" {
				req.Header.Set(echo.HeaderAuthorization, "Bearer token")
			}
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func TestRecipientParseAPIRejectsEmptyAndOverlongWithoutEchoingRawText(t *testing.T) {
	portal := fakeService{me: recipientParseMiniEmployee("customers.read")}
	for _, tc := range []struct {
		name string
		text string
	}{
		{name: "empty", text: "  "},
		{name: "overlong", text: "SENSITIVE-RAW-" + strings.Repeat("收", customerapp.MaxRecipientTextRunes+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			registerRecipientParseAPI(e, portal, nil)
			body := `{"text":"` + tc.text + `"}`
			req := httptest.NewRequest(http.MethodPost, "/api/customer-recipient/parse", strings.NewReader(body))
			req.Header.Set(echo.HeaderAuthorization, "Bearer token")
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), tc.text) || strings.Contains(rec.Body.String(), "SENSITIVE-RAW") {
				t.Fatalf("error response echoed raw input: %s", rec.Body.String())
			}
		})
	}
}

func TestRecipientParseAPIMasksAuthorizationServiceErrors(t *testing.T) {
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(7))
			return next(c)
		}
	})
	registerRecipientParseAPI(e, nil, recipientParseAuthz{err: errors.New("secret auth database detail")})
	req := httptest.NewRequest(http.MethodPost, "/api/customer-recipient/parse", strings.NewReader(`{"text":"张三 13800138000 云南省昆明市"}`))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || strings.Contains(rec.Body.String(), "secret auth database detail") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
