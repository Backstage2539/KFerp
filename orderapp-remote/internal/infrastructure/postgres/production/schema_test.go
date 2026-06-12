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
	for _, want := range []string{
		"material_snapshot JSONB NOT NULL DEFAULT '[]'::jsonb",
		"process_template_id BIGINT NOT NULL DEFAULT 0",
		"process_snapshot_json JSONB NOT NULL DEFAULT '{}'::jsonb",
		"operation_summary_json JSONB NOT NULL DEFAULT '[]'::jsonb",
		"production_config_snapshot_json JSONB NOT NULL DEFAULT '{}'::jsonb",
	} {
		if !strings.Contains(workOrdersDDL, want) {
			t.Fatalf("work_orders clean-schema DDL missing %q", want)
		}
	}
}

func TestProductionPlanSchemaCreatesFormalPlanTables(t *testing.T) {
	src, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS %s.production_plans",
		"plan_no TEXT NOT NULL UNIQUE",
		"status TEXT NOT NULL DEFAULT 'draft'",
		"submitted_at TIMESTAMPTZ",
		"CREATE TABLE IF NOT EXISTS %s.production_plan_items",
		"production_plan_id BIGINT NOT NULL",
		"component_snapshot_json JSONB NOT NULL DEFAULT '[]'::jsonb",
		"process_route_snapshot_json JSONB NOT NULL DEFAULT '{}'::jsonb",
		"production_plan_items_plan_idx",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("production schema must create formal production plan tables; missing %q", want)
		}
	}
}

func TestWorkOrderSchemaAllowsReleasedOrdersBeforeRunningItem(t *testing.T) {
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
	for _, forbidden := range []string{
		"running_item_id BIGINT NOT NULL UNIQUE",
		"ON CONFLICT (running_item_id) DO UPDATE",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("released work orders must not depend on a unique nonzero running item; found %q", forbidden)
		}
	}
	for _, want := range []string{
		"running_item_id BIGINT NOT NULL DEFAULT 0",
		"production_plan_id BIGINT NOT NULL DEFAULT 0",
		"production_plan_item_id BIGINT NOT NULL DEFAULT 0",
		"planned_output_g BIGINT NOT NULL DEFAULT 0",
		"order_nos TEXT NOT NULL DEFAULT ''",
		"work_orders_running_item_started_uq",
		"WHERE running_item_id > 0",
		"DROP CONSTRAINT IF EXISTS work_orders_running_item_id_key",
	} {
		if !strings.Contains(text, want) && !strings.Contains(workOrdersDDL, want) {
			t.Fatalf("work_orders schema must support released-before-start lifecycle; missing %q", want)
		}
	}
}

func TestJobCardsSchemaCreatesActualLossColumnsOnCleanSchema(t *testing.T) {
	src, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	start := strings.Index(text, "CREATE TABLE IF NOT EXISTS %s.job_cards")
	if start < 0 {
		t.Fatal("schema.go missing job_cards create table DDL")
	}
	end := strings.Index(text[start:], "CREATE INDEX IF NOT EXISTS job_cards_work_order_idx")
	if end < 0 {
		t.Fatal("schema.go missing job_cards index after create table")
	}
	jobCardsDDL := text[start : start+end]
	for _, want := range []string{
		"actual_input_qty NUMERIC(14,4) NOT NULL DEFAULT 0",
		"actual_output_qty NUMERIC(14,4) NOT NULL DEFAULT 0",
		"actual_loss_rate NUMERIC(10,4) NOT NULL DEFAULT 0",
		"parameter_schema_json JSONB NOT NULL DEFAULT '{}'::jsonb",
	} {
		if !strings.Contains(jobCardsDDL, want) {
			t.Fatalf("job_cards clean-schema DDL missing %q", want)
		}
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

func TestManufacturingPhase2SchemaCreatesStockEntriesAndExecutionColumns(t *testing.T) {
	src, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, want := range []string{
		"ensureStockEntryTables(ctx, pool, schema)",
		"CREATE TABLE IF NOT EXISTS %s.stock_entries",
		"entry_no TEXT NOT NULL UNIQUE",
		"entry_type TEXT NOT NULL DEFAULT ''",
		"work_order_id BIGINT NOT NULL DEFAULT 0",
		"job_card_id BIGINT NOT NULL DEFAULT 0",
		"running_item_id BIGINT NOT NULL DEFAULT 0",
		"CREATE TABLE IF NOT EXISTS %s.stock_entry_items",
		"stock_entry_id BIGINT NOT NULL",
		"from_warehouse TEXT NOT NULL DEFAULT ''",
		"to_warehouse TEXT NOT NULL DEFAULT ''",
		"qty_g BIGINT NOT NULL DEFAULT 0",
		"unit_cost NUMERIC(12,4) NOT NULL DEFAULT 0",
		"stock_entries_work_order_idx",
		"stock_entries_type_idx",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("phase2 stock entry schema missing %q", want)
		}
	}
	for _, want := range []string{
		"status TEXT NOT NULL DEFAULT 'pending'",
		"paused_at TIMESTAMPTZ",
		"resumed_at TIMESTAMPTZ",
		"loss_reason TEXT NOT NULL DEFAULT ''",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("phase2 job card execution schema missing %q", want)
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
		"production_config_snapshot_json JSONB NOT NULL DEFAULT '{}'::jsonb",
		"sequence_no INT NOT NULL DEFAULT 1",
		"records_loss BOOLEAN NOT NULL DEFAULT false",
		"parameter_schema_json JSONB NOT NULL DEFAULT '{}'::jsonb",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("production schema must support process template snapshots; missing %q", want)
		}
	}
}

func TestProductionPlanReadsExpectedLossFromProductProductionConfig(t *testing.T) {
	for _, file := range []string{"plan_queries.go", "repository.go"} {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(src)
		for _, want := range []string{
			"product_production_configs",
			"expected_loss_rate",
			"1 - COALESCE(NULLIF(ppc.expected_loss_rate,0)",
		} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s must read expected loss from product production config; missing %q", file, want)
			}
		}
	}
}
