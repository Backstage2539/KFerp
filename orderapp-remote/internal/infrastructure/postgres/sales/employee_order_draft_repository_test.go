package sales

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	salesapp "orderapp/internal/application/sales"
)

func TestEmployeeOrderDraftRepositoryIsolatesEmployeesAndAuditsWrites(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	prepareSalesSchemaPrerequisites(t, ctx, pool, schema)
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.company_employees(id,name) VALUES(41,'销售甲'),(42,'销售乙')
	`, schema)); err != nil {
		t.Fatalf("seed employees: %v", err)
	}

	repo := NewRepository(pool, schema)
	created, err := repo.SaveEmployeeOrderDraft(ctx, salesapp.SaveEmployeeOrderDraftCommand{
		EmployeeID: 41,
		Actor:      "mini-employee:41:销售甲",
		Payload:    json.RawMessage(`{"customer_id":7}`),
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if created.ID <= 0 || created.EmployeeID != 41 || string(created.Payload) != `{"customer_id": 7}` && string(created.Payload) != `{"customer_id":7}` {
		t.Fatalf("created draft=%+v", created)
	}
	other, err := repo.GetEmployeeOrderDraft(ctx, 42)
	if err != nil || other != nil {
		t.Fatalf("other employee draft=%+v err=%v", other, err)
	}

	updated, err := repo.SaveEmployeeOrderDraft(ctx, salesapp.SaveEmployeeOrderDraftCommand{
		EmployeeID: 41,
		Actor:      "mini-employee:41:销售甲",
		Payload:    json.RawMessage(`{"customer_id":8,"items":[{"qty":2}]}`),
	})
	if err != nil {
		t.Fatalf("update draft: %v", err)
	}
	if updated.ID != created.ID || updated.EmployeeID != 41 {
		t.Fatalf("updated draft=%+v, created=%+v", updated, created)
	}
	if deleted, err := repo.DeleteEmployeeOrderDraft(ctx, 42, "mini-employee:42:销售乙"); err != nil || deleted {
		t.Fatalf("delete other absent draft deleted=%v err=%v", deleted, err)
	}
	if deleted, err := repo.DeleteEmployeeOrderDraft(ctx, 41, "mini-employee:41:销售甲"); err != nil || !deleted {
		t.Fatalf("delete own draft deleted=%v err=%v", deleted, err)
	}
	loaded, err := repo.GetEmployeeOrderDraft(ctx, 41)
	if err != nil || loaded != nil {
		t.Fatalf("deleted draft=%+v err=%v", loaded, err)
	}

	rows, err := pool.Query(ctx, fmt.Sprintf(`
		SELECT action, COALESCE(meta::text,'')
		FROM %s.audit_logs
		WHERE entity_type='employee_order_draft'
		ORDER BY id
	`, schema))
	if err != nil {
		t.Fatalf("query audits: %v", err)
	}
	defer rows.Close()
	actions := make([]string, 0, 3)
	metas := make([]string, 0, 3)
	for rows.Next() {
		var action, meta string
		if err := rows.Scan(&action, &meta); err != nil {
			t.Fatalf("scan audit: %v", err)
		}
		actions = append(actions, action)
		metas = append(metas, meta)
	}
	if strings.Join(actions, ",") != "create,update,delete" {
		t.Fatalf("draft audit actions=%v", actions)
	}
	for _, meta := range metas {
		for _, sensitive := range []string{"customer_id", "items", "phone", "address", "payload"} {
			if strings.Contains(strings.ToLower(meta), sensitive) {
				t.Fatalf("draft audit meta leaked %q: %s", sensitive, meta)
			}
		}
	}
}

func TestFormalOrderSaveClearsDraftInsideOrderTransaction(t *testing.T) {
	body, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	deleteAt := strings.Index(text, `deleteEmployeeOrderDraftTx(ctx, tx, r.schema, cmd.DraftEmployeeID`)
	commitAt := strings.Index(text, `tx.Commit(ctx)`)
	if deleteAt < 0 || commitAt < 0 || deleteAt > commitAt {
		t.Fatal("formal order save must delete the current employee draft before committing the order transaction")
	}
}
