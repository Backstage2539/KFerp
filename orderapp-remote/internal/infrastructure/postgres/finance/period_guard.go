package finance

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EnsurePeriodProtection also runs after all application modules are initialized.
// Statement triggers acquire the monthly-close lock before any source row locks.
// This includes legacy writers that do not call the new order handling service.
func EnsurePeriodProtection(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
CREATE OR REPLACE FUNCTION %[1]s.finance_lock_source_write()RETURNS trigger LANGUAGE plpgsql AS $fn$
BEGIN PERFORM pg_advisory_xact_lock(hashtext('%[1]s:finance-period'));RETURN NULL;END $fn$;
CREATE OR REPLACE FUNCTION %[1]s.finance_guard_source_period()RETURNS trigger LANGUAGE plpgsql AS $fn$
BEGIN
 IF TG_OP='UPDATE' AND to_jsonb(NEW)=to_jsonb(OLD)THEN RETURN NEW;END IF;
 IF COALESCE((SELECT closing_mode FROM %[1]s.finance_settings WHERE id=1),'strong_lock')='strong_lock'
 AND EXISTS(SELECT 1 FROM %[1]s.finance_monthly_reports WHERE status IN('closed','adjusted') AND month IN(to_jsonb(NEW)->>'month',to_jsonb(OLD)->>'month'))THEN
 RAISE EXCEPTION '所选账期已结账，请选择开放账期或办理财务更正';END IF;
 IF TG_OP='DELETE'THEN RETURN OLD;END IF;RETURN NEW;
END $fn$;`, schema))
	if err != nil {
		return err
	}
	for _, table := range []string{"orders", "production_batch_costs", "finance_expenses", "finance_tax_ledger", "finance_settings", "finance_adjustments", "finance_monthly_reports", "order_resolution_financial_entries"} {
		var exists bool
		if err = pool.QueryRow(ctx, `SELECT to_regclass($1)IS NOT NULL`, schema+"."+table).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			continue
		}
		if _, err = pool.Exec(ctx, fmt.Sprintf(`DROP TRIGGER IF EXISTS finance_lock_source_write ON %[1]s.%[2]s;CREATE TRIGGER finance_lock_source_write BEFORE INSERT OR UPDATE OR DELETE ON %[1]s.%[2]s FOR EACH STATEMENT EXECUTE FUNCTION %[1]s.finance_lock_source_write()`, schema, table)); err != nil {
			return err
		}
		if table == "finance_expenses" || table == "finance_tax_ledger" {
			if _, err = pool.Exec(ctx, fmt.Sprintf(`DROP TRIGGER IF EXISTS finance_guard_source_period ON %[1]s.%[2]s;CREATE TRIGGER finance_guard_source_period BEFORE INSERT OR UPDATE OR DELETE ON %[1]s.%[2]s FOR EACH ROW EXECUTE FUNCTION %[1]s.finance_guard_source_period()`, schema, table)); err != nil {
				return err
			}
		}
	}
	return nil
}
