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
