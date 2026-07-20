package catalog

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestBackfillProductDefaultSKUsPriorityAndIdempotence(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for catalog postgres tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)

	schema := fmt.Sprintf("test_catalog_default_sku_backfill_%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
	})
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
CREATE SCHEMA %[1]s;
CREATE TABLE %[1]s.product_unit_templates (
	id BIGINT PRIMARY KEY,
	sales_specs_json JSONB NOT NULL DEFAULT '[]'::jsonb
);
CREATE TABLE %[1]s.products (
	id BIGINT PRIMARY KEY,
	parent_product_id BIGINT NOT NULL DEFAULT 0,
	default_sku_id BIGINT NOT NULL DEFAULT 0,
	unit_template_id BIGINT NOT NULL DEFAULT 0,
	active BOOLEAN NOT NULL DEFAULT true,
	derived_spec_status TEXT NOT NULL DEFAULT '',
	auto_derived_sku BOOLEAN NOT NULL DEFAULT false,
	derived_unit_template_id BIGINT NOT NULL DEFAULT 0,
	derived_spec_key TEXT NOT NULL DEFAULT '',
	is_default_sku BOOLEAN NOT NULL DEFAULT false
);

INSERT INTO %[1]s.product_unit_templates(id, sales_specs_json)
VALUES (10, '[{"spec_key":"template-default","default":true}]'::jsonb);

-- A valid parent pointer wins over both the legacy flag and template default.
INSERT INTO %[1]s.products(id, default_sku_id, unit_template_id) VALUES (100,103,10);
INSERT INTO %[1]s.products(id, parent_product_id, derived_spec_key, is_default_sku) VALUES
	(101,100,'legacy-default',true),
	(102,100,'template-default',false),
	(103,100,'current-pointer',false);

-- One valid legacy flag wins over the template default.
INSERT INTO %[1]s.products(id, unit_template_id) VALUES (200,10);
INSERT INTO %[1]s.products(id, parent_product_id, derived_spec_key, is_default_sku) VALUES
	(201,200,'legacy-default',true),
	(202,200,'template-default',false);

-- Ambiguous legacy flags are ignored in favor of the template default.
INSERT INTO %[1]s.products(id, unit_template_id) VALUES (300,10);
INSERT INTO %[1]s.products(id, parent_product_id, derived_spec_key, is_default_sku) VALUES
	(301,300,'legacy-default-a',true),
	(302,300,'template-default',false),
	(303,300,'legacy-default-b',true);

-- Without a usable pointer, legacy flag, or template default, the first child wins.
INSERT INTO %[1]s.products(id, unit_template_id) VALUES (400,10);
INSERT INTO %[1]s.products(id, parent_product_id, derived_spec_key) VALUES
	(401,400,'first-valid'),
	(402,400,'second-valid');
`, schema)); err != nil {
		t.Fatalf("create default SKU backfill fixture: %v", err)
	}

	for run := 1; run <= 2; run++ {
		if err := backfillProductDefaultSKUs(ctx, pool, schema); err != nil {
			t.Fatalf("backfillProductDefaultSKUs run %d: %v", run, err)
		}
		for parentID, wantSKUID := range map[int64]int64{
			100: 103,
			200: 201,
			300: 302,
			400: 401,
		} {
			assertProductDefaultSKU(t, ctx, pool, schema, parentID, wantSKUID)
		}
	}
}

func assertProductDefaultSKU(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, parentID, wantSKUID int64) {
	t.Helper()
	var gotSKUID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT default_sku_id FROM %s.products WHERE id=$1`, schema), parentID).Scan(&gotSKUID); err != nil {
		t.Fatalf("query parent %d default SKU: %v", parentID, err)
	}
	if gotSKUID != wantSKUID {
		t.Fatalf("parent %d default SKU = %d, want %d", parentID, gotSKUID, wantSKUID)
	}

	rows, err := pool.Query(ctx, fmt.Sprintf(`
SELECT id
FROM %s.products
WHERE (id=$1 OR parent_product_id=$1) AND is_default_sku=true
ORDER BY id
`, schema), parentID)
	if err != nil {
		t.Fatalf("query parent %d default SKU flags: %v", parentID, err)
	}
	defer rows.Close()
	var flagged []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan parent %d default SKU flag: %v", parentID, err)
		}
		flagged = append(flagged, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate parent %d default SKU flags: %v", parentID, err)
	}
	if len(flagged) != 1 || flagged[0] != wantSKUID {
		t.Fatalf("parent %d default SKU flags = %v, want [%d]", parentID, flagged, wantSKUID)
	}
}
