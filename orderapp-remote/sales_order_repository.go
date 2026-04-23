package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	salesapp "orderapp/internal/application/sales"
	salesdomain "orderapp/internal/domain/sales"

	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresSalesRepository struct {
	pool   *pgxpool.Pool
	schema string
}

func (r postgresSalesRepository) SaveOrder(ctx context.Context, cmd salesapp.SaveOrderCommand) (salesapp.SaveOrderResult, error) {
	orderDate := strings.TrimSpace(cmd.OrderDate)
	if orderDate == "" {
		orderDate = time.Now().Format("2006-01-02")
	}
	od, err := time.Parse("2006-01-02", orderDate)
	if err != nil {
		return salesapp.SaveOrderResult{}, fmt.Errorf("invalid order_date")
	}
	if cmd.CustomerID <= 0 {
		return salesapp.SaveOrderResult{}, fmt.Errorf("customer required")
	}

	type item struct {
		productID     *int64
		tierID        *int64
		manualPrice   *float64
		name          string
		units         int64
		specG         int64
		unit          *string
		spec          *string
		unitPrice     float64
		lineTotal     float64
		priceOverride bool
	}
	items := make([]item, 0)
	for i := 0; i < maxLen(cmd.ItemName, cmd.ProductID, cmd.TierID, cmd.UnitPrice, cmd.Qty, cmd.Unit, cmd.Spec); i++ {
		pidStr := strings.TrimSpace(getStr(cmd.ProductID, i))
		name := strings.TrimSpace(getStr(cmd.ItemName, i))

		// If no product and no name, skip row.
		if pidStr == "" && name == "" {
			continue
		}

		it := item{name: name}
		if pidStr != "" {
			if pid, err := strconv.ParseInt(pidStr, 10, 64); err == nil && pid > 0 {
				it.productID = &pid
			}
		}
		if tidStr := strings.TrimSpace(getStr(cmd.TierID, i)); tidStr != "" && tidStr != "auto" {
			if tidStr == "manual" {
				if v := strings.TrimSpace(getStr(cmd.UnitPrice, i)); v != "" {
					if f, err := strconv.ParseFloat(v, 64); err == nil {
						it.manualPrice = &f
						it.priceOverride = true
					}
				}
			} else {
				if tid, err := strconv.ParseInt(tidStr, 10, 64); err == nil && tid > 0 {
					it.tierID = &tid
				}
			}
		}
		if q := strings.TrimSpace(getStr(cmd.Qty, i)); q != "" {
			if n, err := strconv.ParseInt(q, 10, 64); err == nil && n > 0 {
				it.units = n
			}
		}
		if sg := strings.TrimSpace(getStr(cmd.Spec, i)); sg != "" {
			// spec is grams (e.g. "227" or "227g")
			sg = strings.TrimSuffix(strings.ToLower(sg), "g")
			if n, err := strconv.ParseInt(sg, 10, 64); err == nil && n > 0 {
				it.specG = n
				ss := fmt.Sprintf("%dg", n)
				it.spec = &ss
			}
		}
		if u := strings.TrimSpace(getStr(cmd.Unit, i)); u != "" {
			it.unit = &u
		}
		items = append(items, it)
	}
	// Validate: need at least one item with spec+units
	valid := false
	for _, it := range items {
		if it.productID != nil && it.specG > 0 && it.units > 0 {
			valid = true
			break
		}
	}
	if !valid {
		return salesapp.SaveOrderResult{}, fmt.Errorf("at least one item required")
	}

	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return salesapp.SaveOrderResult{}, err
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return salesapp.SaveOrderResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, fmt.Sprintf("LOCK TABLE %s.orders IN SHARE ROW EXCLUSIVE MODE", r.schema)); err != nil {
		return salesapp.SaveOrderResult{}, err
	}

	orderNo := ""
	retailOrder := false
	if cmd.OrderTypeID > 0 {
		var orderTypeName string
		_ = tx.QueryRow(ctx, fmt.Sprintf("SELECT COALESCE(name,'') FROM %s.order_types WHERE id=$1", r.schema), cmd.OrderTypeID).Scan(&orderTypeName)
		retailOrder = isRetailOrderTypeName(orderTypeName)
	}

	// Pricing: use tier match by qty(lb). Allow manual override.
	totalAmt := 0.0
	orderWeightG := int64(0)
	for idx := range items {
		itemWeightG := items[idx].specG * items[idx].units
		orderWeightG += itemWeightG
		totalG := float64(itemWeightG)
		qtyLb := totalG / 454.0

		if retailOrder && items[idx].productID != nil {
			retailPrice227G := 0.0
			q := fmt.Sprintf(`SELECT COALESCE(NULLIF(retail_price_227g,0), default_price, 0) FROM %s.products WHERE id=$1`, r.schema)
			_ = tx.QueryRow(ctx, q, *items[idx].productID).Scan(&retailPrice227G)
			_, lineTotal := salesdomain.RetailLinePrice(retailPrice227G, items[idx].specG, items[idx].units)
			items[idx].lineTotal = lineTotal
			if qtyLb > 0 {
				items[idx].unitPrice = lineTotal / qtyLb
			}
			totalAmt += items[idx].lineTotal
			continue
		} else if items[idx].manualPrice != nil {
			// manual price is 元/磅
			items[idx].unitPrice = *items[idx].manualPrice
			items[idx].priceOverride = true
		} else if items[idx].productID != nil {
			// If user selected a tier explicitly
			if items[idx].tierID != nil {
				var price float64
				q := fmt.Sprintf(`SELECT price_per_lb FROM %s.product_price_tiers WHERE id=$1 AND active=true`, r.schema)
				if err := tx.QueryRow(ctx, q, *items[idx].tierID).Scan(&price); err != nil {
					return salesapp.SaveOrderResult{}, fmt.Errorf("invalid tier")
				}
				items[idx].unitPrice = price
			} else {
				// Auto-match tier by qty(lb)
				var tid *int64
				var price float64
				q := fmt.Sprintf(`
						SELECT id, price_per_lb
						FROM %s.product_price_tiers
						WHERE product_id=$1 AND active=true
						  AND min_qty_lb <= $2
						  AND (max_qty_lb IS NULL OR max_qty_lb >= $2)
						ORDER BY min_qty_lb DESC
						LIMIT 1
					`, r.schema)
				err := tx.QueryRow(ctx, q, *items[idx].productID, qtyLb).Scan(&tid, &price)
				if err != nil {
					// fallback: highest tier with min<=qty
					q2 := fmt.Sprintf(`
							SELECT id, price_per_lb
							FROM %s.product_price_tiers
							WHERE product_id=$1 AND active=true AND min_qty_lb <= $2
							ORDER BY min_qty_lb DESC
							LIMIT 1
						`, r.schema)
					if err2 := tx.QueryRow(ctx, q2, *items[idx].productID, qtyLb).Scan(&tid, &price); err2 != nil {
						// below minimum tier: use minimum tier price
						q3 := fmt.Sprintf(`
								SELECT id, price_per_lb
								FROM %s.product_price_tiers
								WHERE product_id=$1 AND active=true
								ORDER BY min_qty_lb ASC
								LIMIT 1
							`, r.schema)
						if err3 := tx.QueryRow(ctx, q3, *items[idx].productID).Scan(&tid, &price); err3 != nil {
							price = 0
							tid = nil
						}
					}
				}
				items[idx].tierID = tid
				items[idx].unitPrice = price
			}
		}

		items[idx].lineTotal = qtyLb * items[idx].unitPrice
		totalAmt += items[idx].lineTotal
	}

	// Amount calculation (items + shipping - discount)
	shippingAmt := 0.0
	if v := strings.TrimSpace(cmd.ShippingAmount); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return salesapp.SaveOrderResult{}, fmt.Errorf("invalid shipping_amount")
		}
		shippingAmt = f
	}
	discountAmt := 0.0
	if v := strings.TrimSpace(cmd.DiscountAmount); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return salesapp.SaveOrderResult{}, fmt.Errorf("invalid discount_amount")
		}
		discountAmt = f
	}
	roundToInt := strings.TrimSpace(cmd.RoundToInt) != ""
	outsourceTotal, outsourceFees, err := calcOutsourceTotal(&cmd)
	if err != nil {
		return salesapp.SaveOrderResult{}, err
	}
	grand0 := totalAmt + shippingAmt - discountAmt + outsourceTotal
	grandTotal, roundingAmt := applyRoundToInt(grand0, roundToInt)

	// 默认发货状态：未选择时自动写入“未发货”。
	shipStatusID := cmd.ShipStatusID
	if shipStatusID == 0 {
		_ = tx.QueryRow(ctx, fmt.Sprintf("SELECT id FROM %s.ship_statuses WHERE name='未发货' ORDER BY id LIMIT 1", r.schema)).Scan(&shipStatusID)
	}

	shipMethod := strings.TrimSpace(cmd.ShipMethod)
	if shipMethod == "" {
		if orderWeightG <= 15000 {
			shipMethod = "sf_small"
		} else {
			shipMethod = "sf_large"
		}
	}

	editID := cmd.EditID

	insertItemSQL := fmt.Sprintf(`INSERT INTO %s.order_items(order_id,line_no,product_id,price_tier_id,price_overridden,item_name,qty,unit,spec,unit_price,line_total)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, r.schema)

	var orderID int64
	if editID > 0 {
		if err := tx.QueryRow(ctx, fmt.Sprintf("SELECT id, order_no FROM %s.orders WHERE id=$1 FOR UPDATE", r.schema), editID).Scan(&orderID, &orderNo); err != nil {
			return salesapp.SaveOrderResult{}, fmt.Errorf("invalid edit_id")
		}
		uq := fmt.Sprintf(`
				UPDATE %s.orders
				SET order_date=$2,
					customer_id=$3,
					source_id=$4,
					order_type_id=$5,
					pay_status_id=$6,
					ship_status_id=$7,
					ship_method=$8,
					ship_tracking_no=$9,
					notes=$10,
					total_amount=$11,
					shipping_amount=$12,
					discount_amount=$13,
					round_to_int=$14,
					rounding_amount=$15,
					grand_total=$16,
					express_fee=$17,
					outsource_material_fee=$18,
					outsource_roast_fee=$19,
					outsource_packaging_fee=$20,
					outsource_manual_fee=$21,
					outsource_tax_fee=$22,
					outsource_other_fee=$23,
					outsource_total_fee=$24
				WHERE id=$1
			`, r.schema)
		if _, err := tx.Exec(ctx, uq,
			orderID,
			od,
			cmd.CustomerID,
			nullInt(cmd.SourceID),
			nullInt(cmd.OrderTypeID),
			nullInt(cmd.PayStatusID),
			nullInt(shipStatusID),
			nullText(shipMethod),
			nullText(cmd.ShipTrackingNo),
			nullText(cmd.Notes),
			totalAmt,
			shippingAmt,
			discountAmt,
			roundToInt,
			roundingAmt,
			grandTotal,
			nullText(cmd.ExpressFee),
			outsourceFees[0],
			outsourceFees[1],
			outsourceFees[2],
			outsourceFees[3],
			outsourceFees[4],
			outsourceFees[5],
			outsourceTotal,
		); err != nil {
			return salesapp.SaveOrderResult{}, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s.order_items WHERE order_id=$1", r.schema), orderID); err != nil {
			return salesapp.SaveOrderResult{}, err
		}
	} else {
		orderNo, err = nextOrderNo(ctx, tx, r.schema, od)
		if err != nil {
			return salesapp.SaveOrderResult{}, err
		}
		insertOrderSQL := fmt.Sprintf(`
				INSERT INTO %s.orders(
					order_date, customer_id,
					source_id, order_type_id, pay_status_id, ship_status_id,
					ship_method, ship_tracking_no,
					notes,
					total_amount, shipping_amount, discount_amount,
					round_to_int, rounding_amount, grand_total,
					express_fee,
					outsource_material_fee, outsource_roast_fee, outsource_packaging_fee,
					outsource_manual_fee, outsource_tax_fee, outsource_other_fee, outsource_total_fee,
					order_no
				) VALUES (
					$1,$2,$3,$4,$5,$6,$7,$8,$9,
					$10,$11,$12,
					$13,$14,$15,
					$16,$17,$18,$19,$20,$21,$22,$23,
					$24
				)
				RETURNING id
			`, r.schema)
		err = tx.QueryRow(ctx, insertOrderSQL,
			od,
			cmd.CustomerID,
			nullInt(cmd.SourceID),
			nullInt(cmd.OrderTypeID),
			nullInt(cmd.PayStatusID),
			nullInt(shipStatusID),
			nullText(shipMethod),
			nullText(cmd.ShipTrackingNo),
			nullText(cmd.Notes),
			totalAmt,
			shippingAmt,
			discountAmt,
			roundToInt,
			roundingAmt,
			grandTotal,
			nullText(cmd.ExpressFee),
			outsourceFees[0],
			outsourceFees[1],
			outsourceFees[2],
			outsourceFees[3],
			outsourceFees[4],
			outsourceFees[5],
			outsourceTotal,
			orderNo,
		).Scan(&orderID)
		if err != nil {
			return salesapp.SaveOrderResult{}, err
		}
	}

	for idx, it := range items {
		qtyAny := any(nil)
		if it.units > 0 {
			qtyAny = it.units
		}
		if _, err := tx.Exec(ctx, insertItemSQL, orderID, idx+1, it.productID, it.tierID, it.priceOverride, it.name, qtyAny, it.unit, it.spec, it.unitPrice, it.lineTotal); err != nil {
			return salesapp.SaveOrderResult{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return salesapp.SaveOrderResult{}, err
	}

	r.logOrderSave(ctx, cmd.Actor, orderID, orderNo, editID > 0)

	return salesapp.SaveOrderResult{OrderID: orderID, OrderNo: orderNo, Edited: editID > 0}, nil

}

func (r postgresSalesRepository) UpdateHeader(ctx context.Context, id int64, cmd salesapp.UpdateHeaderCommand) error {
	req := UpdateOrderRequest{
		OrderDate:             cmd.OrderDate,
		CustomerID:            cmd.CustomerID,
		SourceID:              cmd.SourceID,
		OrderTypeID:           cmd.OrderTypeID,
		PayStatusID:           cmd.PayStatusID,
		ShipStatusID:          cmd.ShipStatusID,
		ShipMethod:            cmd.ShipMethod,
		ShipTrackingNo:        cmd.ShipTrackingNo,
		Notes:                 cmd.Notes,
		ShippingAmount:        cmd.ShippingAmount,
		DiscountAmount:        cmd.DiscountAmount,
		RoundToInt:            cmd.RoundToInt,
		ExpressFee:            cmd.ExpressFee,
		OutsourceMaterialFee:  cmd.OutsourceMaterialFee,
		OutsourceRoastFee:     cmd.OutsourceRoastFee,
		OutsourcePackagingFee: cmd.OutsourcePackagingFee,
		OutsourceManualFee:    cmd.OutsourceManualFee,
		OutsourceTaxFee:       cmd.OutsourceTaxFee,
		OutsourceOtherFee:     cmd.OutsourceOtherFee,
		ItemID:                cmd.ItemID,
		Qty:                   cmd.Qty,
		UnitPrice:             cmd.UnitPrice,
	}
	if err := updateOrderHeader(ctx, r.pool, r.schema, id, &req); err != nil {
		return err
	}
	r.logOrderHeaderUpdate(ctx, cmd.Actor, id)
	return nil
}

func (r postgresSalesRepository) InlineUpdate(ctx context.Context, id int64, actor string, cmd salesapp.InlineUpdateCommand) error {
	req := InlineUpdateRequest{
		OrderTypeID:     cmd.OrderTypeID,
		PayStatusID:     cmd.PayStatusID,
		ShipStatusID:    cmd.ShipStatusID,
		ProcessStatusID: cmd.ProcessStatusID,
		Notes:           cmd.Notes,
	}
	return inlineUpdateOrder(ctx, r.pool, r.schema, id, actor, &req)
}

func (r postgresSalesRepository) Void(ctx context.Context, id int64, actor, reason string) error {
	q := fmt.Sprintf("UPDATE %s.orders SET is_void=true, voided_at=now(), void_reason=$2 WHERE id=$1", r.schema)
	if _, err := r.pool.Exec(ctx, q, id, nullText(reason)); err != nil {
		return err
	}
	var rv *string
	if strings.TrimSpace(reason) != "" {
		rv = &reason
	}
	auditInsert(ctx, r.pool, r.schema, actor, "order", &id, "void", nil, nil, rv, AuditMeta{"order_id": id})
	return nil
}

func (r postgresSalesRepository) Unvoid(ctx context.Context, id int64, actor string) error {
	q := fmt.Sprintf("UPDATE %s.orders SET is_void=false, voided_at=NULL, void_reason=NULL WHERE id=$1", r.schema)
	if _, err := r.pool.Exec(ctx, q, id); err != nil {
		return err
	}
	auditInsert(ctx, r.pool, r.schema, actor, "order", &id, "unvoid", nil, nil, nil, AuditMeta{"order_id": id})
	return nil
}

func (r postgresSalesRepository) logOrderSave(ctx context.Context, actor string, orderID int64, orderNo string, edited bool) {
	action := "create"
	field := "created"
	newValue := orderNo
	if edited {
		action = "update"
		field = "order"
		newValue = "updated"
	}
	r.insertOrderAudit(ctx, actor, orderID, field, nil, strPtrStr(newValue))
	auditInsert(ctx, r.pool, r.schema, actor, "order", &orderID, action, strPtrStr(field), nil, strPtrStr(newValue), AuditMeta{"order_id": orderID, "order_no": orderNo})
}

func (r postgresSalesRepository) logOrderHeaderUpdate(ctx context.Context, actor string, orderID int64) {
	r.insertOrderAudit(ctx, actor, orderID, "header", nil, strPtrStr("updated"))
	auditInsert(ctx, r.pool, r.schema, actor, "order", &orderID, "update", strPtrStr("header"), nil, strPtrStr("updated"), AuditMeta{"order_id": orderID})
}

func (r postgresSalesRepository) insertOrderAudit(ctx context.Context, actor string, orderID int64, field string, oldValue, newValue *string) {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "unknown"
	}
	_, _ = r.pool.Exec(ctx,
		fmt.Sprintf(`INSERT INTO %s.order_audit_logs(order_id, actor, field, old_value, new_value) VALUES ($1,$2,$3,$4,$5)`, r.schema),
		orderID,
		actor,
		field,
		oldValue,
		newValue,
	)
}
