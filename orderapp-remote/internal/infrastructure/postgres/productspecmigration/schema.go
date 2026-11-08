package productspecmigration

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// EnsureSchema deliberately runs after all business modules. The compatibility
// columns are additive and keep legacy rows valid with zero identifiers while
// allowing cutover products to persist parent product + BOM spec identity.
func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %[1]s.product_bom_spec_migrations (
	product_id BIGINT PRIMARY KEY,
	state TEXT NOT NULL DEFAULT 'legacy',
	legacy_catalog_product BOOLEAN NOT NULL DEFAULT true,
	spec_identity_mode TEXT NOT NULL DEFAULT '',
	readiness_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	prepared_at TIMESTAMPTZ,
	prepared_by TEXT NOT NULL DEFAULT '',
	ready_at TIMESTAMPTZ,
	ready_by TEXT NOT NULL DEFAULT '',
	cutover_at TIMESTAMPTZ,
	cutover_by TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	CHECK(state IN ('legacy','preparing','ready','cutover'))
);
CREATE INDEX IF NOT EXISTS product_bom_spec_migrations_state_idx
	ON %[1]s.product_bom_spec_migrations(state, updated_at DESC);
ALTER TABLE %[1]s.product_bom_spec_migrations
	ADD COLUMN IF NOT EXISTS legacy_catalog_product BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE %[1]s.product_bom_spec_migrations
	ADD COLUMN IF NOT EXISTS spec_identity_mode TEXT NOT NULL DEFAULT '';
UPDATE %[1]s.product_bom_spec_migrations
SET spec_identity_mode=CASE
	WHEN state='cutover' OR legacy_catalog_product=false THEN 'bom_spec'
	ELSE 'legacy_sku'
END
WHERE spec_identity_mode NOT IN ('legacy_sku','product','bom_spec');

CREATE TABLE IF NOT EXISTS %[1]s.legacy_child_sku_bom_spec_mappings (
	id BIGSERIAL PRIMARY KEY,
	parent_product_id BIGINT NOT NULL,
	legacy_child_product_id BIGINT NOT NULL,
	bom_id BIGINT NOT NULL DEFAULT 0,
	bom_spec_id BIGINT NOT NULL DEFAULT 0,
	bom_variant_id BIGINT NOT NULL DEFAULT 0,
	legacy_spec_key TEXT NOT NULL DEFAULT '',
	legacy_spec_name TEXT NOT NULL DEFAULT '',
	legacy_sales_unit TEXT NOT NULL DEFAULT '',
	legacy_spec_g BIGINT NOT NULL DEFAULT 0,
	metadata_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_by TEXT NOT NULL DEFAULT '',
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_by TEXT NOT NULL DEFAULT '',
	tombstoned_at TIMESTAMPTZ,
	UNIQUE(legacy_child_product_id)
);
CREATE INDEX IF NOT EXISTS legacy_child_sku_bom_spec_mappings_parent_idx
	ON %[1]s.legacy_child_sku_bom_spec_mappings(parent_product_id, legacy_spec_key, legacy_child_product_id);
CREATE INDEX IF NOT EXISTS legacy_child_sku_bom_spec_mappings_spec_idx
	ON %[1]s.legacy_child_sku_bom_spec_mappings(bom_spec_id, bom_variant_id)
	WHERE bom_spec_id > 0;

-- PR-608 stores the deterministic catalog manifest and the original child
-- rows outside the product catalog.  The compatibility tables above remain
-- available for historical reads and a guarded database-backup rollback.
CREATE TABLE IF NOT EXISTS %[1]s.product_bom_spec_authority_upgrades (
	manifest_id TEXT PRIMARY KEY,
	state TEXT NOT NULL CHECK (state IN ('prepared','applied','rolled_back')),
	manifest_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	snapshot_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_by TEXT NOT NULL DEFAULT '',
	prepared_at TIMESTAMPTZ,
	applied_at TIMESTAMPTZ,
	rolled_back_at TIMESTAMPTZ,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS product_bom_spec_authority_upgrades_state_idx
	ON %[1]s.product_bom_spec_authority_upgrades(state, updated_at DESC);
`, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	if err := backfillDefaultBOMProductIdentityModes(ctx, pool, schema); err != nil {
		return err
	}

	// These are current mutable business records only. Existing snapshots and
	// published JSON remain untouched and keep their historical child SKU/spec_g.
	for _, target := range businessIdentityTables {
		stmt := fmt.Sprintf(`
ALTER TABLE IF EXISTS %s.%s ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE IF EXISTS %s.%s ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0;
`, schema, target.Table, schema, target.Table)
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("extend %s business identity: %w", target.Table, err)
		}
	}
	if err := ensureBusinessIdentityWriteGuards(ctx, pool, schema); err != nil {
		return err
	}
	return ensureLegacyChildCatalogWriteGuard(ctx, pool, schema)
}

func backfillDefaultBOMProductIdentityModes(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	for _, candidate := range []struct {
		table   string
		columns []string
	}{
		{table: "production_bom_output_bindings", columns: []string{"output_type", "output_id", "bom_id", "is_default"}},
		{table: "production_boms", columns: []string{"id", "output_type", "output_product_id", "specification_mode"}},
	} {
		ok, err := tableHasColumns(ctx, pool, schema, candidate.table, candidate.columns...)
		if err != nil || !ok {
			return err
		}
	}
	_, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.product_bom_spec_migrations(product_id,state,legacy_catalog_product,spec_identity_mode,updated_at)
		SELECT binding.output_id,
		       'legacy',
		       true,
		       CASE WHEN COALESCE(NULLIF(bom.specification_mode,''),'single')='spec_group' THEN 'bom_spec' ELSE 'product' END,
		       now()
		FROM %[1]s.production_bom_output_bindings binding
		JOIN %[1]s.production_boms bom
		  ON bom.id=binding.bom_id
		 AND bom.output_type='product'
		 AND bom.output_product_id=binding.output_id
		WHERE binding.output_type='product' AND binding.is_default=true
		ON CONFLICT(product_id) DO UPDATE SET
			spec_identity_mode=excluded.spec_identity_mode,
			updated_at=now()
	`, schema))
	return err
}

type businessIdentityTarget struct {
	Table      string
	ProductCol string
	TypeCol    string
}

var businessIdentityTables = []businessIdentityTarget{
	// Catalog/options and current pricing masters.
	{Table: "product_price_tiers", ProductCol: "product_id"},
	{Table: "product_price_records", ProductCol: "product_id"},
	{Table: "product_tier_price_scheme_tiers", ProductCol: "product_id"},
	// customer_product_aliases is a parent-product catalog alias, not a
	// sellable/inventory identity.  It deliberately stays family-level after
	// cutover; price/order rows carry bom_spec_id instead.
	// Orders and active finished-product allocations.
	{Table: "order_items", ProductCol: "product_id"},
	{Table: "order_stock_batch_allocations", ProductCol: "product_id"},
	{Table: "order_stock_deductions", ProductCol: "product_id"},
	// Finished stock, batch/ledger and stock documents.
	{Table: "finished_inventory", ProductCol: "product_id"},
	{Table: "finished_allocation_logs", ProductCol: "product_id"},
	{Table: "stock_batches", ProductCol: "item_id", TypeCol: "item_type"},
	{Table: "stock_ledger_entries", ProductCol: "item_id", TypeCol: "item_type"},
	{Table: "stock_entry_items", ProductCol: "product_id", TypeCol: "item_type"},
	{Table: "stock_adjustment_items", ProductCol: "item_id", TypeCol: "item_type"},
	{Table: "finished_product_transfer_items", ProductCol: "product_id"},
	// Production planning, execution, reservations and traceability.
	{Table: "production_plan_items", ProductCol: "product_id"},
	{Table: "work_orders", ProductCol: "product_id"},
	{Table: "produce_running_items", ProductCol: "product_id"},
	{Table: "produce_running_outputs", ProductCol: "product_id"},
	{Table: "production_logs", ProductCol: "product_id"},
	{Table: "work_order_dependencies", ProductCol: "component_id", TypeCol: "component_type"},
	{Table: "work_order_material_reservations", ProductCol: "component_id", TypeCol: "component_type"},
	{Table: "work_order_material_reservation_batches", ProductCol: "component_id", TypeCol: "component_type"},
	// Mini-program/customer fulfillment mutable records.
	{Table: "customer_inventory_items", ProductCol: "item_id", TypeCol: "item_type"},
	{Table: "customer_custody_items", ProductCol: "item_id", TypeCol: "item_type"},
	{Table: "processing_job_request_items", ProductCol: "product_id"},
	{Table: "customer_processing_production_demands", ProductCol: "product_id"},
	{Table: "customer_processing_material_reservations", ProductCol: "component_product_id", TypeCol: "component_type"},
	{Table: "customer_processing_work_orders", ProductCol: "product_id"},
	{Table: "customer_processing_packaging_jobs", ProductCol: "product_id"},
	{Table: "customer_direct_ship_import_order_items", ProductCol: "product_id"},
	{Table: "customer_direct_ship_request_items", ProductCol: "product_id"},
	{Table: "customer_direct_ship_request_allocations", ProductCol: "product_id"},
	{Table: "mall_products", ProductCol: "product_id"},
}

func ensureBusinessIdentityWriteGuards(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
CREATE OR REPLACE FUNCTION %[1]s.validate_bom_spec_business_identity()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
	business_product_id BIGINT;
	business_bom_spec_id BIGINT;
	business_bom_variant_id BIGINT;
	business_item_type TEXT := '';
	old_business_product_id BIGINT;
	old_business_bom_spec_id BIGINT;
	old_business_bom_variant_id BIGINT;
	old_business_item_type TEXT := '';
	identity_changed BOOLEAN := true;
	product_configured BOOLEAN := false;
	production_source_id BIGINT := 0;
	production_source_text TEXT := '';
	frozen_order_refs TEXT[];
	frozen_production_identity_derived BOOLEAN := false;
BEGIN
	business_product_id := COALESCE(NULLIF(to_jsonb(NEW)->>TG_ARGV[0], '')::bigint, 0);
	business_bom_spec_id := COALESCE(NULLIF(to_jsonb(NEW)->>TG_ARGV[1], '')::bigint, 0);
	business_bom_variant_id := COALESCE(NULLIF(to_jsonb(NEW)->>TG_ARGV[2], '')::bigint, 0);
	IF business_product_id <= 0 THEN
		RETURN NEW;
	END IF;
	IF COALESCE(TG_ARGV[3], '') <> '' THEN
		business_item_type := lower(COALESCE(to_jsonb(NEW)->>TG_ARGV[3], ''));
		IF business_item_type NOT IN ('product','finished_product') THEN
			RETURN NEW;
		END IF;
	END IF;
	IF TG_OP='UPDATE' THEN
		old_business_product_id := COALESCE(NULLIF(to_jsonb(OLD)->>TG_ARGV[0], '')::bigint, 0);
		old_business_bom_spec_id := COALESCE(NULLIF(to_jsonb(OLD)->>TG_ARGV[1], '')::bigint, 0);
		old_business_bom_variant_id := COALESCE(NULLIF(to_jsonb(OLD)->>TG_ARGV[2], '')::bigint, 0);
		old_business_item_type := lower(COALESCE(to_jsonb(OLD)->>TG_ARGV[3], ''));
		identity_changed := old_business_product_id IS DISTINCT FROM business_product_id
			OR old_business_bom_spec_id IS DISTINCT FROM business_bom_spec_id
			OR old_business_bom_variant_id IS DISTINCT FROM business_bom_variant_id
			OR old_business_item_type IS DISTINCT FROM business_item_type;
	END IF;

	-- PR-622 retired the per-product migration and legacy child mapping tables.
	-- Runtime identity now comes only from the current default published BOM
	-- authority view, which is rebuilt before these triggers are installed.
	SELECT COALESCE(authority.configured,false)
	INTO product_configured
	FROM %[1]s.product_bom_spec_authorities authority
	WHERE authority.product_id=business_product_id;
	IF NOT FOUND OR NOT product_configured THEN
		IF business_bom_spec_id<>0 OR business_bom_variant_id<>0 THEN
			RAISE EXCEPTION 'direct_product_identity_requires_zero_spec: product %%',business_product_id
				USING ERRCODE='check_violation';
		END IF;
		RETURN NEW;
	END IF;
	IF TG_OP='UPDATE' AND NOT identity_changed
	   AND business_bom_spec_id > 0 AND business_bom_variant_id > 0
	   AND EXISTS (
		SELECT 1
		FROM %[1]s.production_bom_version_variants historical_variant
		JOIN %[1]s.production_bom_versions historical_version
		  ON historical_version.id=historical_variant.version_id
		 AND historical_version.status IN ('published','archived')
		WHERE historical_variant.id=business_bom_variant_id
		  AND historical_variant.bom_spec_id=business_bom_spec_id
	   ) THEN
		RETURN NEW;
	END IF;
	-- A production record derived from an already-frozen order or production
	-- record must retain that exact historical variant after the same BOM
	-- publishes a newer version.  Every allowance below follows an explicit
	-- foreign/business key to an upstream row with the same full identity;
	-- an isolated insert with an arbitrary archived variant still falls through
	-- to the current-default checks below.
	IF business_bom_spec_id > 0 AND business_bom_variant_id > 0
	   AND EXISTS (
		SELECT 1
		FROM %[1]s.production_bom_version_variants historical_variant
		JOIN %[1]s.production_bom_versions historical_version
		  ON historical_version.id=historical_variant.version_id
		 AND historical_version.status IN ('published','archived')
		WHERE historical_variant.id=business_bom_variant_id
		  AND historical_variant.bom_spec_id=business_bom_spec_id
	   ) THEN
		IF TG_TABLE_NAME='production_plan_items' THEN
			production_source_id := COALESCE(NULLIF(to_jsonb(NEW)->>'processing_request_item_id','')::bigint,0);
			IF production_source_id > 0 AND to_regclass('%[1]s.processing_job_request_items') IS NOT NULL THEN
				SELECT EXISTS (
					SELECT 1 FROM %[1]s.processing_job_request_items request_item
					WHERE request_item.id=production_source_id
					  AND request_item.product_id=business_product_id
					  AND request_item.bom_spec_id=business_bom_spec_id
					  AND request_item.bom_variant_id=business_bom_variant_id
				) INTO frozen_production_identity_derived;
			END IF;
			production_source_text := btrim(COALESCE(to_jsonb(NEW)->>'order_nos',''));
			IF NOT frozen_production_identity_derived AND production_source_text <> ''
			   AND to_regclass('%[1]s.orders') IS NOT NULL AND to_regclass('%[1]s.order_items') IS NOT NULL THEN
				frozen_order_refs := regexp_split_to_array(production_source_text,'[[:space:]]*,[[:space:]]*');
				SELECT COALESCE(cardinality(frozen_order_refs),0)>0 AND NOT EXISTS (
					SELECT 1
					FROM unnest(frozen_order_refs) AS ref(order_no)
					WHERE btrim(ref.order_no)=''
					   OR NOT EXISTS (
						SELECT 1
						FROM %[1]s.orders source_order
						JOIN %[1]s.order_items source_item ON source_item.order_id=source_order.id
						WHERE source_order.order_no=btrim(ref.order_no)
						  AND source_item.product_id=business_product_id
						  AND source_item.bom_spec_id=business_bom_spec_id
						  AND source_item.bom_variant_id=business_bom_variant_id
					   )
				) INTO frozen_production_identity_derived;
			END IF;
		ELSIF TG_TABLE_NAME='work_orders' THEN
			production_source_id := COALESCE(NULLIF(to_jsonb(NEW)->>'production_plan_item_id','')::bigint,0);
			SELECT EXISTS (
				SELECT 1 FROM %[1]s.production_plan_items plan_item
				WHERE plan_item.id=production_source_id
				  AND plan_item.product_id=business_product_id
				  AND plan_item.bom_spec_id=business_bom_spec_id
				  AND plan_item.bom_variant_id=business_bom_variant_id
			) INTO frozen_production_identity_derived;
		ELSIF TG_TABLE_NAME='produce_running_items' THEN
			production_source_text := COALESCE(to_jsonb(NEW)->>'batch_id','');
			SELECT EXISTS (
				SELECT 1 FROM %[1]s.work_orders work_order
				WHERE work_order.batch_id=production_source_text
				  AND work_order.product_id=business_product_id
				  AND work_order.bom_spec_id=business_bom_spec_id
				  AND work_order.bom_variant_id=business_bom_variant_id
			) INTO frozen_production_identity_derived;
		ELSIF TG_TABLE_NAME='produce_running_outputs' THEN
			production_source_id := COALESCE(NULLIF(to_jsonb(NEW)->>'running_item_id','')::bigint,0);
			SELECT EXISTS (
				SELECT 1 FROM %[1]s.produce_running_items running_item
				WHERE running_item.id=production_source_id
				  AND running_item.product_id=business_product_id
				  AND running_item.bom_spec_id=business_bom_spec_id
				  AND running_item.bom_variant_id=business_bom_variant_id
			) INTO frozen_production_identity_derived;
		ELSIF TG_TABLE_NAME='production_logs' THEN
			production_source_id := COALESCE(NULLIF(to_jsonb(NEW)->>'running_item_id','')::bigint,0);
			SELECT EXISTS (
				SELECT 1 FROM %[1]s.produce_running_items running_item
				WHERE running_item.id=production_source_id
				  AND running_item.product_id=business_product_id
				  AND running_item.bom_spec_id=business_bom_spec_id
				  AND running_item.bom_variant_id=business_bom_variant_id
			) INTO frozen_production_identity_derived;
		ELSIF TG_TABLE_NAME IN ('order_stock_batch_allocations','customer_direct_ship_request_allocations') THEN
			-- Receipt of retained production can fulfill a dropship reserved
			-- against that retained sale after BOM republish. Keep the ordinary
			-- exact-order-line path, then allow a different destination variant
			-- only when a posted-in-transaction receipt links that precise batch,
			-- frozen entitlement, production run and open reservation.
			production_source_id := COALESCE(NULLIF(to_jsonb(NEW)->>'order_item_id','')::bigint,0);
			IF to_regclass('%[1]s.order_items') IS NOT NULL AND to_regclass('%[1]s.orders') IS NOT NULL AND to_regclass('%[1]s.stock_batches') IS NOT NULL THEN
			SELECT EXISTS (
				SELECT 1
				FROM %[1]s.order_items source_item
				JOIN %[1]s.orders source_order ON source_order.id=source_item.order_id
				JOIN %[1]s.stock_batches source_batch
				  ON source_batch.id=COALESCE(NULLIF(to_jsonb(NEW)->>'batch_id','')::bigint,0)
				 AND source_batch.item_type='finished_product'
				 AND source_batch.item_id=source_item.product_id
				 AND source_batch.bom_spec_id=business_bom_spec_id
				 AND source_batch.bom_variant_id=business_bom_variant_id
				WHERE source_item.id=production_source_id
				  AND source_item.order_id=COALESCE(NULLIF(to_jsonb(NEW)->>'order_id','')::bigint,0)
				  AND source_item.product_id=business_product_id
				  AND source_item.bom_spec_id=business_bom_spec_id
				  AND source_item.bom_variant_id=business_bom_variant_id
				  AND source_batch.batch_code=COALESCE(to_jsonb(NEW)->>'batch_code','')
				  AND source_batch.spec_g=COALESCE(NULLIF(to_jsonb(NEW)->>'spec_g','')::bigint,0)
				  AND source_batch.owner_customer_id=COALESCE((to_jsonb(source_order)->>'customer_id')::bigint,0)
			) INTO frozen_production_identity_derived;
			END IF;
			-- An ordinary sale may reserve a company batch frozen to the same
			-- historical order line. This is not a new choice of BOM: require the
			-- exact batch, stock location, unit and remaining demand, with no
			-- ownership conversion or implied weight conversion.
			IF NOT frozen_production_identity_derived AND TG_TABLE_NAME='order_stock_batch_allocations' AND TG_OP='INSERT'
			   AND to_regclass('%[1]s.orders') IS NOT NULL AND to_regclass('%[1]s.order_items') IS NOT NULL
			   AND to_regclass('%[1]s.stock_batches') IS NOT NULL AND to_regclass('%[1]s.stock_ledger_entries') IS NOT NULL
			   AND to_regclass('%[1]s.order_stock_deductions') IS NOT NULL AND to_regclass('%[1]s.warehouses') IS NOT NULL THEN
				SELECT EXISTS (
				 SELECT 1 FROM %[1]s.order_items item JOIN %[1]s.orders sale ON sale.id=item.order_id
				 JOIN %[1]s.stock_batches batch ON batch.id=COALESCE((to_jsonb(NEW)->>'batch_id')::bigint,0)
				  AND batch.batch_code=to_jsonb(NEW)->>'batch_code' AND batch.item_type='finished_product'
				  AND batch.item_id=item.product_id AND batch.bom_spec_id=item.bom_spec_id AND batch.bom_variant_id=item.bom_variant_id
				  AND batch.owner_customer_id=0 AND batch.spec_g=0
				 JOIN %[1]s.production_bom_version_variants variant ON variant.id=batch.bom_variant_id AND variant.inventory_unit=item.unit
				 JOIN %[1]s.warehouses warehouse ON warehouse.code=to_jsonb(NEW)->>'warehouse' AND warehouse.active
				  AND COALESCE(warehouse.customer_id,0)=0
				 WHERE item.id=production_source_id AND sale.id=COALESCE((to_jsonb(NEW)->>'order_id')::bigint,0)
				  AND item.product_id=business_product_id AND item.bom_spec_id=business_bom_spec_id AND item.bom_variant_id=business_bom_variant_id
				  AND NOT sale.is_void AND NOT COALESCE((to_jsonb(sale)->>'fulfillment_paused')::boolean,false)
				  AND COALESCE(to_jsonb(sale)->>'resolution_status','') NOT IN('closed','cancelled')
				  AND COALESCE(sale.fulfillment_mode,'')='' AND COALESCE(sale.delivery_mode,'')<>'customer_stock'
				  AND COALESCE(NULLIF(sale.source_warehouse,''),'finished_goods')=warehouse.code
				  AND warehouse.code=(SELECT l.warehouse FROM %[1]s.stock_ledger_entries l WHERE l.source_batch_code=batch.batch_code AND l.item_type=batch.item_type AND l.item_id=batch.item_id AND l.bom_spec_id=batch.bom_spec_id AND l.bom_variant_id=batch.bom_variant_id AND l.spec_g=batch.spec_g AND l.owner_customer_id=0 ORDER BY l.id DESC LIMIT 1)
				  AND lower(btrim(COALESCE(batch.quality_status,''))) NOT IN('hold','held','frozen','blocked','rejected','reject','failed','冻结','冻结批次','不合格','拒收')
				  AND COALESCE((to_jsonb(NEW)->>'request_id')::bigint,0)=0
				  AND COALESCE((to_jsonb(NEW)->>'spec_g')::bigint,0)=0 AND COALESCE((to_jsonb(NEW)->>'allocated_g')::bigint,0)=0
				  AND (to_jsonb(NEW)->>'allocated_units')::bigint>0
				  AND (to_jsonb(NEW)->>'allocated_units')::bigint<=batch.remaining_units-COALESCE((
				   SELECT sum(GREATEST(0,reserved.units-COALESCE(deducted.units,0)))
				   FROM (SELECT a.order_id,sum(a.allocated_units) units FROM %[1]s.order_stock_batch_allocations a JOIN %[1]s.orders prior ON prior.id=a.order_id AND NOT prior.is_void WHERE a.batch_id=batch.id GROUP BY a.order_id) reserved
				   LEFT JOIN LATERAL (SELECT sum(d.deducted_units) units FROM %[1]s.order_stock_deductions d WHERE d.order_id=reserved.order_id AND d.batch_id=batch.id) deducted ON true),0)
				  AND (to_jsonb(NEW)->>'allocated_units')::bigint<=item.qty-COALESCE((to_jsonb(item)->>'resolution_closed_qty')::numeric,0)-GREATEST(
				   COALESCE((SELECT sum(a.allocated_units) FROM %[1]s.order_stock_batch_allocations a WHERE a.order_id=sale.id AND a.product_id=item.product_id AND a.bom_spec_id=item.bom_spec_id),0),
				   COALESCE((SELECT sum(d.deducted_units) FROM %[1]s.order_stock_deductions d WHERE d.order_id=sale.id AND d.product_id=item.product_id AND d.bom_spec_id=item.bom_spec_id),0))
				) INTO frozen_production_identity_derived;
			END IF;
			IF NOT frozen_production_identity_derived
			   AND to_regclass('%[1]s.orders') IS NOT NULL AND to_regclass('%[1]s.order_items') IS NOT NULL
			   AND to_regclass('%[1]s.stock_batches') IS NOT NULL AND to_regclass('%[1]s.produce_running_items') IS NOT NULL
			   AND to_regclass('%[1]s.work_orders') IS NOT NULL
			   AND to_regclass('%[1]s.customer_stock_supply_reservations') IS NOT NULL
			   AND to_regclass('%[1]s.customer_stock_entitlement_receipts') IS NOT NULL
			   AND to_regclass('%[1]s.customer_stock_entitlements') IS NOT NULL THEN
				SELECT EXISTS (
					SELECT 1
					FROM %[1]s.order_items destination_item
					JOIN %[1]s.orders destination_order ON destination_order.id=destination_item.order_id
					JOIN %[1]s.stock_batches source_batch
					  ON source_batch.id=COALESCE(NULLIF(to_jsonb(NEW)->>'batch_id','')::bigint,0)
					 AND source_batch.item_type='finished_product'
					 AND source_batch.item_id=business_product_id
					 AND source_batch.bom_spec_id=business_bom_spec_id
					 AND source_batch.bom_variant_id=business_bom_variant_id
					 AND source_batch.batch_code=COALESCE(to_jsonb(NEW)->>'batch_code','')
					 AND source_batch.spec_g=COALESCE(NULLIF(to_jsonb(NEW)->>'spec_g','')::bigint,0)
					JOIN %[1]s.customer_stock_supply_reservations supply_reservation
					  ON supply_reservation.order_id=destination_order.id
					 AND supply_reservation.order_item_id=destination_item.id
					 AND supply_reservation.request_id=COALESCE(NULLIF(to_jsonb(NEW)->>'request_id','')::bigint,0)
					 AND supply_reservation.status IN ('reserved','partially_converted')
					 AND supply_reservation.reserved_qty>supply_reservation.converted_qty+supply_reservation.released_qty
					JOIN %[1]s.customer_stock_entitlements entitlement
					  ON entitlement.id=supply_reservation.entitlement_id
					 AND entitlement.customer_id=source_batch.owner_customer_id
					 AND entitlement.product_id=business_product_id
					 AND entitlement.bom_spec_id=business_bom_spec_id
					 AND entitlement.bom_variant_id=business_bom_variant_id
					 AND entitlement.spec_g=source_batch.spec_g
					 AND entitlement.warehouse_code=COALESCE(NULLIF(to_jsonb(NEW)->>'warehouse',''),to_jsonb(NEW)->>'warehouse_code','')
					 AND entitlement.status='active'
					JOIN %[1]s.customer_stock_entitlement_receipts entitlement_receipt
					  ON entitlement_receipt.entitlement_id=entitlement.id
					 AND entitlement_receipt.stock_batch_id=source_batch.id
					 AND entitlement_receipt.batch_code=source_batch.batch_code
					 AND entitlement_receipt.warehouse_code=entitlement.warehouse_code
					 AND entitlement_receipt.quantity>0
					JOIN %[1]s.orders retained_order
					  ON retained_order.id=entitlement.source_order_id
					 AND retained_order.customer_id=entitlement.customer_id
					 AND NOT retained_order.is_void
					JOIN %[1]s.order_items retained_item
					  ON retained_item.order_id=retained_order.id
					 AND retained_item.product_id=entitlement.product_id
					 AND retained_item.bom_spec_id=entitlement.bom_spec_id
					 AND retained_item.bom_variant_id=entitlement.bom_variant_id
					JOIN %[1]s.produce_running_items producing_run
					  ON producing_run.id=entitlement_receipt.running_item_id
				 AND producing_run.product_id=entitlement.product_id
				 AND producing_run.bom_spec_id=entitlement.bom_spec_id
				 AND producing_run.bom_variant_id=entitlement.bom_variant_id
				JOIN %[1]s.work_orders producing_order
				  ON producing_order.batch_id=producing_run.batch_id
				 AND producing_order.product_id=entitlement.product_id
				 AND producing_order.bom_spec_id=entitlement.bom_spec_id
				 AND producing_order.bom_variant_id=entitlement.bom_variant_id
				WHERE destination_item.id=production_source_id
				  AND destination_item.order_id=COALESCE(NULLIF(to_jsonb(NEW)->>'order_id','')::bigint,0)
				  AND destination_item.product_id=business_product_id
				  AND destination_item.bom_spec_id=business_bom_spec_id
				  AND destination_order.customer_id=entitlement.customer_id
				  AND retained_order.order_no=ANY(regexp_split_to_array(btrim(COALESCE(producing_order.order_nos,'')),'[[:space:]]*,[[:space:]]*'))
				  AND retained_item.qty>=entitlement.promised_qty
				  AND entitlement.received_qty>=entitlement_receipt.quantity
				  AND COALESCE(NULLIF(to_jsonb(NEW)->>'allocated_units','')::bigint,NULLIF(to_jsonb(NEW)->>'allocated_qty','')::bigint,0)>0
				  AND COALESCE(NULLIF(to_jsonb(NEW)->>'allocated_units','')::bigint,NULLIF(to_jsonb(NEW)->>'allocated_qty','')::bigint,0)<=entitlement_receipt.quantity
				  AND COALESCE(NULLIF(to_jsonb(NEW)->>'allocated_units','')::bigint,NULLIF(to_jsonb(NEW)->>'allocated_qty','')::bigint,0)<=supply_reservation.reserved_qty-supply_reservation.converted_qty-supply_reservation.released_qty
			) INTO frozen_production_identity_derived;
			END IF;
			-- Physical customer stock transferred from a frozen historical batch
			-- may satisfy a new order for the same stable specification. Verify
			-- the complete submitted transfer and customer-owned batch chain.
			IF NOT frozen_production_identity_derived
			   AND to_regclass('%[1]s.customer_direct_ship_requests') IS NOT NULL
			   AND to_regclass('%[1]s.customer_direct_ship_request_orders') IS NOT NULL
			   AND to_regclass('%[1]s.customer_direct_ship_request_items') IS NOT NULL
			   AND to_regclass('%[1]s.customer_direct_ship_request_allocations') IS NOT NULL
			   AND to_regclass('%[1]s.order_stock_batch_allocations') IS NOT NULL
			   AND to_regclass('%[1]s.order_stock_deductions') IS NOT NULL
			   AND to_regclass('%[1]s.orders') IS NOT NULL AND to_regclass('%[1]s.order_items') IS NOT NULL
			   AND to_regclass('%[1]s.stock_batches') IS NOT NULL AND to_regclass('%[1]s.stock_ledger_entries') IS NOT NULL
			   AND to_regclass('%[1]s.stock_entries') IS NOT NULL AND to_regclass('%[1]s.stock_entry_items') IS NOT NULL
			   AND to_regclass('%[1]s.warehouses') IS NOT NULL
			   AND to_regclass('%[1]s.stock_entry_finished_batch_moves') IS NOT NULL THEN
				SELECT EXISTS (
				 SELECT 1 FROM %[1]s.orders destination_order
				 JOIN %[1]s.order_items destination_item ON destination_item.id=production_source_id AND destination_item.order_id=destination_order.id
				 JOIN %[1]s.customer_direct_ship_requests request ON request.id=COALESCE(NULLIF(to_jsonb(NEW)->>'request_id','')::bigint,0)
				   AND request.customer_id=destination_order.customer_id AND request.fulfillment_mode='stock' AND request.status='reserved'
				 JOIN %[1]s.customer_direct_ship_request_orders request_order ON request_order.request_id=request.id AND request_order.order_id=destination_order.id AND request_order.status='reserved'
				 JOIN %[1]s.stock_batches batch ON batch.id=COALESCE(NULLIF(to_jsonb(NEW)->>'batch_id','')::bigint,0)
				   AND batch.batch_code=COALESCE(to_jsonb(NEW)->>'batch_code','') AND batch.item_type='finished_product'
				   AND batch.item_id=business_product_id AND batch.bom_spec_id=business_bom_spec_id AND batch.bom_variant_id=business_bom_variant_id
				   AND batch.spec_g=0 AND batch.owner_customer_id=destination_order.customer_id AND batch.source_doc_type='stock_entry_transfer'
				 JOIN %[1]s.stock_entry_finished_batch_moves move ON move.id=batch.source_doc_id AND move.target_batch_id=batch.id AND move.target_batch_code=batch.batch_code
				   AND move.bom_spec_id=batch.bom_spec_id AND move.bom_variant_id=batch.bom_variant_id AND move.spec_g=batch.spec_g AND move.qty_units=batch.qty_units AND move.qty_g=0
				 JOIN %[1]s.stock_entries entry ON entry.id=move.stock_entry_id AND entry.purpose='material_transfer' AND entry.status='submitted'
				 JOIN %[1]s.stock_entry_items item ON item.id=move.stock_entry_item_id AND item.stock_entry_id=entry.id
				   AND item.item_type='finished_product' AND item.product_id=batch.item_id AND item.bom_spec_id=batch.bom_spec_id AND item.bom_variant_id=batch.bom_variant_id
				   AND item.owner_customer_id=0 AND item.target_owner_customer_id=destination_order.customer_id AND item.spec_g=batch.spec_g
				   AND item.qty_units>=move.qty_units AND item.qty_g=0 AND item.batch_code=move.target_batch_code
				 JOIN %[1]s.stock_batches source ON source.id=move.source_batch_id AND source.batch_code=move.source_batch_code AND batch.source_batch_id=source.batch_code
				   AND source.item_type='finished_product' AND source.item_id=batch.item_id AND source.bom_spec_id=batch.bom_spec_id AND source.bom_variant_id=batch.bom_variant_id AND source.spec_g=batch.spec_g AND source.owner_customer_id=0
				 JOIN %[1]s.warehouses warehouse ON warehouse.code=item.to_warehouse AND warehouse.active=true AND warehouse.kind IN ('finished','customer_finished','customer_processing','customer') AND warehouse.customer_id=destination_order.customer_id
				 WHERE destination_order.id=COALESCE(NULLIF(to_jsonb(NEW)->>'order_id','')::bigint,0) AND NOT destination_order.is_void AND destination_order.fulfillment_mode='stock'
				   AND destination_item.product_id=business_product_id AND destination_item.bom_spec_id=business_bom_spec_id
				   AND destination_order.source_warehouse=warehouse.code
				   AND warehouse.code=COALESCE(NULLIF(to_jsonb(NEW)->>'warehouse',''),to_jsonb(NEW)->>'warehouse_code','')
				   AND COALESCE((SELECT l.warehouse FROM %[1]s.stock_ledger_entries l WHERE l.source_batch_code=batch.batch_code AND l.item_type=batch.item_type AND l.item_id=batch.item_id AND l.bom_spec_id=batch.bom_spec_id AND l.bom_variant_id=batch.bom_variant_id AND l.owner_customer_id=batch.owner_customer_id AND l.spec_g=batch.spec_g ORDER BY l.id DESC LIMIT 1),'')=warehouse.code
				   AND lower(btrim(COALESCE(batch.quality_status,''))) NOT IN ('hold','held','frozen','blocked','rejected','reject','failed','冻结','冻结批次','不合格','拒收')
				   AND COALESCE(NULLIF(to_jsonb(NEW)->>'spec_g','')::bigint,0)=0 AND COALESCE(NULLIF(to_jsonb(NEW)->>'allocated_g','')::bigint,0)=0
				   AND COALESCE(NULLIF(to_jsonb(NEW)->>'allocated_units','')::bigint,0)>0
				   AND (to_jsonb(NEW)->>'allocated_units')::bigint <= batch.remaining_units - COALESCE((
				     SELECT SUM(GREATEST(0, reserved.units-COALESCE(deducted.units,0)))
				     FROM (SELECT a.order_id,SUM(a.allocated_units) AS units FROM %[1]s.order_stock_batch_allocations a
				       JOIN %[1]s.orders prior_order ON prior_order.id=a.order_id AND NOT prior_order.is_void
				       WHERE a.batch_id=batch.id GROUP BY a.order_id) reserved
				     LEFT JOIN LATERAL (SELECT SUM(d.deducted_units) AS units FROM %[1]s.order_stock_deductions d WHERE d.order_id=reserved.order_id AND d.batch_id=batch.id) deducted ON true
				   ),0)
				   AND CASE WHEN TG_TABLE_NAME='customer_direct_ship_request_allocations' THEN
				     request_order.id=COALESCE(NULLIF(to_jsonb(NEW)->>'request_order_id','')::bigint,0)
				     AND (to_jsonb(NEW)->>'allocated_qty')::bigint=(to_jsonb(NEW)->>'allocated_units')::bigint
				     AND EXISTS (SELECT 1 FROM %[1]s.customer_direct_ship_request_items request_item WHERE request_item.id=COALESCE(NULLIF(to_jsonb(NEW)->>'request_item_id','')::bigint,0) AND request_item.request_id=request.id AND request_item.product_id=destination_item.product_id AND request_item.bom_spec_id=destination_item.bom_spec_id AND request_item.bom_variant_id=destination_item.bom_variant_id AND request_item.qty=destination_item.qty)
				     AND (to_jsonb(NEW)->>'allocated_units')::bigint + COALESCE((SELECT SUM(a.allocated_units) FROM %[1]s.customer_direct_ship_request_allocations a WHERE a.request_id=request.id AND a.order_item_id=destination_item.id AND a.status='reserved'),0)<=destination_item.qty
				     AND NOT EXISTS (SELECT 1 FROM %[1]s.customer_direct_ship_request_allocations a WHERE a.request_id=request.id AND a.order_item_id=destination_item.id AND a.batch_id=batch.id AND a.status='reserved')
				   ELSE
				     EXISTS (SELECT 1 FROM %[1]s.customer_direct_ship_request_allocations a WHERE a.request_id=request.id AND a.request_order_id=request_order.id AND a.order_id=destination_order.id AND a.order_item_id=destination_item.id AND a.product_id=business_product_id AND a.bom_spec_id=business_bom_spec_id AND a.bom_variant_id=business_bom_variant_id AND a.batch_id=batch.id AND a.batch_code=batch.batch_code AND a.warehouse_code=warehouse.code AND a.status='reserved' AND a.allocated_units=(to_jsonb(NEW)->>'allocated_units')::bigint AND a.allocated_g=0)
				     AND NOT EXISTS (SELECT 1 FROM %[1]s.order_stock_batch_allocations existing WHERE existing.request_id=request.id AND existing.order_item_id=destination_item.id AND existing.batch_id=batch.id)
				   END
				) INTO frozen_production_identity_derived;
			END IF;
		ELSIF (TG_TABLE_NAME='stock_ledger_entries' AND to_jsonb(NEW)->>'source_doc_type'='sales_order_shipment')
		   OR (TG_TABLE_NAME='order_stock_deductions' AND to_jsonb(NEW)->>'source_doc_type'='sales_order_shipment_inventory_sync_v2') THEN
			-- Shipment consumes an existing allocation, not a newly selected BOM.
			-- Keep the batch's frozen identity even when the destination order uses
			-- a newer variant of the same stable spec. Require the actual batch,
			-- owner, order line, warehouse and complete allocated quantity; a source
			-- type alone must never authorize arbitrary historical inventory writes.
			IF to_regclass('%[1]s.order_stock_batch_allocations') IS NOT NULL
			   AND to_regclass('%[1]s.stock_batches') IS NOT NULL
			   AND to_regclass('%[1]s.stock_ledger_entries') IS NOT NULL THEN
				SELECT EXISTS (
					SELECT 1
					FROM %[1]s.order_stock_batch_allocations allocation
					JOIN %[1]s.orders destination_order ON destination_order.id=allocation.order_id
					JOIN %[1]s.order_items destination_item
					  ON destination_item.id=allocation.order_item_id AND destination_item.order_id=destination_order.id
					 AND destination_item.product_id=business_product_id AND destination_item.bom_spec_id=business_bom_spec_id
					JOIN %[1]s.stock_batches source_batch
					  ON source_batch.id=allocation.batch_id AND source_batch.batch_code=allocation.batch_code
					 AND source_batch.item_type='finished_product' AND source_batch.item_id=business_product_id
					 AND source_batch.bom_spec_id=business_bom_spec_id AND source_batch.bom_variant_id=business_bom_variant_id
					 AND source_batch.spec_g=0
					 AND (source_batch.owner_customer_id=0 OR source_batch.owner_customer_id=destination_order.customer_id)
					WHERE allocation.order_id=COALESCE(NULLIF(to_jsonb(NEW)->>'source_doc_id','')::bigint,0)
					  AND NOT destination_order.is_void
					  AND allocation.product_id=business_product_id AND allocation.bom_spec_id=business_bom_spec_id
					  AND allocation.bom_variant_id=business_bom_variant_id AND allocation.spec_g=0
					  AND COALESCE(NULLIF(to_jsonb(NEW)->>'spec_g','')::bigint,0)=0
					  AND allocation.batch_code=COALESCE(to_jsonb(NEW)->>'source_batch_code',to_jsonb(NEW)->>'batch_code','')
					  AND allocation.warehouse=(SELECT location.warehouse FROM %[1]s.stock_ledger_entries location
						WHERE location.source_batch_code=source_batch.batch_code AND location.item_type='finished_product'
						  AND location.item_id=business_product_id AND location.bom_spec_id=business_bom_spec_id
						ORDER BY location.id DESC LIMIT 1)
					  AND (TG_TABLE_NAME='stock_ledger_entries' OR
						(COALESCE(NULLIF(to_jsonb(NEW)->>'batch_id','')::bigint,0)=source_batch.id
						 AND COALESCE(NULLIF(to_jsonb(NEW)->>'order_id','')::bigint,0)=destination_order.id))
					GROUP BY allocation.order_id,allocation.warehouse,source_batch.id,source_batch.batch_code,source_batch.remaining_units
					HAVING SUM(allocation.allocated_units)>0 AND SUM(allocation.allocated_g)=0
					  AND CASE WHEN TG_TABLE_NAME='stock_ledger_entries' THEN
						to_jsonb(NEW)->>'warehouse'=allocation.warehouse
						AND (to_jsonb(NEW)->>'qty_change_units')::bigint=-SUM(allocation.allocated_units)
						AND (to_jsonb(NEW)->>'qty_after_units')::bigint=source_batch.remaining_units
						AND source_batch.remaining_units>=0
						AND (to_jsonb(NEW)->>'qty_before_units')::bigint-SUM(allocation.allocated_units)=source_batch.remaining_units
						AND COALESCE((to_jsonb(NEW)->>'qty_change_g')::bigint,0)=0
					  ELSE
						(to_jsonb(NEW)->>'deducted_units')::bigint=SUM(allocation.allocated_units)
						AND COALESCE((to_jsonb(NEW)->>'deducted_g')::bigint,0)=0
						AND EXISTS (SELECT 1 FROM %[1]s.stock_ledger_entries outbound
							WHERE outbound.source_doc_type='sales_order_shipment' AND outbound.source_doc_id=allocation.order_id
							  AND outbound.source_batch_code=source_batch.batch_code AND outbound.item_type='finished_product'
							  AND outbound.item_id=business_product_id AND outbound.bom_spec_id=business_bom_spec_id
							  AND outbound.bom_variant_id=business_bom_variant_id AND outbound.warehouse=allocation.warehouse
							  AND outbound.qty_change_units=-(to_jsonb(NEW)->>'deducted_units')::bigint)
					  END
				) INTO frozen_production_identity_derived;
			END IF;
		ELSIF TG_TABLE_NAME='stock_batches' AND to_jsonb(NEW)->>'source_doc_type'='stock_entry_transfer'
		   AND to_regclass('%[1]s.stock_entry_finished_batch_moves') IS NOT NULL THEN
			-- A transfer target inherits the exact source batch identity and quantity.
			SELECT EXISTS (
			 SELECT 1 FROM %[1]s.stock_entry_finished_batch_moves move
			 JOIN %[1]s.stock_entries entry ON entry.id=move.stock_entry_id AND entry.purpose='material_transfer'
			 JOIN %[1]s.stock_entry_items item ON item.id=move.stock_entry_item_id AND item.stock_entry_id=entry.id
			 JOIN %[1]s.stock_batches source ON source.id=move.source_batch_id AND source.batch_code=move.source_batch_code
			 WHERE move.id=COALESCE(NULLIF(to_jsonb(NEW)->>'source_doc_id','')::bigint,0)
			   AND source.item_type='finished_product' AND source.item_id=business_product_id
			   AND source.bom_spec_id=business_bom_spec_id AND source.bom_variant_id=business_bom_variant_id
			   AND item.product_id=business_product_id AND item.bom_spec_id=business_bom_spec_id AND item.bom_variant_id=business_bom_variant_id
			   AND move.bom_spec_id=business_bom_spec_id AND move.bom_variant_id=business_bom_variant_id
			   AND COALESCE((to_jsonb(NEW)->>'owner_customer_id')::bigint,0)=COALESCE(NULLIF(item.target_owner_customer_id,0),source.owner_customer_id)
			   AND (item.target_owner_customer_id=0 OR (source.owner_customer_id=0 AND EXISTS (SELECT 1 FROM %[1]s.warehouses destination WHERE destination.code=item.to_warehouse AND destination.customer_id=item.target_owner_customer_id)))
			   AND item.owner_customer_id=source.owner_customer_id AND source.spec_g=move.spec_g
			   AND COALESCE(to_jsonb(NEW)->>'source_batch_id','')=source.batch_code
			   AND COALESCE((to_jsonb(NEW)->>'spec_g')::bigint,0)=move.spec_g
			   AND COALESCE((to_jsonb(NEW)->>'qty_units')::bigint,0)=move.qty_units
			   AND COALESCE((to_jsonb(NEW)->>'qty_g')::bigint,0)=move.qty_g
			) INTO frozen_production_identity_derived;
		ELSIF TG_TABLE_NAME='stock_ledger_entries' AND to_jsonb(NEW)->>'source_doc_type' IN ('stock_entry','stock_entry_cancel')
		   AND to_regclass('%[1]s.stock_entry_finished_batch_moves') IS NOT NULL
		   AND EXISTS (SELECT 1 FROM %[1]s.stock_entries transfer_entry WHERE transfer_entry.id=COALESCE(NULLIF(to_jsonb(NEW)->>'source_doc_id','')::bigint,0) AND transfer_entry.purpose='material_transfer') THEN
			-- Outbound, inbound and reversal ledgers must follow a recorded batch move.
			SELECT EXISTS (
			 SELECT 1 FROM %[1]s.stock_entry_finished_batch_moves move
			 JOIN %[1]s.stock_entries entry ON entry.id=move.stock_entry_id AND entry.purpose='material_transfer'
			 JOIN %[1]s.stock_entry_items item ON item.id=move.stock_entry_item_id AND item.stock_entry_id=entry.id
			 JOIN %[1]s.stock_batches source ON source.id=move.source_batch_id AND source.batch_code=move.source_batch_code
			 JOIN %[1]s.stock_batches target ON target.id=move.target_batch_id AND target.batch_code=move.target_batch_code
			 WHERE entry.id=COALESCE(NULLIF(to_jsonb(NEW)->>'source_doc_id','')::bigint,0)
			   AND source.item_type='finished_product' AND source.item_id=business_product_id
			   AND source.bom_spec_id=business_bom_spec_id AND source.bom_variant_id=business_bom_variant_id
			   AND item.product_id=business_product_id AND item.bom_spec_id=business_bom_spec_id AND item.bom_variant_id=business_bom_variant_id
			   AND move.bom_spec_id=business_bom_spec_id AND move.bom_variant_id=business_bom_variant_id
			   AND item.owner_customer_id=source.owner_customer_id AND target.owner_customer_id=COALESCE(NULLIF(item.target_owner_customer_id,0),source.owner_customer_id)
			   AND (item.target_owner_customer_id=0 OR (source.owner_customer_id=0 AND EXISTS (SELECT 1 FROM %[1]s.warehouses destination WHERE destination.code=item.to_warehouse AND destination.customer_id=item.target_owner_customer_id)))
			   AND target.item_type='finished_product' AND target.item_id=business_product_id
			   AND target.bom_spec_id=business_bom_spec_id AND target.bom_variant_id=business_bom_variant_id
			   AND source.spec_g=move.spec_g AND target.spec_g=move.spec_g
			   AND COALESCE((to_jsonb(NEW)->>'owner_customer_id')::bigint,0)=CASE WHEN to_jsonb(NEW)->>'source_batch_code'=move.source_batch_code THEN source.owner_customer_id ELSE target.owner_customer_id END
			   AND COALESCE((to_jsonb(NEW)->>'spec_g')::bigint,0)=move.spec_g
			   AND (CASE WHEN to_jsonb(NEW)->>'source_doc_type'='stock_entry' THEN
			      (to_jsonb(NEW)->>'source_batch_code'=move.source_batch_code AND to_jsonb(NEW)->>'warehouse'=item.from_warehouse AND (to_jsonb(NEW)->>'qty_change_units')::bigint=-move.qty_units AND (to_jsonb(NEW)->>'qty_change_g')::bigint=-move.qty_g)
			      OR (to_jsonb(NEW)->>'source_batch_code'=move.target_batch_code AND to_jsonb(NEW)->>'warehouse'=item.to_warehouse AND (to_jsonb(NEW)->>'qty_change_units')::bigint=move.qty_units AND (to_jsonb(NEW)->>'qty_change_g')::bigint=move.qty_g)
			     ELSE
			      (to_jsonb(NEW)->>'source_batch_code'=move.target_batch_code AND to_jsonb(NEW)->>'warehouse'=item.to_warehouse AND (to_jsonb(NEW)->>'qty_change_units')::bigint=-move.qty_units AND (to_jsonb(NEW)->>'qty_change_g')::bigint=-move.qty_g)
			      OR (to_jsonb(NEW)->>'source_batch_code'=move.source_batch_code AND to_jsonb(NEW)->>'warehouse'=item.from_warehouse AND (to_jsonb(NEW)->>'qty_change_units')::bigint=move.qty_units AND (to_jsonb(NEW)->>'qty_change_g')::bigint=move.qty_g)
			     END)
			) INTO frozen_production_identity_derived;
		ELSIF to_regclass('%[1]s.order_resolution_stock_returns') IS NOT NULL AND (
          (TG_TABLE_NAME IN('stock_batches','stock_ledger_entries') AND to_jsonb(NEW)->>'source_doc_type'='order_resolution_return')
          OR (TG_TABLE_NAME='finished_inventory' AND COALESCE(current_setting('kferp.order_return_id',true),'')<>'')) THEN
          -- A return inherits a posted shipment's batch identity and cost. The
          -- processing command and pending warehouse task authorize this one
          -- receipt, and a posted return cannot authorize another receipt.
          SELECT EXISTS(
           SELECT 1 FROM %[1]s.order_resolution_stock_returns x
           JOIN %[1]s.order_resolution_requests request ON request.id=x.request_id AND request.order_id=x.order_id AND request.status='pending' AND request.command_json->>'action'='return'
           JOIN %[1]s.order_resolution_steps step ON step.id=x.step_id AND step.request_id=request.id AND step.kind='return_stock' AND step.choice='receive' AND step.permission='stock.resolve' AND step.status IN('pending','failed')
           JOIN %[1]s.stock_batches source ON source.id=x.batch_id AND source.item_type='finished_product' AND source.item_id=x.product_id AND source.bom_spec_id=x.bom_spec_id AND source.bom_variant_id=x.bom_variant_id AND source.spec_g=0 AND source.owner_customer_id=x.source_owner_customer_id AND source.unit_cost=x.unit_cost
           JOIN %[1]s.order_stock_deductions deduction ON deduction.id=x.source_id AND deduction.order_id=x.order_id AND deduction.batch_id=source.id AND deduction.batch_code=source.batch_code AND deduction.product_id=source.item_id AND deduction.bom_spec_id=source.bom_spec_id AND deduction.bom_variant_id=source.bom_variant_id AND deduction.spec_g=0 AND deduction.deducted_g=0 AND deduction.source_doc_type='sales_order_shipment_inventory_sync_v2' AND deduction.source_doc_id=x.order_id
           JOIN %[1]s.order_items item ON item.id=x.order_item_id AND item.order_id=x.order_id AND item.product_id=x.product_id AND item.bom_spec_id=x.bom_spec_id
           JOIN %[1]s.warehouses destination ON destination.code=x.target_warehouse AND destination.active AND COALESCE(destination.customer_id,0)=x.target_owner_customer_id
           WHERE x.id=CASE WHEN TG_TABLE_NAME='finished_inventory' THEN COALESCE(NULLIF(current_setting('kferp.order_return_id',true),'')::bigint,0) ELSE COALESCE((to_jsonb(NEW)->>'source_doc_id')::bigint,0) END
            AND x.status='posting' AND x.source_kind='shipment' AND x.source_owner_customer_id=x.target_owner_customer_id
            AND x.product_id=business_product_id AND x.bom_spec_id=business_bom_spec_id AND x.bom_variant_id=business_bom_variant_id
            AND COALESCE((to_jsonb(NEW)->>'spec_g')::bigint,0)=0 AND COALESCE((to_jsonb(NEW)->>'owner_customer_id')::bigint,0)=x.target_owner_customer_id
            AND EXISTS(SELECT 1 FROM %[1]s.order_stock_batch_allocations a WHERE a.order_id=x.order_id AND a.order_item_id=x.order_item_id AND a.product_id=x.product_id AND a.bom_spec_id=x.bom_spec_id AND a.bom_variant_id=x.bom_variant_id AND a.batch_id=x.batch_id AND a.batch_code=source.batch_code AND a.warehouse=x.source_warehouse AND a.allocated_units>=deduction.deducted_units AND a.allocated_g=0 AND a.spec_g=0)
            AND x.quantity<=deduction.deducted_units-COALESCE((SELECT sum(previous.quantity) FROM %[1]s.order_resolution_stock_returns previous WHERE previous.source_kind=x.source_kind AND previous.source_id=x.source_id AND previous.status='posted'),0)
            AND EXISTS(SELECT 1 FROM %[1]s.stock_ledger_entries shipped WHERE shipped.source_doc_type='sales_order_shipment' AND shipped.source_doc_id=x.order_id AND shipped.source_batch_code=source.batch_code AND shipped.item_type='finished_product' AND shipped.item_id=x.product_id AND shipped.bom_spec_id=x.bom_spec_id AND shipped.bom_variant_id=x.bom_variant_id AND shipped.spec_g=0 AND shipped.owner_customer_id=x.source_owner_customer_id AND shipped.warehouse=x.source_warehouse AND shipped.qty_change_units<0 AND shipped.qty_change_g=0)
            AND EXISTS(SELECT 1 FROM jsonb_array_elements(request.command_json->'return_selections') selected WHERE selected->>'source_kind'=x.source_kind AND (selected->>'source_id')::bigint=x.source_id AND (selected->>'batch_id')::bigint=x.batch_id AND (selected->>'quantity')::bigint=x.quantity AND selected->>'warehouse'=x.target_warehouse AND selected->>'quality'=x.quality AND selected->>'ownership'=CASE WHEN x.target_owner_customer_id=0 THEN 'company' ELSE 'customer' END)
            AND CASE WHEN TG_TABLE_NAME='stock_batches' THEN
              x.target_batch_id=0 AND to_jsonb(NEW)->>'batch_code'='ORRET-'||x.id::text AND to_jsonb(NEW)->>'source_batch_id'=source.batch_code
              AND (to_jsonb(NEW)->>'qty_units')::bigint=x.quantity AND (to_jsonb(NEW)->>'remaining_units')::bigint=x.quantity AND COALESCE((to_jsonb(NEW)->>'qty_g')::bigint,0)=0
              AND (to_jsonb(NEW)->>'unit_cost')::numeric=x.unit_cost AND to_jsonb(NEW)->>'quality_status'=CASE WHEN x.quality='restock' THEN 'passed' ELSE 'hold' END
            WHEN TG_TABLE_NAME='stock_ledger_entries' THEN
              to_jsonb(NEW)->>'source_batch_code'=x.target_batch_code AND to_jsonb(NEW)->>'source_batch_id'=source.batch_code AND to_jsonb(NEW)->>'warehouse'=x.target_warehouse
              AND (to_jsonb(NEW)->>'qty_change_units')::bigint=x.quantity AND COALESCE((to_jsonb(NEW)->>'qty_change_g')::bigint,0)=0
              AND (to_jsonb(NEW)->>'qty_after_units')::bigint=(to_jsonb(NEW)->>'qty_before_units')::bigint+x.quantity
              AND (to_jsonb(NEW)->>'qty_after_units')::bigint=(SELECT onhand_units FROM %[1]s.finished_inventory f WHERE f.product_id=x.product_id AND f.bom_spec_id=x.bom_spec_id AND f.spec_g=0 AND f.warehouse=x.target_warehouse AND f.owner_customer_id=x.target_owner_customer_id)
              AND NOT EXISTS(SELECT 1 FROM %[1]s.stock_ledger_entries prior WHERE prior.source_doc_type='order_resolution_return' AND prior.source_doc_id=x.id)
              AND EXISTS(SELECT 1 FROM %[1]s.stock_batches target WHERE target.id=x.target_batch_id AND target.batch_code=x.target_batch_code AND target.source_doc_type='order_resolution_return' AND target.source_doc_id=x.id AND target.item_id=x.product_id AND target.bom_spec_id=x.bom_spec_id AND target.bom_variant_id=x.bom_variant_id AND target.owner_customer_id=x.target_owner_customer_id AND target.qty_units=x.quantity)
            ELSE
              to_jsonb(NEW)->>'warehouse'=x.target_warehouse
              AND (to_jsonb(NEW)->>'onhand_units')::bigint=x.quantity+COALESCE((SELECT f.onhand_units FROM %[1]s.finished_inventory f WHERE f.product_id=x.product_id AND f.bom_spec_id=x.bom_spec_id AND f.spec_g=0 AND f.warehouse=x.target_warehouse AND f.owner_customer_id=x.target_owner_customer_id),0)
              AND COALESCE((to_jsonb(NEW)->>'onhand_loose_g')::bigint,0)=COALESCE((SELECT f.onhand_loose_g FROM %[1]s.finished_inventory f WHERE f.product_id=x.product_id AND f.bom_spec_id=x.bom_spec_id AND f.spec_g=0 AND f.warehouse=x.target_warehouse AND f.owner_customer_id=x.target_owner_customer_id),0)
            END
          ) INTO frozen_production_identity_derived;
		ELSIF TG_TABLE_NAME='finished_inventory' THEN
			-- Aggregate inventory has no source-document columns. Production receipt
			-- code supplies a transaction-local work-order id only for the duration
			-- of its inventory upsert; accept the historical identity only when that
			-- exact order froze the same product/spec/variant and BOM version.
			production_source_id := COALESCE(NULLIF(current_setting('kferp.production_frozen_work_order_id',true), '')::bigint,0);
			SELECT EXISTS (
				SELECT 1
				FROM %[1]s.work_orders work_order
				JOIN %[1]s.production_bom_version_variants frozen_variant
				  ON frozen_variant.id=work_order.bom_variant_id
				 AND frozen_variant.bom_spec_id=work_order.bom_spec_id
				 AND frozen_variant.version_id=work_order.bom_version_id
				WHERE work_order.id=production_source_id
				  AND work_order.product_id=business_product_id
				  AND work_order.bom_spec_id=business_bom_spec_id
				  AND work_order.bom_variant_id=business_bom_variant_id
			) INTO frozen_production_identity_derived;
			IF NOT frozen_production_identity_derived AND to_regclass('%[1]s.stock_entry_finished_batch_moves') IS NOT NULL THEN
			 SELECT EXISTS (
			  SELECT 1 FROM %[1]s.stock_entry_items item
			  JOIN %[1]s.stock_entries entry ON entry.id=item.stock_entry_id AND entry.purpose='material_transfer'
			  JOIN %[1]s.stock_batches source ON source.id=COALESCE((SELECT m.source_batch_id FROM %[1]s.stock_entry_finished_batch_moves m WHERE m.stock_entry_item_id=item.id ORDER BY m.id LIMIT 1), (SELECT b.id FROM %[1]s.stock_batches b WHERE b.batch_code=item.batch_code LIMIT 1))
			  WHERE item.id=COALESCE(NULLIF(current_setting('kferp.stock_transfer_item_id',true),'')::bigint,0)
			    AND item.item_type='finished_product' AND item.product_id=business_product_id
			    AND item.bom_spec_id=business_bom_spec_id AND item.bom_variant_id=business_bom_variant_id
			    AND source.item_type='finished_product' AND source.item_id=business_product_id
			    AND source.bom_spec_id=business_bom_spec_id AND source.bom_variant_id=business_bom_variant_id
			    AND item.owner_customer_id=source.owner_customer_id
			    AND COALESCE((to_jsonb(NEW)->>'owner_customer_id')::bigint,0)=CASE WHEN to_jsonb(NEW)->>'warehouse'=item.to_warehouse THEN COALESCE(NULLIF(item.target_owner_customer_id,0),source.owner_customer_id) ELSE source.owner_customer_id END
			    AND (item.target_owner_customer_id=0 OR (source.owner_customer_id=0 AND EXISTS (SELECT 1 FROM %[1]s.warehouses destination WHERE destination.code=item.to_warehouse AND destination.customer_id=item.target_owner_customer_id)))
			    AND to_jsonb(NEW)->>'warehouse' IN (item.from_warehouse,item.to_warehouse)
			    AND COALESCE((to_jsonb(NEW)->>'spec_g')::bigint,0)=source.spec_g
			 ) INTO frozen_production_identity_derived;
			END IF;
		ELSIF TG_TABLE_NAME IN ('stock_batches','stock_ledger_entries') THEN
			production_source_id := COALESCE(NULLIF(to_jsonb(NEW)->>'source_doc_id','')::bigint,0);
			production_source_text := lower(COALESCE(to_jsonb(NEW)->>'source_doc_type',''));
			IF production_source_text='production_run' THEN
				SELECT EXISTS (
					SELECT 1 FROM %[1]s.produce_running_items running_item
					WHERE running_item.id=production_source_id
					  AND running_item.product_id=business_product_id
					  AND running_item.bom_spec_id=business_bom_spec_id
					  AND running_item.bom_variant_id=business_bom_variant_id
				) INTO frozen_production_identity_derived;
			ELSIF production_source_text='production_operation_event' THEN
				-- Operation receipts use the completion event as their inventory
				-- source. Follow that event to its work order and accept only the
				-- exact frozen product/spec/variant/version identity.
				SELECT EXISTS (
					SELECT 1
					FROM %[1]s.production_operation_events operation_event
					JOIN %[1]s.work_orders work_order ON work_order.id=operation_event.work_order_id
					JOIN %[1]s.production_bom_version_variants frozen_variant
					  ON frozen_variant.id=work_order.bom_variant_id
					 AND frozen_variant.bom_spec_id=work_order.bom_spec_id
					 AND frozen_variant.version_id=work_order.bom_version_id
					WHERE operation_event.id=production_source_id
					  AND work_order.product_id=business_product_id
					  AND work_order.bom_spec_id=business_bom_spec_id
					  AND work_order.bom_variant_id=business_bom_variant_id
				) INTO frozen_production_identity_derived;
			ELSIF TG_TABLE_NAME='stock_ledger_entries' AND production_source_text='stock_entry' THEN
				SELECT EXISTS (
					SELECT 1
					FROM %[1]s.stock_entries stock_entry
					JOIN %[1]s.produce_running_items running_item ON running_item.id=stock_entry.running_item_id
					WHERE stock_entry.id=production_source_id
					  AND running_item.product_id=business_product_id
					  AND running_item.bom_spec_id=business_bom_spec_id
					  AND running_item.bom_variant_id=business_bom_variant_id
				) INTO frozen_production_identity_derived;
			END IF;
		ELSIF TG_TABLE_NAME='stock_entry_items' THEN
			production_source_id := COALESCE(NULLIF(to_jsonb(NEW)->>'stock_entry_id','')::bigint,0);
			SELECT EXISTS (
				SELECT 1
				FROM %[1]s.stock_entries stock_entry
				JOIN %[1]s.produce_running_items running_item ON running_item.id=stock_entry.running_item_id
				WHERE stock_entry.id=production_source_id
				  AND running_item.product_id=business_product_id
				  AND running_item.bom_spec_id=business_bom_spec_id
				  AND running_item.bom_variant_id=business_bom_variant_id
			) OR EXISTS (
				SELECT 1
				FROM %[1]s.stock_entries stock_entry
				JOIN %[1]s.work_orders work_order ON work_order.id=stock_entry.work_order_id
				WHERE COALESCE(to_jsonb(NEW)->>'item_type','')='finished_product'
				  AND stock_entry.id=production_source_id
				  AND stock_entry.entry_type='finished_receipt'
				  AND stock_entry.purpose='manufacture'
				  AND work_order.product_id=business_product_id
				  AND work_order.bom_spec_id=business_bom_spec_id
				  AND work_order.bom_variant_id=business_bom_variant_id
			) INTO frozen_production_identity_derived;
			IF NOT frozen_production_identity_derived AND COALESCE(to_jsonb(NEW)->>'batch_code','')<>'' THEN
			 SELECT EXISTS (
			  SELECT 1 FROM %[1]s.stock_entries entry
			  JOIN %[1]s.stock_batches source ON source.batch_code=to_jsonb(NEW)->>'batch_code'
			  WHERE entry.id=production_source_id AND entry.purpose='material_transfer'
			    AND source.item_type='finished_product' AND source.item_id=business_product_id
			    AND source.bom_spec_id=business_bom_spec_id AND source.bom_variant_id=business_bom_variant_id
			    AND source.owner_customer_id=COALESCE((to_jsonb(NEW)->>'owner_customer_id')::bigint,0)
			    AND source.spec_g=COALESCE((to_jsonb(NEW)->>'spec_g')::bigint,0)
			    AND COALESCE((SELECT l.warehouse FROM %[1]s.stock_ledger_entries l WHERE l.source_batch_code=source.batch_code AND l.item_type=source.item_type AND l.item_id=source.item_id AND l.bom_spec_id=source.bom_spec_id AND l.owner_customer_id=source.owner_customer_id ORDER BY l.id DESC LIMIT 1),'finished_goods')=to_jsonb(NEW)->>'from_warehouse'
			 ) INTO frozen_production_identity_derived;
			END IF;
		END IF;
		IF frozen_production_identity_derived THEN
			RETURN NEW;
		END IF;
	END IF;
	IF business_bom_spec_id <= 0 THEN
		RAISE EXCEPTION 'bom_spec_id_required: product %%',business_product_id USING ERRCODE='check_violation';
	END IF;
	IF business_bom_variant_id <= 0 THEN
		RAISE EXCEPTION 'bom_variant_id_required: product %% spec %%',business_product_id,business_bom_spec_id
			USING ERRCODE='check_violation';
	END IF;
	IF NOT EXISTS (
		SELECT 1
		FROM %[1]s.production_bom_output_bindings binding
		JOIN %[1]s.production_bom_versions version
		  ON version.id=binding.bom_version_id AND version.bom_id=binding.bom_id AND version.status='published'
		JOIN %[1]s.production_bom_specs spec ON spec.id=business_bom_spec_id AND spec.bom_id=binding.bom_id
		JOIN %[1]s.production_bom_version_variants variant
		  ON variant.version_id=version.id AND variant.bom_spec_id=spec.id
		WHERE binding.output_type='product' AND binding.output_id=business_product_id AND binding.is_default=true
	) THEN
		RAISE EXCEPTION 'bom_spec_not_published: product %% spec %%',business_product_id,business_bom_spec_id
			USING ERRCODE='check_violation';
	END IF;
	IF NOT EXISTS (
		SELECT 1
		FROM %[1]s.production_bom_output_bindings binding
		JOIN %[1]s.production_bom_versions version
		  ON version.id=binding.bom_version_id AND version.bom_id=binding.bom_id AND version.status='published'
		JOIN %[1]s.production_bom_specs spec ON spec.id=business_bom_spec_id AND spec.bom_id=binding.bom_id
		JOIN %[1]s.production_bom_version_variants variant
		  ON variant.id=business_bom_variant_id AND variant.version_id=version.id AND variant.bom_spec_id=spec.id
		WHERE binding.output_type='product' AND binding.output_id=business_product_id AND binding.is_default=true
	) THEN
		RAISE EXCEPTION 'bom_variant_not_current: product %% spec %% variant %%',business_product_id,business_bom_spec_id,business_bom_variant_id
			USING ERRCODE='check_violation';
	END IF;
	RETURN NEW;
END $$;
`, schema)); err != nil {
		return err
	}
	for _, target := range businessIdentityTables {
		if target.ProductCol == "" {
			continue
		}
		columns := []string{target.ProductCol, "bom_spec_id", "bom_variant_id"}
		if target.TypeCol != "" {
			columns = append(columns, target.TypeCol)
		}
		ok, err := tableHasColumns(ctx, pool, schema, target.Table, columns...)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		triggerColumns := strings.Join(columns, ",")
		stmt := fmt.Sprintf(`
DROP TRIGGER IF EXISTS bom_spec_identity_guard ON %s.%s;
CREATE TRIGGER bom_spec_identity_guard
		BEFORE INSERT OR UPDATE OF %s ON %s.%s
		FOR EACH ROW EXECUTE FUNCTION %s.validate_bom_spec_business_identity('%s','bom_spec_id','bom_variant_id','%s');
`, schema, target.Table, triggerColumns, schema, target.Table, schema, target.ProductCol, target.TypeCol)
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("install %s identity guard: %w", target.Table, err)
		}
	}
	// PR-600 initially extended aliases with the generic identity guard.  Drop
	// it on upgrades too: an alias describes the parent product and has no
	// concrete specification field in its API contract.
	aliasExists, err := tableHasColumns(ctx, pool, schema, "customer_product_aliases", "product_id")
	if err != nil {
		return err
	}
	if aliasExists {
		if _, err := pool.Exec(ctx, fmt.Sprintf(`DROP TRIGGER IF EXISTS bom_spec_identity_guard ON %s.customer_product_aliases`, schema)); err != nil {
			return fmt.Errorf("remove family alias identity guard: %w", err)
		}
	}
	return nil
}

func ensureLegacyChildCatalogWriteGuard(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	hasColumns, err := tableHasColumns(ctx, pool, schema, "products", "parent_product_id", "base_product_id", "custom_type", "active")
	if err != nil || !hasColumns {
		return err
	}
	_, err = pool.Exec(ctx, fmt.Sprintf(`
CREATE OR REPLACE FUNCTION %[1]s.validate_legacy_child_product_write()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
	direct_parent_id BIGINT;
	base_parent_id BIGINT;
	child_parent_id BIGINT;
	child_custom_type TEXT;
	child_active BOOLEAN;
	parent_state TEXT;
	parent_identity_mode TEXT;
BEGIN
	direct_parent_id := COALESCE(NULLIF(to_jsonb(NEW)->>'parent_product_id','')::bigint,0);
	base_parent_id := COALESCE(NULLIF(to_jsonb(NEW)->>'base_product_id','')::bigint,0);
	child_custom_type := lower(COALESCE(to_jsonb(NEW)->>'custom_type',''));
	child_active := COALESCE(NULLIF(to_jsonb(NEW)->>'active','')::boolean,true);
	child_parent_id := direct_parent_id;
	IF child_parent_id <= 0 AND base_parent_id > 0 AND child_custom_type <> 'public_sku_alias' THEN
		child_parent_id := base_parent_id;
	END IF;
	IF child_parent_id <= 0 OR NOT child_active THEN
		RETURN NEW;
	END IF;

	-- Match repository cutover/catalog serialization so an insert cannot pass
	-- between creation of the migration row and the final cutover state update.
	PERFORM pg_advisory_xact_lock(child_parent_id);
	SELECT state,
	       COALESCE(NULLIF(spec_identity_mode,''),CASE WHEN state='cutover' OR legacy_catalog_product=false THEN 'bom_spec' ELSE 'legacy_sku' END)
	INTO parent_state,parent_identity_mode
	FROM %[1]s.product_bom_spec_migrations
	WHERE product_id=child_parent_id
	FOR SHARE;
	IF COALESCE(parent_state,'legacy')='cutover' OR COALESCE(parent_identity_mode,'legacy_sku') IN ('product','bom_spec') THEN
		RAISE EXCEPTION 'legacy_child_sku_write_rejected: parent %% is already cut over to BOM specs',child_parent_id
			USING ERRCODE='check_violation';
	END IF;
	RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS legacy_child_sku_cutover_guard ON %[1]s.products;
CREATE TRIGGER legacy_child_sku_cutover_guard
	BEFORE INSERT OR UPDATE OF parent_product_id,base_product_id,custom_type,active ON %[1]s.products
	FOR EACH ROW EXECUTE FUNCTION %[1]s.validate_legacy_child_product_write();
`, schema))
	return err
}

func tableHasColumns(ctx context.Context, pool *pgxpool.Pool, schema, table string, columns ...string) (bool, error) {
	var count int
	err := pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT column_name)::int
		FROM information_schema.columns
		WHERE table_schema=$1 AND table_name=$2 AND column_name=ANY($3)
	`, schema, table, columns).Scan(&count)
	return count == len(columns), err
}
