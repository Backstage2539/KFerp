package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBeanProfileChildTableRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"PR-076",
		"DEV-076-01",
		"UT-076-01",
		"API-076-01",
		"REV-076-01",
		"material_bean_profiles",
		"咖啡豆物料子表",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("requirements seed missing %q", want)
		}
	}
}
