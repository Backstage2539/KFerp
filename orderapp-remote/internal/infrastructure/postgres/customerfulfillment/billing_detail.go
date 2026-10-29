package customerfulfillment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/customerfulfillment"
)

// billingLineDetail reads only sources belonging to the bill's customer.
// It is used within the issuing transaction and, for older bills, read-only.
func (r *Repository) billingLineDetail(ctx context.Context, q billingReader, customerID int64, line app.SettlementLineSummary, source string) (*app.BillingLineDetail, error) {
	if line.ChargeType != "order_goods" && line.ChargeType != "product" && line.ChargeType != "shipping" && line.ChargeType != "direct_ship_service" {
		return nil, nil
	}
	orderID := int64(0)
	fee := app.AccountFee{}
	if line.SourceType == "order" {
		orderID = line.SourceID
	} else if line.SourceType == "customer_fee_item" {
		var feeSource string
		var feeSourceID int64
		err := q.QueryRow(ctx, fmt.Sprintf(`SELECT coalesce(source_type,''),coalesce(source_id,0) FROM %s.customer_fee_items WHERE id=$1 AND customer_id=$2`, r.schema), line.SourceID, customerID).Scan(&feeSource, &feeSourceID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if feeSource == "order" {
			orderID = feeSourceID
		}
		if feeSource == "freight_actual" && relationExists(ctx, q, r.schema+".freight_actual_entries") && relationExists(ctx, q, r.schema+".freight_actual_shipments") {
			err = q.QueryRow(ctx, fmt.Sprintf(`SELECT s.order_id,coalesce(s.tracking_no,'') FROM %[1]s.freight_actual_entries e JOIN %[1]s.freight_actual_shipments s ON s.id=e.shipment_id WHERE e.id=$1 AND s.customer_id=$2`, r.schema), feeSourceID, customerID).Scan(&orderID, &fee.TrackingNo)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return nil, err
			}
		}
	}
	order := app.AccountOrder{}
	if orderID > 0 {
		err := q.QueryRow(ctx, fmt.Sprintf(`SELECT id,coalesce(order_no,''),round(coalesce(discount_amount,0)*100)::bigint FROM %s.orders WHERE id=$1 AND customer_id=$2`, r.schema), orderID, customerID).Scan(&order.ID, &order.OrderNo, &order.DiscountCents)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	if order.ID > 0 && (line.ChargeType == "order_goods" || line.ChargeType == "product") {
		rows, err := q.Query(ctx, fmt.Sprintf(`SELECT coalesce(item_name,''),coalesce(spec,''),coalesce(qty,0)::text,coalesce(unit,''),coalesce(unit_price,0)::text,coalesce(line_total,0)::text FROM %s.order_items WHERE order_id=$1 AND qty>0 ORDER BY id`, r.schema), order.ID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var i app.AccountOrderItem
			if err = rows.Scan(&i.Name, &i.Spec, &i.Quantity, &i.Unit, &i.UnitPrice, &i.LineTotal); err != nil {
				break
			}
			order.Items = append(order.Items, i)
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	shipments := []app.AccountShipment{}
	if order.ID > 0 && (line.ChargeType == "shipping" || line.ChargeType == "direct_ship_service") && relationExists(ctx, q, r.schema+".order_shipment_orders") {
		join, fields := "", "''::text,''::text"
		if relationExists(ctx, q, r.schema+".shipping_return_rows") {
			join = fmt.Sprintf(`LEFT JOIN LATERAL (SELECT payload FROM %s.shipping_return_rows sr WHERE sr.status='applied' AND sr.selected_order_id=so.order_id AND sr.payload->>'tracking_no'=so.tracking_no ORDER BY sr.id DESC LIMIT 1) sr ON true`, r.schema)
			fields = `coalesce(sr.payload->>'pickup_time',''),coalesce(sr.payload->>'settlement_weight_kg','')`
		}
		rows, err := q.Query(ctx, fmt.Sprintf(`SELECT coalesce(so.tracking_no,''),coalesce(to_char(so.shipped_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD HH24:MI:SS'),''),%[3]s FROM %[1]s.order_shipment_orders so %[2]s WHERE so.order_id=$1 ORDER BY so.id`, r.schema, join, fields), order.ID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var s app.AccountShipment
			var pickup string
			if err = rows.Scan(&s.TrackingNo, &s.ShippedAt, &pickup, &s.SettlementWeightKG); err != nil {
				break
			}
			s.OrderID = order.ID
			s.OrderNo = order.OrderNo
			s.Source = "ERP发货记录"
			if pickup != "" {
				s.ShippedAt = pickup
				s.PickupAt = pickup
				s.Source = "快递揽件记录"
			}
			shipments = append(shipments, s)
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	if order.ID > 0 && (line.ChargeType == "shipping" || line.ChargeType == "direct_ship_service") && relationExists(ctx, q, r.schema+".shipping_return_rows") {
		rows, err := q.Query(ctx, fmt.Sprintf(`SELECT coalesce(payload->>'tracking_no',''),coalesce(payload->>'pickup_time',''),coalesce(payload->>'settlement_weight_kg','') FROM %s.shipping_return_rows WHERE status='applied' AND selected_order_id=$1 ORDER BY id DESC`, r.schema), order.ID)
		if err != nil {
			return nil, err
		}
		seen := map[string]bool{}
		for rows.Next() {
			var tracking, pickup, weight string
			if err = rows.Scan(&tracking, &pickup, &weight); err != nil {
				break
			}
			if tracking == "" || seen[tracking] {
				continue
			}
			seen[tracking] = true
			found := false
			for i := range shipments {
				if shipments[i].TrackingNo != tracking {
					continue
				}
				found = true
				shipments[i].SettlementWeightKG = weight
				if pickup != "" {
					shipments[i].ShippedAt, shipments[i].PickupAt, shipments[i].Source = pickup, pickup, "快递揽件记录"
				}
				break
			}
			if !found {
				shipments = append(shipments, app.AccountShipment{OrderID: order.ID, OrderNo: order.OrderNo, TrackingNo: tracking, ShippedAt: pickup, PickupAt: pickup, SettlementWeightKG: weight, Source: "快递回传记录"})
			}
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	if fee.TrackingNo != "" {
		found := false
		for _, s := range shipments {
			if s.TrackingNo == fee.TrackingNo {
				found = true
				break
			}
		}
		if !found {
			shipments = append(shipments, app.AccountShipment{OrderID: order.ID, OrderNo: order.OrderNo, TrackingNo: fee.TrackingNo, Source: "快递费用记录"})
		}
	}
	detail := app.BuildBillingLineDetail(line, order, fee, shipments, source)
	return &detail, nil
}

func (r *Repository) hydrateBillingLineDetails(ctx context.Context, q billingReader, customerID int64, lines []app.SettlementLineSummary) error {
	for i := range lines {
		var raw []byte
		if err := q.QueryRow(ctx, fmt.Sprintf(`SELECT detail_snapshot FROM %s.customer_settlement_lines WHERE id=$1 AND customer_id=$2`, r.schema), lines[i].ID, customerID).Scan(&raw); err != nil {
			return err
		}
		if len(raw) > 0 && string(raw) != "null" {
			var d app.BillingLineDetail
			if err := json.Unmarshal(raw, &d); err != nil {
				return err
			}
			lines[i].Detail = &d
			continue
		}
		detail, err := r.billingLineDetail(ctx, q, customerID, lines[i], "原单当前信息")
		if err != nil {
			return err
		}
		lines[i].Detail = detail
	}
	return nil
}
