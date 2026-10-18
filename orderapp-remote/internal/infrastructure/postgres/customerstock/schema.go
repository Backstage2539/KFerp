// Package customerstock keeps retained sales promises separate from physical stock.
package customerstock

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
 ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS delivery_mode TEXT NOT NULL DEFAULT 'ship';
 ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS delivery_warehouse_code TEXT NOT NULL DEFAULT '';
 ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS delivery_status TEXT NOT NULL DEFAULT '';
 ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS fulfillment_mode TEXT NOT NULL DEFAULT '';
 CREATE TABLE IF NOT EXISTS %[1]s.customer_stock_entitlements(
 id BIGSERIAL PRIMARY KEY,source_order_id BIGINT NOT NULL REFERENCES %[1]s.orders(id),
 source_order_item_id BIGINT NOT NULL UNIQUE REFERENCES %[1]s.order_items(id),customer_id BIGINT NOT NULL,
 product_id BIGINT NOT NULL,bom_spec_id BIGINT NOT NULL DEFAULT 0,bom_variant_id BIGINT NOT NULL DEFAULT 0,
 spec_g BIGINT NOT NULL DEFAULT 0,warehouse_code TEXT NOT NULL,
 promised_qty BIGINT NOT NULL CHECK(promised_qty>0),received_qty BIGINT NOT NULL DEFAULT 0 CHECK(received_qty>=0),
 status TEXT NOT NULL DEFAULT 'active',created_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 CHECK(received_qty<=promised_qty));
 CREATE INDEX IF NOT EXISTS customer_stock_entitlements_customer ON %[1]s.customer_stock_entitlements(customer_id,warehouse_code,status);
 CREATE TABLE IF NOT EXISTS %[1]s.customer_stock_supply_reservations(
 id BIGSERIAL PRIMARY KEY,entitlement_id BIGINT NOT NULL REFERENCES %[1]s.customer_stock_entitlements(id),
 customer_id BIGINT NOT NULL,request_id BIGINT NOT NULL,request_item_id BIGINT NOT NULL,
 order_id BIGINT NOT NULL REFERENCES %[1]s.orders(id),order_item_id BIGINT NOT NULL REFERENCES %[1]s.order_items(id),
 reserved_qty BIGINT NOT NULL CHECK(reserved_qty>0),converted_qty BIGINT NOT NULL DEFAULT 0,released_qty BIGINT NOT NULL DEFAULT 0,
 status TEXT NOT NULL DEFAULT 'reserved',created_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(entitlement_id,order_item_id),CHECK(converted_qty>=0 AND released_qty>=0 AND converted_qty+released_qty<=reserved_qty));
 CREATE INDEX IF NOT EXISTS customer_stock_supply_reservations_source ON %[1]s.customer_stock_supply_reservations(entitlement_id,status);
 CREATE TABLE IF NOT EXISTS %[1]s.customer_stock_entitlement_receipts(
 id BIGSERIAL PRIMARY KEY,entitlement_id BIGINT NOT NULL REFERENCES %[1]s.customer_stock_entitlements(id),
 receipt_key TEXT NOT NULL,running_item_id BIGINT NOT NULL,stock_batch_id BIGINT NOT NULL,batch_code TEXT NOT NULL,
 warehouse_code TEXT NOT NULL,quantity BIGINT NOT NULL CHECK(quantity>0),created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(entitlement_id,receipt_key));
 CREATE OR REPLACE FUNCTION %[1]s.customer_stock_has_dependencies(target BIGINT) RETURNS BOOLEAN LANGUAGE plpgsql AS $fn$
 DECLARE depended BOOLEAN; nos TEXT;
 BEGIN
 SELECT EXISTS(SELECT 1 FROM %[1]s.customer_stock_entitlements e WHERE e.source_order_id=target AND
  (e.received_qty>0 OR EXISTS(SELECT 1 FROM %[1]s.customer_stock_supply_reservations r WHERE r.entitlement_id=e.id AND r.reserved_qty>r.released_qty))) INTO depended;
 IF depended THEN RETURN true; END IF;
 SELECT order_no INTO nos FROM %[1]s.orders WHERE id=target;
 IF to_regclass('%[1]s.work_orders') IS NOT NULL THEN
  EXECUTE 'SELECT EXISTS(SELECT 1 FROM %[1]s.work_orders w WHERE $1=ANY(string_to_array(replace(w.order_nos,'' '',''''),'','')) AND w.status<>''cancelled'')' INTO depended USING nos;
  IF depended THEN RETURN true; END IF;
 END IF;
 IF to_regclass('%[1]s.production_plan_items') IS NOT NULL THEN
  EXECUTE 'SELECT EXISTS(SELECT 1 FROM %[1]s.production_plan_items i JOIN %[1]s.production_plans p ON p.id=i.production_plan_id WHERE $1=ANY(string_to_array(replace(i.order_nos,'' '',''''),'','')) AND p.status<>''cancelled'')' INTO depended USING nos;
 END IF;
 RETURN COALESCE(depended,false);
 END $fn$;
 CREATE OR REPLACE FUNCTION %[1]s.customer_stock_guard_header() RETURNS trigger LANGUAGE plpgsql AS $fn$
 BEGIN
 IF OLD.delivery_mode='customer_stock' AND (
  NEW.customer_id IS DISTINCT FROM OLD.customer_id OR NEW.delivery_mode IS DISTINCT FROM OLD.delivery_mode OR
  NEW.delivery_warehouse_code IS DISTINCT FROM OLD.delivery_warehouse_code OR NEW.is_void IS DISTINCT FROM OLD.is_void OR
  COALESCE(to_jsonb(NEW)->>'confirmation_status','accepted') IS DISTINCT FROM COALESCE(to_jsonb(OLD)->>'confirmation_status','accepted'))
  AND %[1]s.customer_stock_has_dependencies(OLD.id) THEN
  RAISE EXCEPTION '留厂销售已有代发预订、生产或入库，须先处理关联单据，不能修改或作废';
 END IF;
 RETURN NEW;
 END $fn$;
 DROP TRIGGER IF EXISTS customer_stock_guard_header ON %[1]s.orders;
 CREATE TRIGGER customer_stock_guard_header BEFORE UPDATE ON %[1]s.orders FOR EACH ROW EXECUTE FUNCTION %[1]s.customer_stock_guard_header();
 CREATE OR REPLACE FUNCTION %[1]s.customer_stock_frozen_spec_g(item JSONB) RETURNS BIGINT LANGUAGE plpgsql AS $fn$
 DECLARE source JSONB; spec JSONB; amount NUMERIC; unit TEXT; parts TEXT[];
 BEGIN
 source:=COALESCE(item->'price_source_json','{}'::jsonb);spec:=source->'effective_sales_spec';
 amount:=COALESCE((spec->>'net_content_qty')::numeric,0);unit:=lower(COALESCE(spec->>'net_content_unit',''));
 IF amount<=0 THEN spec:=source->'production_quantity_snapshot';amount:=COALESCE((spec->>'inventory_qty_per_sales_unit')::numeric,0);unit:=lower(COALESCE(spec->>'inventory_unit',''));END IF;
 IF amount>0 AND unit IN('g','克','kg','千克','公斤','lb','磅') THEN
 RETURN round(amount*CASE WHEN unit IN('kg','千克','公斤') THEN 1000 WHEN unit IN('lb','磅') THEN 454 ELSE 1 END)::bigint;
 END IF;
 parts:=regexp_match(lower(COALESCE(item->>'spec','')),'([0-9]+(?:[.][0-9]+)?)[[:space:]]*(kg|lb|g|千克|公斤|克|磅)');
 IF parts IS NOT NULL THEN RETURN round(parts[1]::numeric*CASE WHEN parts[2] IN('kg','千克','公斤') THEN 1000 WHEN parts[2] IN('lb','磅') THEN 454 ELSE 1 END)::bigint;END IF;
 RETURN 0;
 END $fn$;
 CREATE OR REPLACE FUNCTION %[1]s.customer_stock_sync_order(target BIGINT) RETURNS VOID LANGUAGE plpgsql AS $fn$
 DECLARE doc RECORD; dest RECORD;
 BEGIN
 SELECT o.* INTO doc FROM %[1]s.orders o WHERE id=target;
 IF NOT FOUND THEN RETURN; END IF;
 IF doc.delivery_mode NOT IN ('ship','customer_stock') THEN RAISE EXCEPTION '交付方式无效'; END IF;
 IF doc.delivery_mode<>'customer_stock' OR COALESCE((to_jsonb(doc)->>'is_void')::boolean,false) OR COALESCE(to_jsonb(doc)->>'confirmation_status','accepted')<>'accepted' THEN
  UPDATE %[1]s.customer_stock_entitlements SET status='cancelled',updated_at=now() WHERE source_order_id=target AND status='active';
  RETURN;
 END IF;
 SELECT * INTO dest FROM %[1]s.warehouses WHERE code=doc.delivery_warehouse_code AND active=true;
 IF NOT FOUND OR COALESCE(dest.customer_id,0)<>COALESCE(doc.customer_id,0) OR COALESCE(doc.customer_id,0)<=0 OR dest.kind NOT IN ('finished','customer_processing','customer_finished','customer') THEN
  RAISE EXCEPTION '留厂交付必须选择当前客户的有效成品仓';
 END IF;
 IF EXISTS(SELECT 1 FROM %[1]s.order_items WHERE order_id=target AND (COALESCE(qty,0)<=0 OR qty<>trunc(qty) OR COALESCE(product_id,0)<=0)) THEN
  RAISE EXCEPTION '留厂交付商品必须填写正整数库存数量';
 END IF;
 IF EXISTS(SELECT 1 FROM %[1]s.order_items i WHERE i.order_id=target AND COALESCE((to_jsonb(i)->>'bom_spec_id')::bigint,0)=0 AND %[1]s.customer_stock_frozen_spec_g(to_jsonb(i))<=0) THEN RAISE EXCEPTION '留厂销售缺少冻结的定量商品规格';END IF;
 INSERT INTO %[1]s.customer_stock_entitlements(source_order_id,source_order_item_id,customer_id,product_id,bom_spec_id,bom_variant_id,spec_g,warehouse_code,promised_qty)
 SELECT target,i.id,doc.customer_id,i.product_id,COALESCE((to_jsonb(i)->>'bom_spec_id')::bigint,0),COALESCE((to_jsonb(i)->>'bom_variant_id')::bigint,0),
 CASE WHEN COALESCE((to_jsonb(i)->>'bom_spec_id')::bigint,0)>0 THEN 0 ELSE %[1]s.customer_stock_frozen_spec_g(to_jsonb(i)) END,
 doc.delivery_warehouse_code,i.qty::bigint FROM %[1]s.order_items i JOIN %[1]s.products p ON p.id=i.product_id WHERE i.order_id=target
 ON CONFLICT(source_order_item_id) DO UPDATE SET promised_qty=EXCLUDED.promised_qty,warehouse_code=EXCLUDED.warehouse_code,customer_id=EXCLUDED.customer_id,
 product_id=EXCLUDED.product_id,bom_spec_id=EXCLUDED.bom_spec_id,bom_variant_id=EXCLUDED.bom_variant_id,spec_g=EXCLUDED.spec_g,status='active',updated_at=now();
 END $fn$;
 CREATE OR REPLACE FUNCTION %[1]s.customer_stock_sync_header() RETURNS trigger LANGUAGE plpgsql AS $fn$
 BEGIN PERFORM %[1]s.customer_stock_sync_order(NEW.id); RETURN NEW; END $fn$;
 DROP TRIGGER IF EXISTS customer_stock_sync_header ON %[1]s.orders;
 CREATE TRIGGER customer_stock_sync_header AFTER INSERT OR UPDATE OF delivery_mode,delivery_warehouse_code,customer_id,is_void,confirmation_status ON %[1]s.orders FOR EACH ROW EXECUTE FUNCTION %[1]s.customer_stock_sync_header();
 CREATE OR REPLACE FUNCTION %[1]s.customer_stock_guard_item() RETURNS trigger LANGUAGE plpgsql AS $fn$
 BEGIN
 IF TG_OP='UPDATE' AND NEW.product_id IS NOT DISTINCT FROM OLD.product_id AND NEW.qty IS NOT DISTINCT FROM OLD.qty AND NEW.spec IS NOT DISTINCT FROM OLD.spec AND NEW.unit IS NOT DISTINCT FROM OLD.unit AND (to_jsonb(NEW)->'bom_spec_id') IS NOT DISTINCT FROM (to_jsonb(OLD)->'bom_spec_id') AND (to_jsonb(NEW)->'bom_variant_id') IS NOT DISTINCT FROM (to_jsonb(OLD)->'bom_variant_id') THEN RETURN NEW; END IF;
 IF EXISTS(SELECT 1 FROM %[1]s.customer_stock_entitlements WHERE source_order_item_id=OLD.id) THEN
  IF %[1]s.customer_stock_has_dependencies(OLD.order_id) THEN RAISE EXCEPTION '留厂销售已有关联代发、生产或入库，不能修改商品数量'; END IF;
  DELETE FROM %[1]s.customer_stock_supply_reservations WHERE entitlement_id IN(SELECT id FROM %[1]s.customer_stock_entitlements WHERE source_order_item_id=OLD.id);
  DELETE FROM %[1]s.customer_stock_entitlements WHERE source_order_item_id=OLD.id;
 END IF;
 IF TG_OP='DELETE' THEN RETURN OLD; END IF; RETURN NEW;
 END $fn$;
 DROP TRIGGER IF EXISTS customer_stock_guard_item ON %[1]s.order_items;
 CREATE TRIGGER customer_stock_guard_item BEFORE UPDATE OR DELETE ON %[1]s.order_items FOR EACH ROW EXECUTE FUNCTION %[1]s.customer_stock_guard_item();
 CREATE OR REPLACE FUNCTION %[1]s.customer_stock_sync_item() RETURNS trigger LANGUAGE plpgsql AS $fn$
 BEGIN PERFORM %[1]s.customer_stock_sync_order(NEW.order_id); RETURN NEW; END $fn$;
 DROP TRIGGER IF EXISTS customer_stock_sync_item ON %[1]s.order_items;
 CREATE TRIGGER customer_stock_sync_item AFTER INSERT OR UPDATE ON %[1]s.order_items FOR EACH ROW EXECUTE FUNCTION %[1]s.customer_stock_sync_item();
 `, schema))
	if err != nil {
		return err
	}
	return ensureShippingGuards(ctx, pool, schema)
}
