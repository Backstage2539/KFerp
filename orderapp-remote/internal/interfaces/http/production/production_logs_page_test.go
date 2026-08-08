package production

import (
	"os"
	"strings"
	"testing"
)

func TestProductionLogsVueContainsKeyColumns(t *testing.T) {
	body, err := os.ReadFile("frontend-vue-shell/src/views/ProductionLogsView.vue")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	for _, needle := range []string{"生产日志", "实际产出率", "投料数(g)", "完成时间", "/api/produce/logs", "running_item_id", "applyProductionContextParams"} {
		if !strings.Contains(content, needle) {
			t.Fatalf("ProductionLogsView.vue missing %q", needle)
		}
	}
	if strings.Contains(content, "BOM预期产出率") || strings.Contains(content, "row.bom_yield_rate") {
		t.Fatal("ProductionLogsView.vue must show actual yield only")
	}
}

func TestProductionLogPagesExposeJSONAPIAndVueRedirect(t *testing.T) {
	body, err := os.ReadFile("internal/interfaces/http/production/production_logs_page.go")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	for _, want := range []string{`e.GET("/api/produce/logs"`, `target := "/vue-shell?view=produceLogs"`, "RunningItemID", `c.QueryParam("running_item_id")`} {
		if !strings.Contains(content, want) {
			t.Fatalf("production_logs_page.go missing %q", want)
		}
	}
}

func TestProductionMenusContainLogsEntry(t *testing.T) {
	checks := []struct {
		path  string
		wants []string
	}{
		{
			path:  "frontend-vue-shell/src/App.vue",
			wants: []string{"ProductionLogsView", "produceLogs: ProductionLogsView", "produceLogs"},
		},
		{
			path:  "frontend-vue-shell/src/lib/menu-ia.js",
			wants: []string{"produceLogs", "生产日志"},
		},
	}
	for _, tc := range checks {
		body, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatal(err)
		}
		content := string(body)
		for _, want := range tc.wants {
			if !strings.Contains(content, want) {
				t.Fatalf("%s missing %q", tc.path, want)
			}
		}
	}
}
