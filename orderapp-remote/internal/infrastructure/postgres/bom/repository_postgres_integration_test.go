package bom

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepairLegacyProductionBomBindingsPostgresOnce(t *testing.T) {
	if os.Getenv("KF_RUN_POSTGRES_INTEGRATION") != "1" {
		t.Skip("set KF_RUN_POSTGRES_INTEGRATION=1 to run against a disposable PostgreSQL schema")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	schema := fmt.Sprintf("codex_pr577_bom_repair_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, dropErr := pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); dropErr != nil {
			t.Errorf("drop test schema: %v", dropErr)
		}
	}()

	ddl := []string{
		`CREATE TABLE ` + schema + `.products (
			id BIGINT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			active BOOLEAN NOT NULL DEFAULT true
		)`,
		`CREATE TABLE ` + schema + `.product_bom (
			product_id BIGINT PRIMARY KEY,
			status TEXT NOT NULL DEFAULT 'active',
			yield_rate NUMERIC(10,4) NOT NULL DEFAULT 1,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE ` + schema + `.product_bom_items (
			id BIGSERIAL PRIMARY KEY,
			product_id BIGINT NOT NULL,
			material_id BIGINT NOT NULL DEFAULT 0,
			component_type TEXT NOT NULL DEFAULT 'material',
			component_product_id BIGINT NOT NULL DEFAULT 0,
			component_spec_g NUMERIC(18,4) NOT NULL DEFAULT 0,
			consume_unit TEXT NOT NULL DEFAULT 'ratio_pct',
			qty_per_unit NUMERIC(18,6) NOT NULL DEFAULT 0,
			ratio_pct NUMERIC(10,4) NOT NULL DEFAULT 0,
			unit_cost_snapshot NUMERIC(18,6) NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE ` + schema + `.production_boms (
			id BIGSERIAL PRIMARY KEY,
			code TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			output_product_id BIGINT NOT NULL,
			group_id BIGINT NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'active',
			legacy_product_id BIGINT NOT NULL UNIQUE,
			created_by TEXT NOT NULL DEFAULT '',
			updated_by TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE ` + schema + `.production_bom_versions (
			id BIGSERIAL PRIMARY KEY,
			bom_id BIGINT NOT NULL,
			version_no TEXT NOT NULL,
			status TEXT NOT NULL,
			yield_rate NUMERIC(10,4) NOT NULL DEFAULT 1,
			output_qty NUMERIC(18,6) NOT NULL DEFAULT 1,
			output_unit TEXT NOT NULL DEFAULT 'kg',
			note TEXT NOT NULL DEFAULT '',
			legacy_product_id BIGINT NOT NULL DEFAULT 0,
			legacy_bom_version_id BIGINT NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			published_at TIMESTAMPTZ,
			created_by TEXT NOT NULL DEFAULT '',
			published_by TEXT NOT NULL DEFAULT '',
			UNIQUE (bom_id, version_no)
		)`,
		`CREATE TABLE ` + schema + `.production_bom_version_items (
			id BIGSERIAL PRIMARY KEY,
			version_id BIGINT NOT NULL,
			material_id BIGINT NOT NULL DEFAULT 0,
			component_type TEXT NOT NULL DEFAULT 'material',
			component_product_id BIGINT NOT NULL DEFAULT 0,
			component_spec_g NUMERIC(18,4) NOT NULL DEFAULT 0,
			consume_unit TEXT NOT NULL DEFAULT 'ratio_pct',
			qty_per_unit NUMERIC(18,6) NOT NULL DEFAULT 0,
			ratio_pct NUMERIC(10,4) NOT NULL DEFAULT 0,
			material_loss_rate NUMERIC(10,4) NOT NULL DEFAULT 0,
			unit_cost_snapshot NUMERIC(18,6) NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE ` + schema + `.product_production_bom_bindings (
			product_id BIGINT PRIMARY KEY,
			bom_id BIGINT NOT NULL,
			bom_version_id BIGINT NOT NULL,
			bound_by TEXT NOT NULL DEFAULT ''
		)`,
	}
	for _, statement := range ddl {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO `+schema+`.products(id,name) VALUES (1,'item-backed'),(2,'empty-shell'),(3,'existing-published'),(5,'intentionally-inactive');
		INSERT INTO `+schema+`.product_bom(product_id,yield_rate) VALUES (1,1),(2,1),(5,1);
		INSERT INTO `+schema+`.product_bom_items(product_id,material_id,ratio_pct,unit_cost_snapshot) VALUES (1,95,100,54),(5,98,100,70);
		INSERT INTO `+schema+`.production_boms(code,name,output_product_id,legacy_product_id,status) VALUES ('BOM-000003','existing',3,3,'active'),('BOM-000005','cut-over source',5,5,'inactive');
		INSERT INTO `+schema+`.production_bom_versions(bom_id,version_no,status,legacy_product_id,published_at)
		SELECT id,'V001','published',legacy_product_id,now() FROM `+schema+`.production_boms WHERE legacy_product_id IN (3,5);
		INSERT INTO `+schema+`.production_bom_version_items(version_id,material_id,ratio_pct,unit_cost_snapshot)
		SELECT id,CASE WHEN legacy_product_id=3 THEN 96 ELSE 98 END,100,CASE WHEN legacy_product_id=3 THEN 60 ELSE 70 END FROM `+schema+`.production_bom_versions WHERE legacy_product_id IN (3,5);
	`); err != nil {
		t.Fatal(err)
	}
	var typedOutputBindingsMissing bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NULL`, schema+`.production_bom_output_bindings`).Scan(&typedOutputBindingsMissing); err != nil {
		t.Fatal(err)
	}
	if !typedOutputBindingsMissing {
		t.Fatal("legacy fixture must not contain production_bom_output_bindings")
	}

	if err := repairLegacyProductionBomBindings(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}

	assertCounts := func(productID int64, wantBoms, wantPublished, wantItems, wantBindings, wantEmptyPublished int) {
		t.Helper()
		var boms, published, items, bindings, emptyPublished int
		if err := pool.QueryRow(ctx, `
			SELECT
			  COUNT(DISTINCT bom.id)::int,
			  COUNT(DISTINCT version.id) FILTER (WHERE version.status='published')::int,
			  COUNT(DISTINCT item.id)::int,
			  COUNT(DISTINCT binding.product_id)::int,
			  COUNT(DISTINCT version.id) FILTER (
			    WHERE version.status='published'
			      AND NOT EXISTS (SELECT 1 FROM `+schema+`.production_bom_version_items check_item WHERE check_item.version_id=version.id)
			  )::int
			FROM `+schema+`.products product
			LEFT JOIN `+schema+`.production_boms bom ON bom.legacy_product_id=product.id
			LEFT JOIN `+schema+`.production_bom_versions version ON version.bom_id=bom.id
			LEFT JOIN `+schema+`.production_bom_version_items item ON item.version_id=version.id
			LEFT JOIN `+schema+`.product_production_bom_bindings binding ON binding.product_id=product.id
			WHERE product.id=$1
		`, productID).Scan(&boms, &published, &items, &bindings, &emptyPublished); err != nil {
			t.Fatal(err)
		}
		if boms != wantBoms || published != wantPublished || items != wantItems || bindings != wantBindings || emptyPublished != wantEmptyPublished {
			t.Fatalf("product %d counts = boms %d published %d items %d bindings %d empty_published %d", productID, boms, published, items, bindings, emptyPublished)
		}
	}

	assertCounts(1, 1, 1, 1, 1, 0)
	assertCounts(2, 0, 0, 0, 0, 0)
	assertCounts(3, 1, 1, 1, 1, 0)
	assertCounts(5, 1, 1, 1, 0, 0)

	if err := repairLegacyProductionBomBindings(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	assertCounts(1, 1, 1, 1, 1, 0)

	if _, err := pool.Exec(ctx, `
		INSERT INTO `+schema+`.products(id,name) VALUES (4,'concurrent-existing-empty');
		INSERT INTO `+schema+`.product_bom(product_id,yield_rate) VALUES (4,1);
		INSERT INTO `+schema+`.product_bom_items(product_id,material_id,ratio_pct,unit_cost_snapshot) VALUES (4,97,100,66);
		INSERT INTO `+schema+`.production_boms(code,name,output_product_id,legacy_product_id) VALUES ('BOM-000004','concurrent-empty',4,4);
		INSERT INTO `+schema+`.production_bom_versions(bom_id,version_no,status,legacy_product_id,published_at)
		SELECT id,'V001','published',4,now() FROM `+schema+`.production_boms WHERE legacy_product_id=4;
	`); err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	errs := make(chan error, 2)
	var workers sync.WaitGroup
	workers.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer workers.Done()
			<-start
			errs <- repairLegacyProductionBomBindings(ctx, pool, schema)
		}()
	}
	close(start)
	workers.Wait()
	close(errs)
	for repairErr := range errs {
		if repairErr != nil {
			t.Fatal(repairErr)
		}
	}
	assertCounts(4, 1, 1, 1, 1, 0)
}
