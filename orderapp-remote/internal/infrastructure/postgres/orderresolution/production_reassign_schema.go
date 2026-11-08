package orderresolution

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ensureProductionReassignSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
 CREATE TABLE IF NOT EXISTS %[1]s.order_resolution_production_reassignments(
 id BIGSERIAL PRIMARY KEY,request_id BIGINT NOT NULL REFERENCES %[1]s.order_resolution_requests(id),source_order_id BIGINT NOT NULL REFERENCES %[1]s.orders(id),target_order_id BIGINT NOT NULL REFERENCES %[1]s.orders(id),target_item_id BIGINT NOT NULL REFERENCES %[1]s.order_items(id),plan_id BIGINT NOT NULL,plan_item_id BIGINT NOT NULL,work_order_id BIGINT NOT NULL UNIQUE,running_item_id BIGINT NOT NULL,quantity BIGINT NOT NULL CHECK(quantity>0),unit TEXT NOT NULL CHECK(unit<>''),snapshot_json JSONB NOT NULL,target_json JSONB NOT NULL,status TEXT NOT NULL DEFAULT 'posting' CHECK(status IN('posting','applied')),actor TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now());
 CREATE OR REPLACE FUNCTION %[1]s.order_resolution_reassign_record_guard()RETURNS trigger LANGUAGE plpgsql AS $fn$
 BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION '转供记录须保留，不可删除';END IF;
 IF TG_OP='INSERT' THEN
 IF NEW.status<>'posting' OR NOT EXISTS(SELECT 1 FROM %[1]s.order_resolution_requests r JOIN %[1]s.work_orders w ON w.id=NEW.work_order_id JOIN %[1]s.orders source ON source.id=NEW.source_order_id JOIN %[1]s.order_items target_item ON target_item.id=NEW.target_item_id JOIN %[1]s.orders target ON target.id=target_item.order_id
 WHERE r.id=NEW.request_id AND r.order_id=NEW.source_order_id AND r.status='pending' AND NOT r.applied AND r.command_json->>'action'='close' AND target.id=NEW.target_order_id AND target.id<>source.id AND NOT target.is_void AND NOT target.fulfillment_paused AND target.resolution_status NOT IN('closed','cancelled')
 AND w.production_plan_id=NEW.plan_id AND w.production_plan_item_id=NEW.plan_item_id AND w.running_item_id=NEW.running_item_id AND w.order_nos=source.order_no AND to_jsonb(w)=NEW.snapshot_json->'work' AND w.planned_inventory_qty=NEW.quantity AND w.inventory_unit=NEW.unit
 AND target_item.product_id=w.product_id AND target_item.bom_spec_id=w.bom_spec_id AND target_item.bom_variant_id=w.bom_variant_id AND target_item.unit=NEW.unit
 AND jsonb_build_object('order',to_jsonb(target),'item',to_jsonb(target_item))=NEW.target_json
 AND EXISTS(SELECT 1 FROM jsonb_array_elements(r.command_json->'dispositions') d CROSS JOIN LATERAL jsonb_array_elements(d->'reassignments') s WHERE d->>'kind'='production' AND d->>'choice'='reassign' AND (d->>'id')::bigint=NEW.plan_id AND (s->>'work_order_id')::bigint=NEW.work_order_id AND (s->>'target_item_id')::bigint=NEW.target_item_id AND (s->>'quantity')::bigint=NEW.quantity)
 AND EXISTS(SELECT 1 FROM %[1]s.order_resolution_steps step WHERE step.request_id=r.id AND step.kind='production' AND step.source_id=NEW.plan_id AND step.choice='reassign' AND step.permission='production.resolve' AND step.status='completed' AND btrim(step.result_json->>'reference')<>'')) THEN RAISE EXCEPTION '转供缺少已确认的原工单及目标需求';END IF;
 ELSE
 IF OLD.status='applied' OR to_jsonb(NEW)-'status'<>to_jsonb(OLD)-'status' OR NEW.status<>'applied' THEN RAISE EXCEPTION '原转供事实不可改写';END IF;
 IF NOT EXISTS(SELECT 1 FROM %[1]s.work_orders w JOIN %[1]s.production_plan_items p ON p.id=NEW.plan_item_id WHERE w.id=NEW.work_order_id AND w.order_nos=NEW.target_json->'order'->>'order_no' AND w.customer_id=(NEW.target_json->'order'->>'customer_id')::bigint AND to_jsonb(w)-ARRAY['order_nos','customer_id']=(NEW.snapshot_json->'work')-ARRAY['order_nos','customer_id'] AND to_jsonb(p)-ARRAY['order_nos','customer_id','demand_sources_json']=(NEW.snapshot_json->'plan_item')-ARRAY['order_nos','customer_id','demand_sources_json'] AND p.order_nos=w.order_nos AND p.customer_id=w.customer_id AND jsonb_array_length(p.demand_sources_json)=1 AND (p.demand_sources_json->0->>'order_item_id')::bigint=NEW.target_item_id AND (p.demand_sources_json->0->>'quantity')::bigint=NEW.quantity) THEN RAISE EXCEPTION '转供只能变更明确的需求关联，原生产数量及冻结来源须保留';END IF;
 IF NEW.running_item_id>0 AND NOT EXISTS(SELECT 1 FROM %[1]s.produce_running_items run WHERE run.id=NEW.running_item_id AND run.order_nos=NEW.target_json->'order'->>'order_no' AND to_jsonb(run)-'order_nos'=(NEW.snapshot_json->'run')-'order_nos') THEN RAISE EXCEPTION '运行批次未同步明确的目标需求';END IF;
 END IF;RETURN NEW;END $fn$;
 DROP TRIGGER IF EXISTS order_resolution_reassign_record_guard ON %[1]s.order_resolution_production_reassignments;
 CREATE TRIGGER order_resolution_reassign_record_guard BEFORE INSERT OR UPDATE OR DELETE ON %[1]s.order_resolution_production_reassignments FOR EACH ROW EXECUTE FUNCTION %[1]s.order_resolution_reassign_record_guard();
 CREATE OR REPLACE FUNCTION %[1]s.order_resolution_reassign_target_guard()RETURNS trigger LANGUAGE plpgsql AS $fn$
 DECLARE doc JSONB:=to_jsonb(OLD);nextdoc JSONB:=to_jsonb(NEW);key TEXT;keys TEXT[];
 BEGIN
 IF EXISTS(SELECT 1 FROM %[1]s.order_resolution_production_reassignments x WHERE x.status='applied' AND ((TG_TABLE_NAME='orders' AND x.target_order_id=(doc->>'id')::bigint) OR (TG_TABLE_NAME='order_items' AND x.target_item_id=(doc->>'id')::bigint))) THEN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION '转供目标订单须保留，请通过订单处理更正';END IF;
 IF TG_TABLE_NAME='orders' THEN keys:=ARRAY['order_no','customer_id','delivery_mode','delivery_warehouse_code','source_warehouse','fulfillment_mode'];ELSE keys:=ARRAY['order_id','product_id','bom_spec_id','bom_variant_id','qty','unit','customer_product_alias_id','customer_product_display_name_snapshot','customer_item_code_snapshot','item_note'];END IF;
 FOREACH key IN ARRAY keys LOOP IF doc->key IS DISTINCT FROM nextdoc->key THEN RAISE EXCEPTION '已有转供生产，目标身份及数量须通过订单处理更正';END IF;END LOOP;
 END IF;
 IF TG_OP='DELETE' THEN RETURN OLD;END IF;RETURN NEW;END $fn$;
 DROP TRIGGER IF EXISTS order_resolution_reassign_target_guard ON %[1]s.orders;
 CREATE TRIGGER order_resolution_reassign_target_guard BEFORE UPDATE OR DELETE ON %[1]s.orders FOR EACH ROW EXECUTE FUNCTION %[1]s.order_resolution_reassign_target_guard();
 DROP TRIGGER IF EXISTS order_resolution_reassign_target_guard ON %[1]s.order_items;
 CREATE TRIGGER order_resolution_reassign_target_guard BEFORE UPDATE OR DELETE ON %[1]s.order_items FOR EACH ROW EXECUTE FUNCTION %[1]s.order_resolution_reassign_target_guard();

 CREATE OR REPLACE FUNCTION %[1]s.order_resolution_reassign_demand_guard() RETURNS trigger LANGUAGE plpgsql AS $fn$
 DECLARE doc JSONB:=to_jsonb(NEW);target BIGINT;
 BEGIN
 -- Share the target order lock with posting and all production writers. A plan
 -- that was prepared before transfer must not add a second supply afterwards.
 FOR target IN SELECT id FROM %[1]s.orders WHERE order_no=ANY(string_to_array(replace(COALESCE(doc->>'order_nos',''),' ',''),',')) ORDER BY id FOR UPDATE LOOP
 IF EXISTS(SELECT 1 FROM %[1]s.order_resolution_production_reassignments x WHERE x.status='applied' AND x.target_order_id=target
 AND (x.snapshot_json->'work'->>'product_id')::bigint=(doc->>'product_id')::bigint AND (x.snapshot_json->'work'->>'bom_spec_id')::bigint=(doc->>'bom_spec_id')::bigint
 AND NOT ((TG_TABLE_NAME='work_orders' AND x.work_order_id=(doc->>'id')::bigint) OR(TG_TABLE_NAME='production_plan_items' AND x.plan_item_id=(doc->>'id')::bigint))) THEN RAISE EXCEPTION '目标需求已接收转供生产，不能重复安排；请另办生产更正';END IF;
 END LOOP;
 IF TG_OP='INSERT' AND EXISTS(SELECT 1 FROM %[1]s.order_resolution_production_reassignments x WHERE x.status='applied' AND x.plan_id=(doc->>'production_plan_id')::bigint)THEN RAISE EXCEPTION '整批转供的原计划不可追加生产或需求';END IF;
 RETURN NEW;END $fn$;
 DROP TRIGGER IF EXISTS order_resolution_reassign_demand_guard ON %[1]s.production_plan_items;
 CREATE TRIGGER order_resolution_reassign_demand_guard BEFORE INSERT OR UPDATE OF order_nos,product_id,bom_spec_id ON %[1]s.production_plan_items FOR EACH ROW EXECUTE FUNCTION %[1]s.order_resolution_reassign_demand_guard();
 DROP TRIGGER IF EXISTS order_resolution_reassign_demand_guard ON %[1]s.work_orders;
 CREATE TRIGGER order_resolution_reassign_demand_guard BEFORE INSERT OR UPDATE OF order_nos,product_id,bom_spec_id ON %[1]s.work_orders FOR EACH ROW EXECUTE FUNCTION %[1]s.order_resolution_reassign_demand_guard();
 CREATE OR REPLACE FUNCTION %[1]s.order_resolution_reassign_history_guard()RETURNS trigger LANGUAGE plpgsql AS $fn$
 DECLARE doc JSONB:=to_jsonb(OLD);nextdoc JSONB:=to_jsonb(NEW);keys TEXT[]:=ARRAY['order_nos','customer_id','product_id','bom_spec_id','bom_variant_id','spec_g','inventory_unit','planned_inventory_qty','target_warehouse','production_plan_id','production_plan_item_id','running_item_id','demand_sources_json','batch_id','parent_product_id','bom_source_product_id','bom_version_id','bom_inherited','material_snapshot','process_snapshot_json','production_config_snapshot_json','customer_product_snapshot_json','sales_spec_snapshot_json','inventory_qty_per_sales_unit','sales_spec_count','planned_units','planned_g','planned_output_g','output_type','output_product_id','output_material_id','output_qty','output_unit','material_consumption_policy','operation_material_bindings','operation_template_id','process_template_id'];key TEXT;frozen BOOLEAN;
 BEGIN
 SELECT EXISTS(SELECT 1 FROM %[1]s.order_resolution_production_reassignments x WHERE x.status='applied' AND ((TG_TABLE_NAME='work_orders' AND x.work_order_id=(doc->>'id')::bigint) OR (TG_TABLE_NAME='production_plan_items' AND x.plan_item_id=(doc->>'id')::bigint) OR (TG_TABLE_NAME='produce_running_items' AND x.running_item_id=(doc->>'id')::bigint))) INTO frozen;
 IF frozen THEN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION '原转供生产记录须保留';END IF;
 FOREACH key IN ARRAY keys LOOP IF doc->key IS DISTINCT FROM nextdoc->key THEN RAISE EXCEPTION '转供的冻结身份及需求关联不可直接改写，请另办生产更正';END IF;END LOOP;
 END IF;
 IF TG_OP='DELETE' THEN RETURN OLD;END IF;RETURN NEW;END $fn$;
 `, schema))
	if err != nil {
		return err
	}
	for _, table := range []string{"work_orders", "production_plan_items", "produce_running_items"} {
		if _, err = pool.Exec(ctx, fmt.Sprintf(`DROP TRIGGER IF EXISTS order_resolution_reassign_history_guard ON %[1]s.%[2]s;CREATE TRIGGER order_resolution_reassign_history_guard BEFORE UPDATE OR DELETE ON %[1]s.%[2]s FOR EACH ROW EXECUTE FUNCTION %[1]s.order_resolution_reassign_history_guard()`, schema, table)); err != nil {
			return err
		}
	}
	return nil
}
