package bom

import (
	"os"
	"strings"
	"testing"
)

func TestRepositoryDeleteItemScopesByProductAndAuditsActualRow(t *testing.T) {
	src := readRepositorySource(t)
	for _, want := range []string{
		"WHERE id=$1 AND product_id=$2",
		"bom item not found",
		"row.ComponentType",
		"row.ComponentProductID",
		"product_bom_item",
		"AuditInsertTx(ctx, tx",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("repository delete item missing marker %q", want)
		}
	}
}

func TestRepositoryWritesAuditForBomWritePaths(t *testing.T) {
	src := readRepositorySource(t)
	for _, want := range []string{
		`AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "product_bom",`,
		`AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_bom",`,
		`AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "bom_version",`,
		`AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_bom_item",`,
		`AuditInsert(ctx, pool, schema, cmd.Actor, "packaging_spec_material_map",`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("repository audit coverage missing marker %q", want)
		}
	}
}

func TestBomRepositoryExposesProductKindForGreenBeanFiltering(t *testing.T) {
	src := readRepositorySource(t)
	for _, want := range []string{
		"COALESCE(NULLIF(p.product_kind,''),'roasted_bean')",
		"&item.ProductKind",
		"&opt.ProductKind",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("BOM repository must expose product_kind so green bean SKUs stay out of BOM maintenance; missing %q", want)
		}
	}
}

func TestBomRepositoryExposesOrderUsageForCustomerSkuSorting(t *testing.T) {
	src := readRepositorySource(t)
	for _, want := range []string{
		"order_usage_count",
		"FROM %[1]s.order_items oi",
		"&item.OrderUsageCount",
		"&opt.OrderUsageCount",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("BOM repository must expose order usage for customer/BOM sorting; missing %q", want)
		}
	}
}

func TestBomRepositoryPersistsSourceMetadataAndDeriveAudit(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	repository := readRepositorySource(t)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS %[1]s.product_bom_sources",
		"source_product_code_snapshot",
		"source_bom_version_no_snapshot",
		"derived_at TIMESTAMPTZ",
		"deriveOwnedBomTx",
		`"derive_owned"`,
		`"source_product_code"`,
		`"source_bom_version_no"`,
		"can_edit_bom",
	} {
		if !strings.Contains(string(schema)+"\n"+repository, want) {
			t.Fatalf("BOM source metadata or derive audit missing marker %q", want)
		}
	}
}

func TestProductionBomLibrarySchemaBackfillAndBindingMarkers(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	repository := readRepositorySource(t)
	combined := string(schema) + "\n" + repository
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS %[1]s.production_bom_groups",
		"CREATE TABLE IF NOT EXISTS %[1]s.production_boms",
		"CREATE TABLE IF NOT EXISTS %[1]s.production_bom_versions",
		"CREATE TABLE IF NOT EXISTS %[1]s.production_bom_version_items",
		"CREATE TABLE IF NOT EXISTS %[1]s.product_production_bom_bindings",
		"backfillProductionBomLibrary",
		"inherit_current",
		"inherit_version",
		"derived_owned",
		"product_production_bom_bindings",
		`"bind_production_bom"`,
		`"copy_production_bom"`,
		`"publish_production_bom_version"`,
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("production BOM library implementation missing marker %q", want)
		}
	}
}

func TestProductionBomVersionSpecialAttrsSchemaBackfillAndAuditMarkers(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	repository := readRepositorySource(t)
	combined := string(schema) + "\n" + repository
	for _, want := range []string{
		"ALTER TABLE %[1]s.production_bom_versions ADD COLUMN IF NOT EXISTS special_attrs_schema_json JSONB NOT NULL DEFAULT '[]'::jsonb",
		"ALTER TABLE %[1]s.production_bom_versions ADD COLUMN IF NOT EXISTS special_attrs_json JSONB NOT NULL DEFAULT '{}'::jsonb",
		"backfillProductionBomVersionSpecialAttrs",
		"copyProductionBomForSpecialAttrsConflict",
		"source_bom_version_id",
		"special_attrs_schema_json",
		"special_attrs_json",
		"CASE WHEN $3<>'' THEN $3::jsonb ELSE special_attrs_schema_json END",
		"CASE WHEN $4<>'' THEN $4::jsonb ELSE special_attrs_json END",
		`"update_special_attrs"`,
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("production BOM version special attrs implementation missing marker %q", want)
		}
	}
}

func TestProductionBomGroupsArePureUIFoldersWithDeleteAndSort(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	repository := readRepositorySource(t)
	combined := string(schema) + "\n" + repository
	for _, want := range []string{
		"DeleteProductionBomGroup",
		"MoveProductionBomGroup",
		"move_production_bom_group",
		"delete_production_bom_group",
		"SET group_id=0",
		"sort_order=$2",
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("production BOM group folder behavior missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"DEFAULT_PRODUCTION_BOM_GROUP_NAME",
		"VALUES('默认分组'",
		"VALUES($1,100,true,'system','system')",
		"DisableProductionBomGroup",
		"disable_production_bom_group",
		"include_inactive",
		"ON CONFLICT DO NOTHING;\n\nWITH default_group",
	} {
		if strings.Contains(combined, forbidden) {
			t.Fatalf("production BOM groups should not use inactive/disable model; found %q", forbidden)
		}
	}
}

func TestProductionBomBackfillRepairsLegacyItemsWithoutBindings(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	combined := string(schema) + "\n" + readRepositorySource(t)
	for _, want := range []string{
		"PR-403: repair legacy product BOM rows that still have items but no production BOM binding",
		"missing_legacy_bindings",
		"LEFT JOIN %[1]s.product_production_bom_bindings existing_binding",
		"existing_binding.product_id IS NULL",
		"EXISTS (SELECT 1 FROM %[1]s.product_bom_items bi WHERE bi.product_id=p.id)",
		"INSERT INTO %[1]s.product_production_bom_bindings(product_id, bom_id, bom_version_id, bound_by)",
		"'system-pr403-legacy-binding-repair'",
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("legacy BOM binding repair missing marker %q", want)
		}
	}
}

func TestProductionBomDetailListsReferencedProducts(t *testing.T) {
	repository := readRepositorySource(t)
	for _, want := range []string{
		"listProductionBomReferencedProducts",
		"JOIN %[1]s.products p ON p.id=b.product_id",
		"WHERE b.bom_id=$1",
		"ReferencedProducts: referencedProducts",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("production BOM detail referenced product implementation missing marker %q", want)
		}
	}
}

func TestProductionBomCanDeactivateWhenActiveProductsReferenceIt(t *testing.T) {
	repository := readRepositorySource(t)
	start := strings.Index(repository, "func (r Repository) UpdateProductionBom")
	end := strings.Index(repository, "func (r Repository) CopyProductionBom")
	if start == -1 || end == -1 || end <= start {
		t.Fatalf("cannot locate UpdateProductionBom source")
	}
	updateProductionBom := repository[start:end]
	for _, forbidden := range []string{
		"production BOM is used by active products",
		"deactivate products first",
		"activeReferences",
		"p.active=true",
	} {
		if strings.Contains(updateProductionBom, forbidden) {
			t.Fatalf("production BOM deactivation should not block active product references; found %q", forbidden)
		}
	}
}

func readRepositorySource(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
