package productcreator

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	app "orderapp/internal/application/productcreator"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	"os"
	"testing"
	"time"
)

func consumedMaterialRun() app.Run {
	return app.Run{Workflow: app.Workflow{Version: 5, Nodes: []app.Node{
		{ID: "raw", Kind: app.ModuleMaterial, Name: "投入原料", Config: map[string]any{"data_role": "input"}},
		{ID: "bom", Kind: app.ModuleBOM, Config: map[string]any{"output_type": "material"}},
	}, Edges: []app.Edge{{ID: "e", Source: "raw", SourceHandle: "material", Target: "bom", TargetHandle: "components", Kind: app.EdgeData}}}, Inputs: map[string]map[string]any{
		"raw": {"rows": []any{map[string]any{"row_id": "r1", "action": "create", "name": "生豆", "unit": "kg", "supply_mode": "manufacture"}}},
		"bom": {"components": []any{map[string]any{"source_node_id": "raw", "source_row_id": "r1", "quantity": 100, "unit": "ratio_pct"}}},
	}}
}
func TestPreviewRejectsConsumedNewManufacturedInputWithoutProducer(t *testing.T) {
	run := consumedMaterialRun()
	_, issues := (BusinessExecutor{}).InspectConfigurationPreview(context.Background(), run)
	if !previewHasCode(issues, "manufacturing_source_missing") {
		t.Fatalf("must reject orphan self-manufactured input, got %+v", issues)
	}
	if issues[0].NodeID != "raw" || issues[0].Field != "rows.r1.supply_mode" {
		t.Fatalf("must locate input row: %+v", issues)
	}
}
func TestPreviewAllowsPurchasedInputAndUnusedManufacturedCandidate(t *testing.T) {
	run := consumedMaterialRun()
	mapRows(run.Inputs["raw"]["rows"])[0]["supply_mode"] = "purchase"
	_, issues := (BusinessExecutor{}).InspectConfigurationPreview(context.Background(), run)
	if len(issues) > 0 {
		t.Fatal(issues)
	}
	mapRows(run.Inputs["raw"]["rows"])[0]["supply_mode"] = "manufacture"
	run.Inputs["bom"]["components"] = []any{}
	_, issues = (BusinessExecutor{}).InspectConfigurationPreview(context.Background(), run)
	if len(issues) > 0 {
		t.Fatalf("unused candidates are not recipe components: %+v", issues)
	}
}

func TestCommitRejectsOrphanBeforeAnyBusinessWrite(t *testing.T) {
	_, err := (BusinessExecutor{}).ExecuteConfiguration(context.Background(), consumedMaterialRun(), "test")
	if err == nil {
		t.Fatal("commit must revalidate sources even after an old valid preview")
	}
	var failure app.ExecutionError
	if !errors.As(err, &failure) || !previewHasCode(failure.Issues, "manufacturing_source_missing") {
		t.Fatalf("wrong commit error: %v", err)
	}
}

func TestMaterialSourceReadsRealModeAndPublishedBindingPostgres(t *testing.T) {
	dsn := os.Getenv("ORDERAPP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires isolated test database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := fmt.Sprintf("pc_sources_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	_, err = pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %[1]s.materials(id bigint,name text,is_semi_finished bool,deprecated_at timestamptz);
 CREATE TABLE %[1]s.production_boms(id bigint,output_type text,output_material_id bigint,status text);
 CREATE TABLE %[1]s.production_bom_versions(id bigint,bom_id bigint,status text);
 CREATE TABLE %[1]s.production_bom_output_bindings(output_type text,output_id bigint,bom_id bigint,bom_version_id bigint,is_default bool);
 INSERT INTO %[1]s.materials VALUES(166,'已存自制物料',true,NULL);
 INSERT INTO %[1]s.production_boms VALUES(1,'material',166,'active');
 INSERT INTO %[1]s.production_bom_versions VALUES(2,1,'draft');
 INSERT INTO %[1]s.production_bom_output_bindings VALUES('material',166,1,2,true);`, schema))
	if err != nil {
		t.Fatal(err)
	}
	run := consumedMaterialRun()
	row := mapRows(run.Inputs["raw"]["rows"])[0]
	row["action"] = "reuse"
	row["material_id"] = 166
	row["supply_mode"] = "purchase"
	e := BusinessExecutor{pool: pool, schema: schema}
	_, issues := e.InspectConfigurationPreview(ctx, run)
	if !previewHasCode(issues, "manufacturing_source_missing") {
		t.Fatalf("forged mode and draft BOM must fail: %+v", issues)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+schema+".production_bom_versions SET status='published'"); err != nil {
		t.Fatal(err)
	}
	details, issues := e.InspectConfigurationPreview(ctx, run)
	if len(issues) != 0 {
		t.Fatal(issues)
	}
	if details["raw"]["material_objects"].([]map[string]any)[0]["supply_mode"] != "manufacture" {
		t.Fatal("preview must show real archive mode")
	}
	// V7 nodes declare their acquisition mode. A forged runtime label cannot
	// make an existing manufactured archive eligible for an external input.
	run.Workflow.Version = 7
	run.Workflow.Nodes[0].Config["supply_mode"] = "purchase"
	_, issues = e.InspectConfigurationPreview(ctx, run)
	if !previewHasCode(issues, "material_supply_mode_mismatch") {
		t.Fatalf("V7 must use real archive mode: %+v", issues)
	}
	run.Workflow.Version = 5
	// A shared producer in this run is enough; the existing archive is read in the same transaction.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "UPDATE "+schema+".production_bom_versions SET status='draft'"); err != nil {
		t.Fatal(err)
	}
	txCtx := postgresinfra.WithTransaction(ctx, tx)
	_, issues = e.InspectConfigurationPreview(txCtx, run)
	if !previewHasCode(issues, "manufacturing_source_missing") {
		t.Fatal("must see current transaction, not old pool state")
	}
	run.Workflow.Nodes = append(run.Workflow.Nodes, app.Node{ID: "producer", Kind: app.ModuleBOM})
	run.Workflow.Edges = append(run.Workflow.Edges, app.Edge{ID: "produce", Kind: app.EdgeData, Source: "producer", SourceHandle: "assembly", Target: "raw", TargetHandle: "from_bom"})
	run.Workflow.Nodes[0].Config["data_role"] = "output"
	run.Inputs["raw"] = map[string]any{"action": "create", "name": "本次自制产出", "supply_mode": "manufacture"}
	mapRows(run.Inputs["bom"]["components"])[0]["source_row_id"] = "output"
	_, issues = e.InspectConfigurationPreview(txCtx, run)
	if len(issues) != 0 {
		t.Fatalf("upstream output should pass: %+v", issues)
	}
}
