package production

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ensureScheduleStaffTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
 ALTER TABLE %[1]s.job_cards ADD COLUMN IF NOT EXISTS schedule_version BIGINT NOT NULL DEFAULT 0;
 CREATE TABLE IF NOT EXISTS %[1]s.job_card_collaborators (
  job_card_id BIGINT NOT NULL REFERENCES %[1]s.job_cards(id) ON DELETE CASCADE, employee_id BIGINT NOT NULL,
  employee_name TEXT NOT NULL DEFAULT '', PRIMARY KEY(job_card_id,employee_id)
 );
 CREATE INDEX IF NOT EXISTS job_card_collaborators_employee_idx ON %[1]s.job_card_collaborators(employee_id,job_card_id);
 CREATE TABLE IF NOT EXISTS %[1]s.production_schedule_requests (
  actor TEXT NOT NULL, request_id TEXT NOT NULL, payload_hash TEXT NOT NULL,
  response_json JSONB NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), PRIMARY KEY(actor,request_id)
 );`, schema))
	return err
}
