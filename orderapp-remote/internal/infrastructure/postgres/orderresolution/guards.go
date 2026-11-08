package orderresolution

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

// These guards cover legacy writers as well as the new UI. They only stop new
// obligations; historical receipts, completed production and release events stay intact.
func ensureGuards(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
 CREATE OR REPLACE FUNCTION %[1]s.order_resolution_require_open(target BIGINT) RETURNS VOID LANGUAGE plpgsql AS $fn$
 DECLARE doc RECORD;
 BEGIN SELECT resolution_status,fulfillment_paused INTO doc FROM %[1]s.orders WHERE id=target FOR SHARE;
 IF FOUND AND (doc.fulfillment_paused OR doc.resolution_status IN('closed','cancelled')) THEN RAISE EXCEPTION '订单已暂停或结束履约，请先处理订单处理单';END IF;
 END $fn$;
 CREATE OR REPLACE FUNCTION %[1]s.order_resolution_guard_link()RETURNS trigger LANGUAGE plpgsql AS $fn$
 DECLARE target BIGINT; source BIGINT;
 BEGIN
 IF TG_OP='UPDATE' AND to_jsonb(NEW)=to_jsonb(OLD) THEN RETURN NEW;END IF;
 IF TG_OP='UPDATE' AND TG_TABLE_NAME='order_stock_batch_allocations'
 AND EXISTS(SELECT 1 FROM %[1]s.order_resolution_reservation_releases e JOIN %[1]s.order_resolution_requests r ON r.id=e.request_id WHERE e.allocation_id=(to_jsonb(OLD)->>'id')::bigint AND e.snapshot_json=to_jsonb(OLD) AND e.after_json=to_jsonb(NEW) AND r.status='pending')THEN RETURN NEW;END IF;
 IF TG_OP='UPDATE' AND TG_TABLE_NAME='order_stock_batch_allocations'
 AND (to_jsonb(NEW)-ARRAY['allocated_g','allocated_units'])=(to_jsonb(OLD)-ARRAY['allocated_g','allocated_units'])
 AND (to_jsonb(NEW)->>'allocated_g')::numeric=0 AND(to_jsonb(NEW)->>'allocated_units')::numeric=0
 AND EXISTS(SELECT 1 FROM %[1]s.order_resolution_reservation_releases e JOIN %[1]s.order_resolution_requests r ON r.id=e.request_id WHERE e.allocation_id=(to_jsonb(OLD)->>'id')::bigint AND e.snapshot_json=to_jsonb(OLD) AND r.status='pending')THEN RETURN NEW;END IF;
 IF TG_OP='UPDATE' AND TG_TABLE_NAME='customer_stock_supply_reservations'
  AND (to_jsonb(NEW)-ARRAY['released_qty','status','updated_at'])=(to_jsonb(OLD)-ARRAY['released_qty','status','updated_at'])
  AND COALESCE((to_jsonb(NEW)->>'released_qty')::numeric,0)>=COALESCE((to_jsonb(OLD)->>'released_qty')::numeric,0)
  AND (to_jsonb(NEW)->>'status') IN('released','cancelled') THEN RETURN NEW;END IF;
 target:=COALESCE((to_jsonb(NEW)->>'order_id')::bigint,0);
 IF target>0 THEN PERFORM %[1]s.order_resolution_require_open(target); END IF;
 source:=COALESCE((to_jsonb(NEW)->>'entitlement_id')::bigint,0);
 IF source>0 THEN SELECT source_order_id INTO target FROM %[1]s.customer_stock_entitlements WHERE id=source;PERFORM %[1]s.order_resolution_require_open(target);END IF;
 RETURN NEW;END $fn$;
 CREATE OR REPLACE FUNCTION %[1]s.order_resolution_guard_shipping()RETURNS trigger LANGUAGE plpgsql AS $fn$
 BEGIN IF NEW.ship_status_id IS DISTINCT FROM OLD.ship_status_id OR NEW.ship_tracking_no IS DISTINCT FROM OLD.ship_tracking_no THEN PERFORM %[1]s.order_resolution_require_open(OLD.id);END IF;RETURN NEW;END $fn$;
 DROP TRIGGER IF EXISTS order_resolution_guard_shipping ON %[1]s.orders;
 CREATE TRIGGER order_resolution_guard_shipping BEFORE UPDATE OF ship_status_id,ship_tracking_no ON %[1]s.orders FOR EACH ROW EXECUTE FUNCTION %[1]s.order_resolution_guard_shipping();
 CREATE OR REPLACE FUNCTION %[1]s.order_resolution_guard_header()RETURNS trigger LANGUAGE plpgsql AS $fn$
 BEGIN
 IF NEW.customer_id IS DISTINCT FROM OLD.customer_id OR NEW.total_amount IS DISTINCT FROM OLD.total_amount OR NEW.grand_total IS DISTINCT FROM OLD.grand_total OR NEW.shipping_amount IS DISTINCT FROM OLD.shipping_amount OR NEW.discount_amount IS DISTINCT FROM OLD.discount_amount OR NEW.delivery_mode IS DISTINCT FROM OLD.delivery_mode OR NEW.delivery_warehouse_code IS DISTINCT FROM OLD.delivery_warehouse_code THEN
 IF EXISTS(SELECT 1 FROM %[1]s.order_resolution_requests WHERE order_id=OLD.id AND(status='pending' OR EXISTS(SELECT 1 FROM %[1]s.order_resolution_line_events e WHERE e.request_id=order_resolution_requests.id))) THEN RAISE EXCEPTION '订单已有处理记录，数量、金额及货权须通过订单处理更正';END IF;
 END IF;RETURN NEW;END $fn$;
 DROP TRIGGER IF EXISTS order_resolution_guard_header ON %[1]s.orders;
 CREATE TRIGGER order_resolution_guard_header BEFORE UPDATE ON %[1]s.orders FOR EACH ROW EXECUTE FUNCTION %[1]s.order_resolution_guard_header();
 CREATE OR REPLACE FUNCTION %[1]s.order_resolution_company_production_issue(plan BIGINT)RETURNS TEXT LANGUAGE plpgsql AS $fn$
 BEGIN
 -- customer_id is the demand/customer-price identity, not inventory ownership.
 -- Match the canonical receipt owner: destination warehouse or output material;
 -- processing requests, consumed inputs and posted stock remain authoritative.
 IF EXISTS(SELECT 1 FROM %[1]s.work_orders WHERE production_plan_id=plan AND processing_request_item_id<>0) OR EXISTS(SELECT 1 FROM %[1]s.production_plan_items WHERE production_plan_id=plan AND processing_request_item_id<>0)THEN RETURN '客户加工生产不能转为公司备货，请先办理货权处置';END IF;
 IF EXISTS(SELECT 1 FROM (SELECT target_warehouse,output_material_id FROM %[1]s.work_orders WHERE production_plan_id=plan UNION ALL SELECT target_warehouse,output_material_id FROM %[1]s.production_plan_items WHERE production_plan_id=plan) p LEFT JOIN %[1]s.warehouses h ON h.code=p.target_warehouse LEFT JOIN %[1]s.materials m ON m.id=p.output_material_id WHERE COALESCE(h.customer_id,0)<>0 OR COALESCE(m.owner_customer_id,0)<>0)THEN RETURN '生产目标仓库或产出物料属于客户，不能作为公司备货';END IF;
 IF EXISTS(SELECT 1 FROM %[1]s.stock_entries e JOIN %[1]s.stock_entry_items i ON i.stock_entry_id=e.id JOIN %[1]s.work_orders w ON w.id=e.work_order_id WHERE w.production_plan_id=plan AND e.status='submitted' AND i.owner_customer_id<>0)THEN RETURN '生产出入库记录含客户货权，请先办理货权处置';END IF;
 IF to_regclass('%[1]s.production_plan_component_sources')IS NOT NULL THEN
 IF EXISTS(SELECT 1 FROM %[1]s.production_plan_component_sources WHERE production_plan_id=plan AND source_owner_customer_id<>0)THEN RETURN '生产用料含客户货权，不能转为公司备货';END IF;END IF;
 IF to_regclass('%[1]s.work_order_material_reservations')IS NOT NULL THEN
 IF EXISTS(SELECT 1 FROM %[1]s.work_order_material_reservations m JOIN %[1]s.work_orders w ON w.id=m.work_order_id WHERE w.production_plan_id=plan AND m.source_owner_customer_id<>0)THEN RETURN '生产预留含客户货权，不能转为公司备货';END IF;END IF;
 IF to_regclass('%[1]s.production_started_input_components')IS NOT NULL THEN
 IF EXISTS(SELECT 1 FROM %[1]s.production_started_input_components c JOIN %[1]s.production_started_inputs i ON i.id=c.input_id JOIN %[1]s.work_orders w ON w.id=i.work_order_id WHERE w.production_plan_id=plan AND c.owner_customer_id<>0)THEN RETURN '已投入客户用料，须先办理货权处置';END IF;END IF;
 RETURN ''; END $fn$;
 CREATE OR REPLACE FUNCTION %[1]s.order_resolution_require_company_production(plan BIGINT)RETURNS VOID LANGUAGE plpgsql AS $fn$
 DECLARE issue TEXT;BEGIN issue:=%[1]s.order_resolution_company_production_issue(plan);IF issue<>''THEN RAISE EXCEPTION '%%',issue;END IF;END $fn$;
 CREATE OR REPLACE FUNCTION %[1]s.order_resolution_require_production(target BIGINT,plan BIGINT)RETURNS VOID LANGUAGE plpgsql AS $fn$
 BEGIN
 IF EXISTS(SELECT 1 FROM %[1]s.order_resolution_production_dispositions d JOIN %[1]s.order_resolution_requests r ON r.id=d.request_id JOIN %[1]s.orders o ON o.id=d.order_id WHERE d.order_id=target AND d.plan_id=plan AND d.choice='continue_company' AND r.applied AND NOT o.fulfillment_paused AND o.resolution_status IN('closed','partially_closed')) THEN
 PERFORM %[1]s.order_resolution_require_company_production(plan);RETURN;END IF;
 PERFORM %[1]s.order_resolution_require_open(target);
 END $fn$;
 CREATE OR REPLACE FUNCTION %[1]s.order_resolution_guard_job_card()RETURNS trigger LANGUAGE plpgsql AS $fn$
 DECLARE target BIGINT;plan BIGINT;
 BEGIN
 IF NEW.status='stopped' AND EXISTS(SELECT 1 FROM %[1]s.order_resolution_production_stops WHERE work_order_id=NEW.work_order_id) THEN RETURN NEW;END IF;
 IF TG_OP='UPDATE' AND NEW.status IS NOT DISTINCT FROM OLD.status AND NEW.started_at IS NOT DISTINCT FROM OLD.started_at AND NEW.actual_input_qty<=OLD.actual_input_qty THEN RETURN NEW;END IF;
 IF NEW.status IN('cancelled','paused') THEN RETURN NEW;END IF;
 SELECT production_plan_id INTO plan FROM %[1]s.work_orders WHERE id=NEW.work_order_id;
 FOR target IN SELECT o.id FROM %[1]s.orders o JOIN %[1]s.work_orders w ON o.order_no=ANY(string_to_array(replace(w.order_nos,' ',''),','))WHERE w.id=NEW.work_order_id LOOP PERFORM %[1]s.order_resolution_require_production(target,plan);END LOOP;
 RETURN NEW;END $fn$;
 DROP TRIGGER IF EXISTS order_resolution_guard_job_card ON %[1]s.job_cards;
 CREATE TRIGGER order_resolution_guard_job_card BEFORE INSERT OR UPDATE OF status,started_at,actual_input_qty ON %[1]s.job_cards FOR EACH ROW EXECUTE FUNCTION %[1]s.order_resolution_guard_job_card();
 CREATE OR REPLACE FUNCTION %[1]s.order_resolution_guard_production()RETURNS trigger LANGUAGE plpgsql AS $fn$
 DECLARE target BIGINT;
 BEGIN
 IF (to_jsonb(NEW)->>'status')='stopped' AND EXISTS(SELECT 1 FROM %[1]s.order_resolution_production_stops WHERE work_order_id=(to_jsonb(NEW)->>'id')::bigint) THEN RETURN NEW;END IF;
 IF TG_OP='UPDATE' AND (to_jsonb(NEW)->>'status') IN('cancelled','completed') THEN RETURN NEW;END IF;
 FOR target IN SELECT id FROM %[1]s.orders WHERE order_no=ANY(string_to_array(replace(COALESCE(to_jsonb(NEW)->>'order_nos',''),' ',''),',')) LOOP
 PERFORM %[1]s.order_resolution_require_production(target,COALESCE((to_jsonb(NEW)->>'production_plan_id')::bigint,0));
 END LOOP;
 RETURN NEW;END $fn$;
 CREATE OR REPLACE FUNCTION %[1]s.order_resolution_guard_item()RETURNS trigger LANGUAGE plpgsql AS $fn$
 BEGIN
 IF TG_OP='INSERT' THEN
 PERFORM %[1]s.order_resolution_require_open(NEW.order_id);
 IF EXISTS(SELECT 1 FROM %[1]s.order_resolution_line_events e JOIN %[1]s.order_resolution_requests r ON r.id=e.request_id WHERE r.order_id=NEW.order_id) THEN RAISE EXCEPTION '订单已有处理记录，请通过修订新增商品';END IF;RETURN NEW;END IF;
 IF TG_OP='DELETE' OR (to_jsonb(NEW)-'resolution_closed_qty') IS DISTINCT FROM (to_jsonb(OLD)-'resolution_closed_qty') THEN
 IF EXISTS(SELECT 1 FROM %[1]s.order_resolution_line_events WHERE order_item_id=OLD.id) OR EXISTS(SELECT 1 FROM %[1]s.order_resolution_requests WHERE order_id=OLD.order_id AND status='pending') THEN RAISE EXCEPTION '商品已有订单处理记录，请通过重新开启、退货或修订处理';END IF;
 END IF;
 IF TG_OP='DELETE' THEN RETURN OLD;END IF;
 IF NEW.resolution_closed_qty<0 OR NEW.resolution_closed_qty>NEW.qty THEN RAISE EXCEPTION '关闭数量超出订单商品范围';END IF;
 RETURN NEW;END $fn$;
 DROP TRIGGER IF EXISTS order_resolution_guard_item ON %[1]s.order_items;
 CREATE TRIGGER order_resolution_guard_item BEFORE INSERT OR UPDATE OR DELETE ON %[1]s.order_items FOR EACH ROW EXECUTE FUNCTION %[1]s.order_resolution_guard_item();
 CREATE OR REPLACE FUNCTION %[1]s.order_resolution_guard_fee_payment()RETURNS trigger LANGUAGE plpgsql AS $fn$
 BEGIN
 IF OLD.status IN('paid','settled') AND NEW.status NOT IN('paid','settled') AND EXISTS(SELECT 1 FROM %[1]s.customer_fee_items f WHERE f.settlement_batch_id=OLD.id AND f.resolution_cash_tracked AND f.resolution_cash_received_cents>0 AND f.resolution_credit_cents>0) THEN RAISE EXCEPTION '关联费用已有冲减或资金处置，不能撤销原收款，请通过财务更正保留历史';END IF;
 RETURN NEW;END $fn$;
 DROP TRIGGER IF EXISTS order_resolution_guard_fee_payment ON %[1]s.customer_settlement_batches;
 CREATE TRIGGER order_resolution_guard_fee_payment BEFORE UPDATE OF status ON %[1]s.customer_settlement_batches FOR EACH ROW EXECUTE FUNCTION %[1]s.order_resolution_guard_fee_payment();
 CREATE OR REPLACE FUNCTION %[1]s.order_resolution_guard_fee()RETURNS trigger LANGUAGE plpgsql AS $fn$
 BEGIN
 IF (NEW.amount IS DISTINCT FROM OLD.amount OR NEW.customer_id IS DISTINCT FROM OLD.customer_id OR NEW.source_type IS DISTINCT FROM OLD.source_type OR NEW.source_id IS DISTINCT FROM OLD.source_id OR NEW.status IN('reversed','void')) AND EXISTS(SELECT 1 FROM %[1]s.order_resolution_fee_events e WHERE e.source_id=OLD.id)THEN RAISE EXCEPTION '费用已有订单处理记录，请通过费用调整保留原始金额';END IF;
 RETURN NEW;END $fn$;
 DROP TRIGGER IF EXISTS order_resolution_guard_fee ON %[1]s.customer_fee_items;
 CREATE TRIGGER order_resolution_guard_fee BEFORE UPDATE ON %[1]s.customer_fee_items FOR EACH ROW EXECUTE FUNCTION %[1]s.order_resolution_guard_fee();
 `, schema))
	if err != nil {
		return err
	}
	for _, table := range []string{"order_stock_batch_allocations", "order_stock_deductions", "order_shipment_orders", "customer_stock_supply_reservations", "customer_stock_entitlement_receipts", "produce_batch_order_items"} {
		var exists bool
		if err = pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, schema+"."+table).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			continue
		}
		if _, err = pool.Exec(ctx, fmt.Sprintf(`DROP TRIGGER IF EXISTS order_resolution_guard_link ON %[1]s.%[2]s;CREATE TRIGGER order_resolution_guard_link BEFORE INSERT OR UPDATE ON %[1]s.%[2]s FOR EACH ROW EXECUTE FUNCTION %[1]s.order_resolution_guard_link()`, schema, table)); err != nil {
			return err
		}
	}
	for _, table := range []string{"production_plan_items", "work_orders"} {
		var exists bool
		if err = pool.QueryRow(ctx, `SELECT to_regclass($1)IS NOT NULL`, schema+"."+table).Scan(&exists); err != nil {
			return err
		}
		if exists {
			events := "INSERT OR UPDATE OF order_nos"
			if table == "work_orders" {
				events = "INSERT OR UPDATE OF status,order_nos"
			}
			if _, err = pool.Exec(ctx, fmt.Sprintf(`DROP TRIGGER IF EXISTS order_resolution_guard_production ON %[1]s.%[2]s;CREATE TRIGGER order_resolution_guard_production BEFORE %[3]s ON %[1]s.%[2]s FOR EACH ROW EXECUTE FUNCTION %[1]s.order_resolution_guard_production()`, schema, table, events)); err != nil {
				return err
			}
		}
	}
	return ensureQuantityGuards(ctx, pool, schema)
}
