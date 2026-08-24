package costing

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	appcosting "orderapp/internal/application/costing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestResolveProductBOMSpecIdentityUsesOnlyCurrentDefaultPublishedBOMPostgres(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("pr600_cost_spec_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %[1]s.products(id BIGINT PRIMARY KEY,name TEXT NOT NULL,active BOOLEAN NOT NULL DEFAULT true);
		CREATE TABLE %[1]s.product_bom_spec_migrations(product_id BIGINT PRIMARY KEY,state TEXT NOT NULL);
		CREATE TABLE %[1]s.production_bom_output_bindings(output_type TEXT NOT NULL,output_id BIGINT NOT NULL,is_default BOOLEAN NOT NULL,bom_id BIGINT NOT NULL,bom_version_id BIGINT NOT NULL);
		CREATE TABLE %[1]s.production_bom_versions(id BIGINT PRIMARY KEY,bom_id BIGINT NOT NULL,version_no TEXT NOT NULL,status TEXT NOT NULL);
		CREATE TABLE %[1]s.production_bom_specs(id BIGINT PRIMARY KEY,bom_id BIGINT NOT NULL,code TEXT NOT NULL,barcode TEXT NOT NULL,spec_key TEXT NOT NULL,name TEXT NOT NULL,inventory_unit TEXT NOT NULL);
		CREATE TABLE %[1]s.production_bom_version_variants(id BIGINT PRIMARY KEY,version_id BIGINT NOT NULL,bom_spec_id BIGINT NOT NULL,spec_name_snapshot TEXT NOT NULL,inventory_unit TEXT NOT NULL,is_default BOOLEAN NOT NULL,sort_order INT NOT NULL);

		INSERT INTO %[1]s.products(id,name,active) VALUES(600,'初晓',true);
		INSERT INTO %[1]s.product_bom_spec_migrations(product_id,state) VALUES(600,'cutover');
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status) VALUES(901,90,'V001','published'),(902,91,'V001','published');
		INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,is_default,bom_id,bom_version_id)
		VALUES('product',600,true,90,901),('product',600,false,91,902);
		INSERT INTO %[1]s.production_bom_specs(id,bom_id,code,barcode,spec_key,name,inventory_unit)
		VALUES(701,90,'BOM-SPEC-000701','6900000000701','same-name-a','同名袋','袋'),
		      (702,91,'BOM-SPEC-000702','6900000000702','same-name-b','同名袋','袋');
		INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order)
		VALUES(1701,901,701,'同名袋','袋',true,10),(1702,902,702,'同名袋','袋',true,10);
	`, schema)); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(pool, schema)
	current, err := repo.ResolveProductBOMSpecIdentity(ctx, 600, 701, 1701)
	if err != nil {
		t.Fatal(err)
	}
	if current.BomSpecID != 701 || current.BomVariantID != 1701 || current.SpecCode != "BOM-SPEC-000701" || current.Barcode != "6900000000701" || current.InventoryUnit != "袋" {
		t.Fatalf("current default published identity = %+v", current)
	}
	if _, err := repo.ResolveProductBOMSpecIdentity(ctx, 600, 702, 1702); !errors.Is(err, appcosting.ErrProductBOMSpecIdentityNotFound) {
		t.Fatalf("same-name spec from non-default BOM error = %v", err)
	}
}
