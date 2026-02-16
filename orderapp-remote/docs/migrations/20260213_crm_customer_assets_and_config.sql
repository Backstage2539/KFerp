-- CRM-oriented customer archive optimization scaffolding
-- Apply manually to your tenant schema (replace :schema).

-- 1) Config center (key/value)
CREATE TABLE IF NOT EXISTS :schema.app_config (
  key text PRIMARY KEY,
  value text NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now(),
  updated_by text
);

-- 2) Customer assets (logos, etc.)
CREATE TABLE IF NOT EXISTS :schema.customer_assets (
  id bigserial PRIMARY KEY,
  customer_id bigint NOT NULL REFERENCES :schema.customers(id) ON DELETE CASCADE,
  kind text NOT NULL,
  object_key text NOT NULL,
  content_type text NOT NULL,
  bytes bigint NOT NULL,
  sha256 text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  created_by text,
  UNIQUE(customer_id, kind)
);
CREATE INDEX IF NOT EXISTS customer_assets_customer_id_idx ON :schema.customer_assets(customer_id);

-- 3) Optional: add explicit archive flag/timestamp for CRM archive optimization
-- If you only use active=false today, keep that.
ALTER TABLE :schema.customers
  ADD COLUMN IF NOT EXISTS archived_at timestamptz;

CREATE INDEX IF NOT EXISTS customers_archived_at_idx ON :schema.customers(archived_at);
