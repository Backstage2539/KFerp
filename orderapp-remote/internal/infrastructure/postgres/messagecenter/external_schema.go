package messagecenter

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Installed after all business schema migrations. Deferred order triggers capture
// final transaction state, including shipment changes made by other triggers.
func EnsureExternalSchema(ctx context.Context, p *pgxpool.Pool, s string) error {
	if err := EnsureBusinessEventSchema(ctx, p, s); err != nil {
		return err
	}
	_, err := p.Exec(ctx, fmt.Sprintf(`
 CREATE TABLE IF NOT EXISTS %[1]s.message_external_settings(id INT PRIMARY KEY CHECK(id=1),config JSONB NOT NULL DEFAULT '{}',updated_at TIMESTAMPTZ NOT NULL DEFAULT now());
 INSERT INTO %[1]s.message_external_settings(id) VALUES(1) ON CONFLICT DO NOTHING;
 CREATE TABLE IF NOT EXISTS %[1]s.message_order_changes(
 id BIGSERIAL PRIMARY KEY,order_id BIGINT NOT NULL,transaction_id BIGINT NOT NULL,created BOOLEAN NOT NULL,
 before_state JSONB NOT NULL,after_state JSONB NOT NULL,snapshot JSONB NOT NULL,occurred_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 processed_at TIMESTAMPTZ,UNIQUE(order_id,transaction_id));
 CREATE INDEX IF NOT EXISTS message_order_changes_pending ON %[1]s.message_order_changes(id) WHERE processed_at IS NULL;
 ALTER TABLE %[1]s.message_deliveries ADD COLUMN IF NOT EXISTS lifecycle BOOLEAN NOT NULL DEFAULT false;
 ALTER TABLE %[1]s.message_deliveries ADD COLUMN IF NOT EXISTS lease_token TEXT NOT NULL DEFAULT '';
 ALTER TABLE %[1]s.message_deliveries ADD COLUMN IF NOT EXISTS lease_until TIMESTAMPTZ;
 ALTER TABLE %[1]s.message_deliveries ADD COLUMN IF NOT EXISTS template_fingerprint TEXT NOT NULL DEFAULT '';
 CREATE TABLE IF NOT EXISTS %[1]s.message_business_test_verifications(event TEXT NOT NULL,status_value TEXT NOT NULL,template_fingerprint TEXT NOT NULL,delivery_id BIGINT NOT NULL UNIQUE,verified_by TEXT NOT NULL,verified_at TIMESTAMPTZ NOT NULL DEFAULT now(),PRIMARY KEY(event,status_value,template_fingerprint));
 ALTER TABLE %[1]s.message_business_test_verifications ADD COLUMN IF NOT EXISTS detail_verified BOOLEAN NOT NULL DEFAULT false;
 ALTER TABLE %[1]s.message_deliveries ADD COLUMN IF NOT EXISTS provider_message_id TEXT NOT NULL DEFAULT '';
 ALTER TABLE %[1]s.message_deliveries ADD COLUMN IF NOT EXISTS recipient_json JSONB NOT NULL DEFAULT '{}';
 ALTER TABLE %[1]s.message_deliveries ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
 CREATE TABLE IF NOT EXISTS %[1]s.message_provider_receipts(app_id TEXT NOT NULL,msg_id TEXT NOT NULL,status TEXT NOT NULL,received_at TIMESTAMPTZ NOT NULL DEFAULT now(),PRIMARY KEY(app_id,msg_id));
 CREATE INDEX IF NOT EXISTS message_delivery_provider ON %[1]s.message_deliveries(provider_message_id) WHERE provider_message_id<>'';
 ALTER TABLE %[1]s.wechat_bindings ADD COLUMN IF NOT EXISTS employee_id BIGINT NOT NULL DEFAULT 0;
 ALTER TABLE %[1]s.wechat_bindings ADD COLUMN IF NOT EXISTS subscribed BOOLEAN NOT NULL DEFAULT false;
 ALTER TABLE %[1]s.wechat_bindings ADD COLUMN IF NOT EXISTS notifications_enabled BOOLEAN NOT NULL DEFAULT false;
 ALTER TABLE %[1]s.wechat_bindings ADD COLUMN IF NOT EXISTS notification_events JSONB NOT NULL DEFAULT '["created","paid","production_started","production_finished","shipped","cancelled"]';
 ALTER TABLE %[1]s.wechat_bindings ADD COLUMN IF NOT EXISTS notification_consent_at TIMESTAMPTZ;
 ALTER TABLE %[1]s.wechat_bindings ADD COLUMN IF NOT EXISTS notification_subscription_time BIGINT NOT NULL DEFAULT 0;
 CREATE TABLE IF NOT EXISTS %[1]s.message_desktop_devices(
 id BIGSERIAL PRIMARY KEY,name TEXT NOT NULL,group_name TEXT NOT NULL,token_hash TEXT NOT NULL UNIQUE,
 enabled BOOLEAN NOT NULL DEFAULT true,verified BOOLEAN NOT NULL DEFAULT false,last_seen TIMESTAMPTZ,
 health TEXT NOT NULL DEFAULT '未连接',created_at TIMESTAMPTZ NOT NULL DEFAULT now());
 ALTER TABLE %[1]s.message_desktop_devices ADD COLUMN IF NOT EXISTS fingerprint TEXT NOT NULL DEFAULT '';
 CREATE TABLE IF NOT EXISTS %[1]s.message_notification_oauth(
 state_hash TEXT PRIMARY KEY,mini_user_id BIGINT NOT NULL,employee_id BIGINT NOT NULL DEFAULT 0,customer_id BIGINT NOT NULL DEFAULT 0,
 expires_at TIMESTAMPTZ NOT NULL,consumed_at TIMESTAMPTZ,app_id TEXT NOT NULL DEFAULT '',openid TEXT NOT NULL DEFAULT '');
 CREATE OR REPLACE FUNCTION %[1]s.message_order_state(row_data JSONB) RETURNS JSONB LANGUAGE sql STABLE AS $fn$
 SELECT jsonb_build_object('payment',COALESCE((SELECT name FROM %[1]s.pay_statuses WHERE id=(row_data->>'pay_status_id')::bigint),''),
 'shipping',COALESCE((SELECT name FROM %[1]s.ship_statuses WHERE id=(row_data->>'ship_status_id')::bigint),''),
 'production',COALESCE((SELECT name FROM %[1]s.order_process_statuses WHERE id=(row_data->>'process_status_id')::bigint),''),
 'void',COALESCE((row_data->>'is_void')::boolean,false),'settlement_reason',COALESCE(row_data->>'settlement_reason','')) $fn$;
 CREATE OR REPLACE FUNCTION %[1]s.capture_order_notification() RETURNS trigger LANGUAGE plpgsql AS $fn$
 DECLARE current_row JSONB;before_value JSONB;after_value JSONB;customer_row JSONB;details TEXT;data JSONB;
 BEGIN
 SELECT to_jsonb(o) INTO current_row FROM %[1]s.orders o WHERE o.id=NEW.id;
 IF current_row IS NULL THEN RETURN NULL; END IF;
 before_value:=CASE WHEN TG_OP='INSERT' THEN '{}'::jsonb ELSE %[1]s.message_order_state(to_jsonb(OLD)) END;
 after_value:=%[1]s.message_order_state(current_row);
 -- Keep the first OLD state even for a net-zero transaction; later deferred triggers must not manufacture a milestone.
 SELECT to_jsonb(c) INTO customer_row FROM %[1]s.customers c WHERE c.id=(current_row->>'customer_id')::bigint;
 SELECT string_agg(item_name||' '||COALESCE(spec,'')||' × '||qty::text||COALESCE(unit,''),'；' ORDER BY id) INTO details
 FROM (SELECT * FROM %[1]s.order_items WHERE order_id=NEW.id ORDER BY id LIMIT 10) i;
 data:=jsonb_build_object('order_id',NEW.id,'order_no',COALESCE(current_row->>'order_no',''),'customer_id',COALESCE((current_row->>'customer_id')::bigint,0),
 'customer_name',COALESCE(customer_row->>'name',''),'items',left(COALESCE(details,''),800),
 'responsible_id',CASE WHEN current_row->>'responsible_party_type'='employee' THEN COALESCE((current_row->>'responsible_party_id')::bigint,0) ELSE COALESCE((customer_row->>'responsible_employee_id')::bigint,0) END,
 'responsible_name',COALESCE(current_row->>'responsible_party_name',''),'due_date',COALESCE(current_row->>'delivery_date',current_row->>'expected_delivery_date',''),
 'occurred_at',to_char(clock_timestamp() AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD HH24:MI:SS'));
 INSERT INTO %[1]s.message_order_changes(order_id,transaction_id,created,before_state,after_state,snapshot)
 VALUES(NEW.id,txid_current(),TG_OP='INSERT',before_value,after_value,data)
 ON CONFLICT(order_id,transaction_id) DO UPDATE SET after_state=excluded.after_state,snapshot=excluded.snapshot;
 RETURN NULL;
 END $fn$;
 DROP TRIGGER IF EXISTS capture_order_notification ON %[1]s.orders;
 CREATE CONSTRAINT TRIGGER capture_order_notification AFTER INSERT ON %[1]s.orders
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION %[1]s.capture_order_notification();
 DROP TRIGGER IF EXISTS capture_order_notification_update ON %[1]s.orders;
 CREATE CONSTRAINT TRIGGER capture_order_notification_update AFTER UPDATE ON %[1]s.orders
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN (OLD.pay_status_id IS DISTINCT FROM NEW.pay_status_id OR OLD.process_status_id IS DISTINCT FROM NEW.process_status_id OR OLD.ship_status_id IS DISTINCT FROM NEW.ship_status_id OR OLD.is_void IS DISTINCT FROM NEW.is_void OR to_jsonb(OLD)->'settlement_reason' IS DISTINCT FROM to_jsonb(NEW)->'settlement_reason')
 EXECUTE FUNCTION %[1]s.capture_order_notification();
 `, s))
	return err
}
