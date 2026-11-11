package sales

import (
	"context"
	"fmt"
	messagepg "orderapp/internal/infrastructure/postgres/messagecenter"
	"orderapp/internal/infrastructure/postgres/receipts"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ensureCombinedPaymentSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	if err := receipts.EnsureSchema(ctx, pool, schema); err != nil {
		return err
	}
	if err := messagepg.EnsureBusinessEventSchema(ctx, pool, schema); err != nil {
		return err
	}
	_, err := pool.Exec(ctx, fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %[1]s.order_single_receipt_requests (
 id BIGSERIAL PRIMARY KEY, idempotency_key TEXT NOT NULL UNIQUE, request_hash TEXT NOT NULL, receipt_id BIGINT NOT NULL DEFAULT 0, receipt_no TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS %[1]s.order_create_idempotency_keys (
 request_id TEXT PRIMARY KEY, actor TEXT NOT NULL, customer_id BIGINT NOT NULL,
 request_hash TEXT NOT NULL, order_id BIGINT NOT NULL UNIQUE
);
ALTER TABLE %[1]s.order_create_idempotency_keys ADD COLUMN IF NOT EXISTS receipt_warning TEXT NOT NULL DEFAULT '';
CREATE TABLE IF NOT EXISTS %[1]s.order_create_request_cancellations (
 request_id TEXT PRIMARY KEY, request_actor TEXT NOT NULL, request_hash TEXT NOT NULL,
 cancelled_by TEXT NOT NULL, reason TEXT NOT NULL, cancelled_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS %[1]s.order_combined_payments (
 id BIGSERIAL PRIMARY KEY,
 idempotency_key TEXT NOT NULL UNIQUE,
 request_hash TEXT NOT NULL,
 customer_id BIGINT NOT NULL,
 actual_cents BIGINT NOT NULL CHECK (actual_cents > 0),
 due_cents BIGINT NOT NULL CHECK (due_cents > 0),
 unallocated_cents BIGINT NOT NULL CHECK (unallocated_cents >= 0),
 payment_method TEXT NOT NULL,
 payment_voucher_asset_id BIGINT NOT NULL DEFAULT 0,
 actor TEXT NOT NULL,
 result_json JSONB NOT NULL DEFAULT '{}'::jsonb,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 CHECK (actual_cents = due_cents + unallocated_cents)
);
CREATE TABLE IF NOT EXISTS %[1]s.order_combined_payment_allocations (
 batch_id BIGINT NOT NULL REFERENCES %[1]s.order_combined_payments(id),
 order_id BIGINT NOT NULL REFERENCES %[1]s.orders(id),
 order_no TEXT NOT NULL,
 goods_cents BIGINT NOT NULL CHECK (goods_cents >= 0),
 shipping_cents BIGINT NOT NULL CHECK (shipping_cents >= 0),
 amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
 PRIMARY KEY(batch_id,order_id),
 CHECK (amount_cents = goods_cents + shipping_cents)
);
CREATE INDEX IF NOT EXISTS order_combined_payment_order_idx ON %[1]s.order_combined_payment_allocations(order_id,batch_id);
`, schema))
	return err
}
