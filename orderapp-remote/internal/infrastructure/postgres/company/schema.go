package company

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.company_departments (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS %s.company_employees (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	phone TEXT NOT NULL,
	account_type TEXT NOT NULL DEFAULT 'internal_employee',
	department_id BIGINT NOT NULL REFERENCES %s.company_departments(id),
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS company_employees_phone_uq ON %s.company_employees(phone);
CREATE TABLE IF NOT EXISTS %s.company_profile (
	id INTEGER PRIMARY KEY DEFAULT 1,
	company_name TEXT NOT NULL DEFAULT '',
	company_address TEXT NOT NULL DEFAULT '',
	company_phone TEXT NOT NULL DEFAULT '',
	taxpayer_id TEXT NOT NULL DEFAULT '',
	bank_account_name TEXT NOT NULL DEFAULT '',
	bank_name TEXT NOT NULL DEFAULT '',
	bank_account_no TEXT NOT NULL DEFAULT '',
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_by TEXT NOT NULL DEFAULT '',
	CONSTRAINT company_profile_singleton CHECK (id = 1)
);
`, schema, schema, schema, schema, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	for _, stmt := range []string{
		fmt.Sprintf(`ALTER TABLE %s.company_employees ADD COLUMN IF NOT EXISTS account_type TEXT NOT NULL DEFAULT 'internal_employee'`, schema),
		fmt.Sprintf(`UPDATE %s.company_employees SET account_type='internal_employee' WHERE COALESCE(account_type,'')=''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.company_profile ADD COLUMN IF NOT EXISTS taxpayer_id TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.company_profile ADD COLUMN IF NOT EXISTS bank_account_name TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.company_profile ADD COLUMN IF NOT EXISTS bank_name TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.company_profile ADD COLUMN IF NOT EXISTS bank_account_no TEXT NOT NULL DEFAULT ''`, schema),
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	seed := fmt.Sprintf(`
INSERT INTO %s.company_departments(name,active) VALUES
('销售', true),('生产', true),('财务', true)
ON CONFLICT (name) DO NOTHING;
`, schema)
	_, err := pool.Exec(ctx, seed)
	return err
}
