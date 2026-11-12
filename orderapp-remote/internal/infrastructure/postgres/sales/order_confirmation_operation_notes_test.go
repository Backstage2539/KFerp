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
