package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWIPSharedPoolRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"PR-082",
		"DEV-082-01",
		"DEV-082-02",
		"DEV-082-03",
		"UT-082-01",
		"API-082-01",
		"REV-082-01",
		"WIP共享池",
		"领料不消耗",
		"生产完工从 WIP 扣料",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("requirements seed missing %q", want)
		}
	}
}
