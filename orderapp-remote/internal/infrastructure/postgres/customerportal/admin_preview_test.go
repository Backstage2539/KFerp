package customerportal

import (
	"context"
	"fmt"
	"strings"
	"testing"

	app "orderapp/internal/application/customerportal"
)

func TestAdminPreviewCustomerContextAndAudit(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.company_employees (id bigint primary key, name text, phone text)`, schema)); err != nil {
		t.Fatal(err)
	}
	var a, b, inactive int64
	for _, c := range []struct {
		name   string
		active bool
		id     *int64
	}{{"甲", true, &a}, {"乙", true, &b}, {"停用", false, &inactive}} {
		if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name,active) VALUES($1,$2) RETURNING id`, schema), c.name, c.active).Scan(c.id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_portal_profiles(customer_id,enabled) VALUES($1,true),($2,true),($3,true)`, schema), a, b, inactive); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_service_capabilities(customer_id,capability_code,enabled,config_json) VALUES($1,'product_order',true,'{}')`, schema), a); err != nil {
		t.Fatal(err)
	}
	ca, err := repo.AdminPreviewCustomer(ctx, a)
	if err != nil || ca.CurrentCustomerID != a || ca.CurrentCustomerName != "甲" || !ca.HasCapability("product_order") {
		t.Fatalf("customer A: %+v %v", ca, err)
	}
	cb, err := repo.AdminPreviewCustomer(ctx, b)
	if err != nil || cb.CurrentCustomerID != b || cb.HasCapability("product_order") {
		t.Fatalf("customer B: %+v %v", cb, err)
	}
	if _, err = repo.AdminPreviewCustomer(ctx, inactive); err == nil {
		t.Fatal("inactive customer accepted")
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_portal_profiles SET enabled=false WHERE customer_id=$1`, schema), b); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.AdminPreviewCustomer(ctx, b); err == nil {
		t.Fatal("disabled customer portal accepted")
	}
	if err = repo.AdminPreviewAudit(ctx, 15, a, "enter"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE actor='employee:15' AND entity_type='customer_preview' AND entity_id=$1 AND action='enter' AND meta->>'employee_id'='15' AND meta->>'customer_id'=$2`, schema), a, fmt.Sprint(a)).Scan(&count); err != nil || count != 1 {
		t.Fatalf("audit count=%d err=%v", count, err)
	}
}

func TestWechatAdminPreviewSecurity(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	ensureCustomerPortalERPBindingTestSchema(t, ctx, pool, schema)
	repo := NewRepository(pool, schema)
	var employeeID, customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.company_employees(name,phone,account_type,department_id,active) VALUES('admin','13960009998','internal_employee',(SELECT id FROM %s.company_departments WHERE active LIMIT 1),true) RETURNING id`, schema, schema)).Scan(&employeeID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.employee_roles(employee_id,role_code) VALUES($1,'admin')`, schema), employeeID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name) VALUES('客户甲') RETURNING id`, schema)).Scan(&customerID); err != nil {
		t.Fatal(err)
	}
	seedEnabledCustomerPortalProfiles(t, ctx, pool, schema, customerID)
	login, err := repo.CreatePhoneVerifiedLoginSession(ctx, app.CreatePhoneVerifiedLoginSessionCommand{Identify: true, Registration: true, CredentialHash: strings.Repeat("d", 64), OpenID: "verified-wx", Phone: "13960009998"})
	if err != nil || login.AccountType != "employee" {
		t.Fatalf("wechat login: %+v %v", login, err)
	}
	svc := app.NewService(repo, nil)
	preview, err := svc.StartAdminPreview(ctx, login.Token, customerID)
	if err != nil {
		t.Fatal(err)
	}
	current, err := svc.Me(ctx, preview.Token)
	if err != nil || current.AccountType != "preview" || current.MiniUserID != 0 || current.CurrentCustomerID != customerID || len(current.Bindings) != 0 {
		t.Fatalf("preview context: %+v %v", current, err)
	}
	if err = svc.EndAdminPreview(ctx, preview.Token); err != nil {
		t.Fatal(err)
	}
	if current, err = svc.Me(ctx, login.Token); err != nil || current.AccountType != "employee" {
		t.Fatalf("employee session changed: %+v %v", current, err)
	}
	preview, err = svc.StartAdminPreview(ctx, login.Token, customerID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_portal_profiles SET enabled=false WHERE customer_id=$1`, schema), customerID); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Me(ctx, preview.Token); err == nil {
		t.Fatal("disabled customer portal remained accessible")
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_portal_profiles SET enabled=true WHERE customer_id=$1`, schema), customerID); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Me(ctx, preview.Token); err == nil {
		t.Fatal("preview resumed after customer portal was disabled")
	}
	preview, err = svc.StartAdminPreview(ctx, login.Token, customerID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.employee_roles WHERE employee_id=$1 AND role_code='admin'`, schema), employeeID); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Me(ctx, preview.Token); err == nil {
		t.Fatal("revoked admin retained preview access")
	}
}
