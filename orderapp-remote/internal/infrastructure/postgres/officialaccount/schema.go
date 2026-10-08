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
 ALTER TABLE %[1]s.wechat_price_entries ADD COLUMN IF NOT EXISTS type_key text NOT NULL DEFAULT '';
 ALTER TABLE %[1]s.wechat_price_entries ADD COLUMN IF NOT EXISTS type_name text NOT NULL DEFAULT '';
 ALTER TABLE %[1]s.wechat_price_entries ADD COLUMN IF NOT EXISTS purpose text NOT NULL DEFAULT '';
 ALTER TABLE %[1]s.wechat_price_entries ALTER COLUMN publication_id DROP NOT NULL;
 CREATE UNIQUE INDEX IF NOT EXISTS wechat_price_entries_type_purpose_idx ON %[1]s.wechat_price_entries(type_key,purpose) WHERE type_key<>'';
 CREATE OR REPLACE FUNCTION %[1]s.wechat_price_scope(p %[1]s.bean_list_publications) RETURNS text LANGUAGE sql IMMUTABLE AS $$
 SELECT jsonb_build_array(p.owner_type,p.owner_key,COALESCE(NULLIF(p.publication_purpose,''),'factory_supply'),p.list_type,p.product_type_category_id,p.classification_template_id,p.classification_category_id,COALESCE(NULLIF(p.publication_table_key,''),NULLIF(p.config_json->'publication_batch'->>'table_key',''),'legacy:'||p.id::text))::text $$;
 CREATE OR REPLACE FUNCTION %[1]s.wechat_create_price_entry() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NEW.status='published' AND NEW.deleted_at IS NULL THEN
 INSERT INTO %[1]s.wechat_price_entries(scope_key,name,publication_id)
 VALUES(%[1]s.wechat_price_scope(NEW),COALESCE(NULLIF(NEW.publication_table_name,''),NULLIF(NEW.config_json->'publication_batch'->>'table_name',''),NULLIF(NEW.config_json->>'title',''),NULLIF(NEW.product_type_name,''),NEW.list_type),NEW.id)
 ON CONFLICT(scope_key) DO NOTHING; END IF; RETURN NEW; END $$;
 DROP TRIGGER IF EXISTS wechat_create_price_entry ON %[1]s.bean_list_publications;
 CREATE OR REPLACE FUNCTION %[1]s.wechat_price_type_key(p %[1]s.bean_list_publications) RETURNS text LANGUAGE sql IMMUTABLE AS $$
 SELECT CASE
   WHEN COALESCE(p.classification_template_id,0)>0 THEN 'classification-template:'||p.classification_template_id::text
   WHEN COALESCE(p.product_type_category_id,0)>0 THEN 'product-type:'||p.product_type_category_id::text
   ELSE '' END $$;
 CREATE OR REPLACE FUNCTION %[1]s.wechat_create_typed_price_entries() RETURNS trigger LANGUAGE plpgsql AS $$
 DECLARE resolved_type_key text; resolved_type_name text;
 BEGIN
   IF NEW.status='published' AND NEW.deleted_at IS NULL THEN
     resolved_type_key := %[1]s.wechat_price_type_key(NEW);
     resolved_type_name := COALESCE(NULLIF(BTRIM(NEW.classification_template_name),''),NULLIF(BTRIM(NEW.product_type_name),''),NULLIF(BTRIM(NEW.classification_category_name),''),NULLIF(BTRIM(NEW.publication_table_name),''),NULLIF(NEW.config_json->'publication_batch'->>'table_name',''),NULLIF(NEW.list_type,''),'价格表');
     IF resolved_type_key<>'' THEN
       INSERT INTO %[1]s.wechat_price_entries(entry_key,scope_key,type_key,type_name,purpose,name,publication_id,visibility,enabled)
       VALUES
         (md5('wechat-price-entry:'||resolved_type_key||':wholesale'),'type:'||resolved_type_key||':wholesale',resolved_type_key,resolved_type_name,'wholesale',resolved_type_name||' 批发',NULL,'authenticated',false),
         (md5('wechat-price-entry:'||resolved_type_key||':direct_ship'),'type:'||resolved_type_key||':direct_ship',resolved_type_key,resolved_type_name,'direct_ship',resolved_type_name||' 一件代发',NULL,'authenticated',false)
       ON CONFLICT(type_key,purpose) WHERE type_key<>'' DO UPDATE SET type_name=excluded.type_name,name=excluded.name,updated_at=now();
     END IF;
   END IF;
   RETURN NEW;
 END $$;
 DROP TRIGGER IF EXISTS wechat_create_typed_price_entries ON %[1]s.bean_list_publications;
 CREATE TRIGGER wechat_create_typed_price_entries AFTER INSERT OR UPDATE OF status ON %[1]s.bean_list_publications FOR EACH ROW EXECUTE FUNCTION %[1]s.wechat_create_typed_price_entries();
 INSERT INTO %[1]s.wechat_price_entries(entry_key,scope_key,type_key,type_name,purpose,name,publication_id,visibility,enabled)
 SELECT md5('wechat-price-entry:'||x.type_key||':'||u.purpose),'type:'||x.type_key||':'||u.purpose,x.type_key,x.type_name,u.purpose,x.type_name||CASE WHEN u.purpose='wholesale' THEN ' 批发' ELSE ' 一件代发' END,NULL,'authenticated',false
 FROM (
   SELECT DISTINCT ON (%[1]s.wechat_price_type_key(p)) %[1]s.wechat_price_type_key(p) AS type_key,
     COALESCE(NULLIF(BTRIM(p.product_type_name),''),NULLIF(BTRIM(p.classification_template_name),''),NULLIF(BTRIM(p.classification_category_name),''),NULLIF(BTRIM(p.publication_table_name),''),NULLIF(p.config_json->'publication_batch'->>'table_name',''),NULLIF(p.list_type,''),'价格表') AS type_name
   FROM %[1]s.bean_list_publications p
   WHERE p.status='published' AND p.deleted_at IS NULL AND %[1]s.wechat_price_type_key(p)<>''
   ORDER BY %[1]s.wechat_price_type_key(p),p.published_at DESC,p.id DESC
 ) x CROSS JOIN (VALUES('wholesale'),('direct_ship')) AS u(purpose)
 ON CONFLICT(type_key,purpose) WHERE type_key<>'' DO UPDATE SET type_name=excluded.type_name,name=excluded.name,updated_at=now();
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
