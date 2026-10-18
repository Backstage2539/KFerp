package customerstock

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ensureShippingGuards(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
 CREATE OR REPLACE FUNCTION %[1]s.customer_stock_require_shippable(target BIGINT) RETURNS VOID LANGUAGE plpgsql AS $fn$
 DECLARE doc RECORD; incomplete BOOLEAN;
 BEGIN
 SELECT * INTO doc FROM %[1]s.orders WHERE id=target FOR UPDATE;
 IF NOT FOUND THEN RETURN; END IF;
 IF doc.delivery_mode='customer_stock' THEN RAISE EXCEPTION '留厂交付订单无需发货，请使用客户库存代发'; END IF;
 IF doc.fulfillment_mode<>'stock' THEN RETURN; END IF;
 SELECT EXISTS(SELECT 1 FROM %[1]s.order_items i WHERE i.order_id=target AND COALESCE((SELECT SUM(a.allocated_qty) FROM %[1]s.customer_direct_ship_request_allocations a WHERE a.order_item_id=i.id AND a.status='reserved'),0)<i.qty) INTO incomplete;
 IF incomplete THEN RAISE EXCEPTION '客户库存尚未实际入库，不能发货'; END IF;
 END $fn$;
 CREATE OR REPLACE FUNCTION %[1]s.customer_stock_guard_shipping_row() RETURNS trigger LANGUAGE plpgsql AS $fn$
 BEGIN PERFORM %[1]s.customer_stock_require_shippable(NEW.order_id); RETURN NEW; END $fn$;
 DO $block$ DECLARE target TEXT; BEGIN
 FOREACH target IN ARRAY ARRAY['order_shipment_orders','order_stock_deductions'] LOOP
 IF to_regclass('%[1]s.'||target) IS NOT NULL THEN
 EXECUTE format('DROP TRIGGER IF EXISTS customer_stock_guard_shipping ON %%I.%%I','%[1]s',target);
 EXECUTE format('CREATE TRIGGER customer_stock_guard_shipping BEFORE INSERT OR UPDATE ON %%I.%%I FOR EACH ROW EXECUTE FUNCTION %[1]s.customer_stock_guard_shipping_row()','%[1]s',target);
 END IF;END LOOP;END $block$;
 CREATE OR REPLACE FUNCTION %[1]s.customer_stock_guard_shipping_status() RETURNS trigger LANGUAGE plpgsql AS $fn$
 BEGIN
 IF NEW.ship_status_id IS DISTINCT FROM OLD.ship_status_id AND EXISTS(SELECT 1 FROM %[1]s.ship_statuses WHERE id=NEW.ship_status_id AND name IN('已发货','部分发货','已出库','已签收','已收货','已完成')) THEN
 PERFORM %[1]s.customer_stock_require_shippable(NEW.id);
 END IF;RETURN NEW;END $fn$;
 DROP TRIGGER IF EXISTS customer_stock_guard_shipping_status ON %[1]s.orders;
 CREATE TRIGGER customer_stock_guard_shipping_status BEFORE UPDATE OF ship_status_id ON %[1]s.orders FOR EACH ROW EXECUTE FUNCTION %[1]s.customer_stock_guard_shipping_status();
 `, schema))
	return err
}
