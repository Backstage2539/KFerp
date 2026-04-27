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
	if err := EnsureSenderSettingsTable(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureOutsourceFeeColumns(ctx, pool, schema); err != nil {
		return err
	}
	return ensureOutsourceTemplateTables(ctx, pool, schema)
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
