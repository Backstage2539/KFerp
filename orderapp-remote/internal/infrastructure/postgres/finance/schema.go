package finance

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %[1]s.finance_settings (
	id INTEGER PRIMARY KEY DEFAULT 1,
	company_type TEXT NOT NULL DEFAULT 'coffee_roaster',
	taxpayer_type TEXT NOT NULL DEFAULT 'small_scale',
	declaration_period TEXT NOT NULL DEFAULT 'monthly',
	closing_mode TEXT NOT NULL DEFAULT 'strong_lock',
	small_scale_vat_rate NUMERIC(10,6) NOT NULL DEFAULT 0.03,
	small_scale_vat_threshold NUMERIC(14,2) NOT NULL DEFAULT 100000,
	general_output_vat_rate NUMERIC(10,6) NOT NULL DEFAULT 0.13,
	default_input_vat_rate NUMERIC(10,6) NOT NULL DEFAULT 0.13,
	surtax_rate NUMERIC(10,6) NOT NULL DEFAULT 0.12,
	cit_standard_rate NUMERIC(10,6) NOT NULL DEFAULT 0.25,
	small_low_profit_enabled BOOLEAN NOT NULL DEFAULT true,
	small_low_profit_effective_rate NUMERIC(10,6) NOT NULL DEFAULT 0.05,
	small_low_profit_annual_profit_limit NUMERIC(14,2) NOT NULL DEFAULT 3000000,
	close_mode_admin_users JSONB NOT NULL DEFAULT '[]'::jsonb,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_by TEXT NOT NULL DEFAULT '',
	CONSTRAINT finance_settings_singleton CHECK (id = 1)
);
CREATE TABLE IF NOT EXISTS %[1]s.finance_expenses (
	id BIGSERIAL PRIMARY KEY,
	expense_date DATE NOT NULL,
	month TEXT NOT NULL DEFAULT '',
	category TEXT NOT NULL DEFAULT '',
	amount NUMERIC(14,2) NOT NULL DEFAULT 0,
	allocation TEXT NOT NULL DEFAULT 'period_expense',
	employee_id BIGINT NULL REFERENCES %[1]s.company_employees(id) ON DELETE SET NULL,
	order_id BIGINT NOT NULL DEFAULT 0,
	customer_id BIGINT NOT NULL DEFAULT 0,
	product_id BIGINT NOT NULL DEFAULT 0,
	batch_no TEXT NOT NULL DEFAULT '',
	dimension_note TEXT NOT NULL DEFAULT '',
	input_vat NUMERIC(14,2) NOT NULL DEFAULT 0,
	non_deductible_input_vat NUMERIC(14,2) NOT NULL DEFAULT 0,
	payment TEXT NOT NULL DEFAULT '',
	note TEXT NOT NULL DEFAULT '',
	created_by TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS finance_expenses_month_idx ON %[1]s.finance_expenses(month, expense_date, id);
CREATE TABLE IF NOT EXISTS %[1]s.finance_tax_ledger (
	id BIGSERIAL PRIMARY KEY,
	month TEXT NOT NULL DEFAULT '',
	kind TEXT NOT NULL DEFAULT '',
	invoice_no TEXT NOT NULL DEFAULT '',
	counterparty TEXT NOT NULL DEFAULT '',
	total_amount NUMERIC(14,2) NOT NULL DEFAULT 0,
	tax_amount NUMERIC(14,2) NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'pending',
	note TEXT NOT NULL DEFAULT '',
	created_by TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS finance_tax_ledger_month_idx ON %[1]s.finance_tax_ledger(month, id);
CREATE TABLE IF NOT EXISTS %[1]s.finance_monthly_reports (
	month TEXT PRIMARY KEY,
	status TEXT NOT NULL DEFAULT 'draft',
	snapshot_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	generated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	closed_at TIMESTAMPTZ NULL,
	closed_by TEXT NOT NULL DEFAULT '',
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS %[1]s.finance_adjustments (
	id BIGSERIAL PRIMARY KEY,
	month TEXT NOT NULL DEFAULT '',
	type TEXT NOT NULL DEFAULT '',
	amount NUMERIC(14,2) NOT NULL DEFAULT 0,
	reason TEXT NOT NULL DEFAULT '',
	note TEXT NOT NULL DEFAULT '',
	actor TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS finance_adjustments_month_idx ON %[1]s.finance_adjustments(month, id);
`, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	for _, stmt := range []string{
		fmt.Sprintf(`ALTER TABLE %s.finance_expenses ADD COLUMN IF NOT EXISTS employee_id BIGINT NULL`, schema),
		fmt.Sprintf(`ALTER TABLE %s.finance_expenses ADD COLUMN IF NOT EXISTS order_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.finance_expenses ADD COLUMN IF NOT EXISTS customer_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.finance_expenses ADD COLUMN IF NOT EXISTS product_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.finance_expenses ADD COLUMN IF NOT EXISTS batch_no TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.finance_expenses ADD COLUMN IF NOT EXISTS dimension_note TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS finance_expenses_employee_idx ON %s.finance_expenses(employee_id, month, expense_date, id)`, schema),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS finance_expenses_dimension_idx ON %s.finance_expenses(month, order_id, customer_id, product_id, batch_no)`, schema),
		fmt.Sprintf(`
DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1 FROM pg_constraint
		WHERE conname = 'finance_expenses_employee_fk'
		  AND conrelid = '%[1]s.finance_expenses'::regclass
	) THEN
		ALTER TABLE %[1]s.finance_expenses
		ADD CONSTRAINT finance_expenses_employee_fk
		FOREIGN KEY (employee_id) REFERENCES %[1]s.company_employees(id) ON DELETE SET NULL;
	END IF;
END $$;
`, schema),
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.finance_settings(id) VALUES(1) ON CONFLICT(id) DO NOTHING`, schema))
	return err
}
