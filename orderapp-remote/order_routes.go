package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func registerOrderRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	h := orderHandler{pool: pool, schema: schema}

	// Orders list
	e.GET("/orders", h.index)

	e.POST("/orders/:id/inline", h.inlineUpdate)

	e.GET("/orders/:id/audit", h.audit)

	// Merged detail+edit: clicking order number goes to unified edit page.
	e.GET("/orders/:id", h.detailRedirect)

	// Unified edit: reuse /order page logic.
	e.GET("/orders/:id/edit", h.editRedirect)
	e.POST("/orders/:id/edit", h.editPost)

	// Void / unvoid
	e.POST("/orders/:id/void", h.void)
	e.POST("/orders/:id/unvoid", h.unvoid)

	// Create order
	e.GET("/order", h.entry)

	e.POST("/order", h.create)

}

type orderHandler struct {
	pool   *pgxpool.Pool
	schema string
}

func (h orderHandler) index(c echo.Context) error {
	data := OrdersPageData{
		Q:      strings.TrimSpace(c.QueryParam("q")),
		From:   strings.TrimSpace(c.QueryParam("from")),
		To:     strings.TrimSpace(c.QueryParam("to")),
		Preset: strings.TrimSpace(c.QueryParam("preset")),
		Void:   strings.TrimSpace(c.QueryParam("void")),
		Limit:  10,
		Offset: 0,
	}
	if v := strings.TrimSpace(c.QueryParam("customer_id")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			data.CustomerID = n
		}
	}
	if v := strings.TrimSpace(c.QueryParam("pay_status_id")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			data.PayStatusFilter = n
		}
	}
	if v := strings.TrimSpace(c.QueryParam("ship_status_id")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			data.ShipStatusFilter = n
		}
	}
	if v := strings.TrimSpace(c.QueryParam("process_status_id")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			data.ProcStatusFilter = n
		}
	}
	data.CompletedOnly = strings.TrimSpace(c.QueryParam("completed")) == "1"
	if data.Preset == "unprod" {
		data.UnproducedOnly = true
	}
	if data.Void == "" {
		data.Void = "normal"
	}
	if v := strings.TrimSpace(c.QueryParam("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			data.Limit = n
		}
	}
	if v := strings.TrimSpace(c.QueryParam("offset")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			data.Offset = n
		}
	}
	// page is 1-indexed; overrides offset when provided
	if v := strings.TrimSpace(c.QueryParam("page")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			data.Offset = (n - 1) * data.Limit
		}
	}

	if data.Limit > 0 {
		data.Page = (data.Offset / data.Limit) + 1
	} else {
		data.Page = 1
	}

	rows, hasNext, errOrders := fetchOrders(c.Request().Context(), h.pool, h.schema, data.Q, data.From, data.To, data.Void, data.CustomerID, data.PayStatusFilter, data.ShipStatusFilter, data.ProcStatusFilter, data.UnproducedOnly, data.CompletedOnly, data.Limit, data.Offset)
	if opts, err := fetchOptions(c.Request().Context(), h.pool, fmt.Sprintf("SELECT id, name FROM %s.order_types ORDER BY id", h.schema)); err == nil {
		data.OrderTypeOpts = opts
	} else {
		data.OrderTypeOpts = nil
	}
	if opts, err := fetchOptions(c.Request().Context(), h.pool, fmt.Sprintf("SELECT id, name FROM %s.pay_statuses ORDER BY id", h.schema)); err == nil {
		data.PayOpts = opts
	} else {
		data.PayOpts = nil
	}
	if opts, err := fetchOptions(c.Request().Context(), h.pool, fmt.Sprintf("SELECT id, name FROM %s.ship_statuses ORDER BY id", h.schema)); err == nil {
		data.ShipOpts = opts
	} else {
		data.ShipOpts = nil
	}
	if opts, err := fetchOptions(c.Request().Context(), h.pool, fmt.Sprintf("SELECT id, name FROM %s.order_process_statuses WHERE active=true ORDER BY sort,id", h.schema)); err == nil {
		data.ProcessOpts = opts
	} else {
		data.ProcessOpts = nil
	}
	// summary (best effort)
	if s, err := fetchOrdersSummary(c.Request().Context(), h.pool, h.schema, data.Q, data.From, data.To, data.Void, data.CustomerID, data.PayStatusFilter, data.ShipStatusFilter, data.ProcStatusFilter, data.UnproducedOnly, data.CompletedOnly); err == nil {
		data.Summary = s
	}

	if errOrders != nil {
		data.Error = errOrders.Error()
		return c.Render(http.StatusOK, "orders.html", data)
	}
	data.Rows = rows
	data.HasPrev = data.Offset > 0
	data.HasNext = hasNext
	if data.Limit > 0 {
		data.Page = (data.Offset / data.Limit) + 1
	} else {
		data.Page = 1
	}
	return c.Render(http.StatusOK, "orders.html", data)

}

func (h orderHandler) inlineUpdate(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	var req InlineUpdateRequest
	if err := c.Bind(&req); err != nil {
		return c.String(http.StatusBadRequest, "bad request")
	}
	ctx := c.Request().Context()
	if err := inlineUpdateOrder(ctx, h.pool, h.schema, id, actorOf(c), &req); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	return c.NoContent(http.StatusNoContent)

}

func (h orderHandler) audit(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	rows, err := fetchAuditLogs(c.Request().Context(), h.pool, h.schema, id, 50)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, rows)

}

func (h orderHandler) detailRedirect(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/orders/%d/edit", id))

}

func (h orderHandler) editRedirect(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/order?edit_id=%d", id))

}

func (h orderHandler) editPost(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	var req UpdateOrderRequest
	if err := c.Bind(&req); err != nil {
		return c.String(http.StatusBadRequest, "bad request")
	}
	ctx := c.Request().Context()
	if err := updateOrderHeader(ctx, h.pool, h.schema, id, &req); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/orders/%d", id))

}

func (h orderHandler) void(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	reason := strings.TrimSpace(c.FormValue("reason"))
	ctx := c.Request().Context()
	q := fmt.Sprintf("UPDATE %s.orders SET is_void=true, voided_at=now(), void_reason=$2 WHERE id=$1", h.schema)
	if _, err := h.pool.Exec(ctx, q, id, nullText(reason)); err != nil {
		return err
	}
	var rv *string
	if strings.TrimSpace(reason) != "" {
		rv = &reason
	}
	auditInsert(ctx, h.pool, h.schema, actorOf(c), "order", &id, "void", nil, nil, rv, AuditMeta{"order_id": id})
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/orders/%d", id))

}

func (h orderHandler) unvoid(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	ctx := c.Request().Context()
	q := fmt.Sprintf("UPDATE %s.orders SET is_void=false, voided_at=NULL, void_reason=NULL WHERE id=$1", h.schema)
	if _, err := h.pool.Exec(ctx, q, id); err != nil {
		return err
	}
	auditInsert(ctx, h.pool, h.schema, actorOf(c), "order", &id, "unvoid", nil, nil, nil, AuditMeta{"order_id": id})
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/orders/%d", id))

}

func (h orderHandler) entry(c echo.Context) error {
	data := PageData{Today: time.Now().Format("2006-01-02")}
	if c.QueryParam("ok") == "1" {
		data.Ok = true
		data.OrderNo = c.QueryParam("order_no")
	}
	if err := loadOptions(c.Request().Context(), h.pool, h.schema, &data); err != nil {
		data.Error = err.Error()
	}
	if v := strings.TrimSpace(c.QueryParam("edit_id")); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil && id > 0 {
			ed, ferr := fetchOrderEdit(c.Request().Context(), h.pool, h.schema, id)
			if ferr != nil {
				data.Error = ferr.Error()
			} else if ed == nil {
				data.Error = "order not found"
			} else {
				data.EditMode = true
				data.EditID = id
				if ed.OrderDate != "" {
					data.Today = ed.OrderDate
				}
				type editItem struct {
					ProductID   int64  `json:"product_id"`
					ProductName string `json:"product_name"`
					TierID      string `json:"tier_id"`
					UnitPrice   string `json:"unit_price"`
					Qty         string `json:"qty"`
					Unit        string `json:"unit"`
					Spec        string `json:"spec"`
				}
				items := make([]editItem, 0, len(ed.Items))
				for _, it := range ed.Items {
					spec := strings.TrimSuffix(strings.TrimSpace(strings.ToLower(it.Spec)), "g")
					items = append(items, editItem{
						ProductID:   it.ProductID,
						ProductName: it.Product,
						TierID:      "auto",
						UnitPrice:   it.UnitPrice,
						Qty:         it.Qty,
						Unit:        it.Unit,
						Spec:        spec,
					})
				}
				payload := map[string]any{
					"order_date":              ed.OrderDate,
					"customer_id":             strconv.FormatInt(ed.CustomerID, 10),
					"source_id":               strconv.FormatInt(ed.SourceID, 10),
					"order_type_id":           strconv.FormatInt(ed.OrderTypeID, 10),
					"pay_status_id":           strconv.FormatInt(ed.PayStatusID, 10),
					"ship_status_id":          strconv.FormatInt(ed.ShipStatusID, 10),
					"ship_method":             ed.ShipMethod,
					"ship_tracking_no":        ed.ShipTrackingNo,
					"notes":                   ed.Notes,
					"shipping_amount":         ed.ShippingAmount,
					"discount_amount":         ed.DiscountAmount,
					"round_to_int":            ed.RoundToInt,
					"express_fee":             ed.ExpressFee,
					"outsource_material_fee":  ed.OutsourceMaterialFee,
					"outsource_roast_fee":     ed.OutsourceRoastFee,
					"outsource_packaging_fee": ed.OutsourcePackagingFee,
					"outsource_manual_fee":    ed.OutsourceManualFee,
					"outsource_tax_fee":       ed.OutsourceTaxFee,
					"outsource_other_fee":     ed.OutsourceOtherFee,
					"items":                   items,
				}
				if b, err := json.Marshal(payload); err == nil {
					data.EditDataJSON = template.JS(string(b))
				}
			}
		}
	}
	data.ProductsJSON = buildProductsJSON(data.Products)
	return c.Render(http.StatusOK, "order.html", data)

}

func (h orderHandler) create(c echo.Context) error {
	if err := requireEmployeeBound(c); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	var req CreateOrderRequest
	if err := c.Bind(&req); err != nil {
		return c.String(http.StatusBadRequest, "bad request")
	}

	orderDate := strings.TrimSpace(req.OrderDate)
	if orderDate == "" {
		orderDate = time.Now().Format("2006-01-02")
	}
	od, err := time.Parse("2006-01-02", orderDate)
	if err != nil {
		return c.String(http.StatusBadRequest, "invalid order_date")
	}
	if req.CustomerID <= 0 {
		return c.String(http.StatusBadRequest, "customer required")
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
	for i := 0; i < maxLen(req.ItemName, req.ProductID, req.TierID, req.UnitPrice, req.Qty, req.Unit, req.Spec); i++ {
		pidStr := strings.TrimSpace(getStr(req.ProductID, i))
		name := strings.TrimSpace(getStr(req.ItemName, i))

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
		if tidStr := strings.TrimSpace(getStr(req.TierID, i)); tidStr != "" && tidStr != "auto" {
			if tidStr == "manual" {
				if v := strings.TrimSpace(getStr(req.UnitPrice, i)); v != "" {
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
		if q := strings.TrimSpace(getStr(req.Qty, i)); q != "" {
			if n, err := strconv.ParseInt(q, 10, 64); err == nil && n > 0 {
				it.units = n
			}
		}
		if sg := strings.TrimSpace(getStr(req.Spec, i)); sg != "" {
			// spec is grams (e.g. "227" or "227g")
			sg = strings.TrimSuffix(strings.ToLower(sg), "g")
			if n, err := strconv.ParseInt(sg, 10, 64); err == nil && n > 0 {
				it.specG = n
				ss := fmt.Sprintf("%dg", n)
				it.spec = &ss
			}
		}
		if u := strings.TrimSpace(getStr(req.Unit, i)); u != "" {
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
		return c.String(http.StatusBadRequest, "at least one item required")
	}

	ctx := c.Request().Context()
	conn, err := h.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, fmt.Sprintf("LOCK TABLE %s.orders IN SHARE ROW EXCLUSIVE MODE", h.schema)); err != nil {
		return err
	}

	orderNo := ""

	// Pricing: use tier match by qty(lb). Allow manual override.
	totalAmt := 0.0
	orderWeightG := int64(0)
	for idx := range items {
		itemWeightG := items[idx].specG * items[idx].units
		orderWeightG += itemWeightG
		totalG := float64(itemWeightG)
		qtyLb := totalG / 454.0

		if items[idx].manualPrice != nil {
			// manual price is 元/磅
			items[idx].unitPrice = *items[idx].manualPrice
			items[idx].priceOverride = true
		} else if items[idx].productID != nil {
			// If user selected a tier explicitly
			if items[idx].tierID != nil {
				var price float64
				q := fmt.Sprintf(`SELECT price_per_lb FROM %s.product_price_tiers WHERE id=$1 AND active=true`, h.schema)
				if err := tx.QueryRow(ctx, q, *items[idx].tierID).Scan(&price); err != nil {
					return c.String(http.StatusBadRequest, "invalid tier")
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
					`, h.schema)
				err := tx.QueryRow(ctx, q, *items[idx].productID, qtyLb).Scan(&tid, &price)
				if err != nil {
					// fallback: highest tier with min<=qty
					q2 := fmt.Sprintf(`
							SELECT id, price_per_lb
							FROM %s.product_price_tiers
							WHERE product_id=$1 AND active=true AND min_qty_lb <= $2
							ORDER BY min_qty_lb DESC
							LIMIT 1
						`, h.schema)
					if err2 := tx.QueryRow(ctx, q2, *items[idx].productID, qtyLb).Scan(&tid, &price); err2 != nil {
						// below minimum tier: use minimum tier price
						q3 := fmt.Sprintf(`
								SELECT id, price_per_lb
								FROM %s.product_price_tiers
								WHERE product_id=$1 AND active=true
								ORDER BY min_qty_lb ASC
								LIMIT 1
							`, h.schema)
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
	if v := strings.TrimSpace(req.ShippingAmount); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return c.String(http.StatusBadRequest, "invalid shipping_amount")
		}
		shippingAmt = f
	}
	discountAmt := 0.0
	if v := strings.TrimSpace(req.DiscountAmount); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return c.String(http.StatusBadRequest, "invalid discount_amount")
		}
		discountAmt = f
	}
	roundToInt := strings.TrimSpace(req.RoundToInt) != ""
	outsourceTotal, outsourceFees, err := calcOutsourceTotal(&req)
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	grand0 := totalAmt + shippingAmt - discountAmt + outsourceTotal
	grandTotal, roundingAmt := applyRoundToInt(grand0, roundToInt)

	// 默认发货状态：未选择时自动写入“未发货”。
	shipStatusID := req.ShipStatusID
	if shipStatusID == 0 {
		_ = tx.QueryRow(ctx, fmt.Sprintf("SELECT id FROM %s.ship_statuses WHERE name='未发货' ORDER BY id LIMIT 1", h.schema)).Scan(&shipStatusID)
	}

	shipMethod := strings.TrimSpace(req.ShipMethod)
	if shipMethod == "" {
		if orderWeightG <= 15000 {
			shipMethod = "sf_small"
		} else {
			shipMethod = "sf_large"
		}
	}

	editID := int64(0)
	if v := strings.TrimSpace(c.FormValue("edit_id")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			editID = n
		}
	}

	insertItemSQL := fmt.Sprintf(`INSERT INTO %s.order_items(order_id,line_no,product_id,price_tier_id,price_overridden,item_name,qty,unit,spec,unit_price,line_total)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, h.schema)

	var orderID int64
	if editID > 0 {
		if err := tx.QueryRow(ctx, fmt.Sprintf("SELECT id, order_no FROM %s.orders WHERE id=$1 FOR UPDATE", h.schema), editID).Scan(&orderID, &orderNo); err != nil {
			return c.String(http.StatusBadRequest, "invalid edit_id")
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
			`, h.schema)
		if _, err := tx.Exec(ctx, uq,
			orderID,
			od,
			req.CustomerID,
			nullInt(req.SourceID),
			nullInt(req.OrderTypeID),
			nullInt(req.PayStatusID),
			nullInt(shipStatusID),
			nullText(shipMethod),
			nullText(req.ShipTrackingNo),
			nullText(req.Notes),
			totalAmt,
			shippingAmt,
			discountAmt,
			roundToInt,
			roundingAmt,
			grandTotal,
			nullText(req.ExpressFee),
			outsourceFees[0],
			outsourceFees[1],
			outsourceFees[2],
			outsourceFees[3],
			outsourceFees[4],
			outsourceFees[5],
			outsourceTotal,
		); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s.order_items WHERE order_id=$1", h.schema), orderID); err != nil {
			return err
		}
	} else {
		orderNo, err = nextOrderNo(ctx, tx, h.schema, od)
		if err != nil {
			return err
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
			`, h.schema)
		err = tx.QueryRow(ctx, insertOrderSQL,
			od,
			req.CustomerID,
			nullInt(req.SourceID),
			nullInt(req.OrderTypeID),
			nullInt(req.PayStatusID),
			nullInt(shipStatusID),
			nullText(shipMethod),
			nullText(req.ShipTrackingNo),
			nullText(req.Notes),
			totalAmt,
			shippingAmt,
			discountAmt,
			roundToInt,
			roundingAmt,
			grandTotal,
			nullText(req.ExpressFee),
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
			return err
		}
	}

	for idx, it := range items {
		qtyAny := any(nil)
		if it.units > 0 {
			qtyAny = it.units
		}
		if _, err := tx.Exec(ctx, insertItemSQL, orderID, idx+1, it.productID, it.tierID, it.priceOverride, it.name, qtyAny, it.unit, it.spec, it.unitPrice, it.lineTotal); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	if editID > 0 {
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/orders/%d", orderID))
	}
	return c.Redirect(http.StatusSeeOther, "/order?ok=1&order_no="+url.QueryEscape(orderNo))

}
