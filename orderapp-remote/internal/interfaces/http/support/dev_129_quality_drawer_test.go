package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQualityInspectionDrawerRequirementSeeds(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, want := range []string{
		"PR-129",
		"DEV-129-01",
		"DEV-129-02",
		"UT-129-01",
		"API-129-01",
		"REV-129-01",
		"生产质检对象选择抽屉",
		"工单质检",
		"原料质检",
		"产品质检",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestQualityInspectionVueUsesWorkspaceDrawerSelection(t *testing.T) {
	view := readDev129File(t, filepath.Join("frontend-vue-shell", "src", "views", "QualityInspectionsView.vue"))
	helper := readDev129File(t, filepath.Join("frontend-vue-shell", "src", "lib", "quality-inspections.js"))
	combined := view + "\n" + helper
	for _, want := range []string{
		"workspace",
		"targetDrawerOpen",
		"qualityTargetTabs",
		"/api/produce/work-orders",
		"/api/stock/material-batches",
		"/api/stock/batches",
		"工单质检",
		"原料质检",
		"产品质检",
		"选择质检对象",
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("quality inspection Vue/helper source missing %q", want)
		}
	}
}

func TestQualityInspectionDrawerDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("docs", "production-flow-user-manual.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		text := readDev129File(t, path)
		for _, want := range []string{"生产质检", "选择质检对象", "工单质检", "原料质检", "产品质检"} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing %q", path, want)
			}
		}
	}
}

func readDev129File(t *testing.T, path string) string {
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
