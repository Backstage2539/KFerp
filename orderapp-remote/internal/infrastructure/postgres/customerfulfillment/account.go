package customerfulfillment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	app "orderapp/internal/application/customerfulfillment"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	freightpg "orderapp/internal/infrastructure/postgres/freight"
	"orderapp/internal/infrastructure/postgres/orderconfirmation"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// CustomerAccount reads a consistent view of existing sales and fee documents.
// Settlement batches are references to fees, never additional order receivables.
func (r *Repository) CustomerAccount(ctx context.Context, q app.AccountQuery) (app.AccountData, error) {
	d := app.AccountData{Rows: []app.AccountOrder{}, Fees: []app.AccountFee{}, Settlements: []app.AccountSettlement{}, DateFrom: q.DateFrom, DateTo: q.DateTo, AsOf: time.Now().In(time.FixedZone("Asia/Shanghai", 28800)).Format("2006-01-02 15:04:05"), View: q.View, ChargeType: q.ChargeType}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return d, err
	}
	defer tx.Rollback(ctx)
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT name FROM %s.customers WHERE id=$1`, r.schema), q.CustomerID).Scan(&d.CustomerName); err != nil {
		return d, err
	}
	where := []string{"o.customer_id=$1"}
	if !q.CurrentVersion {
		where = append(where, "(NOT COALESCE((to_jsonb(o)->>'confirmation_required')::boolean,false) OR COALESCE((to_jsonb(o)->>'confirmation_accepted_revision')::bigint,0)>0)")
	}
	currentQuery := func(query string) string {
		if q.CurrentVersion {
			return orderconfirmation.CurrentRead(query, r.schema)
		}
		return query
	}
	args := []any{q.CustomerID}
	add := func(clause string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if !q.IncludeVoid {
		where = append(where, "coalesce(o.is_void,false)=false")
	}
	if q.OrderID > 0 {
		add("o.id=$%d", q.OrderID)
	}
	if q.DateFrom != "" {
		add("o.order_date >= $%d::date", q.DateFrom)
	}
	if q.DateTo != "" {
		add("o.order_date <= $%d::date", q.DateTo)
	}
	if q.ShipStatus != "" {
		add("coalesce(ss.name,'')=$%d", q.ShipStatus)
	}
	if q.Query != "" {
		args = append(args, "%"+q.Query+"%")
		n := len(args)
		where = append(where, fmt.Sprintf(`(o.order_no ILIKE $%[1]d OR o.receiver_name ILIKE $%[1]d OR o.receiver_phone ILIKE $%[1]d OR EXISTS(SELECT 1 FROM %[2]s.order_items i WHERE i.order_id=o.id AND i.qty>0 AND i.item_name ILIKE $%[1]d))`, n, r.schema))
	}
	rows, err := tx.Query(ctx, currentQuery(fmt.Sprintf(`SELECT o.id,coalesce(o.order_no,''),coalesce(to_char(o.order_date,'YYYY-MM-DD'),''),coalesce(o.receiver_name,''),coalesce(o.receiver_phone,''),coalesce(o.receiver_address,''),coalesce(ss.name,''),coalesce(o.ship_tracking_no,''),coalesce(ps.name,''),coalesce(o.portal_service_code,''),coalesce(o.is_void,false),round(coalesce(o.grand_total,0)*100)::bigint,round(coalesce(o.shipping_amount,0)*100)::bigint,round(coalesce(o.discount_amount,0)*100)::bigint,round(coalesce((to_jsonb(o)->>'prepayment_amount')::numeric,0)*100)::bigint,COALESCE(to_jsonb(o)->>'confirmation_status','accepted'),COALESCE((to_jsonb(o)->>'confirmation_required')::boolean,false),COALESCE((to_jsonb(o)->>'confirmation_accepted_revision')::bigint,0),COALESCE(ops.name,'') FROM %[1]s.orders o LEFT JOIN %[1]s.ship_statuses ss ON ss.id=o.ship_status_id LEFT JOIN %[1]s.pay_statuses ps ON ps.id=o.pay_status_id LEFT JOIN %[1]s.order_process_statuses ops ON ops.id=o.process_status_id WHERE %[2]s ORDER BY o.order_date DESC NULLS LAST,o.id DESC`, r.schema, strings.Join(where, " AND "))), args...)
	if err != nil {
		return d, err
	}
	for rows.Next() {
		var o app.AccountOrder
		if err = rows.Scan(&o.ID, &o.OrderNo, &o.OrderDate, &o.ReceiverName, &o.ReceiverPhone, &o.ReceiverAddress, &o.ShipStatus, &o.TrackingNo, &o.PayStatus, &o.Service, &o.IsVoid, &o.TotalCents, &o.ShippingCents, &o.DiscountCents, &o.PrepaymentCents, &o.ConfirmationStatus, &o.ConfirmationRequired, &o.AcceptedRevision, &o.ProcessStatus); err != nil {
			rows.Close()
			return d, err
		}
		o.GoodsCents = o.TotalCents - o.ShippingCents + o.DiscountCents
		o.InPeriod = true
		o.SetPayment()
		if q.PayStatus != "" && o.PaymentStatus != q.PayStatus {
			continue
		}
		d.Rows = append(d.Rows, o)
		if !o.IsVoid {
			d.Summary.Count++
			d.Summary.GoodsCents += o.GoodsCents
			d.Summary.ShippingCents += o.ShippingCents
			d.Summary.DiscountCents += o.DiscountCents
			d.Summary.TotalCents += o.TotalCents
			d.Summary.PaidCents += o.PaidCents
			d.Summary.DueCents += o.DueCents
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return d, err
	}
	ids := []int64{}
	for _, o := range d.Rows {
		ids = append(ids, o.ID)
	}
	views, e := freightpg.NewRepository(r.pool, r.schema).OrderViews(ctx, tx, ids)
	if e != nil {
		return d, e
	}
	for i := range d.Rows {
		d.Rows[i].Freight = views[d.Rows[i].ID]
	}
	if q.OrderID > 0 && len(d.Rows) > 0 {
		items, err := tx.Query(ctx, currentQuery(fmt.Sprintf(`SELECT coalesce(item_name,''),coalesce(spec,''),coalesce(qty,0)::text,coalesce(unit,''),coalesce(unit_price,0)::text,coalesce(line_total,0)::text,to_jsonb(current_item)->>'material_unit_weight_g',
				to_jsonb(current_item)->>'material_item_weight_g',
				to_jsonb(current_item)->>'logistics_unit_weight_g',
				to_jsonb(current_item)->>'logistics_item_weight_g',
				COALESCE(to_jsonb(current_item)->>'weight_snapshot_json','{}') FROM %s.order_items current_item WHERE order_id=$1 AND qty>0 ORDER BY id`, r.schema)), q.OrderID)
		if err != nil {
			return d, err
		}
		d.Rows[0].Items = []app.AccountOrderItem{}
		for items.Next() {
			var i app.AccountOrderItem
			if err = items.Scan(&i.Name, &i.Spec, &i.Quantity, &i.Unit, &i.UnitPrice, &i.LineTotal, &i.MaterialUnitWeightG, &i.MaterialItemWeightG, &i.LogisticsUnitWeightG, &i.LogisticsItemWeightG, &i.WeightSnapshotJSON); err != nil {
				items.Close()
				return d, err
			}
			d.Rows[0].Items = append(d.Rows[0].Items, i)
		}
		err = items.Err()
		items.Close()
		if err != nil {
			return d, err
		}
	}

	freightJoin := ""
	shipmentFields := "0::bigint,''::text"
	orderJoin := "f.source_type='order' AND o.id=f.source_id"
	if relationExists(ctx, tx, r.schema+".freight_actual_entries") {
		freightJoin = fmt.Sprintf(" LEFT JOIN %s.freight_actual_entries fe ON f.source_type='freight_actual' AND fe.id=f.source_id LEFT JOIN %s.freight_actual_shipments fs ON fs.id=fe.shipment_id AND fs.customer_id=f.customer_id ", r.schema, r.schema)
		orderJoin = "((f.source_type='order' AND o.id=f.source_id) OR (f.source_type='freight_actual' AND o.id=fs.order_id))"
		shipmentFields = "coalesce(fs.id,0),coalesce(fs.tracking_no,'')"
	}
	// An order link is only valid when its source explicitly identifies an order of this customer.
	fees, err := tx.Query(ctx, fmt.Sprintf(`SELECT f.id,coalesce(f.source_type,''),coalesce(f.source_id,0),coalesce(o.id,0),coalesce(o.order_no,''),f.fee_type,coalesce(to_jsonb(f)->>'note',''),round(f.amount*100)::bigint,coalesce(nullif(f.currency,''),'CNY'),to_char(f.occurred_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD HH24:MI'),coalesce(s.id,0),coalesce(s.settlement_no,''),coalesce(s.status,''),coalesce(to_char(s.paid_at,'YYYY-MM-DD HH24:MI'),''),%[4]s FROM %[1]s.customer_fee_items f %[2]s LEFT JOIN %[1]s.orders o ON %[3]s AND o.customer_id=f.customer_id LEFT JOIN %[1]s.customer_settlement_batches s ON s.id=f.settlement_batch_id AND s.customer_id=f.customer_id WHERE f.customer_id=$1 ORDER BY f.occurred_at DESC,f.id DESC`, r.schema, freightJoin, orderJoin, shipmentFields), q.CustomerID)
	if err != nil {
		return d, err
	}
	allFees := []app.AccountFee{}
	seenFeeSources := map[string]bool{}
	for fees.Next() {
		var f app.AccountFee
		var paid string
		if err = fees.Scan(&f.ID, &f.SourceType, &f.SourceID, &f.OrderID, &f.OrderNo, &f.FeeType, &f.FeeName, &f.AmountCents, &f.Currency, &f.OccurredAt, &f.SettlementID, &f.SettlementNo, &f.SettlementStatus, &paid, &f.ShipmentID, &f.TrackingNo); err != nil {
			fees.Close()
			return d, err
		}
		f.PaymentStatus = "unpaid"
		if f.SettlementStatus == "paid" || (paid != "" && f.SettlementStatus != "reversed") {
			f.PaymentStatus = "paid"
		} else if f.SettlementStatus == "reversed" {
			f.PaymentStatus = "reversed"
		} else if f.SettlementStatus == "settled" {
			f.PaymentStatus = "unknown"
		}
		if strings.TrimSpace(f.FeeName) == "" {
			f.FeeName = app.AccountFeeLabel(f.FeeType)
		}
		if f.SourceID > 0 {
			key := fmt.Sprintf("%s:%d:%s", f.SourceType, f.SourceID, f.FeeType)
			if seenFeeSources[key] {
				continue
			}
			seenFeeSources[key] = true
		}
		f.IncludedInOrder = f.OrderID > 0 && (f.SourceType == "order" && f.FeeType == "shipping" || f.FeeType == "product" || f.FeeType == "order_goods")
		allFees = append(allFees, f)
		date := f.OccurredAt
		if len(date) > 10 {
			date = date[:10]
		}
		if (q.DateFrom == "" || date >= q.DateFrom) && (q.DateTo == "" || date <= q.DateTo) && (q.OrderID == 0 || f.OrderID == q.OrderID) {
			d.Fees = append(d.Fees, f)
			if !f.IncludedInOrder {
				if f.AmountCents < 0 {
					d.Summary.RefundCents += -f.AmountCents
				}
				switch f.FeeType {
				case "processing", "roasting", "labor", "material", "packaging":
					d.Summary.ProcessingCents += f.AmountCents
				case "direct_ship_service":
					d.Summary.DirectShipServiceCents += f.AmountCents
				case "shipping":
					d.Summary.FeeShippingCents += f.AmountCents
				case "adjustment":
					d.Summary.AdjustmentCents += f.AmountCents
				}
				d.Summary.AdditionalFeeCents += f.AmountCents
				if f.PaymentStatus == "paid" {
					d.Summary.PaidCents += f.AmountCents
				}
			}
		}
	}
	err = fees.Err()
	fees.Close()
	if err != nil {
		return d, err
	}
	if q.View == "order_fees" {
		groupOrders := append([]app.AccountOrder{}, d.Rows...)
		hasOrderShippingFee := map[int64]bool{}
		for _, fee := range allFees {
			if fee.OrderID > 0 && fee.SourceType == "order" && fee.FeeType == "shipping" {
				hasOrderShippingFee[fee.OrderID] = true
			}
		}
		knownOrders := map[int64]bool{}
		for i := range groupOrders {
			groupOrders[i].HasOrderShippingFee = hasOrderShippingFee[groupOrders[i].ID]
			knownOrders[groupOrders[i].ID] = true
		}
		missingIDs := []int64{}
		for _, fee := range d.Fees {
			if fee.OrderID > 0 && !knownOrders[fee.OrderID] {
				knownOrders[fee.OrderID] = true
				missingIDs = append(missingIDs, fee.OrderID)
			}
		}
		if len(missingIDs) > 0 {
			missing, queryErr := tx.Query(ctx, currentQuery(fmt.Sprintf(`SELECT o.id,coalesce(o.order_no,''),coalesce(to_char(o.order_date,'YYYY-MM-DD'),''),coalesce(o.receiver_name,''),coalesce(o.receiver_phone,''),coalesce(o.receiver_address,''),coalesce(ss.name,''),coalesce(o.ship_tracking_no,''),coalesce(ps.name,''),coalesce(o.portal_service_code,''),coalesce(o.is_void,false),round(coalesce(o.grand_total,0)*100)::bigint,round(coalesce(o.shipping_amount,0)*100)::bigint,round(coalesce(o.discount_amount,0)*100)::bigint,round(coalesce((to_jsonb(o)->>'prepayment_amount')::numeric,0)*100)::bigint,COALESCE(to_jsonb(o)->>'confirmation_status','accepted'),COALESCE((to_jsonb(o)->>'confirmation_required')::boolean,false),COALESCE((to_jsonb(o)->>'confirmation_accepted_revision')::bigint,0),COALESCE(ops.name,'') FROM %[1]s.orders o LEFT JOIN %[1]s.ship_statuses ss ON ss.id=o.ship_status_id LEFT JOIN %[1]s.pay_statuses ps ON ps.id=o.pay_status_id LEFT JOIN %[1]s.order_process_statuses ops ON ops.id=o.process_status_id WHERE o.customer_id=$1 AND o.id=ANY($2)`, r.schema)), q.CustomerID, missingIDs)
			if queryErr != nil {
				return d, queryErr
			}
			for missing.Next() {
				var order app.AccountOrder
				if queryErr = missing.Scan(&order.ID, &order.OrderNo, &order.OrderDate, &order.ReceiverName, &order.ReceiverPhone, &order.ReceiverAddress, &order.ShipStatus, &order.TrackingNo, &order.PayStatus, &order.Service, &order.IsVoid, &order.TotalCents, &order.ShippingCents, &order.DiscountCents, &order.PrepaymentCents, &order.ConfirmationStatus, &order.ConfirmationRequired, &order.AcceptedRevision, &order.ProcessStatus); queryErr != nil {
					missing.Close()
					return d, queryErr
				}
				order.GoodsCents = order.TotalCents - order.ShippingCents + order.DiscountCents
				order.SetPayment()
				order.HasOrderShippingFee = hasOrderShippingFee[order.ID]
				groupOrders = append(groupOrders, order)
			}
			queryErr = missing.Err()
			missing.Close()
			if queryErr != nil {
				return d, queryErr
			}
		}
		orderIDs := make([]int64, 0, len(groupOrders))
		orderIndex := map[int64]int{}
		for i, order := range groupOrders {
			orderIDs = append(orderIDs, order.ID)
			orderIndex[order.ID] = i
		}
		if len(orderIDs) > 0 {
			items, queryErr := tx.Query(ctx, currentQuery(fmt.Sprintf(`SELECT order_id,coalesce(item_name,''),coalesce(spec,''),coalesce(qty,0)::text,coalesce(unit,''),coalesce(unit_price,0)::text,coalesce(line_total,0)::text,to_jsonb(current_item)->>'material_unit_weight_g',to_jsonb(current_item)->>'material_item_weight_g',to_jsonb(current_item)->>'logistics_unit_weight_g',to_jsonb(current_item)->>'logistics_item_weight_g',COALESCE(to_jsonb(current_item)->>'weight_snapshot_json','{}') FROM %s.order_items current_item WHERE order_id=ANY($1) AND qty>0 ORDER BY order_id,id`, r.schema)), orderIDs)
			if queryErr == nil {
				for items.Next() {
					var orderID int64
					var item app.AccountOrderItem
					if queryErr = items.Scan(&orderID, &item.Name, &item.Spec, &item.Quantity, &item.Unit, &item.UnitPrice, &item.LineTotal, &item.MaterialUnitWeightG, &item.MaterialItemWeightG, &item.LogisticsUnitWeightG, &item.LogisticsItemWeightG, &item.WeightSnapshotJSON); queryErr != nil {
						break
					}
					groupOrders[orderIndex[orderID]].Items = append(groupOrders[orderIndex[orderID]].Items, item)
				}
				if queryErr == nil {
					queryErr = items.Err()
				}
				items.Close()
			}
			if queryErr != nil {
				return d, queryErr
			}
		}
		groups, feeSummary := app.BuildOrderFeeGroups(groupOrders, d.Fees)
		shipmentKeys := map[string]bool{}
		if len(orderIDs) > 0 && relationExists(ctx, tx, r.schema+".order_shipment_orders") {
			payloadJoin := ""
			payloadFields := "''::text,''::text"
			if relationExists(ctx, tx, r.schema+".shipping_return_rows") {
				payloadJoin = fmt.Sprintf(` LEFT JOIN LATERAL (SELECT payload FROM %s.shipping_return_rows sr WHERE sr.status='applied' AND sr.selected_order_id=so.order_id AND sr.payload->>'tracking_no'=so.tracking_no ORDER BY sr.id DESC LIMIT 1) sr ON true `, r.schema)
				payloadFields = `coalesce(sr.payload->>'pickup_time',''),coalesce(sr.payload->>'settlement_weight_kg','')`
			}
			shipments, queryErr := tx.Query(ctx, fmt.Sprintf(`SELECT so.order_id,coalesce(so.tracking_no,''),coalesce(to_char(so.shipped_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD HH24:MI:SS'),''),%[3]s FROM %[1]s.order_shipment_orders so %[2]s WHERE so.order_id=ANY($1) ORDER BY so.order_id,so.id`, r.schema, payloadJoin, payloadFields), orderIDs)
			if queryErr != nil {
				return d, queryErr
			}
			for shipments.Next() {
				var orderID int64
				var tracking, erpTime, pickup, weight string
				if queryErr = shipments.Scan(&orderID, &tracking, &erpTime, &pickup, &weight); queryErr != nil {
					break
				}
				order := groupOrders[orderIndex[orderID]]
				when, source := erpTime, "ERP发货记录"
				if strings.TrimSpace(pickup) != "" {
					when, source = pickup, "快递揽件记录"
				} else if erpTime == "" {
					source = "快递回传记录"
				}
				shipment := app.AccountShipment{OrderID: orderID, OrderNo: order.OrderNo, TrackingNo: tracking, ShippedAt: when, PickupAt: pickup, SettlementWeightKG: weight, Source: source}
				for i := range groups {
					if groups[i].Order.ID == orderID {
						groups[i].Shipments = append(groups[i].Shipments, shipment)
						shipmentKeys[fmt.Sprintf("%d:%s", orderID, tracking)] = true
						break
					}
				}
			}
			if queryErr == nil {
				queryErr = shipments.Err()
			}
			shipments.Close()
			if queryErr != nil {
				return d, queryErr
			}
		}
		if len(orderIDs) > 0 && relationExists(ctx, tx, r.schema+".order_shipping_trackings") {
			trackings, queryErr := tx.Query(ctx, fmt.Sprintf(`SELECT order_id,tracking_no FROM %s.order_shipping_trackings WHERE order_id=ANY($1) ORDER BY order_id,id`, r.schema), orderIDs)
			if queryErr != nil {
				return d, queryErr
			}
			for trackings.Next() {
				var orderID int64
				var tracking string
				if queryErr = trackings.Scan(&orderID, &tracking); queryErr != nil {
					break
				}
				key := fmt.Sprintf("%d:%s", orderID, tracking)
				if tracking == "" || shipmentKeys[key] {
					continue
				}
				order := groupOrders[orderIndex[orderID]]
				for i := range groups {
					if groups[i].Order.ID == orderID {
						groups[i].Shipments = append(groups[i].Shipments, app.AccountShipment{OrderID: orderID, OrderNo: order.OrderNo, TrackingNo: tracking, Source: "ERP物流记录"})
						shipmentKeys[key] = true
						break
					}
				}
			}
			if queryErr == nil {
				queryErr = trackings.Err()
			}
			trackings.Close()
			if queryErr != nil {
				return d, queryErr
			}
		}
		for _, order := range groupOrders {
			for _, tracking := range strings.FieldsFunc(order.TrackingNo, func(r rune) bool { return strings.ContainsRune(" \t\r\n,;，；、", r) }) {
				key := fmt.Sprintf("%d:%s", order.ID, tracking)
				if tracking == "" || shipmentKeys[key] {
					continue
				}
				for i := range groups {
					if groups[i].Order.ID == order.ID {
						groups[i].Shipments = append(groups[i].Shipments, app.AccountShipment{OrderID: order.ID, OrderNo: order.OrderNo, TrackingNo: tracking, Source: "ERP订单记录"})
						shipmentKeys[key] = true
						break
					}
				}
			}
		}
		if len(orderIDs) > 0 && relationExists(ctx, tx, r.schema+".shipping_return_rows") {
			returns, queryErr := tx.Query(ctx, fmt.Sprintf(`SELECT selected_order_id,coalesce(payload->>'tracking_no',''),coalesce(payload->>'pickup_time',''),coalesce(payload->>'settlement_weight_kg','') FROM %s.shipping_return_rows WHERE status='applied' AND selected_order_id=ANY($1) ORDER BY id`, r.schema), orderIDs)
			if queryErr != nil {
				return d, queryErr
			}
			for returns.Next() {
				var orderID int64
				var tracking, pickup, weight string
				if queryErr = returns.Scan(&orderID, &tracking, &pickup, &weight); queryErr != nil {
					break
				}
				if tracking == "" {
					continue
				}
				found := false
				for i := range groups {
					if groups[i].Order.ID != orderID {
						continue
					}
					for j := range groups[i].Shipments {
						if groups[i].Shipments[j].TrackingNo != tracking {
							continue
						}
						groups[i].Shipments[j].PickupAt = pickup
						groups[i].Shipments[j].SettlementWeightKG = weight
						if pickup != "" {
							groups[i].Shipments[j].ShippedAt = pickup
							groups[i].Shipments[j].Source = "快递揽件记录"
						}
						found = true
						break
					}
				}
				if !found {
					order := groupOrders[orderIndex[orderID]]
					when, source := "", "快递回传记录"
					if pickup != "" {
						when, source = pickup, "快递揽件记录"
					}
					for i := range groups {
						if groups[i].Order.ID == orderID {
							groups[i].Shipments = append(groups[i].Shipments, app.AccountShipment{OrderID: orderID, OrderNo: order.OrderNo, TrackingNo: tracking, ShippedAt: when, PickupAt: pickup, SettlementWeightKG: weight, Source: source})
							shipmentKeys[fmt.Sprintf("%d:%s", orderID, tracking)] = true
							break
						}
					}
				}
			}
			if queryErr == nil {
				queryErr = returns.Err()
			}
			returns.Close()
			if queryErr != nil {
				return d, queryErr
			}
		}
		for _, fee := range d.Fees {
			if fee.OrderID == 0 || fee.TrackingNo == "" {
				continue
			}
			for i := range groups {
				if groups[i].Order.ID != fee.OrderID {
					continue
				}
				found := false
				for j := range groups[i].Shipments {
					if groups[i].Shipments[j].TrackingNo == fee.TrackingNo {
						groups[i].Shipments[j].ShippingCents += fee.AmountCents
						found = true
						break
					}
				}
				if !found {
					groups[i].Shipments = append(groups[i].Shipments, app.AccountShipment{OrderID: fee.OrderID, OrderNo: fee.OrderNo, TrackingNo: fee.TrackingNo, ShippingCents: fee.AmountCents, Source: "快递费用记录"})
					shipmentKeys[fmt.Sprintf("%d:%s", fee.OrderID, fee.TrackingNo)] = true
				}
			}
		}
		d.OrderFeeGroups = groups
		d.FeeSummary = feeSummary
	}
	bills, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id,settlement_no,coalesce(to_char(period_from,'YYYY-MM-DD'),''),coalesce(to_char(period_to,'YYYY-MM-DD'),''),
		       status,round(total_amount*100)::bigint,
		       coalesce(to_char(confirmed_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD HH24:MI'),''),
		       coalesce(to_char(paid_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD HH24:MI'),'')
		FROM %s.customer_settlement_batches
		WHERE customer_id=$1 AND status IN ('confirmed','paid','reversed','settled')
		  AND ($2='' OR COALESCE(period_to,period_from)>=$2::date)
		  AND ($3='' OR COALESCE(period_from,period_to)<=$3::date)
		ORDER BY created_at DESC,id DESC
	`, r.schema), q.CustomerID, q.DateFrom, q.DateTo)
	if err != nil {
		return d, err
	}
	for bills.Next() {
		var b app.AccountSettlement
		if err = bills.Scan(&b.ID, &b.SettlementNo, &b.PeriodFrom, &b.PeriodTo, &b.Status, &b.TotalCents, &b.ConfirmedAt, &b.PaidAt); err != nil {
			bills.Close()
			return d, err
		}
		d.Settlements = append(d.Settlements, b)
	}
	err = bills.Err()
	bills.Close()
	if err != nil {
		return d, err
	}
	for idx := range d.Settlements {
		b := &d.Settlements[idx]
		b.Fees = []app.AccountFee{}
		for _, f := range allFees {
			if f.SettlementID == b.ID {
				b.Fees = append(b.Fees, f)
			}
		}
		if relationExists(ctx, tx, fmt.Sprintf("%s.customer_settlement_lines", r.schema)) {
			b.Lines, err = customerSettlementLinesTx(ctx, tx, r.schema, q.CustomerID, b.ID)
			if err != nil {
				return d, err
			}
		}
		b.StatementRevision, err = customerStatementRevisionTx(ctx, tx, r.schema, q.CustomerID, b.ID)
		if err != nil {
			return d, err
		}
		if err = loadCustomerStatementReconciliationTx(ctx, tx, r.schema, q.CustomerID, b); err != nil {
			return d, err
		}
	}
	d.Summary.PayableCents = d.Summary.TotalCents + d.Summary.AdditionalFeeCents
	d.Summary.DueCents = d.Summary.PayableCents - d.Summary.PaidCents
	if d.Summary.DueCents < 0 {
		d.Summary.DueCents = 0
	}
	return d, tx.Commit(ctx)
}

func customerStatementRevisionTx(ctx context.Context, tx pgx.Tx, schema string, customerID, settlementID int64) (string, error) {
	hash := sha256.New()
	var settlementNo, periodFrom, periodTo, total string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT settlement_no,COALESCE(period_from::text,''),COALESCE(period_to::text,''),total_amount::text
		FROM %s.customer_settlement_batches WHERE id=$1 AND customer_id=$2
	`, schema), settlementID, customerID).Scan(&settlementNo, &periodFrom, &periodTo, &total); err != nil {
		return "", err
	}
	fmt.Fprintf(hash, "%s|%s|%s|%s\n", settlementNo, periodFrom, periodTo, total)
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id,source_type,source_id,fee_type,amount::text,currency
		FROM %s.customer_fee_items
		WHERE customer_id=$1 AND settlement_batch_id=$2
		ORDER BY source_type,source_id,fee_type,id
	`, schema), customerID, settlementID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	for rows.Next() {
		var id, sourceID int64
		var sourceType, feeType, amount, currency string
		if err := rows.Scan(&id, &sourceType, &sourceID, &feeType, &amount, &currency); err != nil {
			return "", err
		}
		fmt.Fprintf(hash, "%d|%s|%d|%s|%s|%s\n", id, sourceType, sourceID, feeType, amount, currency)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if relationExists(ctx, tx, fmt.Sprintf("%s.customer_settlement_lines", schema)) {
		lines, err := tx.Query(ctx, fmt.Sprintf(`
			SELECT l.charge_type,l.source_type,l.source_id,l.source_no,l.description,l.occurred_at::text,
				l.receivable_cents,l.amount_cents,l.received_cents,l.due_cents,l.confirmation_status,COALESCE(b.status,l.settlement_status)
			FROM %s.customer_settlement_lines l
			LEFT JOIN %s.customer_settlement_batches b ON b.id=l.settlement_batch_id AND b.customer_id=l.customer_id
			WHERE l.customer_id=$1 AND l.settlement_batch_id=$2 AND l.reversed_at IS NULL
			ORDER BY l.occurred_at,l.charge_type,l.source_type,l.source_id,l.id
		`, schema, schema), customerID, settlementID)
		if err != nil {
			return "", err
		}
		defer lines.Close()
		for lines.Next() {
			var line app.SettlementLineSummary
			if err := lines.Scan(&line.ChargeType, &line.SourceType, &line.SourceID, &line.SourceNo, &line.Description, &line.OccurredAt,
				&line.ReceivableCents, &line.AmountCents, &line.ReceivedCents, &line.DueCents, &line.ConfirmationStatus, &line.SettlementStatus); err != nil {
				return "", err
			}
			fmt.Fprintf(hash, "line|%s|%s|%d|%s|%s|%s|%d|%d|%d|%d|%s|%s\n", line.ChargeType, line.SourceType, line.SourceID, line.SourceNo, line.Description, line.OccurredAt,
				line.ReceivableCents, line.AmountCents, line.ReceivedCents, line.DueCents, line.ConfirmationStatus, line.SettlementStatus)
		}
		if err := lines.Err(); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func customerSettlementLinesTx(ctx context.Context, tx pgx.Tx, schema string, customerID, settlementID int64) ([]app.SettlementLineSummary, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
			SELECT l.id,l.charge_type,l.source_type,l.source_id,l.source_no,l.description,l.occurred_at::text,
				l.receivable_cents,l.amount_cents,l.received_cents,l.due_cents,l.confirmation_status,COALESCE(b.status,l.settlement_status)
			FROM %s.customer_settlement_lines l
			LEFT JOIN %s.customer_settlement_batches b ON b.id=l.settlement_batch_id AND b.customer_id=l.customer_id
			WHERE l.customer_id=$1 AND l.settlement_batch_id=$2 AND l.reversed_at IS NULL
			ORDER BY l.occurred_at,l.charge_type,l.source_type,l.source_id,l.id
		`, schema, schema), customerID, settlementID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]app.SettlementLineSummary, 0)
	for rows.Next() {
		var line app.SettlementLineSummary
		if err := rows.Scan(&line.ID, &line.ChargeType, &line.SourceType, &line.SourceID, &line.SourceNo, &line.Description, &line.OccurredAt,
			&line.ReceivableCents, &line.AmountCents, &line.ReceivedCents, &line.DueCents, &line.ConfirmationStatus, &line.SettlementStatus); err != nil {
			return nil, err
		}
		out = append(out, line)
	}
	return out, rows.Err()
}

func loadCustomerStatementReconciliationTx(ctx context.Context, tx pgx.Tx, schema string, customerID int64, settlement *app.AccountSettlement) error {
	settlement.ReconciliationStatus = "pending"
	settlement.Disputes = []app.AccountStatementDispute{}
	var revision, status, reconciledAt string
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT statement_revision,status,to_char(confirmed_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD HH24:MI')
		FROM %s.customer_statement_reconciliations
		WHERE customer_id=$1 AND settlement_id=$2
	`, schema), customerID, settlement.ID).Scan(&revision, &status, &reconciledAt)
	if err == nil {
		if revision == settlement.StatementRevision && status == "confirmed" {
			settlement.ReconciliationStatus = "confirmed"
			settlement.ReconciledAt = reconciledAt
		} else {
			settlement.ReconciliationStatus = "changed"
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id,settlement_id,fee_item_id,statement_revision,reason,status,
		       to_char(created_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD HH24:MI'),
		       reply,replied_by,COALESCE(to_char(replied_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD HH24:MI'),'')
		FROM %s.customer_statement_disputes
		WHERE customer_id=$1 AND settlement_id=$2 ORDER BY created_at DESC,id DESC
	`, schema), customerID, settlement.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var dispute app.AccountStatementDispute
		if err := rows.Scan(&dispute.ID, &dispute.SettlementID, &dispute.FeeItemID, &dispute.StatementRevision,
			&dispute.Reason, &dispute.Status, &dispute.CreatedAt, &dispute.Reply, &dispute.RepliedBy, &dispute.RepliedAt); err != nil {
			return err
		}
		settlement.Disputes = append(settlement.Disputes, dispute)
		if dispute.Status == "open" || dispute.Status == "replied" {
			settlement.ReconciliationStatus = "disputed"
		}
	}
	return rows.Err()
}

func (r *Repository) ConfirmCustomerStatement(ctx context.Context, cmd app.ConfirmCustomerStatementCommand) (app.AccountSettlement, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return app.AccountSettlement{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, fmt.Sprintf(`SELECT id FROM %s.customer_settlement_batches WHERE id=$1 AND customer_id=$2 FOR UPDATE`, r.schema), cmd.SettlementID, cmd.CustomerID); err != nil {
		return app.AccountSettlement{}, err
	}
	current, err := customerStatementRevisionTx(ctx, tx, r.schema, cmd.CustomerID, cmd.SettlementID)
	if err != nil {
		return app.AccountSettlement{}, err
	}
	if current != cmd.StatementRevision {
		return app.AccountSettlement{}, fmt.Errorf("账单内容已更新，请刷新后重新确认")
	}
	var openDispute bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.customer_statement_disputes WHERE customer_id=$1 AND settlement_id=$2 AND status IN ('open','replied'))`, r.schema), cmd.CustomerID, cmd.SettlementID).Scan(&openDispute); err != nil {
		return app.AccountSettlement{}, err
	}
	if openDispute {
		return app.AccountSettlement{}, fmt.Errorf("账单仍有未处理异议")
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_statement_reconciliations(customer_id,settlement_id,statement_revision,status,confirmed_by_mini_user_id,confirmed_by,confirmed_at,updated_at)
		VALUES($1,$2,$3,'confirmed',$4,$5,now(),now())
		ON CONFLICT(settlement_id) DO UPDATE SET statement_revision=excluded.statement_revision,status='confirmed',
			confirmed_by_mini_user_id=excluded.confirmed_by_mini_user_id,confirmed_by=excluded.confirmed_by,confirmed_at=now(),updated_at=now()
	`, r.schema), cmd.CustomerID, cmd.SettlementID, current, cmd.MiniUserID, cmd.Actor); err != nil {
		return app.AccountSettlement{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "customer_statement_reconciliation", &cmd.SettlementID, "confirm", postgresinfra.StrPtr("statement_revision"), nil, postgresinfra.StrPtr(current), postgresinfra.AuditMeta{
		"customer_id": cmd.CustomerID, "settlement_id": cmd.SettlementID, "mini_user_id": cmd.MiniUserID,
	}); err != nil {
		return app.AccountSettlement{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return app.AccountSettlement{}, err
	}
	return r.customerAccountSettlement(ctx, cmd.CustomerID, cmd.SettlementID)
}

func (r *Repository) CreateCustomerStatementDispute(ctx context.Context, cmd app.CreateCustomerStatementDisputeCommand) (app.AccountStatementDispute, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return app.AccountStatementDispute{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, fmt.Sprintf(`SELECT id FROM %s.customer_settlement_batches WHERE id=$1 AND customer_id=$2 FOR UPDATE`, r.schema), cmd.SettlementID, cmd.CustomerID); err != nil {
		return app.AccountStatementDispute{}, err
	}
	current, err := customerStatementRevisionTx(ctx, tx, r.schema, cmd.CustomerID, cmd.SettlementID)
	if err != nil {
		return app.AccountStatementDispute{}, err
	}
	if current != cmd.StatementRevision {
		return app.AccountStatementDispute{}, fmt.Errorf("账单内容已更新，请刷新后再提出异议")
	}
	if cmd.FeeItemID > 0 {
		var valid bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.customer_fee_items WHERE id=$1 AND customer_id=$2 AND settlement_batch_id=$3)`, r.schema), cmd.FeeItemID, cmd.CustomerID, cmd.SettlementID).Scan(&valid); err != nil {
			return app.AccountStatementDispute{}, err
		}
		if !valid {
			return app.AccountStatementDispute{}, fmt.Errorf("费用项不属于当前账单")
		}
	}
	var id int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_statement_disputes(customer_id,settlement_id,fee_item_id,statement_revision,reason,status,created_by_mini_user_id,created_by)
		VALUES($1,$2,$3,$4,$5,'open',$6,$7) RETURNING id
	`, r.schema), cmd.CustomerID, cmd.SettlementID, cmd.FeeItemID, current, cmd.Reason, cmd.MiniUserID, cmd.Actor).Scan(&id); err != nil {
		return app.AccountStatementDispute{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "customer_statement_dispute", &id, "create", postgresinfra.StrPtr("status"), nil, postgresinfra.StrPtr("open"), postgresinfra.AuditMeta{
		"customer_id": cmd.CustomerID, "settlement_id": cmd.SettlementID, "fee_item_id": cmd.FeeItemID,
	}); err != nil {
		return app.AccountStatementDispute{}, err
	}
	dispute, err := readCustomerStatementDisputeTx(ctx, tx, r.schema, id)
	if err != nil {
		return app.AccountStatementDispute{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return app.AccountStatementDispute{}, err
	}
	return dispute, nil
}

func (r *Repository) ReplyCustomerStatementDispute(ctx context.Context, cmd app.ReplyCustomerStatementDisputeCommand) (app.AccountStatementDispute, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return app.AccountStatementDispute{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var customerID, settlementID int64
	var oldStatus string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT customer_id,settlement_id,status FROM %s.customer_statement_disputes WHERE id=$1 FOR UPDATE`, r.schema), cmd.DisputeID).Scan(&customerID, &settlementID, &oldStatus); err != nil {
		return app.AccountStatementDispute{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_statement_disputes SET reply=$2,replied_by=$3,replied_at=now(),status=$4,updated_at=now() WHERE id=$1`, r.schema), cmd.DisputeID, cmd.Reply, cmd.Actor, cmd.Status); err != nil {
		return app.AccountStatementDispute{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "customer_statement_dispute", &cmd.DisputeID, "reply", postgresinfra.StrPtr("status"), postgresinfra.StrPtr(oldStatus), postgresinfra.StrPtr(cmd.Status), postgresinfra.AuditMeta{
		"customer_id": customerID, "settlement_id": settlementID,
	}); err != nil {
		return app.AccountStatementDispute{}, err
	}
	dispute, err := readCustomerStatementDisputeTx(ctx, tx, r.schema, cmd.DisputeID)
	if err != nil {
		return app.AccountStatementDispute{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return app.AccountStatementDispute{}, err
	}
	return dispute, nil
}

func readCustomerStatementDisputeTx(ctx context.Context, tx pgx.Tx, schema string, id int64) (app.AccountStatementDispute, error) {
	var row app.AccountStatementDispute
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id,settlement_id,fee_item_id,statement_revision,reason,status,
		       to_char(created_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD HH24:MI'),reply,replied_by,
		       COALESCE(to_char(replied_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD HH24:MI'),'')
		FROM %s.customer_statement_disputes WHERE id=$1
	`, schema), id).Scan(&row.ID, &row.SettlementID, &row.FeeItemID, &row.StatementRevision, &row.Reason,
		&row.Status, &row.CreatedAt, &row.Reply, &row.RepliedBy, &row.RepliedAt)
	return row, err
}

func (r *Repository) customerAccountSettlement(ctx context.Context, customerID, settlementID int64) (app.AccountSettlement, error) {
	data, err := r.CustomerAccount(ctx, app.AccountQuery{CustomerID: customerID, Page: 1, Limit: 200})
	if err != nil {
		return app.AccountSettlement{}, err
	}
	for _, settlement := range data.Settlements {
		if settlement.ID == settlementID {
			return settlement, nil
		}
	}
	return app.AccountSettlement{}, fmt.Errorf("账单不存在或不属于当前客户")
}
