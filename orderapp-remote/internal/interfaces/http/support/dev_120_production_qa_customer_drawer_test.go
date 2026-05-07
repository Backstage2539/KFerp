package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionQACustomerDrawerRequirementSeeds(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, want := range []string{
		"PR-120",
		"DEV-120-01",
		"DEV-120-02",
		"DEV-120-03",
		"DEV-120-04",
		"UT-120-01",
		"API-120-01",
		"REV-120-01",
		"生产质检",
		"LEGACY-MAT",
		"抽屉式新增客户",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestProductionQACustomerDrawerVueWiring(t *testing.T) {
	root := filepath.Join("frontend-vue-shell", "src")
	app := readDev120File(t, filepath.Join(root, "App.vue"))
	acceptance := readDev120File(t, filepath.Join(root, "views", "ProductionAcceptanceView.vue"))
	warehouse := readDev120File(t, filepath.Join(root, "views", "WarehouseInventoryView.vue"))
	order := readDev120File(t, filepath.Join(root, "views", "OrderEntryView.vue"))
	parser := readDev120File(t, filepath.Join(root, "lib", "customer-recipient.js"))

	for _, want := range []string{"currentViewParams", "view-params", "event?.detail?.params"} {
		if !strings.Contains(app, want) {
			t.Fatalf("App.vue missing %q", want)
		}
	}
	for _, want := range []string{"view_params", "openView(row)", "params"} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("ProductionAcceptanceView.vue missing %q", want)
		}
	}
	for _, want := range []string{"defineProps", "viewParams", "applyViewParams", "material_batch", "LEGACY-MAT"} {
		if !strings.Contains(warehouse, want) {
			t.Fatalf("WarehouseInventoryView.vue missing %q", want)
		}
	}
	for _, want := range []string{"customerDrawerOpen", "parseRecipientText", "/api/customers", "defaultSourceID", "defaultOrderTypeID"} {
		if !strings.Contains(order, want) {
			t.Fatalf("OrderEntryView.vue missing %q", want)
		}
	}
	for _, want := range []string{"export function parseRecipientText", "recipient_name", "address"} {
		if !strings.Contains(parser, want) {
			t.Fatalf("customer-recipient.js missing %q", want)
		}
	}
}

func TestProductionQACustomerDrawerManualDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		text := readDev120File(t, path)
		for _, want := range []string{"生产质检", "LEGACY-MAT", "录单", "新增客户"} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing %q", path, want)
			}
		}
	}
}

func readDev120File(t *testing.T, path string) string {
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
