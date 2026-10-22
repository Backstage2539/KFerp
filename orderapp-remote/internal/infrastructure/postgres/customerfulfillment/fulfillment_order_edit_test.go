package customerfulfillment

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"math"
	"net/http/httptest"
	authzapp "orderapp/internal/application/authz"
	app "orderapp/internal/application/customerfulfillment"
	salesapp "orderapp/internal/application/sales"
	customerstock "orderapp/internal/infrastructure/postgres/customerstock"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
	saleshttp "orderapp/internal/interfaces/http/sales"
	"os"
	"strings"
	"testing"
	"time"
)

type fulfillmentEditFixture struct {
	pool    *pgxpool.Pool
	schema  string
	repo    *Repository
	svc     *app.Service
	request app.MiniDirectShipRequest
}

func newFulfillmentEditFixture(t *testing.T) fulfillmentEditFixture {
	t.Helper()
	ctx := context.Background()
	pool, schema := seedPR703StockProduct(t)
	seedPR706FrozenWeightMaterials(t, pool, schema)
	configurePR703StockFees(t, pool, schema, 7031)
	templates, err := NewRepository(pool, schema).ListShippingFeeTemplates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	tm := templates[0]
	tm.Rules = append(tm.Rules, app.ShippingFeeRegionRule{Province: "北京市", FirstWeightG: 1000, FirstFeeCents: 610, AdditionalWeightG: 1000, AdditionalFeeCents: 200})
	if _, err = NewRepository(pool, schema).SaveShippingFeeTemplate(ctx, app.SaveShippingFeeTemplateCommand{Template: tm, ExpectedRevision: tm.Revision, Actor: "test"}); err != nil {
		t.Fatal(err)
	}
	if _, e := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,delivery_mode,delivery_warehouse_code) VALUES(71401,'EDIT-SOURCE',501,'customer_stock','DEV-E2E-A1');INSERT INTO %[1]s.order_items(order_id,line_no,product_id,bom_spec_id,bom_variant_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(71401,1,910,7031,7032,'客户已购','454g袋',12,'袋',30,360)`, schema)); e != nil {
		t.Fatal(e)
	}
	repo := NewRepository(pool, schema)
	svc := app.NewService(repo)
	svc.UseSalesOrderService(salesapp.NewService(postgressales.NewRepository(pool, schema)))
	c := app.MiniDirectShipCommand{FreightProtocol: "deferred_freight_v1", CustomerID: 501, FulfillmentMode: "stock", IdempotencyKey: "EDIT-STOCK", RecipientName: "测试", RecipientPhone: "13800000000", Province: "云南省", City: "大理市", DetailAddress: "测试路1号", RecipientCompany: "原公司", Actor: "employee:703", Items: []app.MiniDirectShipItemCommand{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 4}}}
	p, e := svc.PreviewMiniDirectShip(ctx, c)
	if e != nil {
		t.Fatal(e)
	}
	c.PriceQuoteToken, c.FeeQuoteToken = p.PriceQuoteToken, p.FeeQuoteToken
	q, e := svc.SubmitMiniDirectShip(ctx, c)
	if e != nil {
		t.Fatal(e)
	}
	return fulfillmentEditFixture{pool, schema, repo, svc, q}
}
func editCommand(v app.FulfillmentOrderEditContext) app.FulfillmentOrderEditCommand {
	c := app.FulfillmentOrderEditCommand{OrderID: v.OrderID, CustomerID: v.CustomerID, Actor: "employee:703", ExpectedEditRevision: v.EditRevision, OrderDate: v.OrderDate, ReceiverName: v.ReceiverName, ReceiverPhone: v.ReceiverPhone, ReceiverAddress: v.ReceiverAddress, Province: v.Province, City: v.City, District: v.District, Notes: v.Notes}
	for _, i := range v.Items {
		c.Items = append(c.Items, app.FulfillmentOrderEditItemCommand{ItemID: i.ItemID, ProductID: i.ProductID, BomSpecID: i.BomSpecID, BomVariantID: i.BomVariantID, SpecG: i.SpecG, Qty: i.Qty, SalesUnit: i.SalesUnit, PublicationID: i.PublicationID})
	}
	return c
}
func TestFulfillmentOrderEditPreservesIdentityAndAdjustsOwnedPendingRights(t *testing.T) {
	f := newFulfillmentEditFixture(t)
	ctx := context.Background()
	v, e := f.svc.FulfillmentOrderEdit(ctx, f.request.OrderID)
	if e != nil {
		t.Fatalf("load editable fulfillment order: %v", e)
	}
	if !v.CanEdit || len(v.Items) != 1 || v.Items[0].Qty != 4 {
		t.Fatalf("context %+v", v)
	}
	item, requestItem := v.Items[0].ItemID, v.Items[0].RequestItemID
	c := editCommand(v)
	c.Items[0].Qty = 7
	c.ReceiverAddress = "新地址"
	c.Notes = "调整代发数量"
	p, e := f.svc.PreviewFulfillmentOrderEdit(ctx, c)
	if e != nil {
		t.Fatal(e)
	}
	c.PreviewToken = p.PreviewToken
	got, e := f.svc.UpdateFulfillmentOrder(ctx, c)
	if e != nil {
		t.Fatal(e)
	}
	if !got.OK || got.OrderID != v.OrderID || got.RequestID != v.RequestID || got.Items[0].ItemID != item || got.Items[0].RequestItemID != requestItem || got.GoodsTotalCents != 0 {
		t.Fatalf("identity or duplicate goods charge %+v", got)
	}
	var pending, sourceQty int64
	var goods, paid float64
	var company string
	if e = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(reserved_qty-converted_qty-released_qty),0) FROM %s.customer_stock_supply_reservations WHERE order_id=$1`, f.schema), v.OrderID).Scan(&pending); e != nil {
		t.Fatal(e)
	}
	if e = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty FROM %s.order_items WHERE order_id=71401`, f.schema)).Scan(&sourceQty); e != nil {
		t.Fatal(e)
	}
	if e = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT total_amount,payment_goods_amount,receiver_company FROM %s.orders WHERE id=$1`, f.schema), v.OrderID).Scan(&goods, &paid, &company); e != nil {
		t.Fatal(e)
	}
	if pending != 7 || sourceQty != 12 || goods != 0 || paid != 0 || company != "原公司" {
		t.Fatalf("pending%d source%d goods%v paid%v company%s", pending, sourceQty, goods, paid, company)
	}
	if _, e = f.svc.UpdateFulfillmentOrder(ctx, c); e == nil {
		t.Fatal("stale draft accepted")
	}
	v, e = f.svc.FulfillmentOrderEdit(ctx, v.OrderID)
	if e != nil {
		t.Fatal(e)
	}
	c = editCommand(v)
	c.Items[0].Qty = 13
	if _, e = f.svc.PreviewFulfillmentOrderEdit(ctx, c); e == nil {
		t.Fatal("accepted more than all owned rights")
	}
}

func (f fulfillmentEditFixture) save(t *testing.T, c app.FulfillmentOrderEditCommand) app.FulfillmentOrderEditResult {
	t.Helper()
	p, e := f.svc.PreviewFulfillmentOrderEdit(context.Background(), c)
	if e != nil {
		t.Fatal(e)
	}
	c.PreviewToken = p.PreviewToken
	r, e := f.svc.UpdateFulfillmentOrder(context.Background(), c)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func TestFulfillmentOrderEditAllOwnedWarehousesAndOtherOrders(t *testing.T) {
	f := newFulfillmentEditFixture(t)
	ctx := context.Background()
	// A second order holds every remaining entitlement in the original warehouse.
	var entitlement int64
	if e := f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.customer_stock_entitlements WHERE source_order_id=71401`, f.schema)).Scan(&entitlement); e != nil {
		t.Fatal(e)
	}
	if _, e := f.pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id)VALUES(71402,'OTHER-DROPSHIP',501);INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,bom_spec_id,bom_variant_id,qty)VALUES(71402,71402,1,910,7031,7032,8);INSERT INTO %[1]s.customer_stock_supply_reservations(entitlement_id,customer_id,request_id,request_item_id,order_id,order_item_id,reserved_qty)VALUES(%[2]d,501,999,999,71402,71402,8);INSERT INTO %[1]s.orders(id,order_no,customer_id,delivery_mode,delivery_warehouse_code)VALUES(71403,'OTHER-OWNED-WAREHOUSE',501,'customer_stock','DEV-E2E-A2');INSERT INTO %[1]s.order_items(order_id,line_no,product_id,bom_spec_id,bom_variant_id,item_name,qty,unit,unit_price,line_total)VALUES(71403,1,910,7031,7032,'另一个本人仓库权益',3,'袋',30,90)`, f.schema, entitlement)); e != nil {
		t.Fatal(e)
	}
	v, e := f.svc.FulfillmentOrderEdit(ctx, f.request.OrderID)
	if e != nil {
		t.Fatal(e)
	}
	c := editCommand(v)
	c.Items[0].Qty = 7
	f.save(t, c)
	var own, other, extra int64
	if e = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(r.reserved_qty-r.converted_qty-r.released_qty)FILTER(WHERE r.order_id=$1 AND e.source_order_id=71401),0),COALESCE(SUM(r.reserved_qty-r.converted_qty-r.released_qty)FILTER(WHERE r.order_id=71402),0),COALESCE(SUM(r.reserved_qty-r.converted_qty-r.released_qty)FILTER(WHERE r.order_id=$1 AND e.source_order_id=71403),0)FROM %[1]s.customer_stock_supply_reservations r JOIN %[1]s.customer_stock_entitlements e ON e.id=r.entitlement_id`, f.schema), v.OrderID).Scan(&own, &other, &extra); e != nil {
		t.Fatal(e)
	}
	if own != 4 || other != 8 || extra != 3 {
		t.Fatalf("ownaddback%d other%d extraWarehouse%d", own, other, extra)
	}
	v, e = f.svc.FulfillmentOrderEdit(ctx, v.OrderID)
	if e != nil {
		t.Fatal(e)
	}
	c = editCommand(v)
	c.Items[0].Qty = 8
	if _, e = f.svc.PreviewFulfillmentOrderEdit(ctx, c); e == nil {
		t.Fatal("another order's reserved stock was taken")
	}
	c = editCommand(v)
	c.CustomerID = 502
	if _, e = f.svc.PreviewFulfillmentOrderEdit(ctx, c); e == nil {
		t.Fatal("customer mismatch accepted")
	}
}
func TestFulfillmentOrderEditAddRemoveReplaceAndTombstone(t *testing.T) {
	f := newFulfillmentEditFixture(t)
	ctx := context.Background()
	if _, e := f.pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.production_bom_specs(id,bom_id,code,spec_key,name,inventory_unit)VALUES(7143,7030,'EDIT-2','EDIT-2','新规格','袋');INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,logistics_weight_g)VALUES(7144,7030,7143,'新规格','袋',270);INSERT INTO %[1]s.orders(id,order_no,customer_id,delivery_mode,delivery_warehouse_code)VALUES(71404,'EDIT-NEW-SPEC-SOURCE',501,'customer_stock','DEV-E2E-A1');INSERT INTO %[1]s.order_items(order_id,line_no,product_id,bom_spec_id,bom_variant_id,item_name,qty,unit,unit_price,line_total)VALUES(71404,1,910,7143,7144,'新规格已购',5,'袋',20,100)`, f.schema)); e != nil {
		t.Fatal(e)
	}
	v, e := f.svc.FulfillmentOrderEdit(ctx, f.request.OrderID)
	if e != nil {
		t.Fatal(e)
	}
	oldItem := v.Items[0].ItemID
	c := editCommand(v)
	c.Items = append(c.Items, app.FulfillmentOrderEditItemCommand{ProductID: 910, BomSpecID: 7143, BomVariantID: 7144, Qty: 2})
	r := f.save(t, c)
	if len(r.Items) != 2 || r.Items[0].ItemID != oldItem || r.Items[1].ItemID == 0 {
		t.Fatalf("add %+v", r.Items)
	}
	newItem := r.Items[1].ItemID
	v, e = f.svc.FulfillmentOrderEdit(ctx, v.OrderID)
	if e != nil {
		t.Fatal(e)
	}
	c = editCommand(v)
	c.Items = c.Items[1:]
	r = f.save(t, c)
	if len(r.Items) != 1 || r.Items[0].ItemID != newItem {
		t.Fatalf("remove %+v", r)
	}
	var qty, grams, amount int64
	if e = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty::bigint,COALESCE(logistics_item_weight_g,0)::bigint,line_total::bigint FROM %s.order_items WHERE id=$1`, f.schema), oldItem).Scan(&qty, &grams, &amount); e != nil {
		t.Fatal(e)
	}
	if qty != 0 || grams != 0 || amount != 0 {
		t.Fatalf("tombstone qty%d weight%d amount%d", qty, grams, amount)
	}
	request, e := f.svc.GetMiniDirectShipRequest(ctx, 501, v.RequestID)
	if e != nil {
		t.Fatal(e)
	}
	if len(request.Items) != 1 || request.Items[0].BomSpecID != 7143 {
		t.Fatalf("mini request still exposes removed line %+v", request.Items)
	}
	v, e = f.svc.FulfillmentOrderEdit(ctx, v.OrderID)
	if e != nil {
		t.Fatal(e)
	}
	c = editCommand(v)
	c.Items[0].BomSpecID, c.Items[0].BomVariantID = 7031, 7032
	c.Items[0].Qty = 3
	r = f.save(t, c)
	if r.Items[0].ItemID != oldItem || r.Items[0].BomSpecID != 7031 {
		t.Fatalf("same-identity retired row not reused %+v", r.Items)
	}
	// The replaced line's historical identity is still available for audit.
	if e = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty::bigint FROM %s.order_items WHERE id=$1 AND bom_spec_id=7143`, f.schema), newItem).Scan(&qty); e != nil || qty != 0 {
		t.Fatalf("replacement destroyed old identity qty%d %v", qty, e)
	}
}
func TestFulfillmentOrderEditGuardsAndPreviewAtomicity(t *testing.T) {
	f := newFulfillmentEditFixture(t)
	ctx := context.Background()
	v, e := f.svc.FulfillmentOrderEdit(ctx, f.request.OrderID)
	if e != nil {
		t.Fatal(e)
	}
	c := editCommand(v)
	p, e := f.svc.PreviewFulfillmentOrderEdit(ctx, c)
	if e != nil {
		t.Fatal(e)
	}
	c.PreviewToken = p.PreviewToken
	c.ReceiverAddress = "预览后篡改地址"
	if _, e = f.svc.UpdateFulfillmentOrder(ctx, c); e == nil {
		t.Fatal("tampered preview accepted")
	}
	for _, tc := range []struct{ name, sql string }{{"managed", `confirmation_required=true`}, {"pending", `confirmation_status='pending'`}, {"void", `is_void=true`}, {"shipped", `ship_status_id=(SELECT id FROM %s.ship_statuses WHERE name='已发货' LIMIT 1)`}} {
		t.Run(tc.name, func(t *testing.T) {
			sql := fmt.Sprintf(tc.sql, f.schema)
			if tc.name == "shipped" {
				if _, e = f.pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.ship_statuses(name)VALUES('已发货 ')`, f.schema)); e != nil {
					t.Fatal(e)
				}
				sql = fmt.Sprintf(`ship_status_id=(SELECT id FROM %s.ship_statuses WHERE name='已发货 ' LIMIT 1)`, f.schema)
			}
			if tc.name != "shipped" {
				sql = tc.sql
			}
			if _, e = f.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET %s WHERE id=$1`, f.schema, sql), v.OrderID); e != nil {
				t.Fatal(e)
			}
			locked, e := f.svc.FulfillmentOrderEdit(ctx, v.OrderID)
			if e != nil {
				t.Fatal(e)
			}
			if locked.CanEdit || locked.EditBlockReason == "" || len(locked.Items) != 1 {
				t.Fatalf("locked context %+v", locked)
			}
			c = editCommand(locked)
			if _, e = f.svc.PreviewFulfillmentOrderEdit(ctx, c); e == nil {
				t.Fatal("locked order preview accepted")
			}
			if _, e = f.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET confirmation_required=false,confirmation_status='accepted',is_void=false,ship_status_id=(SELECT id FROM %s.ship_statuses WHERE name IN('未发货','待发货')ORDER BY id LIMIT 1)WHERE id=$1`, f.schema, f.schema), v.OrderID); e != nil {
				t.Fatal(e)
			}
		})
	}
	var qty int64
	if e = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty::bigint FROM %s.order_items WHERE id=$1`, f.schema), v.Items[0].ItemID).Scan(&qty); e != nil || qty != 4 {
		t.Fatalf("failed edits mutated qty%d %v", qty, e)
	}
}

func TestFulfillmentOrderEditConvertedHistoryAndPhysicalBatches(t *testing.T) {
	f := newFulfillmentEditFixture(t)
	ctx := context.Background()
	tx, e := f.pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	if e = customerstock.LockFulfillmentCustomersTx(ctx, tx, f.schema, "EDIT-SOURCE"); e != nil {
		t.Fatal(e)
	}
	var batch int64
	if e = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.stock_batches(batch_code,item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,owner_customer_id,qty_units,remaining_units,quality_status)VALUES('EDIT-RECEIPT','finished_product',910,'实际入库',0,7031,7032,501,3,3,'passed')RETURNING id`, f.schema)).Scan(&batch); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.finished_inventory SET onhand_units=onhand_units+3 WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='DEV-E2E-A1';INSERT INTO %[1]s.stock_ledger_entries(item_type,item_id,item_name,owner_customer_id,spec_g,bom_spec_id,bom_variant_id,warehouse,source_doc_type,source_doc_id,source_batch_code,qty_change_units,qty_after_units)VALUES('finished_product',910,'实际入库',501,0,7031,7032,'DEV-E2E-A1','production_run',703,'EDIT-RECEIPT',3,3)`, f.schema)); e != nil {
		t.Fatal(e)
	}
	if e = customerstock.ReceiveProductionTx(ctx, tx, f.schema, 703, 910, 7031, 7032, 0, 3, "EDIT-RECEIPT", "EDIT-RECEIPT", "DEV-E2E-A1", "EDIT-SOURCE", "employee:703"); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	v, e := f.svc.FulfillmentOrderEdit(ctx, f.request.OrderID)
	if e != nil {
		t.Fatal(e)
	}
	item := v.Items[0].ItemID
	c := editCommand(v)
	c.ReceiverAddress = "云南省大理市测试新地址"
	f.save(t, c)
	var allocated, received, converted, pending, receipts, ledger int64
	var chosen int64
	check := func() {
		t.Helper()
		if e = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(allocated_units),0),COALESCE(MIN(batch_id),0)FROM %s.order_stock_batch_allocations WHERE order_id=$1`, f.schema), v.OrderID).Scan(&allocated, &chosen); e != nil {
			t.Fatal(e)
		}
		if e = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT e.received_qty,r.converted_qty,r.reserved_qty-r.converted_qty-r.released_qty FROM %[1]s.customer_stock_entitlements e JOIN %[1]s.customer_stock_supply_reservations r ON r.entitlement_id=e.id WHERE r.order_item_id=$1`, f.schema), item).Scan(&received, &converted, &pending); e != nil {
			t.Fatal(e)
		}
		if e = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT count(*)FROM %[1]s.customer_stock_entitlement_receipts WHERE receipt_key='EDIT-RECEIPT'),(SELECT count(*)FROM %[1]s.stock_ledger_entries WHERE source_batch_code='EDIT-RECEIPT')`, f.schema)).Scan(&receipts, &ledger); e != nil {
			t.Fatal(e)
		}
	}
	check()
	if allocated != 3 || chosen != batch || received != 3 || converted != 3 || pending != 1 || receipts != 1 || ledger != 1 {
		t.Fatalf("unchanged physical%d batch%d received%d converted%d pending%d receipts%d ledger%d", allocated, chosen, received, converted, pending, receipts, ledger)
	}
	v, e = f.svc.FulfillmentOrderEdit(ctx, v.OrderID)
	if e != nil {
		t.Fatal(e)
	}
	c = editCommand(v)
	c.Items[0].Qty = 2
	f.save(t, c)
	check()
	if allocated != 2 || chosen != batch || received != 3 || converted != 3 || pending != 0 || receipts != 1 || ledger != 1 {
		t.Fatalf("decrease changed history: physical%d batch%d received%d converted%d pending%d receipts%d ledger%d", allocated, chosen, received, converted, pending, receipts, ledger)
	}
	var available int64
	if e = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units FROM %s.finished_inventory WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='DEV-E2E-A1'`, f.schema)).Scan(&available); e != nil || available != 3 {
		t.Fatalf("physical balance rewritten %d %v", available, e)
	}
}

func newOnDemandFulfillmentEditFixture(t *testing.T) fulfillmentEditFixture {
	f := newFulfillmentEditFixture(t)
	ctx := context.Background()
	settings, e := f.repo.LoadFulfillmentSettings(ctx, 501)
	if e != nil {
		t.Fatal(e)
	}
	for j := range settings.Modes {
		settings.Modes[j].ShippingPolicy = nil
	}
	mode := settings.Modes[0]
	mode.Mode = "on_demand"
	mode.WarehouseCode = ""
	settings.Modes = []app.FulfillmentModeSettings{settings.Modes[0], mode}
	if _, e = f.repo.SaveFulfillmentSettings(ctx, app.SaveFulfillmentSettingsCommand{Settings: settings, ExpectedRevision: settings.Revision, Actor: "employee:703"}); e != nil {
		t.Fatal(e)
	}
	var pub int64
	content := `{"price_rows":[{"product_id":910,"parent_product_id":910,"bom_spec_id":7031,"bom_variant_id":7032,"source_price_record_id":703454,"min_qty":1,"final_unit_price":30,"price_unit":"袋","inventory_unit":"袋","quantity_basis":"sales_spec_count","effective_sales_spec":{"product_id":910,"parent_product_id":910,"bom_spec_id":7031,"bom_variant_id":7032,"net_content_qty":454,"net_content_unit":"g","sales_unit":"袋","inventory_unit":"袋"}}]}`
	if e = f.pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.bean_list_publications(list_type,version_no,status,owner_type,owner_key,publication_purpose,config_json,content_json,changelog,actor)VALUES('commercial','EDIT-ON-DEMAND','published','official','','factory_supply','{}',$1::jsonb,'','test')RETURNING id`, f.schema), content).Scan(&pub); e != nil {
		t.Fatal(e)
	}
	if _, e = f.pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_order_price_table_bindings(customer_id,usage_code,product_type_key,publication_id)VALUES(501,'direct_ship','commercial',$1)`, f.schema), pub); e != nil {
		t.Fatal(e)
	}
	c := app.MiniDirectShipCommand{FreightProtocol: "deferred_freight_v1", CustomerID: 501, FulfillmentMode: "on_demand", IdempotencyKey: "EDIT-ON-DEMAND", RecipientName: "测试", RecipientPhone: "13800000000", Province: "云南省", City: "大理市", DetailAddress: "云南省大理市测试路", RecipientCompany: "原公司", Actor: "employee:703", Items: []app.MiniDirectShipItemCommand{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 4}}}
	p, e := f.svc.PreviewMiniDirectShip(ctx, c)
	if e != nil {
		t.Fatal(e)
	}
	c.PriceQuoteToken, c.FeeQuoteToken = p.PriceQuoteToken, p.FeeQuoteToken
	f.request, e = f.svc.SubmitMiniDirectShip(ctx, c)
	if e != nil {
		t.Fatal(e)
	}
	return f
}
func TestFulfillmentOrderEditOnDemandFrozenPricesAndHistoricalPayment(t *testing.T) {
	f := newOnDemandFulfillmentEditFixture(t)
	ctx := context.Background()
	v, e := f.svc.FulfillmentOrderEdit(ctx, f.request.OrderID)
	if e != nil {
		t.Fatal(e)
	}
	price := v.Items[0].UnitPrice
	item := v.Items[0].ItemID
	if price != 30 || len(v.Catalog.ProductFamilies) == 0 {
		t.Fatalf("on demand context %+v", v)
	}
	// A historical fully-paid order with no voucher still represents real money.
	if _, e = f.pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.pay_statuses(name)SELECT '预付款（付款未完成）' WHERE NOT EXISTS(SELECT 1 FROM %[1]s.pay_statuses WHERE name LIKE '%%预付款%%');UPDATE %[1]s.orders SET pay_status_id=(SELECT id FROM %[1]s.pay_statuses WHERE name='已付款' LIMIT 1),payment_goods_amount=0,prepayment_amount=0,document_date='2026-01-01' WHERE id=%[2]d;UPDATE %[1]s.bean_list_publications SET content_json=jsonb_set(content_json,'{price_rows,0,final_unit_price}','99')WHERE version_no='EDIT-ON-DEMAND'`, f.schema, v.OrderID)); e != nil {
		t.Fatal(e)
	}
	v, e = f.svc.FulfillmentOrderEdit(ctx, v.OrderID)
	if e != nil {
		t.Fatal(e)
	}
	c := editCommand(v)
	c.Items[0].Qty = 5
	r := f.save(t, c)
	if r.Items[0].ItemID != item || r.Items[0].UnitPrice != 30 || r.GoodsTotalCents != 15000 {
		t.Fatalf("historical price or identity changed %+v", r)
	}
	var prepay, total, receipt float64
	var status, date string
	if e = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT prepayment_amount,grand_total,payment_goods_amount,ps.name,to_char(o.document_date,'YYYY-MM-DD')FROM %[1]s.orders o LEFT JOIN %[1]s.pay_statuses ps ON ps.id=o.pay_status_id WHERE o.id=$1`, f.schema), v.OrderID).Scan(&prepay, &total, &receipt, &status, &date); e != nil {
		t.Fatal(e)
	}
	if prepay != 120 || total != 150 || receipt != 0 || !strings.Contains(status, "预付款") || date != "2026-01-01" {
		t.Fatalf("receipt lost prepay%v total%v receipt%v status%s date%s", prepay, total, receipt, status, date)
	}
	v, e = f.svc.FulfillmentOrderEdit(ctx, v.OrderID)
	if e != nil {
		t.Fatal(e)
	}
	c = editCommand(v)
	c.Items[0].Qty = 3
	p, e := f.svc.PreviewFulfillmentOrderEdit(ctx, c)
	if e == nil {
		c.PreviewToken = p.PreviewToken
		if _, e = f.svc.UpdateFulfillmentOrder(ctx, c); e == nil {
			t.Fatal("reduction erased legacy paid money")
		}
	}
	// Both staff edit entrances use the same locked stock/fee-aware operation.
	v, e = f.svc.FulfillmentOrderEdit(ctx, v.OrderID)
	if e != nil {
		t.Fatal(e)
	}
	c = app.FulfillmentOrderEditCommand{OrderID: v.OrderID, CustomerID: 501, ExpectedEditRevision: v.EditRevision, Actor: "employee:703", RecipientOnly: true, ReceiverName: v.ReceiverName, ReceiverPhone: v.ReceiverPhone, ReceiverAddress: "北京市朝阳区测试路1号"}
	r, e = f.svc.UpdateFulfillmentOrder(ctx, c)
	if e != nil {
		t.Fatal(e)
	}
	if r.Items[0].UnitPrice != 30 || r.Items[0].Qty != 5 || r.Items[0].ItemID != item || r.Fees.Freight.Destination.Province != "北京市" {
		t.Fatalf("recipient edit repriced goods or wrong destination %+v", r)
	}
}
func TestFulfillmentOrderEditConcurrentDraftsOnlyOneWins(t *testing.T) {
	f := newFulfillmentEditFixture(t)
	ctx := context.Background()
	v, e := f.svc.FulfillmentOrderEdit(ctx, f.request.OrderID)
	if e != nil {
		t.Fatal(e)
	}
	c := editCommand(v)
	c.Items[0].Qty = 6
	p, e := f.svc.PreviewFulfillmentOrderEdit(ctx, c)
	if e != nil {
		t.Fatal(e)
	}
	c.PreviewToken = p.PreviewToken
	start := make(chan struct{})
	done := make(chan error, 2)
	for j := 0; j < 2; j++ {
		go func() { <-start; _, e := f.svc.UpdateFulfillmentOrder(ctx, c); done <- e }()
	}
	close(start)
	success := 0
	for j := 0; j < 2; j++ {
		if e := <-done; e == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("concurrent draft successes%d", success)
	}
	var pending int64
	if e = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT SUM(reserved_qty-converted_qty-released_qty)FROM %s.customer_stock_supply_reservations WHERE order_id=$1`, f.schema), v.OrderID).Scan(&pending); e != nil || pending != 6 {
		t.Fatalf("double reservation%d %v", pending, e)
	}
}

type fulfillmentEditTestAuthz struct{}

func (fulfillmentEditTestAuthz) ActorByEmployeeID(_ context.Context, id int64) (authzapp.Actor, error) {
	return authzapp.Actor{EmployeeID: id, AccountType: "internal_employee", Permissions: []string{"orders.write", "orders.read", "customers.write"}}, nil
}
func (fulfillmentEditTestAuthz) ListRoles(context.Context) ([]authzapp.Role, error) { return nil, nil }
func (fulfillmentEditTestAuthz) ListEmployeeRoles(context.Context) (map[int64][]string, error) {
	return nil, nil
}
func (fulfillmentEditTestAuthz) AssignEmployeeRoles(context.Context, authzapp.AssignmentCommand) error {
	return nil
}
func TestFulfillmentOrderEditHTTPPersistsOneOrderAndInvalidatesDocuments(t *testing.T) {
	f := newFulfillmentEditFixture(t)
	ctx := context.Background()
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(703))
			c.Set("actor", "employee:703")
			return next(c)
		}
	})
	saleshttp.RegisterRoutes(e, saleshttp.Dependencies{Sales: salesapp.NewService(postgressales.NewRepository(f.pool, f.schema)), Authz: fulfillmentEditTestAuthz{}, FulfillmentEditor: f.svc})
	path := fmt.Sprintf("/api/orders/%d/fulfillment", f.request.OrderID)
	send := func(method, url string, body any) *httptest.ResponseRecorder {
		t.Helper()
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(method, url, strings.NewReader(string(raw)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
		return w
	}
	w := send("GET", path, nil)
	if w.Code != 200 {
		t.Fatalf("GET %d %s", w.Code, w.Body.String())
	}
	var v app.FulfillmentOrderEditContext
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.sales_order_documents(order_id,version_no,snapshot_json,is_latest)VALUES($1,1,'{}',true)`, f.schema), v.OrderID); err != nil {
		t.Fatal(err)
	}
	c := editCommand(v)
	c.Items[0].Qty = 6
	c.ReceiverAddress = "北京市朝阳区测试路1号"
	c.Province, c.City, c.District = "北京市", "北京市", "朝阳区"
	w = send("POST", path+"/preview", c)
	if w.Code != 200 {
		t.Fatalf("POST preview %d %s", w.Code, w.Body.String())
	}
	var p app.FulfillmentOrderEditPreview
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	c.PreviewToken = p.PreviewToken
	w = send("PATCH", path, c)
	if w.Code != 200 {
		t.Fatalf("PATCH %d %s", w.Code, w.Body.String())
	}
	var r app.FulfillmentOrderEditResult
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if r.OrderID != v.OrderID || r.RequestID != v.RequestID || r.Items[0].ItemID != v.Items[0].ItemID || r.Items[0].Qty != 6 || r.Fees.Freight.Destination.Province != "北京市" || r.GoodsTotalCents != 0 {
		t.Fatalf("wrong API saved identity/fees %+v", r)
	}
	var latest bool
	var count int
	if err := f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT is_latest FROM %s.sales_order_documents WHERE order_id=$1`, f.schema), v.OrderID).Scan(&latest); err != nil {
		t.Fatal(err)
	}
	if latest {
		t.Fatal("old cached sales document stayed latest")
	}
	if err := f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*)FROM %s.audit_logs WHERE entity_type='order' AND entity_id=$1 AND action='fulfillment_edit'`, f.schema), v.OrderID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("business write invisible in operation log count%d %v", count, err)
	}
	var oldAudit, newAudit string
	if err := f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT old_value,new_value FROM %s.audit_logs WHERE entity_type='order' AND entity_id=$1 AND action='fulfillment_edit' ORDER BY id DESC LIMIT 1`, f.schema), v.OrderID).Scan(&oldAudit, &newAudit); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(oldAudit, v.ReceiverAddress) || !strings.Contains(newAudit, c.ReceiverAddress) || !strings.Contains(newAudit, "order_date") || !strings.Contains(newAudit, "notes") || !strings.Contains(newAudit, "fees") {
		t.Fatalf("header change not reviewable in operation log old%s new%s", oldAudit, newAudit)
	}
	// The compatibility endpoint is also fee-aware and preserves omitted company.
	old := map[string]any{"edit_revision": r.EditRevision, "receiver_name": v.ReceiverName, "receiver_phone": v.ReceiverPhone, "receiver_address": "云南省昆明市五华区测试路2号"}
	w = send("PATCH", fmt.Sprintf("/api/orders/%d/recipient", v.OrderID), old)
	if w.Code != 200 {
		t.Fatalf("legacy recipient PATCH %d %s", w.Code, w.Body.String())
	}
	updated, err := f.svc.FulfillmentOrderEdit(ctx, v.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Items[0].Qty != 6 || updated.Items[0].ItemID != v.Items[0].ItemID || updated.ReceiverCompany != "原公司" || updated.Fees.Freight.Destination.City != "昆明市" {
		t.Fatalf("legacy edit lost snapshots %+v", updated)
	}
	w = send("PATCH", path, c)
	if w.Code != 409 {
		t.Fatalf("stale PATCH %d %s", w.Code, w.Body.String())
	}
}

func TestFulfillmentOrderEditDisabledOnDemandKeepsExistingOrderEditable(t *testing.T) {
	f := newOnDemandFulfillmentEditFixture(t)
	ctx := context.Background()
	settings, e := f.repo.LoadFulfillmentSettings(ctx, 501)
	if e != nil {
		t.Fatal(e)
	}
	for j := range settings.Modes {
		settings.Modes[j].ShippingPolicy = nil
		if settings.Modes[j].Mode == "on_demand" {
			settings.Modes[j].Enabled = false
		}
	}
	if _, e = f.repo.SaveFulfillmentSettings(ctx, app.SaveFulfillmentSettingsCommand{Settings: settings, ExpectedRevision: settings.Revision, Actor: "employee:703"}); e != nil {
		t.Fatal(e)
	}
	v, e := f.svc.FulfillmentOrderEdit(ctx, f.request.OrderID)
	if e != nil || !v.CanEdit || len(v.Items) != 1 || v.CatalogWarning == "" {
		t.Fatalf("disabled mode hides old order %+v %v", v, e)
	}
	c := editCommand(v)
	c.ReceiverAddress = "云南省昆明市五华区新地址"
	c.Province, c.City, c.District = "云南省", "昆明市", "五华区"
	r := f.save(t, c)
	if r.Items[0].UnitPrice != 30 || r.Items[0].Qty != 4 {
		t.Fatalf("disabled mode repriced old order %+v", r)
	}
	c = editCommand(v)
	c.ReceiverAddress = "北京市朝阳区新地址"
	if _, e = f.svc.PreviewFulfillmentOrderEdit(ctx, c); e == nil {
		t.Fatal("address changed but retained obsolete explicit destination")
	}
}
func TestFulfillmentOrderEditRetiredIdentityIndexMigration(t *testing.T) {
	f := newFulfillmentEditFixture(t)
	ctx := context.Background()
	var item int64
	if e := f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.customer_direct_ship_request_items WHERE request_id=$1`, f.schema), f.request.ID).Scan(&item); e != nil {
		t.Fatal(e)
	}
	if _, e := f.pool.Exec(ctx, fmt.Sprintf(`DROP INDEX %[1]s.customer_direct_ship_request_items_bom_spec_identity_uq;CREATE UNIQUE INDEX customer_direct_ship_request_items_bom_spec_identity_uq ON %[1]s.customer_direct_ship_request_items(request_id,product_id,bom_spec_id)WHERE bom_spec_id>0;UPDATE %[1]s.customer_direct_ship_request_items SET qty=0 WHERE id=%[2]d`, f.schema, item)); e != nil {
		t.Fatal(e)
	}
	insert := fmt.Sprintf(`INSERT INTO %s.customer_direct_ship_request_items(request_id,line_no,product_id,bom_spec_id,bom_variant_id,qty)VALUES($1,2,910,7031,9999,1)`, f.schema)
	if _, e := f.pool.Exec(ctx, insert, f.request.ID); e == nil {
		t.Fatal("old index unexpectedly allowed a same-spec retired identity")
	}
	for j := 0; j < 2; j++ {
		if e := EnsureSchema(ctx, f.pool, f.schema); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := f.pool.Exec(ctx, insert, f.request.ID); e != nil {
		t.Fatalf("new active variant cannot retain old row: %v", e)
	}
	if _, e := f.pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_direct_ship_request_items(request_id,line_no,product_id,bom_spec_id,bom_variant_id,qty)VALUES($1,3,910,7031,8888,1)`, f.schema), f.request.ID); e == nil {
		t.Fatal("migration allowed duplicate active stable spec")
	}
	var variant int64
	if e := f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT bom_variant_id FROM %s.customer_direct_ship_request_items WHERE id=$1`, f.schema), item).Scan(&variant); e != nil || variant != 7032 {
		t.Fatalf("migration rewrote history variant%d %v", variant, e)
	}
}
func TestFulfillmentOrderEditNewPublishedVariantNeverRepurposesHistoricalLine(t *testing.T) {
	f := newFulfillmentEditFixture(t)
	ctx := context.Background()
	v, e := f.svc.FulfillmentOrderEdit(ctx, f.request.OrderID)
	if e != nil {
		t.Fatal(e)
	}
	old := v.Items[0]
	// A newer publication of the same stable specification is a distinct frozen
	// manufacturing identity for newly added merchandise.
	if _, e = f.pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.production_bom_versions SET status='archived' WHERE id=7030;INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,output_qty,output_unit)VALUES(7145,7030,'EDIT-V2','published',1,'袋');INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,logistics_weight_g,is_default)VALUES(7146,7145,7031,'454g新版','袋',500,true);UPDATE %[1]s.production_bom_output_bindings SET bom_version_id=7145 WHERE output_id=910`, f.schema)); e != nil {
		t.Fatal(e)
	}
	c := editCommand(v)
	c.Items[0].BomVariantID = 7146
	r := f.save(t, c)
	if r.Items[0].ItemID == old.ItemID || r.Items[0].RequestItemID == old.RequestItemID || r.Items[0].BomVariantID != 7146 {
		t.Fatalf("old identity reused for a new variant %+v", r.Items)
	}
	var qty, variant int64
	if e = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty::bigint,bom_variant_id FROM %s.order_items WHERE id=$1`, f.schema), old.ItemID).Scan(&qty, &variant); e != nil || qty != 0 || variant != 7032 {
		t.Fatalf("historical line rewritten qty%d variant%d %v", qty, variant, e)
	}
	if e = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty,bom_variant_id FROM %s.customer_direct_ship_request_items WHERE id=$1`, f.schema), old.RequestItemID).Scan(&qty, &variant); e != nil || qty != 0 || variant != 7032 {
		t.Fatalf("historical request line rewritten qty%d variant%d %v", qty, variant, e)
	}
}
func TestFulfillmentOrderEditCancellationAndReceiptCannotRaceReservations(t *testing.T) {
	f := newFulfillmentEditFixture(t)
	ctx := context.Background()
	v, e := f.svc.FulfillmentOrderEdit(ctx, f.request.OrderID)
	if e != nil {
		t.Fatal(e)
	}
	c := editCommand(v)
	p, e := f.svc.PreviewFulfillmentOrderEdit(ctx, c)
	if e != nil {
		t.Fatal(e)
	}
	c.PreviewToken = p.PreviewToken
	// Simulate production's pre-movement owner lock. Editing must wait, then
	// detect the converted source allocations rather than replay its old plan.
	tx, e := f.pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	if e = customerstock.LockFulfillmentCustomersTx(ctx, tx, f.schema, "EDIT-SOURCE"); e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { _, err := f.svc.UpdateFulfillmentOrder(ctx, c); done <- err }()
	select {
	case err := <-done:
		t.Fatalf("edit bypassed production owner lock: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	var batch int64
	if e = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.stock_batches(batch_code,item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,owner_customer_id,qty_units,remaining_units,quality_status)VALUES('RACE-RECEIPT','finished_product',910,'收货并发',0,7031,7032,501,2,2,'passed')RETURNING id`, f.schema)).Scan(&batch); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.finished_inventory SET onhand_units=onhand_units+2 WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='DEV-E2E-A1';INSERT INTO %[1]s.stock_ledger_entries(item_type,item_id,item_name,owner_customer_id,spec_g,bom_spec_id,bom_variant_id,warehouse,source_doc_type,source_doc_id,source_batch_code,qty_change_units,qty_after_units)VALUES('finished_product',910,'收货并发',501,0,7031,7032,'DEV-E2E-A1','production_run',703,'RACE-RECEIPT',2,2)`, f.schema)); e != nil {
		t.Fatal(e)
	}
	if e = customerstock.ReceiveProductionTx(ctx, tx, f.schema, 703, 910, 7031, 7032, 0, 2, "RACE-RECEIPT", "RACE-RECEIPT", "DEV-E2E-A1", "EDIT-SOURCE", "employee:703"); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("pre-receipt preview token overwrote converted allocations")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("receipt/edit deadlocked")
	}
	var units int64
	if e = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT SUM(allocated_units)FROM %s.order_stock_batch_allocations WHERE order_id=$1 AND batch_id=$2`, f.schema), v.OrderID, batch).Scan(&units); e != nil || units != 2 {
		t.Fatalf("receipt allocation lost units%d %v", units, e)
	}
	v, e = f.svc.FulfillmentOrderEdit(ctx, v.OrderID)
	if e != nil {
		t.Fatal(e)
	}
	c = editCommand(v)
	p, e = f.svc.PreviewFulfillmentOrderEdit(ctx, c)
	if e != nil {
		t.Fatal(e)
	}
	c.PreviewToken = p.PreviewToken
	if _, e = f.svc.CancelMiniDirectShipRequest(ctx, 501, v.RequestID, "employee:703"); e != nil {
		t.Fatal(e)
	}
	if _, e = f.svc.UpdateFulfillmentOrder(ctx, c); e == nil {
		t.Fatal("cancelled request revived by stale editor")
	}
}
func TestFulfillmentOrderEditProductionLocksBeforePhysicalMovement(t *testing.T) {
	raw, e := os.ReadFile("../production/running_repository.go")
	if e != nil {
		t.Fatal(e)
	}
	s := string(raw)
	start := strings.Index(s, "func (repo Repository) Finish(")
	body := s[start:]
	lock := strings.Index(body, "LockRunningFulfillmentCustomersTx")
	running := strings.Index(body, "FOR UPDATE")
	physical := strings.Index(body, "finishedInventoryQtyIdentityOwnedTx")
	if lock < 0 || lock > running || lock > physical {
		t.Fatal("production must acquire customer fulfillment locks before running and physical inventory rows")
	}
}

func TestFulfillmentOrderEditRejectsRegionOnlyMismatchAndProductionPlan(t *testing.T) {
	f := newFulfillmentEditFixture(t)
	ctx := context.Background()
	v, e := f.svc.FulfillmentOrderEdit(ctx, f.request.OrderID)
	if e != nil {
		t.Fatal(e)
	}
	c := editCommand(v)
	c.ReceiverAddress = "云南省昆明市五华区明确地址"
	c.Province, c.City, c.District = "云南省", "昆明市", "五华区"
	f.save(t, c)
	v, e = f.svc.FulfillmentOrderEdit(ctx, v.OrderID)
	if e != nil {
		t.Fatal(e)
	}
	c = editCommand(v)
	c.Province, c.City, c.District = "北京市", "北京市", "朝阳区"
	if _, e = f.svc.PreviewFulfillmentOrderEdit(ctx, c); e == nil || !strings.Contains(e.Error(), "不一致") {
		t.Fatalf("region-only contradiction accepted or wrong reason: %v", e)
	}
	if _, e = f.pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.production_plans(id,plan_no,status)VALUES(714,'EDIT-LOCKED-PLAN','draft');INSERT INTO %[1]s.production_plan_items(production_plan_id,product_id,bom_spec_id,bom_variant_id,order_nos)VALUES(714,910,7031,7032,'%[2]s')`, f.schema, v.OrderNo)); e != nil {
		t.Fatal(e)
	}
	v, e = f.svc.FulfillmentOrderEdit(ctx, v.OrderID)
	if e != nil || v.CanEdit || !strings.Contains(v.EditBlockReason, "生产计划") {
		t.Fatalf("production-protected context %+v %v", v, e)
	}
	c = editCommand(v)
	if _, e = f.svc.PreviewFulfillmentOrderEdit(ctx, c); e == nil {
		t.Fatal("production planned order edited")
	}
}
func TestFulfillmentOrderEditLateAuditFailureRollsBackEntireTransaction(t *testing.T) {
	f := newFulfillmentEditFixture(t)
	ctx := context.Background()
	v, e := f.svc.FulfillmentOrderEdit(ctx, f.request.OrderID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.sales_order_documents(order_id,version_no,snapshot_json,is_latest)VALUES(%[2]d,1,'{}',true);CREATE FUNCTION %[1]s.reject_fulfillment_edit_audit()RETURNS trigger LANGUAGE plpgsql AS $audit$ BEGIN IF NEW.action='fulfillment_edit' THEN RAISE EXCEPTION 'injected late audit failure';END IF;RETURN NEW;END $audit$;CREATE TRIGGER reject_fulfillment_edit_audit BEFORE INSERT ON %[1]s.audit_logs FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_fulfillment_edit_audit()`, f.schema, v.OrderID)); e != nil {
		t.Fatal(e)
	}
	c := editCommand(v)
	c.Items[0].Qty = 7
	c.ReceiverAddress = "北京市朝阳区回滚测试地址"
	c.Province, c.City, c.District = "北京市", "北京市", "朝阳区"
	c.Notes = "不应提交的备注"
	p, e := f.svc.PreviewFulfillmentOrderEdit(ctx, c)
	if e != nil {
		t.Fatal(e)
	}
	c.PreviewToken = p.PreviewToken
	if _, e = f.svc.UpdateFulfillmentOrder(ctx, c); e == nil || !strings.Contains(e.Error(), "injected late audit failure") {
		t.Fatalf("expected late failure got%v", e)
	}
	got, e := f.svc.FulfillmentOrderEdit(ctx, v.OrderID)
	if e != nil {
		t.Fatal(e)
	}
	if got.EditRevision != v.EditRevision || got.ReceiverAddress != v.ReceiverAddress || got.Notes != v.Notes || got.Items[0].ItemID != v.Items[0].ItemID || got.Items[0].Qty != 4 || got.Fees.FeeQuoteToken != v.Fees.FeeQuoteToken || got.Fees.ServiceFeeCents != v.Fees.ServiceFeeCents || got.Fees.Freight.Token != v.Fees.Freight.Token {
		t.Fatalf("late failure persisted part of transaction before%+v after%+v", v, got)
	}
	var held int64
	var latest bool
	if e = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT SUM(reserved_qty-converted_qty-released_qty)FROM %s.customer_stock_supply_reservations WHERE order_id=$1`, f.schema), v.OrderID).Scan(&held); e != nil || held != 4 {
		t.Fatalf("rights survived rollback incorrectly %d %v", held, e)
	}
	if e = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT is_latest FROM %s.sales_order_documents WHERE order_id=$1`, f.schema), v.OrderID).Scan(&latest); e != nil || !latest {
		t.Fatalf("document invalidation escaped transaction %v %v", latest, e)
	}
	var service float64
	if e = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT amount::float8 FROM %s.customer_fee_items WHERE source_type='order' AND source_id=$1 AND fee_type='direct_ship_service'`, f.schema), v.OrderID).Scan(&service); e != nil || service != float64(v.Fees.ServiceFeeCents)/100 {
		t.Fatalf("service fee escaped rollback %v %v", service, e)
	}
}

func TestFulfillmentOrderEditHistoricalZeroGrandAndStatusOnlyReceipts(t *testing.T) {
	for _, kind := range []string{"legacy_unset_grand", "calculated_zero"} {
		t.Run(kind, func(t *testing.T) {
			f := newOnDemandFulfillmentEditFixture(t)
			ctx := context.Background()
			if _, e := f.pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.pay_statuses(name)SELECT '预付款（付款未完成）'WHERE NOT EXISTS(SELECT 1 FROM %[1]s.pay_statuses WHERE name LIKE '%%预付款%%');UPDATE %[1]s.orders SET grand_total=0,pay_status_id=(SELECT id FROM %[1]s.pay_statuses WHERE name='已付款'LIMIT 1),prepayment_amount=0,payment_goods_amount=0 WHERE id=%[2]d`, f.schema, f.request.OrderID)); e != nil {
				t.Fatal(e)
			}
			if kind == "calculated_zero" {
				if _, e := f.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET discount_amount=120 WHERE id=$1`, f.schema), f.request.OrderID); e != nil {
					t.Fatal(e)
				}
			}
			v, e := f.svc.FulfillmentOrderEdit(ctx, f.request.OrderID)
			if e != nil {
				t.Fatal(e)
			}
			c := editCommand(v)
			c.Notes = "不应丢失历史收款含义"
			f.save(t, c)
			var total, prepay float64
			var status string
			if e = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT grand_total,prepayment_amount,p.name FROM %[1]s.orders o LEFT JOIN %[1]s.pay_statuses p ON p.id=o.pay_status_id WHERE o.id=$1`, f.schema), v.OrderID).Scan(&total, &prepay, &status); e != nil {
				t.Fatal(e)
			}
			expected := 120.0
			if kind == "calculated_zero" {
				expected = 0
			}
			if total != expected || prepay != 0 || status != "已付款" {
				t.Fatalf("address/header changed payment total%v prepay%v status%s", total, prepay, status)
			}
			v, e = f.svc.FulfillmentOrderEdit(ctx, v.OrderID)
			if e != nil {
				t.Fatal(e)
			}
			c = editCommand(v)
			c.Items[0].Qty = 5
			f.save(t, c)
			if e = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT grand_total,prepayment_amount,p.name FROM %[1]s.orders o LEFT JOIN %[1]s.pay_statuses p ON p.id=o.pay_status_id WHERE o.id=$1`, f.schema), v.OrderID).Scan(&total, &prepay, &status); e != nil {
				t.Fatal(e)
			}
			if kind == "legacy_unset_grand" {
				if total != 150 || prepay != 120 || !strings.Contains(status, "预付款") {
					t.Fatalf("legacy real paid lost total%v prepay%v status%s", total, prepay, status)
				}
			} else {
				if total != 30 || prepay != 0 || status != "未付款" {
					t.Fatalf("status-only unpaid transition dropped total%v prepay%v status%s", total, prepay, status)
				}
			}
		})
	}
}

func TestFulfillmentOrderEditAddressPreservesHistoricalLineDiscount(t *testing.T) {
	f := newOnDemandFulfillmentEditFixture(t)
	ctx := context.Background()
	if _, e := f.pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.order_items SET line_total_before_discount=120,discount_type='amount',discount_value=5,discount_amount=5,line_total=115 WHERE order_id=%[2]d;UPDATE %[1]s.orders SET total_amount=120,discount_amount=5,grand_total=115 WHERE id=%[2]d`, f.schema, f.request.OrderID)); e != nil {
		t.Fatal(e)
	}
	v, e := f.svc.FulfillmentOrderEdit(ctx, f.request.OrderID)
	if e != nil {
		t.Fatal(e)
	}
	c := editCommand(v)
	c.Notes = "只改备注保留原折扣"
	c.ReceiverAddress = "北京市朝阳区历史折扣地址"
	c.Province, c.City, c.District = "北京市", "北京市", "朝阳区"
	r := f.save(t, c)
	if r.Items[0].UnitPrice != 30 || r.Items[0].LineAmount != 115 || r.Items[0].BaseLineAmount != 120 || r.Items[0].DiscountAmount != 5 || r.GoodsTotalCents != 11500 {
		t.Fatalf("frozen line discount repriced %+v", r)
	}
	var gross, grand, lineBase, lineTotal, discount, headerDiscount, discountValue float64
	var kind string
	if e = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT o.total_amount,o.discount_amount,o.grand_total,i.line_total_before_discount,i.line_total,i.discount_amount,i.discount_type,i.discount_value FROM %[1]s.orders o JOIN %[1]s.order_items i ON i.order_id=o.id WHERE o.id=$1 AND i.qty>0`, f.schema), v.OrderID).Scan(&gross, &headerDiscount, &grand, &lineBase, &lineTotal, &discount, &kind, &discountValue); e != nil {
		t.Fatal(e)
	}
	if gross != 120 || headerDiscount != 5 || grand != 115 || lineBase != 120 || lineTotal != 115 || discount != 5 || kind != "amount" || discountValue != 5 {
		t.Fatalf("discount fields changed gross%v headerDiscount%v grand%v lineBase%v lineTotal%v discount%v type%s value%v", gross, headerDiscount, grand, lineBase, lineTotal, discount, kind, discountValue)
	}
	v, e = f.svc.FulfillmentOrderEdit(ctx, v.OrderID)
	if e != nil {
		t.Fatal(e)
	}
	c = editCommand(v)
	c.Items[0].Qty = 5
	if _, e = f.svc.PreviewFulfillmentOrderEdit(ctx, c); e == nil || !strings.Contains(e.Error(), "历史行折扣") {
		t.Fatalf("quantity silently rewrote historical discount: %v", e)
	}
}

func TestFulfillmentOrderEditNetGoodsContractPreservesOrderDiscountAndRounding(t *testing.T) {
	for _, lineDiscount := range []bool{true, false} {
		t.Run(fmt.Sprintf("line_discount_%v", lineDiscount), func(t *testing.T) {
			f := newOnDemandFulfillmentEditFixture(t)
			ctx := context.Background()
			itemDiscount, lineNet, headerDiscount, grand := 0.0, 120.0, 10.0, 109.8
			if lineDiscount {
				itemDiscount, lineNet, headerDiscount, grand = 5, 115, 15, 104.8
			}
			if _, e := f.pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.order_items SET line_total_before_discount=120,discount_type='amount',discount_value=%[3]v,discount_amount=%[3]v,line_total=%[4]v WHERE order_id=%[2]d;UPDATE %[1]s.orders SET total_amount=120,discount_amount=%[5]v,rounding_amount=-0.2,grand_total=%[6]v WHERE id=%[2]d`, f.schema, f.request.OrderID, itemDiscount, lineNet, headerDiscount, grand)); e != nil {
				t.Fatal(e)
			}
			v, e := f.svc.FulfillmentOrderEdit(ctx, f.request.OrderID)
			if e != nil {
				t.Fatal(e)
			}
			expected := int64(math.Round(grand * 100))
			if v.GoodsTotalCents != expected {
				t.Fatalf("context net goods%d want%d, must include full-order discount/rounding", v.GoodsTotalCents, expected)
			}
			c := editCommand(v)
			c.Notes = "只改备注，完整货款保留"
			if !lineDiscount {
				c.Items[0].Qty = 5
				expected += 3000
			}
			p, e := f.svc.PreviewFulfillmentOrderEdit(ctx, c)
			if e != nil {
				t.Fatal(e)
			}
			if p.GoodsTotalCents != expected {
				t.Fatalf("preview net goods%d want%d", p.GoodsTotalCents, expected)
			}
			c.PreviewToken = p.PreviewToken
			r, e := f.svc.UpdateFulfillmentOrder(ctx, c)
			if e != nil {
				t.Fatal(e)
			}
			if r.GoodsTotalCents != expected {
				t.Fatalf("save returned net goods%d want%d", r.GoodsTotalCents, expected)
			}
			var due, shipping, discount, rounding float64
			if e = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT grand_total,shipping_amount,discount_amount,rounding_amount FROM %s.orders WHERE id=$1`, f.schema), v.OrderID).Scan(&due, &shipping, &discount, &rounding); e != nil {
				t.Fatal(e)
			}
			if int64(math.Round((due-shipping)*100)) != expected || discount != headerDiscount || rounding != -.2 {
				t.Fatalf("saved finance differs from preview due%v shipping%v discount%v rounding%v expected%d", due, shipping, discount, rounding, expected)
			}
		})
	}
}
