package sales

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ensureCombinedPaymentSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
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
