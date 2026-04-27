package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaterialMasterDetailRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"PR-079",
		"DEV-079-01",
		"DEV-079-02",
		"UT-079-01",
		"API-079-01",
		"REV-079-01",
		"物料主从详情页",
		"包材属性",
		"废弃物料",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("requirements seed missing %q", want)
		}
	}
}
