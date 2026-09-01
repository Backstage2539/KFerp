package customerportal

import (
	"os"
	"strings"
	"testing"
)

func TestBOMSpecResolversDoNotReadMigrationState(t *testing.T) {
	t.Parallel()
	for _, file := range []string{"mall_bom_spec.go", "processing_bom_spec.go"} {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(source), "product_bom_spec_migrations") || strings.Contains(string(source), "legacy_child_sku_bom_spec_mappings") {
			t.Fatalf("%s must resolve only from the current default published BOM", file)
		}
	}
}
