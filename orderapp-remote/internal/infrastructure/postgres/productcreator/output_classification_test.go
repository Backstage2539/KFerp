package productcreator

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	materialsapp "orderapp/internal/application/materials"
	app "orderapp/internal/application/productcreator"
	pg "orderapp/internal/infrastructure/postgres"
	"os"
	"testing"
	"time"
)

type outputClassificationMaterialRepo struct {
	materialsapp.Repository
	schema string
}

func (r outputClassificationMaterialRepo) Create(ctx context.Context, cmd materialsapp.CreateCommand) (materialsapp.Material, error) {
	var id int64
	err := queryWithTransaction(ctx).QueryRow(ctx, "INSERT INTO "+r.schema+".materials(name) VALUES($1) RETURNING id", cmd.Input.Name).Scan(&id)
	return materialsapp.Material{ID: id, Name: cmd.Input.Name, Unit: cmd.Input.Unit}, err
}
func TestOutputMaterialClassificationSavedAndRolledBack(t *testing.T) {
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
	schema := fmt.Sprintf("pc_output_class_%d", time.Now().UnixNano())
	_, err = pool.Exec(ctx, "CREATE SCHEMA "+schema)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	_, err = pool.Exec(ctx, fmt.Sprintf(`
 CREATE TABLE %[1]s.materials(id bigserial primary key,name text);
 CREATE TABLE %[1]s.business_groups(id bigint primary key,active bool);
 CREATE TABLE %[1]s.business_group_items(id bigint primary key,group_id bigint,active bool);
 CREATE TABLE %[1]s.business_group_usages(group_id bigint,usage_key text,active bool);
 CREATE TABLE %[1]s.business_group_assignments(id bigserial primary key,group_id bigint,group_item_id bigint,usage_key text,object_key text,object_id bigint,object_ref text,sort_order int,created_by text,updated_by text);
 CREATE TABLE %[1]s.audit_logs(id bigserial primary key,actor text,entity_type text,entity_id bigint,action text,field text,old_value text,new_value text,meta jsonb);
 INSERT INTO %[1]s.business_groups VALUES(30,true),(31,false);
 INSERT INTO %[1]s.business_group_items VALUES(40,30,true);
 INSERT INTO %[1]s.business_group_usages VALUES(30,'material_catalog',true);`, schema))
	if err != nil {
		t.Fatal(err)
	}
	e := BusinessExecutor{pool: pool, schema: schema, materials: materialsapp.NewService(outputClassificationMaterialRepo{schema: schema})}
	for _, tc := range []struct {
		name        string
		group, item int
		invalid     bool
	}{{"root", 30, 0, false}, {"child", 30, 40, false}, {"inactive", 31, 0, true}} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			run := app.Run{Workflow: app.Workflow{Version: 6, Nodes: []app.Node{{ID: "powder", Kind: app.ModuleMaterial, Config: map[string]any{"data_role": "output", "supply_mode": "manufacture"}}}}, Inputs: map[string]map[string]any{"powder": {"name": "咖啡粉", "action": "create", "supply_mode": "manufacture", "unit": "kg", "classification_group_id": tc.group, "classification_item_id": tc.item}}}
			row := outputMaterialValues(run, run.Workflow.Nodes[0], run.Inputs["powder"])
			_, err = e.executeMaterials(pg.WithTransaction(ctx, tx), run.Workflow.Nodes[0], map[string]any{"rows": []any{row}}, "tester", map[string]map[string]createdReference{"powder": {}})
			if tc.invalid {
				if err == nil {
					t.Fatal("inactive category accepted")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				var group, item int
				err = tx.QueryRow(ctx, "SELECT group_id,group_item_id FROM "+schema+".business_group_assignments").Scan(&group, &item)
				if err != nil || group != tc.group || item != tc.item {
					t.Fatalf("output classification lost: (%d,%d), err=%v", group, item, err)
				}
				var audits int
				err = tx.QueryRow(ctx, "SELECT count(*) FROM "+schema+".audit_logs WHERE action='assign_product_creator_classification'").Scan(&audits)
				if err != nil || audits != 1 {
					t.Fatalf("classification audit %d: %v", audits, err)
				}
			}
			if err := tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			var count int
			pool.QueryRow(ctx, "SELECT count(*) FROM "+schema+".materials").Scan(&count)
			if count != 0 {
				t.Fatal("rollback left material records")
			}
		})
	}
}
