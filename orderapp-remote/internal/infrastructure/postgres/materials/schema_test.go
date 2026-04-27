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
	} {
		if strings.Contains(materialsDDL, forbidden) {
			t.Fatalf("materials DDL contains bean profile column %q", forbidden)
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
