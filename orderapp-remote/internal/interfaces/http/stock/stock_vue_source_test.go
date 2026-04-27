package stock

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVueShellIncludesWIPMaterialsView(t *testing.T) {
	app, err := readStockWorkspaceFile(filepath.Join("frontend-vue-shell", "src", "App.vue"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(app)
	for _, want := range []string{
		"WipMaterialsView",
		"wipMaterials",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("App.vue missing %q", want)
		}
	}
	menuIA, err := readStockWorkspaceFile(filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(menuIA), "WIP在制仓") {
		t.Fatal("menu-ia.js missing WIP legacy view title")
	}
	view, err := readStockWorkspaceFile(filepath.Join("frontend-vue-shell", "src", "views", "WipMaterialsView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	viewSrc := string(view)
	for _, want := range []string{
		"/api/stock/material-transfers",
		"/api/stock/material-batch-locations",
		"领料到WIP",
		"退回原料仓",
	} {
		if !strings.Contains(viewSrc, want) {
			t.Fatalf("WipMaterialsView.vue missing %q", want)
		}
	}
}

func TestVueStockWorkspaceIncludesFinishedTransferAndTraceLookup(t *testing.T) {
	operations, err := readStockWorkspaceFile(filepath.Join("frontend-vue-shell", "src", "views", "StockOperationsView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	operationsSrc := string(operations)
	for _, want := range []string{
		"成品转仓",
		"FinishedTransfersView",
	} {
		if !strings.Contains(operationsSrc, want) {
			t.Fatalf("StockOperationsView.vue missing %q", want)
		}
	}

	transferView, err := readStockWorkspaceFile(filepath.Join("frontend-vue-shell", "src", "views", "FinishedTransfersView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	transferSrc := string(transferView)
	for _, want := range []string{
		"/api/stock/finished-transfers",
		"from_warehouse",
		"to_warehouse",
		"qty_units",
	} {
		if !strings.Contains(transferSrc, want) {
			t.Fatalf("FinishedTransfersView.vue missing %q", want)
		}
	}

	warehouse, err := readStockWorkspaceFile(filepath.Join("frontend-vue-shell", "src", "views", "WarehouseInventoryView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	warehouseSrc := string(warehouse)
	for _, want := range []string{
		"/api/stock/trace",
		"traceDrawerOpen",
		"追溯",
	} {
		if !strings.Contains(warehouseSrc, want) {
			t.Fatalf("WarehouseInventoryView.vue missing trace lookup %q", want)
		}
	}
}

func readStockWorkspaceFile(path string) ([]byte, error) {
	if b, err := os.ReadFile(path); err == nil {
		return b, nil
	}
	return os.ReadFile(filepath.Join("..", "..", "..", "..", path))
}
