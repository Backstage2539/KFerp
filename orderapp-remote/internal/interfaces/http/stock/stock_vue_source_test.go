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

func readStockWorkspaceFile(path string) ([]byte, error) {
	if b, err := os.ReadFile(path); err == nil {
		return b, nil
	}
	return os.ReadFile(filepath.Join("..", "..", "..", "..", path))
}
