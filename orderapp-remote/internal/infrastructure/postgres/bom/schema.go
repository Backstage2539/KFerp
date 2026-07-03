package bom

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	if err := ensureBomTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureBomVersionTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureProductionBomLibraryTables(ctx, pool, schema); err != nil {
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
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %[1]s.product_bom_sources (
			product_id BIGINT PRIMARY KEY,
			source_type TEXT NOT NULL DEFAULT 'owned',
			source_product_id BIGINT NOT NULL DEFAULT 0,
			source_product_code_snapshot TEXT NOT NULL DEFAULT '',
			source_product_name_snapshot TEXT NOT NULL DEFAULT '',
			source_bom_product_id BIGINT NOT NULL DEFAULT 0,
			source_bom_version_id BIGINT NOT NULL DEFAULT 0,
			source_bom_version_no_snapshot TEXT NOT NULL DEFAULT '',
			derived_from_product_id BIGINT NOT NULL DEFAULT 0,
			derived_from_bom_version_id BIGINT NOT NULL DEFAULT 0,
			derived_at TIMESTAMPTZ,
			derived_by TEXT NOT NULL DEFAULT '',
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
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
ALTER TABLE %[1]s.product_bom_sources ADD COLUMN IF NOT EXISTS source_type TEXT NOT NULL DEFAULT 'owned';
ALTER TABLE %[1]s.product_bom_sources ADD COLUMN IF NOT EXISTS source_product_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.product_bom_sources ADD COLUMN IF NOT EXISTS source_product_code_snapshot TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.product_bom_sources ADD COLUMN IF NOT EXISTS source_product_name_snapshot TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.product_bom_sources ADD COLUMN IF NOT EXISTS source_bom_product_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.product_bom_sources ADD COLUMN IF NOT EXISTS source_bom_version_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.product_bom_sources ADD COLUMN IF NOT EXISTS source_bom_version_no_snapshot TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.product_bom_sources ADD COLUMN IF NOT EXISTS derived_from_product_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.product_bom_sources ADD COLUMN IF NOT EXISTS derived_from_bom_version_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.product_bom_sources ADD COLUMN IF NOT EXISTS derived_at TIMESTAMPTZ;
ALTER TABLE %[1]s.product_bom_sources ADD COLUMN IF NOT EXISTS derived_by TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.product_bom_sources ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
CREATE INDEX IF NOT EXISTS product_bom_sources_source_product_idx ON %[1]s.product_bom_sources(source_product_id);
ALTER TABLE %[1]s.product_bom_items ADD COLUMN IF NOT EXISTS component_type TEXT NOT NULL DEFAULT 'material';
ALTER TABLE %[1]s.product_bom_items ADD COLUMN IF NOT EXISTS component_product_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.product_bom_items ADD COLUMN IF NOT EXISTS component_spec_g BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.product_bom_items ADD COLUMN IF NOT EXISTS consume_unit TEXT NOT NULL DEFAULT 'ratio_pct';
ALTER TABLE %[1]s.product_bom_items ADD COLUMN IF NOT EXISTS qty_per_unit NUMERIC(14,6) NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.product_bom_items ADD COLUMN IF NOT EXISTS unit_cost_snapshot NUMERIC(12,4) NOT NULL DEFAULT 0;
UPDATE %[1]s.product_bom_items SET component_type='material' WHERE COALESCE(component_type,'')='';
UPDATE %[1]s.product_bom_items SET component_product_id=0 WHERE component_product_id IS NULL;
UPDATE %[1]s.product_bom_items SET component_spec_g=0 WHERE component_spec_g IS NULL;
UPDATE %[1]s.product_bom_items SET consume_unit='ratio_pct' WHERE COALESCE(consume_unit,'')='';
UPDATE %[1]s.product_bom_items SET qty_per_unit=0 WHERE qty_per_unit IS NULL;
UPDATE %[1]s.product_bom_items bi
SET unit_cost_snapshot=COALESCE(m.purchase_price,0)
FROM %[1]s.materials m
WHERE bi.component_type='material'
  AND bi.material_id=m.id
  AND COALESCE(bi.unit_cost_snapshot,0)=0;
ALTER TABLE %[1]s.product_bom_items ALTER COLUMN component_type SET DEFAULT 'material';
ALTER TABLE %[1]s.product_bom_items ALTER COLUMN component_product_id SET DEFAULT 0;
ALTER TABLE %[1]s.product_bom_items ALTER COLUMN component_spec_g SET DEFAULT 0;
ALTER TABLE %[1]s.product_bom_items ALTER COLUMN consume_unit SET DEFAULT 'ratio_pct';
ALTER TABLE %[1]s.product_bom_items ALTER COLUMN qty_per_unit SET DEFAULT 0;
ALTER TABLE %[1]s.product_bom_items ALTER COLUMN unit_cost_snapshot SET DEFAULT 0;
ALTER TABLE %[1]s.product_bom_items ALTER COLUMN component_type SET NOT NULL;
ALTER TABLE %[1]s.product_bom_items ALTER COLUMN component_product_id SET NOT NULL;
ALTER TABLE %[1]s.product_bom_items ALTER COLUMN component_spec_g SET NOT NULL;
ALTER TABLE %[1]s.product_bom_items ALTER COLUMN consume_unit SET NOT NULL;
ALTER TABLE %[1]s.product_bom_items ALTER COLUMN qty_per_unit SET NOT NULL;
ALTER TABLE %[1]s.product_bom_items ALTER COLUMN unit_cost_snapshot SET NOT NULL;
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
ALTER TABLE %[1]s.bom_version_items ADD COLUMN IF NOT EXISTS unit_cost_snapshot NUMERIC(12,4) NOT NULL DEFAULT 0;
UPDATE %[1]s.bom_version_items SET component_type='material' WHERE COALESCE(component_type,'')='';
UPDATE %[1]s.bom_version_items SET component_product_id=0 WHERE component_product_id IS NULL;
UPDATE %[1]s.bom_version_items SET component_spec_g=0 WHERE component_spec_g IS NULL;
UPDATE %[1]s.bom_version_items SET consume_unit='ratio_pct' WHERE COALESCE(consume_unit,'')='';
UPDATE %[1]s.bom_version_items SET qty_per_unit=0 WHERE qty_per_unit IS NULL;
UPDATE %[1]s.bom_version_items vi
SET unit_cost_snapshot=COALESCE(m.purchase_price,0)
FROM %[1]s.materials m
WHERE vi.component_type='material'
  AND vi.material_id=m.id
  AND COALESCE(vi.unit_cost_snapshot,0)=0;
ALTER TABLE %[1]s.bom_version_items ALTER COLUMN component_type SET DEFAULT 'material';
ALTER TABLE %[1]s.bom_version_items ALTER COLUMN component_product_id SET DEFAULT 0;
ALTER TABLE %[1]s.bom_version_items ALTER COLUMN component_spec_g SET DEFAULT 0;
ALTER TABLE %[1]s.bom_version_items ALTER COLUMN consume_unit SET DEFAULT 'ratio_pct';
ALTER TABLE %[1]s.bom_version_items ALTER COLUMN qty_per_unit SET DEFAULT 0;
ALTER TABLE %[1]s.bom_version_items ALTER COLUMN unit_cost_snapshot SET DEFAULT 0;
ALTER TABLE %[1]s.bom_version_items ALTER COLUMN component_type SET NOT NULL;
ALTER TABLE %[1]s.bom_version_items ALTER COLUMN component_product_id SET NOT NULL;
ALTER TABLE %[1]s.bom_version_items ALTER COLUMN component_spec_g SET NOT NULL;
ALTER TABLE %[1]s.bom_version_items ALTER COLUMN consume_unit SET NOT NULL;
ALTER TABLE %[1]s.bom_version_items ALTER COLUMN qty_per_unit SET NOT NULL;
ALTER TABLE %[1]s.bom_version_items ALTER COLUMN unit_cost_snapshot SET NOT NULL;
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

func ensureProductionBomLibraryTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %[1]s.production_bom_groups (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL DEFAULT '',
	sort_order INTEGER NOT NULL DEFAULT 100,
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_by TEXT NOT NULL DEFAULT '',
	updated_by TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS production_bom_groups_name_uq
	ON %[1]s.production_bom_groups(lower(name))
	WHERE active=true;

CREATE TABLE IF NOT EXISTS %[1]s.production_bom_group_categories (
	id BIGSERIAL PRIMARY KEY,
	group_id BIGINT NOT NULL,
	name TEXT NOT NULL DEFAULT '',
	sort_order INTEGER NOT NULL DEFAULT 100,
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_by TEXT NOT NULL DEFAULT '',
	updated_by TEXT NOT NULL DEFAULT ''
);
ALTER TABLE %[1]s.production_bom_group_categories ADD COLUMN IF NOT EXISTS active BOOLEAN NOT NULL DEFAULT true;
CREATE UNIQUE INDEX IF NOT EXISTS production_bom_group_categories_name_uq
	ON %[1]s.production_bom_group_categories(group_id, lower(name));
CREATE INDEX IF NOT EXISTS production_bom_group_categories_group_sort_idx
	ON %[1]s.production_bom_group_categories(group_id, sort_order, id);

CREATE TABLE IF NOT EXISTS %[1]s.production_boms (
	id BIGSERIAL PRIMARY KEY,
	code TEXT NOT NULL DEFAULT '',
	name TEXT NOT NULL DEFAULT '',
	output_product_id BIGINT NOT NULL DEFAULT 0,
	group_id BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'active',
	source_bom_id BIGINT NOT NULL DEFAULT 0,
	source_bom_version_id BIGINT NOT NULL DEFAULT 0,
	source_product_id BIGINT NOT NULL DEFAULT 0,
	source_product_code_snapshot TEXT NOT NULL DEFAULT '',
	source_product_name_snapshot TEXT NOT NULL DEFAULT '',
	legacy_product_id BIGINT NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_by TEXT NOT NULL DEFAULT '',
	updated_by TEXT NOT NULL DEFAULT ''
);
ALTER TABLE %[1]s.production_boms ADD COLUMN IF NOT EXISTS group_category_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.production_boms ADD COLUMN IF NOT EXISTS output_product_id BIGINT NOT NULL DEFAULT 0;
CREATE TABLE IF NOT EXISTS %[1]s.product_production_bom_bindings (
	product_id BIGINT PRIMARY KEY,
	bom_id BIGINT NOT NULL,
	bom_version_id BIGINT NOT NULL,
	bound_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	bound_by TEXT NOT NULL DEFAULT ''
);
UPDATE %[1]s.production_boms SET output_product_id=legacy_product_id WHERE output_product_id=0 AND legacy_product_id > 0;
UPDATE %[1]s.production_boms pb
SET output_product_id=b.product_id
FROM %[1]s.product_production_bom_bindings b
WHERE pb.id=b.bom_id AND pb.output_product_id=0 AND b.product_id > 0;
CREATE UNIQUE INDEX IF NOT EXISTS production_boms_code_uq
	ON %[1]s.production_boms(code);
CREATE UNIQUE INDEX IF NOT EXISTS production_boms_legacy_product_uq
	ON %[1]s.production_boms(legacy_product_id)
	WHERE legacy_product_id > 0;
CREATE INDEX IF NOT EXISTS production_boms_group_idx
	ON %[1]s.production_boms(group_id, status);
CREATE INDEX IF NOT EXISTS production_boms_group_category_idx
	ON %[1]s.production_boms(group_id, group_category_id, status);
CREATE INDEX IF NOT EXISTS production_boms_output_product_idx
	ON %[1]s.production_boms(output_product_id, status);

UPDATE %[1]s.production_boms
SET group_id=0, group_category_id=0
WHERE group_id IN (
	SELECT id FROM %[1]s.production_bom_groups WHERE name IN ('默认分组','默认配方组')
);
DELETE FROM %[1]s.production_bom_groups WHERE name IN ('默认分组','默认配方组');

CREATE TABLE IF NOT EXISTS %[1]s.production_bom_versions (
	id BIGSERIAL PRIMARY KEY,
	bom_id BIGINT NOT NULL,
	version_no TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'draft',
	yield_rate NUMERIC(10,4) NOT NULL DEFAULT 0.8000,
	material_loss_rate NUMERIC(10,4) NOT NULL DEFAULT 0,
	output_qty NUMERIC(14,6) NOT NULL DEFAULT 1,
	output_unit TEXT NOT NULL DEFAULT 'unit',
	note TEXT NOT NULL DEFAULT '',
	legacy_product_id BIGINT NOT NULL DEFAULT 0,
	legacy_bom_version_id BIGINT NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	published_at TIMESTAMPTZ,
	created_by TEXT NOT NULL DEFAULT '',
	published_by TEXT NOT NULL DEFAULT ''
);
ALTER TABLE %[1]s.production_bom_versions ADD COLUMN IF NOT EXISTS output_qty NUMERIC(14,6) NOT NULL DEFAULT 1;
ALTER TABLE %[1]s.production_bom_versions ADD COLUMN IF NOT EXISTS output_unit TEXT NOT NULL DEFAULT 'unit';
ALTER TABLE %[1]s.production_bom_versions ADD COLUMN IF NOT EXISTS special_attrs_schema_json JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE %[1]s.production_bom_versions ADD COLUMN IF NOT EXISTS special_attrs_json JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE %[1]s.production_bom_versions ADD COLUMN IF NOT EXISTS process_route_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.production_bom_versions ADD COLUMN IF NOT EXISTS material_loss_rate NUMERIC(10,4) NOT NULL DEFAULT 0;
UPDATE %[1]s.production_bom_versions SET output_qty=1 WHERE output_qty IS NULL OR output_qty <= 0;
UPDATE %[1]s.production_bom_versions SET output_unit='unit' WHERE COALESCE(output_unit,'')='';
UPDATE %[1]s.production_bom_versions SET material_loss_rate=0 WHERE material_loss_rate IS NULL;
UPDATE %[1]s.production_bom_versions SET special_attrs_schema_json='[]'::jsonb WHERE special_attrs_schema_json IS NULL;
UPDATE %[1]s.production_bom_versions SET special_attrs_json='{}'::jsonb WHERE special_attrs_json IS NULL;
UPDATE %[1]s.production_bom_versions SET process_route_id=0 WHERE process_route_id IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS production_bom_versions_bom_version_uq
	ON %[1]s.production_bom_versions(bom_id, version_no);
CREATE UNIQUE INDEX IF NOT EXISTS production_bom_versions_legacy_version_uq
	ON %[1]s.production_bom_versions(legacy_bom_version_id)
	WHERE legacy_bom_version_id > 0;
CREATE INDEX IF NOT EXISTS production_bom_versions_bom_status_idx
	ON %[1]s.production_bom_versions(bom_id, status, id DESC);

CREATE TABLE IF NOT EXISTS %[1]s.production_bom_version_items (
	id BIGSERIAL PRIMARY KEY,
	version_id BIGINT NOT NULL,
	material_id BIGINT NOT NULL DEFAULT 0,
	component_type TEXT NOT NULL DEFAULT 'material',
	component_product_id BIGINT NOT NULL DEFAULT 0,
	component_spec_g BIGINT NOT NULL DEFAULT 0,
	consume_unit TEXT NOT NULL DEFAULT 'ratio_pct',
	qty_per_unit NUMERIC(14,6) NOT NULL DEFAULT 0,
	ratio_pct NUMERIC(10,4) NOT NULL DEFAULT 0,
	material_loss_rate NUMERIC(10,4) NOT NULL DEFAULT 0,
	unit_cost_snapshot NUMERIC(12,4) NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %[1]s.production_bom_version_items ADD COLUMN IF NOT EXISTS material_loss_rate NUMERIC(10,4) NOT NULL DEFAULT 0;
UPDATE %[1]s.production_bom_version_items SET material_loss_rate=0 WHERE material_loss_rate IS NULL;
UPDATE %[1]s.production_bom_versions v
SET material_loss_rate=loss.max_loss
FROM (
	SELECT version_id, MAX(COALESCE(i.material_loss_rate,0)) AS max_loss
	FROM %[1]s.production_bom_version_items i
	GROUP BY version_id
) loss
WHERE v.id=loss.version_id
  AND COALESCE(v.material_loss_rate,0)=0
  AND COALESCE(loss.max_loss,0)>0;
CREATE INDEX IF NOT EXISTS production_bom_version_items_version_idx
	ON %[1]s.production_bom_version_items(version_id, id);

CREATE TABLE IF NOT EXISTS %[1]s.production_bom_version_operation_costs (
	id BIGSERIAL PRIMARY KEY,
	version_id BIGINT NOT NULL,
	operation_id BIGINT NOT NULL DEFAULT 0,
	operation_name TEXT NOT NULL DEFAULT '',
	workstation_id BIGINT NOT NULL DEFAULT 0,
	workstation_name TEXT NOT NULL DEFAULT '',
	workstation_capacity_id BIGINT NOT NULL DEFAULT 0,
	capacity_name TEXT NOT NULL DEFAULT '',
	hourly_rate_snapshot NUMERIC(14,4) NOT NULL DEFAULT 0,
	standard_minutes_snapshot NUMERIC(14,4) NOT NULL DEFAULT 0,
	batch_size_qty_snapshot NUMERIC(14,6) NOT NULL DEFAULT 0,
	batch_size_unit_snapshot TEXT NOT NULL DEFAULT '',
	operation_unit_cost NUMERIC(14,4) NOT NULL DEFAULT 0,
	operation_cost_unit TEXT NOT NULL DEFAULT '',
	sort_order INT NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS production_bom_version_operation_costs_version_idx
	ON %[1]s.production_bom_version_operation_costs(version_id, sort_order, id);

CREATE TABLE IF NOT EXISTS %[1]s.product_production_bom_bindings (
	product_id BIGINT PRIMARY KEY,
	bom_id BIGINT NOT NULL,
	bom_version_id BIGINT NOT NULL,
	bound_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	bound_by TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS product_production_bom_bindings_bom_idx
	ON %[1]s.product_production_bom_bindings(bom_id, bom_version_id);
`, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	if err := backfillProductionBomLibrary(ctx, pool, schema); err != nil {
		return err
	}
	if err := resetInvalidProductionBomGroupCategories(ctx, pool, schema); err != nil {
		return err
	}
	if err := repairEmptyInitialPublishedProductionBomVersions(ctx, pool, schema); err != nil {
		return err
	}
	if err := archiveNonLatestPublishedProductionBomVersions(ctx, pool, schema); err != nil {
		return err
	}
	return backfillProductionBomVersionSpecialAttrs(ctx, pool, schema)
}

func archiveNonLatestPublishedProductionBomVersions(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
WITH ranked AS (
	SELECT id,
	       row_number() OVER (
	         PARTITION BY bom_id
	         ORDER BY published_at DESC NULLS LAST, created_at DESC, id DESC
	       ) AS rn
	FROM %[1]s.production_bom_versions
	WHERE status='published'
)
UPDATE %[1]s.production_bom_versions v
SET status='archived'
FROM ranked
WHERE ranked.id=v.id AND ranked.rn > 1;
CREATE UNIQUE INDEX IF NOT EXISTS production_bom_versions_one_published_uq
	ON %[1]s.production_bom_versions(bom_id)
	WHERE status='published';
`, schema)
	_, err := pool.Exec(ctx, q)
	return err
}

func resetInvalidProductionBomGroupCategories(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
UPDATE %[1]s.production_boms pb
SET group_category_id=0
WHERE group_category_id > 0
  AND NOT EXISTS (
    SELECT 1
    FROM %[1]s.production_bom_group_categories c
    WHERE c.id=pb.group_category_id AND c.group_id=pb.group_id
  );
`, schema))
	return err
}

func repairEmptyInitialPublishedProductionBomVersions(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
UPDATE %[1]s.production_bom_versions v
SET status='draft', published_at=NULL, published_by=''
WHERE v.version_no='V001'
  AND v.status='published'
  AND COALESCE(v.note,'')='初始版本'
  AND NOT EXISTS (
    SELECT 1 FROM %[1]s.production_bom_version_items i WHERE i.version_id=v.id
  )
  AND NOT EXISTS (
    SELECT 1 FROM %[1]s.product_production_bom_bindings b WHERE b.bom_version_id=v.id
  )
  AND NOT EXISTS (
    SELECT 1 FROM %[1]s.production_bom_versions other WHERE other.bom_id=v.bom_id AND other.id<>v.id
  );
`, schema))
	return err
}

func backfillProductionBomLibrary(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	var hasProducts bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, schema+".products").Scan(&hasProducts); err != nil || !hasProducts {
		return err
	}
	q := fmt.Sprintf(`
WITH legacy_products AS (
	SELECT DISTINCT p.id, COALESCE(NULLIF(p.name,''), '商品 ' || p.id::text) AS name,
	       COALESCE(NULLIF(pb.status,''), 'active') AS status
	FROM %[1]s.products p
	LEFT JOIN %[1]s.product_bom pb ON pb.product_id=p.id
	WHERE COALESCE(p.active,true)=true
	  AND (
	    pb.product_id IS NOT NULL
	    OR EXISTS (SELECT 1 FROM %[1]s.product_bom_items bi WHERE bi.product_id=p.id)
	    OR EXISTS (SELECT 1 FROM %[1]s.bom_versions bv WHERE bv.product_id=p.id)
	  )
)
INSERT INTO %[1]s.production_boms(code, name, output_product_id, group_id, status, legacy_product_id, created_by, updated_by)
SELECT 'BOM-' || LPAD(lp.id::text, 6, '0'),
       lp.name || ' 生产 BOM',
       lp.id,
       0,
       CASE WHEN lp.status='inactive' THEN 'inactive' ELSE 'active' END,
       lp.id,
       'system-backfill',
       'system-backfill'
FROM legacy_products lp
ON CONFLICT DO NOTHING;

WITH version_rows AS (
	SELECT pbom.id AS bom_id, bv.product_id, bv.id AS legacy_bom_version_id,
	       COALESCE(NULLIF(bv.version_no,''), 'V' || LPAD(row_number() OVER (PARTITION BY bv.product_id ORDER BY bv.id)::text, 3, '0')) AS version_no,
	       CASE WHEN bv.status='active' THEN 'published' WHEN bv.status='draft' THEN 'draft' ELSE 'archived' END AS status,
	       COALESCE(NULLIF(bv.yield_rate,0), 0.8) AS yield_rate,
	       COALESCE(bv.note,'') AS note,
	       COALESCE(bv.activated_at, bv.created_at) AS published_at,
	       bv.created_at
	FROM %[1]s.bom_versions bv
	JOIN %[1]s.production_boms pbom ON pbom.legacy_product_id=bv.product_id
)
INSERT INTO %[1]s.production_bom_versions(bom_id, version_no, status, yield_rate, output_qty, output_unit, note, legacy_product_id, legacy_bom_version_id, created_at, published_at, created_by, published_by)
SELECT bom_id, version_no, status, yield_rate, 1, 'kg', note, product_id, legacy_bom_version_id, created_at,
       CASE WHEN status='published' THEN published_at ELSE NULL END,
       'system-backfill',
       CASE WHEN status='published' THEN 'system-backfill' ELSE '' END
FROM version_rows
ON CONFLICT DO NOTHING;

WITH fallback_products AS (
	SELECT pbom.id AS bom_id, pbom.legacy_product_id AS product_id,
	       COALESCE(NULLIF(pb.yield_rate,0),0.8) AS yield_rate,
	       COALESCE(pb.updated_at, now()) AS updated_at
	FROM %[1]s.production_boms pbom
	LEFT JOIN %[1]s.product_bom pb ON pb.product_id=pbom.legacy_product_id
	WHERE NOT EXISTS (
		SELECT 1 FROM %[1]s.production_bom_versions v WHERE v.bom_id=pbom.id
	)
)
INSERT INTO %[1]s.production_bom_versions(bom_id, version_no, status, yield_rate, output_qty, output_unit, note, legacy_product_id, legacy_bom_version_id, created_at, published_at, created_by, published_by)
SELECT bom_id, 'V001', 'published', yield_rate, 1, 'kg', '旧 BOM 回填', product_id, 0, updated_at, updated_at, 'system-backfill', 'system-backfill'
FROM fallback_products
ON CONFLICT DO NOTHING;

INSERT INTO %[1]s.production_bom_version_items(version_id, material_id, component_type, component_product_id, component_spec_g, consume_unit, qty_per_unit, ratio_pct, material_loss_rate, unit_cost_snapshot)
SELECT v.id, i.material_id, COALESCE(NULLIF(i.component_type,''),'material'), COALESCE(i.component_product_id,0),
       COALESCE(i.component_spec_g,0), COALESCE(NULLIF(i.consume_unit,''),'ratio_pct'), COALESCE(i.qty_per_unit,0),
       COALESCE(i.ratio_pct,0), 0, COALESCE(i.unit_cost_snapshot,0)
FROM %[1]s.production_bom_versions v
JOIN %[1]s.bom_version_items i ON i.version_id=v.legacy_bom_version_id
WHERE v.legacy_bom_version_id > 0
  AND NOT EXISTS (SELECT 1 FROM %[1]s.production_bom_version_items existing WHERE existing.version_id=v.id)
ORDER BY v.id, i.id;

INSERT INTO %[1]s.production_bom_version_items(version_id, material_id, component_type, component_product_id, component_spec_g, consume_unit, qty_per_unit, ratio_pct, material_loss_rate, unit_cost_snapshot)
SELECT v.id, i.material_id, COALESCE(NULLIF(i.component_type,''),'material'), COALESCE(i.component_product_id,0),
       COALESCE(i.component_spec_g,0), COALESCE(NULLIF(i.consume_unit,''),'ratio_pct'), COALESCE(i.qty_per_unit,0),
       COALESCE(i.ratio_pct,0), 0, COALESCE(i.unit_cost_snapshot,0)
FROM %[1]s.production_bom_versions v
JOIN %[1]s.product_bom_items i ON i.product_id=v.legacy_product_id
WHERE NOT EXISTS (SELECT 1 FROM %[1]s.production_bom_version_items existing WHERE existing.version_id=v.id)
ORDER BY v.id, i.id;

WITH source_rows AS (
	SELECT p.id AS product_id,
	       COALESCE(NULLIF(s.source_type,''), '') AS source_type,
	       COALESCE(NULLIF(s.source_product_id,0), NULLIF(p.base_product_id,0), p.id) AS source_product_id,
	       COALESCE(s.source_bom_version_id,0) AS source_bom_version_id,
	       EXISTS (SELECT 1 FROM %[1]s.product_bom pb WHERE pb.product_id=p.id)
	        OR EXISTS (SELECT 1 FROM %[1]s.product_bom_items bi WHERE bi.product_id=p.id) AS has_own_bom
	FROM %[1]s.products p
	LEFT JOIN %[1]s.product_bom_sources s ON s.product_id=p.id
	WHERE COALESCE(p.active,true)=true
),
target_rows AS (
	SELECT product_id,
	       CASE
	         WHEN source_type IN ('inherit_current','inherit_version') AND source_product_id > 0 THEN source_product_id
	         WHEN source_type='' AND source_product_id <> product_id AND NOT has_own_bom THEN source_product_id
	         ELSE product_id
	       END AS bom_product_id,
	       CASE WHEN source_type='inherit_version' THEN source_bom_version_id ELSE 0 END AS source_bom_version_id
	FROM source_rows
),
binding_rows AS (
	SELECT tr.product_id, pbom.id AS bom_id,
	       COALESCE(locked.id, latest.id) AS bom_version_id
	FROM target_rows tr
	JOIN %[1]s.production_boms pbom ON pbom.legacy_product_id=tr.bom_product_id
	LEFT JOIN %[1]s.production_bom_versions locked ON locked.legacy_bom_version_id=tr.source_bom_version_id AND tr.source_bom_version_id > 0
	LEFT JOIN LATERAL (
		SELECT id
		FROM %[1]s.production_bom_versions v
		WHERE v.bom_id=pbom.id AND v.status='published'
		ORDER BY v.published_at DESC NULLS LAST, v.id DESC
		LIMIT 1
	) latest ON true
)
INSERT INTO %[1]s.product_production_bom_bindings(product_id, bom_id, bom_version_id, bound_by)
SELECT product_id, bom_id, bom_version_id, 'system-backfill'
FROM binding_rows
WHERE bom_version_id > 0
ON CONFLICT DO NOTHING;
`, schema)
	_, err := pool.Exec(ctx, q)
	return err
}

type bomVersionSpecialAttrCandidate struct {
	ProductID  int64
	BomID      int64
	VersionID  int64
	SchemaJSON string
	AttrsJSON  string
}

type bomVersionSpecialAttrGroup struct {
	SchemaJSON string
	AttrsJSON  string
	ProductIDs []int64
}

func backfillProductionBomVersionSpecialAttrs(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	var hasConfigTables bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL AND to_regclass($2) IS NOT NULL`, schema+".product_categories", schema+".product_config_templates").Scan(&hasConfigTables); err != nil || !hasConfigTables {
		return err
	}
	rows, err := pool.Query(ctx, fmt.Sprintf(`
		WITH candidates AS (
			SELECT p.id AS product_id,
			       b.bom_id,
			       b.bom_version_id,
			       COALESCE(
			         NULLIF(pc_config.special_attrs_schema_json::text,'[]'),
			         NULLIF(parent_pc_config.special_attrs_schema_json::text,'[]'),
			         '[]'
			       ) AS schema_json,
			       CASE
			         WHEN COALESCE(NULLIF(p.roast_level,''),'') <> ''
			         THEN jsonb_set(COALESCE(p.special_attrs_json, '{}'::jsonb), '{roast_level}', to_jsonb(p.roast_level), true)::text
			         ELSE COALESCE(p.special_attrs_json::text, '{}')
			       END AS attrs_json
			FROM %[1]s.products p
			JOIN %[1]s.product_production_bom_bindings b ON b.product_id=p.id
			JOIN %[1]s.production_bom_versions v ON v.id=b.bom_version_id
			LEFT JOIN %[1]s.product_categories pc ON pc.id=p.product_category_id AND pc.active=true
			LEFT JOIN %[1]s.product_categories parent_pc ON parent_pc.id=pc.parent_id AND parent_pc.active=true
			LEFT JOIN %[1]s.product_config_templates pc_config ON pc_config.id=pc.product_config_template_id AND pc_config.active=true
			LEFT JOIN %[1]s.product_config_templates parent_pc_config ON parent_pc_config.id=parent_pc.product_config_template_id AND parent_pc_config.active=true
			WHERE COALESCE(v.special_attrs_schema_json, '[]'::jsonb) = '[]'::jsonb
			  AND COALESCE(v.special_attrs_json, '{}'::jsonb) = '{}'::jsonb
		)
		SELECT product_id, bom_id, bom_version_id, schema_json, attrs_json
		FROM candidates
		ORDER BY bom_version_id, product_id
	`, schema))
	if err != nil {
		return err
	}
	defer rows.Close()
	byVersion := map[int64][]bomVersionSpecialAttrCandidate{}
	for rows.Next() {
		var c bomVersionSpecialAttrCandidate
		if err := rows.Scan(&c.ProductID, &c.BomID, &c.VersionID, &c.SchemaJSON, &c.AttrsJSON); err != nil {
			return err
		}
		c.SchemaJSON = strings.TrimSpace(c.SchemaJSON)
		if c.SchemaJSON == "" {
			c.SchemaJSON = "[]"
		}
		c.AttrsJSON = strings.TrimSpace(c.AttrsJSON)
		if c.AttrsJSON == "" {
			c.AttrsJSON = "{}"
		}
		byVersion[c.VersionID] = append(byVersion[c.VersionID], c)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for versionID, candidates := range byVersion {
		if len(candidates) == 0 {
			continue
		}
		groupsBySig := map[string]*bomVersionSpecialAttrGroup{}
		for _, c := range candidates {
			sig := c.SchemaJSON + "\n" + c.AttrsJSON
			group := groupsBySig[sig]
			if group == nil {
				group = &bomVersionSpecialAttrGroup{SchemaJSON: c.SchemaJSON, AttrsJSON: c.AttrsJSON}
				groupsBySig[sig] = group
			}
			group.ProductIDs = append(group.ProductIDs, c.ProductID)
		}
		groups := make([]*bomVersionSpecialAttrGroup, 0, len(groupsBySig))
		for _, group := range groupsBySig {
			groups = append(groups, group)
		}
		sort.SliceStable(groups, func(i, j int) bool {
			if len(groups[i].ProductIDs) != len(groups[j].ProductIDs) {
				return len(groups[i].ProductIDs) > len(groups[j].ProductIDs)
			}
			return groups[i].ProductIDs[0] < groups[j].ProductIDs[0]
		})
		primary := groups[0]
		if primary.SchemaJSON != "[]" || primary.AttrsJSON != "{}" {
			if _, err := pool.Exec(ctx, fmt.Sprintf(`
				UPDATE %s.production_bom_versions
				SET special_attrs_schema_json=$2::jsonb, special_attrs_json=$3::jsonb
				WHERE id=$1
			`, schema), versionID, primary.SchemaJSON, primary.AttrsJSON); err != nil {
				return err
			}
		}
		for _, group := range groups[1:] {
			if err := copyProductionBomForSpecialAttrsConflict(ctx, pool, schema, candidates[0].BomID, versionID, group); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyProductionBomForSpecialAttrsConflict(ctx context.Context, pool *pgxpool.Pool, schema string, sourceBomID int64, sourceVersionID int64, group *bomVersionSpecialAttrGroup) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var name string
	var groupID int64
	var status string
	var yieldRate float64
	var materialLossRate float64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT pb.name, pb.group_id, pb.status, COALESCE(v.yield_rate,0.8)::float8, COALESCE(v.material_loss_rate,0)::float8
		FROM %[1]s.production_boms pb
		JOIN %[1]s.production_bom_versions v ON v.id=$2
		WHERE pb.id=$1
	`, schema), sourceBomID, sourceVersionID).Scan(&name, &groupID, &status, &yieldRate, &materialLossRate); err != nil {
		return err
	}
	var newBomID int64
	tempCode := fmt.Sprintf("PENDING-%d-%d", sourceVersionID, group.ProductIDs[0])
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.production_boms(code, name, output_product_id, group_id, status, source_bom_id, source_bom_version_id, created_by, updated_by)
		VALUES($1,$2,(SELECT output_product_id FROM %[1]s.production_boms WHERE id=$5),$3,$4,$5,$6,'system-special-attrs-backfill','system-special-attrs-backfill')
		RETURNING id
	`, schema), tempCode, name+" 特殊属性副本", groupID, status, sourceBomID, sourceVersionID).Scan(&newBomID); err != nil {
		return err
	}
	code := fmt.Sprintf("BOM-%06d", newBomID)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.production_boms SET code=$1 WHERE id=$2`, schema), code, newBomID); err != nil {
		return err
	}
	var newVersionID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.production_bom_versions(bom_id, version_no, status, yield_rate, output_qty, output_unit, material_loss_rate, note, special_attrs_schema_json, special_attrs_json, created_at, published_at, created_by, published_by)
		VALUES($1,'V001','published',$2,1,'kg',$3,'旧 SKU 特殊属性冲突自动拆分',$4::jsonb,$5::jsonb,now(),now(),'system-special-attrs-backfill','system-special-attrs-backfill')
		RETURNING id
	`, schema), newBomID, yieldRate, materialLossRate, group.SchemaJSON, group.AttrsJSON).Scan(&newVersionID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.production_bom_version_items(version_id, material_id, component_type, component_product_id, component_spec_g, consume_unit, qty_per_unit, ratio_pct, material_loss_rate, unit_cost_snapshot)
		SELECT $1, material_id, component_type, component_product_id, component_spec_g, consume_unit, qty_per_unit, ratio_pct, material_loss_rate, unit_cost_snapshot
		FROM %[1]s.production_bom_version_items
		WHERE version_id=$2
		ORDER BY id
	`, schema), newVersionID, sourceVersionID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %[1]s.product_production_bom_bindings
		SET bom_id=$1, bom_version_id=$2, bound_at=now(), bound_by='system-special-attrs-backfill'
		WHERE product_id=ANY($3)
	`, schema), newBomID, newVersionID, group.ProductIDs); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
