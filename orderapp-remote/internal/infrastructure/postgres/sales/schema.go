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
	if err := ensureCustomerCompanyColumns(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureOrderResponsibleColumns(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureOrderPaymentMethodColumn(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureOrderBeanListColumns(ctx, pool, schema); err != nil {
		return err
	}
	if err := backfillERPOrdersForFulfillmentCustomers(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureSalesOrderTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureOrderInvoiceTables(ctx, pool, schema); err != nil {
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

func ensureCustomerCompanyColumns(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	stmts := []string{
		fmt.Sprintf(`ALTER TABLE %s.customers ADD COLUMN IF NOT EXISTS company_name TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.customers ADD COLUMN IF NOT EXISTS company_address TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.customers ADD COLUMN IF NOT EXISTS company_phone TEXT NOT NULL DEFAULT ''`, schema),
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
			LEFT JOIN %[1]s.customer_portal_profiles p ON p.customer_id=b.customer_id
			WHERE b.customer_id=o.customer_id
			  AND b.status='active'
			  AND e.active=true
			  AND e.account_type='channel_customer'
			  AND COALESCE(lp.login_disabled,false)=false
			  AND (
				COALESCE(NULLIF(p.capability_template_key,''),'processing_fulfillment') IN ('processing_fulfillment','public_sku_direct_ship')
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
			spec_g BIGINT NOT NULL DEFAULT 0,
			need_g BIGINT NOT NULL DEFAULT 0,
			batch_id BIGINT NOT NULL DEFAULT 0,
			batch_code TEXT NOT NULL DEFAULT '',
			allocated_g BIGINT NOT NULL DEFAULT 0,
			operator TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schema, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.order_stock_deductions (
			id BIGSERIAL PRIMARY KEY,
			order_id BIGINT NOT NULL REFERENCES %s.orders(id) ON DELETE CASCADE,
			product_id BIGINT NOT NULL DEFAULT 0,
			spec_g BIGINT NOT NULL DEFAULT 0,
			batch_id BIGINT NOT NULL DEFAULT 0,
			batch_code TEXT NOT NULL DEFAULT '',
			deducted_g BIGINT NOT NULL DEFAULT 0,
			source_doc_type TEXT NOT NULL DEFAULT '',
			source_doc_id BIGINT NOT NULL DEFAULT 0,
			operator TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			UNIQUE(order_id, product_id, spec_g, batch_code)
		)`, schema, schema),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_order_stock_alloc_order ON %s.order_stock_batch_allocations(order_id)`, schema, schema),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_order_stock_alloc_batch ON %s.order_stock_batch_allocations(batch_code)`, schema, schema),
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
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
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
		fmt.Sprintf(`ALTER TABLE %s.sales_order_settings ADD COLUMN IF NOT EXISTS bank_account_name TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.sales_order_settings ADD COLUMN IF NOT EXISTS bank_name TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.sales_order_settings ADD COLUMN IF NOT EXISTS bank_account_no TEXT NOT NULL DEFAULT ''`, schema),
	} {
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
			is_latest BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			created_by TEXT NOT NULL DEFAULT '',
			UNIQUE(order_id, version_no)
		)`, schema, schema, schema),
		fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS idx_%s_delivery_note_latest ON %s.delivery_note_documents(order_id) WHERE is_latest`, schema, schema),
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
