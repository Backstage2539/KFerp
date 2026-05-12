package messagecenter

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %[1]s.message_events (
	id BIGSERIAL PRIMARY KEY,
	event_key TEXT NOT NULL DEFAULT '',
	topic TEXT NOT NULL DEFAULT '',
	event_type TEXT NOT NULL DEFAULT '',
	source_type TEXT NOT NULL DEFAULT '',
	source_id BIGINT NOT NULL DEFAULT 0,
	actor TEXT NOT NULL DEFAULT '',
	title TEXT NOT NULL DEFAULT '',
	body TEXT NOT NULL DEFAULT '',
	tone TEXT NOT NULL DEFAULT 'info',
	payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS message_events_event_key_uq
	ON %[1]s.message_events(event_key)
	WHERE event_key <> '';
CREATE INDEX IF NOT EXISTS message_events_topic_created_idx
	ON %[1]s.message_events(topic, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS message_events_source_idx
	ON %[1]s.message_events(source_type, source_id);

CREATE TABLE IF NOT EXISTS %[1]s.message_deliveries (
	id BIGSERIAL PRIMARY KEY,
	event_id BIGINT NOT NULL REFERENCES %[1]s.message_events(id) ON DELETE CASCADE,
	channel TEXT NOT NULL DEFAULT 'erp_platform',
	target_type TEXT NOT NULL DEFAULT 'broadcast',
	target_key TEXT NOT NULL DEFAULT '',
	target_employee_id BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'ready',
	deliver_after TIMESTAMPTZ NULL,
	delivered_at TIMESTAMPTZ NULL,
	error TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS message_deliveries_target_uq
	ON %[1]s.message_deliveries(event_id, channel, target_type, target_key, target_employee_id);
CREATE INDEX IF NOT EXISTS message_deliveries_channel_status_idx
	ON %[1]s.message_deliveries(channel, status, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS %[1]s.message_reads (
	event_id BIGINT NOT NULL REFERENCES %[1]s.message_events(id) ON DELETE CASCADE,
	employee_id BIGINT NOT NULL REFERENCES %[1]s.company_employees(id) ON DELETE CASCADE,
	read_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY(event_id, employee_id)
);
`, schema)
	_, err := pool.Exec(ctx, q)
	return err
}
