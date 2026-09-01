package sales

import (
	"os"
	"strings"
	"testing"
)

func TestSalesBOMSpecResolverUsesPublishedBOMAuthority(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if !strings.Contains(text, "product_bom_spec_authorities") || strings.Contains(text, "product_bom_spec_migrations") {
		t.Fatal("sales BOM spec resolver must use the published default BOM authority without migration state")
	}
}
