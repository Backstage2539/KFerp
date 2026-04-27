package materials

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.materials (
		id BIGSERIAL PRIMARY KEY,
		code TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		kind TEXT NOT NULL DEFAULT 'other',
		unit TEXT NOT NULL DEFAULT 'g',
		batch_no TEXT NOT NULL DEFAULT '',
		purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0,
		sale_price NUMERIC(12,2) NOT NULL DEFAULT 0,
		onhand_g BIGINT NOT NULL DEFAULT 0,
		onhand_units BIGINT NOT NULL DEFAULT 0,
		min_level_g BIGINT NOT NULL DEFAULT 0,
		min_level_units BIGINT NOT NULL DEFAULT 0,
		origin TEXT NOT NULL DEFAULT '',
		processing_station TEXT NOT NULL DEFAULT '',
		variety TEXT NOT NULL DEFAULT '',
		process_method TEXT NOT NULL DEFAULT '',
		grade TEXT NOT NULL DEFAULT '',
		altitude TEXT NOT NULL DEFAULT '',
		flavor TEXT NOT NULL DEFAULT '',
		bean_list_note TEXT NOT NULL DEFAULT '',
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	for _, stmt := range []string{
		`ALTER TABLE %[1]s.materials ADD COLUMN IF NOT EXISTS purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0`,
		`ALTER TABLE %[1]s.materials ADD COLUMN IF NOT EXISTS sale_price NUMERIC(12,2) NOT NULL DEFAULT 0`,
		`ALTER TABLE %[1]s.materials ADD COLUMN IF NOT EXISTS batch_no TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.materials ADD COLUMN IF NOT EXISTS origin TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.materials ADD COLUMN IF NOT EXISTS processing_station TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.materials ADD COLUMN IF NOT EXISTS variety TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.materials ADD COLUMN IF NOT EXISTS process_method TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.materials ADD COLUMN IF NOT EXISTS grade TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.materials ADD COLUMN IF NOT EXISTS altitude TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.materials ADD COLUMN IF NOT EXISTS flavor TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.materials ADD COLUMN IF NOT EXISTS bean_list_note TEXT NOT NULL DEFAULT ''`,
		`UPDATE %[1]s.materials SET batch_no=to_char(now(),'YYYYMMDD') WHERE batch_no=''`,
	} {
		if _, err := pool.Exec(ctx, fmt.Sprintf(stmt, schema)); err != nil {
			return err
		}
	}
	logQ := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.material_consumption_logs (
		id BIGSERIAL PRIMARY KEY,
		running_item_id BIGINT NOT NULL,
		batch_id TEXT NOT NULL DEFAULT '',
		product_id BIGINT NOT NULL DEFAULT 0,
		product_name TEXT NOT NULL DEFAULT '',
		spec_g BIGINT NOT NULL DEFAULT 0,
		material_id BIGINT NOT NULL,
		material_name TEXT NOT NULL DEFAULT '',
		unit TEXT NOT NULL DEFAULT '',
		deduct_g BIGINT NOT NULL DEFAULT 0,
		deduct_units BIGINT NOT NULL DEFAULT 0,
		before_g BIGINT NOT NULL DEFAULT 0,
		after_g BIGINT NOT NULL DEFAULT 0,
		before_units BIGINT NOT NULL DEFAULT 0,
		after_units BIGINT NOT NULL DEFAULT 0,
		operator TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);
	CREATE INDEX IF NOT EXISTS material_consumption_logs_running_idx ON %s.material_consumption_logs(running_item_id, id);
	CREATE INDEX IF NOT EXISTS material_consumption_logs_material_idx ON %s.material_consumption_logs(material_id, created_at DESC);`, schema, schema, schema)
	if _, err := pool.Exec(ctx, logQ); err != nil {
		return err
	}
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.material_consumption_logs ADD COLUMN IF NOT EXISTS material_batch_id BIGINT NOT NULL DEFAULT 0`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.material_consumption_logs ADD COLUMN IF NOT EXISTS material_batch_code TEXT NOT NULL DEFAULT ''`, schema))
	return nil
}
