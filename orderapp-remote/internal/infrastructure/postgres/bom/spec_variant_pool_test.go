package bom

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSpecVariantReadersDoNotExhaustSingleConnectionPool(t *testing.T) {
	ctx, pool, schema := newSpecVariantPoolTestDB(t)
	repo := NewRepository(pool, schema)

	t.Run("template variants", func(t *testing.T) {
		callCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		rows, err := repo.listProductionBomSpecTemplateVariants(callCtx, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || len(rows[0].Items) != 1 {
			t.Fatalf("variants/items=%d/%d, want 1/1", len(rows), len(rows[0].Items))
		}
	})

	t.Run("BOM version variants", func(t *testing.T) {
		callCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		rows, err := repo.listProductionBomVersionVariants(callCtx, 20)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || len(rows[0].Items) != 1 {
			t.Fatalf("variants/items=%d/%d, want 1/1", len(rows), len(rows[0].Items))
		}
	})
}

func newSpecVariantPoolTestDB(t *testing.T) (context.Context, *pgxpool.Pool, string) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required")
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.MaxConns = 1
	ctx := context.Background()
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("bom_variant_pool_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %[1]s.materials(id BIGINT PRIMARY KEY,name TEXT NOT NULL DEFAULT '');
		CREATE TABLE %[1]s.products(id BIGINT PRIMARY KEY,name TEXT NOT NULL DEFAULT '');
		CREATE TABLE %[1]s.production_bom_specs(id BIGINT PRIMARY KEY,code TEXT NOT NULL DEFAULT '',barcode TEXT NOT NULL DEFAULT '',spec_key TEXT NOT NULL DEFAULT '');
		CREATE TABLE %[1]s.production_bom_spec_template_variants(id BIGINT PRIMARY KEY,version_id BIGINT NOT NULL,spec_key TEXT NOT NULL,name TEXT NOT NULL,inventory_unit TEXT NOT NULL,is_default BOOLEAN NOT NULL DEFAULT false,sort_order INT NOT NULL DEFAULT 0,material_loss_rate NUMERIC NOT NULL DEFAULT 0,process_route_id BIGINT NOT NULL DEFAULT 0);
		CREATE TABLE %[1]s.production_bom_spec_template_variant_items(id BIGSERIAL PRIMARY KEY,variant_id BIGINT NOT NULL,is_main_input BOOLEAN NOT NULL DEFAULT false,material_id BIGINT NOT NULL DEFAULT 0,component_type TEXT NOT NULL DEFAULT 'material',component_product_id BIGINT NOT NULL DEFAULT 0,component_bom_spec_id BIGINT NOT NULL DEFAULT 0,component_spec_g BIGINT NOT NULL DEFAULT 0,consume_unit TEXT NOT NULL DEFAULT 'kg',qty_per_unit NUMERIC NOT NULL DEFAULT 0,ratio_pct NUMERIC NOT NULL DEFAULT 0,material_loss_rate NUMERIC NOT NULL DEFAULT 0,sort_order INT NOT NULL DEFAULT 0);
		CREATE TABLE %[1]s.production_bom_version_variants(id BIGINT PRIMARY KEY,version_id BIGINT NOT NULL,bom_spec_id BIGINT NOT NULL,spec_name_snapshot TEXT NOT NULL,inventory_unit TEXT NOT NULL,is_default BOOLEAN NOT NULL DEFAULT false,sort_order INT NOT NULL DEFAULT 0,material_loss_rate NUMERIC NOT NULL DEFAULT 0,process_route_id BIGINT NOT NULL DEFAULT 0);
		CREATE TABLE %[1]s.production_bom_version_items(id BIGSERIAL PRIMARY KEY,version_id BIGINT NOT NULL,variant_id BIGINT NOT NULL,material_id BIGINT NOT NULL DEFAULT 0,component_type TEXT NOT NULL DEFAULT 'material',component_product_id BIGINT NOT NULL DEFAULT 0,component_bom_spec_id BIGINT NOT NULL DEFAULT 0,component_spec_g BIGINT NOT NULL DEFAULT 0,consume_unit TEXT NOT NULL DEFAULT 'kg',qty_per_unit NUMERIC NOT NULL DEFAULT 0,ratio_pct NUMERIC NOT NULL DEFAULT 0,material_loss_rate NUMERIC NOT NULL DEFAULT 0);
		INSERT INTO %[1]s.materials(id,name) VALUES(1,'生豆');
		INSERT INTO %[1]s.production_bom_specs(id,code,barcode,spec_key) VALUES(2,'SPEC-2','','1kg');
		INSERT INTO %[1]s.production_bom_spec_template_variants(id,version_id,spec_key,name,inventory_unit,is_default) VALUES(11,10,'1kg','1kg','kg',true);
		INSERT INTO %[1]s.production_bom_spec_template_variant_items(variant_id,is_main_input,material_id,qty_per_unit) VALUES(11,true,1,1);
		INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default) VALUES(21,20,2,'1kg','kg',true);
		INSERT INTO %[1]s.production_bom_version_items(version_id,variant_id,material_id,qty_per_unit) VALUES(20,21,1,1);
	`, schema)); err != nil {
		t.Fatal(err)
	}
	return ctx, pool, schema
}
