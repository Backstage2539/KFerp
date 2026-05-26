package production

import (
	"os"
	"strings"
	"testing"
)

func TestWorkOrderSchemaCreatesMaterialSnapshotOnCleanSchema(t *testing.T) {
	src, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	start := strings.Index(text, "CREATE TABLE IF NOT EXISTS %s.work_orders")
	if start < 0 {
		t.Fatal("schema.go missing work_orders create table DDL")
	}
	end := strings.Index(text[start:], "CREATE INDEX IF NOT EXISTS work_orders_status_idx")
	if end < 0 {
		t.Fatal("schema.go missing work_orders status index after create table")
	}
	workOrdersDDL := text[start : start+end]
	if !strings.Contains(workOrdersDDL, "material_snapshot JSONB NOT NULL DEFAULT '[]'::jsonb") {
		t.Fatal("work_orders clean-schema DDL must create material_snapshot; ALTER before table creation is not enough")
	}
}

func TestProductionLogCompletionColumnMigratesBeforeIndex(t *testing.T) {
	src, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	addColumn := strings.Index(text, "ADD COLUMN IF NOT EXISTS completion_no")
	if addColumn < 0 {
		t.Fatal("schema.go must add completion_no for existing production_logs tables")
	}
	createIndex := strings.Index(text, "production_logs_running_completion_idx")
	if createIndex < 0 {
		t.Fatal("schema.go must create running/completion index")
	}
	if addColumn > createIndex {
		t.Fatal("completion_no must be added before creating indexes that reference it")
	}
}

func TestOperationTemplateSchemaSupportsWorkOrdersAndCosts(t *testing.T) {
	src, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, want := range []string{
		"operation_templates",
		"operation_template_steps",
		"operation_template_id BIGINT NOT NULL DEFAULT 0",
		"operation_template_step_id BIGINT NOT NULL DEFAULT 0",
		"cost_type TEXT NOT NULL DEFAULT ''",
		"cost_rate NUMERIC(12,4) NOT NULL DEFAULT 0",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("production schema must support operation template costing; missing %q", want)
		}
	}
}

func TestWorkOrderSchemaSupportsProcessTemplateSnapshots(t *testing.T) {
	src, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, want := range []string{
		"process_template_id BIGINT NOT NULL DEFAULT 0",
		"process_template_name TEXT NOT NULL DEFAULT ''",
		"process_snapshot_json JSONB NOT NULL DEFAULT '{}'::jsonb",
		"operation_summary_json JSONB NOT NULL DEFAULT '[]'::jsonb",
		"sequence_no INT NOT NULL DEFAULT 1",
		"records_loss BOOLEAN NOT NULL DEFAULT false",
		"parameter_schema_json JSONB NOT NULL DEFAULT '{}'::jsonb",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("production schema must support process template snapshots; missing %q", want)
		}
	}
}
