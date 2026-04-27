package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCostingExcelRoundedBeanListRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"PR-082",
		"DEV-082-01",
		"DEV-082-02",
		"UT-082-01",
		"API-082-01",
		"REV-082-01",
		"熟豆豆单-3.0",
		"零售豆单-3.0",
		"四舍五入",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("requirements seed missing %q", want)
		}
	}
}
