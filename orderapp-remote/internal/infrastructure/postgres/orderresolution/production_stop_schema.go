package orderresolution

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ensureProductionStopSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
 CREATE TABLE IF NOT EXISTS %[1]s.order_resolution_production_stops(
 work_order_id BIGINT PRIMARY KEY REFERENCES %[1]s.work_orders(id),request_id BIGINT NOT NULL REFERENCES %[1]s.order_resolution_requests(id),
 order_id BIGINT NOT NULL REFERENCES %[1]s.orders(id),plan_id BIGINT NOT NULL,selection_json JSONB NOT NULL,snapshot_json JSONB NOT NULL,actor TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now());
 CREATE OR REPLACE FUNCTION %[1]s.order_resolution_stop_record_guard()RETURNS trigger LANGUAGE plpgsql AS $fn$
 BEGIN
 IF TG_OP<>'INSERT' THEN RAISE EXCEPTION '停工处置记录不可改写或删除，请保留原工单历史';END IF;
 IF NOT EXISTS(SELECT 1 FROM %[1]s.order_resolution_requests r WHERE r.id=NEW.request_id AND r.order_id=NEW.order_id AND r.status='pending' AND NOT r.applied
 AND r.command_json->>'action'='close' AND r.command_json->>'scope'='all'
 AND EXISTS(SELECT 1 FROM jsonb_array_elements(r.command_json->'dispositions') d CROSS JOIN LATERAL jsonb_array_elements(d->'stops') s WHERE d->>'kind'='production' AND d->>'choice'='stop' AND (d->>'id')::bigint=NEW.plan_id AND s=NEW.selection_json)
 AND EXISTS(SELECT 1 FROM %[1]s.order_resolution_steps s WHERE s.request_id=r.id AND s.source_id=NEW.plan_id AND s.kind='production' AND s.choice='stop' AND s.status='completed')
 AND EXISTS(SELECT 1 FROM %[1]s.order_resolution_steps s WHERE s.request_id=r.id AND s.source_id=NEW.plan_id AND s.kind='production_stock' AND s.choice='stop' AND s.status='completed'))
 THEN RAISE EXCEPTION '停工须完成原方案的生产和仓库确认';END IF;
 RETURN NEW;END $fn$;
 DROP TRIGGER IF EXISTS order_resolution_stop_record_guard ON %[1]s.order_resolution_production_stops;
 CREATE TRIGGER order_resolution_stop_record_guard BEFORE INSERT OR UPDATE OR DELETE ON %[1]s.order_resolution_production_stops FOR EACH ROW EXECUTE FUNCTION %[1]s.order_resolution_stop_record_guard();
 CREATE OR REPLACE FUNCTION %[1]s.order_resolution_stopped_guard()RETURNS trigger LANGUAGE plpgsql AS $fn$
 DECLARE doc JSONB;old_doc JSONB;identity_doc JSONB;work BIGINT;work2 BIGINT;stop RECORD;plan BIGINT;works BIGINT[]:='{}';plans BIGINT[]:='{}';
 BEGIN
 doc:=CASE WHEN TG_OP='DELETE' THEN to_jsonb(OLD) ELSE to_jsonb(NEW) END;
 IF TG_OP='UPDATE' THEN old_doc:=to_jsonb(OLD);IF doc=old_doc THEN RETURN NEW;END IF;END IF;
 FOR identity_doc IN SELECT doc UNION ALL SELECT old_doc WHERE old_doc IS NOT NULL LOOP
 work:=0;work2:=0;plan:=0;
 work:=COALESCE((identity_doc->>'work_order_id')::bigint,0);
 IF TG_TABLE_NAME='work_orders' THEN work:=(identity_doc->>'id')::bigint;plan:=(identity_doc->>'production_plan_id')::bigint;END IF;
 IF TG_TABLE_NAME='produce_running_items' THEN SELECT id INTO work FROM %[1]s.work_orders WHERE running_item_id=(identity_doc->>'id')::bigint;END IF;
 IF TG_TABLE_NAME='work_order_material_reservations' AND work=0 THEN SELECT id INTO work FROM %[1]s.work_orders WHERE running_item_id=(identity_doc->>'running_item_id')::bigint AND running_item_id>0;END IF;
 IF TG_TABLE_NAME IN('production_started_input_components','production_started_input_allocations') THEN SELECT work_order_id INTO work FROM %[1]s.production_started_inputs WHERE id=(identity_doc->>'input_id')::bigint;END IF;
 IF TG_TABLE_NAME IN('production_operation_movements','production_started_handoffs','production_started_input_component_allocations','production_started_input_returns') THEN SELECT work_order_id INTO work FROM %[1]s.production_operation_events WHERE id=(identity_doc->>'event_id')::bigint;END IF;
 IF TG_TABLE_NAME='production_plans' THEN plan:=(identity_doc->>'id')::bigint;END IF;
 IF TG_TABLE_NAME='production_plan_items' THEN plan:=(identity_doc->>'production_plan_id')::bigint;END IF;
 IF TG_TABLE_NAME='stock_entry_items' THEN SELECT work_order_id INTO work FROM %[1]s.stock_entries WHERE id=(identity_doc->>'stock_entry_id')::bigint;END IF;
 IF TG_TABLE_NAME='production_supply_allocations' THEN work2:=(identity_doc->>'supplier_work_order_id')::bigint;END IF;
 IF TG_TABLE_NAME='work_order_dependencies' THEN work2:=(identity_doc->>'depends_on_work_order_id')::bigint;END IF;
 works:=works||ARRAY[work,work2];plans:=plans||ARRAY[plan];
 -- Follow both sides of consumption/transfer links, including the old parent on edits.
 IF TG_TABLE_NAME IN('production_operation_movements','production_started_handoffs') THEN
 works:=works||ARRAY(SELECT work_order_id FROM %[1]s.production_operation_balances WHERE id=(identity_doc->>'source_balance_id')::bigint);END IF;
 IF TG_TABLE_NAME IN('production_started_input_component_allocations','production_started_input_returns') THEN
 works:=works||ARRAY(SELECT i.work_order_id FROM %[1]s.production_started_input_components c JOIN %[1]s.production_started_inputs i ON i.id=c.input_id WHERE c.id=(identity_doc->>'component_id')::bigint);END IF;
 IF TG_TABLE_NAME IN('production_started_input_allocations','production_started_handoffs') THEN
 works:=works||ARRAY(SELECT work_order_id FROM %[1]s.production_operation_events WHERE id=(identity_doc->>'event_id')::bigint);END IF;
 IF TG_TABLE_NAME='production_started_handoffs' THEN
 works:=works||ARRAY(SELECT work_order_id FROM %[1]s.job_cards WHERE id=(identity_doc->>'job_card_id')::bigint);END IF;
 END LOOP;
 SELECT s.*,r.applied,r.status request_status INTO stop FROM %[1]s.order_resolution_production_stops s JOIN %[1]s.order_resolution_requests r ON r.id=s.request_id WHERE s.work_order_id=ANY(works) OR s.plan_id=ANY(plans) LIMIT 1;
 IF NOT FOUND THEN IF TG_OP='DELETE' THEN RETURN OLD;END IF;RETURN NEW;END IF;
 IF TG_OP='UPDATE' AND NOT stop.applied AND stop.request_status='pending' THEN
 IF TG_TABLE_NAME IN('work_orders','job_cards','produce_running_items','production_plans') AND doc->>'status'='stopped' AND old_doc->>'status'<>'stopped' AND doc-ARRAY['status','updated_at']=old_doc-ARRAY['status','updated_at'] THEN RETURN NEW;END IF;
 IF TG_TABLE_NAME IN('work_order_material_reservations','work_order_material_reservation_batches') AND stop.selection_json->>'reservations'='release_unconsumed' AND old_doc->>'status'='reserved' AND doc->>'status'='released'
 AND doc-ARRAY['status','returned_g','returned_units','updated_at']=old_doc-ARRAY['status','returned_g','returned_units','updated_at']
 AND (doc->>'returned_g')::numeric=GREATEST(0,(old_doc->>'reserved_g')::numeric-(old_doc->>'consumed_g')::numeric)
 AND (doc->>'returned_units')::numeric=GREATEST(0,(old_doc->>'reserved_units')::numeric-(old_doc->>'consumed_units')::numeric) THEN RETURN NEW;END IF;
 END IF;
 RAISE EXCEPTION '工单已停止剩余生产，原扣料、在制品和产出记录须保留；重新开启销售订单不会恢复该工单';
 END $fn$;
 `, schema))
	if err != nil {
		return err
	}
	for _, table := range []string{"work_orders", "job_cards", "produce_running_items", "production_plans", "production_plan_items", "production_operation_events", "production_operation_balances", "production_operation_movements", "production_started_inputs", "production_started_input_components", "production_started_input_allocations", "production_started_input_component_allocations", "production_started_handoffs", "production_started_input_returns", "work_order_material_reservations", "work_order_material_reservation_batches", "stock_entries", "stock_entry_items", "production_supply_allocations", "work_order_dependencies"} {
		if _, err = pool.Exec(ctx, fmt.Sprintf(`DROP TRIGGER IF EXISTS order_resolution_stopped_guard ON %[1]s.%[2]s;CREATE TRIGGER order_resolution_stopped_guard BEFORE INSERT OR UPDATE OR DELETE ON %[1]s.%[2]s FOR EACH ROW EXECUTE FUNCTION %[1]s.order_resolution_stopped_guard()`, schema, table)); err != nil {
			return err
		}
	}
	return nil
}
