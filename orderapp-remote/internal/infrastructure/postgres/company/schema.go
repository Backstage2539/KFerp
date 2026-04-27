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
	department_id BIGINT NOT NULL REFERENCES %s.company_departments(id),
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS company_employees_phone_uq ON %s.company_employees(phone);
`, schema, schema, schema, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	seed := fmt.Sprintf(`
INSERT INTO %s.company_departments(name,active) VALUES
('销售', true),('生产', true),('财务', true)
ON CONFLICT (name) DO NOTHING;
`, schema)
	_, err := pool.Exec(ctx, seed)
	return err
}
