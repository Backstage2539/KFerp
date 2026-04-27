package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCostingMigrationRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, want := range []string{
		"PR-074",
		"DEV-074-01",
		"UT-074-01",
		"API-074-01",
		"REV-074-01",
		"成本核算",
		"豆单",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("requirements seed missing %q", want)
		}
	}
}
