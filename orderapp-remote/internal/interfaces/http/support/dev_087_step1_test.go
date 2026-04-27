package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWarehouseInventoryMenuRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"PR-087",
		"DEV-087-01",
		"DEV-087-02",
		"DEV-087-03",
		"DEV-087-04",
		"UT-087-01",
		"API-087-01",
		"REV-087-01",
		"仓库库存",
		"库存作业",
		"多成品仓",
		"工单冻结 BOM",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("requirements seed missing %q", want)
		}
	}
}
