package sales

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	salesapp "orderapp/internal/application/sales"
	"orderapp/internal/infrastructure/postgres/orderconfirmation"
)

// Read header and every line from one consistent, read-only snapshot. Do not
// reuse OrderForm: its legacy recipient defaults come from today's customer.
func (r Repository) LoadOrderCopySnapshot(ctx context.Context, id int64) (*salesapp.OrderCopySnapshot, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	snapshot := &salesapp.OrderCopySnapshot{}
	o := &snapshot.Order
	query := fmt.Sprintf(`SELECT o.id,COALESCE(o.customer_id,0),COALESCE(o.order_no,''),COALESCE(to_char(o.order_date,'YYYY-MM-DD'),''),COALESCE(ps.name,''),COALESCE(o.is_void,false),COALESCE(o.receiver_name,''),COALESCE(o.receiver_phone,''),COALESCE(o.receiver_address,'') FROM %s.orders o LEFT JOIN %s.pay_statuses ps ON ps.id=o.pay_status_id WHERE o.id=$1`, r.schema, r.schema)
	err = tx.QueryRow(ctx, orderconfirmation.CurrentRead(query, r.schema), id).Scan(&o.ID, &o.CustomerID, &o.OrderNo, &o.OrderDate, &snapshot.PayStatus, &o.IsVoid, &o.ReceiverName, &o.ReceiverPhone, &o.ReceiverAddress)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	query = fmt.Sprintf(`SELECT COALESCE(oi.customer_product_display_name_snapshot,''),COALESCE(oi.product_name_snapshot,''),COALESCE(oi.item_name,''),COALESCE(oi.spec,''),COALESCE(oi.qty::text,''),COALESCE(oi.unit,''),COALESCE(oi.product_kind,''),COALESCE(oi.sales_unit,''),COALESCE(oi.unit_bag_count,0),COALESCE(oi.unit_bean_g::text,''),COALESCE(oi.price_source_json,'{}'::jsonb)::text FROM %s.order_items oi WHERE oi.order_id=$1 ORDER BY oi.line_no,oi.id`, r.schema)
	rows, err := tx.Query(ctx, orderconfirmation.CurrentRead(query, r.schema), id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var item salesapp.OrderEditItem
		if err = rows.Scan(&item.CustomerProductDisplayNameSnapshot, &item.ProductNameSnapshot, &item.Product, &item.Spec, &item.Qty, &item.Unit, &item.ProductKind, &item.SalesUnit, &item.UnitBagCount, &item.UnitBeanG, &item.PriceSourceJSON); err != nil {
			rows.Close()
			return nil, err
		}
		o.Items = append(o.Items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return snapshot, nil
}
