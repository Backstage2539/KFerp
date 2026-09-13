package production

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ensureProductionRosterTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %[1]s.production_roster_weeks (
	week_start DATE PRIMARY KEY,
	version BIGINT NOT NULL DEFAULT 0,
	updated_by TEXT NOT NULL DEFAULT '',
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS %[1]s.production_employee_attendance (
	work_date DATE NOT NULL,
	employee_id BIGINT NOT NULL,
	status TEXT NOT NULL CHECK(status IN ('working','off','unplanned')),
	updated_by TEXT NOT NULL DEFAULT '',
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY(work_date, employee_id)
);
CREATE INDEX IF NOT EXISTS production_employee_attendance_employee_idx ON %[1]s.production_employee_attendance(employee_id,work_date);
CREATE TABLE IF NOT EXISTS %[1]s.production_workstation_overrides (
	work_date DATE NOT NULL,
	workstation_id BIGINT NOT NULL,
	employee_id BIGINT NOT NULL,
	reason TEXT NOT NULL DEFAULT '',
	updated_by TEXT NOT NULL DEFAULT '',
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY(work_date, workstation_id)
);
CREATE INDEX IF NOT EXISTS production_workstation_overrides_employee_idx ON %[1]s.production_workstation_overrides(employee_id,work_date);
CREATE TABLE IF NOT EXISTS %[1]s.production_workstation_handovers (
	id BIGSERIAL PRIMARY KEY,
	work_date DATE NOT NULL,
	workstation_id BIGINT NOT NULL,
	previous_employee_id BIGINT NOT NULL DEFAULT 0,
	new_employee_id BIGINT NOT NULL,
	job_card_ids_json JSONB NOT NULL DEFAULT '[]'::jsonb,
	previous_assignments_json JSONB NOT NULL DEFAULT '[]'::jsonb,
	request_id TEXT NOT NULL,
	operator TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE(operator,request_id)
);
ALTER TABLE %[1]s.production_workstation_handovers
	ADD COLUMN IF NOT EXISTS previous_assignments_json JSONB NOT NULL DEFAULT '[]'::jsonb;
CREATE TABLE IF NOT EXISTS %[1]s.production_roster_requests (
	actor TEXT NOT NULL,
	request_id TEXT NOT NULL,
	payload_hash TEXT NOT NULL,
	response_json JSONB NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY(actor,request_id)
);
`, schema))
	return err
}
