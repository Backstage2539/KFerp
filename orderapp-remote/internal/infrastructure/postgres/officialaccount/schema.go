package officialaccount

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
 CREATE TABLE IF NOT EXISTS %[1]s.wechat_price_entries(
 entry_key text PRIMARY KEY DEFAULT md5(random()::text||clock_timestamp()::text),scope_key text NOT NULL UNIQUE,
 name text NOT NULL,publication_id bigint NOT NULL REFERENCES %[1]s.bean_list_publications(id),
 visibility text NOT NULL DEFAULT 'authenticated' CHECK(visibility IN ('public','authenticated')),
 enabled boolean NOT NULL DEFAULT true,revision bigint NOT NULL DEFAULT 1,updated_at timestamptz NOT NULL DEFAULT now());
 CREATE OR REPLACE FUNCTION %[1]s.wechat_price_scope(p %[1]s.bean_list_publications) RETURNS text LANGUAGE sql IMMUTABLE AS $$
 SELECT jsonb_build_array(p.owner_type,p.owner_key,COALESCE(NULLIF(p.publication_purpose,''),'factory_supply'),p.list_type,p.product_type_category_id,p.classification_template_id,p.classification_category_id,COALESCE(NULLIF(p.publication_table_key,''),NULLIF(p.config_json->'publication_batch'->>'table_key',''),'legacy:'||p.id::text))::text $$;
 CREATE OR REPLACE FUNCTION %[1]s.wechat_create_price_entry() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NEW.status='published' AND NEW.deleted_at IS NULL THEN
 INSERT INTO %[1]s.wechat_price_entries(scope_key,name,publication_id)
 VALUES(%[1]s.wechat_price_scope(NEW),COALESCE(NULLIF(NEW.publication_table_name,''),NULLIF(NEW.config_json->'publication_batch'->>'table_name',''),NULLIF(NEW.config_json->>'title',''),NULLIF(NEW.product_type_name,''),NEW.list_type),NEW.id)
 ON CONFLICT(scope_key) DO NOTHING; END IF; RETURN NEW; END $$;
 DROP TRIGGER IF EXISTS wechat_create_price_entry ON %[1]s.bean_list_publications;
 CREATE TRIGGER wechat_create_price_entry AFTER INSERT OR UPDATE OF status ON %[1]s.bean_list_publications FOR EACH ROW EXECUTE FUNCTION %[1]s.wechat_create_price_entry();
 INSERT INTO %[1]s.wechat_price_entries(scope_key,name,publication_id)
 SELECT DISTINCT ON (%[1]s.wechat_price_scope(p)) %[1]s.wechat_price_scope(p),COALESCE(NULLIF(p.publication_table_name,''),NULLIF(p.config_json->'publication_batch'->>'table_name',''),NULLIF(p.config_json->>'title',''),NULLIF(p.product_type_name,''),p.list_type),p.id
 FROM %[1]s.bean_list_publications p WHERE p.status='published' AND p.deleted_at IS NULL
 ORDER BY %[1]s.wechat_price_scope(p),p.published_at DESC,p.id DESC ON CONFLICT(scope_key) DO NOTHING;
 CREATE TABLE IF NOT EXISTS %[1]s.wechat_bindings(
 app_id text NOT NULL,openid text NOT NULL,mini_user_id bigint NOT NULL REFERENCES %[1]s.mini_users(id),
 customer_id bigint NOT NULL DEFAULT 0,active boolean NOT NULL DEFAULT true,bound_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(app_id,openid));
 CREATE INDEX IF NOT EXISTS wechat_bindings_user_idx ON %[1]s.wechat_bindings(app_id,mini_user_id);
 CREATE TABLE IF NOT EXISTS %[1]s.wechat_binding_codes(
 code_hash text PRIMARY KEY,app_id text NOT NULL,mini_user_id bigint NOT NULL REFERENCES %[1]s.mini_users(id),customer_id bigint NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),expires_at timestamptz NOT NULL,consumed_at timestamptz);
 CREATE TABLE IF NOT EXISTS %[1]s.wechat_binding_attempts(app_id text NOT NULL,openid text NOT NULL,bucket timestamptz NOT NULL DEFAULT now(),attempts int NOT NULL DEFAULT 0,PRIMARY KEY(app_id,openid));
 CREATE TABLE IF NOT EXISTS %[1]s.wechat_menu_versions(id bigserial PRIMARY KEY,app_id text NOT NULL,menu_json jsonb NOT NULL,status text NOT NULL,actor text NOT NULL,created_at timestamptz NOT NULL DEFAULT now());
 CREATE TABLE IF NOT EXISTS %[1]s.wechat_callback_events(app_id text NOT NULL,event_key text NOT NULL,expires_at timestamptz NOT NULL DEFAULT now()+interval '10 minutes',PRIMARY KEY(app_id,event_key));
 `, schema))
	return err
}
