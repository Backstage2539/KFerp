package manufacturing

import (
	"os"
	"strings"
	"testing"
)

func TestManufacturingSchemaDefinesProcessAndIndustryTemplates(t *testing.T) {
	src, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS %[1]s.process_templates",
		"CREATE TABLE IF NOT EXISTS %[1]s.process_template_operations",
		"CREATE TABLE IF NOT EXISTS %[1]s.industry_field_templates",
		"CREATE TABLE IF NOT EXISTS %[1]s.industry_field_definitions",
		"parameter_schema_json JSONB NOT NULL DEFAULT '{}'::jsonb",
		"records_loss BOOLEAN NOT NULL DEFAULT false",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("schema missing %q", want)
		}
	}
}

func TestManufacturingSchemaCreatesProcessRoutesWithoutProductParameters(t *testing.T) {
	src, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS %[1]s.process_routes",
		"CREATE TABLE IF NOT EXISTS %[1]s.process_route_operations",
		"records_loss BOOLEAN NOT NULL DEFAULT false",
		"quality_checklist_json JSONB NOT NULL DEFAULT '[]'::jsonb",
		"process_route_operations_route_seq_uq",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("process route schema missing marker %q", want)
		}
	}
	processRouteStart := strings.Index(text, "CREATE TABLE IF NOT EXISTS %[1]s.process_routes")
	processRouteEnd := strings.Index(text[processRouteStart:], "CREATE TABLE IF NOT EXISTS %[1]s.process_route_operations")
	routeDDL := ""
	if processRouteStart >= 0 && processRouteEnd > 0 {
		routeDDL = text[processRouteStart : processRouteStart+processRouteEnd]
	}
	for _, forbidden := range []string{
		"key_params_json JSONB",
		"expected_loss_rate",
		"roast_level",
	} {
		if strings.Contains(routeDDL, forbidden) {
			t.Fatalf("process routes must not store product-specific parameters; found %q", forbidden)
		}
	}
}

func TestManufacturingSchemaAddsOperationAndWorkstationMasterData(t *testing.T) {
	src, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS %[1]s.manufacturing_operations",
		"CREATE TABLE IF NOT EXISTS %[1]s.manufacturing_workstations",
		"ALTER TABLE %[1]s.process_route_operations ADD COLUMN IF NOT EXISTS operation_id",
		"ALTER TABLE %[1]s.process_route_operations ADD COLUMN IF NOT EXISTS workstation_id",
		"ALTER TABLE %[1]s.process_template_operations ADD COLUMN IF NOT EXISTS operation_id",
		"ALTER TABLE %[1]s.process_template_operations ADD COLUMN IF NOT EXISTS workstation_id",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("manufacturing schema missing master-data marker %q", want)
		}
	}
}

func TestManufacturingSchemaAddsWorkstationCapacitiesAndRouteCostSnapshots(t *testing.T) {
	src, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS %[1]s.manufacturing_workstation_capacities",
		"workstation_id BIGINT NOT NULL DEFAULT 0",
		"batch_size_qty NUMERIC(14,4) NOT NULL DEFAULT 0",
		"batch_size_unit TEXT NOT NULL DEFAULT ''",
		"standard_minutes INT NOT NULL DEFAULT 0",
		"hourly_rate NUMERIC(14,4) NOT NULL DEFAULT 0",
		"production_capacity INT NOT NULL DEFAULT 1",
		"manufacturing_workstation_capacities_workstation_idx",
		"ALTER TABLE %[1]s.process_route_operations ADD COLUMN IF NOT EXISTS workstation_capacity_id",
		"ALTER TABLE %[1]s.process_route_operations ADD COLUMN IF NOT EXISTS workstation_capacity_name",
		"ALTER TABLE %[1]s.process_route_operations ADD COLUMN IF NOT EXISTS batch_size_qty",
		"ALTER TABLE %[1]s.process_route_operations ADD COLUMN IF NOT EXISTS standard_minutes",
		"ALTER TABLE %[1]s.process_route_operations ADD COLUMN IF NOT EXISTS planned_operation_cost",
		"ALTER TABLE %[1]s.process_template_operations ADD COLUMN IF NOT EXISTS workstation_capacity_id",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("manufacturing schema missing workstation capacity marker %q", want)
		}
	}
}
