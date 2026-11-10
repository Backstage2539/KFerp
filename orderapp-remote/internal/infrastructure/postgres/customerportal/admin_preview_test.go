package customerportal

import (
	"context"
	"fmt"
	"testing"
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
	if err = repo.AdminPreviewAudit(ctx, 15, a, "enter"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE actor='employee:15' AND entity_type='customer_preview' AND entity_id=$1 AND action='enter' AND meta->>'employee_id'='15' AND meta->>'customer_id'=$2`, schema), a, fmt.Sprint(a)).Scan(&count); err != nil || count != 1 {
		t.Fatalf("audit count=%d err=%v", count, err)
	}
}
