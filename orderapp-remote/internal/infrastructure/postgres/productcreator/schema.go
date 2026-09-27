package productcreator

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	statements := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.business_templates (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published','disabled')),
			revision BIGINT NOT NULL DEFAULT 1,
			published_version BIGINT NOT NULL DEFAULT 0,
			draft_graph JSONB NOT NULL DEFAULT '{"nodes":[],"edges":[]}'::jsonb,
			created_by TEXT NOT NULL DEFAULT '',
			updated_by TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.business_template_versions (
			id BIGSERIAL PRIMARY KEY,
			template_id BIGINT NOT NULL REFERENCES %s.business_templates(id),
			version BIGINT NOT NULL,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			workflow JSONB NOT NULL,
			published_by TEXT NOT NULL DEFAULT '',
			published_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			UNIQUE(template_id,version)
		)`, schema, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.product_creator_runs (
			id BIGSERIAL PRIMARY KEY,
			template_id BIGINT NOT NULL REFERENCES %s.business_templates(id),
			template_version BIGINT NOT NULL,
			status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','config_committed','in_progress','completed','failed')),
			revision BIGINT NOT NULL DEFAULT 1,
			workflow_snapshot JSONB NOT NULL,
			inputs JSONB NOT NULL DEFAULT '{}'::jsonb,
			preview JSONB,
			commit_key TEXT NOT NULL DEFAULT '',
			commit_hash TEXT NOT NULL DEFAULT '',
			commit_result JSONB,
			created_by TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schema, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.product_creator_run_steps (
			id BIGSERIAL PRIMARY KEY,
			run_id BIGINT NOT NULL REFERENCES %s.product_creator_runs(id) ON DELETE CASCADE,
			node_id TEXT NOT NULL,
			row_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','ready','running','succeeded','failed','skipped')),
			input_json JSONB NOT NULL DEFAULT '{}'::jsonb,
			output_json JSONB,
			idempotency_key TEXT NOT NULL DEFAULT '',
			request_hash TEXT NOT NULL DEFAULT '',
			error_json JSONB,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			UNIQUE(run_id,node_id,row_id)
		)`, schema, schema),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS product_creator_runs_template_idx ON %s.product_creator_runs(template_id,created_at DESC)`, schema),
		fmt.Sprintf(`DROP INDEX IF EXISTS %s.product_creator_runs_commit_key_idx`, schema),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS product_creator_steps_run_idx ON %s.product_creator_run_steps(run_id,node_id,status)`, schema),
		fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS product_creator_steps_idempotency_idx ON %s.product_creator_run_steps(run_id,idempotency_key) WHERE idempotency_key<>''`, schema),
	}
	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}
