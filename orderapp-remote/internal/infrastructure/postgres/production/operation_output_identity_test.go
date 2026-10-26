package production

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"

	app "orderapp/internal/application/production"
)

func TestResolveOperationOutputProductIdentityAcceptsFrozenLegacyProducts(t *testing.T) {
	ctx := context.Background()
	pool, schema := newProductionTestDB(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
		pool.Close()
	})

	for _, statement := range []string{
		fmt.Sprintf(`CREATE TABLE %s.products(id BIGINT PRIMARY KEY,active BOOLEAN NOT NULL)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.product_bom_spec_authorities(product_id BIGINT PRIMARY KEY,configured BOOLEAN NOT NULL)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.production_bom_output_bindings(output_type TEXT,output_id BIGINT,bom_id BIGINT,bom_version_id BIGINT,is_default BOOLEAN)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.production_bom_versions(id BIGINT PRIMARY KEY,bom_id BIGINT,status TEXT)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.production_boms(id BIGINT PRIMARY KEY,output_type TEXT,output_product_id BIGINT)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.production_bom_specs(id BIGINT PRIMARY KEY,bom_id BIGINT)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.production_bom_version_variants(id BIGINT PRIMARY KEY,version_id BIGINT,bom_spec_id BIGINT)`, schema),
	} {
		mustExecProductionSQL(t, ctx, pool, statement)
	}
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.products VALUES (619,false),(658,true);
		INSERT INTO %s.product_bom_spec_authorities VALUES (619,true),(658,true);
		INSERT INTO %s.production_boms VALUES (8338,'product',619),(8375,'product',658);
		INSERT INTO %s.production_bom_versions VALUES (1544,8338,'archived'),(1841,8338,'published'),(1995,8375,'archived'),(3690,8375,'published');
		INSERT INTO %s.production_bom_specs VALUES (2,8338),(403,8375);
		INSERT INTO %s.production_bom_version_variants VALUES (15,1841,2),(497,1995,403),(498,3690,403);
		INSERT INTO %s.production_bom_output_bindings VALUES ('product',619,8338,1841,true),('product',658,8375,3690,true);
	`, schema, schema, schema, schema, schema, schema, schema))

	t.Run("retired child SKU resolves through inactive parent and current published variant", func(t *testing.T) {
		tx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		got, specID, variantID, err := resolveOperationOutputProductIdentityTx(ctx, tx, schema, app.WorkOrderRow{
			OutputProductID: 765, ProductID: 619, BomSpecID: 2, BomVariantID: 15, BomVersionID: 1544,
		})
		if err != nil {
			t.Fatalf("resolve retired output identity: %v", err)
		}
		if got != 619 || specID != 2 || variantID != 15 {
			t.Fatalf("resolved identity = %d/%d/%d, want 619/2/15", got, specID, variantID)
		}
	})
	t.Run("archived frozen variant remains valid after BOM publication", func(t *testing.T) {
		tx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		got, specID, variantID, err := resolveOperationOutputProductIdentityTx(ctx, tx, schema, app.WorkOrderRow{
			OutputProductID: 658, ProductID: 658, BomSpecID: 403, BomVariantID: 497, BomVersionID: 1995,
		})
		if err != nil {
			t.Fatalf("resolve archived frozen identity: %v", err)
		}
		if got != 658 || specID != 403 || variantID != 497 {
			t.Fatalf("resolved identity = %d/%d/%d, want 658/403/497", got, specID, variantID)
		}
	})
	t.Run("mismatched frozen variant is still rejected", func(t *testing.T) {
		tx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		_, _, _, err = resolveOperationOutputProductIdentityTx(ctx, tx, schema, app.WorkOrderRow{
			OutputProductID: 658, ProductID: 658, BomSpecID: 403, BomVariantID: 999, BomVersionID: 1995,
		})
		if err == nil {
			t.Fatal("expected mismatched frozen variant to fail closed")
		}
	})
}
