package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManufacturingPlanningQualityRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"PR-110",
		"DEV-110-01",
		"DEV-110-02",
		"DEV-110-03",
		"DEV-110-04",
		"UT-110-01",
		"API-110-01",
		"REV-110-01",
		"物料需求计划",
		"WIP软占用",
		"部分完工",
		"生产质检闭环",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("manufacturing requirement seed missing %q", want)
		}
	}
}

func TestManufacturingPlanningQualityVueWiring(t *testing.T) {
	app, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "App.vue"))
	if err != nil {
		t.Fatal(err)
	}
	menu, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js"))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	running, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "views", "ProduceRunningView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	quality, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "views", "QualityInspectionsView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	combined := string(app) + "\n" + string(menu) + "\n" + string(plan) + "\n" + string(running) + "\n" + string(quality)
	for _, want := range []string{
		"QualityInspectionsView",
		"qualityInspections",
		"生产质检",
		"物料需求计划",
		"缺料(g)",
		"采购建议(g)",
		"部分完工",
		"本次消耗投料(g)",
		"质检范围",
		"检查结果",
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("manufacturing Vue source missing %q", want)
		}
	}
}
