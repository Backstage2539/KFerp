package customerfulfillment

import (
	"os"
	"strings"
	"testing"
)

func TestCustomerFulfillmentBOMSpecResolverUsesPublishedBOMAuthority(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile("bom_spec_identity.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if !strings.Contains(text, "product_bom_spec_authorities") || strings.Contains(text, "product_bom_spec_migrations") {
		t.Fatal("customer fulfillment BOM spec resolver must use the published default BOM authority without migration state")
	}
}
