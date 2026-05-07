package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaterialStockBackfillRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"PR-080",
		"DEV-080-01",
		"DEV-080-02",
		"UT-080-01",
		"API-080-01",
		"REV-080-01",
		"库存补录",
		"补录说明",
		"stock adjustment",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("requirements seed missing %q", want)
		}
	}
}
