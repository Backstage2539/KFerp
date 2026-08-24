package purchase

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	stmts := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.purchase_suppliers (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			contact TEXT NOT NULL DEFAULT '',
			phone TEXT NOT NULL DEFAULT '',
			address TEXT NOT NULL DEFAULT '',
			active BOOLEAN NOT NULL DEFAULT true,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.purchase_orders (
			id BIGSERIAL PRIMARY KEY,
			order_no TEXT NOT NULL UNIQUE,
			supplier_id BIGINT NOT NULL REFERENCES %s.purchase_suppliers(id),
			material_id BIGINT NOT NULL REFERENCES %s.materials(id),
			qty_g BIGINT NOT NULL DEFAULT 0,
			qty NUMERIC(14,4) NOT NULL DEFAULT 0,
			unit_code TEXT NOT NULL DEFAULT 'kg',
			qty_units BIGINT NOT NULL DEFAULT 0,
			target_warehouse TEXT NOT NULL DEFAULT 'raw_materials',
			unit_cost NUMERIC(12,4) NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'ordered',
			note TEXT NOT NULL DEFAULT '',
			operator TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schema, schema, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.purchase_receipts (
			id BIGSERIAL PRIMARY KEY,
			receipt_no TEXT NOT NULL UNIQUE,
			purchase_order_id BIGINT NOT NULL DEFAULT 0,
			supplier_id BIGINT NOT NULL DEFAULT 0,
			supplier_name TEXT NOT NULL DEFAULT '',
			material_id BIGINT NOT NULL REFERENCES %s.materials(id),
			qty_g BIGINT NOT NULL DEFAULT 0,
			qty NUMERIC(14,4) NOT NULL DEFAULT 0,
			unit_code TEXT NOT NULL DEFAULT 'kg',
			qty_units BIGINT NOT NULL DEFAULT 0,
			target_warehouse TEXT NOT NULL DEFAULT 'raw_materials',
			unit_cost NUMERIC(12,4) NOT NULL DEFAULT 0,
			stock_receipt_id BIGINT NOT NULL DEFAULT 0,
			stock_batch_code TEXT NOT NULL DEFAULT '',
			note TEXT NOT NULL DEFAULT '',
			operator TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schema, schema),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_purchase_orders_created ON %s.purchase_orders(created_at DESC, id DESC)`, schema, schema),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_purchase_receipts_created ON %s.purchase_receipts(created_at DESC, id DESC)`, schema, schema),
		fmt.Sprintf(`ALTER TABLE %s.purchase_orders ADD COLUMN IF NOT EXISTS qty NUMERIC(14,4) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.purchase_orders ADD COLUMN IF NOT EXISTS unit_code TEXT NOT NULL DEFAULT 'kg'`, schema),
		fmt.Sprintf(`ALTER TABLE %s.purchase_orders ADD COLUMN IF NOT EXISTS qty_units BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.purchase_orders ADD COLUMN IF NOT EXISTS target_warehouse TEXT NOT NULL DEFAULT 'raw_materials'`, schema),
		fmt.Sprintf(`ALTER TABLE %s.purchase_receipts ADD COLUMN IF NOT EXISTS qty NUMERIC(14,4) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.purchase_receipts ADD COLUMN IF NOT EXISTS unit_code TEXT NOT NULL DEFAULT 'kg'`, schema),
		fmt.Sprintf(`ALTER TABLE %s.purchase_receipts ADD COLUMN IF NOT EXISTS qty_units BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.purchase_receipts ADD COLUMN IF NOT EXISTS target_warehouse TEXT NOT NULL DEFAULT 'raw_materials'`, schema),
		fmt.Sprintf(`UPDATE %s.purchase_orders SET qty=qty_g::numeric/1000,unit_code='kg',target_warehouse='raw_materials' WHERE qty=0 AND qty_g>0`, schema),
		fmt.Sprintf(`UPDATE %s.purchase_receipts SET qty=qty_g::numeric/1000,unit_code='kg',target_warehouse='raw_materials' WHERE qty=0 AND qty_g>0`, schema),
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}
