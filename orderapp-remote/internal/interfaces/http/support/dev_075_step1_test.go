package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCostingSettingsAndMaterialMetadataRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, want := range []string{
		"PR-075",
		"DEV-075-01",
		"UT-075-01",
		"API-075-01",
		"REV-075-01",
		"成本参数设置",
		"批次号",
		"2-13磅",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("requirements seed missing %q", want)
		}
	}
}
