package main

import (
	"os"
	"strings"
	"testing"
)

func TestProductCatalog202604V304SQL(t *testing.T) {
	b, err := os.ReadFile("db/product_catalog_202604_v304.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(b)

	required := []string{
		"('橘皮乌龙', 61)",
		"('橘皮乌龙', 24, 48, 49)",
		"('橘皮乌龙', 49, NULL, 46)",
		"('白月光-瑰夏', 1, 3, 380)",
		"('白月光-瑰夏', 3.5, NULL, 330)",
		"('曲奇拼配', 52.86343612334802, 107.9295154185022, 37.18714)",
		"UPDATE p2rms15pepb5ciz.products",
		"AND name NOT IN (SELECT name FROM catalog_import_202604_v304)",
	}
	for _, want := range required {
		if !strings.Contains(sql, want) {
			t.Fatalf("catalog SQL missing %q", want)
		}
	}
}
