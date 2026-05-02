package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev129StockOperationRequirementSeeds(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(content)
	for _, want := range []string{
		"PR-129",
		"DEV-129-01",
		"DEV-129-02",
		"DEV-129-03",
		"UT-129-01",
		"API-129-01",
		"REV-129-01",
		"库存作业下拉框内搜索",
		"生产中WIP不足时打开库存作业抽屉",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestDev129StockOperationVueUsesInDropdownSearchAndUnifiedLayout(t *testing.T) {
	views := map[string]string{
		"MaterialReceiptsView.vue":  "原料入库",
		"WipMaterialsView.vue":     "WIP在制仓",
		"FinishedTransfersView.vue": "成品转仓",
		"StockAdjustmentsView.vue": "库存调整单",
	}
	for file, title := range views {
		content, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "views", file))
		if err != nil {
			t.Fatal(err)
		}
		src := string(content)
		for _, want := range []string{"stock-operation-page", "panel", "operation-grid"} {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing unified stock operation layout marker %q", file, want)
			}
		}
		if !strings.Contains(src, title) {
			t.Fatalf("%s missing title %q", file, title)
		}
	}

	for _, file := range []string{"MaterialReceiptsView.vue", "WipMaterialsView.vue", "FinishedTransfersView.vue"} {
		content, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "views", file))
		if err != nil {
			t.Fatal(err)
		}
		src := string(content)
		if !strings.Contains(src, "SearchableSelect") {
			t.Fatalf("%s must use SearchableSelect for in-dropdown fuzzy search", file)
		}
	}

	receipt, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "views", "MaterialReceiptsView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(receipt), "materialQuery") {
		t.Fatal("MaterialReceiptsView.vue should not keep a separate material search input")
	}
}

func TestDev129ProduceRunningOpensStockOperationDrawerOnWIPShortage(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "views", "ProduceRunningView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(content)
	for _, want := range []string{
		"StockOperationsView",
		"stockDrawerOpen",
		"isWipInsufficientError",
		"initial-tab=\"wip\"",
		"打开库存作业",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("ProduceRunningView.vue missing WIP stock drawer marker %q", want)
		}
	}
}
