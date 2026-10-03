package productcreator

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	app "orderapp/internal/application/productcreator"
	"os"
	"testing"
	"time"
)

func resultAliasFixture() app.Run {
	return app.Run{Workflow: app.Workflow{Nodes: []app.Node{{ID: "raw"}, {ID: "semi"}, {ID: "bom"}}}, BusinessResults: map[string]any{"objects": map[string]any{
		"raw":  []any{map[string]any{"type": "material", "id": 166, "name": "误填半成品"}},
		"semi": []any{map[string]any{"type": "material", "id": 167, "name": "半成品", "code": "M167"}},
		"bom":  []any{map[string]any{"type": "material", "id": 167, "name": "半成品", "bom_id": 24468}, map[string]any{"type": "material", "id": 168, "name": "半成品"}},
	}}}
}
func TestResultObjectSummariesMergeAliasesNotNames(t *testing.T) {
	run := resultAliasFixture()
	rows := summarizeResultObjects(run)
	if len(rows) != 3 || len(rows[1].SourceNodeIDs) != 2 || len(rows[1].BOMIDs) != 1 || rows[1].Code != "M167" {
		t.Fatalf("unexpected summaries: %+v", rows)
	}
	if rows[1].ID == rows[2].ID {
		t.Fatal("same name must not merge distinct records")
	}
}
func TestCurrentResultObjectsReadCurrentNamesWithoutRewritingSnapshot(t *testing.T) {
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
	schema := fmt.Sprintf("pc_result_%d", time.Now().UnixNano())
	_, err = pool.Exec(ctx, "CREATE SCHEMA "+schema)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	_, err = pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s.materials(id bigint, name text, code text, unit text); INSERT INTO %s.materials VALUES(166,'快乐樱桃-生豆','M166','kg'),(167,'半成品','M167','kg'),(168,'半成品','M168','kg')`, schema, schema))
	if err != nil {
		t.Fatal(err)
	}
	run := resultAliasFixture()
	rows, err := NewRepository(pool, schema).currentResultObjects(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Name != "快乐樱桃-生豆" || rows[0].CreationName != "误填半成品" {
		t.Fatalf("live name and original name must coexist: %+v", rows)
	}
	if summarizeResultObjects(run)[0].Name != "误填半成品" {
		t.Fatal("stored execution snapshot was mutated")
	}
}

func TestResultObjectsDistinguishCreatedAndReusedArchives(t *testing.T) {
	run := resultAliasFixture()
	run.BusinessResults["objects"].(map[string]any)["semi"].([]any)[0].(map[string]any)["new"] = true
	rows := summarizeResultObjects(run)
	raw, _ := json.Marshal(rows)
	var objects []map[string]any
	json.Unmarshal(raw, &objects)
	if objects[0]["origin"] != "reused" || objects[1]["origin"] != "created" {
		t.Fatalf("missing origin: %s", raw)
	}
}

func TestResultObjectsInferLegacyCreatedSpecsButKeepReusedSpecs(t *testing.T) {
	run := app.Run{Inputs: map[string]map[string]any{"existing": {"action": "reuse"}}, BusinessResults: map[string]any{"objects": map[string]any{
		"new":      []any{map[string]any{"type": "bom", "id": 10}, map[string]any{"type": "spec", "id": 20, "bom_id": 10}},
		"existing": []any{map[string]any{"type": "bom", "id": 11}, map[string]any{"type": "spec", "id": 21, "bom_id": 11}},
	}}}
	for _, row := range summarizeResultObjects(run) {
		expected := "created"
		if row.ID == 11 || row.ID == 21 {
			expected = "reused"
		}
		if row.Origin != expected {
			t.Fatalf("%s %d origin=%s, want %s", row.Type, row.ID, row.Origin, expected)
		}
	}
}
