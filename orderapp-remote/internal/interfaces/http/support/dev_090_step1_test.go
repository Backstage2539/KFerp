package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionManualRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"PR-091",
		"DEV-091-01",
		"UT-091-01",
		"API-091-01",
		"REV-091-01",
		"生产流程用户手册展示在前端页面",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("production manual requirement seed missing %q", want)
		}
	}
}

func TestProductionManualVueWiring(t *testing.T) {
	app, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "App.vue"))
	if err != nil {
		t.Fatal(err)
	}
	menu, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js"))
	if err != nil {
		t.Fatal(err)
	}
	view, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "views", "OperationManualView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	manual, err := os.ReadFile(filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"))
	if err != nil {
		t.Fatal(err)
	}
	combined := string(app) + "\n" + string(menu) + "\n" + string(view) + "\n" + string(manual)
	for _, want := range []string{
		"OperationManualView",
		"productionManual",
		"生产手册",
		"原料入库",
		"WIP 在制仓",
		"生产工单",
		"成品批次",
		"生产验收",
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("production manual Vue source missing %q", want)
		}
	}
}
