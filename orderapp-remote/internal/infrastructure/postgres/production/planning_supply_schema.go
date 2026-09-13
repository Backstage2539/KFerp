package production

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ensurePlanningSupplyTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
 ALTER TABLE %[1]s.production_plans ADD COLUMN IF NOT EXISTS supply_graph_json JSONB;
 ALTER TABLE %[1]s.production_plan_items ADD COLUMN IF NOT EXISTS demand_sources_json JSONB NOT NULL DEFAULT '[]';
 ALTER TABLE %[1]s.work_order_dependencies ADD COLUMN IF NOT EXISTS delivered_g BIGINT NOT NULL DEFAULT 0;
 ALTER TABLE %[1]s.work_order_dependencies ADD COLUMN IF NOT EXISTS delivered_units BIGINT NOT NULL DEFAULT 0;
 CREATE TABLE IF NOT EXISTS %[1]s.production_supply_allocations (
  id BIGSERIAL PRIMARY KEY, production_plan_id BIGINT NOT NULL, production_plan_item_id BIGINT NOT NULL,
  supplier_work_order_id BIGINT NOT NULL, supplier_plan_item_id BIGINT NOT NULL, work_order_id BIGINT NOT NULL DEFAULT 0,
  material_id BIGINT NOT NULL, warehouse TEXT NOT NULL, owner_customer_id BIGINT NOT NULL DEFAULT 0,
  required_g BIGINT NOT NULL DEFAULT 0 CHECK(required_g>=0), required_units BIGINT NOT NULL DEFAULT 0 CHECK(required_units>=0),
  status TEXT NOT NULL DEFAULT 'proposed',created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(production_plan_item_id,supplier_work_order_id,material_id)
 );
 CREATE INDEX IF NOT EXISTS production_supply_allocations_plan_idx ON %[1]s.production_supply_allocations(production_plan_id);
 CREATE TABLE IF NOT EXISTS %[1]s.production_request_keys (
  action TEXT NOT NULL,actor TEXT NOT NULL,request_id TEXT NOT NULL,payload_hash TEXT NOT NULL,
  result_id BIGINT NOT NULL DEFAULT 0,result_json JSONB,created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY(action,actor,request_id)
 );
 CREATE TABLE IF NOT EXISTS %[1]s.production_material_receipts (
  id BIGSERIAL PRIMARY KEY,work_order_id BIGINT NOT NULL,running_item_id BIGINT NOT NULL,
  material_batch_id BIGINT NOT NULL,stock_entry_id BIGINT NOT NULL,
  finished_g BIGINT NOT NULL DEFAULT 0,finished_units BIGINT NOT NULL DEFAULT 0,
  input_g BIGINT NOT NULL DEFAULT 0,material_cost NUMERIC(18,6) NOT NULL DEFAULT 0,operation_cost NUMERIC(18,6) NOT NULL DEFAULT 0,
  completion_mode TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now()
 );
 CREATE INDEX IF NOT EXISTS production_material_receipts_work_idx ON %[1]s.production_material_receipts(work_order_id);
 `, schema))
	return err
}
