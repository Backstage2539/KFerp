package main

import (
	"os"
	"strings"
	"testing"
)

func TestProductionLogsTemplateContainsKeyColumns(t *testing.T) {
	body, err := os.ReadFile("templates/production_logs.html")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	for _, needle := range []string{"生产日志", "真实出品率", "投料数(g)", "完成时间", "materialSummaryText .MaterialSummary"} {
		if !strings.Contains(content, needle) {
			t.Fatalf("production_logs.html missing %q", needle)
		}
	}
	if strings.Contains(content, "<pre>{{.MaterialSummary}}</pre>") {
		t.Fatal("production_logs.html still renders raw material summary json")
	}
}

func TestAppRoutesRegisterProductionLogPages(t *testing.T) {
	body, err := os.ReadFile("app_routes.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "registerProductionLogPages(e, pool, schema)") {
		t.Fatal("app_routes.go missing registerProductionLogPages")
	}
}

func TestProductionMenusContainLogsEntry(t *testing.T) {
	checks := []struct {
		path  string
		wants []string
	}{
		{
			path:  "frontend-vue-shell/src/App.vue",
			wants: []string{"produceLogs", "生产日志", "/produce/logs"},
		},
		{
			path:  "frontend/src/bom/BomManager.tsx",
			wants: []string{"生产日志", "/produce/logs"},
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
