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
	template_key TEXT NOT NULL DEFAULT '',
	adapter_key TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'ready',
	attempts INT NOT NULL DEFAULT 0,
	deliver_after TIMESTAMPTZ NULL,
	delivered_at TIMESTAMPTZ NULL,
	error TEXT NOT NULL DEFAULT '',
	request_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	response_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS message_deliveries_target_uq
	ON %[1]s.message_deliveries(event_id, channel, target_type, target_key, target_employee_id);
CREATE INDEX IF NOT EXISTS message_deliveries_channel_status_idx
	ON %[1]s.message_deliveries(channel, status, created_at DESC, id DESC);
ALTER TABLE %[1]s.message_deliveries ADD COLUMN IF NOT EXISTS template_key TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.message_deliveries ADD COLUMN IF NOT EXISTS adapter_key TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.message_deliveries ADD COLUMN IF NOT EXISTS attempts INT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.message_deliveries ADD COLUMN IF NOT EXISTS request_json JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE %[1]s.message_deliveries ADD COLUMN IF NOT EXISTS response_json JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE TABLE IF NOT EXISTS %[1]s.message_reads (
	event_id BIGINT NOT NULL REFERENCES %[1]s.message_events(id) ON DELETE CASCADE,
	employee_id BIGINT NOT NULL REFERENCES %[1]s.company_employees(id) ON DELETE CASCADE,
	read_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY(event_id, employee_id)
);

CREATE TABLE IF NOT EXISTS %[1]s.message_notification_rules (
	id BIGSERIAL PRIMARY KEY,
	code TEXT NOT NULL UNIQUE,
	name TEXT NOT NULL DEFAULT '',
	enabled BOOLEAN NOT NULL DEFAULT true,
	topic TEXT NOT NULL DEFAULT '',
	event_type TEXT NOT NULL DEFAULT '',
	source_type TEXT NOT NULL DEFAULT '',
	channel TEXT NOT NULL DEFAULT 'erp_platform',
	target_type TEXT NOT NULL DEFAULT 'broadcast',
	target_key TEXT NOT NULL DEFAULT '',
	target_employee_id BIGINT NOT NULL DEFAULT 0,
	template_key TEXT NOT NULL DEFAULT '',
	adapter_key TEXT NOT NULL DEFAULT '',
	tone TEXT NOT NULL DEFAULT 'info',
	payload_match_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS message_notification_rules_event_idx
	ON %[1]s.message_notification_rules(event_type, topic, source_type, enabled);

CREATE TABLE IF NOT EXISTS %[1]s.message_channel_identities (
	id BIGSERIAL PRIMARY KEY,
	subject_type TEXT NOT NULL DEFAULT '',
	subject_id BIGINT NOT NULL DEFAULT 0,
	channel TEXT NOT NULL DEFAULT '',
	external_id TEXT NOT NULL DEFAULT '',
	enabled BOOLEAN NOT NULL DEFAULT true,
	meta_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE(subject_type, subject_id, channel, external_id)
);
CREATE INDEX IF NOT EXISTS message_channel_identities_subject_idx
	ON %[1]s.message_channel_identities(subject_type, subject_id, channel, enabled);
`, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	return seedDefaultNotificationRules(ctx, pool, schema)
}

func seedDefaultNotificationRules(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
INSERT INTO %[1]s.message_notification_rules(code,name,enabled,topic,event_type,source_type,channel,target_type,target_key,template_key,adapter_key,tone)
VALUES
	('order_created_orders_read','新订单通知订单读取用户',true,'orders','order.created','order','erp_platform','permission','orders.read','','','success'),
	('order_created_production_role','新订单通知烘焙师',true,'orders','order.created','order','erp_platform','role','production','','','success'),
	('order_production_finished_sales_role','生产完成通知销售',true,'orders','order.production_finished','order','erp_platform','role','sales','','','success'),
	('order_shipped_sales_role','已发货通知销售',true,'orders','order.shipped','order','erp_platform','role','sales','','','info'),
	('order_shipped_customer_external_im','已发货外部 IM 通知客户',false,'orders','order.shipped','order','external_im','order_customer','customer','order_shipped_customer','external_im','info')
ON CONFLICT(code) DO UPDATE SET
	name=excluded.name,
	topic=excluded.topic,
	event_type=excluded.event_type,
	source_type=excluded.source_type,
	channel=excluded.channel,
	target_type=excluded.target_type,
	target_key=excluded.target_key,
	template_key=excluded.template_key,
	adapter_key=excluded.adapter_key,
	tone=excluded.tone,
	updated_at=now()
`, schema)
	_, err := pool.Exec(ctx, q)
	return err
}
