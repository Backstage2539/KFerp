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
DECLARE old_month TEXT;new_month TEXT;
BEGIN
 IF to_regclass('%[1]s.finance_settings')IS NULL OR to_regclass('%[1]s.finance_monthly_reports')IS NULL THEN IF TG_OP='DELETE'THEN RETURN OLD;END IF;RETURN NEW;END IF;
 IF TG_TABLE_NAME='customer_fee_items' THEN
  IF TG_OP='UPDATE' AND ROW(NEW.customer_id,NEW.source_type,NEW.source_id,NEW.fee_type,NEW.amount,NEW.occurred_at,NEW.status IN('reversed','void','cancelled')) IS NOT DISTINCT FROM ROW(OLD.customer_id,OLD.source_type,OLD.source_id,OLD.fee_type,OLD.amount,OLD.occurred_at,OLD.status IN('reversed','void','cancelled')) THEN RETURN NEW;END IF;
  old_month:=to_char((to_jsonb(OLD)->>'occurred_at')::timestamptz AT TIME ZONE 'Asia/Shanghai','YYYY-MM');
  new_month:=to_char((to_jsonb(NEW)->>'occurred_at')::timestamptz AT TIME ZONE 'Asia/Shanghai','YYYY-MM');
 ELSE old_month:=to_jsonb(OLD)->>'month';new_month:=to_jsonb(NEW)->>'month';END IF;
 IF TG_OP='UPDATE' AND to_jsonb(NEW)=to_jsonb(OLD)THEN RETURN NEW;END IF;
 IF COALESCE((SELECT closing_mode FROM %[1]s.finance_settings WHERE id=1),'strong_lock')='strong_lock'
 AND EXISTS(SELECT 1 FROM %[1]s.finance_monthly_reports WHERE status IN('closed','adjusted') AND month IN(new_month,old_month))THEN
 RAISE EXCEPTION '所选账期已结账，请选择开放账期或办理财务更正';END IF;
 IF TG_OP='DELETE'THEN RETURN OLD;END IF;RETURN NEW;
END $fn$;`, schema))
	if err != nil {
		return err
	}
	for _, table := range []string{"orders", "customer_fee_items", "order_resolution_fee_events", "production_batch_costs", "finance_expenses", "finance_tax_ledger", "finance_settings", "finance_adjustments", "finance_monthly_reports", "order_resolution_financial_entries"} {
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
		if table == "finance_expenses" || table == "finance_tax_ledger" || table == "customer_fee_items" {
			if _, err = pool.Exec(ctx, fmt.Sprintf(`DROP TRIGGER IF EXISTS finance_guard_source_period ON %[1]s.%[2]s;CREATE TRIGGER finance_guard_source_period BEFORE INSERT OR UPDATE OR DELETE ON %[1]s.%[2]s FOR EACH ROW EXECUTE FUNCTION %[1]s.finance_guard_source_period()`, schema, table)); err != nil {
				return err
			}
		}
	}
	return nil
}
