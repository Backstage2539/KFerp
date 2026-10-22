package customerfulfillment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"math"
	app "orderapp/internal/application/customerfulfillment"
	salesapp "orderapp/internal/application/sales"
	freightdomain "orderapp/internal/domain/freight"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	customerstock "orderapp/internal/infrastructure/postgres/customerstock"
	freightpg "orderapp/internal/infrastructure/postgres/freight"
	"orderapp/internal/infrastructure/postgres/orderbeans"
	"orderapp/internal/infrastructure/postgres/orderweight"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
	"sort"
	"strings"
	"time"
)

// All fulfillment writers serialize by the owner before acquiring order,
// entitlement and physical-inventory locks. The order/customer are read again
// after this advisory lock; the first read conveys no authority.
func (r *Repository) beginFulfillmentEdit(ctx context.Context, id, expectedCustomer int64) (pgx.Tx, app.FulfillmentOrderEditContext, error) {
	var v app.FulfillmentOrderEditContext
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return nil, v, e
	}
	fail := func(err error) (pgx.Tx, app.FulfillmentOrderEditContext, error) { tx.Rollback(ctx); return nil, v, err }
	var customer int64
	if e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT customer_id FROM %s.orders WHERE id=$1`, r.schema), id).Scan(&customer); e != nil {
		return fail(fmt.Errorf("订单不存在"))
	}
	if expectedCustomer > 0 && customer != expectedCustomer {
		return fail(fmt.Errorf("订单不属于当前客户"))
	}
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, fmt.Sprintf("mini-direct-ship:%d", customer)); e != nil {
		return fail(e)
	}
	if e = freightpg.LockOrderFreight(ctx, tx); e != nil {
		return fail(e)
	}
	v.CanEdit = true
	block := func(reason string) { v.CanEdit = false; v.EditBlockReason = reason }
	var service, confirmation string
	var required, void bool
	var pending []byte
	e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT o.id,o.order_no,o.customer_id,COALESCE(o.fulfillment_mode,''),COALESCE(o.portal_service_code,''),to_char(o.order_date,'YYYY-MM-DD'),COALESCE(o.receiver_name,''),COALESCE(o.receiver_phone,''),COALESCE(o.receiver_address,''),COALESCE(o.receiver_company,''),COALESCE(o.notes,''),COALESCE(o.confirmation_required,false),COALESCE(o.confirmation_status,'accepted'),o.confirmation_pending,COALESCE(o.is_void,false) FROM %s.orders o WHERE o.id=$1 FOR UPDATE`, r.schema), id).Scan(&v.OrderID, &v.OrderNo, &v.CustomerID, &v.FulfillmentMode, &service, &v.OrderDate, &v.ReceiverName, &v.ReceiverPhone, &v.ReceiverAddress, &v.ReceiverCompany, &v.Notes, &required, &confirmation, &pending, &void)
	if e != nil {
		return fail(e)
	}
	if customer != v.CustomerID || expectedCustomer > 0 && v.CustomerID != expectedCustomer {
		return fail(fmt.Errorf("订单客户已变化，请重新打开"))
	}
	if service != "direct_ship" || (v.FulfillmentMode != "stock" && v.FulfillmentMode != "on_demand") {
		return fail(fmt.Errorf("此入口仅支持库存代发和无库存代发订单"))
	}
	if required || confirmation != "accepted" || len(pending) > 0 && string(pending) != "null" {
		block("订单存在客户确认流程，请先完成确认后再编辑")
	}
	if void {
		block("订单已作废，不能再编辑")
	}
	var requestStatus, requestMode string
	e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT d.id,d.status,COALESCE(d.fulfillment_mode,''),COALESCE(d.province,''),COALESCE(d.city,''),COALESCE(d.district,'') FROM %[1]s.customer_direct_ship_requests d JOIN %[1]s.customer_direct_ship_request_orders ro ON ro.request_id=d.id WHERE ro.order_id=$1 AND d.customer_id=$2 FOR UPDATE OF d`, r.schema), id, customer).Scan(&v.RequestID, &requestStatus, &requestMode, &v.Province, &v.City, &v.District)
	if e != nil {
		return fail(fmt.Errorf("代发申请关联不可用，请核对原订单"))
	}
	var linked int
	if e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.customer_direct_ship_request_orders WHERE request_id=$1`, r.schema), v.RequestID).Scan(&linked); e != nil {
		return fail(e)
	}
	if linked != 1 || requestMode != v.FulfillmentMode || !miniDirectShipCancellationAllowed(requestStatus) {
		block("代发申请已进入发货流程或关联不一致，不能编辑")
	}
	state, e := postgressales.LoadOrderEditStateTx(ctx, tx, r.schema, id, false)
	if e != nil {
		return fail(e)
	}
	// A reservation is precisely what this editor adjusts. It is not a stock
	// deduction or shipment. Physical movements and production execution remain guarded.
	state.HasStockAllocation = false
	if v.FulfillmentMode == "stock" && state.ProcessStatus == "库存待发货" {
		state.ProcessStatus = "待生产"
	}
	edit := salesapp.EvaluateOrderEditability(state)
	if !edit.CanEdit {
		block(edit.BlockReason)
	}
	v.EditRevision, e = postgressales.CurrentOrderEditRevisionTx(ctx, tx, r.schema, id)
	if e != nil {
		return fail(e)
	}
	rows, e := tx.Query(ctx, fmt.Sprintf(`SELECT i.id,COALESCE(ri.id,0),COALESCE(i.product_id,0),COALESCE(i.bom_spec_id,0),COALESCE(i.bom_variant_id,0),COALESCE(ri.bom_spec_key,''),COALESCE(i.item_name,''),COALESCE(ri.sku_code,''),COALESCE(i.spec,''),COALESCE(ri.inventory_unit,i.unit,''),COALESCE(NULLIF(i.sales_unit,''),i.unit,''),COALESCE(ri.spec_g,0),i.qty::bigint,COALESCE(i.unit_price,0)::float8,COALESCE(i.line_total,0)::float8,COALESCE(i.line_total_before_discount,0)::float8,COALESCE(i.discount_amount,0)::float8,COALESCE(i.discount_type,''),COALESCE(i.discount_value,0)::float8,COALESCE(i.bean_list_publication_id,0),COALESCE(i.price_source_json,'{}'::jsonb)::text FROM %[1]s.order_items i LEFT JOIN %[1]s.customer_direct_ship_request_items ri ON ri.request_id=$2 AND ri.line_no=i.line_no WHERE i.order_id=$1 AND i.qty>0 ORDER BY i.line_no,i.id FOR UPDATE OF i`, r.schema), id, v.RequestID)
	if e != nil {
		return fail(e)
	}
	for rows.Next() {
		var i app.FulfillmentOrderEditItem
		if e = rows.Scan(&i.ItemID, &i.RequestItemID, &i.ProductID, &i.BomSpecID, &i.BomVariantID, &i.BomSpecKey, &i.ProductName, &i.SKUCode, &i.SpecName, &i.InventoryUnit, &i.SalesUnit, &i.SpecG, &i.Qty, &i.UnitPrice, &i.LineAmount, &i.BaseLineAmount, &i.DiscountAmount, &i.DiscountType, &i.DiscountValue, &i.PublicationID, &i.PriceSourceJSON); e != nil {
			rows.Close()
			return fail(e)
		}
		if i.BaseLineAmount == 0 && i.LineAmount+i.DiscountAmount > 0 {
			i.BaseLineAmount = i.LineAmount + i.DiscountAmount
		}
		i.SpecLabel = i.SpecName
		if i.RequestItemID == 0 {
			rows.Close()
			return fail(fmt.Errorf("代发明细关联缺失，请核对原订单"))
		}
		v.Items = append(v.Items, i)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return fail(e)
	}
	v.Fees, e = r.loadFulfillmentOrderEditFeesTx(ctx, tx, customer, id)
	if e != nil {
		return fail(e)
	}
	if v.Fees.FulfillmentMode != v.FulfillmentMode {
		return fail(fmt.Errorf("代发费用场景不一致"))
	}
	views, e := freightpg.NewRepository(r.pool, r.schema).OrderViews(ctx, tx, []int64{id})
	if e != nil {
		return fail(e)
	}
	v.Freight = views[id]
	amounts, e := r.fulfillmentEditAmountsTx(ctx, tx, v, v.Items)
	if e != nil {
		return fail(e)
	}
	v.GoodsTotalCents = amounts.GoodsTotalCents
	return tx, v, nil
}

func (r *Repository) FulfillmentOrderEdit(ctx context.Context, id int64) (app.FulfillmentOrderEditContext, error) {
	tx, v, e := r.beginFulfillmentEdit(ctx, id, 0)
	if e != nil {
		return v, e
	}
	defer tx.Rollback(ctx)
	if v.FulfillmentMode == "stock" {
		v.Catalog, e = r.fulfillmentEditStockCatalog(ctx, tx, v)
	}
	// On-demand catalogs are enriched by the application service, using the same
	// selected-publication authority as order creation.
	return v, e
}
func (r *Repository) PreviewFulfillmentOrderEdit(ctx context.Context, c app.FulfillmentOrderEditCommand) (app.FulfillmentOrderEditPreview, error) {
	tx, v, e := r.beginFulfillmentEdit(ctx, c.OrderID, c.CustomerID)
	if e != nil {
		return app.FulfillmentOrderEditPreview{}, e
	}
	defer tx.Rollback(ctx)
	p, _, _, _, e := r.previewFulfillmentEditTx(ctx, tx, v, c)
	return p, e
}
func (r *Repository) UpdateFulfillmentOrder(ctx context.Context, c app.FulfillmentOrderEditCommand) (app.FulfillmentOrderEditResult, error) {
	tx, v, e := r.beginFulfillmentEdit(ctx, c.OrderID, c.CustomerID)
	if e != nil {
		return app.FulfillmentOrderEditResult{}, e
	}
	defer tx.Rollback(ctx)
	p, stock, pending, c, e := r.previewFulfillmentEditTx(ctx, tx, v, c)
	if e != nil {
		return app.FulfillmentOrderEditResult{}, e
	}
	if !c.RecipientOnly && (strings.TrimSpace(c.PreviewToken) == "" || c.PreviewToken != p.PreviewToken) {
		return app.FulfillmentOrderEditResult{}, salesapp.NewOrderEditConflictError("订单、价格、库存或费用已变化，请重新预览后保存")
	}
	if e = orderweight.MarkHistoricalUnknown(ctx, tx, r.schema, v.OrderID); e != nil {
		return app.FulfillmentOrderEditResult{}, e
	}
	if e = r.saveFulfillmentEditItems(ctx, tx, v, &p); e != nil {
		return app.FulfillmentOrderEditResult{}, e
	}
	if v.FulfillmentMode == "stock" {
		if e = r.syncFulfillmentEditReservations(ctx, tx, v, p.Items, stock, pending, c.Actor); e != nil {
			return app.FulfillmentOrderEditResult{}, e
		}
	}
	company := v.ReceiverCompany
	if c.ReceiverCompany != nil {
		company = strings.TrimSpace(*c.ReceiverCompany)
	}
	amounts, e := r.fulfillmentEditAmountsTx(ctx, tx, v, p.Items)
	if e != nil {
		return app.FulfillmentOrderEditResult{}, e
	}
	goods, grand, shipping, discount, payment := amounts.Gross, amounts.Grand, amounts.Shipping, amounts.Discount, amounts.Payment
	p.GoodsTotalCents = amounts.GoodsTotalCents

	if e = postgressales.ProtectOrderReceiptsTx(ctx, tx, r.schema, v.OrderID, &payment, shipping, grand); e != nil {
		return app.FulfillmentOrderEditResult{}, salesapp.NewOrderEditConflictError(e.Error())
	}
	if _, e = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET pay_status_id=NULLIF($2,0),prepayment_amount=COALESCE($3::numeric,prepayment_amount) WHERE id=$1`, r.schema), v.OrderID, payment.PayStatusID, payment.PrepaymentAmount); e != nil {
		return app.FulfillmentOrderEditResult{}, e
	}

	// Keep all historical collection amounts/statuses and separate service fees.
	if _, e = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET order_date=$2::date,receiver_name=$3,receiver_phone=$4,receiver_address=$5,receiver_company=$6,notes=$7,total_amount=$8,grand_total=$9,discount_amount=$10 WHERE id=$1`, r.schema), v.OrderID, c.OrderDate, c.ReceiverName, c.ReceiverPhone, c.ReceiverAddress, company, c.Notes, goods, grand, discount); e != nil {
		return app.FulfillmentOrderEditResult{}, e
	}
	if _, e = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_direct_ship_requests SET recipient_name=$2,recipient_phone=$3,detail_address=$4,recipient_company=$5,province=$6,city=$7,district=$8,note=$9 WHERE id=$1`, r.schema), v.RequestID, c.ReceiverName, c.ReceiverPhone, c.ReceiverAddress, company, c.Province, c.City, c.District, c.Notes); e != nil {
		return app.FulfillmentOrderEditResult{}, e
	}
	if e = r.saveFulfillmentOrderEditFeesTx(ctx, tx, v.CustomerID, v.OrderID, v.Fees.FeeQuoteToken, v.Fees.Freight.Token, p.Fees, c.Actor); e != nil {
		return app.FulfillmentOrderEditResult{}, e
	}
	if v.FulfillmentMode == "stock" {
		name := "库存待发货"
		if len(pending) > 0 {
			name = "待生产"
		}
		pid := customerFulfillmentStatusID(ctx, tx, r.schema, "order_process_statuses", name)
		if _, e = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET process_status_id=$2 WHERE id=$1`, r.schema), v.OrderID, nullableCustomerFulfillmentID(pid)); e != nil {
			return app.FulfillmentOrderEditResult{}, e
		}
	}
	if e = postgressales.InvalidateConfirmedOrderDocumentsTx(ctx, tx, r.schema, v.OrderID); e != nil {
		return app.FulfillmentOrderEditResult{}, e
	}
	before, _ := json.Marshal(map[string]any{"order_date": v.OrderDate, "receiver_name": v.ReceiverName, "receiver_phone": v.ReceiverPhone, "receiver_address": v.ReceiverAddress, "receiver_company": v.ReceiverCompany, "province": v.Province, "city": v.City, "district": v.District, "notes": v.Notes, "items": v.Items, "fees": v.Fees})
	after, _ := json.Marshal(map[string]any{"order_date": c.OrderDate, "receiver_name": c.ReceiverName, "receiver_phone": c.ReceiverPhone, "receiver_address": c.ReceiverAddress, "receiver_company": company, "province": c.Province, "city": c.City, "district": c.District, "notes": c.Notes, "items": p.Items, "fees": p.Fees})
	if e = postgresinfra.AuditInsertTx(ctx, tx, r.schema, c.Actor, "order", &v.OrderID, "fulfillment_edit", postgresinfra.StrPtr("fulfillment"), postgresinfra.StrPtr(string(before)), postgresinfra.StrPtr(string(after)), postgresinfra.AuditMeta{"customer_id": v.CustomerID, "request_id": v.RequestID, "mode": v.FulfillmentMode, "recipient_changed": v.ReceiverAddress != c.ReceiverAddress || v.ReceiverName != c.ReceiverName || v.ReceiverPhone != c.ReceiverPhone || v.ReceiverCompany != company, "old_fee_token": v.Fees.FeeQuoteToken, "fee_token": p.Fees.FeeQuoteToken, "reason": c.AdjustmentReason}); e != nil {
		return app.FulfillmentOrderEditResult{}, e
	}
	p.EditRevision, e = postgressales.CurrentOrderEditRevisionTx(ctx, tx, r.schema, v.OrderID)
	if e != nil {
		return app.FulfillmentOrderEditResult{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return app.FulfillmentOrderEditResult{}, e
	}
	return app.FulfillmentOrderEditResult{OK: true, FulfillmentOrderEditPreview: p}, nil
}

func (r *Repository) previewFulfillmentEditTx(ctx context.Context, tx pgx.Tx, v app.FulfillmentOrderEditContext, c app.FulfillmentOrderEditCommand) (app.FulfillmentOrderEditPreview, []miniPlannedAllocation, []miniPlannedProcessingAllocation, app.FulfillmentOrderEditCommand, error) {
	p := app.FulfillmentOrderEditPreview{OrderID: v.OrderID, OrderNo: v.OrderNo, RequestID: v.RequestID, EditRevision: v.EditRevision, StockReady: true}
	fail := func(e error) (app.FulfillmentOrderEditPreview, []miniPlannedAllocation, []miniPlannedProcessingAllocation, app.FulfillmentOrderEditCommand, error) {
		return p, nil, nil, c, e
	}
	if !v.CanEdit {
		return fail(salesapp.NewOrderEditConflictError(v.EditBlockReason))
	}
	if c.ExpectedEditRevision == "" || c.ExpectedEditRevision != v.EditRevision {
		return fail(salesapp.NewOrderEditConflictError("订单已被其他操作修改，请重新打开订单"))
	}
	if c.RecipientOnly {
		c.OrderDate, c.Notes = v.OrderDate, v.Notes
		c.Province, c.City, c.District = v.Province, v.City, v.District
		if c.ReceiverAddress != v.ReceiverAddress {
			resolved := freightdomain.ResolveAddress(c.ReceiverAddress)
			if resolved.Status != "resolved" {
				return fail(fmt.Errorf("新地址省市区不明确，请从完整编辑页面选择省市区"))
			}
			c.Province, c.City, c.District = resolved.Destination.Province, resolved.Destination.City, resolved.Destination.District
		}
		c.Items = nil
		for _, i := range v.Items {
			c.Items = append(c.Items, app.FulfillmentOrderEditItemCommand{ItemID: i.ItemID, ProductID: i.ProductID, BomSpecID: i.BomSpecID, BomVariantID: i.BomVariantID, SpecG: i.SpecG, Qty: i.Qty, SalesUnit: i.SalesUnit, PublicationID: i.PublicationID})
		}
	}
	c.ReceiverName, c.ReceiverPhone, c.ReceiverAddress = strings.TrimSpace(c.ReceiverName), strings.TrimSpace(c.ReceiverPhone), strings.TrimSpace(c.ReceiverAddress)
	if c.ReceiverAddress != v.ReceiverAddress || c.Province != v.Province || c.City != v.City || c.District != v.District {
		resolved := freightdomain.ResolveAddress(c.ReceiverAddress)
		if resolved.Status == "resolved" {
			manual := freightdomain.ResolveAddress(c.Province + c.City + c.District)
			d := freightdomain.Destination{Province: c.Province, City: c.City, District: c.District}
			if manual.Status == "resolved" {
				d = manual.Destination
			}
			actual := resolved.Destination
			if actual.Province != d.Province || actual.City != "" && d.City != "" && actual.City != d.City || actual.District != "" && d.District != "" && actual.District != d.District {
				return fail(fmt.Errorf("新地址与所选省市区不一致，请重新识别或选择收件地区"))
			}
		}
	}
	if c.ReceiverName == "" || c.ReceiverPhone == "" || c.ReceiverAddress == "" {
		return fail(fmt.Errorf("请完整填写收件人、电话和地址"))
	}
	if _, e := time.Parse("2006-01-02", c.OrderDate); e != nil {
		return fail(fmt.Errorf("订单日期无效"))
	}
	if len(c.Items) == 0 || len(c.Items) > 200 {
		return fail(fmt.Errorf("请至少选择一个商品"))
	}
	old := map[int64]app.FulfillmentOrderEditItem{}
	for _, i := range v.Items {
		old[i.ItemID] = i
	}
	seenIDs, seenSpec := map[int64]bool{}, map[string]bool{}
	for _, in := range c.Items {
		if in.ProductID <= 0 || in.BomSpecID <= 0 || in.Qty <= 0 || in.Qty > 1000000 {
			return fail(fmt.Errorf("商品规格或数量无效"))
		}
		key := miniStockKey(in.ProductID, in.BomSpecID, 0)
		if seenSpec[key] {
			return fail(fmt.Errorf("相同商品规格请合并到同一行"))
		}
		seenSpec[key] = true
		prev, exists := old[in.ItemID]
		if in.ItemID > 0 && (!exists || seenIDs[in.ItemID]) {
			return fail(fmt.Errorf("商品明细不属于当前订单或重复"))
		}
		seenIDs[in.ItemID] = true
		same := exists && prev.ProductID == in.ProductID && prev.BomSpecID == in.BomSpecID && (in.BomVariantID == 0 || prev.BomVariantID == in.BomVariantID)
		var line app.FulfillmentOrderEditItem
		if same {
			line = prev
			line.Qty = in.Qty
			if in.Qty != prev.Qty {
				if prev.DiscountAmount != 0 || prev.DiscountValue != 0 {
					return fail(fmt.Errorf("该商品包含历史行折扣，修改数量前请先通过财务流程核对折扣"))
				}
				line.LineAmount = float64(in.Qty) * prev.UnitPrice
				line.BaseLineAmount = line.LineAmount
			}
		} else {
			identity, e := resolveCustomerFulfillmentBOMSpecIdentityTx(ctx, tx, r.schema, in.ProductID, in.BomSpecID, in.BomVariantID)
			if e != nil {
				return fail(e)
			}
			line = app.FulfillmentOrderEditItem{ItemID: 0, RequestItemID: 0, ProductID: identity.ProductID, BomSpecID: identity.BomSpecID, BomVariantID: identity.BomVariantID, BomSpecKey: identity.BomSpecKey, ProductName: strings.TrimSpace(identity.ProductName + " " + identity.BomSpecName), SKUCode: identity.SpecCode, SpecName: identity.BomSpecName, SpecLabel: identity.BomSpecName, InventoryUnit: identity.InventoryUnit, SalesUnit: identity.InventoryUnit, Qty: in.Qty, PriceSourceJSON: `{"source":"prepaid_customer_stock"}`}
			if v.FulfillmentMode == "on_demand" {
				if e = r.priceFulfillmentEditLine(ctx, tx, v.CustomerID, in, &line); e != nil {
					return fail(e)
				}
			}
		}
		if in.UnitPrice != nil && math.Abs(*in.UnitPrice-line.UnitPrice) > 0.000001 {
			return fail(fmt.Errorf("代发商品价格由原订单或授权价格表确定，请重新选择商品"))
		}
		if v.FulfillmentMode == "stock" {
			line.UnitPrice, line.LineAmount, line.BaseLineAmount, line.DiscountAmount, line.DiscountValue, line.PublicationID = 0, 0, 0, 0, 0, 0
			line.DiscountType = ""
		}
		p.Items = append(p.Items, line)
		if line.LineAmount < 0 || line.LineAmount > 100000000 {
			return fail(fmt.Errorf("商品金额无效"))
		}
	}
	amounts, amountErr := r.fulfillmentEditAmountsTx(ctx, tx, v, p.Items)
	if amountErr != nil {
		return fail(amountErr)
	}
	p.GoodsTotalCents = amounts.GoodsTotalCents
	var physical []miniPlannedAllocation
	var pending []miniPlannedProcessingAllocation
	if v.FulfillmentMode == "stock" {
		stock, outputs, e := r.fulfillmentEditCandidates(ctx, tx, v, true)
		if e != nil {
			return fail(e)
		}
		items := []app.MiniDirectShipItemCommand{}
		for _, i := range p.Items {
			items = append(items, app.MiniDirectShipItemCommand{ProductID: i.ProductID, BomSpecID: i.BomSpecID, BomVariantID: i.BomVariantID, SpecG: i.SpecG, Qty: i.Qty, IsProcessingProduct: true})
		}
		physical, pending, p.Shortages, e = r.planFulfillmentEdit(ctx, tx, v, items, stock, outputs)
		if e != nil {
			return fail(e)
		}
		if len(p.Shortages) > 0 {
			return fail(&miniDirectShipStockError{Shortages: p.Shortages})
		}
		p.StockReady = len(pending) == 0
	}
	fc := app.FulfillmentFeeQuoteCommand{OrderID: v.OrderID, CustomerID: v.CustomerID, FulfillmentMode: v.FulfillmentMode, ExpectedFreightToken: v.Fees.Freight.Token, Province: c.Province, City: c.City, District: c.District, ServiceOverrideCents: c.ServiceFeeCents, AdjustmentReason: c.AdjustmentReason, EmployeeAdjustment: true}
	for _, i := range p.Items {
		fc.Items = append(fc.Items, app.FulfillmentFeeItem{ProductID: i.ProductID, BomSpecID: i.BomSpecID, BomVariantID: i.BomVariantID, SpecG: i.SpecG, Qty: i.Qty})
	}
	var e error
	p.Fees, e = r.previewFulfillmentOrderEditFeesTx(ctx, tx, fc)
	if e != nil {
		return fail(e)
	}
	// The token binds all editable headers, canonical prices, chosen source lots,
	// and both frozen fee versions. Save derives it again while locks are held.
	digest, marshalErr := json.Marshal(struct {
		Command     app.FulfillmentOrderEditCommand
		Items       []app.FulfillmentOrderEditItem
		Physical    []miniPlannedAllocation
		Pending     []miniPlannedProcessingAllocation
		OldFee, Fee string
	}{func() app.FulfillmentOrderEditCommand {
		d := c
		d.PreviewToken = ""
		d.Actor = ""
		d.CustomerID = 0
		return d
	}(), p.Items, physical, pending, v.Fees.FeeQuoteToken, p.Fees.FeeQuoteToken})
	if marshalErr != nil {
		return fail(fmt.Errorf("代发预览内容无效"))
	}
	sum := sha256.Sum256(digest)
	p.PreviewToken = hex.EncodeToString(sum[:])
	return p, physical, pending, c, nil
}

func (r *Repository) priceFulfillmentEditLine(ctx context.Context, tx pgx.Tx, customer int64, in app.FulfillmentOrderEditItemCommand, line *app.FulfillmentOrderEditItem) error {
	rows, e := tx.Query(ctx, fmt.Sprintf(`SELECT b.id,b.list_type,b.content_json FROM %[1]s.customer_order_price_table_bindings x JOIN %[1]s.bean_list_publications b ON b.id=x.publication_id WHERE x.customer_id=$1 AND x.usage_code='direct_ship' AND b.status='published' AND b.deleted_at IS NULL AND b.publication_purpose='factory_supply' AND (b.owner_type='official' OR b.owner_type='customer' AND b.owner_key=$1::text) ORDER BY b.id FOR SHARE OF b,x`, r.schema), customer)
	if e != nil {
		return e
	}
	type table struct {
		id   int64
		kind string
		raw  []byte
	}
	tables := []table{}
	for rows.Next() {
		var t table
		if e = rows.Scan(&t.id, &t.kind, &t.raw); e != nil {
			rows.Close()
			return e
		}
		tables = append(tables, t)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	count := 0
	seen := map[int64]bool{}
	for _, t := range tables {
		if in.PublicationID > 0 && in.PublicationID != t.id || seen[t.id] {
			continue
		}
		seen[t.id] = true
		p, ok := orderbeans.PublishedFixedBOMSpecPricing(t.raw, line.ProductID, line.BomSpecID, t.kind)
		if !ok {
			continue
		}

		if p.MinQty > 1 || p.MaxQty != nil {
			return fmt.Errorf("无库存代发商品须使用唯一固定价")
		}
		count++
		line.PublicationID = t.id
		line.UnitPrice = p.UnitPrice
		line.LineAmount = float64(line.Qty) * p.UnitPrice
		line.BaseLineAmount = line.LineAmount
		line.PriceSourceJSON = mustPayloadJSON(map[string]any{"source": "published_price_table", "publication_id": t.id, "product_id": line.ProductID, "bom_spec_id": line.BomSpecID, "bom_variant_id": line.BomVariantID, "source_price_record_id": p.SourcePriceRecordID, "quantity_basis": "sales_spec_count"})
	}
	if count != 1 {
		return fmt.Errorf("所选商品规格须包含在当前客户授权的唯一固定价格表中")
	}
	return nil
}

func (r *Repository) fulfillmentEditCandidates(ctx context.Context, tx pgx.Tx, v app.FulfillmentOrderEditContext, lock bool) ([]miniStockCandidate, []miniProcessingOutputCandidate, error) {
	stock, e := r.loadMiniCustomerFinishedStockForOrder(ctx, tx, v.CustomerID, lock, v.OrderID)
	if e != nil {
		return nil, nil, e
	}
	supply, e := customerstock.ListSupply(ctx, tx, r.schema, v.CustomerID, "", lock)
	if e != nil {
		return nil, nil, e
	}
	own := map[int64]int64{}
	rows, e := tx.Query(ctx, fmt.Sprintf(`SELECT entitlement_id,SUM(reserved_qty-converted_qty-released_qty)::bigint FROM %s.customer_stock_supply_reservations WHERE order_id=$1 GROUP BY entitlement_id`, r.schema), v.OrderID)
	if e != nil {
		return nil, nil, e
	}
	for rows.Next() {
		var id, n int64
		if e = rows.Scan(&id, &n); e != nil {
			rows.Close()
			return nil, nil, e
		}
		own[id] = n
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, nil, e
	}
	outputs := []miniProcessingOutputCandidate{}
	for _, s := range supply {
		var allowed bool
		if e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.warehouses WHERE code=$1 AND active=true AND customer_id=$2 AND kind IN('finished','customer_finished','customer_processing','customer','shared_finished'))`, r.schema), s.Warehouse, v.CustomerID).Scan(&allowed); e != nil {
			return nil, nil, e
		}
		if !allowed {
			continue
		}
		n := s.AvailableQty + own[s.ID]
		if n > s.PendingQty {
			n = s.PendingQty
		}
		if n > 0 {
			outputs = append(outputs, miniProcessingOutputCandidate{EntitlementID: s.ID, ProductID: s.ProductID, BomSpecID: s.BomSpecID, BomVariantID: s.BomVariantID, SpecG: s.SpecG, Warehouse: s.Warehouse, AvailableQty: n})
		}
	}
	return stock.Candidates, outputs, nil
}

func (r *Repository) fulfillmentEditStockCatalog(ctx context.Context, tx pgx.Tx, v app.FulfillmentOrderEditContext) (app.MiniDirectShipCatalog, error) {
	out := app.MiniDirectShipCatalog{CurrentCustomerID: v.CustomerID, FulfillmentMode: v.FulfillmentMode, ProductFamilies: []map[string]any{}, PriceTables: []app.MiniDirectShipPriceTable{}}
	stock, pending, e := r.fulfillmentEditCandidates(ctx, tx, v, false)
	if e != nil {
		return out, e
	}
	type amount struct{ product, spec, variant, g, physical, pending int64 }
	by := map[string]*amount{}
	add := func(product, spec, variant, g, physical, pending int64) {
		key := miniStockKey(product, spec, g)
		if by[key] == nil {
			by[key] = &amount{product: product, spec: spec, variant: variant, g: g}
		}
		by[key].physical += physical
		by[key].pending += pending
	}
	for _, s := range stock {
		add(s.ProductID, s.BomSpecID, s.BomVariantID, s.SpecG, s.AvailableQty, 0)
	}
	for _, s := range pending {
		add(s.ProductID, s.BomSpecID, s.BomVariantID, s.SpecG, 0, s.AvailableQty)
	}
	own := map[string]int64{}
	rows, e := tx.Query(ctx, fmt.Sprintf(`SELECT product_id,bom_spec_id,spec_g,SUM(qty)::bigint FROM(SELECT product_id,bom_spec_id,spec_g,allocated_qty AS qty FROM %[1]s.customer_direct_ship_request_allocations WHERE order_id=$1 AND status='reserved' UNION ALL SELECT e.product_id,e.bom_spec_id,e.spec_g,r.reserved_qty-r.converted_qty-r.released_qty FROM %[1]s.customer_stock_supply_reservations r JOIN %[1]s.customer_stock_entitlements e ON e.id=r.entitlement_id WHERE r.order_id=$1)own GROUP BY product_id,bom_spec_id,spec_g`, r.schema), v.OrderID)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var product, spec, g, n int64
		if e = rows.Scan(&product, &spec, &g, &n); e != nil {
			rows.Close()
			return out, e
		}
		own[miniStockKey(product, spec, g)] = n
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	keys := []string{}
	for k := range by {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	families := map[int64]map[string]any{}
	for _, k := range keys {
		a := by[k]
		identity, e := resolveCustomerFulfillmentBOMSpecIdentityTx(ctx, tx, r.schema, a.product, a.spec, 0)
		if e != nil {
			continue
		}
		spec := map[string]any{"product_id": identity.ProductID, "bom_spec_id": identity.BomSpecID, "bom_variant_id": identity.BomVariantID, "bom_spec_key": identity.BomSpecKey, "spec_name": identity.BomSpecName, "spec_label": identity.BomSpecName, "inventory_unit": identity.InventoryUnit, "sales_unit": identity.InventoryUnit, "sku_code": identity.SpecCode, "spec_g": 0, "unit_price": 0, "stock_available_qty": a.physical, "production_available_qty": a.pending, "physical_available_qty": a.physical, "pending_inbound_qty": a.pending, "available_qty": a.physical + a.pending, "own_reserved_qty": own[k], "price_tiers": []map[string]any{}}
		family := families[identity.ProductID]
		if family == nil {
			family = map[string]any{"product_id": identity.ProductID, "parent_product_id": identity.ProductID, "id": identity.ProductID, "name": identity.ProductName, "product_name": identity.ProductName, "specs": []map[string]any{}}
			families[identity.ProductID] = family
			out.ProductFamilies = append(out.ProductFamilies, family)
		}
		family["specs"] = append(family["specs"].([]map[string]any), spec)
	}
	return out, nil
}

func (r *Repository) saveFulfillmentEditItems(ctx context.Context, tx pgx.Tx, v app.FulfillmentOrderEditContext, p *app.FulfillmentOrderEditPreview) error {
	old := map[int64]app.FulfillmentOrderEditItem{}
	for _, i := range v.Items {
		old[i.ItemID] = i
	}
	kept := map[int64]bool{}
	var maxLine int
	if e := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(MAX(line_no),0) FROM %s.order_items WHERE order_id=$1`, r.schema), v.OrderID).Scan(&maxLine); e != nil {
		return e
	}
	desiredExisting := map[int64]bool{}
	for _, i := range p.Items {
		if i.ItemID > 0 {
			desiredExisting[i.ItemID] = true
		}
	}
	for id, i := range old {
		if !desiredExisting[id] {
			if _, e := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_items SET qty=0,line_total=0,line_total_before_discount=0,discount_amount=0,matched_price_qty=0,material_item_weight_g=0,logistics_item_weight_g=0,weight_snapshot_json=weight_snapshot_json||'{"qty":0,"material_item_weight_g":0,"logistics_item_weight_g":0}'::jsonb WHERE id=$1`, r.schema), id); e != nil {
				return e
			}
			if _, e := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_direct_ship_request_items SET qty=0,snapshot=snapshot||'{"qty":0,"line_amount":0}'::jsonb WHERE id=$1`, r.schema), i.RequestItemID); e != nil {
				return e
			}
		}
	}
	for idx := range p.Items {
		i := &p.Items[idx]
		var line int
		if i.ItemID == 0 {
			// Tombstones with the same stable identity are reused. Never repurpose a
			// removed line with real conversion history for a different specification.
			e := tx.QueryRow(ctx, fmt.Sprintf(`SELECT i.id,i.line_no,ri.id FROM %[1]s.order_items i JOIN %[1]s.customer_direct_ship_request_items ri ON ri.request_id=$2 AND ri.line_no=i.line_no WHERE i.order_id=$1 AND i.qty=0 AND i.product_id=$3 AND i.bom_spec_id=$4 AND i.bom_variant_id=$5 ORDER BY i.id LIMIT 1 FOR UPDATE OF i`, r.schema), v.OrderID, v.RequestID, i.ProductID, i.BomSpecID, i.BomVariantID).Scan(&i.ItemID, &line, &i.RequestItemID)
			if e != nil && !errors.Is(e, pgx.ErrNoRows) {
				return e
			}
			if i.ItemID == 0 {
				maxLine++
				line = maxLine
				if e = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.order_items(order_id,line_no,product_id,bom_spec_id,bom_variant_id,item_name,spec,qty,unit,sales_unit,unit_price,line_total,bean_list_publication_id,price_source_json)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NULLIF($13,0),$14::jsonb) RETURNING id`, r.schema), v.OrderID, line, i.ProductID, i.BomSpecID, i.BomVariantID, i.ProductName, i.SpecName, i.Qty, i.InventoryUnit, i.SalesUnit, i.UnitPrice, i.LineAmount, i.PublicationID, i.PriceSourceJSON).Scan(&i.ItemID); e != nil {
					return e
				}
			}
		} else {
			if e := tx.QueryRow(ctx, fmt.Sprintf(`SELECT line_no FROM %s.order_items WHERE id=$1 AND order_id=$2 FOR UPDATE`, r.schema), i.ItemID, v.OrderID).Scan(&line); e != nil {
				return e
			}
		}
		if _, e := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_items SET product_id=$2,bom_spec_id=$3,bom_variant_id=$4,item_name=$5,spec=$6,qty=$7,unit=$8,sales_unit=$9,unit_price=$10,line_total=$11,line_total_before_discount=CASE WHEN $14::boolean THEN line_total_before_discount ELSE $11 END,discount_type=CASE WHEN $14::boolean THEN discount_type ELSE '' END,discount_value=CASE WHEN $14::boolean THEN discount_value ELSE 0 END,discount_amount=CASE WHEN $14::boolean THEN discount_amount ELSE 0 END,bean_list_publication_id=NULLIF($12,0),price_source_json=$13::jsonb,weight_snapshot_json=CASE WHEN product_id=$2 AND bom_spec_id=$3 AND bom_variant_id=$4 THEN weight_snapshot_json ELSE '{}'::jsonb END WHERE id=$1`, r.schema), i.ItemID, i.ProductID, i.BomSpecID, i.BomVariantID, i.ProductName, i.SpecName, i.Qty, i.InventoryUnit, i.SalesUnit, i.UnitPrice, i.LineAmount, i.PublicationID, i.PriceSourceJSON, func() bool {
			prev, exists := old[i.ItemID]
			return exists && prev.ProductID == i.ProductID && prev.BomSpecID == i.BomSpecID && prev.BomVariantID == i.BomVariantID && prev.Qty == i.Qty
		}()); e != nil {
			return e
		}
		snapshot := mustPayloadJSON(map[string]any{"order_item_id": i.ItemID, "product_id": i.ProductID, "bom_spec_id": i.BomSpecID, "bom_variant_id": i.BomVariantID, "bom_spec_key": i.BomSpecKey, "product_name": i.ProductName, "sku_code": i.SKUCode, "spec_label": i.SpecName, "inventory_unit": i.InventoryUnit, "sales_unit": i.SalesUnit, "spec_g": i.SpecG, "qty": i.Qty, "unit_price": i.UnitPrice, "line_amount": i.LineAmount, "price_table_publication_id": i.PublicationID})
		if i.RequestItemID == 0 {
			if e := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customer_direct_ship_request_items(request_id,line_no,product_id,bom_spec_id,bom_variant_id,bom_spec_key,product_name,sku_code,spec_label,inventory_unit,spec_g,qty,snapshot)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::jsonb)RETURNING id`, r.schema), v.RequestID, line, i.ProductID, i.BomSpecID, i.BomVariantID, i.BomSpecKey, i.ProductName, i.SKUCode, i.SpecName, i.InventoryUnit, i.SpecG, i.Qty, snapshot).Scan(&i.RequestItemID); e != nil {
				return e
			}
		} else {
			if _, e := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_direct_ship_request_items SET product_id=$2,bom_spec_id=$3,bom_variant_id=$4,bom_spec_key=$5,product_name=$6,sku_code=$7,spec_label=$8,inventory_unit=$9,spec_g=$10,qty=$11,snapshot=$12::jsonb WHERE id=$1`, r.schema), i.RequestItemID, i.ProductID, i.BomSpecID, i.BomVariantID, i.BomSpecKey, i.ProductName, i.SKUCode, i.SpecName, i.InventoryUnit, i.SpecG, i.Qty, snapshot); e != nil {
				return e
			}
		}
		kept[i.ItemID] = true
	}
	for id, i := range old {
		if !kept[id] {
			if _, e := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_items SET qty=0,line_total=0,line_total_before_discount=0,discount_amount=0,matched_price_qty=0,material_item_weight_g=0,logistics_item_weight_g=0,weight_snapshot_json=weight_snapshot_json||'{"qty":0,"material_item_weight_g":0,"logistics_item_weight_g":0}'::jsonb WHERE id=$1`, r.schema), id); e != nil {
				return e
			}
			if _, e := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_direct_ship_request_items SET qty=0,snapshot=snapshot||'{"qty":0,"line_amount":0}'::jsonb WHERE id=$1`, r.schema), i.RequestItemID); e != nil {
				return e
			}
		}
	}
	// Freeze only active rows; tombstones deliberately retain historical unit
	// weights while their commercial amount and total weights are zero.
	return orderweight.FreezeOrder(ctx, tx, r.schema, v.OrderID)
}

func (r *Repository) syncFulfillmentEditReservations(ctx context.Context, tx pgx.Tx, v app.FulfillmentOrderEditContext, items []app.FulfillmentOrderEditItem, stock []miniPlannedAllocation, pending []miniPlannedProcessingAllocation, actor string) error {
	desired := map[string]int64{}
	lines := map[int64]app.FulfillmentOrderEditItem{}
	for _, i := range items {
		lines[i.ItemID] = i
		for _, p := range pending {
			if p.ProductID == i.ProductID && p.BomSpecID == i.BomSpecID && p.SpecG == i.SpecG {
				desired[fmt.Sprintf("%d:%d", p.EntitlementID, i.ItemID)] += p.Qty
			}
		}
	}
	type hold struct{ id, entitlement, item, n int64 }
	holds := []hold{}
	rows, e := tx.Query(ctx, fmt.Sprintf(`SELECT id,entitlement_id,order_item_id,reserved_qty-converted_qty-released_qty FROM %s.customer_stock_supply_reservations WHERE order_id=$1 ORDER BY id FOR UPDATE`, r.schema), v.OrderID)
	if e != nil {
		return e
	}
	for rows.Next() {
		var h hold
		if e = rows.Scan(&h.id, &h.entitlement, &h.item, &h.n); e != nil {
			rows.Close()
			return e
		}
		holds = append(holds, h)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	// Release decreases first so one edit can move rights between its own lines.
	// Existing converted and received amounts never decrease.
	for _, decrease := range []bool{true, false} {
		for _, h := range holds {
			k := fmt.Sprintf("%d:%d", h.entitlement, h.item)
			n := desired[k]
			delta := n - h.n
			if (delta < 0) != decrease || delta == 0 {
				continue
			}
			if _, e = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_stock_supply_reservations SET reserved_qty=reserved_qty+GREATEST($2::bigint,0),released_qty=released_qty+GREATEST(-$2::bigint,0),status=CASE WHEN $3::bigint=0 THEN CASE WHEN converted_qty>0 THEN 'converted' ELSE 'released' END WHEN converted_qty>0 THEN 'partially_converted' ELSE 'reserved' END,updated_at=now() WHERE id=$1`, r.schema), h.id, delta, n); e != nil {
				return e
			}
		}
	}
	for _, h := range holds {
		delete(desired, fmt.Sprintf("%d:%d", h.entitlement, h.item))
	}
	for key, n := range desired {
		if n <= 0 {
			continue
		}
		var entitlement, item int64
		if _, e = fmt.Sscanf(key, "%d:%d", &entitlement, &item); e != nil {
			return e
		}
		i := lines[item]
		if e = customerstock.ReserveTx(ctx, tx, r.schema, entitlement, v.CustomerID, v.RequestID, i.RequestItemID, v.OrderID, item, n); e != nil {
			return e
		}
	}

	// Physical holds carry their real batch identity. Retain row ids and amounts
	// when a draft does not change that allocation, release only removed/excess.
	type physical struct {
		id, item, product, spec, variant, g, batch, n int64
		warehouse, code                               string
	}
	rows, e = tx.Query(ctx, fmt.Sprintf(`SELECT id,order_item_id,product_id,bom_spec_id,bom_variant_id,spec_g,batch_id,allocated_qty,warehouse_code,batch_code FROM %s.customer_direct_ship_request_allocations WHERE order_id=$1 AND status='reserved' ORDER BY id FOR UPDATE`, r.schema), v.OrderID)
	if e != nil {
		return e
	}
	old := []physical{}
	for rows.Next() {
		var a physical
		if e = rows.Scan(&a.id, &a.item, &a.product, &a.spec, &a.variant, &a.g, &a.batch, &a.n, &a.warehouse, &a.code); e != nil {
			rows.Close()
			return e
		}
		old = append(old, a)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	// The active allocation ledger is mutable reservation state, not a stock
	// movement. Its audit history remains in DSR allocations and operation logs.
	if _, e = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.order_stock_batch_allocations WHERE order_id=$1 AND request_id=$2`, r.schema), v.OrderID, v.RequestID); e != nil {
		return e
	}
	var requestOrder int64
	if e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.customer_direct_ship_request_orders WHERE order_id=$1 AND request_id=$2`, r.schema), v.OrderID, v.RequestID).Scan(&requestOrder); e != nil {
		return e
	}
	used := map[int64]bool{}
	for _, a := range stock {
		var i app.FulfillmentOrderEditItem
		for _, line := range items {
			if line.ProductID == a.ProductID && line.BomSpecID == a.BomSpecID && line.SpecG == a.SpecG {
				i = line
				break
			}
		}
		if i.ItemID == 0 {
			return fmt.Errorf("库存预占明细关联不一致")
		}
		grams, units := a.Qty*a.SpecG, int64(0)
		if a.BomSpecID > 0 {
			grams, units = 0, a.Qty
		}
		id := int64(0)
		for _, h := range old {
			if !used[h.id] && h.item == i.ItemID && h.product == a.ProductID && h.spec == a.BomSpecID && h.variant == a.BomVariantID && h.batch == a.BatchID && h.warehouse == a.Warehouse {
				id = h.id
				break
			}
		}
		if id > 0 {
			used[id] = true
			if _, e = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_direct_ship_request_allocations SET allocated_qty=$2,allocated_units=$3,allocated_g=$4 WHERE id=$1`, r.schema), id, a.Qty, units, grams); e != nil {
				return e
			}
		} else {
			if _, e = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_direct_ship_request_allocations(request_id,request_item_id,request_order_id,order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,spec_g,warehouse_code,batch_id,batch_code,allocated_qty,allocated_units,allocated_g,status)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,'reserved')`, r.schema), v.RequestID, i.RequestItemID, requestOrder, v.OrderID, i.ItemID, a.ProductID, a.BomSpecID, a.BomVariantID, a.SpecG, a.Warehouse, a.BatchID, a.BatchCode, a.Qty, units, grams); e != nil {
				return e
			}
		}
		if _, e = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_stock_batch_allocations(order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,spec_g,need_g,need_units,batch_id,batch_code,allocated_g,allocated_units,warehouse,request_id,operator)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$7,$8,$11,$12,$13)`, r.schema), v.OrderID, i.ItemID, a.ProductID, a.BomSpecID, a.BomVariantID, a.SpecG, grams, units, a.BatchID, a.BatchCode, a.Warehouse, v.RequestID, actor); e != nil {
			return e
		}
	}
	for _, h := range old {
		if !used[h.id] {
			if _, e = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_direct_ship_request_allocations SET status='released' WHERE id=$1`, r.schema), h.id); e != nil {
				return e
			}
		}
	}
	return nil
}

// Preserve each old physical batch and pending entitlement before taking any
// newly available stock. This makes an address edit and an unchanged line a
// true no-op on its reservation sources even when other stock has just arrived.
func (r *Repository) planFulfillmentEdit(ctx context.Context, tx pgx.Tx, v app.FulfillmentOrderEditContext, items []app.MiniDirectShipItemCommand, stock []miniStockCandidate, outputs []miniProcessingOutputCandidate) ([]miniPlannedAllocation, []miniPlannedProcessingAllocation, []app.MiniDirectShipShortage, error) {
	type hold struct {
		product, spec, g, batch, entitlement, n int64
		warehouse                               string
	}
	holds := []hold{}
	rows, e := tx.Query(ctx, fmt.Sprintf(`SELECT product_id,bom_spec_id,spec_g,batch_id,warehouse_code,allocated_qty FROM %s.customer_direct_ship_request_allocations WHERE order_id=$1 AND status='reserved' ORDER BY id`, r.schema), v.OrderID)
	if e != nil {
		return nil, nil, nil, e
	}
	for rows.Next() {
		var h hold
		if e = rows.Scan(&h.product, &h.spec, &h.g, &h.batch, &h.warehouse, &h.n); e != nil {
			rows.Close()
			return nil, nil, nil, e
		}
		holds = append(holds, h)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, nil, nil, e
	}
	rows, e = tx.Query(ctx, fmt.Sprintf(`SELECT e.product_id,e.bom_spec_id,e.spec_g,e.id,r.reserved_qty-r.converted_qty-r.released_qty FROM %[1]s.customer_stock_supply_reservations r JOIN %[1]s.customer_stock_entitlements e ON e.id=r.entitlement_id WHERE r.order_id=$1 AND r.reserved_qty>r.converted_qty+r.released_qty ORDER BY r.id`, r.schema), v.OrderID)
	if e != nil {
		return nil, nil, nil, e
	}
	for rows.Next() {
		var h hold
		if e = rows.Scan(&h.product, &h.spec, &h.g, &h.entitlement, &h.n); e != nil {
			rows.Close()
			return nil, nil, nil, e
		}
		holds = append(holds, h)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, nil, nil, e
	}
	physical := []miniPlannedAllocation{}
	pending := []miniPlannedProcessingAllocation{}
	remaining := []app.MiniDirectShipItemCommand{}
	for _, i := range items {
		for _, h := range holds {
			if i.Qty == 0 {
				break
			}
			if h.product != i.ProductID || h.spec != i.BomSpecID || h.g != i.SpecG {
				continue
			}
			if h.entitlement > 0 {
				for index := range outputs {
					a := &outputs[index]
					if a.EntitlementID != h.entitlement {
						continue
					}
					n := min(i.Qty, h.n, a.AvailableQty)
					if n > 0 {
						pending = append(pending, miniPlannedProcessingAllocation{EntitlementID: a.EntitlementID, ProductID: a.ProductID, BomSpecID: a.BomSpecID, BomVariantID: a.BomVariantID, SpecG: a.SpecG, Qty: n})
						a.AvailableQty -= n
						i.Qty -= n
					}
				}
			} else {
				for index := range stock {
					a := &stock[index]
					if a.ProductID != h.product || a.BomSpecID != h.spec || a.SpecG != h.g || a.BatchID != h.batch || a.Warehouse != h.warehouse {
						continue
					}
					n := min(i.Qty, h.n, a.AvailableQty)
					if n > 0 {
						physical = append(physical, miniPlannedAllocation{ProductID: a.ProductID, BomSpecID: a.BomSpecID, BomVariantID: a.BomVariantID, SpecG: a.SpecG, Warehouse: a.Warehouse, BatchID: a.BatchID, BatchCode: a.BatchCode, Qty: n})
						a.AvailableQty -= n
						i.Qty -= n
					}
				}
			}
		}
		if i.Qty > 0 {
			remaining = append(remaining, i)
		}
	}
	a, b, c := planMiniDirectShipFulfillment(remaining, stock, outputs)
	physical = append(physical, a...)
	pending = append(pending, b...)
	// Consolidate a retained allocation and its increased delta into one row.
	combined := []miniPlannedAllocation{}
	for _, a := range physical {
		found := false
		for j := range combined {
			b := &combined[j]
			if a.ProductID == b.ProductID && a.BomSpecID == b.BomSpecID && a.BomVariantID == b.BomVariantID && a.SpecG == b.SpecG && a.Warehouse == b.Warehouse && a.BatchID == b.BatchID {
				b.Qty += a.Qty
				found = true
				break
			}
		}
		if !found {
			combined = append(combined, a)
		}
	}
	return combined, pending, c, nil
}

type fulfillmentEditAmounts struct {
	Gross, Discount, Grand, Shipping float64
	GoodsTotalCents                  int64
	Payment                          salesapp.SaveOrderCommand
}

// The frozen order's net payable includes full-order discounts and historical
// rounding in addition to its line amounts. Change it only by the difference
// between old/new net lines. Context, preview and save share this calculation,
// so displaying a draft never drops a preserved order-level adjustment.
func (r *Repository) fulfillmentEditAmountsTx(ctx context.Context, tx pgx.Tx, v app.FulfillmentOrderEditContext, items []app.FulfillmentOrderEditItem) (fulfillmentEditAmounts, error) {
	var a fulfillmentEditAmounts
	var oldGross, oldGrand, oldDiscount float64
	e := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(total_amount,0)::float8,(CASE WHEN COALESCE(grand_total,0)<>0 OR COALESCE(discount_amount,0)<>0 OR COALESCE(shipping_amount,0)<>0 OR COALESCE(rounding_amount,0)<>0 THEN COALESCE(grand_total,0) ELSE COALESCE(total_amount,0) END)::float8,COALESCE(shipping_amount,0)::float8,COALESCE(discount_amount,0)::float8,COALESCE(pay_status_id,0),COALESCE(payment_goods_amount,0)::float8,COALESCE(payment_shipping_amount,0)::float8,COALESCE(payment_voucher_asset_id,0)FROM %s.orders WHERE id=$1`, r.schema), v.OrderID).Scan(&oldGross, &oldGrand, &a.Shipping, &oldDiscount, &a.Payment.PayStatusID, &a.Payment.PaymentGoodsAmount, &a.Payment.PaymentShippingAmount, &a.Payment.PaymentVoucherAssetID)
	if e != nil {
		return a, e
	}
	cents := func(n float64) int64 { return int64(math.Round(n * 100)) }
	oldLineGross, oldLineNet, oldLineDiscount, newLineGross, newLineNet, newLineDiscount := int64(0), int64(0), int64(0), int64(0), int64(0), int64(0)
	for _, i := range v.Items {
		oldLineGross += cents(i.BaseLineAmount)
		oldLineNet += cents(i.LineAmount)
		oldLineDiscount += cents(i.DiscountAmount)
	}
	for _, i := range items {
		newLineGross += cents(i.BaseLineAmount)
		newLineNet += cents(i.LineAmount)
		newLineDiscount += cents(i.DiscountAmount)
	}
	gross := cents(oldGross) + newLineGross - oldLineGross
	discount := cents(oldDiscount) + newLineDiscount - oldLineDiscount
	grand := cents(oldGrand) + newLineNet - oldLineNet
	if gross < 0 || grand < 0 {
		return a, fmt.Errorf("修改后订单应收金额无效")
	}
	if discount < 0 {
		discount = 0
	}
	a.Gross, a.Discount, a.Grand = float64(gross)/100, float64(discount)/100, float64(grand)/100
	a.GoodsTotalCents = grand - cents(a.Shipping)
	if a.GoodsTotalCents < 0 {
		return a, fmt.Errorf("修改后商品应收金额无效")
	}
	return a, nil
}
