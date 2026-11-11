package production

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ensureOperationCompletionSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
ALTER TABLE IF EXISTS %[1]s.stock_batches ADD COLUMN IF NOT EXISTS cost_unavailable BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE IF EXISTS %[1]s.material_batches ADD COLUMN IF NOT EXISTS cost_unavailable BOOLEAN NOT NULL DEFAULT false;
CREATE TABLE IF NOT EXISTS %[1]s.production_operation_events (
 id BIGSERIAL PRIMARY KEY, work_order_id BIGINT NOT NULL, job_card_id BIGINT NOT NULL,
 kind TEXT NOT NULL DEFAULT 'complete', status TEXT NOT NULL DEFAULT 'posting',
 request_id TEXT NOT NULL, expected_version TEXT NOT NULL, operator TEXT NOT NULL,
 input_qty NUMERIC(20,8) NOT NULL DEFAULT 0, input_unit TEXT NOT NULL,
 output_qty NUMERIC(20,8) NOT NULL DEFAULT 0, output_unit TEXT NOT NULL,
 planned_qty NUMERIC(20,8) NOT NULL DEFAULT 0, difference_reason TEXT NOT NULL DEFAULT '',
 warehouse TEXT NOT NULL, receipt_enabled BOOLEAN NOT NULL DEFAULT false, final_output BOOLEAN NOT NULL DEFAULT false,
 material_cost NUMERIC(20,6) NOT NULL DEFAULT 0, operation_cost NUMERIC(20,6) NOT NULL DEFAULT 0,
 estimated_cost BOOLEAN NOT NULL DEFAULT false, reversal_of BIGINT NOT NULL DEFAULT 0,
 payload JSONB NOT NULL, result JSONB NOT NULL DEFAULT '{}'::jsonb, created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 CHECK(input_qty>=0 AND output_qty>=0)
);
DROP INDEX IF EXISTS %[1]s.production_operation_events_completion_uq;
CREATE INDEX IF NOT EXISTS production_operation_events_completion_idx ON %[1]s.production_operation_events(job_card_id,id) WHERE kind='complete';
CREATE INDEX IF NOT EXISTS production_operation_events_work_idx ON %[1]s.production_operation_events(work_order_id,id);
CREATE TABLE IF NOT EXISTS %[1]s.production_execution_batches (
 id BIGSERIAL PRIMARY KEY, job_card_id BIGINT NOT NULL REFERENCES %[1]s.job_cards(id) ON DELETE CASCADE, batch_no INT NOT NULL CHECK(batch_no>0),
 planned_input_qty NUMERIC(20,8) NOT NULL CHECK(planned_input_qty>0), init_request_id TEXT NOT NULL DEFAULT '', init_expected_version TEXT NOT NULL DEFAULT '', input_unit TEXT NOT NULL DEFAULT '', actual_input_qty NUMERIC(20,8),
 actual_output_qty NUMERIC(20,8), status TEXT NOT NULL DEFAULT 'pending', event_id BIGINT REFERENCES %[1]s.production_operation_events(id),
 UNIQUE(job_card_id,batch_no), UNIQUE(event_id)
);
ALTER TABLE %[1]s.production_execution_batches ADD COLUMN IF NOT EXISTS input_unit TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.production_execution_batches ADD COLUMN IF NOT EXISTS init_request_id TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.production_execution_batches ADD COLUMN IF NOT EXISTS init_expected_version TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.production_execution_batches ADD COLUMN IF NOT EXISTS planned_output_qty NUMERIC(20,8);
ALTER TABLE %[1]s.production_execution_batches ADD COLUMN IF NOT EXISTS output_unit TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.production_execution_batches ADD COLUMN IF NOT EXISTS planned_minutes INT;
CREATE TABLE IF NOT EXISTS %[1]s.production_operation_balances (
 id BIGSERIAL PRIMARY KEY, event_id BIGINT NOT NULL UNIQUE REFERENCES %[1]s.production_operation_events(id),
 work_order_id BIGINT NOT NULL, job_card_id BIGINT NOT NULL, sequence_no INT NOT NULL,
 owner_customer_id BIGINT NOT NULL DEFAULT 0, warehouse TEXT NOT NULL, unit TEXT NOT NULL,
 output_qty NUMERIC(20,8) NOT NULL, remaining_qty NUMERIC(20,8) NOT NULL,
 total_cost NUMERIC(20,6) NOT NULL DEFAULT 0, remaining_cost NUMERIC(20,6) NOT NULL DEFAULT 0,
 quality_status TEXT NOT NULL DEFAULT 'unchecked', created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 CHECK(output_qty>=0 AND remaining_qty>=0 AND remaining_qty<=output_qty)
);
CREATE INDEX IF NOT EXISTS production_operation_balances_available_idx ON %[1]s.production_operation_balances(work_order_id,sequence_no,id) WHERE remaining_qty>0;
CREATE TABLE IF NOT EXISTS %[1]s.production_operation_movements (
 id BIGSERIAL PRIMARY KEY, event_id BIGINT NOT NULL REFERENCES %[1]s.production_operation_events(id),
 source_balance_id BIGINT NOT NULL REFERENCES %[1]s.production_operation_balances(id),
 qty NUMERIC(20,8) NOT NULL, unit TEXT NOT NULL, warehouse TEXT NOT NULL, cost NUMERIC(20,6) NOT NULL DEFAULT 0,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), CHECK(qty<>0)
);
CREATE TABLE IF NOT EXISTS %[1]s.production_started_handoffs (
 id BIGSERIAL PRIMARY KEY, event_id BIGINT NOT NULL REFERENCES %[1]s.production_operation_events(id),
 job_card_id BIGINT NOT NULL, source_balance_id BIGINT NOT NULL REFERENCES %[1]s.production_operation_balances(id),
 qty NUMERIC(20,8) NOT NULL CHECK(qty>0), remaining_qty NUMERIC(20,8) NOT NULL CHECK(remaining_qty>=0 AND remaining_qty<=qty),
 status TEXT NOT NULL DEFAULT 'posted', UNIQUE(event_id,source_balance_id)
);
CREATE INDEX IF NOT EXISTS production_started_handoffs_available_idx ON %[1]s.production_started_handoffs(source_balance_id) WHERE status='posted' AND remaining_qty>0;
CREATE TABLE IF NOT EXISTS %[1]s.production_bom_operation_bindings (
 bom_version_id BIGINT NOT NULL, bom_variant_id BIGINT NOT NULL DEFAULT 0, PRIMARY KEY(bom_version_id,bom_variant_id), bindings JSONB NOT NULL DEFAULT '{}'::jsonb,
 updated_by TEXT NOT NULL, updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %[1]s.work_orders ADD COLUMN IF NOT EXISTS operation_material_bindings JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE %[1]s.work_orders ADD COLUMN IF NOT EXISTS material_consumption_policy TEXT NOT NULL DEFAULT 'legacy';
CREATE TABLE IF NOT EXISTS %[1]s.production_started_inputs (
 id BIGSERIAL PRIMARY KEY,
 work_order_id BIGINT NOT NULL,
 job_card_id BIGINT NOT NULL,
 request_id TEXT NOT NULL,
 input_qty NUMERIC(20,8) NOT NULL,
 remaining_qty NUMERIC(20,8) NOT NULL,
 input_unit TEXT NOT NULL,
 material_cost NUMERIC(20,6) NOT NULL DEFAULT 0,
 remaining_cost NUMERIC(20,6) NOT NULL DEFAULT 0,
 stock_entry_id BIGINT NOT NULL DEFAULT 0,
 material_snapshot JSONB NOT NULL DEFAULT '[]'::jsonb,
 operator TEXT NOT NULL DEFAULT '',
 status TEXT NOT NULL DEFAULT 'posted',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(job_card_id,request_id),
 CHECK(input_qty>0 AND remaining_qty>=0 AND remaining_qty<=input_qty),
 CHECK(material_cost>=0 AND remaining_cost>=0 AND remaining_cost<=material_cost)
);
CREATE TABLE IF NOT EXISTS %[1]s.production_started_input_allocations (
 id BIGSERIAL PRIMARY KEY,
 input_id BIGINT NOT NULL REFERENCES %[1]s.production_started_inputs(id),
 event_id BIGINT NOT NULL REFERENCES %[1]s.production_operation_events(id),
 qty NUMERIC(20,8) NOT NULL CHECK(qty>=0),
 cost NUMERIC(20,6) NOT NULL CHECK(cost>=0),
 UNIQUE(input_id,event_id)
);
DO $packaging_allocation_migration$
BEGIN
 IF EXISTS (SELECT 1 FROM pg_constraint c JOIN pg_class t ON t.oid=c.conrelid JOIN pg_namespace n ON n.oid=t.relnamespace WHERE n.nspname='%[1]s' AND t.relname='production_started_input_allocations' AND c.conname='production_started_input_allocations_qty_check' AND pg_get_constraintdef(c.oid) LIKE '%%qty > %%') THEN
  ALTER TABLE %[1]s.production_started_input_allocations DROP CONSTRAINT production_started_input_allocations_qty_check;
  ALTER TABLE %[1]s.production_started_input_allocations ADD CONSTRAINT production_started_input_allocations_qty_check CHECK(qty>=0);
 END IF;
END $packaging_allocation_migration$;
CREATE TABLE IF NOT EXISTS %[1]s.production_started_input_components (
 id BIGSERIAL PRIMARY KEY,
 input_id BIGINT NOT NULL REFERENCES %[1]s.production_started_inputs(id),
 component_key TEXT NOT NULL CHECK(component_key<>''),
 component_type TEXT NOT NULL CHECK(component_type IN ('material','product')),
 component_id BIGINT NOT NULL CHECK(component_id>0),
 bom_spec_id BIGINT NOT NULL DEFAULT 0,
 bom_variant_id BIGINT NOT NULL DEFAULT 0,
 spec_g BIGINT NOT NULL DEFAULT 0,
 stock_entry_item_id BIGINT NOT NULL DEFAULT 0,
 material_batch_id BIGINT NOT NULL DEFAULT 0,
 stock_batch_id BIGINT NOT NULL DEFAULT 0,
 warehouse TEXT NOT NULL CHECK(warehouse<>''),
 owner_customer_id BIGINT NOT NULL DEFAULT 0,
 qty NUMERIC(20,8) NOT NULL CHECK(qty>0),
 remaining_qty NUMERIC(20,8) NOT NULL,
 unit TEXT NOT NULL CHECK(unit<>''),
 cost NUMERIC(20,6) NOT NULL CHECK(cost>=0),
 remaining_cost NUMERIC(20,6) NOT NULL,
 CHECK((material_batch_id>0 AND stock_batch_id=0) OR (stock_batch_id>0 AND material_batch_id=0)),
 CHECK(remaining_qty>=0 AND remaining_qty<=qty),
 CHECK(remaining_cost>=0 AND remaining_cost<=cost)
);
ALTER TABLE %[1]s.production_started_input_components ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS production_started_input_components_input_idx ON %[1]s.production_started_input_components(input_id,id);
CREATE TABLE IF NOT EXISTS %[1]s.production_started_input_component_allocations (
 id BIGSERIAL PRIMARY KEY,
 component_id BIGINT NOT NULL REFERENCES %[1]s.production_started_input_components(id),
 event_id BIGINT NOT NULL REFERENCES %[1]s.production_operation_events(id),
 qty NUMERIC(20,8) NOT NULL CHECK(qty>0),
 cost NUMERIC(20,6) NOT NULL CHECK(cost>=0),
 UNIQUE(component_id,event_id)
);
CREATE TABLE IF NOT EXISTS %[1]s.production_started_input_returns (
 id BIGSERIAL PRIMARY KEY, component_id BIGINT NOT NULL REFERENCES %[1]s.production_started_input_components(id),
 event_id BIGINT NOT NULL REFERENCES %[1]s.production_operation_events(id), qty NUMERIC(20,8) NOT NULL CHECK(qty>0),cost NUMERIC(20,6) NOT NULL CHECK(cost>=0), UNIQUE(component_id,event_id)
);
ALTER TABLE %[1]s.production_started_inputs ADD COLUMN IF NOT EXISTS event_id BIGINT REFERENCES %[1]s.production_operation_events(id);
ALTER TABLE %[1]s.job_cards ADD COLUMN IF NOT EXISTS completion_draft JSONB NOT NULL DEFAULT '{}'::jsonb;
`, schema))
	return err
}
