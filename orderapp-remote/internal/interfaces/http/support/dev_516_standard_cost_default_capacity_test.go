package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev516StandardCostDefaultCapacitySupersededByPR518(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-516-STANDARD-COST-DEFAULT-CAPACITY",
			"当前新业务以 PR-518 为准",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-516-STANDARD-COST-DEFAULT-CAPACITY",
			"当前新业务以 PR-518 为准",
		},
		filepath.Join("docs", "acceptance", "2026-07-02-standard-cost-default-capacity.md"): {
			"PR-516-STANDARD-COST-DEFAULT-CAPACITY",
			"历史验收证据",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-516 superseded marker %q", rel, want)
			}
		}
	}
}
