package production

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ensureMultilevelProductionTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
ALTER TABLE %[1]s.production_plan_items ADD COLUMN IF NOT EXISTS output_type TEXT NOT NULL DEFAULT 'product';
ALTER TABLE %[1]s.production_plan_items ADD COLUMN IF NOT EXISTS output_product_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.production_plan_items ADD COLUMN IF NOT EXISTS output_material_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.production_plan_items ADD COLUMN IF NOT EXISTS output_name TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.production_plan_items ADD COLUMN IF NOT EXISTS output_qty NUMERIC(18,9) NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.production_plan_items ADD COLUMN IF NOT EXISTS output_unit TEXT NOT NULL DEFAULT '';
UPDATE %[1]s.production_plan_items
SET output_type='product',
    output_product_id=CASE WHEN output_product_id>0 THEN output_product_id ELSE product_id END,
    output_name=CASE WHEN output_name<>'' THEN output_name ELSE product_name END,
    output_qty=CASE WHEN output_qty>0 THEN output_qty ELSE planned_inventory_qty END,
    output_unit=CASE WHEN output_unit<>'' THEN output_unit ELSE inventory_unit END
WHERE output_type='product';

ALTER TABLE %[1]s.work_orders ADD COLUMN IF NOT EXISTS output_type TEXT NOT NULL DEFAULT 'product';
ALTER TABLE %[1]s.work_orders ADD COLUMN IF NOT EXISTS output_product_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.work_orders ADD COLUMN IF NOT EXISTS output_material_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.work_orders ADD COLUMN IF NOT EXISTS output_name TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.work_orders ADD COLUMN IF NOT EXISTS output_qty NUMERIC(18,9) NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.work_orders ADD COLUMN IF NOT EXISTS output_unit TEXT NOT NULL DEFAULT '';
UPDATE %[1]s.work_orders
SET output_type='product',
    output_product_id=CASE WHEN output_product_id>0 THEN output_product_id ELSE product_id END,
    output_name=CASE WHEN output_name<>'' THEN output_name ELSE product_name END,
    output_qty=CASE WHEN output_qty>0 THEN output_qty ELSE planned_inventory_qty END,
    output_unit=CASE WHEN output_unit<>'' THEN output_unit ELSE inventory_unit END
WHERE output_type='product';

ALTER TABLE %[1]s.produce_running_items ADD COLUMN IF NOT EXISTS output_type TEXT NOT NULL DEFAULT 'product';
ALTER TABLE %[1]s.produce_running_items ADD COLUMN IF NOT EXISTS output_product_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.produce_running_items ADD COLUMN IF NOT EXISTS output_material_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.produce_running_items ADD COLUMN IF NOT EXISTS output_name TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.produce_running_items ADD COLUMN IF NOT EXISTS output_qty NUMERIC(18,9) NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.produce_running_items ADD COLUMN IF NOT EXISTS output_unit TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.produce_running_items ADD COLUMN IF NOT EXISTS target_warehouse TEXT NOT NULL DEFAULT '';
UPDATE %[1]s.produce_running_items
SET output_type='product',
    output_product_id=CASE WHEN output_product_id>0 THEN output_product_id ELSE product_id END,
    output_name=CASE WHEN output_name<>'' THEN output_name ELSE product_name END
WHERE output_type='product';

CREATE TABLE IF NOT EXISTS %[1]s.production_plan_item_dependencies (
	id BIGSERIAL PRIMARY KEY,
	production_plan_id BIGINT NOT NULL,
	production_plan_item_id BIGINT NOT NULL,
	depends_on_plan_item_id BIGINT NOT NULL,
	material_id BIGINT NOT NULL DEFAULT 0,
	component_type TEXT NOT NULL DEFAULT 'material',
	component_id BIGINT NOT NULL DEFAULT 0,
	component_spec_g BIGINT NOT NULL DEFAULT 0,
	required_g BIGINT NOT NULL DEFAULT 0,
	required_units BIGINT NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %[1]s.production_plan_item_dependencies ADD COLUMN IF NOT EXISTS component_type TEXT NOT NULL DEFAULT 'material';
ALTER TABLE %[1]s.production_plan_item_dependencies ADD COLUMN IF NOT EXISTS component_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.production_plan_item_dependencies ADD COLUMN IF NOT EXISTS component_spec_g BIGINT NOT NULL DEFAULT 0;
UPDATE %[1]s.production_plan_item_dependencies
SET component_type='material',component_id=material_id,component_spec_g=0
WHERE component_id=0 AND material_id>0;
DO $$
DECLARE old_uq RECORD;
BEGIN
	FOR old_uq IN
		SELECT conname FROM pg_constraint
		WHERE conrelid='%[1]s.production_plan_item_dependencies'::regclass AND contype='u'
		  AND pg_get_constraintdef(oid) LIKE '%%material_id%%'
	LOOP
		EXECUTE format('ALTER TABLE %[1]s.production_plan_item_dependencies DROP CONSTRAINT %%I',old_uq.conname);
	END LOOP;
END $$;
CREATE UNIQUE INDEX IF NOT EXISTS production_plan_item_dependencies_typed_uq
	ON %[1]s.production_plan_item_dependencies(production_plan_item_id,depends_on_plan_item_id,component_type,component_id,component_spec_g);
CREATE INDEX IF NOT EXISTS production_plan_item_dependencies_plan_idx
	ON %[1]s.production_plan_item_dependencies(production_plan_id, production_plan_item_id);

CREATE TABLE IF NOT EXISTS %[1]s.production_plan_supply_gaps (
	id BIGSERIAL PRIMARY KEY,
	production_plan_id BIGINT NOT NULL,
	production_plan_item_id BIGINT NOT NULL DEFAULT 0,
	item_type TEXT NOT NULL DEFAULT 'material',
	item_id BIGINT NOT NULL DEFAULT 0,
	item_name TEXT NOT NULL DEFAULT '',
	required_g BIGINT NOT NULL DEFAULT 0,
	required_units BIGINT NOT NULL DEFAULT 0,
	reason TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'unresolved',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS production_plan_supply_gaps_plan_idx
	ON %[1]s.production_plan_supply_gaps(production_plan_id, status, id);

CREATE TABLE IF NOT EXISTS %[1]s.work_order_dependencies (
	id BIGSERIAL PRIMARY KEY,
	work_order_id BIGINT NOT NULL,
	depends_on_work_order_id BIGINT NOT NULL,
	material_id BIGINT NOT NULL DEFAULT 0,
	component_type TEXT NOT NULL DEFAULT 'material',
	component_id BIGINT NOT NULL DEFAULT 0,
	component_spec_g BIGINT NOT NULL DEFAULT 0,
	required_g BIGINT NOT NULL DEFAULT 0,
	required_units BIGINT NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %[1]s.work_order_dependencies ADD COLUMN IF NOT EXISTS component_type TEXT NOT NULL DEFAULT 'material';
ALTER TABLE %[1]s.work_order_dependencies ADD COLUMN IF NOT EXISTS component_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.work_order_dependencies ADD COLUMN IF NOT EXISTS component_spec_g BIGINT NOT NULL DEFAULT 0;
UPDATE %[1]s.work_order_dependencies
SET component_type='material',component_id=material_id,component_spec_g=0
WHERE component_id=0 AND material_id>0;
DO $$
DECLARE old_uq RECORD;
BEGIN
	FOR old_uq IN
		SELECT conname FROM pg_constraint
		WHERE conrelid='%[1]s.work_order_dependencies'::regclass AND contype='u'
		  AND pg_get_constraintdef(oid) LIKE '%%material_id%%'
	LOOP
		EXECUTE format('ALTER TABLE %[1]s.work_order_dependencies DROP CONSTRAINT %%I',old_uq.conname);
	END LOOP;
END $$;
CREATE UNIQUE INDEX IF NOT EXISTS work_order_dependencies_typed_uq
	ON %[1]s.work_order_dependencies(work_order_id,depends_on_work_order_id,component_type,component_id,component_spec_g);
CREATE INDEX IF NOT EXISTS work_order_dependencies_work_order_idx
	ON %[1]s.work_order_dependencies(work_order_id, depends_on_work_order_id);
CREATE INDEX IF NOT EXISTS work_order_dependencies_upstream_idx
	ON %[1]s.work_order_dependencies(depends_on_work_order_id, work_order_id);

CREATE TABLE IF NOT EXISTS %[1]s.work_order_material_reservation_batches (
	id BIGSERIAL PRIMARY KEY,
	reservation_id BIGINT NOT NULL,
	work_order_id BIGINT NOT NULL,
	material_id BIGINT NOT NULL,
	component_type TEXT NOT NULL DEFAULT 'material',
	component_id BIGINT NOT NULL DEFAULT 0,
	component_spec_g BIGINT NOT NULL DEFAULT 0,
	material_batch_id BIGINT NOT NULL DEFAULT 0,
	stock_batch_id BIGINT NOT NULL DEFAULT 0,
	batch_code TEXT NOT NULL DEFAULT '',
	warehouse TEXT NOT NULL DEFAULT '',
	reserved_g BIGINT NOT NULL DEFAULT 0,
	reserved_units BIGINT NOT NULL DEFAULT 0,
	consumed_g BIGINT NOT NULL DEFAULT 0,
	consumed_units BIGINT NOT NULL DEFAULT 0,
	returned_g BIGINT NOT NULL DEFAULT 0,
	returned_units BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'reserved',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %[1]s.work_order_material_reservations ADD COLUMN IF NOT EXISTS component_type TEXT NOT NULL DEFAULT 'material';
ALTER TABLE %[1]s.work_order_material_reservations ADD COLUMN IF NOT EXISTS component_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.work_order_material_reservations ADD COLUMN IF NOT EXISTS component_spec_g BIGINT NOT NULL DEFAULT 0;
UPDATE %[1]s.work_order_material_reservations
SET component_type='material',component_id=material_id,component_spec_g=0
WHERE component_id=0 AND material_id>0;
CREATE INDEX IF NOT EXISTS work_order_material_reservations_component_idx
	ON %[1]s.work_order_material_reservations(component_type,component_id,component_spec_g,status);
ALTER TABLE %[1]s.work_order_material_reservation_batches ADD COLUMN IF NOT EXISTS component_type TEXT NOT NULL DEFAULT 'material';
ALTER TABLE %[1]s.work_order_material_reservation_batches ADD COLUMN IF NOT EXISTS component_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.work_order_material_reservation_batches ADD COLUMN IF NOT EXISTS component_spec_g BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.work_order_material_reservation_batches ADD COLUMN IF NOT EXISTS stock_batch_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.work_order_material_reservation_batches ADD COLUMN IF NOT EXISTS warehouse TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.work_order_material_reservation_batches ALTER COLUMN material_batch_id SET DEFAULT 0;
UPDATE %[1]s.work_order_material_reservation_batches
SET component_type='material',component_id=material_id,component_spec_g=0
WHERE component_id=0 AND material_id>0;
UPDATE %[1]s.work_order_material_reservation_batches binding
SET warehouse=COALESCE(NULLIF((
	SELECT ledger.warehouse
	FROM %[1]s.stock_batches batch
	JOIN %[1]s.stock_ledger_entries ledger
	  ON ledger.item_type='finished_product'
	 AND ledger.item_id=batch.item_id
	 AND ledger.spec_g=batch.spec_g
	 AND (ledger.source_batch_code=batch.batch_code OR ledger.source_batch_id=batch.batch_code)
	WHERE batch.id=binding.stock_batch_id
	ORDER BY ledger.id DESC
	LIMIT 1
),''),'finished_goods')
WHERE binding.component_type='product' AND binding.stock_batch_id>0 AND binding.warehouse='';
DO $$
DECLARE old_uq RECORD;
BEGIN
	FOR old_uq IN
		SELECT conname FROM pg_constraint
		WHERE conrelid='%[1]s.work_order_material_reservation_batches'::regclass AND contype='u'
		  AND pg_get_constraintdef(oid) LIKE '%%reservation_id%%material_batch_id%%'
	LOOP
		EXECUTE format('ALTER TABLE %[1]s.work_order_material_reservation_batches DROP CONSTRAINT %%I',old_uq.conname);
	END LOOP;
END $$;
CREATE UNIQUE INDEX IF NOT EXISTS work_order_material_reservation_batches_typed_uq
	ON %[1]s.work_order_material_reservation_batches(
		reservation_id,component_type,component_id,component_spec_g,material_batch_id,stock_batch_id
	);
CREATE INDEX IF NOT EXISTS work_order_material_reservation_batches_work_order_idx
	ON %[1]s.work_order_material_reservation_batches(work_order_id, material_id, status);
CREATE INDEX IF NOT EXISTS work_order_material_reservation_batches_batch_idx
	ON %[1]s.work_order_material_reservation_batches(material_batch_id, status);
`, schema))
	return err
}
