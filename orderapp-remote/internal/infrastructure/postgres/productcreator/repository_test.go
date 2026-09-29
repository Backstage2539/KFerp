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

func TestCommitConfigurationIsAtomicAndIdempotent(t *testing.T) {
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
	schema := fmt.Sprintf("test_product_creator_commit_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s.audit_logs(id BIGSERIAL PRIMARY KEY,ts TIMESTAMPTZ NOT NULL DEFAULT now(),actor TEXT NOT NULL DEFAULT '',entity_type TEXT NOT NULL DEFAULT '',entity_id BIGINT,action TEXT NOT NULL DEFAULT '',field TEXT,old_value TEXT,new_value TEXT,meta JSONB)`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s.business_effects(run_id BIGINT NOT NULL, node_id TEXT NOT NULL, UNIQUE(run_id,node_id))`, schema)); err != nil {
		t.Fatal(err)
	}
	workflowJSON, _ := json.Marshal(app.Workflow{Nodes: []app.Node{{ID: "product", Kind: app.ModuleProduct}}})
	inputsJSON, _ := json.Marshal(map[string]map[string]any{"product": {"name": "测试商品", "action": "create", "owner": "factory"}})
	previewJSON, _ := json.Marshal(app.RunPreview{Valid: true, Steps: []app.StepPreview{{NodeID: "product", Name: "商品档案", Kind: app.ModuleProduct, Action: "新建商品", Status: "ready"}}})
	var runID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.business_templates(name,status,published_version,draft_graph) VALUES('事务模板','published',1,$1::jsonb) RETURNING id`, schema), workflowJSON).Scan(&runID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.business_template_versions(template_id,version,name,workflow,published_by) VALUES($1,1,'事务模板',$2::jsonb,'test')`, schema), runID, workflowJSON); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.product_creator_runs(template_id,template_version,status,revision,workflow_snapshot,inputs,preview,created_by) VALUES($1,1,'draft',4,$2::jsonb,$3::jsonb,$4::jsonb,'test') RETURNING id`, schema), runID, workflowJSON, inputsJSON, previewJSON).Scan(&runID); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	callbackCount := 0
	execute := func(txCtx context.Context, run app.Run) (map[string]any, error) {
		callbackCount++
		tx, ok := postgresinfra.TransactionFromContext(txCtx)
		if !ok {
			t.Fatal("business callback must receive the run transaction")
		}
		if _, err := tx.Exec(txCtx, fmt.Sprintf(`INSERT INTO %s.business_effects(run_id,node_id) VALUES($1,$2)`, schema), run.ID, "product"); err != nil {
			return nil, err
		}
		return map[string]any{"run_status": "config_committed", "steps": map[string]map[string]any{"product": {"status": "succeeded", "product_id": 71}}}, nil
	}
	committed, err := repo.CommitConfiguration(ctx, runID, 4, "pc-run-1-rev-4", "hash-1", "test-actor", execute)
	if err != nil {
		t.Fatal(err)
	}
	if committed.Status != "config_committed" || committed.Revision != 5 || committed.BusinessResults["run_status"] != "config_committed" {
		t.Fatalf("unexpected committed run: %+v", committed)
	}
	if _, err := repo.CommitConfiguration(ctx, runID, 4, "pc-run-1-rev-4", "hash-1", "test-actor", func(context.Context, app.Run) (map[string]any, error) {
		callbackCount++
		return nil, errors.New("must not execute twice")
	}); err != nil {
		t.Fatal(err)
	}
	if callbackCount != 1 {
		t.Fatalf("same idempotency key replay invoked executor %d times", callbackCount)
	}
	if _, err := repo.CommitConfiguration(ctx, runID, 4, "pc-run-1-rev-4", "different-hash", "test-actor", execute); !errors.Is(err, app.ErrConflict) {
		t.Fatalf("same key with different parameters error=%v, want conflict", err)
	}
	var effects, steps, auditCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.business_effects`, schema)).Scan(&effects); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.product_creator_run_steps`, schema)).Scan(&steps); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs WHERE entity_type='product_creator_run' AND action='commit_configuration'`, schema)).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if effects != 1 || steps != 1 || auditCount != 1 {
		t.Fatalf("committed effect/step/audit counts=%d/%d/%d, want 1/1/1", effects, steps, auditCount)
	}
}

func TestCommitConfigurationRollsBackBusinessCallbackFailure(t *testing.T) {
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
	schema := fmt.Sprintf("test_product_creator_rollback_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s.audit_logs(id BIGSERIAL PRIMARY KEY,ts TIMESTAMPTZ NOT NULL DEFAULT now(),actor TEXT NOT NULL DEFAULT '',entity_type TEXT NOT NULL DEFAULT '',entity_id BIGINT,action TEXT NOT NULL DEFAULT '',field TEXT,old_value TEXT,new_value TEXT,meta JSONB)`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s.business_effects(run_id BIGINT NOT NULL)`, schema)); err != nil {
		t.Fatal(err)
	}
	workflowJSON, _ := json.Marshal(app.Workflow{Nodes: []app.Node{{ID: "product", Kind: app.ModuleProduct}}})
	inputsJSON, _ := json.Marshal(map[string]map[string]any{"product": {"name": "失败商品", "action": "create", "owner": "factory"}})
	previewJSON, _ := json.Marshal(app.RunPreview{Valid: true, Steps: []app.StepPreview{{NodeID: "product", Kind: app.ModuleProduct, Status: "ready"}}})
	var templateID, runID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.business_templates(name,status,published_version,draft_graph) VALUES('事务模板','published',1,$1::jsonb) RETURNING id`, schema), workflowJSON).Scan(&templateID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.product_creator_runs(template_id,template_version,status,revision,workflow_snapshot,inputs,preview) VALUES($1,1,'draft',1,$2::jsonb,$3::jsonb,$4::jsonb) RETURNING id`, schema), templateID, workflowJSON, inputsJSON, previewJSON).Scan(&runID); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	_, err = repo.CommitConfiguration(ctx, runID, 1, "rollback-key", "rollback-hash", "test-actor", func(txCtx context.Context, run app.Run) (map[string]any, error) {
		tx, _ := postgresinfra.TransactionFromContext(txCtx)
		if _, err := tx.Exec(txCtx, fmt.Sprintf(`INSERT INTO %s.business_effects(run_id) VALUES($1)`, schema), run.ID); err != nil {
			return nil, err
		}
		return nil, errors.New("forced business failure")
	})
	if err == nil || !strings.Contains(err.Error(), "forced business failure") {
		t.Fatalf("commit error=%v, want forced failure", err)
	}
	var effects, auditCount int
	var status string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.business_effects`, schema)).Scan(&effects); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs WHERE action='commit_configuration'`, schema)).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.product_creator_runs WHERE id=$1`, schema), runID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if effects != 0 || auditCount != 0 || status != "draft" {
		t.Fatalf("callback failure was not atomic: effects=%d audits=%d status=%q", effects, auditCount, status)
	}
}

func TestSaveRunDraftPersistsInputsAndVariablesTogether(t *testing.T) {
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
	schema := fmt.Sprintf("test_product_creator_draft_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s.audit_logs(id BIGSERIAL PRIMARY KEY,ts TIMESTAMPTZ NOT NULL DEFAULT now(),actor TEXT NOT NULL DEFAULT '',entity_type TEXT NOT NULL DEFAULT '',entity_id BIGINT,action TEXT NOT NULL DEFAULT '',field TEXT,old_value TEXT,new_value TEXT,meta JSONB)`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	workflowJSON, _ := json.Marshal(app.Workflow{Version: 3, Variables: []app.WorkflowVariable{{ID: "product_name", Name: "商品名称", DefaultValue: "豆子"}}, Nodes: []app.Node{{ID: "product", Kind: app.ModuleProduct}}})
	inputsJSON, _ := json.Marshal(map[string]map[string]any{"product": {"name": "豆子"}})
	previewJSON, _ := json.Marshal(app.RunPreview{Valid: true})
	var templateID, runID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.business_templates(name,status,published_version,draft_graph) VALUES('变量模板','published',1,$1::jsonb) RETURNING id`, schema), workflowJSON).Scan(&templateID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.product_creator_runs(template_id,template_version,status,revision,workflow_snapshot,inputs,variable_values,preview,created_by) VALUES($1,1,'draft',7,$2::jsonb,$3::jsonb,'{}'::jsonb,$4::jsonb,'test') RETURNING id`, schema), templateID, workflowJSON, inputsJSON, previewJSON).Scan(&runID); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	updatedInputs := map[string]map[string]any{"product": {"name": "云南咖啡豆"}}
	updatedVariables := map[string]string{"product_name": "云南咖啡豆"}
	updated, err := repo.SaveRunDraft(ctx, runID, 7, updatedInputs, updatedVariables, "test-actor")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Revision != 8 || updated.Inputs["product"]["name"] != "云南咖啡豆" || updated.VariableValues["product_name"] != "云南咖啡豆" {
		t.Fatalf("draft inputs and variables were not saved together: revision=%d inputs=%v variables=%v", updated.Revision, updated.Inputs, updated.VariableValues)
	}
	var auditCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs WHERE entity_type='product_creator_run' AND action='save_draft' AND entity_id=$1`, schema), runID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("save draft audit count=%d, want 1", auditCount)
	}
	if _, err := repo.SaveRunDraft(ctx, runID, 7, map[string]map[string]any{}, map[string]string{}, "test-actor"); !errors.Is(err, app.ErrConflict) {
		t.Fatalf("stale revision save error=%v, want conflict", err)
	}
}
