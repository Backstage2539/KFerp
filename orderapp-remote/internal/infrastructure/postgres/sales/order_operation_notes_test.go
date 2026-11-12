package sales

import (
	"context"
	"fmt"
	app "orderapp/internal/application/sales"
	"strings"
	"testing"
)

func TestOrderOperationNoteMustMatchExactProductRouteOption(t *testing.T) {
	options := []app.OrderOperationOption{{OutputType: "material", OutputID: 3, RouteID: 5, StepID: 7, OperationID: 8, OperationName: "烘焙"}}
	good := []app.OrderOperationNote{{OutputType: "material", OutputID: 3, RouteID: 5, StepID: 7, OperationID: 8, OperationName: "伪造", Note: "浅烘"}}
	got, err := app.MatchOrderOperationNotes(good, options)
	if err != nil || got[0].OperationName != "烘焙" {
		t.Fatalf("canonical names: %+v %v", got, err)
	}
	good[0].OutputID = 4
	if _, err = app.MatchOrderOperationNotes(good, options); err == nil {
		t.Fatal("unrelated output accepted")
	}
}

func TestOrderOperationNotesSaveAndClearAreTransactional(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	defer pool.Close()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s.order_items(id bigint primary key,order_id bigint,operation_notes_json jsonb not null default '[]'); CREATE TABLE %s.audit_logs(id BIGSERIAL PRIMARY KEY,actor TEXT NOT NULL,entity_type TEXT NOT NULL,entity_id BIGINT,action TEXT NOT NULL,field TEXT,old_value TEXT,new_value TEXT,meta JSONB); INSERT INTO %s.order_items(id,order_id) VALUES(7,9)`, schema, schema, schema)); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	notes := []app.OrderOperationNote{{OutputType: "material", OutputID: 3, RouteID: 5, StepID: 7, OperationID: 8, OperationName: "烘焙", Note: "浅烘"}}
	if err = repo.saveOrderOperationNotesTx(ctx, tx, 9, 7, "worker", 0, 0, 0, false, &notes); err != nil {
		t.Fatal(err)
	}
	if err = repo.saveOrderOperationNotesTx(ctx, tx, 9, 7, "worker", 0, 0, 0, false, nil); err != nil {
		t.Fatal(err)
	}
	var stored string
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT operation_notes_json::text FROM %s.order_items WHERE id=7`, schema)).Scan(&stored); err != nil || !strings.Contains(stored, "浅烘") {
		t.Fatalf("notes lost: %q %v", stored, err)
	}
	clear := []app.OrderOperationNote{}
	if err = repo.saveOrderOperationNotesTx(ctx, tx, 9, 7, "worker", 0, 0, 0, true, &clear); err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT operation_notes_json::text FROM %s.order_items WHERE id=7`, schema)).Scan(&stored); err != nil || stored != "[]" {
		t.Fatalf("not cleared: %q %v", stored, err)
	}
	var count int
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE entity_id=9`, schema)).Scan(&count); err != nil || count != 2 {
		t.Fatalf("audit count %d %v", count, err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT operation_notes_json::text FROM %s.order_items WHERE id=7`, schema)).Scan(&stored); err != nil || stored != "[]" {
		t.Fatalf("rollback failed: %q %v", stored, err)
	}
}
