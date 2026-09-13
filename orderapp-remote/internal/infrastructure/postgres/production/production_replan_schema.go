package production

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ensureProductionReplanSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
ALTER TABLE %[1]s.production_plans ADD COLUMN IF NOT EXISTS replanned_from_plan_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.production_plans ADD COLUMN IF NOT EXISTS replan_note TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.production_plan_items ADD COLUMN IF NOT EXISTS replan_status TEXT NOT NULL DEFAULT 'active';
ALTER TABLE %[1]s.production_plan_items ADD COLUMN IF NOT EXISTS replaced_by_plan_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.production_plan_items ADD COLUMN IF NOT EXISTS replan_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.work_orders ADD COLUMN IF NOT EXISTS replan_status TEXT NOT NULL DEFAULT 'active';
ALTER TABLE %[1]s.work_orders ADD COLUMN IF NOT EXISTS replaced_by_plan_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.work_orders ADD COLUMN IF NOT EXISTS replan_reason TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS production_plan_items_replan_idx ON %[1]s.production_plan_items(production_plan_id,replan_status,id);
CREATE INDEX IF NOT EXISTS work_orders_replan_idx ON %[1]s.work_orders(production_plan_id,replan_status,id);
`, schema))
	return err
}
