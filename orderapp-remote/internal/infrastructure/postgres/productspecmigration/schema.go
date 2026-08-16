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
`, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
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
	cutover_parent_id BIGINT;
	cutover_parent_state TEXT;
	product_state TEXT;
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

	SELECT mapping.parent_product_id,migration.state INTO cutover_parent_id,cutover_parent_state
	FROM %[1]s.legacy_child_sku_bom_spec_mappings mapping
	JOIN %[1]s.product_bom_spec_migrations migration ON migration.product_id=mapping.parent_product_id
	WHERE mapping.legacy_child_product_id=business_product_id
	FOR SHARE OF migration;
	IF FOUND AND cutover_parent_state='cutover' THEN
		RAISE EXCEPTION 'legacy_child_sku_write_rejected: child %% belongs to cutover product %%', business_product_id,cutover_parent_id
			USING ERRCODE='check_violation';
	END IF;

	SELECT state INTO product_state
	FROM %[1]s.product_bom_spec_migrations
	WHERE product_id=business_product_id
	FOR SHARE;
	IF COALESCE(product_state,'legacy') <> 'cutover' THEN
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
				frozen_order_refs := regexp_split_to_array(production_source_text,'\\s*,\\s*');
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
			) INTO frozen_production_identity_derived;
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
	SELECT state INTO parent_state
	FROM %[1]s.product_bom_spec_migrations
	WHERE product_id=child_parent_id
	FOR SHARE;
	IF COALESCE(parent_state,'legacy')='cutover' THEN
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
