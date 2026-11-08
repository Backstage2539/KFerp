package orderresolution

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	financepg "orderapp/internal/infrastructure/postgres/finance"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
 ALTER TABLE %[1]s.customer_fee_items ADD COLUMN IF NOT EXISTS resolution_credit_cents BIGINT NOT NULL DEFAULT 0;
 ALTER TABLE %[1]s.customer_fee_items ADD COLUMN IF NOT EXISTS resolution_cash_tracked BOOLEAN NOT NULL DEFAULT false;
 ALTER TABLE %[1]s.customer_fee_items ADD COLUMN IF NOT EXISTS resolution_cash_received_cents BIGINT NOT NULL DEFAULT 0;
 ALTER TABLE %[1]s.customer_fee_items ADD COLUMN IF NOT EXISTS resolution_settled_out_cents BIGINT NOT NULL DEFAULT 0;
 ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS resolution_shipping_credit_cents BIGINT NOT NULL DEFAULT 0;
 ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS resolution_payment_tracked BOOLEAN NOT NULL DEFAULT false;
 ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS resolution_legacy_cash_cents BIGINT NOT NULL DEFAULT 0;
 ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS resolution_settled_out_cents BIGINT NOT NULL DEFAULT 0;
 ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS resolution_offset_cents BIGINT NOT NULL DEFAULT 0;
 ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS resolution_status TEXT NOT NULL DEFAULT 'open';
 ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS fulfillment_paused BOOLEAN NOT NULL DEFAULT false;
 ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS resolution_credit_cents BIGINT NOT NULL DEFAULT 0;
 ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS amended_from_order_id BIGINT REFERENCES %[1]s.orders(id);
 ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS resolution_closed_qty NUMERIC NOT NULL DEFAULT 0;
 ALTER TABLE %[1]s.customer_stock_entitlements ADD COLUMN IF NOT EXISTS closed_qty BIGINT NOT NULL DEFAULT 0;
 CREATE TABLE IF NOT EXISTS %[1]s.order_resolution_requests(
 id BIGSERIAL PRIMARY KEY,order_id BIGINT NOT NULL REFERENCES %[1]s.orders(id),
 idempotency_key TEXT NOT NULL,request_hash TEXT NOT NULL,command_json JSONB NOT NULL,plan_json JSONB NOT NULL,
 status TEXT NOT NULL CHECK(status IN('draft','pending','completed','withdrawn')),
 applied BOOLEAN NOT NULL DEFAULT false,context_json JSONB NOT NULL DEFAULT '{}',
 actor TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(order_id,idempotency_key));
 ALTER TABLE %[1]s.order_resolution_requests ADD COLUMN IF NOT EXISTS applied BOOLEAN NOT NULL DEFAULT false;
 ALTER TABLE %[1]s.order_resolution_requests ADD COLUMN IF NOT EXISTS context_json JSONB NOT NULL DEFAULT '{}';
 CREATE UNIQUE INDEX IF NOT EXISTS order_resolution_pending ON %[1]s.order_resolution_requests(order_id) WHERE status='pending';
 CREATE TABLE IF NOT EXISTS %[1]s.order_resolution_amendments(
 id BIGSERIAL PRIMARY KEY, request_id BIGINT NOT NULL UNIQUE REFERENCES %[1]s.order_resolution_requests(id),
 source_order_id BIGINT NOT NULL REFERENCES %[1]s.orders(id),customer_id BIGINT NOT NULL,
 snapshot_json JSONB NOT NULL,status TEXT NOT NULL DEFAULT 'draft' CHECK(status IN('draft','submitted')),
 new_order_id BIGINT UNIQUE REFERENCES %[1]s.orders(id),actor TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now());
 CREATE TABLE IF NOT EXISTS %[1]s.order_resolution_steps(
 id BIGSERIAL PRIMARY KEY,request_id BIGINT NOT NULL REFERENCES %[1]s.order_resolution_requests(id),
 kind TEXT NOT NULL,source_id BIGINT NOT NULL DEFAULT 0,choice TEXT NOT NULL DEFAULT '',label TEXT NOT NULL,
 permission TEXT NOT NULL,status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN('pending','completed','failed','superseded')),
 result_json JSONB NOT NULL DEFAULT '{}',actor TEXT NOT NULL DEFAULT '',updated_at TIMESTAMPTZ NOT NULL DEFAULT now());
 ALTER TABLE %[1]s.order_resolution_steps ADD COLUMN IF NOT EXISTS last_error TEXT NOT NULL DEFAULT '';
 CREATE TABLE IF NOT EXISTS %[1]s.order_resolution_stock_returns(
 id BIGSERIAL PRIMARY KEY,request_id BIGINT NOT NULL REFERENCES %[1]s.order_resolution_requests(id),step_id BIGINT NOT NULL REFERENCES %[1]s.order_resolution_steps(id),
 order_id BIGINT NOT NULL REFERENCES %[1]s.orders(id),order_item_id BIGINT NOT NULL REFERENCES %[1]s.order_items(id),
 source_kind TEXT NOT NULL CHECK(source_kind IN('shipment','ownership')),source_id BIGINT NOT NULL,batch_id BIGINT NOT NULL REFERENCES %[1]s.stock_batches(id),
 product_id BIGINT NOT NULL,bom_spec_id BIGINT NOT NULL,bom_variant_id BIGINT NOT NULL,
 source_warehouse TEXT NOT NULL DEFAULT '',target_warehouse TEXT NOT NULL,source_owner_customer_id BIGINT NOT NULL,target_owner_customer_id BIGINT NOT NULL,
 quantity BIGINT NOT NULL CHECK(quantity>0),unit_cost NUMERIC NOT NULL CHECK(unit_cost>=0),quality TEXT NOT NULL CHECK(quality IN('restock','quarantine')),
 posting_date DATE NOT NULL,target_batch_id BIGINT NOT NULL DEFAULT 0,target_batch_code TEXT NOT NULL DEFAULT '',source_version TEXT NOT NULL,
 status TEXT NOT NULL CHECK(status IN('posting','posted')),actor TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(request_id,source_kind,source_id));
 CREATE OR REPLACE FUNCTION %[1]s.order_resolution_return_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN
 IF TG_OP='DELETE' OR OLD.status='posted' THEN RAISE EXCEPTION '退回记录已形成事实，不能删除或改写'; END IF;
 IF (to_jsonb(NEW)-'target_batch_id'-'target_batch_code'-'status')<>(to_jsonb(OLD)-'target_batch_id'-'target_batch_code'-'status') THEN RAISE EXCEPTION '退回记录的原来源和处置不能改写'; END IF;
 IF NEW.status='posted' AND NOT EXISTS(SELECT 1 FROM %[1]s.stock_batches b JOIN %[1]s.stock_ledger_entries l ON l.source_doc_type='order_resolution_return' AND l.source_doc_id=NEW.id AND l.source_batch_code=b.batch_code AND l.item_type='finished_product' AND l.item_id=NEW.product_id AND l.bom_spec_id=NEW.bom_spec_id AND l.bom_variant_id=NEW.bom_variant_id AND l.owner_customer_id=NEW.target_owner_customer_id AND l.warehouse=NEW.target_warehouse AND l.qty_change_units=NEW.quantity AND l.qty_after_units=l.qty_before_units+NEW.quantity AND l.qty_change_g=0 WHERE b.id=NEW.target_batch_id AND b.batch_code=NEW.target_batch_code AND b.source_doc_type='order_resolution_return' AND b.source_doc_id=NEW.id AND b.qty_units=NEW.quantity AND b.bom_spec_id=NEW.bom_spec_id AND b.bom_variant_id=NEW.bom_variant_id AND b.owner_customer_id=NEW.target_owner_customer_id AND b.unit_cost=NEW.unit_cost) THEN RAISE EXCEPTION '退回记录缺少真实入库批次和库存流水'; END IF;
 RETURN NEW;
 END $$;
 DROP TRIGGER IF EXISTS order_resolution_return_immutable ON %[1]s.order_resolution_stock_returns;
 CREATE TRIGGER order_resolution_return_immutable BEFORE UPDATE OR DELETE ON %[1]s.order_resolution_stock_returns FOR EACH ROW EXECUTE FUNCTION %[1]s.order_resolution_return_immutable();
 CREATE INDEX IF NOT EXISTS order_resolution_stock_return_source ON %[1]s.order_resolution_stock_returns(source_kind,source_id);
 CREATE TABLE IF NOT EXISTS %[1]s.order_resolution_step_failures(
 id BIGSERIAL PRIMARY KEY,step_id BIGINT NOT NULL REFERENCES %[1]s.order_resolution_steps(id),
 idempotency_key TEXT NOT NULL,request_hash TEXT NOT NULL,command_json JSONB NOT NULL,
 message TEXT NOT NULL,actor TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(step_id,idempotency_key,request_hash));
 CREATE TABLE IF NOT EXISTS %[1]s.order_resolution_production_dispositions(
 request_id BIGINT NOT NULL REFERENCES %[1]s.order_resolution_requests(id),order_id BIGINT NOT NULL REFERENCES %[1]s.orders(id),
 plan_id BIGINT NOT NULL,choice TEXT NOT NULL,actor TEXT NOT NULL,note TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now(),PRIMARY KEY(request_id,plan_id));
 CREATE TABLE IF NOT EXISTS %[1]s.order_resolution_reservation_releases(
 request_id BIGINT NOT NULL REFERENCES %[1]s.order_resolution_requests(id),order_id BIGINT NOT NULL REFERENCES %[1]s.orders(id),
 allocation_id BIGINT NOT NULL,snapshot_json JSONB NOT NULL,actor TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now(),PRIMARY KEY(request_id,allocation_id));
 ALTER TABLE %[1]s.order_resolution_reservation_releases ADD COLUMN IF NOT EXISTS after_json JSONB NOT NULL DEFAULT '{}';
 ALTER TABLE %[1]s.order_resolution_steps DROP CONSTRAINT IF EXISTS order_resolution_steps_status_check;
 ALTER TABLE %[1]s.order_resolution_steps ADD CONSTRAINT order_resolution_steps_status_check CHECK(status IN('pending','completed','failed','superseded'));
 CREATE TABLE IF NOT EXISTS %[1]s.order_resolution_revisions(
 id BIGSERIAL PRIMARY KEY,request_id BIGINT NOT NULL REFERENCES %[1]s.order_resolution_requests(id),
 idempotency_key TEXT NOT NULL,request_hash TEXT NOT NULL,before_json JSONB NOT NULL,after_json JSONB NOT NULL,
 actor TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now(),UNIQUE(request_id,idempotency_key));
 CREATE TABLE IF NOT EXISTS %[1]s.order_resolution_step_actions(
 step_id BIGINT NOT NULL REFERENCES %[1]s.order_resolution_steps(id),idempotency_key TEXT NOT NULL,
 request_hash TEXT NOT NULL,result_json JSONB NOT NULL DEFAULT '{}',actor TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now(),PRIMARY KEY(step_id,idempotency_key));
 CREATE TABLE IF NOT EXISTS %[1]s.order_resolution_fee_events(
 id BIGSERIAL PRIMARY KEY,request_id BIGINT NOT NULL REFERENCES %[1]s.order_resolution_requests(id),
 order_id BIGINT NOT NULL REFERENCES %[1]s.orders(id),source_id BIGINT NOT NULL,credit_cents BIGINT NOT NULL CHECK(credit_cents>=0),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),UNIQUE(request_id,source_id));
 CREATE TABLE IF NOT EXISTS %[1]s.order_resolution_line_events(
 id BIGSERIAL PRIMARY KEY,request_id BIGINT NOT NULL REFERENCES %[1]s.order_resolution_requests(id),
 order_item_id BIGINT NOT NULL REFERENCES %[1]s.order_items(id),closed_delta NUMERIC NOT NULL DEFAULT 0,
 return_delta NUMERIC NOT NULL DEFAULT 0,credit_cents BIGINT NOT NULL DEFAULT 0,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),UNIQUE(request_id,order_item_id));
 CREATE TABLE IF NOT EXISTS %[1]s.order_resolution_billing_adjustments(
 id BIGSERIAL PRIMARY KEY,request_id BIGINT NOT NULL REFERENCES %[1]s.order_resolution_requests(id),
 line_id BIGINT NOT NULL REFERENCES %[1]s.customer_settlement_lines(id),credit_cents BIGINT NOT NULL,
 posting_date DATE NOT NULL,actor TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now(),UNIQUE(request_id,line_id));
 CREATE TABLE IF NOT EXISTS %[1]s.order_resolution_financial_entries(
 id BIGSERIAL PRIMARY KEY,request_id BIGINT NOT NULL REFERENCES %[1]s.order_resolution_requests(id),
 order_id BIGINT NOT NULL REFERENCES %[1]s.orders(id),customer_id BIGINT NOT NULL,posting_date DATE NOT NULL,
 kind TEXT NOT NULL,amount_cents BIGINT NOT NULL,reference TEXT NOT NULL DEFAULT '',
 actor TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now());
 ALTER TABLE %[1]s.order_resolution_financial_entries ADD COLUMN IF NOT EXISTS source_fee_id BIGINT NOT NULL DEFAULT 0;
 ALTER TABLE %[1]s.order_resolution_financial_entries DROP CONSTRAINT IF EXISTS order_resolution_financial_entrie_request_id_kind_reference_key;
 CREATE UNIQUE INDEX IF NOT EXISTS order_resolution_financial_source_reference ON %[1]s.order_resolution_financial_entries(request_id,source_fee_id,kind,reference);
 ALTER TABLE %[1]s.order_resolution_financial_entries ADD COLUMN IF NOT EXISTS account TEXT NOT NULL DEFAULT '';
 ALTER TABLE %[1]s.order_resolution_financial_entries ADD COLUMN IF NOT EXISTS target_order_id BIGINT NOT NULL DEFAULT 0;
 CREATE TABLE IF NOT EXISTS %[1]s.order_resolution_balance_uses(
 id BIGSERIAL PRIMARY KEY,customer_id BIGINT NOT NULL,source_entry_id BIGINT NOT NULL REFERENCES %[1]s.order_resolution_financial_entries(id),
 kind TEXT NOT NULL CHECK(kind IN('refund','offset')),amount_cents BIGINT NOT NULL CHECK(amount_cents>0),
 target_order_id BIGINT NOT NULL DEFAULT 0,posting_date DATE NOT NULL,account TEXT NOT NULL DEFAULT '',reference TEXT NOT NULL DEFAULT '',note TEXT NOT NULL,
 idempotency_key TEXT NOT NULL,request_hash TEXT NOT NULL,actor TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now(),UNIQUE(customer_id,idempotency_key));
 CREATE UNIQUE INDEX IF NOT EXISTS order_resolution_balance_refund_voucher ON %[1]s.order_resolution_balance_uses(source_entry_id,reference)WHERE kind='refund' AND reference<>'';
 CREATE TABLE IF NOT EXISTS %[1]s.order_resolution_period_approvals(request_id BIGINT PRIMARY KEY REFERENCES %[1]s.order_resolution_requests(id),month TEXT NOT NULL,original_snapshot JSONB NOT NULL,reference TEXT NOT NULL,note TEXT NOT NULL,actor TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now());
 CREATE TABLE IF NOT EXISTS %[1]s.order_resolution_period_confirmations(id BIGSERIAL PRIMARY KEY,request_id BIGINT NOT NULL REFERENCES %[1]s.order_resolution_requests(id),month TEXT NOT NULL,original_snapshot JSONB NOT NULL,reference TEXT NOT NULL,note TEXT NOT NULL,actor TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now());
 CREATE TABLE IF NOT EXISTS %[1]s.order_resolution_period_revisions(request_id BIGINT PRIMARY KEY REFERENCES %[1]s.order_resolution_requests(id),month TEXT NOT NULL,before_json JSONB NOT NULL,after_json JSONB NOT NULL,before_report_json JSONB NOT NULL,after_report_json JSONB NOT NULL,actor TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now());
 CREATE INDEX IF NOT EXISTS order_resolution_financial_period ON %[1]s.order_resolution_financial_entries(posting_date,customer_id);
 CREATE TABLE IF NOT EXISTS %[1]s.order_resolution_drafts(
 order_id BIGINT NOT NULL REFERENCES %[1]s.orders(id),actor TEXT NOT NULL,command_json JSONB NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),PRIMARY KEY(order_id,actor));
 CREATE TABLE IF NOT EXISTS %[1]s.order_resolution_policy(
 id INTEGER PRIMARY KEY CHECK(id=1),revision BIGINT NOT NULL DEFAULT 1,
 policy_json JSONB NOT NULL DEFAULT '{}',updated_by TEXT NOT NULL DEFAULT '',updated_at TIMESTAMPTZ NOT NULL DEFAULT now());
 INSERT INTO %[1]s.order_resolution_policy(id)VALUES(1)ON CONFLICT DO NOTHING;
 `, schema))
	if err != nil {
		return err
	}
	if err = ensureGuards(ctx, pool, schema); err != nil {
		return err
	}
	return financepg.EnsurePeriodProtection(ctx, pool, schema)
}
