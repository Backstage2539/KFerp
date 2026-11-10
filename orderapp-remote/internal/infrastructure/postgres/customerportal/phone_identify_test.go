package customerportal

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	companyapp "orderapp/internal/application/company"
	app "orderapp/internal/application/customerportal"
	companyrepo "orderapp/internal/infrastructure/postgres/company"
)

func TestPhoneIdentifyGuestAndEmployee(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	ensureCustomerPortalERPBindingTestSchema(t, ctx, pool, schema)
	repo := NewRepository(pool, schema)
	cmd := app.CreatePhoneVerifiedLoginSessionCommand{Identify: true, Registration: true, CredentialHash: strings.Repeat("a", 64), OpenID: "verified-guest", Phone: "13960009999"}
	guest, err := repo.CreatePhoneVerifiedLoginSession(ctx, cmd)
	if err != nil || guest.Token == "" || !guest.RegistrationComplete || len(guest.Bindings) != 0 {
		t.Fatalf("verified guest: %+v, %v", guest, err)
	}
	legacyGuest := app.CreatePhoneVerifiedLoginSessionCommand{CredentialHash: strings.Repeat("7", 64), OpenID: "unregistered-wx", Phone: "13960009999"}
	if result, err := repo.CreatePhoneVerifiedLoginSession(ctx, legacyGuest); !errors.Is(err, app.ErrMiniInvalidLogin) || result.Token != "" {
		t.Fatalf("legacy phone_verify unexpectedly logged in guest: %+v %v", result, err)
	}
	if _, err = repo.CreatePhoneVerifiedLoginSession(ctx, cmd); !errors.Is(err, app.ErrPhoneCredentialUsed) {
		t.Fatalf("replay: %v", err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.mini_registrations SET nickname='已有昵称' WHERE mini_user_id=$1`, schema), guest.MiniUserID); err != nil {
		t.Fatal(err)
	}
	cmd.CredentialHash = strings.Repeat("c", 64)
	guest, err = repo.CreatePhoneVerifiedLoginSession(ctx, cmd)
	if err != nil || guest.Registration == nil || guest.Registration.Nickname != "已有昵称" {
		t.Fatalf("existing nickname changed: %+v %v", guest.Registration, err)
	}
	var id int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.company_employees(name,phone,account_type,department_id,active) VALUES('staff','13960009998','internal_employee',(SELECT id FROM %s.company_departments WHERE active LIMIT 1),true) RETURNING id`, schema, schema)).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.employee_roles(employee_id,role_code) VALUES($1,'sales')`, schema), id); err != nil {
		t.Fatal(err)
	}
	cmd = app.CreatePhoneVerifiedLoginSessionCommand{Identify: true, Registration: true, CredentialHash: strings.Repeat("b", 64), OpenID: "verified-staff", Phone: "13960009998"}
	staff, err := repo.CreatePhoneVerifiedLoginSession(ctx, cmd)
	if err != nil || staff.AccountType != "employee" || staff.EmployeeID != id || staff.Token == "" {
		t.Fatalf("verified staff: %+v, %v", staff, err)
	}
	if _, err = repo.CurrentContextByToken(ctx, staff.Token); err != nil {
		t.Fatalf("staff context: %v", err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.company_employees SET phone='13960009997' WHERE id=$1`, schema), id); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.CurrentContextByToken(ctx, staff.Token); !errors.Is(err, app.ErrMiniSessionNotFound) {
		t.Fatalf("changed phone session accepted: %v", err)
	}
}

func TestPhoneIdentifyStaffRevocationAndLegacyMode(t *testing.T) {
	for _, tc := range []struct {
		name, change string
		want         error
	}{
		{"disabled", `INSERT INTO %s.employee_login_passwords(employee_id,password_hash,login_disabled) VALUES($1,'',true)`, app.ErrMiniSessionNotFound},
		{"inactive", `UPDATE %s.company_employees SET active=false WHERE id=$1`, app.ErrMiniSessionNotFound},
		{"role_removed", `DELETE FROM %s.employee_roles WHERE employee_id=$1`, app.ErrCustomerBindingNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			pool, schema := newCustomerPortalTestDB(t)
			ensureCustomerPortalERPBindingTestSchema(t, ctx, pool, schema)
			repo := NewRepository(pool, schema)
			var id int64
			if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.company_employees(name,phone,account_type,department_id,active) VALUES('staff','13960009998','internal_employee',(SELECT id FROM %s.company_departments WHERE active LIMIT 1),true) RETURNING id`, schema, schema)).Scan(&id); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.employee_roles(employee_id,role_code) VALUES($1,'sales')`, schema), id); err != nil {
				t.Fatal(err)
			}
			cmd := app.CreatePhoneVerifiedLoginSessionCommand{CredentialHash: strings.Repeat("d", 64), OpenID: "wx-staff", Phone: "13960009998"}
			staff, err := repo.CreatePhoneVerifiedLoginSession(ctx, cmd)
			if err != nil || staff.AccountType != "employee" {
				t.Fatalf("legacy phone_verify employee: %+v %v", staff, err)
			}
			if _, err = repo.CreatePhoneVerifiedLoginSession(ctx, cmd); !errors.Is(err, app.ErrPhoneCredentialUsed) {
				t.Fatalf("staff code replay: %v", err)
			}
			if _, err = pool.Exec(ctx, fmt.Sprintf(tc.change, schema), id); err != nil {
				t.Fatal(err)
			}
			if _, err = repo.CurrentContextByToken(ctx, staff.Token); !errors.Is(err, tc.want) {
				t.Fatalf("revoked staff: %v", err)
			}
			var expired bool
			if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT expire_at<=now() FROM %s.mini_sessions WHERE token=$1`, schema), staff.Token).Scan(&expired); err != nil || !expired {
				t.Fatalf("old token remains active: %v", err)
			}
		})
	}
}

func TestPhoneIdentifyAmbiguousAccountsNeverGrantsAccess(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	ensureCustomerPortalERPBindingTestSchema(t, ctx, pool, schema)
	repo := NewRepository(pool, schema)
	// Model a legacy database where the unique phone index is absent.
	if _, err := pool.Exec(ctx, fmt.Sprintf(`DROP INDEX %s.company_employees_phone_uq`, schema)); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"internal_employee", "channel_customer"} {
		if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.company_employees(name,phone,account_type,department_id,active) VALUES('duplicate','13960009998',$1,(SELECT id FROM %s.company_departments WHERE active LIMIT 1),true)`, schema, schema), kind); err != nil {
			t.Fatal(err)
		}
	}
	cmd := app.CreatePhoneVerifiedLoginSessionCommand{Identify: true, Registration: true, CredentialHash: strings.Repeat("e", 64), OpenID: "wx-ambiguous", Phone: "13960009998"}
	if result, err := repo.CreatePhoneVerifiedLoginSession(ctx, cmd); !errors.Is(err, app.ErrMiniInvalidLogin) || result.Token != "" {
		t.Fatalf("ambiguous phone granted session: %+v %v", result, err)
	}
}

func TestWechatEmployeePhoneChangePermanentlyExpiresSession(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	ensureCustomerPortalERPBindingTestSchema(t, ctx, pool, schema)
	repo := NewRepository(pool, schema)
	var id, departmentID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.company_employees(name,phone,account_type,department_id,active) VALUES('staff','13960009998','internal_employee',(SELECT id FROM %s.company_departments WHERE active LIMIT 1),true) RETURNING id,department_id`, schema, schema)).Scan(&id, &departmentID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.employee_roles(employee_id,role_code) VALUES($1,'sales')`, schema), id); err != nil {
		t.Fatal(err)
	}
	cmd := app.CreatePhoneVerifiedLoginSessionCommand{Identify: true, Registration: true, CredentialHash: strings.Repeat("f", 64), OpenID: "wx-staff", Phone: "13960009998"}
	staff, err := repo.CreatePhoneVerifiedLoginSession(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	company := companyrepo.NewRepository(pool, schema)
	change := companyapp.EmployeeCommand{Name: "staff", Phone: "13960009997", DepartmentID: departmentID, Active: true}
	if err = company.UpdateEmployee(ctx, id, change); err != nil {
		t.Fatal(err)
	}
	change.Phone = "13960009998"
	if err = company.UpdateEmployee(ctx, id, change); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.CurrentContextByToken(ctx, staff.Token); !errors.Is(err, app.ErrMiniSessionNotFound) {
		t.Fatalf("old session revived after phone changed away and back: %v", err)
	}
}

func TestPhoneIdentifyCustomerWithoutPassword(t *testing.T) {
	f := newProjectedSessionFixture(t, "13960009995")
	if _, err := f.pool.Exec(f.ctx, fmt.Sprintf(`DELETE FROM %s.employee_login_passwords WHERE employee_id=$1`, f.schema), f.employeeID); err != nil {
		t.Fatal(err)
	}
	cmd := app.CreatePhoneVerifiedLoginSessionCommand{Identify: true, Registration: true, CredentialHash: strings.Repeat("9", 64), OpenID: "wx-customer", Phone: f.login}
	result, err := f.repo.CreatePhoneVerifiedLoginSession(f.ctx, cmd)
	if err != nil || result.CurrentCustomerID != f.customerID || result.Token == "" {
		t.Fatalf("customer identification: %+v %v", result, err)
	}
	current, err := f.repo.CurrentContextByToken(f.ctx, result.Token)
	if err != nil || current.CurrentCustomerID != f.customerID {
		t.Fatalf("customer context: %+v %v", current, err)
	}
}
