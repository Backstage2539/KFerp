package customerportal

import (
	"context"
	"fmt"
	"testing"

	customerportalapp "orderapp/internal/application/customerportal"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProjectedMiniSessionsSurviveBenignProfileEdits(t *testing.T) {
	loginTypes := []struct {
		name    string
		fixture func(*testing.T) *projectedMiniSessionFixture
	}{
		{name: "password", fixture: newProjectedPasswordSessionFixture},
		{name: "phone_verify", fixture: newProjectedPhoneSessionFixture},
	}
	mutations := []struct {
		name   string
		mutate func(*testing.T, *projectedMiniSessionFixture)
	}{
		{
			name: "customer_contact_and_address",
			mutate: func(t *testing.T, f *projectedMiniSessionFixture) {
				execProjectedSessionSQL(t, f, `
					UPDATE %s.customers
					SET contact='新联系人',phone='021-88886666',address='新的收货地址',updated_at=now()
					WHERE id=$1
				`, f.customerID)
			},
		},
		{
			name: "portal_display_theme_and_capabilities",
			mutate: func(t *testing.T, f *projectedMiniSessionFixture) {
				execProjectedSessionSQL(t, f, `
					INSERT INTO %s.customer_capability_templates(template_key,label,theme_key,miniapp_entry_mode,capabilities_json,active)
					VALUES('benign_profile_template','普通能力配置','clean_ops','services','[]'::jsonb,true)
				`)
				execProjectedSessionSQL(t, f, `
					UPDATE %s.customer_portal_profiles
					SET display_name='新的门户显示名',theme_key='clean_ops',capability_template_key='benign_profile_template',updated_at=now()
					WHERE customer_id=$1 AND enabled=true
				`, f.customerID)
			},
		},
		{
			name: "employee_name_department_and_phone",
			mutate: func(t *testing.T, f *projectedMiniSessionFixture) {
				var departmentID int64
				if err := f.pool.QueryRow(f.ctx, fmt.Sprintf(`
					INSERT INTO %s.company_departments(name,active)
					VALUES('渠道服务部',true)
					RETURNING id
				`, f.schema)).Scan(&departmentID); err != nil {
					t.Fatalf("insert benign employee department: %v", err)
				}
				execProjectedSessionSQL(t, f, `
					UPDATE %s.company_employees
					SET name='渠道账号新名称',phone='13961112222',department_id=$2,updated_at=now()
					WHERE id=$1 AND active=true
				`, f.employeeID, departmentID)
			},
		},
	}

	for _, loginType := range loginTypes {
		for _, mutation := range mutations {
			t.Run(loginType.name+"/"+mutation.name, func(t *testing.T) {
				f := loginType.fixture(t)
				mutation.mutate(t, f)
				current, err := f.repo.CurrentContextByToken(f.ctx, f.token)
				if err != nil {
					t.Fatalf("CurrentContextByToken after benign edit: %v", err)
				}
				if current.CurrentCustomerID != f.customerID {
					t.Fatalf("current customer=%d, want %d", current.CurrentCustomerID, f.customerID)
				}
				assertMiniSessionStillActive(t, f.ctx, f.pool, f.schema, f.token)
			})
		}
	}
}

func TestSwitchCurrentCustomerSurvivesBenignProjectedProfileEdits(t *testing.T) {
	for _, loginType := range []struct {
		name    string
		fixture func(*testing.T) *projectedMiniSessionFixture
	}{
		{name: "password", fixture: newProjectedPasswordSessionFixture},
		{name: "phone_verify", fixture: newProjectedPhoneSessionFixture},
	} {
		t.Run(loginType.name, func(t *testing.T) {
			f := loginType.fixture(t)
			var departmentID int64
			if err := f.pool.QueryRow(f.ctx, fmt.Sprintf(`
				INSERT INTO %s.company_departments(name,active)
				VALUES('渠道体验部',true)
				RETURNING id
			`, f.schema)).Scan(&departmentID); err != nil {
				t.Fatalf("insert benign switch department: %v", err)
			}
			execProjectedSessionSQL(t, f, `UPDATE %s.customers SET contact='切换联系人',address='切换地址',updated_at=now() WHERE id=$1`, f.customerID)
			execProjectedSessionSQL(t, f, `UPDATE %s.customer_portal_profiles SET display_name='切换显示名',theme_key='clean_ops',updated_at=now() WHERE customer_id=$1 AND enabled=true`, f.customerID)
			execProjectedSessionSQL(t, f, `UPDATE %s.company_employees SET name='切换账号新名称',phone='13962223333',department_id=$2,updated_at=now() WHERE id=$1 AND active=true`, f.employeeID, departmentID)

			current, err := f.repo.SwitchCurrentCustomer(f.ctx, f.token, f.customerID)
			if err != nil {
				t.Fatalf("SwitchCurrentCustomer after benign edits: %v", err)
			}
			if current.CurrentCustomerID != f.customerID {
				t.Fatalf("current customer=%d, want %d", current.CurrentCustomerID, f.customerID)
			}
			assertMiniSessionStillActive(t, f.ctx, f.pool, f.schema, f.token)
		})
	}
}

func TestManualMiniSessionSurvivesBenignCustomerAndPortalEdits(t *testing.T) {
	f := newManualMiniSessionFixture(t)
	execProjectedSessionSQL(t, f, `UPDATE %s.customers SET contact='人工联系人',phone='021-60000000',address='人工绑定新地址',updated_at=now() WHERE id=$1`, f.customerID)
	execProjectedSessionSQL(t, f, `UPDATE %s.customer_portal_profiles SET display_name='人工门户新名称',theme_key='clean_ops',updated_at=now() WHERE customer_id=$1 AND enabled=true`, f.customerID)
	execProjectedSessionSQL(t, f, `
		INSERT INTO %s.customer_service_capabilities(customer_id,capability_code,enabled,config_json,updated_at)
		VALUES($1,'orders',true,'{"mode":"updated"}'::jsonb,now())
	`, f.customerID)

	switched, err := f.repo.SwitchCurrentCustomer(f.ctx, f.token, f.customerID)
	if err != nil {
		t.Fatalf("manual SwitchCurrentCustomer after benign edits: %v", err)
	}
	if switched.CurrentCustomerID != f.customerID {
		t.Fatalf("manual switched customer=%d, want %d", switched.CurrentCustomerID, f.customerID)
	}
	current, err := f.repo.CurrentContextByToken(f.ctx, f.token)
	if err != nil {
		t.Fatalf("manual CurrentContextByToken after benign edits: %v", err)
	}
	if current.CurrentCustomerID != f.customerID {
		t.Fatalf("manual current customer=%d, want %d", current.CurrentCustomerID, f.customerID)
	}
	assertMiniSessionStillActive(t, f.ctx, f.pool, f.schema, f.token)
}

func TestInternalEmployeeMiniSessionSurvivesBenignEmployeeProfileEdits(t *testing.T) {
	f := newInternalMiniSessionFixture(t)
	var departmentID int64
	if err := f.pool.QueryRow(f.ctx, fmt.Sprintf(`
		INSERT INTO %s.company_departments(name,active)
		VALUES('内部服务部',true)
		RETURNING id
	`, f.schema)).Scan(&departmentID); err != nil {
		t.Fatalf("insert benign internal department: %v", err)
	}
	execProjectedSessionSQL(t, f, `
		UPDATE %s.company_employees
		SET name='内部员工新名称',phone='13963334444',department_id=$2,updated_at=now()
		WHERE id=$1 AND active=true AND account_type='internal_employee'
	`, f.employeeID, departmentID)

	current, err := f.repo.CurrentContextByToken(f.ctx, f.token)
	if err != nil {
		t.Fatalf("internal CurrentContextByToken after benign employee edit: %v", err)
	}
	if current.EmployeeID != f.employeeID || current.EmployeeName != "内部员工新名称" {
		t.Fatalf("internal context employee=%d/%q, want %d/%q", current.EmployeeID, current.EmployeeName, f.employeeID, "内部员工新名称")
	}
	assertMiniSessionStillActive(t, f.ctx, f.pool, f.schema, f.token)
}

func newManualMiniSessionFixture(t *testing.T) *projectedMiniSessionFixture {
	t.Helper()
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	f := &projectedMiniSessionFixture{ctx: ctx, pool: pool, schema: schema, repo: NewRepository(pool, schema), token: "manual-benign-token"}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name,active) VALUES('人工会话客户',true) RETURNING id`, schema)).Scan(&f.customerID); err != nil {
		t.Fatalf("insert manual session customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_portal_profiles(customer_id,display_name,enabled) VALUES($1,'人工会话客户',true)`, schema), f.customerID); err != nil {
		t.Fatalf("insert manual portal profile: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.mini_users(openid,active) VALUES('openid-manual-benign',true) RETURNING id`, schema)).Scan(&f.miniUserID); err != nil {
		t.Fatalf("insert manual mini user: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_portal_user_bindings(mini_user_id,customer_id,role,status,approved_by) VALUES($1,$2,'owner','approved','manual-admin')`, schema), f.miniUserID, f.customerID); err != nil {
		t.Fatalf("insert manual portal binding: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.mini_sessions(token,mini_user_id,current_customer_id,expire_at) VALUES($1,$2,$3,'infinity')`, schema), f.token, f.miniUserID, f.customerID); err != nil {
		t.Fatalf("insert manual mini session: %v", err)
	}
	return f
}

func newInternalMiniSessionFixture(t *testing.T) *projectedMiniSessionFixture {
	t.Helper()
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	ensureCustomerPortalERPBindingTestSchema(t, ctx, pool, schema)
	f := &projectedMiniSessionFixture{ctx: ctx, pool: pool, schema: schema, repo: NewRepository(pool, schema), login: "13963330000", password: "secret123"}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.company_employees(name,phone,account_type,department_id,active)
		VALUES('内部普通资料员工',$1,'internal_employee',(SELECT id FROM %s.company_departments WHERE active=true ORDER BY id LIMIT 1),true)
		RETURNING id
	`, schema, schema), f.login).Scan(&f.employeeID); err != nil {
		t.Fatalf("insert benign internal employee: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.employee_login_passwords(employee_id,password_hash,login_disabled) VALUES($1,$2,false)`, schema), f.employeeID, customerPortalTestPasswordHash(f.password)); err != nil {
		t.Fatalf("insert benign internal password: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.employee_roles(employee_id,role_code) VALUES($1,'sales')`, schema), f.employeeID); err != nil {
		t.Fatalf("insert benign internal role: %v", err)
	}
	login, err := f.repo.CreatePasswordLoginSession(ctx, customerportalapp.CreatePasswordLoginSessionCommand{Login: f.login, Password: f.password})
	if err != nil {
		t.Fatalf("create benign internal mini session: %v", err)
	}
	f.token = login.Token
	f.miniUserID = login.MiniUserID
	return f
}

func assertMiniSessionStillActive(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema, token string) {
	t.Helper()
	var active bool
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT expire_at>now() FROM %s.mini_sessions WHERE token=$1`, schema), token).Scan(&active); err != nil {
		t.Fatalf("load mini session activity: %v", err)
	}
	if !active {
		t.Fatalf("mini session %q expired after benign profile edit", token)
	}
}
