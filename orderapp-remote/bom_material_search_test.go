package main

import (
	"os"
	"strings"
	"testing"
)

func TestBomManagerSupportsMaterialAutocomplete(t *testing.T) {
	b, err := os.ReadFile("frontend/src/bom/BomManager.tsx")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	required := []string{
		"function normalizeKeyword(v: string)",
		"function MaterialAutocomplete(",
		"placeholder=\"搜索生豆/耗材物料\"",
		"placeholder=\"搜索袋子物料\"",
		"没有匹配的物料",
		"onMouseDown={(e) => {",
	}
	for _, want := range required {
		if !strings.Contains(src, want) {
			t.Fatalf("BOM material search source missing %q", want)
		}
	}
}
