package support

import (
	"bytes"
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
	postgresauthz "orderapp/internal/infrastructure/postgres/authz"
	postgrescompany "orderapp/internal/infrastructure/postgres/company"
	postgrescore "orderapp/internal/infrastructure/postgres/core"
	postgrescosting "orderapp/internal/infrastructure/postgres/costing"
	postgrescustomerfulfillment "orderapp/internal/infrastructure/postgres/customerfulfillment"
	postgrescustomerportal "orderapp/internal/infrastructure/postgres/customerportal"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func TestERPPasswordLoginRequiresWorkbenchEligibilityForChannelCustomers(t *testing.T) {
	pool, schema := newERPLoginGateTestDB(t)
	ctx := context.Background()
	eligibility := customerfulfillmentapp.NewService(postgrescustomerfulfillment.NewRepository(pool, schema))

	tests := []struct {
		name          string
		accountType   string
		templateKey   *string
		portalEnabled bool
		wantStatus    int
	}{
		{name: "internal employee", accountType: AccountTypeInternalEmployee, wantStatus: http.StatusOK},
		{name: "workbench channel customer", accountType: AccountTypeChannelCustomer, templateKey: stringPointerForERPLoginTest("processing_fulfillment"), portalEnabled: true, wantStatus: http.StatusOK},
		{name: "disabled workbench portal", accountType: AccountTypeChannelCustomer, templateKey: stringPointerForERPLoginTest("processing_fulfillment"), portalEnabled: false, wantStatus: http.StatusForbidden},
		{name: "retail channel customer", accountType: AccountTypeChannelCustomer, templateKey: stringPointerForERPLoginTest("retail_mall"), portalEnabled: true, wantStatus: http.StatusForbidden},
		{name: "empty template channel customer", accountType: AccountTypeChannelCustomer, templateKey: stringPointerForERPLoginTest(""), portalEnabled: true, wantStatus: http.StatusForbidden},
		{name: "unknown template channel customer", accountType: AccountTypeChannelCustomer, templateKey: stringPointerForERPLoginTest("legacy_unknown_template"), portalEnabled: true, wantStatus: http.StatusForbidden},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			phone := fmt.Sprintf("13910000%03d", i+1)
			employeeID := seedERPLoginGateAccount(t, ctx, pool, schema, tc.name, phone, tc.accountType, tc.templateKey, tc.portalEnabled)
			before := countERPLoginSessions(t, ctx, pool, schema, employeeID)

			e := echo.New()
			registerMobileAuthAPI(e, pool, schema, nil, eligibility)
			rec := serveERPLoginRequest(t, e, map[string]any{
				"mode":     "password",
				"phone":    phone,
				"password": "secret123",
			})
			if rec.Code != tc.wantStatus {
				t.Fatalf("status=%d body=%s, want %d", rec.Code, rec.Body.String(), tc.wantStatus)
			}
			after := countERPLoginSessions(t, ctx, pool, schema, employeeID)
			if tc.wantStatus == http.StatusOK {
				if after != before+1 || !strings.Contains(rec.Body.String(), `"token"`) {
					t.Fatalf("successful login sessions=%d before=%d body=%s", after, before, rec.Body.String())
				}
				return
			}
			if after != before || strings.Contains(rec.Body.String(), `"token"`) {
				t.Fatalf("rejected login created session or token: before=%d after=%d body=%s", before, after, rec.Body.String())
			}
		})
	}
}

func TestERPSMSLoginOnlyAllowsExistingActiveInternalEmployees(t *testing.T) {
	pool, schema := newERPLoginGateTestDB(t)
	ctx := context.Background()
	eligibility := customerfulfillmentapp.NewService(postgrescustomerfulfillment.NewRepository(pool, schema))

	tests := []struct {
		name          string
		accountType   string
		templateKey   *string
		portalEnabled bool
		wantStatus    int
	}{
		{name: "internal employee", accountType: AccountTypeInternalEmployee, wantStatus: http.StatusOK},
		{name: "workbench channel customer", accountType: AccountTypeChannelCustomer, templateKey: stringPointerForERPLoginTest("processing_fulfillment"), portalEnabled: true, wantStatus: http.StatusUnauthorized},
		{name: "disabled workbench portal", accountType: AccountTypeChannelCustomer, templateKey: stringPointerForERPLoginTest("processing_fulfillment"), portalEnabled: false, wantStatus: http.StatusUnauthorized},
		{name: "retail channel customer", accountType: AccountTypeChannelCustomer, templateKey: stringPointerForERPLoginTest("retail_mall"), portalEnabled: true, wantStatus: http.StatusUnauthorized},
		{name: "empty template channel customer", accountType: AccountTypeChannelCustomer, templateKey: stringPointerForERPLoginTest(""), portalEnabled: true, wantStatus: http.StatusUnauthorized},
		{name: "unknown template channel customer", accountType: AccountTypeChannelCustomer, templateKey: stringPointerForERPLoginTest("legacy_unknown_template"), portalEnabled: true, wantStatus: http.StatusUnauthorized},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			phone := fmt.Sprintf("13920000%03d", i+1)
			employeeID := seedERPLoginGateAccount(t, ctx, pool, schema, tc.name, phone, tc.accountType, tc.templateKey, tc.portalEnabled)
			code := fmt.Sprintf("71%04d", i+1)
			mustExecERPLoginGateSQL(t, ctx, pool, fmt.Sprintf(`
				INSERT INTO %s.login_sms_codes(phone,code,expire_at,used)
				VALUES($1,$2,now()+interval '5 minutes',false)
			`, schema), phone, code)
			before := countERPLoginSessions(t, ctx, pool, schema, employeeID)

			e := echo.New()
			registerMobileAuthAPI(e, pool, schema, nil, eligibility)
			rec := serveERPLoginRequest(t, e, map[string]any{
				"mode":  "sms",
				"phone": phone,
				"code":  code,
			})
			if rec.Code != tc.wantStatus {
				t.Fatalf("status=%d body=%s, want %d", rec.Code, rec.Body.String(), tc.wantStatus)
			}
			after := countERPLoginSessions(t, ctx, pool, schema, employeeID)
			if tc.wantStatus == http.StatusOK && after != before+1 {
				t.Fatalf("successful SMS login sessions=%d before=%d", after, before)
			}
			if tc.wantStatus != http.StatusOK && after != before {
				t.Fatalf("rejected SMS login created session: before=%d after=%d", before, after)
			}
			var used bool
			if err := pool.QueryRow(ctx, fmt.Sprintf(`
				SELECT used FROM %s.login_sms_codes WHERE phone=$1 AND code=$2
			`, schema), phone, code).Scan(&used); err != nil {
				t.Fatalf("read SMS code: %v", err)
			}
			if tc.wantStatus == http.StatusOK && !used {
				t.Fatal("successful SMS login did not consume its code")
			}
			if tc.wantStatus != http.StatusOK && used {
				t.Fatal("rejected channel customer SMS login consumed its code")
			}
		})
	}
}

func TestExistingERPSessionFailsClosedAfterWorkbenchTemplateDowngrade(t *testing.T) {
	pool, schema := newERPLoginGateTestDB(t)
	ctx := context.Background()
	eligibility := customerfulfillmentapp.NewService(postgrescustomerfulfillment.NewRepository(pool, schema))
	phone := "13930000001"
	employeeID := seedERPLoginGateAccount(t, ctx, pool, schema, "workbench account", phone, AccountTypeChannelCustomer, stringPointerForERPLoginTest("processing_fulfillment"), true)
	token := "legacy-erp-workbench-token"
	mustExecERPLoginGateSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.login_sessions(token,employee_id,expire_at)
		VALUES($1,$2,now()+interval '7 days')
	`, schema), token, employeeID)

	e := echo.New()
	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	request.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	contextBefore := e.NewContext(request, httptest.NewRecorder())
	gotID, _, err := resolveEmployeeBySessionToken(contextBefore, pool, schema, token, eligibility)
	if err != nil || gotID != employeeID {
		t.Fatalf("workbench session before downgrade id=%d err=%v", gotID, err)
	}

	mustExecERPLoginGateSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.customer_portal_profiles
		SET capability_template_key='retail_mall'
		WHERE customer_id=(SELECT customer_id FROM %s.customer_erp_user_bindings WHERE employee_id=$1 AND status='active')
	`, schema, schema), employeeID)

	request = httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	request.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	contextAfter := e.NewContext(request, httptest.NewRecorder())
	gotID, _, err = resolveEmployeeBySessionToken(contextAfter, pool, schema, token, eligibility)
	if err == nil || gotID != 0 {
		t.Fatalf("downgraded session id=%d err=%v, want fail closed", gotID, err)
	}
}

func TestERPWorkbenchEligibilityRejectsDisabledPortalProfile(t *testing.T) {
	pool, schema := newERPLoginGateTestDB(t)
	ctx := context.Background()
	repo := postgrescustomerfulfillment.NewRepository(pool, schema)
	eligibility := customerfulfillmentapp.NewService(repo)
	employeeID := seedERPLoginGateAccount(t, ctx, pool, schema, "disabled portal account", "13930000002", AccountTypeChannelCustomer, stringPointerForERPLoginTest("processing_fulfillment"), false)

	if _, err := repo.CustomerPortalContext(ctx, employeeID); !errors.Is(err, customerfulfillmentapp.ErrCustomerERPBindingNotFound) {
		t.Fatalf("CustomerPortalContext err=%v, want disabled portal rejected", err)
	}
	if err := eligibility.RequireERPWorkbenchLogin(ctx, employeeID); !errors.Is(err, customerfulfillmentapp.ErrCustomerERPBindingNotFound) {
		t.Fatalf("RequireERPWorkbenchLogin err=%v, want disabled portal rejected", err)
	}
}

func TestExistingERPSessionFailsClosedAfterPortalDisabled(t *testing.T) {
	pool, schema := newERPLoginGateTestDB(t)
	ctx := context.Background()
	eligibility := customerfulfillmentapp.NewService(postgrescustomerfulfillment.NewRepository(pool, schema))
	employeeID := seedERPLoginGateAccount(t, ctx, pool, schema, "disabled-after-login account", "13930000003", AccountTypeChannelCustomer, stringPointerForERPLoginTest("processing_fulfillment"), true)
	token := "legacy-erp-disabled-portal-token"
	mustExecERPLoginGateSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.login_sessions(token,employee_id,expire_at)
		VALUES($1,$2,now()+interval '7 days')
	`, schema), token, employeeID)

	e := echo.New()
	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	contextBefore := e.NewContext(request, httptest.NewRecorder())
	gotID, _, err := resolveEmployeeBySessionToken(contextBefore, pool, schema, token, eligibility)
	if err != nil || gotID != employeeID {
		t.Fatalf("enabled portal session id=%d err=%v", gotID, err)
	}

	mustExecERPLoginGateSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.customer_portal_profiles
		SET enabled=false
		WHERE customer_id=(SELECT customer_id FROM %s.customer_erp_user_bindings WHERE employee_id=$1 AND status='active')
	`, schema, schema), employeeID)

	request = httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	contextAfter := e.NewContext(request, httptest.NewRecorder())
	gotID, _, err = resolveEmployeeBySessionToken(contextAfter, pool, schema, token, eligibility)
	if err == nil || gotID != 0 {
		t.Fatalf("disabled portal session id=%d err=%v, want fail closed", gotID, err)
	}
}

func TestProductionAuthMiddlewareRejectsExistingSessionAfterPortalDisabled(t *testing.T) {
	pool, schema := newERPLoginGateTestDB(t)
	ctx := context.Background()
	eligibility := customerfulfillmentapp.NewService(postgrescustomerfulfillment.NewRepository(pool, schema))
	authz := authzapp.NewService(postgresauthz.NewRepository(pool, schema))
	employeeID := seedERPLoginGateAccount(t, ctx, pool, schema, "middleware portal account", "13930000004", AccountTypeChannelCustomer, stringPointerForERPLoginTest("processing_fulfillment"), true)
	token := "middleware-erp-disabled-portal-token"
	mustExecERPLoginGateSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.login_sessions(token,employee_id,expire_at)
		VALUES($1,$2,now()+interval '7 days')
	`, schema), token, employeeID)

	e := echo.New()
	e.Use(BasicAuth("order", "secret", schema, pool, eligibility))
	e.Use(EmployeeContextMiddleware(pool, schema, eligibility))
	registerAuthzAPI(e, authz)
	serveMe := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}

	if rec := serveMe(); rec.Code != http.StatusOK {
		t.Fatalf("enabled portal middleware status=%d body=%s", rec.Code, rec.Body.String())
	}
	mustExecERPLoginGateSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.customer_portal_profiles
		SET enabled=false
		WHERE customer_id=(SELECT customer_id FROM %s.customer_erp_user_bindings WHERE employee_id=$1 AND status='active')
	`, schema, schema), employeeID)
	if rec := serveMe(); rec.Code != http.StatusUnauthorized {
		t.Fatalf("disabled portal middleware status=%d body=%s, want 401", rec.Code, rec.Body.String())
	}
}

func serveERPLoginRequest(t *testing.T, e *echo.Echo, payload map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func seedERPLoginGateAccount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema, name, phone, accountType string, templateKey *string, portalEnabled bool) int64 {
	t.Helper()
	var employeeID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.company_employees(name,phone,account_type,department_id,active)
		VALUES($1,$2,$3,(SELECT id FROM %[1]s.company_departments WHERE active=true ORDER BY id LIMIT 1),true)
		RETURNING id
	`, schema), name, phone, accountType).Scan(&employeeID); err != nil {
		t.Fatalf("insert employee: %v", err)
	}
	mustExecERPLoginGateSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.employee_login_passwords(employee_id,password_hash,login_disabled,must_reset_password)
		VALUES($1,$2,false,false)
	`, schema), employeeID, hashPassword("secret123"))
	if accountType != AccountTypeChannelCustomer {
		return employeeID
	}

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name,customer_type,active)
		VALUES($1,'wholesale',true)
		RETURNING id
	`, schema), name+" customer").Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if templateKey != nil {
		mustExecERPLoginGateSQL(t, ctx, pool, fmt.Sprintf(`
			INSERT INTO %s.customer_portal_profiles(customer_id,enabled,capability_template_key)
			VALUES($1,$2,$3)
		`, schema), customerID, portalEnabled, *templateKey)
	}
	mustExecERPLoginGateSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.customer_erp_user_bindings(customer_id,employee_id,role,status,updated_by)
		VALUES($1,$2,'customer','active','test')
	`, schema), customerID, employeeID)
	return employeeID
}

func countERPLoginSessions(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, employeeID int64) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.login_sessions WHERE employee_id=$1`, schema), employeeID).Scan(&count); err != nil {
		t.Fatalf("count login sessions: %v", err)
	}
	return count
}

func stringPointerForERPLoginTest(value string) *string {
	return &value
}

func newERPLoginGateTestDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for ERP login gate tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	schema := fmt.Sprintf("erp_login_gate_%d", time.Now().UnixNano())
	mustExecERPLoginGateSQL(t, ctx, pool, "CREATE SCHEMA "+schema)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})
	for _, step := range []struct {
		name   string
		ensure func(context.Context, *pgxpool.Pool, string) error
	}{
		{name: "core", ensure: postgrescore.EnsureSchema},
		{name: "company", ensure: postgrescompany.EnsureSchema},
		{name: "authz", ensure: postgresauthz.EnsureSchema},
		{name: "customer portal", ensure: postgrescustomerportal.EnsureSchema},
		{name: "costing", ensure: postgrescosting.EnsureSchema},
		{name: "customer fulfillment", ensure: postgrescustomerfulfillment.EnsureSchema},
	} {
		if err := step.ensure(ctx, pool, schema); err != nil {
			t.Fatalf("%s EnsureSchema: %v", step.name, err)
		}
	}
	if err := EnsureAuditTables(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureAuditTables: %v", err)
	}
	if err := ensureMobileAuthTables(ctx, pool, schema); err != nil {
		t.Fatalf("ensureMobileAuthTables: %v", err)
	}
	return pool, schema
}

func mustExecERPLoginGateSQL(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(ctx, sql, args...); err != nil {
		t.Fatalf("exec SQL failed: %v\n%s", err, sql)
	}
}
