package main

import (
	"os"
	"strings"
	"testing"
)

func TestBomViewUsesVueMaterialOptions(t *testing.T) {
	b, err := os.ReadFile("frontend-vue-shell/src/views/BomView.vue")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	required := []string{
		"apiGet('/api/bom/materials')",
		`v-for="material in materials"`,
		"@submit.prevent=\"saveItem\"",
		"@submit.prevent=\"saveMapping\"",
		"选择物料",
	}
	for _, want := range required {
		if !strings.Contains(src, want) {
			t.Fatalf("BomView.vue missing %q", want)
		}
	}
}
