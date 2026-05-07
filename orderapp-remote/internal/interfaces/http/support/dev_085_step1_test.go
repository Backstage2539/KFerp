package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBeanListExcelMetadataRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"PR-085",
		"DEV-085-01",
		"DEV-085-02",
		"UT-085-01",
		"API-085-01",
		"REV-085-01",
		"分类并编号",
		"建议出品类型",
		"熟豆豆单-3.0",
		"零售豆单-3.0",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("requirements seed missing %q", want)
		}
	}
}
