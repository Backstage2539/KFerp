package bom

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	if err := ensureBomTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureBomVersionTables(ctx, pool, schema); err != nil {
		return err
	}
	return ensureBagSpecMappingTable(ctx, pool, schema)
}

func ensureBomTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	ddls := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.product_bom (
			product_id BIGINT PRIMARY KEY,
			yield_rate NUMERIC(10,4) NOT NULL DEFAULT 0.8000,
			status TEXT NOT NULL DEFAULT 'active',
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.product_bom_items (
			id BIGSERIAL PRIMARY KEY,
			product_id BIGINT NOT NULL,
			material_id BIGINT NOT NULL,
			ratio_pct NUMERIC(10,4) NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			UNIQUE(product_id, material_id)
		)`, schema),
	}
	for _, q := range ddls {
		if _, err := pool.Exec(ctx, q); err != nil {
			return err
		}
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.product_bom ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active'`, schema)); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.product_bom SET status='active' WHERE COALESCE(NULLIF(status,''),'')=''`, schema)); err != nil {
		return err
	}
	q := fmt.Sprintf(`
ALTER TABLE %[1]s.product_bom_items ADD COLUMN IF NOT EXISTS component_type TEXT NOT NULL DEFAULT 'material';
ALTER TABLE %[1]s.product_bom_items ADD COLUMN IF NOT EXISTS component_product_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.product_bom_items ADD COLUMN IF NOT EXISTS component_spec_g BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.product_bom_items ADD COLUMN IF NOT EXISTS consume_unit TEXT NOT NULL DEFAULT 'ratio_pct';
ALTER TABLE %[1]s.product_bom_items ADD COLUMN IF NOT EXISTS qty_per_unit NUMERIC(14,6) NOT NULL DEFAULT 0;
UPDATE %[1]s.product_bom_items SET component_type='material' WHERE COALESCE(component_type,'')='';
UPDATE %[1]s.product_bom_items SET component_product_id=0 WHERE component_product_id IS NULL;
UPDATE %[1]s.product_bom_items SET component_spec_g=0 WHERE component_spec_g IS NULL;
UPDATE %[1]s.product_bom_items SET consume_unit='ratio_pct' WHERE COALESCE(consume_unit,'')='';
UPDATE %[1]s.product_bom_items SET qty_per_unit=0 WHERE qty_per_unit IS NULL;
ALTER TABLE %[1]s.product_bom_items ALTER COLUMN component_type SET DEFAULT 'material';
ALTER TABLE %[1]s.product_bom_items ALTER COLUMN component_product_id SET DEFAULT 0;
ALTER TABLE %[1]s.product_bom_items ALTER COLUMN component_spec_g SET DEFAULT 0;
ALTER TABLE %[1]s.product_bom_items ALTER COLUMN consume_unit SET DEFAULT 'ratio_pct';
ALTER TABLE %[1]s.product_bom_items ALTER COLUMN qty_per_unit SET DEFAULT 0;
ALTER TABLE %[1]s.product_bom_items ALTER COLUMN component_type SET NOT NULL;
ALTER TABLE %[1]s.product_bom_items ALTER COLUMN component_product_id SET NOT NULL;
ALTER TABLE %[1]s.product_bom_items ALTER COLUMN component_spec_g SET NOT NULL;
ALTER TABLE %[1]s.product_bom_items ALTER COLUMN consume_unit SET NOT NULL;
ALTER TABLE %[1]s.product_bom_items ALTER COLUMN qty_per_unit SET NOT NULL;
ALTER TABLE %[1]s.product_bom_items DROP CONSTRAINT IF EXISTS product_bom_items_product_id_material_id_key;
CREATE UNIQUE INDEX IF NOT EXISTS product_bom_items_material_uq ON %[1]s.product_bom_items(product_id, material_id) WHERE component_type='material';
CREATE UNIQUE INDEX IF NOT EXISTS product_bom_items_finished_product_uq ON %[1]s.product_bom_items(product_id, component_product_id, component_spec_g, consume_unit) WHERE component_type='finished_product';
CREATE INDEX IF NOT EXISTS product_bom_items_component_product_idx ON %[1]s.product_bom_items(component_type, component_product_id);
`, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	return nil
}

func ensureBomVersionTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %[1]s.bom_versions (
	id BIGSERIAL PRIMARY KEY,
	product_id BIGINT NOT NULL,
	version_no TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'draft',
	yield_rate NUMERIC(10,4) NOT NULL DEFAULT 0.8000,
	note TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	activated_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS bom_versions_product_version_uq
	ON %[1]s.bom_versions(product_id, version_no);
CREATE UNIQUE INDEX IF NOT EXISTS bom_versions_one_active_uq
	ON %[1]s.bom_versions(product_id)
	WHERE status='active';

CREATE TABLE IF NOT EXISTS %[1]s.bom_version_items (
	id BIGSERIAL PRIMARY KEY,
	version_id BIGINT NOT NULL,
	material_id BIGINT NOT NULL,
	ratio_pct NUMERIC(10,4) NOT NULL
);
CREATE INDEX IF NOT EXISTS bom_version_items_version_idx
	ON %[1]s.bom_version_items(version_id, id);
ALTER TABLE %[1]s.bom_version_items ADD COLUMN IF NOT EXISTS component_type TEXT NOT NULL DEFAULT 'material';
ALTER TABLE %[1]s.bom_version_items ADD COLUMN IF NOT EXISTS component_product_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.bom_version_items ADD COLUMN IF NOT EXISTS component_spec_g BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.bom_version_items ADD COLUMN IF NOT EXISTS consume_unit TEXT NOT NULL DEFAULT 'ratio_pct';
ALTER TABLE %[1]s.bom_version_items ADD COLUMN IF NOT EXISTS qty_per_unit NUMERIC(14,6) NOT NULL DEFAULT 0;
UPDATE %[1]s.bom_version_items SET component_type='material' WHERE COALESCE(component_type,'')='';
UPDATE %[1]s.bom_version_items SET component_product_id=0 WHERE component_product_id IS NULL;
UPDATE %[1]s.bom_version_items SET component_spec_g=0 WHERE component_spec_g IS NULL;
UPDATE %[1]s.bom_version_items SET consume_unit='ratio_pct' WHERE COALESCE(consume_unit,'')='';
UPDATE %[1]s.bom_version_items SET qty_per_unit=0 WHERE qty_per_unit IS NULL;
ALTER TABLE %[1]s.bom_version_items ALTER COLUMN component_type SET DEFAULT 'material';
ALTER TABLE %[1]s.bom_version_items ALTER COLUMN component_product_id SET DEFAULT 0;
ALTER TABLE %[1]s.bom_version_items ALTER COLUMN component_spec_g SET DEFAULT 0;
ALTER TABLE %[1]s.bom_version_items ALTER COLUMN consume_unit SET DEFAULT 'ratio_pct';
ALTER TABLE %[1]s.bom_version_items ALTER COLUMN qty_per_unit SET DEFAULT 0;
ALTER TABLE %[1]s.bom_version_items ALTER COLUMN component_type SET NOT NULL;
ALTER TABLE %[1]s.bom_version_items ALTER COLUMN component_product_id SET NOT NULL;
ALTER TABLE %[1]s.bom_version_items ALTER COLUMN component_spec_g SET NOT NULL;
ALTER TABLE %[1]s.bom_version_items ALTER COLUMN consume_unit SET NOT NULL;
ALTER TABLE %[1]s.bom_version_items ALTER COLUMN qty_per_unit SET NOT NULL;
`, schema)
	_, err := pool.Exec(ctx, q)
	return err
}

func ensureBagSpecMappingTable(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.packaging_spec_material_map (
		spec_g BIGINT PRIMARY KEY,
		material_id BIGINT NOT NULL,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`, schema)
	_, err := pool.Exec(ctx, q)
	return err
}
