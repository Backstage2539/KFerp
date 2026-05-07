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
