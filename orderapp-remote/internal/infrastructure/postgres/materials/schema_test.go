package materials

import (
	"os"
	"strings"
	"testing"
)

func TestMaterialsSchemaSeparatesBeanProfileTable(t *testing.T) {
	b, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	if !strings.Contains(src, "material_bean_profiles") {
		t.Fatalf("schema must create material_bean_profiles child table")
	}
	if !strings.Contains(src, "material_pack_profiles") {
		t.Fatalf("schema must create material_pack_profiles child table")
	}
	if !strings.Contains(src, "deprecated_at TIMESTAMPTZ") {
		t.Fatalf("materials schema must support deprecating old materials")
	}
	for _, want := range []string{
		"cost_unit TEXT NOT NULL DEFAULT 'kg'",
		"ALTER TABLE %[1]s.materials ADD COLUMN IF NOT EXISTS cost_unit",
		"ALTER TABLE %[1]s.materials ALTER COLUMN cost_unit SET DEFAULT 'kg'",
		"industry_field_template_id BIGINT NOT NULL DEFAULT 0",
		"material_industry_field_values",
		"material_classification_groups",
		"material_classification_group_categories",
		"material_classification_assignments",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("materials schema missing %q", want)
		}
	}
	if !strings.Contains(src, "THEN 'kg'") || !strings.Contains(src, "ELSE COALESCE(NULLIF(unit,''),'unit')") {
		t.Fatal("materials schema must backfill weight costs to kg and discrete costs to inventory unit")
	}
	materialsDDL := between(t, src, "CREATE TABLE IF NOT EXISTS %s.materials", ")`, schema)")
	for _, forbidden := range []string{
		"origin TEXT",
		"processing_station TEXT",
		"variety TEXT",
		"process_method TEXT",
		"grade TEXT",
		"altitude TEXT",
		"flavor TEXT",
		"bean_list_note TEXT",
		"size_spec TEXT",
		"dimensions TEXT",
		"material_texture TEXT",
		"capacity TEXT",
		"color TEXT",
	} {
		if strings.Contains(materialsDDL, forbidden) {
			t.Fatalf("materials DDL contains type-specific profile column %q", forbidden)
		}
	}
}

func between(t *testing.T, src, start, end string) string {
	t.Helper()
	i := strings.Index(src, start)
	if i < 0 {
		t.Fatalf("missing %q", start)
	}
	j := strings.Index(src[i:], end)
	if j < 0 {
		t.Fatalf("missing %q after %q", end, start)
	}
	return src[i : i+j]
}
