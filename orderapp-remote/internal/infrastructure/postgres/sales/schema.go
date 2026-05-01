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
	return ensureSalesOrderTables(ctx, pool, schema)
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

func ensureOrderProcessStatuses(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
		INSERT INTO %s.order_process_statuses(name, sort, active)
		SELECT $1, $2, true
		WHERE NOT EXISTS (
			SELECT 1 FROM %s.order_process_statuses WHERE name=$1
		)
	`, schema, schema)
	_, err := pool.Exec(ctx, q, "生产完成", 35)
	return err
}

func ensureShippingClosureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
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
			seal_y_mm NUMERIC(8,2) NOT NULL DEFAULT 22,
			seal_width_mm NUMERIC(8,2) NOT NULL DEFAULT 42,
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
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	for _, stmt := range []string{
		fmt.Sprintf(`ALTER TABLE %s.sales_order_settings ADD COLUMN IF NOT EXISTS seal_x_mm NUMERIC(8,2) NOT NULL DEFAULT 32`, schema),
		fmt.Sprintf(`ALTER TABLE %s.sales_order_settings ADD COLUMN IF NOT EXISTS seal_y_mm NUMERIC(8,2) NOT NULL DEFAULT 22`, schema),
		fmt.Sprintf(`ALTER TABLE %s.sales_order_settings ADD COLUMN IF NOT EXISTS seal_width_mm NUMERIC(8,2) NOT NULL DEFAULT 42`, schema),
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
