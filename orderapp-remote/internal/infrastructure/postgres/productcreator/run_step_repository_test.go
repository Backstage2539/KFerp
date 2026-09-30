package productcreator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	app "orderapp/internal/application/productcreator"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestExecuteRunStepIsAtomicAndIdempotent(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for product creator postgres tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("test_product_creator_step_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s.audit_logs(
		id BIGSERIAL PRIMARY KEY,ts TIMESTAMPTZ NOT NULL DEFAULT now(),actor TEXT NOT NULL DEFAULT '',
		entity_type TEXT NOT NULL DEFAULT '',entity_id BIGINT,action TEXT NOT NULL DEFAULT '',field TEXT,
		old_value TEXT,new_value TEXT,meta JSONB)`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s.business_effects(run_id BIGINT NOT NULL,node_id TEXT NOT NULL,UNIQUE(run_id,node_id))`, schema)); err != nil {
		t.Fatal(err)
	}
	workflow := app.Workflow{Nodes: []app.Node{{ID: "pricing", Kind: app.ModulePricing}, {ID: "purchase", Kind: app.ModulePurchase}}}
	workflowJSON, _ := json.Marshal(workflow)
	var templateID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.business_templates(name,status,published_version,draft_graph) VALUES('步骤事务模板','published',1,$1::jsonb) RETURNING id`, schema), workflowJSON).Scan(&templateID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.business_template_versions(template_id,version,name,workflow,published_by) VALUES($1,1,'步骤事务模板',$2::jsonb,'test')`, schema), templateID, workflowJSON); err != nil {
		t.Fatal(err)
	}
	var runID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.product_creator_runs(template_id,template_version,status,revision,workflow_snapshot,inputs,created_by) VALUES($1,1,'in_progress',5,$2::jsonb,'{}'::jsonb,'test') RETURNING id`, schema), templateID, workflowJSON).Scan(&runID); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	callbackCount := 0
	apply := func(txCtx context.Context, _ app.Run, node app.Node, _ string, _ map[string]any) (map[string]any, error) {
		callbackCount++
		tx, ok := postgresinfra.TransactionFromContext(txCtx)
		if !ok {
			t.Fatal("step executor must receive the run transaction")
		}
		if _, err := tx.Exec(txCtx, fmt.Sprintf(`INSERT INTO %s.business_effects(run_id,node_id) VALUES($1,$2)`, schema), runID, node.ID); err != nil {
			return nil, err
		}
		return map[string]any{"status": "succeeded", "published_price": map[string]any{"id": 90}}, nil
	}
	committed, err := repo.ExecuteRunStep(ctx, runID, 5, "pricing", "publish_price", "pc-run-7-price", "hash-7", "tester", map[string]any{"confirm": true}, apply)
	if err != nil {
		t.Fatal(err)
	}
	if committed.Revision != 6 || committed.Status != "in_progress" {
		t.Fatalf("unexpected run after follow-up step: status=%q revision=%d", committed.Status, committed.Revision)
	}
	if _, err := repo.ExecuteRunStep(ctx, runID, 5, "pricing", "publish_price", "pc-run-7-price", "hash-7", "tester", map[string]any{"confirm": true}, func(context.Context, app.Run, app.Node, string, map[string]any) (map[string]any, error) {
		callbackCount++
		return nil, errors.New("must not execute twice")
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ExecuteRunStep(ctx, runID, 5, "pricing", "publish_price", "pc-run-7-price", "different-hash", "tester", nil, apply); !errors.Is(err, app.ErrConflict) {
		t.Fatalf("same key with different parameters error=%v, want conflict", err)
	}
	if callbackCount != 1 {
		t.Fatalf("idempotency callback count=%d, want 1", callbackCount)
	}
	failing := func(txCtx context.Context, _ app.Run, node app.Node, _ string, _ map[string]any) (map[string]any, error) {
		tx, ok := postgresinfra.TransactionFromContext(txCtx)
		if !ok {
			t.Fatal("failing step executor must receive the run transaction")
		}
		if _, err := tx.Exec(txCtx, fmt.Sprintf(`INSERT INTO %s.business_effects(run_id,node_id) VALUES($1,$2)`, schema), runID, node.ID); err != nil {
			return nil, err
		}
		return nil, errors.New("simulated business failure")
	}
	if _, err := repo.ExecuteRunStep(ctx, runID, 6, "purchase", "confirm_receipt", "pc-run-7-receipt", "hash-receipt", "tester", map[string]any{"quantity": 2}, failing); err == nil {
		t.Fatal("failing business action should return its error")
	}
	var effects, steps, auditCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.business_effects`, schema)).Scan(&effects); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.product_creator_run_steps`, schema)).Scan(&steps); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs WHERE entity_type='product_creator_run' AND action='execute_step'`, schema)).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if effects != 1 || steps != 1 || auditCount != 1 {
		t.Fatalf("committed effect/step/audit counts=%d/%d/%d, want 1/1/1", effects, steps, auditCount)
	}
}
