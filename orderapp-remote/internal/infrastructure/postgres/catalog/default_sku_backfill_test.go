package catalog

import (
	"os"
	"strings"
	"testing"
)

func TestProductSchemaDoesNotBackfillProductDefaultSKU(t *testing.T) {
	source, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if strings.Contains(text, "backfillProductDefaultSKUs") || strings.Contains(text, "selected_sku_id") {
		t.Fatal("product schema must not restore product-owned default SKU state")
	}
}
