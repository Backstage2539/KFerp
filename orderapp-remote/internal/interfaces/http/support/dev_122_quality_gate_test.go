package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQualityGateRequirementSeeds(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, want := range []string{
		"PR-123",
		"DEV-123-01",
		"DEV-123-02",
		"DEV-123-03",
		"UT-123-01",
		"API-123-01",
		"REV-123-01",
		"质检拦截生产/库存",
		"quality_status",
		"冻结批次",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestQualityGateManualDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("docs", "production-flow-user-manual.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		text := readDev122File(t, path)
		for _, want := range []string{"质检状态", "冻结批次", "出库", "成品批次"} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing %q", path, want)
			}
		}
	}
}

func readDev122File(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err == nil {
		return string(b)
	}
	b, err = os.ReadFile(filepath.Join("..", "..", "..", "..", path))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
