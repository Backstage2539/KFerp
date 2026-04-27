package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCostingSettingsDrawerRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, want := range []string{
		"PR-078",
		"DEV-078-01",
		"DEV-078-02",
		"UT-078-01",
		"API-078-01",
		"REV-078-01",
		"成本参数按分类展示并带小字说明",
		"成本核算页提供右侧快速参数抽屉",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("requirements seed missing %q", want)
		}
	}
}
