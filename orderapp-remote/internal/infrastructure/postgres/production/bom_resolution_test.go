package production

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProductionBOMResolutionFallsBackOnlyFromUnconfiguredSKU(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	ctx := context.Background()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
		CREATE TABLE %[1]s.products (
			id BIGINT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			parent_product_id BIGINT NOT NULL DEFAULT 0,
			active BOOLEAN NOT NULL DEFAULT true
		);
		CREATE TABLE %[1]s.product_production_configs (
			product_id BIGINT PRIMARY KEY,
			production_bom_id BIGINT NOT NULL DEFAULT 0,
			production_bom_version_id BIGINT NOT NULL DEFAULT 0
		);
		CREATE TABLE %[1]s.product_production_bom_bindings (
			product_id BIGINT PRIMARY KEY,
			bom_id BIGINT NOT NULL DEFAULT 0,
			bom_version_id BIGINT NOT NULL DEFAULT 0
		);
		CREATE TABLE %[1]s.production_boms (
			id BIGINT PRIMARY KEY,
			code TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL DEFAULT '',
			output_product_id BIGINT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE %[1]s.process_routes (
			id BIGINT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'active'
		);
		CREATE TABLE %[1]s.production_bom_versions (
			id BIGINT PRIMARY KEY,
			bom_id BIGINT NOT NULL,
			version_no TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'draft',
			process_route_id BIGINT NOT NULL DEFAULT 0,
			yield_rate NUMERIC NOT NULL DEFAULT 1,
			material_loss_rate NUMERIC NOT NULL DEFAULT 0,
			output_qty NUMERIC NOT NULL DEFAULT 1,
			output_unit TEXT NOT NULL DEFAULT 'kg',
			published_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE %[1]s.production_bom_version_items (
			id BIGSERIAL PRIMARY KEY,
			version_id BIGINT NOT NULL
		);

		INSERT INTO %[1]s.products(id,name,parent_product_id,active) VALUES
			(644,'如目达摩',0,true),
			(645,'新父商品',0,true),
			(789,'如目达摩',644,true);
		INSERT INTO %[1]s.process_routes(id,name,status) VALUES (20,'标准烘焙','active');
		INSERT INTO %[1]s.production_boms(id,code,name,output_product_id,status)
		VALUES (10,'PARENT-BOM','父商品 BOM',644,'active');
		INSERT INTO %[1]s.production_bom_versions(
			id,bom_id,version_no,status,process_route_id,yield_rate,output_qty,output_unit,published_at
		) VALUES (11,10,'V001','published',20,1,1,'kg',now());
		INSERT INTO %[1]s.production_bom_version_items(version_id) VALUES (11);
		INSERT INTO %[1]s.product_production_bom_bindings(product_id,bom_id,bom_version_id)
		VALUES (644,10,11);
	`, schema))

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := resolveProductionBomForDemandProductTx(ctx, tx, schema, 789, 644, "如目达摩")
	_ = tx.Rollback(ctx)
	if err != nil {
		t.Fatalf("unconfigured SKU should inherit parent BOM: %v", err)
	}
	if !resolved.BomInherited || resolved.BomSourceProductID != 644 || resolved.BomVersionID != 11 {
		t.Fatalf("inherited BOM=%+v", resolved)
	}

	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %[1]s.products SET parent_product_id=645 WHERE id=789;
	`, schema))
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err = resolveProductionBomForDemandProductTx(ctx, tx, schema, 789, 644, "如目达摩")
	_ = tx.Rollback(ctx)
	if err != nil || !resolved.BomInherited || resolved.BomSourceProductID != 644 {
		t.Fatalf("re-parented SKU must still use frozen parent 644 BOM, resolved=%+v err=%v", resolved, err)
	}

	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %[1]s.products SET active=false WHERE id=644;
	`, schema))
	assertProductionBOMResolutionError(t, ctx, pool, schema, "frozen parent product not found or inactive")
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %[1]s.products SET active=true WHERE id=644;
	`, schema))

	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.production_boms(id,code,name,output_product_id,status)
		VALUES (40,'CHILD-BOM','子规格 BOM',789,'active');
		INSERT INTO %[1]s.production_bom_versions(
			id,bom_id,version_no,status,process_route_id,yield_rate,output_qty,output_unit
		) VALUES (41,40,'DRAFT','draft',20,1,1,'kg');
		INSERT INTO %[1]s.production_bom_version_items(version_id) VALUES (41);
		INSERT INTO %[1]s.product_production_bom_bindings(product_id,bom_id,bom_version_id)
		VALUES (789,40,41);
	`, schema))
	assertProductionBOMResolutionError(t, ctx, pool, schema, "not published")

	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %[1]s.product_production_bom_bindings SET bom_version_id=11 WHERE product_id=789;
	`, schema))
	assertProductionBOMResolutionError(t, ctx, pool, schema, "belongs to another BOM")

	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %[1]s.product_production_bom_bindings SET bom_version_id=0 WHERE product_id=789;
		UPDATE %[1]s.production_boms SET status='inactive' WHERE id=40;
	`, schema))
	assertProductionBOMResolutionError(t, ctx, pool, schema, "no longer an active output BOM")
}

func assertProductionBOMResolutionError(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema, want string) {
	t.Helper()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = resolveProductionBomForDemandProductTx(ctx, tx, schema, 789, 644, "如目达摩")
	_ = tx.Rollback(ctx)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("BOM resolution error=%v, want %q", err, want)
	}
}
