package bom

import (
	"os"
	"strings"
	"testing"
)

func TestVueShellEmbedsBomWithoutNestedMenu(t *testing.T) {
	app, err := os.ReadFile("frontend-vue-shell/src/views/ProductionSettingsView.vue")
	if err != nil {
		t.Fatalf("ReadFile(ProductionSettingsView.vue): %v", err)
	}
	appSrc := string(app)
	for _, want := range []string{
		"import BomView from './BomView.vue'",
		"key: 'bom'",
		"label: '生产 BOM'",
	} {
		if !strings.Contains(appSrc, want) {
			t.Fatalf("frontend-vue-shell/src/views/ProductionSettingsView.vue missing %q", want)
		}
	}
	menuIA, err := os.ReadFile("frontend-vue-shell/src/lib/menu-ia.js")
	if err != nil {
		t.Fatalf("ReadFile(menu-ia.js): %v", err)
	}
	menuSrc := string(menuIA)
	if !strings.Contains(menuSrc, "bom: '生产 BOM'") {
		t.Fatal("frontend-vue-shell/src/lib/menu-ia.js missing legacy BOM view title")
	}
	if strings.Contains(menuSrc, "{ key: 'bom', label: '生产 BOM'") {
		t.Fatal("frontend-vue-shell/src/lib/menu-ia.js should not keep a standalone BOM menu entry")
	}
	for _, bad := range []string{"BOM_REACT_URL", "legacyUrl: BOM_REACT_URL"} {
		if strings.Contains(appSrc, bad) {
			t.Fatalf("frontend-vue-shell/src/App.vue should not contain %q", bad)
		}
	}

	bom, err := os.ReadFile("frontend-vue-shell/src/views/BomView.vue")
	if err != nil {
		t.Fatalf("ReadFile(BomView.vue): %v", err)
	}
	bomSrc := string(bom)
	for _, want := range []string{
		"/api/production-boms?status=all",
		"/api/production-boms/${id}${query}",
		"/api/production-bom-versions/${draftVersionID}/draft",
		"/api/product-settings/units",
	} {
		if !strings.Contains(bomSrc, want) {
			t.Fatalf("frontend-vue-shell/src/views/BomView.vue missing %q", want)
		}
	}
}
