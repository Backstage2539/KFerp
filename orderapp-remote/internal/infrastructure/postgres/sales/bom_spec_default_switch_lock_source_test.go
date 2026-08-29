package sales

import (
	"os"
	"regexp"
	"testing"
)

func TestSalesBOMSpecResolverLocksMigrationDuringCurrentVariantResolution(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(`(?s)SELECT state,.*FROM %s\.product_bom_spec_migrations\s+WHERE product_id=\$1\s+FOR SHARE`)
	if !pattern.Match(source) {
		t.Fatal("sales BOM spec resolver must hold the migration row FOR SHARE while resolving the current default BOM variant")
	}
}
