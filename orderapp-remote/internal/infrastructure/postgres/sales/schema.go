package sales

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	if err := ensureOrderProcessStatuses(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureShippingClosureSchema(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureOrderShippingTrackingTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := EnsureSenderSettingsTable(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureOutsourceFeeColumns(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureOutsourceTemplateTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureProcessingBillingTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureCustomerCompanyColumns(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureOrderResponsibleColumns(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureOrderPaymentMethodColumn(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureOrderDocumentDateColumn(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureSalesOrderNoteColumn(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureOrderBeanListColumns(ctx, pool, schema); err != nil {
		return err
	}
	if err := backfillERPOrdersForFulfillmentCustomers(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureLogisticsSettingsTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureSalesOrderTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureOrderFulfillmentColumns(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureOrderItemUnitPricingColumns(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureOrderInvoiceTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureEmployeeOrderDraftTable(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureOrderStockDecisionTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureDeliveryNoteTables(ctx, pool, schema); err != nil {
		return err
	}
	return ensureExternalShareResourceTables(ctx, pool, schema)
}

func ensureEmployeeOrderDraftTable(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s.employee_order_drafts (
			id BIGSERIAL PRIMARY KEY,
			employee_id BIGINT NOT NULL REFERENCES %s.company_employees(id) ON DELETE CASCADE,
			payload JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			UNIQUE(employee_id)
		)
	`, schema, schema))
	return err
}

func ensureOrderBeanListColumns(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	stmts := []string{
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS bean_list_publication_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS bean_list_version_no TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_orders_bean_list_publication ON %s.orders(bean_list_publication_id)`, schema, schema),
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func ensureSalesOrderNoteColumn(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS sales_order_note TEXT NOT NULL DEFAULT ''`, schema))
	return err
}

func ensureOrderDocumentDateColumn(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	stmts := []string{
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS document_date DATE`, schema),
		fmt.Sprintf(`UPDATE %s.orders SET document_date=order_date WHERE document_date IS NULL`, schema),
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func ensureCustomerCompanyColumns(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	stmts := []string{
		fmt.Sprintf(`ALTER TABLE %s.customers ADD COLUMN IF NOT EXISTS company_name TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.customers ADD COLUMN IF NOT EXISTS company_address TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.customers ADD COLUMN IF NOT EXISTS company_phone TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.customers ADD COLUMN IF NOT EXISTS responsible_employee_id BIGINT NOT NULL DEFAULT 0`, schema),
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func ensureOrderPaymentMethodColumn(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS payment_method TEXT NOT NULL DEFAULT ''`, schema))
	return err
}

func ensureOrderResponsibleColumns(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	stmts := []string{
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS responsible_party_type TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS responsible_party_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS responsible_party_name TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_orders_responsible_party ON %s.orders(responsible_party_type, responsible_party_id)`, schema, schema),
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func backfillERPOrdersForFulfillmentCustomers(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
DO $$
BEGIN
	IF to_regclass('%[1]s.customer_erp_user_bindings') IS NOT NULL
		AND to_regclass('%[1]s.company_employees') IS NOT NULL
		AND to_regclass('%[1]s.employee_login_passwords') IS NOT NULL
		AND to_regclass('%[1]s.customer_portal_profiles') IS NOT NULL
		AND to_regclass('%[1]s.customer_capability_templates') IS NOT NULL
	THEN
		UPDATE %[1]s.orders o
		SET portal_service_code='product_order',
			source_warehouse=COALESCE(NULLIF(o.source_warehouse,''),'finished_goods')
		WHERE COALESCE(o.portal_service_code,'')=''
		  AND EXISTS (
			SELECT 1
			FROM %[1]s.customer_erp_user_bindings b
			JOIN %[1]s.company_employees e ON e.id=b.employee_id
			LEFT JOIN %[1]s.employee_login_passwords lp ON lp.employee_id=e.id
			JOIN %[1]s.customer_portal_profiles p ON p.customer_id=b.customer_id AND p.enabled=true
			WHERE b.customer_id=o.customer_id
			  AND b.status='active'
			  AND e.active=true
			  AND e.account_type='channel_customer'
			  AND COALESCE(lp.login_disabled,false)=false
			  AND (
				p.capability_template_key IN ('processing_fulfillment','public_sku_direct_ship','channel_direct_ship')
				OR EXISTS (
					SELECT 1 FROM %[1]s.customer_capability_templates active_template
					WHERE active_template.template_key=p.capability_template_key
					  AND active_template.active=true
					  AND (jsonb_array_length(active_template.erp_permissions)>0 OR jsonb_array_length(active_template.erp_view_keys)>0)
				)
			  )
		  );
	END IF;
END $$;
`, schema)
	_, err := pool.Exec(ctx, q)
	return err
}

func ensureOrderProcessStatuses(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	if err := syncSerialIDSequence(ctx, pool, schema, "order_process_statuses"); err != nil {
		return err
	}
	q := fmt.Sprintf(`
		INSERT INTO %s.order_process_statuses(name, sort, active)
		SELECT $1, $2, true
		WHERE NOT EXISTS (
			SELECT 1 FROM %s.order_process_statuses WHERE name=$1
		)
	`, schema, schema)
	for _, item := range []struct {
		name string
		sort int
	}{
		{name: "库存待发货", sort: 33},
		{name: "无需生产", sort: 34},
		{name: "生产完成", sort: 35},
	} {
		if _, err := pool.Exec(ctx, q, item.name, item.sort); err != nil {
			return err
		}
	}
	return nil
}

func ensureOrderStockDecisionTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	stmts := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.order_stock_decisions (
			order_id BIGINT PRIMARY KEY REFERENCES %s.orders(id) ON DELETE CASCADE,
			decision TEXT NOT NULL DEFAULT '',
			operator TEXT NOT NULL DEFAULT '',
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schema, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.order_stock_batch_allocations (
			id BIGSERIAL PRIMARY KEY,
			order_id BIGINT NOT NULL REFERENCES %s.orders(id) ON DELETE CASCADE,
			product_id BIGINT NOT NULL DEFAULT 0,
			bom_spec_id BIGINT NOT NULL DEFAULT 0,
			bom_variant_id BIGINT NOT NULL DEFAULT 0,
			spec_g BIGINT NOT NULL DEFAULT 0,
			need_g BIGINT NOT NULL DEFAULT 0,
			need_units BIGINT NOT NULL DEFAULT 0,
			batch_id BIGINT NOT NULL DEFAULT 0,
			batch_code TEXT NOT NULL DEFAULT '',
			allocated_g BIGINT NOT NULL DEFAULT 0,
			allocated_units BIGINT NOT NULL DEFAULT 0,
			operator TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schema, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.order_stock_deductions (
			id BIGSERIAL PRIMARY KEY,
			order_id BIGINT NOT NULL REFERENCES %s.orders(id) ON DELETE CASCADE,
			product_id BIGINT NOT NULL DEFAULT 0,
			bom_spec_id BIGINT NOT NULL DEFAULT 0,
			bom_variant_id BIGINT NOT NULL DEFAULT 0,
			spec_g BIGINT NOT NULL DEFAULT 0,
			batch_id BIGINT NOT NULL DEFAULT 0,
			batch_code TEXT NOT NULL DEFAULT '',
			deducted_g BIGINT NOT NULL DEFAULT 0,
			deducted_units BIGINT NOT NULL DEFAULT 0,
			source_doc_type TEXT NOT NULL DEFAULT '',
			source_doc_id BIGINT NOT NULL DEFAULT 0,
			operator TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			UNIQUE(order_id, product_id, spec_g, batch_code)
		)`, schema, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_stock_batch_allocations ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_stock_batch_allocations ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_stock_batch_allocations ADD COLUMN IF NOT EXISTS need_units BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_stock_batch_allocations ADD COLUMN IF NOT EXISTS allocated_units BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_stock_deductions ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_stock_deductions ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_stock_deductions ADD COLUMN IF NOT EXISTS deducted_units BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_stock_deductions DROP CONSTRAINT IF EXISTS order_stock_deductions_order_id_product_id_spec_g_batch_code_key`, schema),
		fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS idx_%s_order_stock_deduct_identity_uq ON %s.order_stock_deductions(order_id,product_id,bom_spec_id,batch_code)`, schema, schema),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_order_stock_alloc_order ON %s.order_stock_batch_allocations(order_id)`, schema, schema),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_order_stock_alloc_batch ON %s.order_stock_batch_allocations(batch_code)`, schema, schema),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_order_stock_alloc_identity ON %s.order_stock_batch_allocations(product_id,bom_spec_id,batch_code)`, schema, schema),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_order_stock_deduct_order ON %s.order_stock_deductions(order_id)`, schema, schema),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_order_stock_deduct_batch ON %s.order_stock_deductions(batch_code)`, schema, schema),
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func ensureShippingClosureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	if err := syncSerialIDSequence(ctx, pool, schema, "ship_statuses"); err != nil {
		return err
	}
	statusQ := fmt.Sprintf(`
		INSERT INTO %s.ship_statuses(name)
		SELECT $1
		WHERE NOT EXISTS (
			SELECT 1 FROM %s.ship_statuses WHERE name=$1
		)
	`, schema, schema)
	for _, name := range []string{"待发货", "已发货"} {
		if _, err := pool.Exec(ctx, statusQ, name); err != nil {
			return err
		}
	}
	stmts := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.order_shipments (
			id BIGSERIAL PRIMARY KEY,
			shipment_no TEXT NOT NULL UNIQUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			created_by TEXT NOT NULL DEFAULT '',
			sender_id BIGINT,
			file_url TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'excel_generated'
		)`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.order_shipment_orders (
			id BIGSERIAL PRIMARY KEY,
			shipment_id BIGINT NOT NULL REFERENCES %s.order_shipments(id) ON DELETE CASCADE,
			order_id BIGINT NOT NULL REFERENCES %s.orders(id) ON DELETE CASCADE,
			sender_id BIGINT,
			tracking_no TEXT NOT NULL DEFAULT '',
			shipped_at TIMESTAMPTZ,
			UNIQUE(shipment_id, order_id)
		)`, schema, schema, schema),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_order_shipment_orders_order_id ON %s.order_shipment_orders(order_id)`, schema, schema),
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func ensureOrderShippingTrackingTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	stmts := []string{
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS ship_tracking_no TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.order_shipping_trackings (
			id BIGSERIAL PRIMARY KEY,
			order_id BIGINT NOT NULL REFERENCES %s.orders(id) ON DELETE CASCADE,
			tracking_no TEXT NOT NULL,
			source TEXT NOT NULL DEFAULT '',
			created_by TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			UNIQUE(order_id, tracking_no)
		)`, schema, schema),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_order_shipping_trackings_order ON %s.order_shipping_trackings(order_id, id)`, schema, schema),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_order_shipping_trackings_no ON %s.order_shipping_trackings(tracking_no)`, schema, schema),
		fmt.Sprintf(`
			INSERT INTO %s.order_shipping_trackings(order_id, tracking_no, source, created_by)
			SELECT o.id, trim(x.tracking_no), 'legacy_order_field', 'schema_migration'
			FROM %s.orders o
			CROSS JOIN LATERAL regexp_split_to_table(COALESCE(o.ship_tracking_no,''), '[[:space:],;，；、]+') AS x(tracking_no)
			WHERE trim(x.tracking_no) <> ''
			ON CONFLICT (order_id, tracking_no) DO NOTHING
		`, schema, schema),
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func syncSerialIDSequence(ctx context.Context, pool *pgxpool.Pool, schema, table string) error {
	q := fmt.Sprintf(`
DO $$
DECLARE
	seq TEXT;
BEGIN
	SELECT pg_get_serial_sequence('%[1]s.%[2]s', 'id') INTO seq;
	IF seq IS NOT NULL THEN
		PERFORM setval(seq, COALESCE((SELECT MAX(id) FROM %[1]s.%[2]s), 0) + 1, false);
	END IF;
END $$;`, schema, table)
	_, err := pool.Exec(ctx, q)
	return err
}

func ensureOutsourceFeeColumns(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	stmts := []string{
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS outsource_material_fee NUMERIC(12,2) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS outsource_roast_fee NUMERIC(12,2) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS outsource_packaging_fee NUMERIC(12,2) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS outsource_manual_fee NUMERIC(12,2) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS outsource_tax_fee NUMERIC(12,2) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS outsource_other_fee NUMERIC(12,2) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS outsource_total_fee NUMERIC(12,2) NOT NULL DEFAULT 0`, schema),
	}
	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

func ensureOutsourceTemplateTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q1 := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.outsource_templates (
		id BIGSERIAL PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		is_default BOOLEAN NOT NULL DEFAULT false,
		roast_unit_price NUMERIC(12,2) NOT NULL DEFAULT 0,
		bean_pack_unit_price NUMERIC(12,2) NOT NULL DEFAULT 0,
		drip_pack_unit_price NUMERIC(12,2) NOT NULL DEFAULT 0,
		sc_unit_price NUMERIC(12,2) NOT NULL DEFAULT 0,
		active BOOLEAN NOT NULL DEFAULT true,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`, schema)
	if _, err := pool.Exec(ctx, q1); err != nil {
		return err
	}
	q2 := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.outsource_template_tiers (
		id BIGSERIAL PRIMARY KEY,
		template_id BIGINT NOT NULL REFERENCES %s.outsource_templates(id) ON DELETE CASCADE,
		min_qty BIGINT NOT NULL DEFAULT 1,
		max_qty BIGINT,
		multiplier NUMERIC(10,4) NOT NULL DEFAULT 1,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`, schema, schema)
	if _, err := pool.Exec(ctx, q2); err != nil {
		return err
	}
	_, _ = pool.Exec(ctx, fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS idx_%s_outsource_templates_default_true ON %s.outsource_templates ((is_default)) WHERE is_default=true`, schema, schema))
	versions := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %[1]s.outsource_template_versions (
	id BIGSERIAL PRIMARY KEY,
	template_id BIGINT NOT NULL REFERENCES %[1]s.outsource_templates(id) ON DELETE RESTRICT,
	version_no INTEGER NOT NULL,
	status TEXT NOT NULL DEFAULT 'published' CHECK (status IN ('published','inactive')),
	roast_unit_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	bean_pack_unit_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	drip_pack_unit_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	sc_unit_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	created_by TEXT NOT NULL DEFAULT '',
	published_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE(template_id, version_no)
);
CREATE INDEX IF NOT EXISTS outsource_template_versions_current_idx
	ON %[1]s.outsource_template_versions(template_id, status, version_no DESC);
CREATE TABLE IF NOT EXISTS %[1]s.outsource_template_rules (
	id BIGSERIAL PRIMARY KEY,
	version_id BIGINT NOT NULL REFERENCES %[1]s.outsource_template_versions(id) ON DELETE RESTRICT,
	fee_type TEXT NOT NULL CHECK (fee_type IN ('roasting','labor','material','packaging','processing','storage')),
	name TEXT NOT NULL DEFAULT '',
	basis TEXT NOT NULL CHECK (basis IN ('actual_input_kg','actual_output_kg','actual_minutes','actual_units','fixed_per_work_order','factory_material_actual_cost')),
	unit_price NUMERIC(14,4) NOT NULL DEFAULT 0 CHECK (unit_price >= 0),
	sort_order INTEGER NOT NULL DEFAULT 10,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %[1]s.outsource_template_rules DROP CONSTRAINT IF EXISTS outsource_template_rules_fee_type_check;
ALTER TABLE %[1]s.outsource_template_rules ADD CONSTRAINT outsource_template_rules_fee_type_check CHECK (fee_type IN ('roasting','labor','material','packaging','processing','storage'));
CREATE INDEX IF NOT EXISTS outsource_template_rules_version_idx
	ON %[1]s.outsource_template_rules(version_id, sort_order, id);
INSERT INTO %[1]s.outsource_template_versions(
	template_id,version_no,status,roast_unit_price,bean_pack_unit_price,drip_pack_unit_price,sc_unit_price,created_by,published_at,created_at
)
SELECT t.id,1,'published',t.roast_unit_price,t.bean_pack_unit_price,t.drip_pack_unit_price,t.sc_unit_price,'legacy-migration',t.updated_at,t.updated_at
FROM %[1]s.outsource_templates t
WHERE NOT EXISTS (SELECT 1 FROM %[1]s.outsource_template_versions v WHERE v.template_id=t.id);
INSERT INTO %[1]s.outsource_template_rules(version_id,fee_type,name,basis,unit_price,sort_order)
SELECT v.id,x.fee_type,x.name,x.basis,x.unit_price,x.sort_order
FROM %[1]s.outsource_template_versions v
JOIN %[1]s.outsource_templates t ON t.id=v.template_id
CROSS JOIN LATERAL (VALUES
	('processing','烘焙费','actual_input_kg',t.roast_unit_price,10),
	('packaging','咖啡豆包装费','actual_units',t.bean_pack_unit_price,20),
	('packaging','挂耳包装费','actual_units',t.drip_pack_unit_price,30),
	('processing','SC挂靠费','actual_units',t.sc_unit_price,40)
) AS x(fee_type,name,basis,unit_price,sort_order)
WHERE v.version_no=1 AND x.unit_price > 0
  AND NOT EXISTS (SELECT 1 FROM %[1]s.outsource_template_rules r WHERE r.version_id=v.id);
`, schema)
	_, err := pool.Exec(ctx, versions)
	return err
}

func ensureProcessingBillingTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %[1]s.processing_billing_runs (
	id BIGSERIAL PRIMARY KEY,
	customer_id BIGINT NOT NULL DEFAULT 0,
	template_id BIGINT NOT NULL REFERENCES %[1]s.outsource_templates(id) ON DELETE RESTRICT,
	template_version_id BIGINT NOT NULL REFERENCES %[1]s.outsource_template_versions(id) ON DELETE RESTRICT,
	settlement_batch_id BIGINT NOT NULL DEFAULT 0,
	run_kind TEXT NOT NULL DEFAULT 'standard' CHECK (run_kind IN ('standard','adjustment','reversal')),
	source_billing_run_id BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'confirmed' CHECK (status IN ('confirmed','paid','reversed')),
	total_amount NUMERIC(14,2) NOT NULL DEFAULT 0,
	currency TEXT NOT NULL DEFAULT 'CNY',
	created_by TEXT NOT NULL DEFAULT '',
	confirmed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	paid_at TIMESTAMPTZ,
	reversed_at TIMESTAMPTZ,
	lifecycle_reason TEXT NOT NULL DEFAULT '',
	request_key TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %[1]s.processing_billing_runs ADD COLUMN IF NOT EXISTS run_kind TEXT NOT NULL DEFAULT 'standard';
ALTER TABLE %[1]s.processing_billing_runs ADD COLUMN IF NOT EXISTS source_billing_run_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.processing_billing_runs ADD COLUMN IF NOT EXISTS paid_at TIMESTAMPTZ;
ALTER TABLE %[1]s.processing_billing_runs ADD COLUMN IF NOT EXISTS lifecycle_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.processing_billing_runs ADD COLUMN IF NOT EXISTS request_key TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.processing_billing_runs DROP CONSTRAINT IF EXISTS processing_billing_runs_status_check;
ALTER TABLE %[1]s.processing_billing_runs ADD CONSTRAINT processing_billing_runs_status_check CHECK (status IN ('confirmed','paid','reversed'));
ALTER TABLE %[1]s.processing_billing_runs DROP CONSTRAINT IF EXISTS processing_billing_runs_run_kind_check;
ALTER TABLE %[1]s.processing_billing_runs ADD CONSTRAINT processing_billing_runs_run_kind_check CHECK (run_kind IN ('standard','adjustment','reversal'));
CREATE INDEX IF NOT EXISTS processing_billing_runs_customer_idx
	ON %[1]s.processing_billing_runs(customer_id, status, confirmed_at DESC, id DESC);
CREATE UNIQUE INDEX IF NOT EXISTS processing_billing_runs_reversal_source_uq
	ON %[1]s.processing_billing_runs(source_billing_run_id)
	WHERE run_kind='reversal' AND source_billing_run_id>0;
CREATE UNIQUE INDEX IF NOT EXISTS processing_billing_runs_adjustment_request_uq
	ON %[1]s.processing_billing_runs(source_billing_run_id,request_key)
	WHERE run_kind='adjustment' AND source_billing_run_id>0 AND request_key<>'';
CREATE TABLE IF NOT EXISTS %[1]s.processing_billing_work_orders (
	id BIGSERIAL PRIMARY KEY,
	billing_run_id BIGINT NOT NULL REFERENCES %[1]s.processing_billing_runs(id) ON DELETE RESTRICT,
	run_kind TEXT NOT NULL DEFAULT 'standard' CHECK (run_kind IN ('standard','adjustment','reversal')),
	work_order_id BIGINT NOT NULL,
	work_order_no TEXT NOT NULL DEFAULT '',
	request_no TEXT NOT NULL DEFAULT '',
	product_name TEXT NOT NULL DEFAULT '',
	completed_at TIMESTAMPTZ,
	actual_input_kg NUMERIC(16,4) NOT NULL DEFAULT 0,
	actual_output_kg NUMERIC(16,4) NOT NULL DEFAULT 0,
	actual_minutes NUMERIC(16,4) NOT NULL DEFAULT 0,
	actual_units NUMERIC(16,4) NOT NULL DEFAULT 0,
	factory_material_actual_cost NUMERIC(16,4) NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %[1]s.processing_billing_work_orders ADD COLUMN IF NOT EXISTS run_kind TEXT NOT NULL DEFAULT 'standard';
ALTER TABLE %[1]s.processing_billing_work_orders DROP CONSTRAINT IF EXISTS processing_billing_work_orders_work_order_id_key;
ALTER TABLE %[1]s.processing_billing_work_orders DROP CONSTRAINT IF EXISTS processing_billing_work_orders_run_kind_check;
ALTER TABLE %[1]s.processing_billing_work_orders ADD CONSTRAINT processing_billing_work_orders_run_kind_check CHECK (run_kind IN ('standard','adjustment','reversal'));
CREATE UNIQUE INDEX IF NOT EXISTS processing_billing_work_orders_standard_uq
	ON %[1]s.processing_billing_work_orders(work_order_id) WHERE run_kind='standard';
CREATE UNIQUE INDEX IF NOT EXISTS processing_billing_work_orders_run_work_order_uq
	ON %[1]s.processing_billing_work_orders(billing_run_id,work_order_id);
CREATE INDEX IF NOT EXISTS processing_billing_work_orders_run_idx
	ON %[1]s.processing_billing_work_orders(billing_run_id, id);
CREATE TABLE IF NOT EXISTS %[1]s.processing_billing_line_snapshots (
	id BIGSERIAL PRIMARY KEY,
	billing_run_id BIGINT NOT NULL REFERENCES %[1]s.processing_billing_runs(id) ON DELETE RESTRICT,
	work_order_id BIGINT NOT NULL,
	rule_id BIGINT REFERENCES %[1]s.outsource_template_rules(id) ON DELETE RESTRICT,
	line_kind TEXT NOT NULL DEFAULT 'calculated' CHECK (line_kind IN ('calculated','adjustment','reversal')),
	source_line_id BIGINT NOT NULL DEFAULT 0,
	fee_item_id BIGINT NOT NULL DEFAULT 0,
	fee_type TEXT NOT NULL DEFAULT '',
	fee_name TEXT NOT NULL DEFAULT '',
	basis TEXT NOT NULL DEFAULT '',
	base_quantity NUMERIC(16,4) NOT NULL DEFAULT 0,
	unit_price NUMERIC(16,4) NOT NULL DEFAULT 0,
	amount NUMERIC(14,2) NOT NULL DEFAULT 0,
	reason TEXT NOT NULL DEFAULT '',
	calculation_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %[1]s.processing_billing_line_snapshots ALTER COLUMN rule_id DROP NOT NULL;
ALTER TABLE %[1]s.processing_billing_line_snapshots ADD COLUMN IF NOT EXISTS line_kind TEXT NOT NULL DEFAULT 'calculated';
ALTER TABLE %[1]s.processing_billing_line_snapshots ADD COLUMN IF NOT EXISTS source_line_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.processing_billing_line_snapshots ADD COLUMN IF NOT EXISTS reason TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.processing_billing_line_snapshots DROP CONSTRAINT IF EXISTS processing_billing_line_snapshots_work_order_id_rule_id_key;
ALTER TABLE %[1]s.processing_billing_line_snapshots DROP CONSTRAINT IF EXISTS processing_billing_line_snapshots_line_kind_check;
ALTER TABLE %[1]s.processing_billing_line_snapshots ADD CONSTRAINT processing_billing_line_snapshots_line_kind_check CHECK (line_kind IN ('calculated','adjustment','reversal'));
CREATE UNIQUE INDEX IF NOT EXISTS processing_billing_line_snapshots_calculated_uq
	ON %[1]s.processing_billing_line_snapshots(billing_run_id,work_order_id,rule_id)
	WHERE line_kind='calculated';
CREATE INDEX IF NOT EXISTS processing_billing_line_snapshots_run_idx
	ON %[1]s.processing_billing_line_snapshots(billing_run_id, work_order_id, id);
DO $$
BEGIN
	IF to_regclass('%[1]s.customer_fee_items') IS NOT NULL THEN
		ALTER TABLE %[1]s.customer_fee_items ADD COLUMN IF NOT EXISTS processing_billing_run_id BIGINT NOT NULL DEFAULT 0;
		ALTER TABLE %[1]s.customer_fee_items DROP CONSTRAINT IF EXISTS customer_fee_items_fee_type_check;
		ALTER TABLE %[1]s.customer_fee_items ADD CONSTRAINT customer_fee_items_fee_type_check CHECK (fee_type IN ('product','roasting','labor','material','processing','shipping','direct_ship_service','packaging','storage','adjustment'));
		CREATE INDEX IF NOT EXISTS customer_fee_items_processing_billing_run_idx ON %[1]s.customer_fee_items(processing_billing_run_id, id);
	END IF;
	IF to_regclass('%[1]s.customer_settlement_batches') IS NOT NULL THEN
		ALTER TABLE %[1]s.customer_settlement_batches ADD COLUMN IF NOT EXISTS processing_billing_run_id BIGINT NOT NULL DEFAULT 0;
		CREATE INDEX IF NOT EXISTS customer_settlement_batches_processing_billing_run_idx ON %[1]s.customer_settlement_batches(processing_billing_run_id, id);
	END IF;
END $$;
`, schema)
	_, err := pool.Exec(ctx, q)
	return err
}

func ensureLogisticsSettingsTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	stmts := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.logistics_companies (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			sort INTEGER NOT NULL DEFAULT 0,
			active BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.logistics_products (
			id BIGSERIAL PRIMARY KEY,
			company_id BIGINT NOT NULL REFERENCES %s.logistics_companies(id) ON DELETE CASCADE,
			name TEXT NOT NULL DEFAULT '',
			sort INTEGER NOT NULL DEFAULT 0,
			active BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schema, schema),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_logistics_products_company ON %s.logistics_products(company_id)`, schema, schema),
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	var companyID int64
	seedCompany := fmt.Sprintf(`
		INSERT INTO %s.logistics_companies(name, sort, active)
		SELECT '顺丰', 10, true
		WHERE NOT EXISTS (SELECT 1 FROM %s.logistics_companies WHERE name='顺丰')
		RETURNING id
	`, schema, schema)
	if err := pool.QueryRow(ctx, seedCompany).Scan(&companyID); err != nil {
		_ = pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.logistics_companies WHERE name='顺丰' ORDER BY id LIMIT 1`, schema)).Scan(&companyID)
	}
	if companyID > 0 {
		seedProduct := fmt.Sprintf(`
			INSERT INTO %s.logistics_products(company_id, name, sort, active)
			SELECT $1, $2, $3, true
			WHERE NOT EXISTS (
				SELECT 1 FROM %s.logistics_products WHERE company_id=$1 AND name=$2
			)
		`, schema, schema)
		for _, item := range []struct {
			name string
			sort int
		}{
			{name: "顺丰小件", sort: 10},
			{name: "顺丰大件", sort: 20},
		} {
			if _, err := pool.Exec(ctx, seedProduct, companyID, item.name, item.sort); err != nil {
				return err
			}
		}
	}
	return nil
}

func ensureOrderFulfillmentColumns(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	stmts := []string{
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS logistics_company_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS logistics_product_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS payment_goods_amount NUMERIC(12,2) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS payment_shipping_amount NUMERIC(12,2) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS payment_voucher_asset_id BIGINT NOT NULL DEFAULT 0`, schema),
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func ensureSalesOrderTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	stmts := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.sales_order_assets (
			id BIGSERIAL PRIMARY KEY,
			kind TEXT NOT NULL,
			filename TEXT NOT NULL DEFAULT '',
			content_type TEXT NOT NULL DEFAULT '',
			bytes BIGINT NOT NULL DEFAULT 0,
			sha256 TEXT NOT NULL DEFAULT '',
			object_key TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			created_by TEXT NOT NULL DEFAULT ''
		)`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.sales_order_settings (
			id INTEGER PRIMARY KEY DEFAULT 1,
			company_name TEXT NOT NULL DEFAULT '',
			note TEXT NOT NULL DEFAULT '',
			payment_text TEXT NOT NULL DEFAULT '',
			bank_account_name TEXT NOT NULL DEFAULT '',
			bank_name TEXT NOT NULL DEFAULT '',
			bank_account_no TEXT NOT NULL DEFAULT '',
			seal_asset_id BIGINT REFERENCES %s.sales_order_assets(id),
			seal_x_mm NUMERIC(8,2) NOT NULL DEFAULT 32,
			seal_y_mm NUMERIC(8,2) NOT NULL DEFAULT 5,
			seal_width_mm NUMERIC(8,2) NOT NULL DEFAULT 36,
			payment_text_x_mm NUMERIC(8,2) NOT NULL DEFAULT 16,
			payment_text_y_mm NUMERIC(8,2) NOT NULL DEFAULT 118,
			payment_text_width_mm NUMERIC(8,2) NOT NULL DEFAULT 104,
			payment_text_height_mm NUMERIC(8,2) NOT NULL DEFAULT 78,
			payment_text_page_number INTEGER NOT NULL DEFAULT 0,
			payment_code_x_mm NUMERIC(8,2) NOT NULL DEFAULT 126,
			payment_code_y_mm NUMERIC(8,2) NOT NULL DEFAULT 106,
			payment_code_width_mm NUMERIC(8,2) NOT NULL DEFAULT 72,
			payment_code_height_mm NUMERIC(8,2) NOT NULL DEFAULT 122,
			payment_code_page_number INTEGER NOT NULL DEFAULT 0,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_by TEXT NOT NULL DEFAULT '',
			CONSTRAINT sales_order_settings_singleton CHECK (id = 1)
		)`, schema, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.sales_order_payment_codes (
			id BIGSERIAL PRIMARY KEY,
			label TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			asset_id BIGINT NOT NULL REFERENCES %s.sales_order_assets(id),
			sort INTEGER NOT NULL DEFAULT 0,
			active BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			deleted_at TIMESTAMPTZ,
			deleted_by TEXT NOT NULL DEFAULT ''
		)`, schema, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.sales_order_documents (
			id BIGSERIAL PRIMARY KEY,
			order_id BIGINT NOT NULL REFERENCES %s.orders(id),
			order_no TEXT NOT NULL DEFAULT '',
			version_no INTEGER NOT NULL,
			snapshot_json JSONB NOT NULL,
			pdf_asset_id BIGINT REFERENCES %s.sales_order_assets(id),
			is_latest BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			created_by TEXT NOT NULL DEFAULT '',
			UNIQUE(order_id, version_no)
		)`, schema, schema, schema),
		fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS idx_%s_sales_order_latest ON %s.sales_order_documents(order_id) WHERE is_latest`, schema, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.sales_order_images (
			id BIGSERIAL PRIMARY KEY,
			order_id BIGINT NOT NULL REFERENCES %s.orders(id),
			order_no TEXT NOT NULL DEFAULT '',
			version_no INTEGER NOT NULL,
			snapshot_json JSONB NOT NULL,
			image_asset_id BIGINT REFERENCES %s.sales_order_assets(id),
			is_latest BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			created_by TEXT NOT NULL DEFAULT '',
			UNIQUE(order_id, version_no)
		)`, schema, schema, schema),
		fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS idx_%s_sales_order_image_latest ON %s.sales_order_images(order_id) WHERE is_latest`, schema, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.combined_sales_order_documents (
			id BIGSERIAL PRIMARY KEY,
			combination_key TEXT NOT NULL,
			customer_id BIGINT NOT NULL DEFAULT 0,
			order_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
			order_nos TEXT NOT NULL DEFAULT '',
			version_no INTEGER NOT NULL,
			snapshot_json JSONB NOT NULL,
			pdf_asset_id BIGINT REFERENCES %s.sales_order_assets(id),
			is_latest BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			created_by TEXT NOT NULL DEFAULT '',
			UNIQUE(combination_key, version_no)
		)`, schema, schema),
		fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS idx_%s_combined_sales_order_latest ON %s.combined_sales_order_documents(combination_key) WHERE is_latest`, schema, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.combined_sales_order_images (
			id BIGSERIAL PRIMARY KEY,
			combination_key TEXT NOT NULL,
			customer_id BIGINT NOT NULL DEFAULT 0,
			order_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
			order_nos TEXT NOT NULL DEFAULT '',
			version_no INTEGER NOT NULL,
			snapshot_json JSONB NOT NULL,
			image_asset_id BIGINT REFERENCES %s.sales_order_assets(id),
			is_latest BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			created_by TEXT NOT NULL DEFAULT '',
			UNIQUE(combination_key, version_no)
		)`, schema, schema),
		fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS idx_%s_combined_sales_order_image_latest ON %s.combined_sales_order_images(combination_key) WHERE is_latest`, schema, schema),
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	for _, stmt := range []string{
		fmt.Sprintf(`ALTER TABLE %s.sales_order_settings ADD COLUMN IF NOT EXISTS seal_x_mm NUMERIC(8,2) NOT NULL DEFAULT 32`, schema),
		fmt.Sprintf(`ALTER TABLE %s.sales_order_settings ADD COLUMN IF NOT EXISTS seal_y_mm NUMERIC(8,2) NOT NULL DEFAULT 5`, schema),
		fmt.Sprintf(`ALTER TABLE %s.sales_order_settings ADD COLUMN IF NOT EXISTS seal_width_mm NUMERIC(8,2) NOT NULL DEFAULT 36`, schema),
		fmt.Sprintf(`ALTER TABLE %s.sales_order_settings ADD COLUMN IF NOT EXISTS payment_text_x_mm NUMERIC(8,2) NOT NULL DEFAULT 16`, schema),
		fmt.Sprintf(`ALTER TABLE %s.sales_order_settings ADD COLUMN IF NOT EXISTS payment_text_y_mm NUMERIC(8,2) NOT NULL DEFAULT 118`, schema),
		fmt.Sprintf(`ALTER TABLE %s.sales_order_settings ADD COLUMN IF NOT EXISTS payment_text_width_mm NUMERIC(8,2) NOT NULL DEFAULT 104`, schema),
		fmt.Sprintf(`ALTER TABLE %s.sales_order_settings ADD COLUMN IF NOT EXISTS payment_text_height_mm NUMERIC(8,2) NOT NULL DEFAULT 78`, schema),
		fmt.Sprintf(`ALTER TABLE %s.sales_order_settings ADD COLUMN IF NOT EXISTS payment_text_page_number INTEGER NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.sales_order_settings ADD COLUMN IF NOT EXISTS payment_code_x_mm NUMERIC(8,2) NOT NULL DEFAULT 126`, schema),
		fmt.Sprintf(`ALTER TABLE %s.sales_order_settings ADD COLUMN IF NOT EXISTS payment_code_y_mm NUMERIC(8,2) NOT NULL DEFAULT 106`, schema),
		fmt.Sprintf(`ALTER TABLE %s.sales_order_settings ADD COLUMN IF NOT EXISTS payment_code_width_mm NUMERIC(8,2) NOT NULL DEFAULT 72`, schema),
		fmt.Sprintf(`ALTER TABLE %s.sales_order_settings ADD COLUMN IF NOT EXISTS payment_code_height_mm NUMERIC(8,2) NOT NULL DEFAULT 122`, schema),
		fmt.Sprintf(`ALTER TABLE %s.sales_order_settings ADD COLUMN IF NOT EXISTS payment_code_page_number INTEGER NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.sales_order_settings ADD COLUMN IF NOT EXISTS bank_account_name TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.sales_order_settings ADD COLUMN IF NOT EXISTS bank_name TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.sales_order_settings ADD COLUMN IF NOT EXISTS bank_account_no TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.sales_order_payment_codes ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ`, schema),
		fmt.Sprintf(`ALTER TABLE %s.sales_order_payment_codes ADD COLUMN IF NOT EXISTS deleted_by TEXT NOT NULL DEFAULT ''`, schema),
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func ensureOrderItemUnitPricingColumns(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	stmts := []string{
		fmt.Sprintf(`ALTER TABLE %s.order_items ADD COLUMN IF NOT EXISTS product_kind TEXT NOT NULL DEFAULT 'roasted_bean'`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ADD COLUMN IF NOT EXISTS sales_unit TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ADD COLUMN IF NOT EXISTS unit_bag_count INT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ADD COLUMN IF NOT EXISTS unit_bean_g NUMERIC(12,3) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ADD COLUMN IF NOT EXISTS matched_price_qty NUMERIC(14,3) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ADD COLUMN IF NOT EXISTS price_source_json JSONB NOT NULL DEFAULT '{}'::jsonb`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ADD COLUMN IF NOT EXISTS customer_product_alias_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ADD COLUMN IF NOT EXISTS customer_product_reference_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ADD COLUMN IF NOT EXISTS material_source_mode TEXT`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ADD COLUMN IF NOT EXISTS customer_product_display_name_snapshot TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ADD COLUMN IF NOT EXISTS customer_item_code_snapshot TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ADD COLUMN IF NOT EXISTS brand_name_snapshot TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ADD COLUMN IF NOT EXISTS product_code_snapshot TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ADD COLUMN IF NOT EXISTS product_name_snapshot TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`UPDATE %s.order_items SET product_kind='roasted_bean' WHERE COALESCE(product_kind,'')=''`, schema),
		fmt.Sprintf(`UPDATE %s.order_items SET sales_unit='' WHERE sales_unit IS NULL`, schema),
		fmt.Sprintf(`UPDATE %s.order_items SET unit_bag_count=0 WHERE unit_bag_count IS NULL`, schema),
		fmt.Sprintf(`UPDATE %s.order_items SET unit_bean_g=0 WHERE unit_bean_g IS NULL`, schema),
		fmt.Sprintf(`UPDATE %s.order_items SET matched_price_qty=0 WHERE matched_price_qty IS NULL`, schema),
		fmt.Sprintf(`UPDATE %s.order_items SET price_source_json='{}'::jsonb WHERE price_source_json IS NULL`, schema),
		fmt.Sprintf(`UPDATE %s.order_items SET customer_product_alias_id=0 WHERE customer_product_alias_id IS NULL`, schema),
		fmt.Sprintf(`UPDATE %s.order_items SET customer_product_reference_id=0 WHERE customer_product_reference_id IS NULL`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ALTER COLUMN material_source_mode DROP NOT NULL`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ALTER COLUMN material_source_mode DROP DEFAULT`, schema),
		fmt.Sprintf(`UPDATE %s.order_items SET customer_product_display_name_snapshot='' WHERE customer_product_display_name_snapshot IS NULL`, schema),
		fmt.Sprintf(`UPDATE %s.order_items SET customer_item_code_snapshot='' WHERE customer_item_code_snapshot IS NULL`, schema),
		fmt.Sprintf(`UPDATE %s.order_items SET brand_name_snapshot='' WHERE brand_name_snapshot IS NULL`, schema),
		fmt.Sprintf(`UPDATE %s.order_items SET product_code_snapshot='' WHERE product_code_snapshot IS NULL`, schema),
		fmt.Sprintf(`UPDATE %s.order_items SET product_name_snapshot='' WHERE product_name_snapshot IS NULL`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ALTER COLUMN product_kind SET DEFAULT 'roasted_bean'`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ALTER COLUMN product_kind SET NOT NULL`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ALTER COLUMN sales_unit SET DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ALTER COLUMN sales_unit SET NOT NULL`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ALTER COLUMN unit_bag_count SET DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ALTER COLUMN unit_bag_count SET NOT NULL`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ALTER COLUMN unit_bean_g SET DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ALTER COLUMN unit_bean_g SET NOT NULL`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ALTER COLUMN matched_price_qty SET DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ALTER COLUMN matched_price_qty SET NOT NULL`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ALTER COLUMN price_source_json SET DEFAULT '{}'::jsonb`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ALTER COLUMN price_source_json SET NOT NULL`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ALTER COLUMN customer_product_alias_id SET DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ALTER COLUMN customer_product_alias_id SET NOT NULL`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ALTER COLUMN customer_product_display_name_snapshot SET DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ALTER COLUMN customer_product_display_name_snapshot SET NOT NULL`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ALTER COLUMN customer_item_code_snapshot SET DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ALTER COLUMN customer_item_code_snapshot SET NOT NULL`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ALTER COLUMN brand_name_snapshot SET DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ALTER COLUMN brand_name_snapshot SET NOT NULL`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ALTER COLUMN product_code_snapshot SET DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ALTER COLUMN product_code_snapshot SET NOT NULL`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ALTER COLUMN product_name_snapshot SET DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ALTER COLUMN product_name_snapshot SET NOT NULL`, schema),
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func ensureOrderInvoiceTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	stmts := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.order_invoices (
			order_id BIGINT PRIMARY KEY REFERENCES %s.orders(id) ON DELETE CASCADE,
			order_no TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'requested',
			requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			requested_by TEXT NOT NULL DEFAULT '',
			invoice_asset_id BIGINT REFERENCES %s.sales_order_assets(id),
			uploaded_at TIMESTAMPTZ,
			uploaded_by TEXT NOT NULL DEFAULT '',
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_by TEXT NOT NULL DEFAULT ''
		)`, schema, schema, schema),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_order_invoices_status ON %s.order_invoices(status)`, schema, schema),
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func ensureDeliveryNoteTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	stmts := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.delivery_note_assets (
			id BIGSERIAL PRIMARY KEY,
			kind TEXT NOT NULL,
			filename TEXT NOT NULL DEFAULT '',
			content_type TEXT NOT NULL DEFAULT '',
			bytes BIGINT NOT NULL DEFAULT 0,
			sha256 TEXT NOT NULL DEFAULT '',
			object_key TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			created_by TEXT NOT NULL DEFAULT ''
		)`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.delivery_note_forms (
			order_id BIGINT PRIMARY KEY REFERENCES %s.orders(id) ON DELETE CASCADE,
			posting_date DATE,
			source_warehouse TEXT NOT NULL DEFAULT 'finished_goods',
			delivery_method TEXT NOT NULL DEFAULT '',
			tracking_no TEXT NOT NULL DEFAULT '',
			note TEXT NOT NULL DEFAULT '',
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_by TEXT NOT NULL DEFAULT ''
		)`, schema, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.delivery_note_documents (
			id BIGSERIAL PRIMARY KEY,
			order_id BIGINT NOT NULL REFERENCES %s.orders(id),
			order_no TEXT NOT NULL DEFAULT '',
			version_no INTEGER NOT NULL,
			snapshot_json JSONB NOT NULL,
			pdf_asset_id BIGINT REFERENCES %s.delivery_note_assets(id),
			image_asset_id BIGINT REFERENCES %s.delivery_note_assets(id),
			is_latest BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			created_by TEXT NOT NULL DEFAULT '',
			UNIQUE(order_id, version_no)
		)`, schema, schema, schema, schema),
		fmt.Sprintf(`ALTER TABLE %s.delivery_note_documents ADD COLUMN IF NOT EXISTS image_asset_id BIGINT REFERENCES %s.delivery_note_assets(id)`, schema, schema),
		fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS idx_%s_delivery_note_latest ON %s.delivery_note_documents(order_id) WHERE is_latest`, schema, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.combined_delivery_note_documents (
			id BIGSERIAL PRIMARY KEY,
			combination_key TEXT NOT NULL,
			customer_id BIGINT NOT NULL DEFAULT 0,
			order_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
			order_nos TEXT NOT NULL DEFAULT '',
			version_no INTEGER NOT NULL,
			snapshot_json JSONB NOT NULL,
			pdf_asset_id BIGINT REFERENCES %s.delivery_note_assets(id),
			is_latest BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			created_by TEXT NOT NULL DEFAULT '',
			UNIQUE(combination_key, version_no)
		)`, schema, schema),
		fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS idx_%s_combined_delivery_note_latest ON %s.combined_delivery_note_documents(combination_key) WHERE is_latest`, schema, schema),
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func ensureExternalShareResourceTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	stmts := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.external_share_resources (
			token TEXT PRIMARY KEY,
			resource_type TEXT NOT NULL,
			order_id BIGINT NOT NULL REFERENCES %s.orders(id) ON DELETE CASCADE,
			resource_id BIGINT NOT NULL,
			title TEXT NOT NULL DEFAULT '',
			filename TEXT NOT NULL DEFAULT '',
			content_type TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			created_by TEXT NOT NULL DEFAULT '',
			last_accessed_at TIMESTAMPTZ
		)`, schema, schema),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_external_share_order ON %s.external_share_resources(order_id, resource_type)`, schema, schema),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_external_share_resource ON %s.external_share_resources(resource_type, resource_id)`, schema, schema),
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}
