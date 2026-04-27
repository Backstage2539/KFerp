package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBeanProfileModalAndExcelImportRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"PR-077",
		"DEV-077-01",
		"UT-077-01",
		"API-077-01",
		"REV-077-01",
		"咖啡豆信息弹框",
		"Excel 生豆信息",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("requirements seed missing %q", want)
		}
	}
}
