package appmain

import (
	"os"
	"strings"
	"testing"
)

func TestVueShellEmbedsBomWithoutNestedMenu(t *testing.T) {
	app, err := os.ReadFile("frontend-vue-shell/src/App.vue")
	if err != nil {
		t.Fatalf("ReadFile(App.vue): %v", err)
	}
	appSrc := string(app)
	for _, want := range []string{
		"import BomView from './views/BomView.vue'",
		"bom: { title: 'BOM配方维护'",
		"bom: BomView",
	} {
		if !strings.Contains(appSrc, want) {
			t.Fatalf("frontend-vue-shell/src/App.vue missing %q", want)
		}
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
		"/api/bom/list",
		"/api/bom/detail/",
		"/api/bom/item/save",
		"/api/bom/bag-spec-mappings",
	} {
		if !strings.Contains(bomSrc, want) {
			t.Fatalf("frontend-vue-shell/src/views/BomView.vue missing %q", want)
		}
	}
}
