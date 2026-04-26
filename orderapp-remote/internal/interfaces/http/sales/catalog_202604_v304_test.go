package sales

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
		"('橘皮乌龙', 454, 24, 48, 49)",
		"('橘皮乌龙', 454, 49, NULL, 46)",
		"('白月光-瑰夏', 227, 2, 6, 190)",
		"('白月光-瑰夏', 227, 7, NULL, 165)",
		"('曲奇拼配', 1000, 24, 49, 81.91)",
		"spec_g BIGINT",
		"price_per_unit",
		"UPDATE p2rms15pepb5ciz.products",
		"AND name NOT IN (SELECT name FROM catalog_import_202604_v304)",
	}
	for _, want := range required {
		if !strings.Contains(sql, want) {
			t.Fatalf("catalog SQL missing %q", want)
		}
	}
}
