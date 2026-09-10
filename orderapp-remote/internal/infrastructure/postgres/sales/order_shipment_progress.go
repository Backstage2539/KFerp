package sales

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Shipment progress follows cumulative stock issues for the entire accepted
// order. Ordinary ERP orders retain their existing shipping workflow.
func ensureOrderShipmentProgressSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
 CREATE OR REPLACE FUNCTION %[1]s.fulfillment_shipment_progress(target BIGINT) RETURNS TEXT LANGUAGE plpgsql AS $fn$
 DECLARE any_shipped BOOLEAN; all_shipped BOOLEAN;
 BEGIN
   IF NOT EXISTS(SELECT 1 FROM %[1]s.orders WHERE id=target AND confirmation_required AND confirmation_status='accepted') THEN RETURN NULL; END IF;
   WITH need AS (
     SELECT product_id,bom_spec_id,SUM(qty) AS units FROM %[1]s.order_items WHERE order_id=target GROUP BY product_id,bom_spec_id
   ), issued AS (
     SELECT product_id,bom_spec_id,SUM(deducted_units) AS units FROM %[1]s.order_stock_deductions WHERE order_id=target GROUP BY product_id,bom_spec_id
   ) SELECT BOOL_OR(COALESCE(i.units,0)>0),BOOL_AND(n.bom_spec_id>0 AND COALESCE(i.units,0)>=n.units)
   INTO any_shipped,all_shipped FROM need n LEFT JOIN issued i USING(product_id,bom_spec_id);
   IF NOT COALESCE(any_shipped,false) THEN RETURN NULL; END IF;
   RETURN CASE WHEN all_shipped THEN '已发货' ELSE '部分发货' END;
 END $fn$;
 CREATE OR REPLACE FUNCTION %[1]s.clamp_fulfillment_shipment_status() RETURNS trigger LANGUAGE plpgsql AS $fn$
 DECLARE progress TEXT;
 BEGIN
   IF NEW.confirmation_required AND EXISTS(SELECT 1 FROM %[1]s.ship_statuses WHERE id=NEW.ship_status_id AND name IN ('已发货','已出库','已签收','已收货','已完成')) THEN
     progress:=%[1]s.fulfillment_shipment_progress(NEW.id);
     IF progress='部分发货' THEN SELECT id INTO NEW.ship_status_id FROM %[1]s.ship_statuses WHERE name=progress ORDER BY id LIMIT 1; END IF;
   END IF;
   RETURN NEW;
 END $fn$;
 DROP TRIGGER IF EXISTS clamp_fulfillment_shipment_status ON %[1]s.orders;
 CREATE TRIGGER clamp_fulfillment_shipment_status BEFORE UPDATE OF ship_status_id ON %[1]s.orders FOR EACH ROW EXECUTE FUNCTION %[1]s.clamp_fulfillment_shipment_status();
 CREATE OR REPLACE FUNCTION %[1]s.update_fulfillment_shipment_progress() RETURNS trigger LANGUAGE plpgsql AS $fn$
 DECLARE progress TEXT; previous TEXT; target BIGINT; actor_name TEXT;
 BEGIN
   target:=NEW.order_id; actor_name:=COALESCE(NULLIF(to_jsonb(NEW)->>'operator',''),'仓库');
   PERFORM id FROM %[1]s.orders WHERE id=target FOR UPDATE;
   progress:=%[1]s.fulfillment_shipment_progress(target);
   IF progress IS NULL THEN RETURN NEW; END IF;
   SELECT s.name INTO previous FROM %[1]s.orders o LEFT JOIN %[1]s.ship_statuses s ON s.id=o.ship_status_id WHERE o.id=target;
   IF previous IS NOT DISTINCT FROM progress OR previous IN ('已签收','已收货','已完成') THEN RETURN NEW; END IF;
   UPDATE %[1]s.orders SET ship_status_id=(SELECT id FROM %[1]s.ship_statuses WHERE name=progress ORDER BY id LIMIT 1) WHERE id=target;
   INSERT INTO %[1]s.audit_logs(actor,entity_type,entity_id,action,field,old_value,new_value,meta)
     VALUES(actor_name,'order',target,'shipping_progress','ship_status',previous,progress,jsonb_build_object('source','stock_deductions'));
   INSERT INTO %[1]s.order_audit_logs(order_id,actor,field,old_value,new_value) VALUES(target,actor_name,'ship_status',previous,progress);
   RETURN NEW;
 END $fn$;
 DO $block$ BEGIN
   IF to_regclass('%[1]s.order_stock_deductions') IS NOT NULL THEN
     DROP TRIGGER IF EXISTS update_fulfillment_shipment_progress ON %[1]s.order_stock_deductions;
     CREATE TRIGGER update_fulfillment_shipment_progress AFTER INSERT OR UPDATE ON %[1]s.order_stock_deductions FOR EACH ROW EXECUTE FUNCTION %[1]s.update_fulfillment_shipment_progress();
   END IF;
 END $block$;
 `, schema))
	return err
}
