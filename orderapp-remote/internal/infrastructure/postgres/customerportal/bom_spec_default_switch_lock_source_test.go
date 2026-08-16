package customerportal

import (
	"os"
	"regexp"
	"testing"
)

func TestCutoverBOMSpecResolversLockMigrationDuringCurrentVariantResolution(t *testing.T) {
	t.Parallel()
	pattern := regexp.MustCompile(`(?s)SELECT state FROM %s\.product_bom_spec_migrations WHERE product_id=\$1\s+FOR SHARE`)
	for _, file := range []string{"mall_bom_spec.go", "processing_bom_spec.go"} {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !pattern.Match(source) {
			t.Fatalf("%s must hold the migration row FOR SHARE while resolving the current default BOM variant", file)
		}
	}
}
