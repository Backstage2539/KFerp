package support

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	authzapp "orderapp/internal/application/authz"
	companyapp "orderapp/internal/application/company"
	customerapp "orderapp/internal/application/customer"
	customerfulfillmentapp "orderapp/internal/application/customerfulfillment"
	customerportalapp "orderapp/internal/application/customerportal"
	postgresauthz "orderapp/internal/infrastructure/postgres/authz"
	postgrescompany "orderapp/internal/infrastructure/postgres/company"
	postgrescustomer "orderapp/internal/infrastructure/postgres/customer"
	postgrescustomerfulfillment "orderapp/internal/infrastructure/postgres/customerfulfillment"
	postgrescustomerportal "orderapp/internal/infrastructure/postgres/customerportal"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func TestERPLoginSessionSecurityChangesPermanentlyRevokeOldBearer(t *testing.T) {
	pool, schema := newERPLoginGateTestDB(t)
	ctx := context.Background()
	repo := postgrescustomerfulfillment.NewRepository(pool, schema)
	service := customerfulfillmentapp.NewService(repo)
	companyRepo := postgrescompany.NewRepository(pool, schema)
	customerRepo := postgrescustomer.NewRepository(pool, schema, t.TempDir())
	portalRepo := postgrescustomerportal.NewRepository(pool, schema)
	portalService := customerportalapp.NewService(portalRepo, nil)
	server := newERPBearerTestServer(pool, schema, service)
	processingTemplate, ok := customerportalapp.CustomerCapabilityTemplateByKey(customerportalapp.CapabilityTemplateProcessingFulfillment)
	if !ok {
		t.Fatal("processing fulfillment template missing")
	}
	retailTemplate, ok := customerportalapp.CustomerCapabilityTemplateByKey(customerportalapp.CapabilityTemplateRetailMall)
	if !ok {
		t.Fatal("retail mall template missing")
	}
	responsibleEmployeeID := seedERPLoginGateAccount(t, ctx, pool, schema, "responsible employee", "13941009999", AccountTypeInternalEmployee, nil, false)

	t.Run("external password reset", func(t *testing.T) {
		employeeID, customerID, token := seedValidERPBearer(t, ctx, pool, schema, "password reset", "13941000001")
		assertERPBearerStatus(t, server, token, http.StatusOK)

		if _, err := service.ResetExternalUserPassword(ctx, customerfulfillmentapp.ResetExternalUserPasswordCommand{
			CustomerID: customerID,
			EmployeeID: employeeID,
			Password:   "new-secret-123",
			Actor:      "session revocation test",
		}); err != nil {
			t.Fatalf("ResetExternalUserPassword: %v", err)
		}

		assertERPBearerPermanentlyRevoked(t, ctx, pool, schema, server, token)
	})

	tests := []struct {
		name   string
		phone  string
		mutate func(t *testing.T, employeeID, customerID int64)
	}{
		{
			name:  "login disable then enable",
			phone: "13941000002",
			mutate: func(t *testing.T, employeeID, customerID int64) {
				for _, enabled := range []bool{false, true} {
					if _, err := service.SetExternalUserLoginEnabled(ctx, customerfulfillmentapp.SetExternalUserLoginEnabledCommand{
						CustomerID:   customerID,
						EmployeeID:   employeeID,
						LoginEnabled: enabled,
						Actor:        "session revocation test",
					}); err != nil {
						t.Fatalf("SetExternalUserLoginEnabled(%t): %v", enabled, err)
					}
				}
			},
		},
		{
			name:  "portal off then on",
			phone: "13941000003",
			mutate: func(t *testing.T, _ int64, customerID int64) {
				for _, enabled := range []bool{false, true} {
					if _, err := portalService.UpdatePortalVisibility(ctx, customerportalapp.UpdatePortalVisibilityCommand{
						CustomerID:            customerID,
						DisplayName:           "portal off then on",
						Enabled:               enabled,
						CapabilityTemplateKey: processingTemplate.Key,
						UpdatedBy:             "session revocation test",
					}); err != nil {
						t.Fatalf("UpdatePortalVisibility(%t): %v", enabled, err)
					}
				}
			},
		},
		{
			name:  "workbench template downgrade then restore",
			phone: "13941000004",
			mutate: func(t *testing.T, _ int64, customerID int64) {
				for _, templateKey := range []string{retailTemplate.Key, processingTemplate.Key} {
					if _, err := portalService.ApplyCapabilityTemplate(ctx, customerportalapp.ApplyCapabilityTemplateCommand{
						CustomerID:  customerID,
						TemplateKey: templateKey,
						UpdatedBy:   "session revocation test",
					}); err != nil {
						t.Fatalf("ApplyCapabilityTemplate(%s): %v", templateKey, err)
					}
				}
			},
		},
		{
			name:  "workbench template definition downgrade then restore",
			phone: "13941000008",
			mutate: func(t *testing.T, _ int64, _ int64) {
				downgraded := processingTemplate
				downgraded.ERPPermissions = []string{}
				downgraded.ERPViewKeys = []string{}
				for _, template := range []customerportalapp.CapabilityTemplate{downgraded, processingTemplate} {
					if _, err := portalRepo.SaveCapabilityTemplate(ctx, customerportalapp.SaveCapabilityTemplateCommand{
						Template: template, UpdatedBy: "session revocation test", ActiveSet: true,
					}); err != nil {
						t.Fatalf("SaveCapabilityTemplate(workbench=%t): %v", template.ExposesERPWorkbench(), err)
					}
				}
			},
		},
		{
			name:  "customer inactive then active",
			phone: "13941000005",
			mutate: func(t *testing.T, _ int64, customerID int64) {
				mustExecERPLoginGateSQL(t, ctx, pool, fmt.Sprintf(`
					UPDATE %s.customers SET responsible_employee_id=$2 WHERE id=$1
				`, schema), customerID, responsibleEmployeeID)
				for _, active := range []string{"", "true"} {
					if err := customerRepo.InlineUpdate(ctx, "session revocation test", customerID, customerapp.InlineUpdateCommand{
						Name:                  "customer inactive then active customer",
						CustomerType:          "wholesale",
						ResponsibleEmployeeID: strconv.FormatInt(responsibleEmployeeID, 10),
						Active:                active,
					}); err != nil {
						t.Fatalf("customer InlineUpdate(active=%q): %v", active, err)
					}
				}
			},
		},
		{
			name:  "employee inactive then active",
			phone: "13941000006",
			mutate: func(t *testing.T, employeeID, _ int64) {
				var name, phone string
				var departmentID int64
				if err := pool.QueryRow(ctx, fmt.Sprintf(`
					SELECT name,phone,department_id FROM %s.company_employees WHERE id=$1
				`, schema), employeeID).Scan(&name, &phone, &departmentID); err != nil {
					t.Fatalf("load employee: %v", err)
				}
				for _, active := range []bool{false, true} {
					if err := companyRepo.UpdateEmployee(ctx, employeeID, companyapp.EmployeeCommand{
						Name: name, Phone: phone, DepartmentID: departmentID, Active: active,
					}); err != nil {
						t.Fatalf("UpdateEmployee(active=%t): %v", active, err)
					}
				}
			},
		},
		{
			name:  "account type downgrade then restore",
			phone: "13941000009",
			mutate: func(t *testing.T, employeeID, _ int64) {
				for _, accountType := range []string{AccountTypeInternalEmployee, AccountTypeChannelCustomer} {
					payload, err := json.Marshal(map[string]any{"employee_id": employeeID, "account_type": accountType})
					if err != nil {
						t.Fatal(err)
					}
					req := httptest.NewRequest(http.MethodPost, "/api/auth/account-type", bytes.NewReader(payload))
					req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
					req.SetBasicAuth("order", "secret")
					rec := httptest.NewRecorder()
					server.ServeHTTP(rec, req)
					if rec.Code != http.StatusOK {
						t.Fatalf("account type %q status=%d body=%s", accountType, rec.Code, rec.Body.String())
					}
				}
			},
		},
		{
			name:  "binding replacement then restore",
			phone: "13941000007",
			mutate: func(t *testing.T, employeeID, customerID int64) {
				var replacementID int64
				if err := pool.QueryRow(ctx, fmt.Sprintf(`
					INSERT INTO %[1]s.company_employees(name,phone,account_type,department_id,active)
					VALUES('replacement account','13941999999',$1,
					       (SELECT id FROM %[1]s.company_departments WHERE active=true ORDER BY id LIMIT 1),true)
					RETURNING id
				`, schema), AccountTypeChannelCustomer).Scan(&replacementID); err != nil {
					t.Fatalf("insert replacement employee: %v", err)
				}
				mustExecERPLoginGateSQL(t, ctx, pool, fmt.Sprintf(`
					INSERT INTO %s.employee_login_passwords(employee_id,password_hash,login_disabled,must_reset_password)
					VALUES($1,$2,false,false)
				`, schema), replacementID, hashPassword("secret123"))
				for _, nextEmployeeID := range []int64{replacementID, employeeID} {
					if _, err := portalService.UpsertPortalERPBinding(ctx, customerportalapp.UpsertPortalERPBindingCommand{
						CustomerID: customerID,
						EmployeeID: nextEmployeeID,
						Status:     "active",
						UpdatedBy:  "session revocation test",
					}); err != nil {
						t.Fatalf("UpsertPortalERPBinding(employee=%d): %v", nextEmployeeID, err)
					}
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			employeeID, customerID, token := seedValidERPBearer(t, ctx, pool, schema, tc.name, tc.phone)
			assertERPBearerStatus(t, server, token, http.StatusOK)

			tc.mutate(t, employeeID, customerID)

			// The account is eligible again, but the token predates a security
			// change and must never become usable again.
			assertERPBearerPermanentlyRevoked(t, ctx, pool, schema, server, token)
		})
	}
}

func TestERPLoginSessionCurrentIneligibilityDeletesTokenBeforeStateRestoration(t *testing.T) {
	pool, schema := newERPLoginGateTestDB(t)
	ctx := context.Background()
	service := customerfulfillmentapp.NewService(postgrescustomerfulfillment.NewRepository(pool, schema))
	server := newERPBearerTestServer(pool, schema, service)
	employeeID, customerID, token := seedValidERPBearer(t, ctx, pool, schema, "currently disabled", "13942000001")
	assertERPBearerStatus(t, server, token, http.StatusOK)

	if _, err := service.SetExternalUserLoginEnabled(ctx, customerfulfillmentapp.SetExternalUserLoginEnabledCommand{
		CustomerID:   customerID,
		EmployeeID:   employeeID,
		LoginEnabled: false,
		Actor:        "session revocation test",
	}); err != nil {
		t.Fatalf("disable external user: %v", err)
	}
	assertERPBearerStatus(t, server, token, http.StatusUnauthorized)
	assertERPLoginTokenCount(t, ctx, pool, schema, token, 0)

	if _, err := service.SetExternalUserLoginEnabled(ctx, customerfulfillmentapp.SetExternalUserLoginEnabledCommand{
		CustomerID:   customerID,
		EmployeeID:   employeeID,
		LoginEnabled: true,
		Actor:        "session revocation test",
	}); err != nil {
		t.Fatalf("re-enable external user: %v", err)
	}
	assertERPBearerStatus(t, server, token, http.StatusUnauthorized)
}

func TestERPLoginSessionBenignProfileEditsKeepBearerValid(t *testing.T) {
	pool, schema := newERPLoginGateTestDB(t)
	ctx := context.Background()
	service := customerfulfillmentapp.NewService(postgrescustomerfulfillment.NewRepository(pool, schema))
	companyRepo := postgrescompany.NewRepository(pool, schema)
	customerRepo := postgrescustomer.NewRepository(pool, schema, t.TempDir())
	portalRepo := postgrescustomerportal.NewRepository(pool, schema)
	portalService := customerportalapp.NewService(portalRepo, nil)
	server := newERPBearerTestServer(pool, schema, service)
	mustExecERPLoginGateSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.customer_capability_templates(
			template_key,label,description,erp_permissions,erp_view_keys,active,updated_by
		)
		VALUES(
			'processing_fulfillment','加工履约客户','初始说明',
			'["customer_processing.read"]'::jsonb,
			'["customerProcessingPortal"]'::jsonb,
			true,'session revocation test'
		)
	`, schema))
	processingTemplate, ok := customerportalapp.CustomerCapabilityTemplateByKey(customerportalapp.CapabilityTemplateProcessingFulfillment)
	if !ok {
		t.Fatal("processing fulfillment template missing")
	}
	processingTemplate.Label = "加工履约客户"
	processingTemplate.Description = "初始说明"
	responsibleEmployeeID := seedERPLoginGateAccount(t, ctx, pool, schema, "benign responsible employee", "13942509999", AccountTypeInternalEmployee, nil, false)

	tests := []struct {
		name   string
		phone  string
		mutate func(t *testing.T, employeeID, customerID int64)
	}{
		{
			name:  "employee name edit",
			phone: "13942500001",
			mutate: func(t *testing.T, employeeID, _ int64) {
				var phone string
				var departmentID int64
				if err := pool.QueryRow(ctx, fmt.Sprintf(`
					SELECT phone,department_id FROM %s.company_employees WHERE id=$1
				`, schema), employeeID).Scan(&phone, &departmentID); err != nil {
					t.Fatalf("load employee: %v", err)
				}
				if err := companyRepo.UpdateEmployee(ctx, employeeID, companyapp.EmployeeCommand{
					Name: "仅修改姓名", Phone: phone, DepartmentID: departmentID, Active: true,
				}); err != nil {
					t.Fatalf("UpdateEmployee(name): %v", err)
				}
			},
		},
		{
			name:  "customer address edit",
			phone: "13942500002",
			mutate: func(t *testing.T, _ int64, customerID int64) {
				mustExecERPLoginGateSQL(t, ctx, pool, fmt.Sprintf(`
					UPDATE %s.customers SET responsible_employee_id=$2 WHERE id=$1
				`, schema), customerID, responsibleEmployeeID)
				if err := customerRepo.InlineUpdate(ctx, "session revocation test", customerID, customerapp.InlineUpdateCommand{
					Name:                  "customer address edit customer",
					CustomerType:          "wholesale",
					CompanyAddress:        "仅修改客户地址",
					ResponsibleEmployeeID: strconv.FormatInt(responsibleEmployeeID, 10),
					Active:                "true",
				}); err != nil {
					t.Fatalf("customer InlineUpdate(address): %v", err)
				}
			},
		},
		{
			name:  "portal appearance edit",
			phone: "13942500003",
			mutate: func(t *testing.T, _ int64, customerID int64) {
				if _, err := portalRepo.UpdatePortalVisibility(ctx, customerportalapp.UpdatePortalVisibilityCommand{
					CustomerID:            customerID,
					DisplayName:           "仅修改显示名",
					Enabled:               true,
					ThemeKey:              customerportalapp.PortalThemePremiumPartner,
					MiniappEntryMode:      customerportalapp.MiniappEntryModeServices,
					CapabilityTemplateKey: processingTemplate.Key,
					UpdatedBy:             "session revocation test",
				}); err != nil {
					t.Fatalf("UpdatePortalVisibility(appearance): %v", err)
				}
			},
		},
		{
			name:  "template copy edit",
			phone: "13942500004",
			mutate: func(t *testing.T, _ int64, _ int64) {
				edited := processingTemplate
				edited.Label = "仅修改模板名称"
				edited.Description = "仅修改模板说明"
				if _, err := portalService.SaveCapabilityTemplate(ctx, customerportalapp.SaveCapabilityTemplateCommand{
					Template: edited, UpdatedBy: "session revocation test", ActiveSet: true,
				}); err != nil {
					t.Fatalf("SaveCapabilityTemplate(copy): %v", err)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			employeeID, customerID, token := seedValidERPBearer(t, ctx, pool, schema, tc.name, tc.phone)
			assertERPBearerStatus(t, server, token, http.StatusOK)
			tc.mutate(t, employeeID, customerID)
			assertERPBearerStatus(t, server, token, http.StatusOK)
			assertERPLoginTokenCount(t, ctx, pool, schema, token, 1)
		})
	}
}

func TestInternalEmployeePasswordResetRevokesOnlyOldSessionAndFreshLoginWorks(t *testing.T) {
	pool, schema := newERPLoginGateTestDB(t)
	ctx := context.Background()
	service := customerfulfillmentapp.NewService(postgrescustomerfulfillment.NewRepository(pool, schema))
	server := newERPBearerTestServer(pool, schema, service)
	employeeID := seedERPLoginGateAccount(t, ctx, pool, schema, "internal employee", "13943000001", AccountTypeInternalEmployee, nil, false)
	token := "internal-password-reset-old-token"
	setERPLoginSecurityBaseline(t, ctx, pool, schema, employeeID, 0)
	insertERPLoginSession(t, ctx, pool, schema, employeeID, token)
	assertERPBearerStatus(t, server, token, http.StatusOK)

	payload, err := json.Marshal(map[string]any{"employee_id": employeeID, "password": "new-internal-secret"})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/auth/password/reset", bytes.NewReader(payload))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.SetBasicAuth("order", "secret")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("password reset status=%d body=%s", rec.Code, rec.Body.String())
	}

	assertERPBearerPermanentlyRevoked(t, ctx, pool, schema, server, token)
	loginRec := serveERPLoginRequest(t, server, map[string]any{
		"mode":     "password",
		"phone":    "13943000001",
		"password": "new-internal-secret",
	})
	if loginRec.Code != http.StatusOK {
		t.Fatalf("fresh password login status=%d body=%s", loginRec.Code, loginRec.Body.String())
	}
	var loginBody struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginBody); err != nil || loginBody.Token == "" {
		t.Fatalf("decode fresh login token: token=%q err=%v body=%s", loginBody.Token, err, loginRec.Body.String())
	}
	assertERPBearerStatus(t, server, loginBody.Token, http.StatusOK)
}

func newERPBearerTestServer(pool *pgxpool.Pool, schema string, eligibility ERPWorkbenchLoginEligibility) *echo.Echo {
	authz := authzapp.NewService(postgresauthz.NewRepository(pool, schema))
	e := echo.New()
	e.Use(BasicAuth("order", "secret", schema, pool, eligibility))
	e.Use(EmployeeContextMiddleware(pool, schema, eligibility))
	registerAuthzAPI(e, authz)
	registerMobileAuthAPI(e, pool, schema, authz, eligibility)
	return e
}

func seedValidERPBearer(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema, name, phone string) (int64, int64, string) {
	t.Helper()
	employeeID := seedERPLoginGateAccount(t, ctx, pool, schema, name, phone, AccountTypeChannelCustomer, stringPointerForERPLoginTest("processing_fulfillment"), true)
	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT customer_id
		FROM %s.customer_erp_user_bindings
		WHERE employee_id=$1 AND status='active'
	`, schema), employeeID).Scan(&customerID); err != nil {
		t.Fatalf("load customer binding: %v", err)
	}
	setERPLoginSecurityBaseline(t, ctx, pool, schema, employeeID, customerID)
	token := fmt.Sprintf("erp-security-token-%d", employeeID)
	insertERPLoginSession(t, ctx, pool, schema, employeeID, token)
	return employeeID, customerID, token
}

func setERPLoginSecurityBaseline(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, employeeID, customerID int64) {
	t.Helper()
	baseline := time.Now().Add(-2 * time.Hour)
	mustExecERPLoginGateSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.company_employees SET updated_at=$2 WHERE id=$1`, schema), employeeID, baseline)
	mustExecERPLoginGateSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.employee_login_passwords SET updated_at=$2 WHERE employee_id=$1`, schema), employeeID, baseline)
	if customerID <= 0 {
		return
	}
	mustExecERPLoginGateSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.customers SET updated_at=$2 WHERE id=$1`, schema), customerID, baseline)
	mustExecERPLoginGateSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.customer_portal_profiles SET updated_at=$2 WHERE customer_id=$1`, schema), customerID, baseline)
	mustExecERPLoginGateSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.customer_erp_user_bindings SET updated_at=$2 WHERE employee_id=$1 AND customer_id=$3`, schema), employeeID, baseline, customerID)
}

func insertERPLoginSession(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, employeeID int64, token string) {
	t.Helper()
	mustExecERPLoginGateSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.login_sessions(token,employee_id,created_at,expire_at)
		VALUES($1,$2,$3,now()+interval '7 days')
	`, schema), token, employeeID, time.Now().Add(-time.Hour))
}

func assertERPBearerPermanentlyRevoked(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, server *echo.Echo, token string) {
	t.Helper()
	assertERPBearerStatus(t, server, token, http.StatusUnauthorized)
	assertERPLoginTokenCount(t, ctx, pool, schema, token, 0)
	assertERPBearerStatus(t, server, token, http.StatusUnauthorized)
}

func assertERPBearerStatus(t *testing.T, server *echo.Echo, token string, want int) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != want {
		t.Fatalf("bearer status=%d body=%s, want %d", rec.Code, rec.Body.String(), want)
	}
}

func assertERPLoginTokenCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema, token string, want int) {
	t.Helper()
	var got int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.login_sessions WHERE token=$1`, schema), token).Scan(&got); err != nil {
		t.Fatalf("count login token: %v", err)
	}
	if got != want {
		t.Fatalf("login token rows=%d, want %d for %q", got, want, token)
	}
}
