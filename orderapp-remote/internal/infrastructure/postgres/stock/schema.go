package stock

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	if err := ensureStockLedgerTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureMaterialBatchTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureWarehouseTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := migrateWarehousesToBusinessGroups(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureFinishedInventoryWarehouse(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureMaterialTransferTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureFinishedProductTransferTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureUnifiedStockDocumentTables(ctx, pool, schema); err != nil {
		return err
	}
	return ensureStockAdjustmentTables(ctx, pool, schema)
}

func ensureUnifiedStockDocumentTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.stock_entries (
	id BIGSERIAL PRIMARY KEY,
	entry_no TEXT NOT NULL UNIQUE,
	entry_type TEXT NOT NULL DEFAULT '',
	purpose TEXT NOT NULL DEFAULT '',
	is_return BOOLEAN NOT NULL DEFAULT false,
	status TEXT NOT NULL DEFAULT 'draft',
	work_order_id BIGINT NOT NULL DEFAULT 0,
	job_card_id BIGINT NOT NULL DEFAULT 0,
	running_item_id BIGINT NOT NULL DEFAULT 0,
	source_type TEXT NOT NULL DEFAULT '',
	source_id BIGINT NOT NULL DEFAULT 0,
	return_source TEXT NOT NULL DEFAULT '',
	operator TEXT NOT NULL DEFAULT '',
	note TEXT NOT NULL DEFAULT '',
	idempotency_key TEXT NOT NULL DEFAULT '',
	legacy BOOLEAN NOT NULL DEFAULT false,
	submitted_at TIMESTAMPTZ,
	cancelled_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS stock_entries_work_order_idx ON %s.stock_entries(work_order_id, created_at DESC);
CREATE INDEX IF NOT EXISTS stock_entries_type_idx ON %s.stock_entries(entry_type, status, created_at DESC);

CREATE TABLE IF NOT EXISTS %s.stock_entry_items (
	id BIGSERIAL PRIMARY KEY,
	stock_entry_id BIGINT NOT NULL,
	material_id BIGINT NOT NULL DEFAULT 0,
	product_id BIGINT NOT NULL DEFAULT 0,
	item_type TEXT NOT NULL DEFAULT '',
	item_name TEXT NOT NULL DEFAULT '',
	owner_customer_id BIGINT NOT NULL DEFAULT 0,
	spec_g BIGINT NOT NULL DEFAULT 0,
	bom_spec_id BIGINT NOT NULL DEFAULT 0,
	bom_variant_id BIGINT NOT NULL DEFAULT 0,
	inventory_unit TEXT NOT NULL DEFAULT '',
	from_warehouse TEXT NOT NULL DEFAULT '',
	to_warehouse TEXT NOT NULL DEFAULT '',
	qty_g BIGINT NOT NULL DEFAULT 0,
	qty_units BIGINT NOT NULL DEFAULT 0,
	batch_code TEXT NOT NULL DEFAULT '',
	unit_cost NUMERIC(12,4) NOT NULL DEFAULT 0,
	total_cost NUMERIC(12,4) NOT NULL DEFAULT 0,
	supplier TEXT NOT NULL DEFAULT '',
	crop_season TEXT NOT NULL DEFAULT '',
	origin TEXT NOT NULL DEFAULT '',
	producer_flavor_description TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS stock_entry_items_entry_idx ON %s.stock_entry_items(stock_entry_id, id);

CREATE TABLE IF NOT EXISTS %s.stock_entry_batch_allocations (
	id BIGSERIAL PRIMARY KEY,
	stock_entry_item_id BIGINT NOT NULL,
	material_batch_id BIGINT NOT NULL DEFAULT 0,
	batch_code TEXT NOT NULL DEFAULT '',
	qty_g BIGINT NOT NULL DEFAULT 0,
	qty_units BIGINT NOT NULL DEFAULT 0,
	unit_cost NUMERIC(12,4) NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS stock_entry_batch_allocations_item_idx
	ON %s.stock_entry_batch_allocations(stock_entry_item_id, id);
`, schema, schema, schema, schema, schema, schema, schema)); err != nil {
		return err
	}
	for _, stmt := range []string{
		fmt.Sprintf(`ALTER TABLE %s.stock_entries ADD COLUMN IF NOT EXISTS purpose TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entries ADD COLUMN IF NOT EXISTS is_return BOOLEAN NOT NULL DEFAULT false`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entries ADD COLUMN IF NOT EXISTS return_source TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entries ADD COLUMN IF NOT EXISTS idempotency_key TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entries ADD COLUMN IF NOT EXISTS legacy BOOLEAN NOT NULL DEFAULT false`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entries ADD COLUMN IF NOT EXISTS submitted_at TIMESTAMPTZ`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entries ADD COLUMN IF NOT EXISTS cancelled_at TIMESTAMPTZ`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entries ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entry_items ADD COLUMN IF NOT EXISTS inventory_unit TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entry_items ADD COLUMN IF NOT EXISTS owner_customer_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entry_items ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entry_items ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entry_items ADD COLUMN IF NOT EXISTS supplier TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entry_items ADD COLUMN IF NOT EXISTS crop_season TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entry_items ADD COLUMN IF NOT EXISTS origin TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entry_items ADD COLUMN IF NOT EXISTS producer_flavor_description TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`UPDATE %s.stock_entries SET purpose=CASE entry_type WHEN 'material_issue_to_wip' THEN 'material_transfer_for_manufacture' WHEN 'wip_return' THEN 'material_transfer_for_manufacture' WHEN 'material_consume' THEN 'material_consumption_for_manufacture' WHEN 'finished_receipt' THEN 'manufacture' WHEN 'finished_transfer' THEN 'material_transfer' ELSE entry_type END, is_return=(entry_type='wip_return'), legacy=true, submitted_at=COALESCE(submitted_at,created_at), updated_at=COALESCE(updated_at,created_at) WHERE purpose=''`, schema),
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.stock_entry_finished_batch_moves (
	id BIGSERIAL PRIMARY KEY,
	stock_entry_id BIGINT NOT NULL,
	stock_entry_item_id BIGINT NOT NULL,
	source_batch_id BIGINT NOT NULL,
	source_batch_code TEXT NOT NULL DEFAULT '',
	target_batch_id BIGINT NOT NULL DEFAULT 0,
	target_batch_code TEXT NOT NULL DEFAULT '',
	bom_spec_id BIGINT NOT NULL DEFAULT 0,
	bom_variant_id BIGINT NOT NULL DEFAULT 0,
	spec_g BIGINT NOT NULL DEFAULT 0,
	qty_g BIGINT NOT NULL DEFAULT 0,
	qty_units BIGINT NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS stock_entry_finished_batch_moves_entry_idx
	ON %s.stock_entry_finished_batch_moves(stock_entry_id,stock_entry_item_id,id);
`, schema, schema)); err != nil {
		return err
	}
	_, err := pool.Exec(ctx, fmt.Sprintf(`
CREATE UNIQUE INDEX IF NOT EXISTS stock_entries_idempotency_uq
	ON %s.stock_entries(idempotency_key) WHERE idempotency_key <> '';
`, schema))
	return err
}

func ensureFinishedInventoryWarehouse(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.finished_inventory (
	product_id BIGINT NOT NULL,
	bom_spec_id BIGINT NOT NULL DEFAULT 0,
	bom_variant_id BIGINT NOT NULL DEFAULT 0,
	spec_g BIGINT NOT NULL,
	warehouse TEXT NOT NULL DEFAULT 'finished_goods',
	owner_customer_id BIGINT NOT NULL DEFAULT 0,
	onhand_units BIGINT NOT NULL DEFAULT 0,
	onhand_loose_g BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %s.finished_inventory ADD COLUMN IF NOT EXISTS warehouse TEXT NOT NULL DEFAULT 'finished_goods';
ALTER TABLE %s.finished_inventory ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0;
	ALTER TABLE %s.finished_inventory ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0;
	ALTER TABLE %s.finished_inventory ADD COLUMN IF NOT EXISTS owner_customer_id BIGINT NOT NULL DEFAULT 0;
UPDATE %s.finished_inventory SET warehouse='finished_goods' WHERE COALESCE(warehouse,'')='';
`, schema, schema, schema, schema, schema, schema)); err != nil {
		return err
	}
	_, err := pool.Exec(ctx, fmt.Sprintf(`
DO $$
DECLARE
	pk_name TEXT;
BEGIN
	SELECT c.conname INTO pk_name
	FROM pg_constraint c
	WHERE c.conrelid = '%s.finished_inventory'::regclass
	  AND c.contype = 'p';
	IF pk_name IS NOT NULL THEN
		EXECUTE format('ALTER TABLE %s.finished_inventory DROP CONSTRAINT %%I', pk_name);
	END IF;
	IF NOT EXISTS (
		SELECT 1
		FROM pg_constraint c
		WHERE c.conrelid = '%s.finished_inventory'::regclass
		  AND c.contype = 'p'
	) THEN
			ALTER TABLE %s.finished_inventory ADD PRIMARY KEY(product_id, bom_spec_id, spec_g, warehouse);
	END IF;
END $$;
	CREATE INDEX IF NOT EXISTS finished_inventory_warehouse_idx
		ON %s.finished_inventory(warehouse, owner_customer_id, product_id, bom_spec_id, spec_g);
`, schema, schema, schema, schema, schema))
	return err
}

func ensureStockLedgerTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.stock_batches (
	id BIGSERIAL PRIMARY KEY,
	batch_code TEXT NOT NULL UNIQUE,
	item_type TEXT NOT NULL DEFAULT '',
	item_id BIGINT NOT NULL DEFAULT 0,
	item_name TEXT NOT NULL DEFAULT '',
	bom_spec_id BIGINT NOT NULL DEFAULT 0,
	bom_variant_id BIGINT NOT NULL DEFAULT 0,
	spec_g BIGINT NOT NULL DEFAULT 0,
	source_doc_type TEXT NOT NULL DEFAULT '',
	source_doc_id BIGINT NOT NULL DEFAULT 0,
	source_batch_id TEXT NOT NULL DEFAULT '',
	qty_g BIGINT NOT NULL DEFAULT 0,
	qty_units BIGINT NOT NULL DEFAULT 0,
	operator TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %s.stock_batches ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %s.stock_batches ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %s.stock_batches ADD COLUMN IF NOT EXISTS owner_customer_id BIGINT NOT NULL DEFAULT 0;
CREATE UNIQUE INDEX IF NOT EXISTS stock_batches_source_uq
	ON %s.stock_batches(source_doc_type, source_doc_id, item_type, item_id, bom_spec_id, spec_g)
	WHERE source_doc_type <> '';
CREATE INDEX IF NOT EXISTS stock_batches_item_idx
	ON %s.stock_batches(item_type, item_id, bom_spec_id, spec_g, created_at DESC);

CREATE TABLE IF NOT EXISTS %s.stock_ledger_entries (
	id BIGSERIAL PRIMARY KEY,
	item_type TEXT NOT NULL DEFAULT '',
	item_id BIGINT NOT NULL DEFAULT 0,
	item_name TEXT NOT NULL DEFAULT '',
	owner_customer_id BIGINT NOT NULL DEFAULT 0,
	bom_spec_id BIGINT NOT NULL DEFAULT 0,
	bom_variant_id BIGINT NOT NULL DEFAULT 0,
	spec_g BIGINT NOT NULL DEFAULT 0,
	warehouse TEXT NOT NULL DEFAULT '',
	source_doc_type TEXT NOT NULL DEFAULT '',
	source_doc_id BIGINT NOT NULL DEFAULT 0,
	source_batch_code TEXT NOT NULL DEFAULT '',
	source_batch_id TEXT NOT NULL DEFAULT '',
	qty_before_g BIGINT NOT NULL DEFAULT 0,
	qty_change_g BIGINT NOT NULL DEFAULT 0,
	qty_after_g BIGINT NOT NULL DEFAULT 0,
	qty_before_units BIGINT NOT NULL DEFAULT 0,
	qty_change_units BIGINT NOT NULL DEFAULT 0,
	qty_after_units BIGINT NOT NULL DEFAULT 0,
	operator TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %s.stock_ledger_entries ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %s.stock_ledger_entries ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %s.stock_ledger_entries ADD COLUMN IF NOT EXISTS owner_customer_id BIGINT NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS stock_ledger_source_idx
	ON %s.stock_ledger_entries(source_doc_type, source_doc_id, id);
CREATE INDEX IF NOT EXISTS stock_ledger_item_idx
	ON %s.stock_ledger_entries(item_type, item_id, bom_spec_id, spec_g, created_at DESC);
`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	for _, stmt := range []string{
		fmt.Sprintf(`ALTER TABLE %s.stock_batches ADD COLUMN IF NOT EXISTS remaining_g BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_batches ADD COLUMN IF NOT EXISTS remaining_units BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_batches ADD COLUMN IF NOT EXISTS unit_cost NUMERIC(12,4) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_batches ADD COLUMN IF NOT EXISTS quality_status TEXT NOT NULL DEFAULT 'unchecked'`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_batches ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_batches ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_ledger_entries ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_ledger_entries ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`DROP INDEX IF EXISTS %s.stock_batches_source_uq`, schema),
		fmt.Sprintf(`CREATE UNIQUE INDEX stock_batches_source_uq ON %s.stock_batches(source_doc_type,source_doc_id,item_type,item_id,bom_spec_id,spec_g) WHERE source_doc_type<>''`, schema),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS stock_batches_quality_idx ON %s.stock_batches(item_type, quality_status, batch_code)`, schema),
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func ensureMaterialBatchTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.material_receipts (
	id BIGSERIAL PRIMARY KEY,
	material_id BIGINT NOT NULL,
	supplier TEXT NOT NULL DEFAULT '',
	qty_g BIGINT NOT NULL DEFAULT 0,
	qty_units BIGINT NOT NULL DEFAULT 0,
	unit_cost NUMERIC(12,4) NOT NULL DEFAULT 0,
	crop_season TEXT NOT NULL DEFAULT '',
	origin TEXT NOT NULL DEFAULT '',
	producer_flavor_description TEXT NOT NULL DEFAULT '',
	note TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'submitted',
	operator TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS material_receipts_material_idx
	ON %s.material_receipts(material_id, created_at DESC);

CREATE TABLE IF NOT EXISTS %s.material_batches (
	id BIGSERIAL PRIMARY KEY,
	batch_code TEXT NOT NULL UNIQUE,
	material_id BIGINT NOT NULL,
	supplier TEXT NOT NULL DEFAULT '',
	receipt_id BIGINT NOT NULL DEFAULT 0,
	qty_g BIGINT NOT NULL DEFAULT 0,
	qty_units BIGINT NOT NULL DEFAULT 0,
	remaining_g BIGINT NOT NULL DEFAULT 0,
	remaining_units BIGINT NOT NULL DEFAULT 0,
	unit_cost NUMERIC(12,4) NOT NULL DEFAULT 0,
	crop_season TEXT NOT NULL DEFAULT '',
	origin TEXT NOT NULL DEFAULT '',
	producer_flavor_description TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'active',
	note TEXT NOT NULL DEFAULT '',
	received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS material_batches_material_fifo_idx
	ON %s.material_batches(material_id, status, received_at, id);
`, schema, schema, schema, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.material_batches ADD COLUMN IF NOT EXISTS quality_status TEXT NOT NULL DEFAULT 'unchecked'`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.material_batches ADD COLUMN IF NOT EXISTS owner_customer_id BIGINT NOT NULL DEFAULT 0`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.material_batches ADD COLUMN IF NOT EXISTS material_name TEXT NOT NULL DEFAULT ''`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.material_batches ADD COLUMN IF NOT EXISTS received_g BIGINT NOT NULL DEFAULT 0`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.material_receipts ADD COLUMN IF NOT EXISTS qty_units BIGINT NOT NULL DEFAULT 0`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.material_batches ADD COLUMN IF NOT EXISTS qty_units BIGINT NOT NULL DEFAULT 0`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.material_batches ADD COLUMN IF NOT EXISTS remaining_units BIGINT NOT NULL DEFAULT 0`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.material_receipts ADD COLUMN IF NOT EXISTS crop_season TEXT NOT NULL DEFAULT ''`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.material_receipts ADD COLUMN IF NOT EXISTS origin TEXT NOT NULL DEFAULT ''`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.material_receipts ADD COLUMN IF NOT EXISTS producer_flavor_description TEXT NOT NULL DEFAULT ''`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.material_batches ADD COLUMN IF NOT EXISTS crop_season TEXT NOT NULL DEFAULT ''`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.material_batches ADD COLUMN IF NOT EXISTS origin TEXT NOT NULL DEFAULT ''`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.material_batches ADD COLUMN IF NOT EXISTS producer_flavor_description TEXT NOT NULL DEFAULT ''`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.material_batches SET received_g=qty_g WHERE received_g=0 AND qty_g > 0`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.material_batches b SET material_name=m.name FROM %[1]s.materials m WHERE b.material_id=m.id AND COALESCE(b.material_name,'')=''`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`CREATE INDEX IF NOT EXISTS material_batches_quality_idx ON %s.material_batches(material_id, quality_status, status, received_at, id)`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`CREATE INDEX IF NOT EXISTS material_batches_owner_idx ON %s.material_batches(owner_customer_id, material_id, status, received_at, id)`, schema))
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
INSERT INTO %s.material_batches(batch_code,material_id,supplier,receipt_id,qty_g,remaining_g,unit_cost,note,received_at,created_at)
SELECT 'LEGACY-MAT-' || lpad(m.id::text, 10, '0'),
       m.id,
       'legacy_onhand',
       0,
       m.onhand_g,
       m.onhand_g,
       COALESCE(m.purchase_price, 0),
       '系统升级按物料现有库存生成的期初批次',
       now(),
       now()
FROM %s.materials m
WHERE COALESCE(m.onhand_g, 0) > 0
  AND NOT EXISTS (
      SELECT 1
      FROM %s.material_batches b
      WHERE b.material_id = m.id
  )
ON CONFLICT (batch_code) DO NOTHING
`, schema, schema, schema)); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.material_batch_locations (
	material_batch_id BIGINT NOT NULL,
	batch_code TEXT NOT NULL DEFAULT '',
	material_id BIGINT NOT NULL DEFAULT 0,
	warehouse TEXT NOT NULL DEFAULT '',
	qty_g BIGINT NOT NULL DEFAULT 0,
	qty_units BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY(material_batch_id, warehouse)
);
ALTER TABLE %s.material_batch_locations ADD COLUMN IF NOT EXISTS qty_units BIGINT NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS material_batch_locations_lookup_idx
	ON %s.material_batch_locations(material_id, warehouse, qty_g, qty_units);
INSERT INTO %s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g,qty_units,updated_at)
SELECT id,batch_code,material_id,'raw_materials',remaining_g,remaining_units,now()
FROM %s.material_batches
WHERE remaining_g > 0 OR remaining_units > 0
ON CONFLICT (material_batch_id, warehouse) DO NOTHING
`, schema, schema, schema, schema, schema)); err != nil {
		return err
	}
	return nil
}

func ensureWarehouseTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.warehouses (
	code TEXT PRIMARY KEY,
	name TEXT NOT NULL DEFAULT '',
	kind TEXT NOT NULL DEFAULT '',
	parent_code TEXT NOT NULL DEFAULT '',
	sort_order INTEGER NOT NULL DEFAULT 0,
	is_default BOOLEAN NOT NULL DEFAULT false,
	active BOOLEAN NOT NULL DEFAULT true,
	description TEXT NOT NULL DEFAULT '',
	customer_id BIGINT NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
`, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	for _, stmt := range []string{
		`ALTER TABLE %[1]s.warehouses ADD COLUMN IF NOT EXISTS customer_id BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE %[1]s.warehouses ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`,
	} {
		if _, err := pool.Exec(ctx, fmt.Sprintf(stmt, schema)); err != nil {
			return err
		}
	}
	_, err := pool.Exec(ctx, fmt.Sprintf(`
INSERT INTO %s.warehouses(code,name,kind,parent_code,sort_order,is_default,active,description)
VALUES
	('raw_materials','原料仓','raw','',10,true,true,'未领用的生豆、辅料，生产领料前保存在这里'),
	('packaging','包材仓','packaging','',20,true,true,'袋子、盒子、标签等包材库存'),
	('wip','WIP在制仓','wip','',30,true,true,'已领到生产现场但尚未消耗的共享在制库存'),
	('finished_goods','成品仓','finished','',40,true,true,'生产完成并可销售/发货的成品库存'),
	('finished_shop','门店成品仓','finished','finished_goods',45,false,true,'门店、展会或临时销售点的成品库存'),
	('loss','损耗/报废仓','loss','',50,false,true,'盘点损耗、报废或异常消耗的记录位置')
ON CONFLICT (code) DO UPDATE SET
	name=excluded.name,
	kind=excluded.kind,
	parent_code=excluded.parent_code,
	sort_order=excluded.sort_order,
	is_default=excluded.is_default,
	active=excluded.active,
	description=excluded.description
`, schema))
	return err
}

func migrateWarehousesToBusinessGroups(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	var hasWarehouses, hasGroups, hasItems, hasUsages, hasAssignments bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL, to_regclass($2) IS NOT NULL, to_regclass($3) IS NOT NULL, to_regclass($4) IS NOT NULL, to_regclass($5) IS NOT NULL`,
		schema+".warehouses",
		schema+".business_groups",
		schema+".business_group_items",
		schema+".business_group_usages",
		schema+".business_group_assignments",
	).Scan(&hasWarehouses, &hasGroups, &hasItems, &hasUsages, &hasAssignments); err != nil {
		return err
	}
	if !hasWarehouses || !hasGroups || !hasItems || !hasUsages || !hasAssignments {
		return nil
	}
	q := fmt.Sprintf(`
WITH group_row AS (
	INSERT INTO %[1]s.business_groups(name, code, remark, active, sort_order, created_by, updated_by)
	VALUES('仓库库存默认分组','default_warehouse_inventory','PR-442: warehouses migrated to generic business group assignments by warehouse code',true,30,'system-pr442-migration','system-pr442-migration')
	ON CONFLICT DO NOTHING
	RETURNING id
),
target_group AS (
	SELECT id FROM group_row
	UNION ALL
	SELECT id FROM %[1]s.business_groups WHERE code='default_warehouse_inventory' OR name='仓库库存默认分组'
	ORDER BY id
	LIMIT 1
),
usage_upsert AS (
	INSERT INTO %[1]s.business_group_usages(group_id, usage_key, usage_label, active, created_by, updated_by)
	SELECT tg.id, 'warehouse_inventory', '仓库库存视图仓库归组', true, 'system-pr442-migration', 'system-pr442-migration'
	FROM target_group tg
	WHERE NOT EXISTS (
		SELECT 1 FROM %[1]s.business_group_usages existing
		WHERE existing.group_id=tg.id AND lower(existing.usage_key)='warehouse_inventory'
	)
	ON CONFLICT DO NOTHING
),
item_rows AS (
	SELECT '普通仓库' AS name, 'normal_warehouses' AS code, 10 AS sort_order
	UNION ALL SELECT '客户仓库', 'customer_warehouses', 20
	UNION ALL SELECT '损耗/报废', 'loss_scrap_warehouses', 30
),
item_upsert AS (
	INSERT INTO %[1]s.business_group_items(group_id, parent_id, name, code, remark, active, sort_order)
	SELECT tg.id, 0, ir.name, ir.code, 'PR-442 默认库存仓库分组', true, ir.sort_order
	FROM target_group tg
	CROSS JOIN item_rows ir
	ON CONFLICT DO NOTHING
)
INSERT INTO %[1]s.business_group_assignments(group_id, group_item_id, usage_key, object_key, object_id, object_ref, sort_order, created_by, updated_by)
SELECT tg.id,
       item.id,
       'warehouse_inventory',
       'warehouse',
       0,
       w.code,
       COALESCE(w.sort_order,100),
       'system-pr442-migration',
       'system-pr442-migration'
FROM %[1]s.warehouses w
CROSS JOIN target_group tg
JOIN %[1]s.business_group_items item ON item.group_id=tg.id AND item.code = CASE
	WHEN COALESCE(w.kind,'')='customer' THEN 'customer_warehouses'
	WHEN COALESCE(w.kind,'') IN ('scrap','loss','waste') THEN 'loss_scrap_warehouses'
	ELSE 'normal_warehouses'
END
ON CONFLICT DO NOTHING;
`, schema)
	_, err := pool.Exec(ctx, q)
	return err
}

func ensureMaterialTransferTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.material_transfers (
	id BIGSERIAL PRIMARY KEY,
	transfer_no TEXT NOT NULL UNIQUE,
	material_id BIGINT NOT NULL DEFAULT 0,
	material_name TEXT NOT NULL DEFAULT '',
	from_warehouse TEXT NOT NULL DEFAULT '',
	to_warehouse TEXT NOT NULL DEFAULT '',
	qty_g BIGINT NOT NULL DEFAULT 0,
	note TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'submitted',
	operator TEXT NOT NULL DEFAULT '',
	idempotency_key TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS material_transfers_idempotency_uq
	ON %s.material_transfers(idempotency_key)
	WHERE idempotency_key <> '';
CREATE INDEX IF NOT EXISTS material_transfers_material_idx
	ON %s.material_transfers(material_id, created_at DESC);

CREATE TABLE IF NOT EXISTS %s.material_transfer_items (
	id BIGSERIAL PRIMARY KEY,
	transfer_id BIGINT NOT NULL,
	material_batch_id BIGINT NOT NULL DEFAULT 0,
	material_batch_code TEXT NOT NULL DEFAULT '',
	qty_g BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS material_transfer_items_transfer_idx
	ON %s.material_transfer_items(transfer_id, id);
`, schema, schema, schema, schema, schema)
	_, err := pool.Exec(ctx, q)
	return err
}

func ensureFinishedProductTransferTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.finished_product_transfers (
	id BIGSERIAL PRIMARY KEY,
	transfer_no TEXT NOT NULL UNIQUE,
	product_id BIGINT NOT NULL DEFAULT 0,
	product_name TEXT NOT NULL DEFAULT '',
	spec_g BIGINT NOT NULL DEFAULT 0,
	bom_spec_id BIGINT NOT NULL DEFAULT 0,
	bom_variant_id BIGINT NOT NULL DEFAULT 0,
	from_warehouse TEXT NOT NULL DEFAULT '',
	to_warehouse TEXT NOT NULL DEFAULT '',
	qty_g BIGINT NOT NULL DEFAULT 0,
	qty_units BIGINT NOT NULL DEFAULT 0,
	qty_loose_g BIGINT NOT NULL DEFAULT 0,
	note TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'submitted',
	operator TEXT NOT NULL DEFAULT '',
	idempotency_key TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %s.finished_product_transfers ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %s.finished_product_transfers ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0;
CREATE UNIQUE INDEX IF NOT EXISTS finished_product_transfers_idempotency_uq
	ON %s.finished_product_transfers(idempotency_key)
	WHERE idempotency_key <> '';
DROP INDEX IF EXISTS %s.finished_product_transfers_product_idx;
CREATE INDEX finished_product_transfers_product_idx
	ON %s.finished_product_transfers(product_id, bom_spec_id, spec_g, created_at DESC);
`, schema, schema, schema, schema, schema, schema)
	_, err := pool.Exec(ctx, q)
	return err
}

func ensureStockAdjustmentTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.stock_adjustments (
	id BIGSERIAL PRIMARY KEY,
	adjustment_type TEXT NOT NULL DEFAULT 'quantity',
	item_type TEXT NOT NULL DEFAULT '',
	item_id BIGINT NOT NULL DEFAULT 0,
	item_name TEXT NOT NULL DEFAULT '',
	spec_g BIGINT NOT NULL DEFAULT 0,
	bom_spec_id BIGINT NOT NULL DEFAULT 0,
	bom_variant_id BIGINT NOT NULL DEFAULT 0,
	warehouse TEXT NOT NULL DEFAULT '',
	reason TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'submitted',
	operator TEXT NOT NULL DEFAULT '',
	material_batch_id BIGINT NOT NULL DEFAULT 0,
	unit_cost_before NUMERIC(12,4) NOT NULL DEFAULT 0,
	unit_cost_after NUMERIC(12,4) NOT NULL DEFAULT 0,
	value_change NUMERIC(14,4) NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS %s.stock_adjustment_items (
	id BIGSERIAL PRIMARY KEY,
	adjustment_id BIGINT NOT NULL,
	item_type TEXT NOT NULL DEFAULT '',
	item_id BIGINT NOT NULL DEFAULT 0,
	spec_g BIGINT NOT NULL DEFAULT 0,
	bom_spec_id BIGINT NOT NULL DEFAULT 0,
	bom_variant_id BIGINT NOT NULL DEFAULT 0,
	qty_before_g BIGINT NOT NULL DEFAULT 0,
	qty_change_g BIGINT NOT NULL DEFAULT 0,
	qty_after_g BIGINT NOT NULL DEFAULT 0,
	qty_before_units BIGINT NOT NULL DEFAULT 0,
	qty_change_units BIGINT NOT NULL DEFAULT 0,
	qty_after_units BIGINT NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS %s.stock_adjustment_batch_allocations (
	id BIGSERIAL PRIMARY KEY,
	adjustment_id BIGINT NOT NULL,
	material_batch_id BIGINT NOT NULL DEFAULT 0,
	batch_code TEXT NOT NULL DEFAULT '',
	warehouse TEXT NOT NULL DEFAULT '',
	qty_change_g BIGINT NOT NULL DEFAULT 0,
	qty_change_units BIGINT NOT NULL DEFAULT 0,
	unit_cost NUMERIC(12,4) NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS stock_adjustments_item_idx
	ON %s.stock_adjustments(item_type, item_id, created_at DESC);
CREATE INDEX IF NOT EXISTS stock_adjustment_batch_allocations_adjustment_idx
	ON %s.stock_adjustment_batch_allocations(adjustment_id, id);
`, schema, schema, schema, schema, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	for _, stmt := range []string{
		fmt.Sprintf(`ALTER TABLE %s.stock_adjustments ADD COLUMN IF NOT EXISTS adjustment_type TEXT NOT NULL DEFAULT 'quantity'`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_adjustments ADD COLUMN IF NOT EXISTS material_batch_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_adjustments ADD COLUMN IF NOT EXISTS unit_cost_before NUMERIC(12,4) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_adjustments ADD COLUMN IF NOT EXISTS unit_cost_after NUMERIC(12,4) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_adjustments ADD COLUMN IF NOT EXISTS value_change NUMERIC(14,4) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_adjustments ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_adjustments ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_adjustment_items ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_adjustment_items ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0`, schema),
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}
