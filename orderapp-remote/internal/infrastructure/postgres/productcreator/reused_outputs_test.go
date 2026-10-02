package productcreator

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	app "orderapp/internal/application/productcreator"
	pg "orderapp/internal/infrastructure/postgres"
	"os"
	"testing"
	"time"
)

func TestV8ReusedOutputPostgresNoWritesAndFreshSnapshot(t *testing.T) {
	dsn := os.Getenv("ORDERAPP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated PostgreSQL required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := fmt.Sprintf("pc_v8_%d", time.Now().UnixNano())
	if _, err = pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	_, err = pool.Exec(ctx, fmt.Sprintf(`
 CREATE TABLE %[1]s.materials(id bigint,name text,unit text,owner_customer_id bigint,is_semi_finished bool,deprecated_at timestamptz);
 CREATE TABLE %[1]s.products(id bigint,name text,customer_id bigint,active bool);
 CREATE TABLE %[1]s.production_boms(id bigint,output_type text,output_material_id bigint,output_product_id bigint,status text);
 CREATE TABLE %[1]s.production_bom_versions(id bigint,bom_id bigint,status text,published_at timestamptz,created_at timestamptz);
 CREATE TABLE %[1]s.production_bom_output_bindings(output_type text,output_id bigint,bom_id bigint,bom_version_id bigint,is_default bool);
 CREATE TABLE %[1]s.production_bom_specs(id bigint,bom_id bigint,spec_key text,name text);
 CREATE TABLE %[1]s.production_bom_version_variants(id bigint,version_id bigint,bom_spec_id bigint,inventory_unit text,spec_name_snapshot text,sort_order int);
 INSERT INTO %[1]s.materials VALUES(42,'已有烘焙半成品','kg',0,true,NULL);
 INSERT INTO %[1]s.products VALUES(52,'已有挂耳',0,true);
 INSERT INTO %[1]s.production_boms VALUES(1,'material',42,0,'active'),(3,'product',0,52,'active');
 INSERT INTO %[1]s.production_bom_versions VALUES(2,1,'published',now(),now()),(4,3,'published',now(),now());
 INSERT INTO %[1]s.production_bom_output_bindings VALUES('material',42,1,2,true),('product',52,3,4,true);
 INSERT INTO %[1]s.production_bom_specs VALUES(101,3,'s1','10g袋装');
 INSERT INTO %[1]s.production_bom_version_variants VALUES(102,4,101,'袋','10g袋装',0);`, schema))
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []app.ModuleKind{app.ModuleMaterial, app.ModuleProduct} {
		t.Run(string(kind), func(t *testing.T) {
			key, id := "material_id", 42
			if kind == app.ModuleProduct {
				key, id = "product_id", 52
			}
			run := app.Run{Workflow: app.Workflow{Version: 8, Nodes: []app.Node{
				{ID: "raw", Kind: app.ModuleMaterial, Config: map[string]any{"data_role": "input", "supply_mode": "purchase"}},
				{ID: "route", Kind: app.ModuleProcess, Config: map[string]any{"route_id": 99}},
				{ID: "bom", Kind: app.ModuleBOM, Config: map[string]any{"output_type": string(kind), "spec_template_version_id": 9999}},
				{ID: "out", Kind: kind, Config: map[string]any{"data_role": "output", "supply_mode": "manufacture"}},
			}, Edges: []app.Edge{
				{ID: "1", Source: "raw", SourceHandle: "material", Target: "bom", TargetHandle: "components", Kind: app.EdgeData},
				{ID: "2", Source: "route", SourceHandle: "route", Target: "bom", TargetHandle: "route", Kind: app.EdgeData},
				{ID: "3", Source: "bom", SourceHandle: "assembly", Target: "out", TargetHandle: "from_bom", Kind: app.EdgeData},
			}}, Inputs: map[string]map[string]any{"out": {"action": "reuse", key: id}}}
			if kind == app.ModuleMaterial {
				delete(run.Workflow.Nodes[2].Config, "spec_template_version_id")
			}
			e := BusinessExecutor{pool: pool, schema: schema}
			details, issues := e.InspectConfigurationPreview(ctx, run)
			if len(issues) > 0 {
				t.Fatal(issues)
			}
			run.Preview = &app.RunPreview{Valid: true, Steps: []app.StepPreview{{NodeID: "out", Details: details["out"]}}}
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			txCtx := pg.WithTransaction(ctx, tx)
			// No catalog/material/BOM services exist: invoking a disabled create path would fail.
			result, err := e.ExecuteConfiguration(txCtx, run, "v8-test")
			if err != nil {
				t.Fatal(err)
			}
			steps := result["steps"].(map[string]any)
			for _, id := range []string{"raw", "route", "bom"} {
				if steps[id].(map[string]any)["status"] != "skipped" {
					t.Fatal(steps)
				}
			}
			if steps["out"].(map[string]any)["action"] != "reuse" {
				t.Fatal(steps)
			}
			var count int
			if err = tx.QueryRow(ctx, "SELECT count(*) FROM "+schema+".production_boms").Scan(&count); err != nil || count != 2 {
				t.Fatalf("BOM changed: %d %v", count, err)
			}
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if kind == app.ModuleProduct {
				if !e.validExistingProductSpec(ctx, 52, 101) || e.validExistingProductSpec(ctx, 999, 101) {
					t.Fatal("spec identity validation failed")
				}
			}
			table, field := "materials", "unit"
			if kind == app.ModuleProduct {
				table, field = "products", "name"
			}
			if _, err = pool.Exec(ctx, fmt.Sprintf("UPDATE %s.%s SET %s='changed' WHERE id=$1", schema, table, field), id); err != nil {
				t.Fatal(err)
			}
			if err = e.validateReusedSnapshots(ctx, run); err == nil {
				t.Fatal("changed archive must require preview again")
			}
			delete(run.Inputs["out"], key)
			// Business field errors belong to the reuse boundary, never the disabled producer.
			p := app.BuildRunPreview(run.Workflow, run.Inputs)
			if p.Valid {
				t.Fatal("missing selection accepted")
			}
		})
	}
}
