package sales

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestLogOrderSaveTxPersistsMiniActorAndSummaryAtomically(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	defer pool.Close()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s.order_audit_logs (
			id BIGSERIAL PRIMARY KEY,
			order_id BIGINT NOT NULL,
			actor TEXT NOT NULL,
			field TEXT NOT NULL,
			old_value TEXT,
			new_value TEXT
		);
		CREATE TABLE %s.audit_logs (
			id BIGSERIAL PRIMARY KEY,
			actor TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			entity_id BIGINT,
			action TEXT NOT NULL,
			field TEXT,
			old_value TEXT,
			new_value TEXT,
			meta JSONB
		)
	`, schema, schema)); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	beforeSummary := `{"notes":"修改前","receiver_name":"旧收件人","item_count":1}`
	afterSummary := `{"notes":"修改后","receiver_name":"新收件人","item_count":1}`
	if err := repo.logOrderSaveTx(ctx, tx, "mini-employee:7:销售甲", 42, "SO-42", true, beforeSummary, afterSummary, 901, "V9.1"); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	var actor, field, oldValue, newValue string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT actor,field,old_value,new_value FROM %s.order_audit_logs WHERE order_id=42`, schema)).Scan(&actor, &field, &oldValue, &newValue); err != nil {
		t.Fatal(err)
	}
	if actor != "mini-employee:7:销售甲" || field != "order" || oldValue != beforeSummary || newValue != afterSummary {
		t.Fatalf("order audit actor=%q field=%q old=%q new=%q", actor, field, oldValue, newValue)
	}
	var action, auditActor, auditOldValue, auditNewValue, meta string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT action,actor,old_value,new_value,meta::text FROM %s.audit_logs WHERE entity_id=42`, schema)).Scan(&action, &auditActor, &auditOldValue, &auditNewValue, &meta); err != nil {
		t.Fatal(err)
	}
	if action != "update" || auditActor != actor || auditOldValue != beforeSummary || auditNewValue != afterSummary || !strings.Contains(meta, `"bean_list_publication_id": 901`) {
		t.Fatalf("operation audit action=%q actor=%q old=%q new=%q meta=%q", action, auditActor, auditOldValue, auditNewValue, meta)
	}
}

func TestOrderSaveAuditSummaryCapturesEditableHeaderReceiverPricingAndItems(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	defer pool.Close()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s.orders (
			id BIGINT PRIMARY KEY, customer_id BIGINT, order_date DATE, notes TEXT,
			receiver_name TEXT, receiver_phone TEXT, receiver_address TEXT, receiver_company TEXT,
			shipping_amount NUMERIC, discount_amount NUMERIC
		);
		CREATE TABLE %s.order_items (
			id BIGINT PRIMARY KEY, order_id BIGINT, line_no INTEGER, product_id BIGINT, product_kind TEXT,
			item_name TEXT, item_note TEXT, qty NUMERIC, unit TEXT, spec TEXT, sales_unit TEXT,
			unit_price NUMERIC, line_total NUMERIC, discount_type TEXT, discount_value NUMERIC,
			discount_amount NUMERIC, bean_list_publication_id BIGINT, bean_list_version_no TEXT
		);
		INSERT INTO %s.orders VALUES(42,8,'2026-08-01','旧备注','旧收件人','13800000000','旧地址','旧公司',15,12);
		INSERT INTO %s.order_items VALUES(51,42,1,551,'roasted_bean','红岩','行备注',2,'袋','227g','bag',68,134,'amount',1,2,901,'V9.1');
	`, schema, schema, schema, schema)); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	before, err := loadOrderSaveAuditSummaryTx(ctx, tx, schema, 42)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		DELETE FROM %s.order_items WHERE id=51;
		INSERT INTO %s.order_items VALUES(52,42,1,551,'roasted_bean','红岩','行备注',2,'袋','227g','bag',68,134,'amount',1,2,901,'V9.1');
	`, schema, schema)); err != nil {
		t.Fatal(err)
	}
	reinserted, err := loadOrderSaveAuditSummaryTx(ctx, tx, schema, 42)
	if err != nil {
		t.Fatal(err)
	}
	if reinserted != before {
		t.Fatalf("database-only item id replacement changed business audit summary: before=%s after=%s", before, reinserted)
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET receiver_name='新收件人' WHERE id=42`, schema)); err != nil {
		t.Fatal(err)
	}
	receiverOnly, err := loadOrderSaveAuditSummaryTx(ctx, tx, schema, 42)
	if err != nil {
		t.Fatal(err)
	}
	if before == receiverOnly || !strings.Contains(receiverOnly, "新收件人") {
		t.Fatalf("receiver-only update was not represented in audit summary: before=%s after=%s", before, receiverOnly)
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.orders
		SET order_date='2026-08-02', notes='新备注',
			receiver_phone='13900000000', receiver_address='新地址', receiver_company='新公司',
			shipping_amount=20, discount_amount=17
		WHERE id=42;
		UPDATE %s.order_items SET qty=3, unit_price=70, line_total=208, discount_amount=2 WHERE id=52;
	`, schema, schema)); err != nil {
		t.Fatal(err)
	}
	after, err := loadOrderSaveAuditSummaryTx(ctx, tx, schema, 42)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("editable field update did not change the order audit summary")
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(after), &got); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]any{
		"order_date": "2026-08-02", "notes": "新备注", "receiver_name": "新收件人",
		"receiver_phone": "13900000000", "receiver_address": "新地址", "receiver_company": "新公司",
	} {
		if got[key] != want {
			t.Fatalf("summary[%q] = %#v, want %#v; summary=%s", key, got[key], want, after)
		}
	}
	if got["shipping_amount"] != float64(20) || got["order_discount_amount"] != float64(15) || got["item_count"] != float64(1) {
		t.Fatalf("summary amounts/count = %s", after)
	}
	items, ok := got["order_items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("summary items = %#v", got["order_items"])
	}
	item, ok := items[0].(map[string]any)
	if !ok || item["qty"] != float64(3) || item["unit_price"] != float64(70) || item["line_total"] != float64(208) {
		t.Fatalf("summary item = %#v", items[0])
	}
}

func TestOrderAuditFailureRollsBackMainMutationAndBothAuditWrites(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	defer pool.Close()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s.orders (id BIGINT PRIMARY KEY, notes TEXT NOT NULL);
		CREATE TABLE %s.order_audit_logs (
			id BIGSERIAL PRIMARY KEY, order_id BIGINT NOT NULL, actor TEXT NOT NULL,
			field TEXT NOT NULL, old_value TEXT, new_value TEXT
		);
		CREATE TABLE %s.audit_logs (
			id BIGSERIAL PRIMARY KEY, actor TEXT NOT NULL, entity_type TEXT NOT NULL, entity_id BIGINT,
			action TEXT NOT NULL CHECK(action <> 'update'), field TEXT, old_value TEXT, new_value TEXT, meta JSONB
		);
		INSERT INTO %s.orders(id,notes) VALUES(42,'unchanged');
	`, schema, schema, schema, schema)); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET notes='must rollback' WHERE id=42`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := repo.logOrderSaveTx(ctx, tx, "mini-employee:7:销售甲", 42, "SO-42", true, `{"notes":"unchanged"}`, `{"notes":"must rollback"}`, 0, ""); err == nil {
		t.Fatal("audit constraint failure = nil")
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var notes string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT notes FROM %s.orders WHERE id=42`, schema)).Scan(&notes); err != nil {
		t.Fatal(err)
	}
	if notes != "unchanged" {
		t.Fatalf("main mutation was not rolled back: notes=%q", notes)
	}
	var auditCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.order_audit_logs`, schema)).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 0 {
		t.Fatalf("partial order audit rows remained after rollback: %d", auditCount)
	}
}
