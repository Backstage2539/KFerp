package sales

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

func TestRestoreOrderSnapshotHandlesOldAndCurrentOperationNotes(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	defer pool.Close()
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s.orders(id bigint primary key,notes text); CREATE TABLE %[1]s.order_items(id bigint primary key,order_id bigint,operation_notes_json jsonb not null default '[]'); INSERT INTO %[1]s.orders VALUES(1,'new'); INSERT INTO %[1]s.order_items VALUES(10,1,'[]');`, schema))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	snapshot := orderContentSnapshot{Order: json.RawMessage(`{"notes":"old"}`), Items: json.RawMessage(`[{"id":10,"order_id":1}]`)}
	if err = applyOrderContentSnapshotTx(ctx, tx, schema, 1, snapshot); err != nil {
		t.Fatalf("legacy snapshot: %v", err)
	}
	var raw string
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT operation_notes_json::text FROM %s.order_items WHERE id=10`, schema)).Scan(&raw); err != nil || raw != "[]" {
		t.Fatalf("legacy notes %s %v", raw, err)
	}
	snapshot.Items = json.RawMessage(`[{"id":10,"order_id":1,"operation_notes_json":[{"note":"浅烘"}]}]`)
	if err = applyOrderContentSnapshotTx(ctx, tx, schema, 1, snapshot); err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT operation_notes_json::text FROM %s.order_items WHERE id=10`, schema)).Scan(&raw); err != nil || raw != `[{"note": "浅烘"}]` {
		t.Fatalf("snapshot notes %s %v", raw, err)
	}
}

func TestPendingOtherFeeOnlyAffectsConfirmedTotalOnAcceptance(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	defer pool.Close()
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %[1]s.orders(id bigint primary key, other_fee numeric not null default 0, other_fee_note text not null default '',grand_total numeric,total_amount numeric,
 confirmation_required boolean,confirmation_revision bigint,confirmation_accepted_revision bigint,confirmation_status text,confirmation_pending jsonb,confirmation_reason text);
 CREATE TABLE %[1]s.order_items(id bigint primary key, order_id bigint, operation_notes_json jsonb not null default '[]');
 CREATE TABLE %[1]s.audit_logs(actor text,entity_type text,entity_id bigint,action text,field text,old_value text,new_value text,meta jsonb);
 CREATE TABLE %[1]s.order_content_versions(order_id bigint,revision bigint,snapshot jsonb,status text,actor text);
 CREATE TABLE %[1]s.order_freight_snapshots(order_id bigint,snapshot_json jsonb);
 INSERT INTO %[1]s.orders(id,other_fee,other_fee_note,grand_total,total_amount,confirmation_required,confirmation_revision,confirmation_accepted_revision) VALUES(1,12.5,'包装',100,87.5,false,0,0);`, schema))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	r := NewRepository(pool, schema)
	if err = r.stageOrderConfirmationTx(ctx, tx, 1, nil, "employee"); err != nil {
		t.Fatal(err)
	}
	var pending []byte
	var f, total float64
	var note string
	if err = tx.QueryRow(ctx, fmt.Sprintf("SELECT other_fee,other_fee_note,grand_total,confirmation_pending FROM %s.orders WHERE id=1", schema)).Scan(&f, &note, &total, &pending); err != nil || f != 0 || note != "" || total != 0 {
		t.Fatalf("unconfirmed charge leaked: %v %s %v %v", f, note, total, err)
	}
	var snapshot orderContentSnapshot
	if err = json.Unmarshal(pending, &snapshot); err != nil {
		t.Fatal(err)
	}
	if err = applyOrderContentSnapshotTx(ctx, tx, schema, 1, snapshot); err != nil {
		t.Fatal(err)
	}
	check := func(fee float64, want string) {
		t.Helper()
		if err := tx.QueryRow(ctx, fmt.Sprintf("SELECT other_fee,other_fee_note FROM %s.orders WHERE id=1", schema)).Scan(&f, &note); err != nil || f != fee || note != want {
			t.Fatalf("restored: %v %q %v", f, note, err)
		}
	}
	check(12.5, "包装")
	snapshot.Order = json.RawMessage(`{"grand_total":87.5,"total_amount":87.5}`)
	if err = applyOrderContentSnapshotTx(ctx, tx, schema, 1, snapshot); err != nil {
		t.Fatal(err)
	}
	check(0, "")
}
