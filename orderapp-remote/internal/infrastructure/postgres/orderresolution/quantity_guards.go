package orderresolution

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

// A stale shipment/reservation writer must obey the same remaining quantity as
// the preview. The order row serializes aggregate checks across different batches.
func ensureQuantityGuards(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	for _, v := range []struct{ table, prefix string }{{"order_stock_batch_allocations", "allocated"}, {"order_stock_deductions", "deducted"}} {
		_, err := pool.Exec(ctx, fmt.Sprintf(`
 CREATE OR REPLACE FUNCTION %[1]s.resolution_quantity_%[2]s()RETURNS trigger LANGUAGE plpgsql AS $fn$
 DECLARE allowed NUMERIC;used NUMERIC;quantity NUMERIC;
 BEGIN
 PERFORM id FROM %[1]s.orders WHERE id=NEW.order_id FOR UPDATE;
 IF NOT EXISTS(SELECT 1 FROM %[1]s.order_items WHERE order_id=NEW.order_id AND resolution_closed_qty>0)THEN RETURN NEW;END IF;
 IF NEW.%[3]s_units<0 OR NEW.%[3]s_g<0 THEN RAISE EXCEPTION '履约数量不得为负数';END IF;
 quantity:=CASE WHEN NEW.%[3]s_units>0 THEN NEW.%[3]s_units WHEN NEW.spec_g>0 THEN NEW.%[3]s_g::numeric/NEW.spec_g ELSE 0 END;
 IF quantity=0 AND NEW.%[3]s_g>0 THEN RAISE EXCEPTION '履约记录缺少有效计量，请核对规格';END IF;
 SELECT COALESCE(sum(GREATEST(0,qty-resolution_closed_qty)),0)INTO allowed FROM %[1]s.order_items i
 WHERE i.order_id=NEW.order_id AND i.product_id=NEW.product_id AND COALESCE(i.bom_spec_id,0)=NEW.bom_spec_id
 AND(NEW.bom_spec_id>0 OR COALESCE(NULLIF(regexp_replace(COALESCE(i.spec,''),'[^0-9]','','g'),''),'0')::bigint=NEW.spec_g);
 IF quantity>0 AND NOT EXISTS(SELECT 1 FROM %[1]s.order_items i WHERE i.order_id=NEW.order_id AND i.product_id=NEW.product_id AND COALESCE(i.bom_spec_id,0)=NEW.bom_spec_id AND(COALESCE(i.bom_variant_id,0)=0 OR COALESCE(i.bom_variant_id,0)=NEW.bom_variant_id))THEN RAISE EXCEPTION '履约规格版本与原单不一致';END IF;
 SELECT COALESCE(sum(CASE WHEN %[3]s_units>0 THEN %[3]s_units WHEN spec_g>0 THEN %[3]s_g::numeric/spec_g ELSE 0 END),0)INTO used FROM %[1]s.%[2]s
 WHERE order_id=NEW.order_id AND product_id=NEW.product_id AND bom_spec_id=NEW.bom_spec_id AND(NEW.bom_spec_id>0 OR spec_g=NEW.spec_g)AND id<>NEW.id;
 IF used+quantity>allowed THEN RAISE EXCEPTION '履约或预留数量超过订单关闭后的可继续数量';END IF;
 RETURN NEW;END $fn$;
 DROP TRIGGER IF EXISTS order_resolution_quantity ON %[1]s.%[2]s;
 CREATE TRIGGER order_resolution_quantity BEFORE INSERT OR UPDATE ON %[1]s.%[2]s FOR EACH ROW EXECUTE FUNCTION %[1]s.resolution_quantity_%[2]s();
 `, schema, v.table, v.prefix))
		if err != nil {
			return err
		}
	}
	return nil
}
