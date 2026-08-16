package customerfulfillment

import (
	"os"
	"regexp"
	"testing"
)

func TestCustomerFulfillmentBOMSpecResolverLocksMigrationDuringCurrentVariantResolution(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile("bom_spec_identity.go")
	if err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(`(?s)SELECT state FROM %s\.product_bom_spec_migrations WHERE product_id=\$1\s+FOR SHARE`)
	if !pattern.Match(source) {
		t.Fatal("customer fulfillment BOM spec resolver must hold the migration row FOR SHARE while resolving the current default BOM variant")
	}
}
