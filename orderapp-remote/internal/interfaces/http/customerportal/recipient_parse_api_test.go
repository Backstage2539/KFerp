package customerportal

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	authzapp "orderapp/internal/application/authz"
	customerapp "orderapp/internal/application/customer"
	customerportalapp "orderapp/internal/application/customerportal"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

type recipientParseReadSpy struct {
	reader io.Reader
	reads  int
}

func (s *recipientParseReadSpy) Read(p []byte) (int, error) {
	s.reads++
	return s.reader.Read(p)
}

func (*recipientParseReadSpy) Close() error { return nil }

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
	want := customerapp.RecipientParseResult{
		RecipientName: "王心星",
		Phone:         "13529003193",
		Address:       "云南省普洱市景谷傣族彝族自治县威远江国际大酒店侧面乾民號茶坊",
		Province:      "云南省",
		City:          "普洱市",
		District:      "景谷傣族彝族自治县",
		DetailAddress: "威远江国际大酒店侧面乾民號茶坊",
	}
	var sharedResult *customerapp.RecipientParseResult
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

			req := httptest.NewRequest(http.MethodPost, "/api/customer-recipient/parse", strings.NewReader(`{"text":"云南省普洱市景谷傣族彝族自治县威远江国际大酒店侧面乾民號茶坊王心星13529003193"}`))
			req.Header.Set(echo.HeaderAuthorization, "Bearer token")
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			var got customerapp.RecipientParseResult
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
			}
			if got != want {
				t.Fatalf("result=%+v, want %+v", got, want)
			}
			if sharedResult == nil {
				copy := got
				sharedResult = &copy
			} else if got != *sharedResult {
				t.Fatalf("result=%+v differs from first session result %+v", got, *sharedResult)
			}
			if strings.Contains(rec.Body.String(), `"text"`) {
				t.Fatalf("response must not echo request text: %s", rec.Body.String())
			}
		})
	}
}

func TestRecipientParseAPIAcceptsPersistedMiniCustomerContextWithoutAccountType(t *testing.T) {
	e := echo.New()
	registerRecipientParseAPI(e, fakeService{me: customerportalapp.CurrentContext{
		CurrentCustomerID: 8,
		Capabilities: []customerportalapp.Capability{{
			Code:    customerportalapp.CapabilityDirectShip,
			Enabled: true,
		}},
	}}, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/customer-recipient/parse", strings.NewReader(`{"text":"张三 13800138000 云南省昆明市"}`))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
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
			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("denied response must be one valid JSON document: body=%q err=%v", rec.Body.String(), err)
			}
		})
	}
}

func TestRecipientParseAPIWithoutAccessStopsBeforeReadingOrParsing(t *testing.T) {
	e := echo.New()
	registerRecipientParseAPI(e, fakeService{}, nil)
	bodyReader := &recipientParseReadSpy{reader: strings.NewReader(`{"text":"张三 13800138000 云南省昆明市"}`)}
	req := httptest.NewRequest(http.MethodPost, "/api/customer-recipient/parse", nil)
	req.Body = bodyReader
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if bodyReader.reads != 0 {
		t.Errorf("unauthorized request body was read %d time(s), so parsing was not short-circuited", bodyReader.reads)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Errorf("response must be one valid JSON document: body=%q err=%v", rec.Body.String(), err)
	} else if body["error"] != "请先登录" {
		t.Errorf("body=%v", body)
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
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("denied response must be one valid JSON document: body=%q err=%v", rec.Body.String(), err)
	}
}
