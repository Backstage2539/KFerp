package orderresolution

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	domain "orderapp/internal/domain/orderresolution"
	stockpg "orderapp/internal/infrastructure/postgres/stock"
)

func (r Repository) ownershipReturnSources(ctx context.Context, q querier, c *domain.Context) error {
	tx, ok := q.(pgx.Tx)
	if !ok {
		return fmt.Errorf("客户货权退回须在一致的库存快照中核对")
	}
	rows, err := q.Query(ctx, fmt.Sprintf(`SELECT receipt.id,e.id,e.source_order_item_id,b.id,receipt.quantity-COALESCE((SELECT sum(x.quantity) FROM %[1]s.order_resolution_stock_returns x WHERE x.source_kind='ownership' AND x.source_id=receipt.id AND x.status='posted'),0),item.item_name,item.unit,b.batch_code,md5(jsonb_build_object('receipt',to_jsonb(receipt),'entitlement',to_jsonb(e),'batch',to_jsonb(b))::text)
 FROM %[1]s.customer_stock_entitlement_receipts receipt JOIN %[1]s.customer_stock_entitlements e ON e.id=receipt.entitlement_id JOIN %[1]s.order_items item ON item.id=e.source_order_item_id JOIN %[1]s.stock_batches b ON b.id=receipt.stock_batch_id AND b.batch_code=receipt.batch_code WHERE e.source_order_id=$1 ORDER BY receipt.id`, r.schema), c.OrderID)
	if err != nil {
		return err
	}
	var sources []domain.ReturnSource
	for rows.Next() {
		var s domain.ReturnSource
		var name, code string
		s.Kind = "ownership"
		if err = rows.Scan(&s.ID, &s.EntitlementID, &s.ItemID, &s.BatchID, &s.AvailableQuantity, &name, &s.Unit, &code, &s.Version); err != nil {
			rows.Close()
			return err
		}
		s.Label = fmt.Sprintf("%s · 客户货权原入库 #%d · %s", name, s.ID, code)
		sources = append(sources, s)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, s := range sources {
		physical, err := stockpg.NewRepository(r.pool, r.schema).OwnershipReturnStockTx(ctx, tx, s.ID, c.OrderID)
		if err != nil {
			if err == pgx.ErrNoRows {
				continue
			}
			return err
		}
		if physical.UnavailableReason != "" || physical.AvailableUnits <= 0 || s.AvailableQuantity <= 0 {
			continue
		}
		s.BatchAvailableQuantity = physical.AvailableUnits
		s.AvailableQuantity = min(s.AvailableQuantity, physical.AvailableUnits)
		raw, _ := json.Marshal(physical)
		s.Version = hash([]byte(s.Version + string(raw)))
		destinations, err := q.Query(ctx, fmt.Sprintf(`SELECT code,name FROM %[1]s.warehouses WHERE active AND kind IN('finished','customer_finished','customer_processing','customer') AND COALESCE(customer_id,0)=0 AND NOT EXISTS(SELECT 1 FROM %[1]s.finished_inventory f WHERE f.product_id=$1 AND f.bom_spec_id=$2 AND f.spec_g=0 AND f.warehouse=warehouses.code AND f.owner_customer_id<>0 AND (f.onhand_units<>0 OR f.onhand_loose_g<>0)) ORDER BY sort_order,code`, r.schema), physical.ItemID, physical.BomSpecID)
		if err != nil {
			return err
		}
		for destinations.Next() {
			d := domain.ReturnDestination{Ownership: "company"}
			if err = destinations.Scan(&d.Warehouse, &d.Label); err != nil {
				destinations.Close()
				return err
			}
			s.Destinations = append(s.Destinations, d)
		}
		err = destinations.Err()
		destinations.Close()
		if err != nil {
			return err
		}
		if len(s.Destinations) > 0 {
			c.ReturnSources = append(c.ReturnSources, s)
		}
	}
	raw, _ := json.Marshal(c.ReturnSources)
	c.Version = hash([]byte(c.Version + string(raw)))
	return nil
}
