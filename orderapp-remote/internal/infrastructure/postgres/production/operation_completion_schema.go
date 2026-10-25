package production

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ensureOperationCompletionSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
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
CREATE TABLE IF NOT EXISTS %[1]s.production_bom_operation_bindings (
 bom_version_id BIGINT NOT NULL, bom_variant_id BIGINT NOT NULL DEFAULT 0, PRIMARY KEY(bom_version_id,bom_variant_id), bindings JSONB NOT NULL DEFAULT '{}'::jsonb,
 updated_by TEXT NOT NULL, updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %[1]s.work_orders ADD COLUMN IF NOT EXISTS operation_material_bindings JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE %[1]s.job_cards ADD COLUMN IF NOT EXISTS completion_draft JSONB NOT NULL DEFAULT '{}'::jsonb;
`, schema))
	return err
}
