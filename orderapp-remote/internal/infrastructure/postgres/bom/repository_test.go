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

func readRepositorySource(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
