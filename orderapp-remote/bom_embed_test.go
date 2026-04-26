package main

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
		"const BOM_REACT_URL = '/bom-react'",
		"bom: { title: 'BOM配方维护', legacyUrl: BOM_REACT_URL }",
		"LegacyMigrationView",
	} {
		if !strings.Contains(appSrc, want) {
			t.Fatalf("frontend-vue-shell/src/App.vue missing %q", want)
		}
	}

	bom, err := os.ReadFile("frontend/src/bom/BomManager.tsx")
	if err != nil {
		t.Fatalf("ReadFile(BomManager.tsx): %v", err)
	}
	bomSrc := string(bom)
	for _, want := range []string{
		"const isEmbeddedInShell = new URLSearchParams(window.location.search).get('embed') === '1'",
		"{!isEmbeddedInShell && <Sidebar />}",
		"style={isEmbeddedInShell ? styles.mainEmbedded : styles.main}",
	} {
		if !strings.Contains(bomSrc, want) {
			t.Fatalf("frontend/src/bom/BomManager.tsx missing %q", want)
		}
	}
}
