package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionAcceptanceWIPRequirementSeeds(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, want := range []string{
		"PR-115",
		"DEV-115-01",
		"DEV-115-02",
		"DEV-115-03",
		"UT-115-01",
		"API-115-01",
		"REV-115-01",
		"生产验收",
		"WIP占用可视化",
		"建议领到WIP",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestProductionAcceptanceWIPVueWiring(t *testing.T) {
	root := filepath.Join("frontend-vue-shell", "src")
	app, err := os.ReadFile(filepath.Join(root, "App.vue"))
	if err != nil {
		t.Fatal(err)
	}
	menu, err := os.ReadFile(filepath.Join(root, "lib", "menu-ia.js"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"ProductionAcceptanceView",
		"productionAcceptance",
	} {
		if !strings.Contains(string(app), want) {
			t.Fatalf("App.vue missing %q", want)
		}
	}
	for _, want := range []string{
		"生产验收",
		"productionAcceptance",
	} {
		if !strings.Contains(string(menu), want) {
			t.Fatalf("menu-ia.js missing %q", want)
		}
	}

	plan, err := os.ReadFile(filepath.Join(root, "views", "ProducePlanView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"WIP可用(g)", "建议领到WIP(g)", "wip_transfer_suggestion_g"} {
		if !strings.Contains(string(plan), want) {
			t.Fatalf("ProducePlanView.vue missing %q", want)
		}
	}

	orders, err := os.ReadFile(filepath.Join(root, "views", "WorkOrdersView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"WIP占用", "remaining_reserved_g"} {
		if !strings.Contains(string(orders), want) {
			t.Fatalf("WorkOrdersView.vue missing %q", want)
		}
	}

	warehouse, err := os.ReadFile(filepath.Join(root, "views", "WarehouseInventoryView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"WIP占用", "/api/produce/wip-reservations"} {
		if !strings.Contains(string(warehouse), want) {
			t.Fatalf("WarehouseInventoryView.vue missing %q", want)
		}
	}

	manual, err := os.ReadFile(filepath.Join(root, "views", "ProductionManualView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"生产验收", "建议领到 WIP", "WIP占用"} {
		if !strings.Contains(string(manual), want) {
			t.Fatalf("ProductionManualView.vue missing %q", want)
		}
	}
}

func TestProductionAcceptanceWIPManualDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("docs", "production-flow-user-manual.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(content)
		for _, want := range []string{"生产验收", "建议领到", "WIP占用"} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing %q", path, want)
			}
		}
	}
}
