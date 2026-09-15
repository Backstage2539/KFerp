package productspecmigration

import (
	"os"
	"strings"
	"testing"
)

func TestBusinessIdentityGuardUsesTableFreeAuthorityAfterLegacyCleanup(t *testing.T) {
	source, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	start := strings.Index(text, "func ensureBusinessIdentityWriteGuards")
	end := strings.Index(text, "func ensureLegacyChildCatalogWriteGuard")
	if start < 0 || end <= start {
		t.Fatal("business identity guard source block not found")
	}
	guard := text[start:end]
	if !strings.Contains(guard, "product_bom_spec_authorities") {
		t.Fatal("business identity guard must derive current identity from the table-free authority view")
	}
	for _, retired := range []string{"product_bom_spec_migrations", "legacy_child_sku_bom_spec_mappings"} {
		if strings.Contains(guard, retired) {
			t.Fatalf("business identity guard still references retired relation %q", retired)
		}
	}
}
