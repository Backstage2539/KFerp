package orderliststaging

const stagingSchemaSQL = `
CREATE SCHEMA IF NOT EXISTS raw;
CREATE SCHEMA IF NOT EXISTS reference;
CREATE SCHEMA IF NOT EXISTS curated;
CREATE SCHEMA IF NOT EXISTS review;

CREATE TABLE IF NOT EXISTS raw.import_runs (
    run_id TEXT PRIMARY KEY,
    source_path TEXT NOT NULL,
    source_sha256 TEXT NOT NULL,
    source_bytes BIGINT NOT NULL,
    start_period TEXT NOT NULL,
    end_period TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    workbook_sheet_count INT NOT NULL,
    included_sheet_count INT NOT NULL,
    raw_order_count INT NOT NULL,
    raw_product_lines INT NOT NULL,
    loaded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source_sha256, start_period, end_period)
);

CREATE TABLE IF NOT EXISTS raw.sheet_inventory (
    run_id TEXT NOT NULL REFERENCES raw.import_runs(run_id) ON DELETE CASCADE,
    sheet_name TEXT NOT NULL,
    period TEXT NOT NULL DEFAULT '',
    included BOOLEAN NOT NULL,
    excluded_reason TEXT NOT NULL DEFAULT '',
    used_row_count INT NOT NULL,
    order_row_count INT NOT NULL,
    PRIMARY KEY (run_id, sheet_name)
);

CREATE TABLE IF NOT EXISTS raw.order_rows (
    run_id TEXT NOT NULL REFERENCES raw.import_runs(run_id) ON DELETE CASCADE,
    source_sheet_name TEXT NOT NULL,
    source_row_number INT NOT NULL,
    source_sequence_original TEXT NOT NULL DEFAULT '',
    duplicate_suffix INT NOT NULL DEFAULT 0 CHECK (duplicate_suffix BETWEEN 0 AND 9),
    source_sequence_effective TEXT NOT NULL DEFAULT '',
    source_order_key TEXT NOT NULL DEFAULT '',
    source_fingerprint TEXT NOT NULL,
    order_date_raw TEXT NOT NULL DEFAULT '',
    customer_raw TEXT NOT NULL DEFAULT '',
    product_raw TEXT NOT NULL DEFAULT '',
    raw_fields JSONB NOT NULL DEFAULT '{}'::jsonb,
    review_status TEXT NOT NULL,
    PRIMARY KEY (run_id, source_sheet_name, source_row_number),
    UNIQUE (run_id, source_fingerprint)
);

CREATE TABLE IF NOT EXISTS raw.order_revisions (
    id BIGSERIAL PRIMARY KEY,
    source_order_key TEXT NOT NULL,
    old_fingerprint TEXT NOT NULL,
    new_fingerprint TEXT NOT NULL,
    old_snapshot JSONB NOT NULL,
    new_snapshot JSONB NOT NULL,
    detected_run_id TEXT NOT NULL REFERENCES raw.import_runs(run_id),
    detected_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source_order_key, old_fingerprint, new_fingerprint)
);

CREATE TABLE IF NOT EXISTS reference.erp_customers (
    environment TEXT NOT NULL DEFAULT 'development',
    erp_customer_id BIGINT NOT NULL,
    name TEXT NOT NULL,
    phone TEXT NOT NULL DEFAULT '',
    active BOOLEAN NOT NULL,
    snapshot_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (environment, erp_customer_id)
);

CREATE TABLE IF NOT EXISTS reference.erp_products (
    environment TEXT NOT NULL DEFAULT 'development',
    erp_product_id BIGINT NOT NULL,
    name TEXT NOT NULL,
    product_kind TEXT NOT NULL DEFAULT '',
    active BOOLEAN NOT NULL,
    snapshot_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (environment, erp_product_id)
);

CREATE TABLE IF NOT EXISTS curated.customers (
    customer_key TEXT PRIMARY KEY,
    canonical_name TEXT NOT NULL,
    normalized_phone TEXT NOT NULL DEFAULT '',
    current_contact TEXT NOT NULL DEFAULT '',
    current_address TEXT NOT NULL DEFAULT '',
    erp_match_id BIGINT NOT NULL DEFAULT 0,
    erp_match_name TEXT NOT NULL DEFAULT '',
    match_method TEXT NOT NULL DEFAULT '',
    review_status TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS curated_customers_phone_uq
ON curated.customers(normalized_phone) WHERE normalized_phone <> '';

CREATE TABLE IF NOT EXISTS curated.customer_aliases (
    id BIGSERIAL PRIMARY KEY,
    customer_key TEXT NOT NULL REFERENCES curated.customers(customer_key) ON DELETE CASCADE,
    alias TEXT NOT NULL,
    alias_normalized TEXT NOT NULL,
    source_order_key TEXT NOT NULL DEFAULT '',
    observed_date TEXT NOT NULL DEFAULT '',
    UNIQUE (customer_key, alias_normalized, source_order_key)
);

CREATE TABLE IF NOT EXISTS curated.customer_phones (
    id BIGSERIAL PRIMARY KEY,
    customer_key TEXT NOT NULL REFERENCES curated.customers(customer_key) ON DELETE CASCADE,
    phone_raw TEXT NOT NULL,
    phone_normalized TEXT NOT NULL,
    is_primary BOOLEAN NOT NULL DEFAULT false,
    source_order_key TEXT NOT NULL DEFAULT '',
    UNIQUE (customer_key, phone_normalized)
);

CREATE TABLE IF NOT EXISTS curated.products (
    product_key TEXT PRIMARY KEY,
    canonical_name TEXT NOT NULL,
    product_kind TEXT NOT NULL,
    roast_level TEXT NOT NULL DEFAULT '',
    erp_match_id BIGINT NOT NULL DEFAULT 0,
    erp_match_name TEXT NOT NULL DEFAULT '',
    match_method TEXT NOT NULL DEFAULT '',
    match_score NUMERIC(8,6) NOT NULL DEFAULT 0,
    review_status TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS curated.skus (
    sku_key TEXT PRIMARY KEY,
    product_key TEXT NOT NULL REFERENCES curated.products(product_key) ON DELETE CASCADE,
    spec_name TEXT NOT NULL,
    sales_unit TEXT NOT NULL DEFAULT '',
    net_content_qty NUMERIC(18,6) NOT NULL DEFAULT 0,
    net_content_unit TEXT NOT NULL DEFAULT '',
    normalized_weight_g NUMERIC(18,6) NOT NULL DEFAULT 0,
    review_status TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS curated.product_aliases (
    id BIGSERIAL PRIMARY KEY,
    product_key TEXT NOT NULL REFERENCES curated.products(product_key) ON DELETE CASCADE,
    sku_key TEXT NOT NULL DEFAULT '',
    raw_line TEXT NOT NULL,
    normalized_line TEXT NOT NULL,
    source_order_key TEXT NOT NULL DEFAULT '',
    match_method TEXT NOT NULL DEFAULT '',
    match_score NUMERIC(8,6) NOT NULL DEFAULT 0,
    UNIQUE (product_key, sku_key, normalized_line, source_order_key)
);

CREATE TABLE IF NOT EXISTS curated.orders (
    id BIGSERIAL PRIMARY KEY,
    source_order_key TEXT NOT NULL,
    sheet_name TEXT NOT NULL,
    sequence_original TEXT NOT NULL,
    sequence_effective TEXT NOT NULL,
    source_row_number INT NOT NULL,
    source_fingerprint TEXT NOT NULL,
    order_date DATE,
    customer_key TEXT NOT NULL REFERENCES curated.customers(customer_key),
    customer_raw TEXT NOT NULL DEFAULT '',
    order_source_raw TEXT NOT NULL DEFAULT '',
    order_type_raw TEXT NOT NULL DEFAULT '',
    payment_status_raw TEXT NOT NULL DEFAULT '',
    shipment_status_raw TEXT NOT NULL DEFAULT '',
    amount_value NUMERIC(18,2),
    amount_raw TEXT NOT NULL DEFAULT '',
    amount_derived BOOLEAN NOT NULL DEFAULT false,
    shipping_amount_value NUMERIC(18,2),
    shipping_amount_raw TEXT NOT NULL DEFAULT '',
    tracking_no_raw TEXT NOT NULL DEFAULT '',
    remark_raw TEXT NOT NULL DEFAULT '',
    review_status TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source_order_key)
);

CREATE TABLE IF NOT EXISTS curated.order_items (
    source_item_key TEXT PRIMARY KEY,
    source_order_key TEXT NOT NULL REFERENCES curated.orders(source_order_key) ON DELETE CASCADE,
    line_no INT NOT NULL,
    raw_line TEXT NOT NULL,
    product_key TEXT NOT NULL REFERENCES curated.products(product_key),
    sku_key TEXT NOT NULL DEFAULT '',
    parent_name TEXT NOT NULL DEFAULT '',
    spec_name TEXT NOT NULL DEFAULT '',
    product_kind TEXT NOT NULL DEFAULT '',
    roast_level TEXT NOT NULL DEFAULT '',
    order_quantity NUMERIC(18,6) NOT NULL DEFAULT 0,
    order_unit TEXT NOT NULL DEFAULT '',
    normalized_weight_g NUMERIC(18,6) NOT NULL DEFAULT 0,
    review_status TEXT NOT NULL,
    UNIQUE (source_order_key, line_no)
);

CREATE TABLE IF NOT EXISTS review.issues (
    issue_key TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES raw.import_runs(run_id),
    entity_type TEXT NOT NULL,
    entity_key TEXT NOT NULL,
    code TEXT NOT NULL,
    severity TEXT NOT NULL,
    message TEXT NOT NULL,
    source_order_key TEXT NOT NULL DEFAULT '',
    sheet_name TEXT NOT NULL DEFAULT '',
    source_row_number INT NOT NULL DEFAULT 0,
    review_status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS review.decisions (
    issue_key TEXT PRIMARY KEY REFERENCES review.issues(issue_key) ON DELETE CASCADE,
    decision TEXT NOT NULL,
    decided_by TEXT NOT NULL DEFAULT '',
    decided_at TIMESTAMPTZ,
    note TEXT NOT NULL DEFAULT ''
);
`

func StagingSchemaSQL() string {
	return stagingSchemaSQL
}

func BuildReviewContract(_ Dataset) ReviewContract {
	return ReviewContract{SheetNames: []string{
		"导入汇总",
		"序号映射",
		"客户候选",
		"客户导入审核",
		"客户别名",
		"父商品候选",
		"SKU规格",
		"订单候选",
		"订单明细",
		"ERP匹配建议",
		"待审核问题",
		"排除工作表",
	}}
}
