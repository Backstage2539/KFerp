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

func readRepositorySource(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
