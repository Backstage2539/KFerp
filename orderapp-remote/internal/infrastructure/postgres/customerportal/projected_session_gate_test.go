package customerportal

import (
	"context"
	"errors"
	"fmt"
	"testing"

	customerportalapp "orderapp/internal/application/customerportal"

	"github.com/jackc/pgx/v5/pgxpool"
)

type projectedMiniSessionFixture struct {
	ctx                   context.Context
	pool                  *pgxpool.Pool
	schema                string
	repo                  Repository
	customerID            int64
	employeeID            int64
	replacementEmployeeID int64
	login                 string
	password              string
	token                 string
	miniUserID            int64
}

func TestPasswordProjectedMiniSessionRevokesWhenLiveAccessBecomesInvalid(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*testing.T, *projectedMiniSessionFixture)
		restore func(*testing.T, *projectedMiniSessionFixture)
	}{
		{
			name: "login disabled",
			mutate: func(t *testing.T, f *projectedMiniSessionFixture) {
				execProjectedSessionSQL(t, f, `UPDATE %s.employee_login_passwords SET login_disabled=true,updated_at=now() WHERE employee_id=$1`, f.employeeID)
			},
			restore: func(t *testing.T, f *projectedMiniSessionFixture) {
				execProjectedSessionSQL(t, f, `UPDATE %s.employee_login_passwords SET login_disabled=false,updated_at=now() WHERE employee_id=$1`, f.employeeID)
			},
		},
		{
			name: "employee inactive",
			mutate: func(t *testing.T, f *projectedMiniSessionFixture) {
				execProjectedSessionSQL(t, f, `UPDATE %s.company_employees SET active=false,updated_at=now() WHERE id=$1`, f.employeeID)
			},
			restore: func(t *testing.T, f *projectedMiniSessionFixture) {
				execProjectedSessionSQL(t, f, `UPDATE %s.company_employees SET active=true,updated_at=now() WHERE id=$1`, f.employeeID)
			},
		},
		{
			name: "employee account type changed",
			mutate: func(t *testing.T, f *projectedMiniSessionFixture) {
				execProjectedSessionSQL(t, f, `UPDATE %s.company_employees SET account_type='internal_employee',updated_at=now() WHERE id=$1`, f.employeeID)
			},
			restore: func(t *testing.T, f *projectedMiniSessionFixture) {
				execProjectedSessionSQL(t, f, `UPDATE %s.company_employees SET account_type='channel_customer',updated_at=now() WHERE id=$1`, f.employeeID)
			},
		},
		{
			name: "ERP binding replaced",
			mutate: func(t *testing.T, f *projectedMiniSessionFixture) {
				execProjectedSessionSQL(t, f, `UPDATE %s.customer_erp_user_bindings SET status='inactive',updated_at=now() WHERE customer_id=$1 AND employee_id=$2`, f.customerID, f.employeeID)
				if err := f.pool.QueryRow(f.ctx, fmt.Sprintf(`
					INSERT INTO %s.company_employees(name,phone,account_type,department_id,active)
					VALUES('替换后的渠道账号','13960000002','channel_customer',(SELECT id FROM %s.company_departments WHERE active=true ORDER BY id LIMIT 1),true)
					RETURNING id
				`, f.schema, f.schema)).Scan(&f.replacementEmployeeID); err != nil {
					t.Fatalf("insert replacement employee: %v", err)
				}
				execProjectedSessionSQL(t, f, `
					INSERT INTO %s.customer_erp_user_bindings(customer_id,employee_id,role,status,updated_by)
					VALUES($1,$2,'customer','active','replacement-test')
				`, f.customerID, f.replacementEmployeeID)
			},
			restore: func(t *testing.T, f *projectedMiniSessionFixture) {
				execProjectedSessionSQL(t, f, `UPDATE %s.customer_erp_user_bindings SET status='inactive',updated_at=now() WHERE customer_id=$1 AND employee_id=$2`, f.customerID, f.replacementEmployeeID)
				execProjectedSessionSQL(t, f, `UPDATE %s.customer_erp_user_bindings SET status='active',updated_at=now() WHERE customer_id=$1 AND employee_id=$2`, f.customerID, f.employeeID)
			},
		},
		{
			name: "customer inactive",
			mutate: func(t *testing.T, f *projectedMiniSessionFixture) {
				execProjectedSessionSQL(t, f, `UPDATE %s.customers SET active=false,updated_at=now() WHERE id=$1`, f.customerID)
			},
			restore: func(t *testing.T, f *projectedMiniSessionFixture) {
				execProjectedSessionSQL(t, f, `UPDATE %s.customers SET active=true,updated_at=now() WHERE id=$1`, f.customerID)
			},
		},
		{
			name: "portal disabled",
			mutate: func(t *testing.T, f *projectedMiniSessionFixture) {
				execProjectedSessionSQL(t, f, `UPDATE %s.customer_portal_profiles SET enabled=false,updated_at=now() WHERE customer_id=$1`, f.customerID)
			},
			restore: func(t *testing.T, f *projectedMiniSessionFixture) {
				execProjectedSessionSQL(t, f, `UPDATE %s.customer_portal_profiles SET enabled=true,updated_at=now() WHERE customer_id=$1`, f.customerID)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newProjectedPasswordSessionFixture(t)
			tc.mutate(t, f)
			assertProjectedSessionRevoked(t, f, "erp-password-login")

			tc.restore(t, f)
			if _, err := f.repo.CurrentContextByToken(f.ctx, f.token); !errors.Is(err, customerportalapp.ErrMiniSessionNotFound) {
				t.Fatalf("old token revived after access was restored: err=%v", err)
			}
			fresh, err := f.repo.CreatePasswordLoginSession(f.ctx, customerportalapp.CreatePasswordLoginSessionCommand{Login: f.login, Password: f.password})
			if err != nil {
				t.Fatalf("fresh password login after access restore: %v", err)
			}
			if fresh.Token == "" || fresh.Token == f.token || fresh.CurrentCustomerID != f.customerID {
				t.Fatalf("fresh login=%+v, want a new token for customer %d", fresh, f.customerID)
			}
		})
	}
}

func TestPhoneVerifiedProjectedMiniSessionRevokesWhenLoginDisabled(t *testing.T) {
	f := newProjectedPhoneSessionFixture(t)
	execProjectedSessionSQL(t, f, `UPDATE %s.employee_login_passwords SET login_disabled=true,updated_at=now() WHERE employee_id=$1`, f.employeeID)
	assertProjectedSessionRevoked(t, f, "phone-verify-login")

	execProjectedSessionSQL(t, f, `UPDATE %s.employee_login_passwords SET login_disabled=false,updated_at=now() WHERE employee_id=$1`, f.employeeID)
	if _, err := f.repo.CurrentContextByToken(f.ctx, f.token); !errors.Is(err, customerportalapp.ErrMiniSessionNotFound) {
		t.Fatalf("old phone-verify token revived after login re-enabled: err=%v", err)
	}
	fresh, err := f.repo.CreatePhoneVerifiedLoginSession(f.ctx, customerportalapp.CreatePhoneVerifiedLoginSessionCommand{
		OpenID: "openid-projected-phone", Phone: f.login, Nickname: "手机号验证客户",
	})
	if err != nil {
		t.Fatalf("fresh phone verified login: %v", err)
	}
	if fresh.Token == "" || fresh.Token == f.token || fresh.CurrentCustomerID != f.customerID {
		t.Fatalf("fresh phone login=%+v, want a new token for customer %d", fresh, f.customerID)
	}
}

func TestAllLegacyProjectedMiniSessionsStayRevokedAfterFirstTokenRevokesBinding(t *testing.T) {
	for _, loginType := range []string{"password", "phone_verify"} {
		t.Run(loginType, func(t *testing.T) {
			var f *projectedMiniSessionFixture
			switch loginType {
			case "password":
				f = newProjectedPasswordSessionFixture(t)
			case "phone_verify":
				f = newProjectedPhoneSessionFixture(t)
			}
			secondToken := "second-legacy-" + loginType
			if _, err := f.pool.Exec(f.ctx, fmt.Sprintf(`
				INSERT INTO %s.mini_sessions(token,mini_user_id,current_customer_id,created_at,expire_at)
				SELECT $2,mini_user_id,current_customer_id,created_at,'infinity'
				FROM %s.mini_sessions WHERE token=$1
			`, f.schema, f.schema), f.token, secondToken); err != nil {
				t.Fatalf("insert second legacy projected session: %v", err)
			}
			execProjectedSessionSQL(t, f, `UPDATE %s.employee_login_passwords SET login_disabled=true,updated_at=now() WHERE employee_id=$1`, f.employeeID)

			if _, err := f.repo.CurrentContextByToken(f.ctx, f.token); !errors.Is(err, customerportalapp.ErrMiniSessionNotFound) {
				t.Fatalf("first legacy token err=%v, want ErrMiniSessionNotFound", err)
			}
			if _, err := f.repo.CurrentContextByToken(f.ctx, secondToken); !errors.Is(err, customerportalapp.ErrMiniSessionNotFound) {
				t.Fatalf("second legacy token after projected binding revoked err=%v, want ErrMiniSessionNotFound", err)
			}
			execProjectedSessionSQL(t, f, `UPDATE %s.employee_login_passwords SET login_disabled=false,updated_at=now() WHERE employee_id=$1`, f.employeeID)
			for _, token := range []string{f.token, secondToken} {
				if _, err := f.repo.CurrentContextByToken(f.ctx, token); !errors.Is(err, customerportalapp.ErrMiniSessionNotFound) {
					t.Fatalf("legacy token %q revived after login re-enabled: err=%v", token, err)
				}
			}
		})
	}
}

func TestValidPasswordSessionIgnoresHistoricalRevokedProjectionForAnotherCustomer(t *testing.T) {
	f := newProjectedPasswordSessionFixture(t)
	var historicalCustomerID int64
	if err := f.pool.QueryRow(f.ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name,active)
		VALUES('历史投影客户',true)
		RETURNING id
	`, f.schema)).Scan(&historicalCustomerID); err != nil {
		t.Fatalf("insert historical projected customer: %v", err)
	}
	execProjectedSessionSQL(t, f, `
		INSERT INTO %s.customer_portal_profiles(customer_id,display_name,enabled)
		VALUES($1,'历史投影客户',true)
	`, historicalCustomerID)
	execProjectedSessionSQL(t, f, `
		INSERT INTO %s.customer_portal_user_bindings(mini_user_id,customer_id,role,status,approved_by)
		VALUES($1,$2,'customer','revoked',$3)
	`, f.miniUserID, historicalCustomerID, projectedMiniBindingSource(projectedMiniBindingPasswordSource, f.employeeID))

	current, err := f.repo.CurrentContextByToken(f.ctx, f.token)
	if err != nil {
		t.Fatalf("valid current projection was invalidated by historical revoked projection: %v", err)
	}
	if current.CurrentCustomerID != f.customerID {
		t.Fatalf("current customer=%d, want live projected customer %d", current.CurrentCustomerID, f.customerID)
	}
}

func TestProjectedMiniSessionDoesNotReviveWhenAccessRestoredBeforeNextRead(t *testing.T) {
	f := newProjectedPasswordSessionFixture(t)
	execProjectedSessionSQL(t, f, `UPDATE %s.employee_login_passwords SET login_disabled=true,updated_at=now() WHERE employee_id=$1`, f.employeeID)
	execProjectedSessionSQL(t, f, `UPDATE %s.employee_login_passwords SET login_disabled=false,updated_at=now() WHERE employee_id=$1`, f.employeeID)

	if _, err := f.repo.CurrentContextByToken(f.ctx, f.token); !errors.Is(err, customerportalapp.ErrMiniSessionNotFound) {
		t.Fatalf("old token revived when login was disabled and re-enabled between reads: err=%v", err)
	}
	fresh, err := f.repo.CreatePasswordLoginSession(f.ctx, customerportalapp.CreatePasswordLoginSessionCommand{Login: f.login, Password: f.password})
	if err != nil {
		t.Fatalf("fresh login after disable/enable cycle: %v", err)
	}
	if fresh.Token == "" || fresh.Token == f.token {
		t.Fatalf("fresh login token=%q, want a new token", fresh.Token)
	}
}

func TestPhoneVerifiedProjectedSessionPinsVerifiedEmployeeIdentity(t *testing.T) {
	f := newProjectedPhoneSessionFixture(t)
	var approvedBy string
	if err := f.pool.QueryRow(f.ctx, fmt.Sprintf(`
		SELECT approved_by FROM %s.customer_portal_user_bindings
		WHERE mini_user_id=$1 AND customer_id=$2
	`, f.schema), f.miniUserID, f.customerID).Scan(&approvedBy); err != nil {
		t.Fatalf("load phone projected binding source: %v", err)
	}
	wantApprovedBy := fmt.Sprintf("phone-verify-login:%d", f.employeeID)
	if approvedBy != wantApprovedBy {
		t.Fatalf("phone projected approved_by=%q, want %q", approvedBy, wantApprovedBy)
	}

	execProjectedSessionSQL(t, f, `UPDATE %s.customer_erp_user_bindings SET status='inactive',updated_at=now() WHERE customer_id=$1 AND employee_id=$2`, f.customerID, f.employeeID)
	var replacementEmployeeID int64
	if err := f.pool.QueryRow(f.ctx, fmt.Sprintf(`
		INSERT INTO %s.company_employees(name,phone,account_type,department_id,active)
		VALUES('手机号替换账号','13960000006','channel_customer',(SELECT id FROM %s.company_departments WHERE active=true ORDER BY id LIMIT 1),true)
		RETURNING id
	`, f.schema, f.schema)).Scan(&replacementEmployeeID); err != nil {
		t.Fatalf("insert phone replacement employee: %v", err)
	}
	execProjectedSessionSQL(t, f, `INSERT INTO %s.employee_login_passwords(employee_id,password_hash,login_disabled) VALUES($1,'hash',false)`, replacementEmployeeID)
	execProjectedSessionSQL(t, f, `INSERT INTO %s.customer_erp_user_bindings(customer_id,employee_id,role,status,updated_by) VALUES($1,$2,'customer','active','phone-replacement')`, f.customerID, replacementEmployeeID)
	execProjectedSessionSQL(t, f, `UPDATE %s.mini_users SET phone='13960000006' WHERE id=$1`, f.miniUserID)

	if _, err := f.repo.CurrentContextByToken(f.ctx, f.token); !errors.Is(err, customerportalapp.ErrMiniSessionNotFound) {
		t.Fatalf("old phone token followed mutable mini user phone to replacement employee: err=%v", err)
	}
	fresh, err := f.repo.CreatePhoneVerifiedLoginSession(f.ctx, customerportalapp.CreatePhoneVerifiedLoginSessionCommand{
		OpenID: "openid-projected-phone", Phone: "13960000006", Nickname: "手机号替换账号",
	})
	if err != nil {
		t.Fatalf("fresh phone verification for replacement employee: %v", err)
	}
	if fresh.Token == "" || fresh.Token == f.token || fresh.CurrentCustomerID != f.customerID {
		t.Fatalf("fresh replacement phone login=%+v, want new token for customer %d", fresh, f.customerID)
	}
	if _, err := f.repo.CurrentContextByToken(f.ctx, f.token); !errors.Is(err, customerportalapp.ErrMiniSessionNotFound) {
		t.Fatalf("old phone token revived after replacement employee logged in: err=%v", err)
	}
}

func TestProjectedMiniSessionValidationPreservesManualApprovedBinding(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	ensureCustomerPortalERPBindingTestSchema(t, ctx, pool, schema)
	repo := NewRepository(pool, schema)

	var manualCustomerID, projectedCustomerID, employeeID, miniUserID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name,active) VALUES('人工绑定客户',true) RETURNING id`, schema)).Scan(&manualCustomerID); err != nil {
		t.Fatalf("insert manual customer: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name,active) VALUES('失效投影客户',true) RETURNING id`, schema)).Scan(&projectedCustomerID); err != nil {
		t.Fatalf("insert projected customer: %v", err)
	}
	execSQL := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, fmt.Sprintf(query, schema), args...); err != nil {
			t.Fatalf("exec test SQL: %v", err)
		}
	}
	execSQL(`INSERT INTO %s.customer_portal_profiles(customer_id,display_name,enabled) VALUES($1,'人工绑定客户',true),($2,'失效投影客户',true)`, manualCustomerID, projectedCustomerID)
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.company_employees(name,phone,account_type,department_id,active)
		VALUES('已停用渠道账号','13960000003','channel_customer',(SELECT id FROM %s.company_departments WHERE active=true ORDER BY id LIMIT 1),true)
		RETURNING id
	`, schema, schema)).Scan(&employeeID); err != nil {
		t.Fatalf("insert channel employee: %v", err)
	}
	execSQL(`INSERT INTO %s.employee_login_passwords(employee_id,password_hash,login_disabled) VALUES($1,'hash',true)`, employeeID)
	execSQL(`INSERT INTO %s.customer_erp_user_bindings(customer_id,employee_id,role,status,updated_by) VALUES($1,$2,'customer','active','test')`, projectedCustomerID, employeeID)
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.mini_users(openid,phone,active) VALUES('openid-manual-plus-projected','13960000003',true) RETURNING id`, schema)).Scan(&miniUserID); err != nil {
		t.Fatalf("insert mini user: %v", err)
	}
	execSQL(`
		INSERT INTO %s.customer_portal_user_bindings(mini_user_id,customer_id,role,status,approved_by)
		VALUES($1,$2,'owner','approved','manual-admin'),($1,$3,'customer','approved','phone-verify-login')
	`, miniUserID, manualCustomerID, projectedCustomerID)
	execSQL(`INSERT INTO %s.mini_sessions(token,mini_user_id,current_customer_id,expire_at) VALUES('manual-token',$1,$2,'infinity')`, miniUserID, manualCustomerID)

	current, err := repo.CurrentContextByToken(ctx, "manual-token")
	if err != nil {
		t.Fatalf("CurrentContextByToken manual binding: %v", err)
	}
	if current.CurrentCustomerID != manualCustomerID || len(current.Bindings) != 1 || current.Bindings[0].CustomerID != manualCustomerID {
		t.Fatalf("manual context=%+v, want only manual customer %d", current, manualCustomerID)
	}
	var manualStatus, projectedStatus string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.customer_portal_user_bindings WHERE mini_user_id=$1 AND customer_id=$2`, schema), miniUserID, manualCustomerID).Scan(&manualStatus); err != nil {
		t.Fatalf("load manual binding status: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.customer_portal_user_bindings WHERE mini_user_id=$1 AND customer_id=$2`, schema), miniUserID, projectedCustomerID).Scan(&projectedStatus); err != nil {
		t.Fatalf("load projected binding status: %v", err)
	}
	if manualStatus != "approved" || projectedStatus != "revoked" {
		t.Fatalf("binding statuses manual/projected=%s/%s, want approved/revoked", manualStatus, projectedStatus)
	}
	var sessionCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.mini_sessions WHERE token='manual-token'`, schema)).Scan(&sessionCount); err != nil {
		t.Fatalf("count manual session: %v", err)
	}
	if sessionCount != 1 {
		t.Fatalf("manual session count=%d, want 1", sessionCount)
	}
}

func TestLegacyPhoneVerifyBindingWithoutEmployeeIdentityFailsClosed(t *testing.T) {
	f := newProjectedPhoneSessionFixture(t)
	execProjectedSessionSQL(t, f, `UPDATE %s.customer_portal_user_bindings SET approved_by='phone-verify-login' WHERE mini_user_id=$1 AND customer_id=$2`, f.miniUserID, f.customerID)

	if _, err := f.repo.CurrentContextByToken(f.ctx, f.token); !errors.Is(err, customerportalapp.ErrMiniSessionNotFound) {
		t.Fatalf("legacy phone-verify binding without employee identity err=%v, want ErrMiniSessionNotFound", err)
	}
	var status string
	if err := f.pool.QueryRow(f.ctx, fmt.Sprintf(`SELECT status FROM %s.customer_portal_user_bindings WHERE mini_user_id=$1 AND customer_id=$2`, f.schema), f.miniUserID, f.customerID).Scan(&status); err != nil {
		t.Fatalf("load legacy phone binding status: %v", err)
	}
	if status != "revoked" {
		t.Fatalf("legacy phone binding status=%q, want revoked", status)
	}
}

func TestSwitchCurrentCustomerRejectsStaleProjectedBindingWithoutChangingCurrentCustomer(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	ensureCustomerPortalERPBindingTestSchema(t, ctx, pool, schema)
	repo := NewRepository(pool, schema)

	var manualCustomerID, projectedCustomerID, employeeID, miniUserID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name,active) VALUES('当前人工客户',true) RETURNING id`, schema)).Scan(&manualCustomerID); err != nil {
		t.Fatalf("insert manual customer: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name,active) VALUES('待切换投影客户',true) RETURNING id`, schema)).Scan(&projectedCustomerID); err != nil {
		t.Fatalf("insert projected customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_portal_profiles(customer_id,display_name,enabled) VALUES($1,'当前人工客户',true),($2,'待切换投影客户',true)`, schema), manualCustomerID, projectedCustomerID); err != nil {
		t.Fatalf("insert customer profiles: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.company_employees(name,phone,account_type,department_id,active)
		VALUES('待切换渠道账号','13960000007','channel_customer',(SELECT id FROM %s.company_departments WHERE active=true ORDER BY id LIMIT 1),true)
		RETURNING id
	`, schema, schema)).Scan(&employeeID); err != nil {
		t.Fatalf("insert projected employee: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.employee_login_passwords(employee_id,password_hash,login_disabled) VALUES($1,'hash',false)`, schema), employeeID); err != nil {
		t.Fatalf("insert projected login: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_erp_user_bindings(customer_id,employee_id,role,status,updated_by) VALUES($1,$2,'customer','active','test')`, schema), projectedCustomerID, employeeID); err != nil {
		t.Fatalf("insert projected ERP binding: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.mini_users(openid,phone,active) VALUES('openid-stale-switch','13960000007',true) RETURNING id`, schema)).Scan(&miniUserID); err != nil {
		t.Fatalf("insert mini user: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_portal_user_bindings(mini_user_id,customer_id,role,status,approved_by)
		VALUES($1,$2,'owner','approved','manual-admin'),($1,$3,'customer','approved',$4)
	`, schema), miniUserID, manualCustomerID, projectedCustomerID, projectedMiniBindingSource(projectedMiniBindingPhoneVerifySource, employeeID)); err != nil {
		t.Fatalf("insert manual and projected bindings: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.mini_sessions(token,mini_user_id,current_customer_id,created_at,expire_at)
		VALUES('stale-switch-token',$1,$2,now()-interval '1 hour','infinity')
	`, schema), miniUserID, manualCustomerID); err != nil {
		t.Fatalf("insert stale session: %v", err)
	}

	if _, err := repo.SwitchCurrentCustomer(ctx, "stale-switch-token", projectedCustomerID); !errors.Is(err, customerportalapp.ErrMiniSessionNotFound) {
		t.Fatalf("SwitchCurrentCustomer stale projected err=%v, want ErrMiniSessionNotFound", err)
	}
	var storedCurrentCustomerID int64
	var expired bool
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(current_customer_id,0), expire_at<=now()
		FROM %s.mini_sessions WHERE token='stale-switch-token'
	`, schema)).Scan(&storedCurrentCustomerID, &expired); err != nil {
		t.Fatalf("load stale switch session: %v", err)
	}
	if storedCurrentCustomerID != manualCustomerID || !expired {
		t.Fatalf("stale switch session current/expired=%d/%v, want %d/true", storedCurrentCustomerID, expired, manualCustomerID)
	}
}

func TestPhoneVerifiedLoginPreservesExistingManualApprovedBindings(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	ensureCustomerPortalERPBindingTestSchema(t, ctx, pool, schema)
	repo := NewRepository(pool, schema)

	var erpCustomerID, extraCustomerID, employeeID, miniUserID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name,active) VALUES('已有人工目标客户',true) RETURNING id`, schema)).Scan(&erpCustomerID); err != nil {
		t.Fatalf("insert ERP customer: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name,active) VALUES('额外人工客户',true) RETURNING id`, schema)).Scan(&extraCustomerID); err != nil {
		t.Fatalf("insert extra customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_portal_profiles(customer_id,display_name,enabled) VALUES($1,'已有人工目标客户',true),($2,'额外人工客户',true)`, schema), erpCustomerID, extraCustomerID); err != nil {
		t.Fatalf("insert customer profiles: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.company_employees(name,phone,account_type,department_id,active)
		VALUES('人工绑定手机号账号','13960000008','channel_customer',(SELECT id FROM %s.company_departments WHERE active=true ORDER BY id LIMIT 1),true)
		RETURNING id
	`, schema, schema)).Scan(&employeeID); err != nil {
		t.Fatalf("insert external employee: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.employee_login_passwords(employee_id,password_hash,login_disabled) VALUES($1,'hash',false)`, schema), employeeID); err != nil {
		t.Fatalf("insert employee login: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_erp_user_bindings(customer_id,employee_id,role,status,updated_by) VALUES($1,$2,'customer','active','test')`, schema), erpCustomerID, employeeID); err != nil {
		t.Fatalf("insert ERP binding: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.mini_users(openid,phone,active) VALUES('openid-manual-phone-login','13960000008',true) RETURNING id`, schema)).Scan(&miniUserID); err != nil {
		t.Fatalf("insert mini user: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_portal_user_bindings(mini_user_id,customer_id,role,status,approved_by)
		VALUES($1,$2,'owner','approved','manual-target'),($1,$3,'member','approved','manual-extra')
	`, schema), miniUserID, erpCustomerID, extraCustomerID); err != nil {
		t.Fatalf("insert manual bindings: %v", err)
	}

	login, err := repo.CreatePhoneVerifiedLoginSession(ctx, customerportalapp.CreatePhoneVerifiedLoginSessionCommand{
		OpenID: "openid-manual-phone-login", Phone: "13960000008", Nickname: "人工绑定手机号账号",
	})
	if err != nil {
		t.Fatalf("phone verified login with manual bindings: %v", err)
	}
	if login.Token == "" || len(login.Bindings) != 2 {
		t.Fatalf("phone login=%+v, want both manual bindings", login)
	}
	rows, err := pool.Query(ctx, fmt.Sprintf(`
		SELECT customer_id,status,approved_by
		FROM %s.customer_portal_user_bindings
		WHERE mini_user_id=$1
		ORDER BY customer_id
	`, schema), miniUserID)
	if err != nil {
		t.Fatalf("load manual bindings after phone login: %v", err)
	}
	defer rows.Close()
	got := map[int64][2]string{}
	for rows.Next() {
		var customerID int64
		var status, approvedBy string
		if err := rows.Scan(&customerID, &status, &approvedBy); err != nil {
			t.Fatalf("scan manual binding: %v", err)
		}
		got[customerID] = [2]string{status, approvedBy}
	}
	if got[erpCustomerID] != [2]string{"approved", "manual-target"} || got[extraCustomerID] != [2]string{"approved", "manual-extra"} {
		t.Fatalf("manual bindings after phone login=%v, want both approved with original sources", got)
	}
}

func TestInternalEmployeeMiniSessionStillChecksLiveRolePermissions(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	ensureCustomerPortalERPBindingTestSchema(t, ctx, pool, schema)
	repo := NewRepository(pool, schema)

	var employeeID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.company_employees(name,phone,account_type,department_id,active)
		VALUES('内部销售员工','13960000004','internal_employee',(SELECT id FROM %s.company_departments WHERE active=true ORDER BY id LIMIT 1),true)
		RETURNING id
	`, schema, schema)).Scan(&employeeID); err != nil {
		t.Fatalf("insert internal employee: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.employee_login_passwords(employee_id,password_hash,login_disabled) VALUES($1,$2,false)`, schema), employeeID, customerPortalTestPasswordHash("secret123")); err != nil {
		t.Fatalf("insert employee password: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.employee_roles(employee_id,role_code) VALUES($1,'sales')`, schema), employeeID); err != nil {
		t.Fatalf("insert employee role: %v", err)
	}
	login, err := repo.CreatePasswordLoginSession(ctx, customerportalapp.CreatePasswordLoginSessionCommand{Login: "13960000004", Password: "secret123"})
	if err != nil {
		t.Fatalf("internal employee login: %v", err)
	}
	if _, err := repo.CurrentContextByToken(ctx, login.Token); err != nil {
		t.Fatalf("internal employee current context before role removal: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.employee_roles WHERE employee_id=$1`, schema), employeeID); err != nil {
		t.Fatalf("remove internal employee role: %v", err)
	}
	if _, err := repo.CurrentContextByToken(ctx, login.Token); !errors.Is(err, customerportalapp.ErrCustomerBindingNotFound) {
		t.Fatalf("internal employee current context after role removal err=%v, want ErrCustomerBindingNotFound", err)
	}
	var expired bool
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT expire_at<=now() FROM %s.mini_sessions WHERE token=$1`, schema), login.Token).Scan(&expired); err != nil {
		t.Fatalf("load internal employee session expiry: %v", err)
	}
	if !expired {
		t.Fatalf("internal employee session remained active after role removal")
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.employee_roles(employee_id,role_code) VALUES($1,'sales')`, schema), employeeID); err != nil {
		t.Fatalf("restore internal employee role: %v", err)
	}
	if _, err := repo.CurrentContextByToken(ctx, login.Token); !errors.Is(err, customerportalapp.ErrMiniSessionNotFound) {
		t.Fatalf("old internal employee token revived after role restore: err=%v", err)
	}
}

func TestInternalEmployeeMiniSessionExpiresWhenLoginDisabled(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	ensureCustomerPortalERPBindingTestSchema(t, ctx, pool, schema)
	repo := NewRepository(pool, schema)

	var employeeID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.company_employees(name,phone,account_type,department_id,active)
		VALUES('停用内部员工','13960000009','internal_employee',(SELECT id FROM %s.company_departments WHERE active=true ORDER BY id LIMIT 1),true)
		RETURNING id
	`, schema, schema)).Scan(&employeeID); err != nil {
		t.Fatalf("insert internal employee: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.employee_login_passwords(employee_id,password_hash,login_disabled) VALUES($1,$2,false)`, schema), employeeID, customerPortalTestPasswordHash("secret123")); err != nil {
		t.Fatalf("insert internal employee password: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.employee_roles(employee_id,role_code) VALUES($1,'sales')`, schema), employeeID); err != nil {
		t.Fatalf("insert internal employee role: %v", err)
	}
	login, err := repo.CreatePasswordLoginSession(ctx, customerportalapp.CreatePasswordLoginSessionCommand{Login: "13960000009", Password: "secret123"})
	if err != nil {
		t.Fatalf("internal employee login: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.employee_login_passwords SET login_disabled=true,updated_at=now() WHERE employee_id=$1`, schema), employeeID); err != nil {
		t.Fatalf("disable internal employee login: %v", err)
	}
	if _, err := repo.CurrentContextByToken(ctx, login.Token); !errors.Is(err, customerportalapp.ErrMiniSessionNotFound) {
		t.Fatalf("disabled internal employee current context err=%v, want ErrMiniSessionNotFound", err)
	}
	var expired bool
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT expire_at<=now() FROM %s.mini_sessions WHERE token=$1`, schema), login.Token).Scan(&expired); err != nil {
		t.Fatalf("load disabled internal employee session expiry: %v", err)
	}
	if !expired {
		t.Fatalf("disabled internal employee session remained active")
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.employee_login_passwords SET login_disabled=false,updated_at=now() WHERE employee_id=$1`, schema), employeeID); err != nil {
		t.Fatalf("re-enable internal employee login: %v", err)
	}
	if _, err := repo.CurrentContextByToken(ctx, login.Token); !errors.Is(err, customerportalapp.ErrMiniSessionNotFound) {
		t.Fatalf("old internal employee token revived after login re-enabled: err=%v", err)
	}
	fresh, err := repo.CreatePasswordLoginSession(ctx, customerportalapp.CreatePasswordLoginSessionCommand{Login: "13960000009", Password: "secret123"})
	if err != nil || fresh.Token == "" || fresh.Token == login.Token {
		t.Fatalf("fresh internal employee login=%+v err=%v, want a new token", fresh, err)
	}
}

func newProjectedPasswordSessionFixture(t *testing.T) *projectedMiniSessionFixture {
	t.Helper()
	f := newProjectedSessionFixture(t, "13960000001")
	login, err := f.repo.CreatePasswordLoginSession(f.ctx, customerportalapp.CreatePasswordLoginSessionCommand{Login: f.login, Password: f.password})
	if err != nil {
		t.Fatalf("create projected password session: %v", err)
	}
	f.token = login.Token
	f.miniUserID = login.MiniUserID
	return f
}

func newProjectedPhoneSessionFixture(t *testing.T) *projectedMiniSessionFixture {
	t.Helper()
	f := newProjectedSessionFixture(t, "13960000005")
	login, err := f.repo.CreatePhoneVerifiedLoginSession(f.ctx, customerportalapp.CreatePhoneVerifiedLoginSessionCommand{
		OpenID: "openid-projected-phone", Phone: f.login, Nickname: "手机号验证客户",
	})
	if err != nil {
		t.Fatalf("create projected phone session: %v", err)
	}
	f.token = login.Token
	f.miniUserID = login.MiniUserID
	return f
}

func newProjectedSessionFixture(t *testing.T, phone string) *projectedMiniSessionFixture {
	t.Helper()
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	ensureCustomerPortalERPBindingTestSchema(t, ctx, pool, schema)
	f := &projectedMiniSessionFixture{ctx: ctx, pool: pool, schema: schema, repo: NewRepository(pool, schema), login: phone, password: "secret123"}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name,active) VALUES('投影会话客户',true) RETURNING id`, schema)).Scan(&f.customerID); err != nil {
		t.Fatalf("insert projected customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_portal_profiles(customer_id,display_name,enabled) VALUES($1,'投影会话客户',true)`, schema), f.customerID); err != nil {
		t.Fatalf("insert projected customer profile: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.company_employees(name,phone,account_type,department_id,active)
		VALUES('投影渠道账号',$1,'channel_customer',(SELECT id FROM %s.company_departments WHERE active=true ORDER BY id LIMIT 1),true)
		RETURNING id
	`, schema, schema), phone).Scan(&f.employeeID); err != nil {
		t.Fatalf("insert projected employee: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.employee_login_passwords(employee_id,password_hash,login_disabled) VALUES($1,$2,false)`, schema), f.employeeID, customerPortalTestPasswordHash(f.password)); err != nil {
		t.Fatalf("insert projected password: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_erp_user_bindings(customer_id,employee_id,role,status,updated_by) VALUES($1,$2,'customer','active','test')`, schema), f.customerID, f.employeeID); err != nil {
		t.Fatalf("insert projected ERP binding: %v", err)
	}
	return f
}

func assertProjectedSessionRevoked(t *testing.T, f *projectedMiniSessionFixture, approvedBy string) {
	t.Helper()
	if _, err := f.repo.CurrentContextByToken(f.ctx, f.token); !errors.Is(err, customerportalapp.ErrMiniSessionNotFound) {
		t.Fatalf("CurrentContextByToken after projected access invalidated err=%v, want ErrMiniSessionNotFound", err)
	}
	var bindingStatus string
	if err := f.pool.QueryRow(f.ctx, fmt.Sprintf(`
		SELECT status FROM %s.customer_portal_user_bindings
		WHERE mini_user_id=$1 AND customer_id=$2 AND approved_by LIKE $3
	`, f.schema), f.miniUserID, f.customerID, approvedBy+"%").Scan(&bindingStatus); err != nil {
		t.Fatalf("load projected binding status: %v", err)
	}
	if bindingStatus != "revoked" {
		t.Fatalf("projected binding status=%q, want revoked", bindingStatus)
	}
	var sessionCount int
	if err := f.pool.QueryRow(f.ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.mini_sessions WHERE token=$1 AND expire_at>now()`, f.schema), f.token).Scan(&sessionCount); err != nil {
		t.Fatalf("count projected mini session: %v", err)
	}
	if sessionCount != 0 {
		t.Fatalf("projected mini session count=%d, want deleted", sessionCount)
	}
}

func execProjectedSessionSQL(t *testing.T, f *projectedMiniSessionFixture, query string, args ...any) {
	t.Helper()
	if _, err := f.pool.Exec(f.ctx, fmt.Sprintf(query, f.schema), args...); err != nil {
		t.Fatalf("exec projected session SQL: %v", err)
	}
}
