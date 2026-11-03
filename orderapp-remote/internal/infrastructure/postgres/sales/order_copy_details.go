package sales

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	salesapp "orderapp/internal/application/sales"
	"orderapp/internal/infrastructure/postgres/orderconfirmation"
)

// Read header, recipient defaults and every line from one consistent, read-only
// snapshot. The formatter applies the same recipient policy as order detail.
func (r Repository) LoadOrderCopySnapshot(ctx context.Context, id int64) (*salesapp.OrderCopySnapshot, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	snapshot := &salesapp.OrderCopySnapshot{}
	o := &snapshot.Order
	query := fmt.Sprintf(`SELECT o.id,COALESCE(o.customer_id,0),COALESCE(c.name,''),COALESCE(o.payment_method,''),COALESCE(o.order_no,''),COALESCE(to_char(o.order_date,'YYYY-MM-DD'),''),COALESCE(ps.name,''),COALESCE(o.is_void,false),COALESCE(o.receiver_name,''),COALESCE(o.receiver_phone,''),COALESCE(o.receiver_address,''),COALESCE(o.portal_service_code,''),COALESCE(NULLIF(c.contact,''),c.name,''),COALESCE(c.phone,''),COALESCE(c.address,''),to_char(COALESCE(o.total_amount,0),'FM999999999999990.00'),to_char(COALESCE(o.shipping_amount,0),'FM999999999999990.00'),to_char(%[2]s,'FM999999999999990.00'),COALESCE(to_jsonb(o)->>'prepayment_amount','0') FROM %[1]s.orders o LEFT JOIN %[1]s.pay_statuses ps ON ps.id=o.pay_status_id LEFT JOIN %[1]s.customers c ON c.id=o.customer_id WHERE o.id=$1`, r.schema, orderReceivableSQL("o"))
	err = tx.QueryRow(ctx, orderconfirmation.CurrentRead(query, r.schema), id).Scan(&o.ID, &o.CustomerID, &snapshot.CustomerName, &snapshot.PaymentMethod, &o.OrderNo, &o.OrderDate, &snapshot.PayStatus, &o.IsVoid, &o.ReceiverName, &o.ReceiverPhone, &o.ReceiverAddress, &o.PortalServiceCode, &snapshot.DefaultRecipient.Name, &snapshot.DefaultRecipient.Phone, &snapshot.DefaultRecipient.Address, &o.TotalAmount, &o.ShippingAmount, &o.GrandTotal, &o.PrepaymentAmount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var freightTable bool
	if err = tx.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", r.schema+".order_freight_snapshots").Scan(&freightTable); err != nil {
		return nil, err
	}
	if freightTable {
		query = fmt.Sprintf(`SELECT (COALESCE(COALESCE(NULLIF(to_jsonb(o)->'confirmation_pending'->'freight','null'::jsonb), f.snapshot_json)->>'charge_mode','') = 'deferred' OR (COALESCE(COALESCE(NULLIF(to_jsonb(o)->'confirmation_pending'->'freight','null'::jsonb), f.snapshot_json)->>'charge_mode','') = '' AND COALESCE(COALESCE(NULLIF(to_jsonb(o)->'confirmation_pending'->'freight','null'::jsonb), f.snapshot_json)->>'status','') = 'pending_actual')) FROM %[1]s.orders o LEFT JOIN %[1]s.order_freight_snapshots f ON f.order_id=o.id WHERE o.id=$1`, r.schema)
		if err = tx.QueryRow(ctx, query, id).Scan(&snapshot.FreightDeferred); err != nil {
			return nil, err
		}
	}
	query = fmt.Sprintf(`SELECT COALESCE(oi.customer_product_display_name_snapshot,''),COALESCE(oi.product_name_snapshot,''),COALESCE(oi.item_name,''),COALESCE(oi.spec,''),COALESCE(oi.qty::text,''),COALESCE(oi.unit,''),COALESCE(oi.product_kind,''),COALESCE(oi.sales_unit,''),COALESCE(oi.unit_bag_count,0),COALESCE(oi.unit_bean_g::text,''),COALESCE(oi.price_source_json,'{}'::jsonb)::text FROM %s.order_items oi WHERE oi.order_id=$1 AND oi.qty>0 ORDER BY oi.line_no,oi.id`, r.schema)
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
