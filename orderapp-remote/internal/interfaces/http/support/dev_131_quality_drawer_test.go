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
		"PR-131",
		"DEV-131-01",
		"DEV-131-02",
		"UT-131-01",
		"API-131-01",
		"REV-131-01",
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
		"选择工单",
		"选择原料批次",
		"选择产品批次",
		"qualityTargetActionLabel(form.scope)",
		"qualityTargetDrawerTitle(activeTargetScope)",
		"qualityTargetSearchPlaceholder(activeTargetScope)",
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("quality inspection Vue/helper source missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"打开对象抽屉",
		"<div class=\"target-tabs\">",
		"class=\"target-tab\"",
		"{{ targetActionLabel",
		"{{ targetDrawerTitle",
		":placeholder=\"targetSearchPlaceholder",
	} {
		if strings.Contains(view, forbidden) {
			t.Fatalf("quality inspection Vue source should not contain %q", forbidden)
		}
	}
}

func TestQualityInspectionDrawerDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		text := readDev129File(t, path)
		for _, want := range []string{"生产质检", "选择工单", "选择原料批次", "选择产品批次"} {
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
