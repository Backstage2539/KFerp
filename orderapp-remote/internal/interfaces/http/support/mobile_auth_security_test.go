package support

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authzapp "orderapp/internal/application/authz"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func TestDeprecatedPasswordSetRouteIsNeitherPublicNorRegistered(t *testing.T) {
	if isAuthPublicPath("/api/auth/password/set") {
		t.Fatal("deprecated password/set route must not bypass authentication")
	}

	e := echo.New()
	registerMobileAuthAPI(e, nil, "public", nil)
	for _, route := range e.Routes() {
		if route.Method == http.MethodPost && route.Path == "/api/auth/password/set" {
			t.Fatal("deprecated password/set route must not remain registered")
		}
	}
}

func TestDeprecatedPasswordSetCannotCreateOrChangeAccountsOverHTTP(t *testing.T) {
	pool, schema := newERPLoginGateTestDB(t)
	ctx := context.Background()
	e := newMobileAuthSecurityEcho(pool, schema)

	t.Run("unauthenticated arbitrary phone cannot create an employee", func(t *testing.T) {
		phone := "13981000001"
		rec := postMobileAuthJSON(t, e, "/api/auth/password/set", map[string]any{
			"phone": phone, "password": "attacker-secret",
		}, false)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status=%d body=%s, want 401", rec.Code, rec.Body.String())
		}
		if got := countMobileAuthRows(t, ctx, pool, schema, "company_employees", "phone=$1", phone); got != 0 {
			t.Errorf("employees for arbitrary phone=%d, want 0", got)
		}
		if got := countMobileAuthRows(t, ctx, pool, schema, "employee_login_passwords", "employee_id IN (SELECT id FROM "+schema+".company_employees WHERE phone=$1)", phone); got != 0 {
			t.Errorf("password rows for arbitrary phone=%d, want 0", got)
		}
	})

	t.Run("authenticated request cannot use the unaudited compatibility path", func(t *testing.T) {
		employeeID := seedMobileAuthSecurityEmployee(t, ctx, pool, schema, "密码受保护员工", "13981000002", AccountTypeInternalEmployee, true, true)
		before := readMobileAuthPasswordState(t, ctx, pool, schema, employeeID)
		rec := postMobileAuthJSON(t, e, "/api/auth/password/set", map[string]any{
			"phone": "13981000002", "password": "replacement-secret",
		}, true)
		if rec.Code != http.StatusNotFound {
			t.Errorf("status=%d body=%s, want 404", rec.Code, rec.Body.String())
		}
		after := readMobileAuthPasswordState(t, ctx, pool, schema, employeeID)
		if after != before {
			t.Errorf("password state changed through removed route: before=%+v after=%+v", before, after)
		}
	})
}

func TestSMSSendFailsClosedWithoutSenderAndWritesNothing(t *testing.T) {
	pool, schema := newERPLoginGateTestDB(t)
	ctx := context.Background()
	e := newMobileAuthSecurityEcho(pool, schema)
	phone := "13982000001"

	rec := postMobileAuthJSON(t, e, "/api/auth/sms/send", map[string]any{"phone": phone}, false)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status=%d body=%s, want 503", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, exists := body["code"]; exists || strings.Contains(rec.Body.String(), `"code"`) {
		t.Errorf("SMS send response exposed a verification code: %s", rec.Body.String())
	}
	if body["error"] != "短信服务暂未开通，请使用密码登录" {
		t.Errorf("SMS send error=%v, want actionable Chinese guidance", body["error"])
	}
	if got := countMobileAuthRows(t, ctx, pool, schema, "company_employees", "phone=$1", phone); got != 0 {
		t.Errorf("SMS send created employees=%d, want 0", got)
	}
	if got := countMobileAuthRows(t, ctx, pool, schema, "login_sms_codes", "phone=$1", phone); got != 0 {
		t.Errorf("SMS send inserted codes=%d, want 0", got)
	}
}

func TestSMSLoginNeverCreatesOrReactivatesEmployees(t *testing.T) {
	pool, schema := newERPLoginGateTestDB(t)
	ctx := context.Background()
	e := newMobileAuthSecurityEcho(pool, schema)

	t.Run("unknown phone with a preloaded code is rejected without writes", func(t *testing.T) {
		phone, code := "13983000001", "830001"
		seedMobileAuthSMSCode(t, ctx, pool, schema, phone, code)
		rec := postMobileAuthJSON(t, e, "/api/auth/login", map[string]any{
			"mode": "sms", "phone": phone, "code": code,
		}, false)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status=%d body=%s, want 401", rec.Code, rec.Body.String())
		}
		if got := countMobileAuthRows(t, ctx, pool, schema, "company_employees", "phone=$1", phone); got != 0 {
			t.Errorf("SMS login created employees=%d, want 0", got)
		}
		if got := countMobileAuthRows(t, ctx, pool, schema, "login_sessions", "employee_id IN (SELECT id FROM "+schema+".company_employees WHERE phone=$1)", phone); got != 0 {
			t.Errorf("SMS login created sessions=%d, want 0", got)
		}
		assertLatestMobileAuthSMSCodeUnused(t, ctx, pool, schema, phone)
	})

	t.Run("inactive internal employee stays inactive and code stays unused", func(t *testing.T) {
		phone, code := "13983000002", "830002"
		employeeID := seedMobileAuthSecurityEmployee(t, ctx, pool, schema, "停用员工", phone, AccountTypeInternalEmployee, false, false)
		seedMobileAuthSMSCode(t, ctx, pool, schema, phone, code)
		rec := postMobileAuthJSON(t, e, "/api/auth/login", map[string]any{
			"mode": "sms", "phone": phone, "code": code,
		}, false)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status=%d body=%s, want 401", rec.Code, rec.Body.String())
		}
		var active bool
		if err := pool.QueryRow(ctx, fmt.Sprintf("SELECT active FROM %s.company_employees WHERE id=$1", schema), employeeID).Scan(&active); err != nil {
			t.Fatalf("read employee active: %v", err)
		}
		if active {
			t.Error("SMS login reactivated an inactive employee")
		}
		if got := countERPLoginSessions(t, ctx, pool, schema, employeeID); got != 0 {
			t.Errorf("inactive employee sessions=%d, want 0", got)
		}
		assertLatestMobileAuthSMSCodeUnused(t, ctx, pool, schema, phone)
	})
}

func TestAdminAuthMaintenanceOnlyWritesActiveInternalEmployees(t *testing.T) {
	pool, schema := newERPLoginGateTestDB(t)
	ctx := context.Background()
	e := newMobileAuthSecurityEcho(pool, schema)

	for _, tc := range []struct {
		name        string
		accountType string
		active      bool
	}{
		{name: "channel customer", accountType: AccountTypeChannelCustomer, active: true},
		{name: "inactive internal employee", accountType: AccountTypeInternalEmployee, active: false},
	} {
		t.Run(tc.name+" password reset", func(t *testing.T) {
			phone := nextMobileAuthSecurityPhone()
			employeeID := seedMobileAuthSecurityEmployee(t, ctx, pool, schema, tc.name, phone, tc.accountType, tc.active, true)
			before := readMobileAuthPasswordState(t, ctx, pool, schema, employeeID)
			beforeAudit := countMobileAuthAuditRows(t, ctx, pool, schema, employeeID)
			rec := postMobileAuthJSON(t, e, "/api/auth/password/reset", map[string]any{
				"employee_id": employeeID, "password": "new-secret-123",
			}, true)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status=%d body=%s, want 400", rec.Code, rec.Body.String())
			}
			if after := readMobileAuthPasswordState(t, ctx, pool, schema, employeeID); after != before {
				t.Errorf("password reset changed forbidden account: before=%+v after=%+v", before, after)
			}
			if afterAudit := countMobileAuthAuditRows(t, ctx, pool, schema, employeeID); afterAudit != beforeAudit {
				t.Errorf("forbidden password reset wrote audit rows: before=%d after=%d", beforeAudit, afterAudit)
			}
		})

		t.Run(tc.name+" account state", func(t *testing.T) {
			phone := nextMobileAuthSecurityPhone()
			employeeID := seedMobileAuthSecurityEmployee(t, ctx, pool, schema, tc.name, phone, tc.accountType, tc.active, true)
			before := readMobileAuthPasswordState(t, ctx, pool, schema, employeeID)
			beforeAudit := countMobileAuthAuditRows(t, ctx, pool, schema, employeeID)
			rec := postMobileAuthJSON(t, e, "/api/auth/account-state", map[string]any{
				"employee_id": employeeID, "login_enabled": true,
			}, true)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status=%d body=%s, want 400", rec.Code, rec.Body.String())
			}
			if after := readMobileAuthPasswordState(t, ctx, pool, schema, employeeID); after != before {
				t.Errorf("account-state changed forbidden account: before=%+v after=%+v", before, after)
			}
			if afterAudit := countMobileAuthAuditRows(t, ctx, pool, schema, employeeID); afterAudit != beforeAudit {
				t.Errorf("forbidden account-state wrote audit rows: before=%d after=%d", beforeAudit, afterAudit)
			}
		})
	}
}

func TestAdminAuthMaintenanceAndPreloadedSMSStillWorkForInternalEmployees(t *testing.T) {
	pool, schema := newERPLoginGateTestDB(t)
	ctx := context.Background()
	e := newMobileAuthSecurityEcho(pool, schema)

	for _, accountType := range []string{AccountTypeInternalEmployee, ""} {
		label := accountType
		if label == "" {
			label = "legacy blank"
		}
		t.Run(label, func(t *testing.T) {
			phone := nextMobileAuthSecurityPhone()
			employeeID := seedMobileAuthSecurityEmployee(t, ctx, pool, schema, label, phone, accountType, true, true)

			reset := postMobileAuthJSON(t, e, "/api/auth/password/reset", map[string]any{
				"employee_id": employeeID, "password": "admin-reset-123",
			}, true)
			if reset.Code != http.StatusOK {
				t.Fatalf("password reset status=%d body=%s", reset.Code, reset.Body.String())
			}
			state := readMobileAuthPasswordState(t, ctx, pool, schema, employeeID)
			if state.PasswordHash != hashPassword("admin-reset-123") || state.LoginDisabled || !state.MustResetPassword {
				t.Errorf("unexpected reset state: %+v", state)
			}

			disable := postMobileAuthJSON(t, e, "/api/auth/account-state", map[string]any{
				"employee_id": employeeID, "login_enabled": false,
			}, true)
			if disable.Code != http.StatusOK {
				t.Fatalf("account-state status=%d body=%s", disable.Code, disable.Body.String())
			}
			if state := readMobileAuthPasswordState(t, ctx, pool, schema, employeeID); !state.LoginDisabled {
				t.Errorf("account-state did not disable login: %+v", state)
			}
			if got := countMobileAuthAuditRows(t, ctx, pool, schema, employeeID); got != 2 {
				t.Errorf("admin maintenance audit rows=%d, want 2", got)
			}
		})
	}

	phone, code := nextMobileAuthSecurityPhone(), "840001"
	employeeID := seedMobileAuthSecurityEmployee(t, ctx, pool, schema, "短信登录员工", phone, AccountTypeInternalEmployee, true, false)
	seedMobileAuthSMSCode(t, ctx, pool, schema, phone, code)
	rec := postMobileAuthJSON(t, e, "/api/auth/login", map[string]any{
		"mode": "sms", "phone": phone, "code": code,
	}, false)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"token"`) {
		t.Fatalf("preloaded SMS login status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := countERPLoginSessions(t, ctx, pool, schema, employeeID); got != 1 {
		t.Errorf("preloaded SMS login sessions=%d, want 1", got)
	}
}

func TestCurrentEmployeeSelfDisableGuardOnlyRejectsEmployeeSession(t *testing.T) {
	tests := []struct {
		name           string
		currentID      int64
		targetID       int64
		loginEnabled   bool
		basicAuthAdmin bool
		wantReject     bool
	}{
		{name: "current employee disables self", currentID: 41, targetID: 41, loginEnabled: false, wantReject: true},
		{name: "current employee enables self", currentID: 41, targetID: 41, loginEnabled: true, wantReject: false},
		{name: "current employee disables another employee", currentID: 41, targetID: 42, loginEnabled: false, wantReject: false},
		{name: "basic auth admin remains recovery channel", currentID: 41, targetID: 41, loginEnabled: false, basicAuthAdmin: true, wantReject: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			ctx := e.NewContext(httptest.NewRequest(http.MethodPost, "/api/auth/account-state", nil), httptest.NewRecorder())
			ctx.Set("employee_id", tc.currentID)
			if tc.basicAuthAdmin {
				ctx.Set("basic_auth_admin", true)
			}
			got := rejectCurrentEmployeeSelfDisable(ctx, accountStateReq{
				EmployeeID:   tc.targetID,
				LoginEnabled: tc.loginEnabled,
			})
			if got != tc.wantReject {
				t.Fatalf("rejectCurrentEmployeeSelfDisable()=%v, want %v", got, tc.wantReject)
			}
		})
	}
}

func TestCurrentEmployeeSelfDisableReturnsConflictBeforeDatabaseAccess(t *testing.T) {
	const employeeID int64 = 41
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", employeeID)
			return next(c)
		}
	})
	registerMobileAuthAPI(e, nil, "public", &fakeAuthzService{actor: authzapp.Actor{
		Permissions: []string{"auth.manage"},
	}})

	rec := postMobileAuthJSON(t, e, "/api/auth/account-state", map[string]any{
		"employee_id": employeeID, "login_enabled": false,
	}, false)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s, want 409 before any database access", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"error":"cannot disable current account"`) {
		t.Fatalf("body=%s, want stable self-disable error", rec.Body.String())
	}
}

func TestCurrentEmployeeCannotDisableOwnLoginOrWriteSuccessAudit(t *testing.T) {
	pool, schema := newERPLoginGateTestDB(t)
	ctx := context.Background()
	employeeID := seedMobileAuthSecurityEmployee(t, ctx, pool, schema, "当前管理员", nextMobileAuthSecurityPhone(), AccountTypeInternalEmployee, true, false)
	beforeState := readMobileAuthPasswordState(t, ctx, pool, schema, employeeID)
	beforeAudit := countMobileAuthAuditRows(t, ctx, pool, schema, employeeID)

	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", employeeID)
			c.Set("actor", "当前管理员")
			return next(c)
		}
	})
	registerMobileAuthAPI(e, pool, schema, &fakeAuthzService{actor: authzapp.Actor{
		Permissions: []string{"auth.manage"},
	}})

	rec := postMobileAuthJSON(t, e, "/api/auth/account-state", map[string]any{
		"employee_id": employeeID, "login_enabled": false,
	}, false)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s, want 409", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"error":"cannot disable current account"`) {
		t.Fatalf("body=%s, want stable self-disable error", rec.Body.String())
	}
	if afterState := readMobileAuthPasswordState(t, ctx, pool, schema, employeeID); afterState != beforeState {
		t.Errorf("self-disable changed password state: before=%+v after=%+v", beforeState, afterState)
	}
	if afterAudit := countMobileAuthAuditRows(t, ctx, pool, schema, employeeID); afterAudit != beforeAudit {
		t.Errorf("rejected self-disable wrote success audit rows: before=%d after=%d", beforeAudit, afterAudit)
	}
}

func TestBasicAuthAdminCanReenableDisabledEmployeeAfterSelfDisableGuard(t *testing.T) {
	pool, schema := newERPLoginGateTestDB(t)
	ctx := context.Background()
	employeeID := seedMobileAuthSecurityEmployee(t, ctx, pool, schema, "待恢复员工", nextMobileAuthSecurityPhone(), AccountTypeInternalEmployee, true, true)
	e := newMobileAuthSecurityEcho(pool, schema)

	rec := postMobileAuthJSON(t, e, "/api/auth/account-state", map[string]any{
		"employee_id": employeeID, "login_enabled": true,
	}, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want BasicAuth recovery 200", rec.Code, rec.Body.String())
	}
	if state := readMobileAuthPasswordState(t, ctx, pool, schema, employeeID); state.LoginDisabled {
		t.Fatalf("BasicAuth recovery left login disabled: %+v", state)
	}
	if auditRows := countMobileAuthAuditRows(t, ctx, pool, schema, employeeID); auditRows != 1 {
		t.Fatalf("BasicAuth recovery audit rows=%d, want 1", auditRows)
	}
}

type mobileAuthPasswordState struct {
	PasswordHash      string
	LoginDisabled     bool
	MustResetPassword bool
}

var mobileAuthSecurityPhoneSuffix = 100

func nextMobileAuthSecurityPhone() string {
	mobileAuthSecurityPhoneSuffix++
	return fmt.Sprintf("13989%06d", mobileAuthSecurityPhoneSuffix)
}

func newMobileAuthSecurityEcho(pool *pgxpool.Pool, schema string) *echo.Echo {
	e := echo.New()
	e.Use(BasicAuth("security-admin", "security-secret", schema, pool))
	e.Use(EmployeeContextMiddleware(pool, schema))
	registerMobileAuthAPI(e, pool, schema, nil)
	return e
}

func postMobileAuthJSON(t *testing.T, e *echo.Echo, path string, payload map[string]any, basicAuth bool) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if basicAuth {
		req.SetBasicAuth("security-admin", "security-secret")
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func seedMobileAuthSecurityEmployee(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema, name, phone, accountType string, active, loginDisabled bool) int64 {
	t.Helper()
	var employeeID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.company_employees(name,phone,account_type,department_id,active)
		VALUES($1,$2,$3,(SELECT id FROM %[1]s.company_departments WHERE active=true ORDER BY id LIMIT 1),$4)
		RETURNING id
	`, schema), name, phone, accountType, active).Scan(&employeeID); err != nil {
		t.Fatalf("insert employee: %v", err)
	}
	mustExecERPLoginGateSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.employee_login_passwords(employee_id,password_hash,login_disabled,must_reset_password)
		VALUES($1,$2,$3,false)
	`, schema), employeeID, hashPassword("original-secret"), loginDisabled)
	return employeeID
}

func seedMobileAuthSMSCode(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema, phone, code string) {
	t.Helper()
	mustExecERPLoginGateSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.login_sms_codes(phone,code,expire_at,used)
		VALUES($1,$2,now()+interval '5 minutes',false)
	`, schema), phone, code)
}

func readMobileAuthPasswordState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, employeeID int64) mobileAuthPasswordState {
	t.Helper()
	var state mobileAuthPasswordState
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT password_hash,login_disabled,must_reset_password
		FROM %s.employee_login_passwords
		WHERE employee_id=$1
	`, schema), employeeID).Scan(&state.PasswordHash, &state.LoginDisabled, &state.MustResetPassword); err != nil {
		t.Fatalf("read password state: %v", err)
	}
	return state
}

func countMobileAuthRows(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema, table, where string, args ...any) int {
	t.Helper()
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s WHERE %s", schema, table, where)
	if err := pool.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}

func countMobileAuthAuditRows(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, employeeID int64) int {
	t.Helper()
	return countMobileAuthRows(t, ctx, pool, schema, "audit_logs", "entity_type='auth_account' AND entity_id=$1", employeeID)
}

func assertLatestMobileAuthSMSCodeUnused(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema, phone string) {
	t.Helper()
	var used bool
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT used FROM %s.login_sms_codes WHERE phone=$1 ORDER BY id DESC LIMIT 1
	`, schema), phone).Scan(&used); err != nil {
		t.Fatalf("read SMS code: %v", err)
	}
	if used {
		t.Error("rejected SMS login consumed the preloaded code")
	}
}
