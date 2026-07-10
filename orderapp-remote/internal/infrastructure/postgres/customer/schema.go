package customer

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %[1]s.customer_assets (
	id BIGSERIAL PRIMARY KEY,
	customer_id BIGINT NOT NULL REFERENCES %[1]s.customers(id) ON DELETE CASCADE,
	kind TEXT NOT NULL,
	object_key TEXT NOT NULL,
	content_type TEXT NOT NULL,
	bytes BIGINT NOT NULL,
	sha256 TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_by TEXT,
	UNIQUE(customer_id, kind)
);
CREATE INDEX IF NOT EXISTS customer_assets_customer_id_idx
	ON %[1]s.customer_assets(customer_id);
`, schema)
	_, err := pool.Exec(ctx, q)
	return err
}
