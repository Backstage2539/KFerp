package customerportal

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	customerportalapp "orderapp/internal/application/customerportal"

	"github.com/jackc/pgx/v5/pgxpool"
)

func recipientAddressPostgres(t *testing.T) (Repository, context.Context) {
	t.Helper()
	if os.Getenv("KF_RUN_POSTGRES_INTEGRATION") != "1" {
		t.Skip("disposable PostgreSQL integration")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("pr664_address_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); pool.Close() })
	ddl := fmt.Sprintf(`
		CREATE TABLE %[1]s.customers(id BIGINT PRIMARY KEY);
		CREATE TABLE %[1]s.customer_recipient_addresses(
			id BIGSERIAL PRIMARY KEY,customer_id BIGINT NOT NULL REFERENCES %[1]s.customers(id),recipient_name TEXT NOT NULL,phone TEXT NOT NULL,
			company TEXT NOT NULL DEFAULT '',province TEXT NOT NULL DEFAULT '',city TEXT NOT NULL DEFAULT '',district TEXT NOT NULL DEFAULT '',detail_address TEXT NOT NULL,
			is_default BOOLEAN NOT NULL DEFAULT false,active BOOLEAN NOT NULL DEFAULT true,revision BIGINT NOT NULL DEFAULT 1,
			created_by TEXT NOT NULL DEFAULT '',updated_by TEXT NOT NULL DEFAULT '',created_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE UNIQUE INDEX customer_recipient_addresses_default_uq ON %[1]s.customer_recipient_addresses(customer_id) WHERE active=true AND is_default=true;
		CREATE TABLE %[1]s.audit_logs(id BIGSERIAL PRIMARY KEY,actor TEXT,entity_type TEXT,entity_id BIGINT,action TEXT,field TEXT,old_value TEXT,new_value TEXT,meta JSONB);
	`, schema)
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	return NewRepository(pool, schema), ctx
}

func TestRecipientAddressDefaultConcurrencyAndCustomerIsolation(t *testing.T) {
	r, ctx := recipientAddressPostgres(t)
	if _, err := r.pool.Exec(ctx, `INSERT INTO `+r.schema+`.customers(id) VALUES(71),(72)`); err != nil {
		t.Fatal(err)
	}
	first, err := r.SaveCustomerRecipientAddress(ctx, customerportalapp.SaveCustomerRecipientAddressCommand{
		CustomerID: 71, RecipientName: "门店甲", Phone: "13800138000", Province: "云南省", City: "昆明市", DetailAddress: "咖啡路 1 号", Actor: "mini-user:1",
	})
	if err != nil || !first.IsDefault {
		t.Fatalf("first address=%+v err=%v", first, err)
	}
	second, err := r.SaveCustomerRecipientAddress(ctx, customerportalapp.SaveCustomerRecipientAddressCommand{
		CustomerID: 71, RecipientName: "门店乙", Phone: "13800138001", Province: "云南省", City: "普洱市", DetailAddress: "咖啡路 2 号", IsDefault: true, Actor: "admin",
	})
	if err != nil || !second.IsDefault {
		t.Fatalf("second address=%+v err=%v", second, err)
	}
	rows, err := r.ListCustomerRecipientAddresses(ctx, 71, "门店")
	if err != nil || len(rows) != 2 || rows[0].ID != second.ID || !rows[0].IsDefault || rows[1].IsDefault {
		t.Fatalf("address list=%+v err=%v", rows, err)
	}
	if _, err := r.SaveCustomerRecipientAddress(ctx, customerportalapp.SaveCustomerRecipientAddressCommand{
		ID: first.ID, CustomerID: 72, RecipientName: "越权", Phone: "13800138009", DetailAddress: "其他地址", ExpectedRevision: first.Revision, Actor: "admin",
	}); err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("cross-customer update error=%v", err)
	}
	if _, err := r.SaveCustomerRecipientAddress(ctx, customerportalapp.SaveCustomerRecipientAddressCommand{
		ID: first.ID, CustomerID: 71, RecipientName: "缺版本", Phone: first.Phone, DetailAddress: first.DetailAddress, Actor: "admin",
	}); err == nil || !strings.Contains(err.Error(), "expected_revision") {
		t.Fatalf("missing revision update error=%v", err)
	}
	if _, err := r.SaveCustomerRecipientAddress(ctx, customerportalapp.SaveCustomerRecipientAddressCommand{
		ID: first.ID, CustomerID: 71, RecipientName: "旧修改", Phone: first.Phone, DetailAddress: first.DetailAddress, ExpectedRevision: first.Revision, Actor: "admin",
	}); err == nil || !strings.Contains(err.Error(), "已变化") {
		t.Fatalf("stale revision update error=%v", err)
	}
	if err := r.DeleteCustomerRecipientAddress(ctx, customerportalapp.DeleteCustomerRecipientAddressCommand{
		ID: second.ID, CustomerID: 71, ExpectedRevision: second.Revision, Actor: "admin",
	}); err != nil {
		t.Fatal(err)
	}
	rows, err = r.ListCustomerRecipientAddresses(ctx, 71, "")
	if err != nil || len(rows) != 1 || rows[0].ID != first.ID || !rows[0].IsDefault {
		t.Fatalf("default promotion rows=%+v err=%v", rows, err)
	}
	var audits int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM `+r.schema+`.audit_logs WHERE entity_type='customer_recipient_address'`).Scan(&audits); err != nil || audits != 3 {
		t.Fatalf("address audits=%d err=%v", audits, err)
	}
}
