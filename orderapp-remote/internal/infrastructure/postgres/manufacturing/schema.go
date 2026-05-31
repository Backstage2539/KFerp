package manufacturing

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %[1]s.industry_field_templates (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL DEFAULT '',
	industry_key TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'active',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS industry_field_templates_status_idx
	ON %[1]s.industry_field_templates(status, updated_at DESC);

CREATE TABLE IF NOT EXISTS %[1]s.industry_field_definitions (
	id BIGSERIAL PRIMARY KEY,
	template_id BIGINT NOT NULL,
	field_key TEXT NOT NULL DEFAULT '',
	label TEXT NOT NULL DEFAULT '',
	field_type TEXT NOT NULL DEFAULT 'text',
	unit TEXT NOT NULL DEFAULT '',
	required BOOLEAN NOT NULL DEFAULT false,
	options_json JSONB NOT NULL DEFAULT '[]'::jsonb,
	sort_order INT NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS industry_field_definitions_template_key_uq
	ON %[1]s.industry_field_definitions(template_id, field_key);
CREATE INDEX IF NOT EXISTS industry_field_definitions_template_idx
	ON %[1]s.industry_field_definitions(template_id, sort_order, id);

CREATE TABLE IF NOT EXISTS %[1]s.process_templates (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL DEFAULT '',
	product_id BIGINT NOT NULL DEFAULT 0,
	bom_version_id BIGINT NOT NULL DEFAULT 0,
	industry_template_id BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'draft',
	default_equipment TEXT NOT NULL DEFAULT '',
	default_minutes INT NOT NULL DEFAULT 0,
	key_params_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	note TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS process_templates_product_status_idx
	ON %[1]s.process_templates(product_id, status, updated_at DESC);
CREATE INDEX IF NOT EXISTS process_templates_bom_version_idx
	ON %[1]s.process_templates(bom_version_id);
CREATE INDEX IF NOT EXISTS process_templates_industry_template_idx
	ON %[1]s.process_templates(industry_template_id);

CREATE TABLE IF NOT EXISTS %[1]s.process_template_operations (
	id BIGSERIAL PRIMARY KEY,
	template_id BIGINT NOT NULL,
	seq INT NOT NULL DEFAULT 0,
	operation TEXT NOT NULL DEFAULT '',
	workstation TEXT NOT NULL DEFAULT '',
	default_equipment TEXT NOT NULL DEFAULT '',
	default_minutes INT NOT NULL DEFAULT 0,
	records_loss BOOLEAN NOT NULL DEFAULT false,
	parameter_schema_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	quality_checklist_json JSONB NOT NULL DEFAULT '[]'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS process_template_operations_template_seq_uq
	ON %[1]s.process_template_operations(template_id, seq);
CREATE INDEX IF NOT EXISTS process_template_operations_template_idx
	ON %[1]s.process_template_operations(template_id, seq, id);

CREATE TABLE IF NOT EXISTS %[1]s.process_routes (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'draft',
	default_equipment TEXT NOT NULL DEFAULT '',
	default_minutes INT NOT NULL DEFAULT 0,
	note TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS process_routes_status_idx
	ON %[1]s.process_routes(status, updated_at DESC);

CREATE TABLE IF NOT EXISTS %[1]s.process_route_operations (
	id BIGSERIAL PRIMARY KEY,
	route_id BIGINT NOT NULL,
	seq INT NOT NULL DEFAULT 0,
	operation TEXT NOT NULL DEFAULT '',
	workstation TEXT NOT NULL DEFAULT '',
	default_equipment TEXT NOT NULL DEFAULT '',
	default_minutes INT NOT NULL DEFAULT 0,
	records_loss BOOLEAN NOT NULL DEFAULT false,
	quality_checklist_json JSONB NOT NULL DEFAULT '[]'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS process_route_operations_route_seq_uq
	ON %[1]s.process_route_operations(route_id, seq);
CREATE INDEX IF NOT EXISTS process_route_operations_route_idx
	ON %[1]s.process_route_operations(route_id, seq, id);
`, schema)
	_, err := pool.Exec(ctx, q)
	return err
}
