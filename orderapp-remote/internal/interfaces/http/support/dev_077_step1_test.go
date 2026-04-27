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
		"PR-077",
		"DEV-077-01",
		"DEV-077-02",
		"UT-077-01",
		"API-077-01",
		"REV-077-01",
		"成本参数按分类展示并带小字说明",
		"成本核算页提供右侧快速参数抽屉",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("requirements seed missing %q", want)
		}
	}
}
