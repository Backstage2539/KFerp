package customerfulfillment

import (
	"context"
	"fmt"
	"testing"
	"time"

	app "orderapp/internal/application/customerfulfillment"
	productionapp "orderapp/internal/application/production"
	salesapp "orderapp/internal/application/sales"
	postproduction "orderapp/internal/infrastructure/postgres/production"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
)

func TestMiniRetainedPurchaseKeepsOriginalProductionDemand(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	productID, price := int64(910), 30.0
	sale, err := postgressales.NewRepository(pool, schema).SaveOrder(ctx, salesapp.SaveOrderCommand{
		CustomerID: 501, PayStatusID: 1, ShipStatusID: 1, OrderDate: time.Now(), DocumentDate: time.Now(),
		DeliveryMode: "customer_stock", DeliveryWarehouseCode: "DEV-E2E-A1", Actor: "mini_user:703",
		Items: []salesapp.OrderItemCommand{{ProductID: &productID, Name: "留厂24包", Units: 24, BomSpecID: 7031, BomVariantID: 7032, SalesUnit: "bag", ManualPrice: &price}},
	})
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	cmd := app.MiniDirectShipCommand{FreightProtocol: "deferred_freight_v1", CustomerID: 501, UsageCode: "product_order", DeliveryMode: "customer_stock", DeliveryWarehouseCode: "DEV-E2E-A1", IdempotencyKey: "MINI-RETAINED24", Actor: "mini_user:703", Items: []app.MiniDirectShipItemCommand{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 24, InventoryUnit: "袋"}}}
	request, err := NewRepository(pool, schema).recordMiniDirectShipOrderTx(ctx, tx, app.PreparedMiniDirectShipOrder{Command: cmd, RequestHash: "MINI-RETAINED24"}, app.DirectShipOrderSummary{OrderID: sale.OrderID, OrderNo: sale.OrderNo})
	if err != nil {
		t.Fatal(err)
	}
	if request.OrderID != sale.OrderID {
		t.Fatalf("unexpected order %+v", request)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	data, err := postproduction.NewRepository(pool, schema).PlanSummary(ctx, productionapp.PlanSummaryQuery{CustomerID: 501})
	if err != nil {
		t.Fatal(err)
	}
	var demand float64
	for _, row := range data.Rows {
		if row.ProductID == productID {
			demand += row.SalesSpecCount
		}
	}
	if demand != 24 {
		t.Fatalf("mini retained purchase production demand=%v, want original purchase24; rows=%+v", demand, data.Rows)
	}
	var status string
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT s.name FROM %s.orders o JOIN %s.order_process_statuses s ON s.id=o.process_status_id WHERE o.id=$1`, schema, schema), sale.OrderID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "待生产" {
		t.Fatalf("retained purchase status=%q", status)
	}
}

func TestMiniRetainedTopupWithoutAddressRefreshesRightsAndCreatesOnlyGapDemand(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	configurePR703StockFees(t, pool, schema, 7031)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_service_capabilities(customer_id,capability_code,enabled)VALUES(501,'product_order',true) ON CONFLICT(customer_id,capability_code) DO UPDATE SET enabled=true`, schema)); err != nil {
		t.Fatal(err)
	}
	content := `{"price_rows":[{"product_id":910,"parent_product_id":910,"bom_spec_id":7031,"bom_variant_id":7032,"source_price_record_id":704454,"min_qty":1,"final_unit_price":30,"price_unit":"袋","inventory_unit":"袋","quantity_basis":"sales_spec_count","effective_sales_spec":{"product_id":910,"parent_product_id":910,"bom_spec_id":7031,"bom_variant_id":7032,"net_content_qty":454,"net_content_unit":"g","sales_unit":"袋","inventory_unit":"袋"}}]}`
	var publication int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.bean_list_publications(list_type,version_no,status,owner_type,owner_key,publication_purpose,content_json)VALUES('commercial','PR704-RETAINED','published','official','','factory_supply',$1::jsonb)RETURNING id`, schema), content).Scan(&publication); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_order_price_table_bindings(customer_id,usage_code,product_type_key,publication_id)VALUES(501,'product_order','commercial',$1)`, schema), publication); err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(pool, schema)
	svc := app.NewService(repository)
	svc.UseSalesOrderService(salesapp.NewService(postgressales.NewRepository(pool, schema)))
	purchase := func(key string, qty int64) app.MiniDirectShipRequest {
		t.Helper()
		cmd := app.MiniDirectShipCommand{FreightProtocol: "deferred_freight_v1", CustomerID: 501, DeliveryMode: "customer_stock", DeliveryWarehouseCode: "DEV-E2E-A1", IdempotencyKey: key, Actor: "employee:703", Items: []app.MiniDirectShipItemCommand{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: qty, SalesUnit: "袋"}}}
		quote, err := svc.PreviewMiniProductOrder(ctx, cmd)
		if err != nil {
			t.Fatal(err)
		}
		cmd.PriceQuoteToken = quote.PriceQuoteToken
		request, err := svc.SubmitMiniProductOrder(ctx, cmd)
		if err != nil {
			t.Fatal(err)
		}
		return request
	}
	purchase("PR704-ORIGINAL20", 20)
	drop := app.MiniDirectShipCommand{FreightProtocol: "deferred_freight_v1", CustomerID: 501, FulfillmentMode: "stock", IdempotencyKey: "PR704-SHIP22", Actor: "employee:703", RecipientName: "测试", RecipientPhone: "13800000000", Province: "云南省", DetailAddress: "测试路", Items: []app.MiniDirectShipItemCommand{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 22}}}
	before, err := svc.PreviewMiniDirectShip(ctx, drop)
	if err != nil || before.CanSubmit || !before.NeedsTopup || before.PurchaseGapQty != 2 {
		t.Fatalf("before topup: %+v %v", before, err)
	}
	topup := purchase("PR704-TOPUP2", 2)
	if retry := purchase("PR704-TOPUP2", 2); retry.OrderID != topup.OrderID {
		t.Fatal("topup retry created another purchase")
	}
	after, err := svc.PreviewMiniDirectShip(ctx, drop)
	if err != nil || !after.CanSubmit || after.StockReady {
		t.Fatalf("after topup should allow reservation, not shipment: %+v %v", after, err)
	}
	drop.PriceQuoteToken, drop.FeeQuoteToken = after.PriceQuoteToken, after.FeeQuoteToken
	if _, err = svc.SubmitMiniDirectShip(ctx, drop); err != nil {
		t.Fatal(err)
	}
	var count, promised, physical int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*),COALESCE(SUM(promised_qty),0) FROM %s.customer_stock_entitlements WHERE customer_id=501 AND status='active'`, schema)).Scan(&count, &promised); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(onhand_units),0) FROM %s.finished_inventory WHERE product_id=910 AND owner_customer_id=501`, schema)).Scan(&physical); err != nil {
		t.Fatal(err)
	}
	if count != 2 || promised != 22 || physical != 0 {
		t.Fatalf("rights count=%d promised=%d physical=%d", count, promised, physical)
	}
	plan, err := postproduction.NewRepository(pool, schema).PlanSummary(ctx, productionapp.PlanSummaryQuery{CustomerID: 501})
	if err != nil {
		t.Fatal(err)
	}
	var demand float64
	for _, row := range plan.Rows {
		if row.ProductID == 910 {
			demand += row.SalesSpecCount
		}
	}
	if demand != 22 {
		t.Fatalf("production demand %v, want original20 + topup2, no duplicate ship22", demand)
	}
}
