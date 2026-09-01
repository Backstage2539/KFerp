package productspecmigration

import (
	"os"
	"strings"
	"testing"
)

func TestSchemaOwnsMigrationStateMappingsAndCompatibilityColumns(t *testing.T) {
	src, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, marker := range []string{
		"legacy_child_sku_bom_spec_mappings",
		"product_bom_spec_migrations",
		"legacy_catalog_product BOOLEAN NOT NULL DEFAULT true",
		"spec_identity_mode TEXT NOT NULL DEFAULT ''",
		"direct_product_identity_requires_zero_spec",
		"CHECK(state IN ('legacy','preparing','ready','cutover'))",
		"bom_spec_id BIGINT NOT NULL DEFAULT 0",
		"bom_variant_id BIGINT NOT NULL DEFAULT 0",
		"order_items",
		"finished_inventory",
		"production_plan_items",
		"work_orders",
		"customer_direct_ship_request_items",
		"validate_bom_spec_business_identity",
		"legacy_child_sku_write_rejected",
		"bom_spec_id_required",
		"bom_spec_not_published",
		"validate_legacy_child_product_write",
		"legacy_child_sku_cutover_guard",
	} {
		if !strings.Contains(text, marker) {
			t.Fatalf("schema missing %q", marker)
		}
	}
}

func TestGenericBOMSpecResolverUsesPublishedBOMAuthorityWithoutMigrationState(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile("resolver.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if !strings.Contains(text, "product_bom_spec_authorities") || strings.Contains(text, "product_bom_spec_migrations") {
		t.Fatal("generic BOM spec resolver must derive identity from the published default BOM authority only")
	}
}
