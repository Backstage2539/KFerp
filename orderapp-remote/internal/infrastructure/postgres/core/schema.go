package core

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %[1]s.sources (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS %[1]s.order_types (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS %[1]s.customer_type_options (
	value TEXT PRIMARY KEY,
	label TEXT NOT NULL DEFAULT '',
	active BOOLEAN NOT NULL DEFAULT true,
	sort_order INTEGER NOT NULL DEFAULT 100,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_by TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS %[1]s.pay_statuses (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS %[1]s.ship_statuses (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS %[1]s.order_process_statuses (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	sort INTEGER NOT NULL DEFAULT 0,
	active BOOLEAN NOT NULL DEFAULT true
);
CREATE TABLE IF NOT EXISTS %[1]s.customers (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL DEFAULT '',
	raw_name TEXT NOT NULL DEFAULT '',
	customer_type TEXT NOT NULL DEFAULT 'retail',
	company_name TEXT NOT NULL DEFAULT '',
	company_address TEXT NOT NULL DEFAULT '',
	company_phone TEXT NOT NULL DEFAULT '',
	contact TEXT NOT NULL DEFAULT '',
	phone TEXT NOT NULL DEFAULT '',
	address TEXT NOT NULL DEFAULT '',
	active BOOLEAN NOT NULL DEFAULT true,
	default_source_id BIGINT,
	default_order_type_id BIGINT,
	customer_product_rule_template_id BIGINT NOT NULL DEFAULT 0,
	responsible_employee_id BIGINT NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS %[1]s.products (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL DEFAULT '',
	roast_level TEXT NOT NULL DEFAULT '',
	special_attrs_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	default_price NUMERIC NOT NULL DEFAULT 0,
	product_kind TEXT NOT NULL DEFAULT 'roasted',
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	retail_price_100g NUMERIC NOT NULL DEFAULT 0,
	retail_price_200g NUMERIC NOT NULL DEFAULT 0,
	retail_price_227g NUMERIC NOT NULL DEFAULT 0,
	retail_price_250g NUMERIC NOT NULL DEFAULT 0,
	customer_id BIGINT NOT NULL DEFAULT 0,
	base_product_id BIGINT NOT NULL DEFAULT 0,
	visibility TEXT NOT NULL DEFAULT 'public',
	custom_type TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS %[1]s.product_price_tiers (
	id BIGSERIAL PRIMARY KEY,
	product_id BIGINT REFERENCES %[1]s.products(id) ON DELETE CASCADE,
	spec_g BIGINT NOT NULL DEFAULT 454,
	min_qty_units NUMERIC NOT NULL DEFAULT 0,
	max_qty_units NUMERIC,
	price_per_unit NUMERIC(12,2) NOT NULL DEFAULT 0,
	min_qty_lb NUMERIC,
	max_qty_lb NUMERIC,
	price_per_lb NUMERIC(12,2),
	active BOOLEAN NOT NULL DEFAULT true
);
CREATE TABLE IF NOT EXISTS %[1]s.orders (
	id BIGSERIAL PRIMARY KEY,
	order_no TEXT NOT NULL DEFAULT '',
	document_date DATE,
	order_date DATE,
	customer_id BIGINT,
	source_id BIGINT,
	order_type_id BIGINT,
	pay_status_id BIGINT,
	payment_method TEXT NOT NULL DEFAULT '',
	ship_status_id BIGINT,
	ship_method TEXT NOT NULL DEFAULT '',
	ship_tracking_no TEXT NOT NULL DEFAULT '',
	receiver_name TEXT NOT NULL DEFAULT '',
	receiver_phone TEXT NOT NULL DEFAULT '',
	receiver_address TEXT NOT NULL DEFAULT '',
	receiver_company TEXT NOT NULL DEFAULT '',
	portal_service_code TEXT NOT NULL DEFAULT '',
	source_warehouse TEXT NOT NULL DEFAULT '',
	sender_id BIGINT NOT NULL DEFAULT 0,
	notes TEXT NOT NULL DEFAULT '',
	total_amount NUMERIC NOT NULL DEFAULT 0,
	shipping_amount NUMERIC NOT NULL DEFAULT 0,
	discount_amount NUMERIC NOT NULL DEFAULT 0,
	round_to_int BOOLEAN NOT NULL DEFAULT false,
	rounding_amount NUMERIC NOT NULL DEFAULT 0,
	grand_total NUMERIC NOT NULL DEFAULT 0,
	express_fee TEXT NOT NULL DEFAULT '',
	outsource_material_fee NUMERIC(12,2) NOT NULL DEFAULT 0,
	outsource_roast_fee NUMERIC(12,2) NOT NULL DEFAULT 0,
	outsource_packaging_fee NUMERIC(12,2) NOT NULL DEFAULT 0,
	outsource_manual_fee NUMERIC(12,2) NOT NULL DEFAULT 0,
	outsource_tax_fee NUMERIC(12,2) NOT NULL DEFAULT 0,
	outsource_other_fee NUMERIC(12,2) NOT NULL DEFAULT 0,
	outsource_total_fee NUMERIC(12,2) NOT NULL DEFAULT 0,
	responsible_party_type TEXT NOT NULL DEFAULT '',
	responsible_party_id BIGINT NOT NULL DEFAULT 0,
	responsible_party_name TEXT NOT NULL DEFAULT '',
	is_void BOOLEAN NOT NULL DEFAULT false,
	voided_at TIMESTAMPTZ,
	void_reason TEXT NOT NULL DEFAULT '',
	process_status_id BIGINT,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS %[1]s.order_items (
	id BIGSERIAL PRIMARY KEY,
	order_id BIGINT REFERENCES %[1]s.orders(id) ON DELETE CASCADE,
	line_no INTEGER NOT NULL DEFAULT 0,
	product_id BIGINT,
	customer_product_reference_id BIGINT NOT NULL DEFAULT 0,
	material_source_mode TEXT,
	price_tier_id BIGINT,
	product_kind TEXT NOT NULL DEFAULT 'roasted',
	price_overridden BOOLEAN NOT NULL DEFAULT false,
	bean_list_publication_id BIGINT,
	bean_list_version_no TEXT NOT NULL DEFAULT '',
	item_name TEXT NOT NULL DEFAULT '',
	item_note TEXT NOT NULL DEFAULT '',
	qty NUMERIC NOT NULL DEFAULT 0,
	unit TEXT NOT NULL DEFAULT '',
		spec TEXT NOT NULL DEFAULT '',
		unit_price NUMERIC NOT NULL DEFAULT 0,
		line_total_before_discount NUMERIC NOT NULL DEFAULT 0,
		discount_type TEXT NOT NULL DEFAULT '',
		discount_value NUMERIC NOT NULL DEFAULT 0,
		discount_amount NUMERIC NOT NULL DEFAULT 0,
		line_total NUMERIC NOT NULL DEFAULT 0
	);
CREATE TABLE IF NOT EXISTS %[1]s.customer_order_production_demands (
	id BIGSERIAL PRIMARY KEY,
	order_id BIGINT NOT NULL REFERENCES %[1]s.orders(id) ON DELETE CASCADE,
	line_no INTEGER NOT NULL,
	customer_id BIGINT NOT NULL DEFAULT 0,
	product_id BIGINT NOT NULL DEFAULT 0,
	bom_spec_id BIGINT NOT NULL DEFAULT 0,
	bom_variant_id BIGINT NOT NULL DEFAULT 0,
	target_qty NUMERIC NOT NULL DEFAULT 0,
	material_source_mode TEXT NOT NULL DEFAULT 'customer',
	status TEXT NOT NULL DEFAULT 'planned',
	migrated_at TIMESTAMPTZ,
	migration_note TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE(order_id, line_no)
	);
CREATE INDEX IF NOT EXISTS customer_order_production_demands_status_idx
	ON %[1]s.customer_order_production_demands(status, customer_id, product_id);
`, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	if err := ensureCoreColumns(ctx, pool, schema); err != nil {
		return err
	}
	if err := syncSerialIDSequence(ctx, pool, schema, "products"); err != nil {
		return err
	}
	return seedCoreOptions(ctx, pool, schema)
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

func ensureCoreColumns(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	stmts := []string{
		`ALTER TABLE %[1]s.customers ADD COLUMN IF NOT EXISTS raw_name TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.customers ADD COLUMN IF NOT EXISTS customer_type TEXT NOT NULL DEFAULT 'retail'`,
		`UPDATE %[1]s.customers SET customer_type='retail' WHERE COALESCE(customer_type,'')=''`,
		`ALTER TABLE %[1]s.customers ADD COLUMN IF NOT EXISTS company_name TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.customers ADD COLUMN IF NOT EXISTS company_address TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.customers ADD COLUMN IF NOT EXISTS company_phone TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.customers ADD COLUMN IF NOT EXISTS default_source_id BIGINT`,
		`ALTER TABLE %[1]s.customers ADD COLUMN IF NOT EXISTS default_order_type_id BIGINT`,
		`ALTER TABLE %[1]s.customers ADD COLUMN IF NOT EXISTS customer_product_rule_template_id BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE %[1]s.customers ADD COLUMN IF NOT EXISTS responsible_employee_id BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE %[1]s.customers ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now()`,
		`ALTER TABLE %[1]s.customers ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`,
		`ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS roast_level TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS special_attrs_json JSONB NOT NULL DEFAULT '{}'::jsonb`,
		`ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS default_price NUMERIC NOT NULL DEFAULT 0`,
		`ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS product_kind TEXT NOT NULL DEFAULT 'roasted'`,
		`UPDATE %[1]s.products SET product_kind='roasted' WHERE COALESCE(product_kind,'')=''`,
		`CREATE INDEX IF NOT EXISTS products_product_kind_active_idx ON %[1]s.products(product_kind, active)`,
		`ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS active BOOLEAN NOT NULL DEFAULT true`,
		`ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now()`,
		`ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS customer_id BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS base_product_id BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS visibility TEXT NOT NULL DEFAULT 'public'`,
		`ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS custom_type TEXT NOT NULL DEFAULT ''`,
		`UPDATE %[1]s.products SET visibility='public' WHERE COALESCE(visibility,'')=''`,
		`CREATE INDEX IF NOT EXISTS products_customer_visibility_idx ON %[1]s.products(customer_id, visibility, active)`,
		`CREATE INDEX IF NOT EXISTS products_base_product_idx ON %[1]s.products(base_product_id)`,
		`ALTER TABLE %[1]s.product_price_tiers ADD COLUMN IF NOT EXISTS active BOOLEAN NOT NULL DEFAULT true`,
		`ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS order_no TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS document_date DATE`,
		`UPDATE %[1]s.orders SET document_date=order_date WHERE document_date IS NULL`,
		`ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS source_id BIGINT`,
		`ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS payment_method TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS ship_method TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS ship_tracking_no TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS receiver_name TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS receiver_phone TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS receiver_address TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS receiver_company TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS portal_service_code TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS source_warehouse TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS sender_id BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS notes TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS express_fee TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS responsible_party_type TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS responsible_party_id BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS responsible_party_name TEXT NOT NULL DEFAULT ''`,
		`CREATE INDEX IF NOT EXISTS orders_responsible_party_idx ON %[1]s.orders(responsible_party_type, responsible_party_id)`,
		`ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS is_void BOOLEAN NOT NULL DEFAULT false`,
		`ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS process_status_id BIGINT`,
		`ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now()`,
		`ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS price_tier_id BIGINT`,
		`ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS customer_product_reference_id BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS material_source_mode TEXT`,
		`UPDATE %[1]s.order_items SET customer_product_reference_id=0 WHERE customer_product_reference_id IS NULL`,
		`ALTER TABLE %[1]s.order_items ALTER COLUMN material_source_mode DROP NOT NULL`,
		`ALTER TABLE %[1]s.order_items ALTER COLUMN material_source_mode DROP DEFAULT`,
		`CREATE TABLE IF NOT EXISTS %[1]s.customer_order_production_demands (id BIGSERIAL PRIMARY KEY, order_id BIGINT NOT NULL REFERENCES %[1]s.orders(id) ON DELETE CASCADE, line_no INTEGER NOT NULL, customer_id BIGINT NOT NULL DEFAULT 0, product_id BIGINT NOT NULL DEFAULT 0, bom_spec_id BIGINT NOT NULL DEFAULT 0, bom_variant_id BIGINT NOT NULL DEFAULT 0, target_qty NUMERIC NOT NULL DEFAULT 0, material_source_mode TEXT NOT NULL DEFAULT 'customer', status TEXT NOT NULL DEFAULT 'planned', migrated_at TIMESTAMPTZ, migration_note TEXT NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(), UNIQUE(order_id,line_no))`,
		`ALTER TABLE %[1]s.customer_order_production_demands ADD COLUMN IF NOT EXISTS migrated_at TIMESTAMPTZ`,
		`ALTER TABLE %[1]s.customer_order_production_demands ADD COLUMN IF NOT EXISTS migration_note TEXT NOT NULL DEFAULT ''`,
		`CREATE INDEX IF NOT EXISTS customer_order_production_demands_status_idx ON %[1]s.customer_order_production_demands(status, customer_id, product_id)`,
		`ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS product_kind TEXT NOT NULL DEFAULT 'roasted'`,
		`UPDATE %[1]s.order_items oi SET product_kind=COALESCE(NULLIF(p.product_kind,''),'roasted') FROM %[1]s.products p WHERE oi.product_id=p.id AND COALESCE(oi.product_kind,'')=''`,
		`ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS price_overridden BOOLEAN NOT NULL DEFAULT false`,
		`ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS bean_list_publication_id BIGINT`,
		`ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS bean_list_version_no TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS item_note TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS line_total_before_discount NUMERIC NOT NULL DEFAULT 0`,
		`ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS discount_type TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS discount_value NUMERIC NOT NULL DEFAULT 0`,
		`ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS discount_amount NUMERIC NOT NULL DEFAULT 0`,
		`ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS sales_unit TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS unit_bag_count INT NOT NULL DEFAULT 0`,
		`ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS unit_bean_g NUMERIC(12,3) NOT NULL DEFAULT 0`,
		`ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS matched_price_qty NUMERIC(14,3) NOT NULL DEFAULT 0`,
		`ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS price_source_json JSONB NOT NULL DEFAULT '{}'::jsonb`,
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, fmt.Sprintf(stmt, schema)); err != nil {
			return err
		}
	}
	return nil
}

func seedCoreOptions(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	stmts := []string{
		`INSERT INTO %[1]s.sources(name) SELECT '小程序' WHERE NOT EXISTS (SELECT 1 FROM %[1]s.sources WHERE name='小程序')`,
		`INSERT INTO %[1]s.order_types(name) SELECT '批发订单' WHERE NOT EXISTS (SELECT 1 FROM %[1]s.order_types WHERE name='批发订单')`,
		`INSERT INTO %[1]s.order_types(name) SELECT '零售订单' WHERE NOT EXISTS (SELECT 1 FROM %[1]s.order_types WHERE name='零售订单')`,
		`INSERT INTO %[1]s.customer_type_options(value,label,sort_order) VALUES ('retail','零售客户',10),('ecommerce','电商客户',20),('wholesale','批发客户',30),('channel','渠道客户',40) ON CONFLICT(value) DO UPDATE SET label=excluded.label, active=true, sort_order=excluded.sort_order`,
		`INSERT INTO %[1]s.pay_statuses(name) SELECT '未付款' WHERE NOT EXISTS (SELECT 1 FROM %[1]s.pay_statuses WHERE name='未付款')`,
		`INSERT INTO %[1]s.pay_statuses(name) SELECT '已付款' WHERE NOT EXISTS (SELECT 1 FROM %[1]s.pay_statuses WHERE name='已付款')`,
		`INSERT INTO %[1]s.ship_statuses(name) SELECT '未发货' WHERE NOT EXISTS (SELECT 1 FROM %[1]s.ship_statuses WHERE name='未发货')`,
		`INSERT INTO %[1]s.order_process_statuses(name, sort, active) SELECT '待处理', 10, true WHERE NOT EXISTS (SELECT 1 FROM %[1]s.order_process_statuses WHERE name='待处理')`,
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, fmt.Sprintf(stmt, schema)); err != nil {
			return err
		}
	}
	return nil
}
